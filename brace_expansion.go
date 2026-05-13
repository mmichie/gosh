package gosh

import (
	"strconv"
	"strings"
)

// ExpandBraces performs bash-style brace expansion on input as a preprocessor
// pass. It rewrites words containing `{a,b,c}`, `{1..5}`, `{a..z..2}` and
// nested/cartesian combinations into the equivalent space-separated forms.
//
// Brace expansion is purely syntactic: it never evaluates `$x`, `$(...)`,
// `${...}`, or `$((...))` inside braces, and it leaves single- and
// double-quoted regions untouched. Braces preceded by `$` (parameter
// expansion) are also skipped. Invalid brace groups (no comma, no `..`, or
// no matching `}`) pass through unchanged, matching bash behavior.
func ExpandBraces(input string) string {
	var out strings.Builder
	i := 0
	wordStart := 0
	for i < len(input) {
		c := input[i]
		if isWordSeparator(c) {
			if wordStart < i {
				expandAndEmit(input[wordStart:i], &out)
			}
			out.WriteByte(c)
			i++
			wordStart = i
			continue
		}
		switch c {
		case '\'':
			i = skipSingleQuoted(input, i)
		case '"':
			i = skipDoubleQuoted(input, i)
		case '\\':
			if i+1 < len(input) {
				i += 2
			} else {
				i++
			}
		case '$':
			i = skipDollarConstruct(input, i)
		default:
			i++
		}
	}
	if wordStart < len(input) {
		expandAndEmit(input[wordStart:], &out)
	}
	return out.String()
}

// isWordSeparator reports whether c terminates the current shell word at the
// top level. We intentionally include only the unambiguous separators —
// anything more nuanced would require running the actual parser.
func isWordSeparator(c byte) bool {
	switch c {
	case ' ', '\t', '\n', ';', '|', '&', '<', '>', '(', ')':
		return true
	}
	return false
}

func expandAndEmit(word string, out *strings.Builder) {
	results := expandWord(word)
	for i, r := range results {
		if i > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(r)
	}
}

// expandWord recursively expands one shell word. It locates the first valid
// brace group, splits it into options, expands each option, and takes the
// cross product with the recursively expanded suffix.
func expandWord(word string) []string {
	open, closeIdx, options, ok := findBraceGroup(word)
	if !ok {
		return []string{word}
	}
	prefix := word[:open]
	suffix := word[closeIdx+1:]
	suffixExpansions := expandWord(suffix)
	results := make([]string, 0, len(options)*len(suffixExpansions))
	for _, opt := range options {
		for _, optExp := range expandWord(opt) {
			for _, suf := range suffixExpansions {
				results = append(results, prefix+optExp+suf)
			}
		}
	}
	if len(results) == 0 {
		return []string{word}
	}
	return results
}

// findBraceGroup scans word for the first `{...}` that forms a valid brace
// expansion. On success it returns the indices of `{` and `}` plus the list
// of expanded option strings. `..` ranges are pre-expanded here so the
// caller can treat every option uniformly.
func findBraceGroup(word string) (open, closeIdx int, options []string, ok bool) {
	i := 0
	for i < len(word) {
		c := word[i]
		switch c {
		case '\'':
			i = skipSingleQuoted(word, i)
			continue
		case '"':
			i = skipDoubleQuoted(word, i)
			continue
		case '\\':
			if i+1 < len(word) {
				i += 2
			} else {
				i++
			}
			continue
		case '$':
			i = skipDollarConstruct(word, i)
			continue
		case '{':
			if end, opts, valid := tryParseBraceGroup(word, i); valid {
				return i, end, opts, true
			}
		}
		i++
	}
	return 0, 0, nil, false
}

// tryParseBraceGroup attempts to parse a brace group starting at openIdx
// (where word[openIdx] == '{'). It returns the index of the matching `}`,
// the list of option strings (with `..` ranges expanded), and ok=true when
// the contents form a real brace expansion (i.e. contains a top-level comma
// or a `..` range).
func tryParseBraceGroup(word string, openIdx int) (closeIdx int, options []string, ok bool) {
	depth := 1
	i := openIdx + 1
	contentStart := i
	commaPositions := []int{}
	for i < len(word) {
		c := word[i]
		switch c {
		case '\'':
			i = skipSingleQuoted(word, i)
			continue
		case '"':
			i = skipDoubleQuoted(word, i)
			continue
		case '\\':
			if i+1 < len(word) {
				i += 2
			} else {
				i++
			}
			continue
		case '$':
			i = skipDollarConstruct(word, i)
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				content := word[contentStart:i]
				if len(commaPositions) > 0 {
					opts := splitOnPositions(content, commaPositions, contentStart)
					return i, opts, true
				}
				if seq, ok := expandSequence(content); ok {
					return i, seq, true
				}
				return 0, nil, false
			}
		case ',':
			if depth == 1 {
				commaPositions = append(commaPositions, i)
			}
		}
		i++
	}
	return 0, nil, false
}

// splitOnPositions slices content at the given absolute comma positions.
// positions are indices into the parent word, and contentStart is the
// absolute index of content[0].
func splitOnPositions(content string, positions []int, contentStart int) []string {
	out := make([]string, 0, len(positions)+1)
	prev := 0
	for _, p := range positions {
		rel := p - contentStart
		out = append(out, content[prev:rel])
		prev = rel + 1
	}
	out = append(out, content[prev:])
	return out
}

// expandSequence handles `{a..b}` and `{a..b..step}` for numeric and single
// character endpoints. Returns the expanded option list and ok=true when the
// content matches the sequence form, otherwise ok=false (caller falls back
// to leaving the braces untouched).
func expandSequence(content string) ([]string, bool) {
	parts := strings.Split(content, "..")
	if len(parts) != 2 && len(parts) != 3 {
		return nil, false
	}
	step := int64(0)
	hasStep := false
	if len(parts) == 3 {
		s, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, false
		}
		step = s
		hasStep = true
	}
	if seq, ok := expandNumericSequence(parts[0], parts[1], step, hasStep); ok {
		return seq, true
	}
	if seq, ok := expandCharSequence(parts[0], parts[1], step, hasStep); ok {
		return seq, true
	}
	return nil, false
}

func expandNumericSequence(startStr, endStr string, step int64, hasStep bool) ([]string, bool) {
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return nil, false
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return nil, false
	}
	width := 0
	if zeroPaddedWidth(startStr) > 0 || zeroPaddedWidth(endStr) > 0 {
		w1 := paddedDigitWidth(startStr)
		w2 := paddedDigitWidth(endStr)
		if w1 > w2 {
			width = w1
		} else {
			width = w2
		}
	}
	if !hasStep {
		if end >= start {
			step = 1
		} else {
			step = -1
		}
	}
	if step == 0 {
		return nil, false
	}
	if (end > start && step < 0) || (end < start && step > 0) {
		step = -step
	}
	out := []string{}
	guard := 0
	if step > 0 {
		for v := start; v <= end; v += step {
			out = append(out, formatPaddedInt(v, width))
			guard++
			if guard > 1000000 {
				return nil, false
			}
		}
	} else {
		for v := start; v >= end; v += step {
			out = append(out, formatPaddedInt(v, width))
			guard++
			if guard > 1000000 {
				return nil, false
			}
		}
	}
	return out, true
}

// zeroPaddedWidth returns the width of a literal numeric token if it is
// zero-padded (i.e. starts with `0` or `-0` and has more than one digit
// character). A bare `0` is not considered padded.
func zeroPaddedWidth(s string) int {
	if s == "" {
		return 0
	}
	digits := s
	if digits[0] == '-' || digits[0] == '+' {
		digits = digits[1:]
	}
	if len(digits) < 2 {
		return 0
	}
	if digits[0] != '0' {
		return 0
	}
	return len(s)
}

// paddedDigitWidth returns the full width (including sign) of a numeric
// literal, used when determining the output column width for zero-padding.
func paddedDigitWidth(s string) int {
	return len(s)
}

func formatPaddedInt(v int64, width int) string {
	if width == 0 {
		return strconv.FormatInt(v, 10)
	}
	negative := v < 0
	abs := v
	if negative {
		abs = -v
	}
	s := strconv.FormatInt(abs, 10)
	padTo := width
	if negative {
		padTo = width - 1
	}
	for len(s) < padTo {
		s = "0" + s
	}
	if negative {
		s = "-" + s
	}
	return s
}

func expandCharSequence(startStr, endStr string, step int64, hasStep bool) ([]string, bool) {
	if len(startStr) != 1 || len(endStr) != 1 {
		return nil, false
	}
	start := startStr[0]
	end := endStr[0]
	if !isSequenceChar(start) || !isSequenceChar(end) {
		return nil, false
	}
	if !hasStep {
		if end >= start {
			step = 1
		} else {
			step = -1
		}
	}
	if step == 0 {
		return nil, false
	}
	if (end > start && step < 0) || (end < start && step > 0) {
		step = -step
	}
	out := []string{}
	if step > 0 {
		for v := int(start); v <= int(end); v += int(step) {
			out = append(out, string(byte(v)))
		}
	} else {
		for v := int(start); v >= int(end); v += int(step) {
			out = append(out, string(byte(v)))
		}
	}
	return out, true
}

func isSequenceChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// skipSingleQuoted returns the index just past a single-quoted region
// starting at word[i] == '\”. Single quotes have no escapes.
func skipSingleQuoted(s string, i int) int {
	i++
	for i < len(s) && s[i] != '\'' {
		i++
	}
	if i < len(s) {
		i++
	}
	return i
}

// skipDoubleQuoted returns the index just past a double-quoted region
// starting at word[i] == '"'. Backslash escapes the next byte.
func skipDoubleQuoted(s string, i int) int {
	i++
	for i < len(s) && s[i] != '"' {
		if s[i] == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		i++
	}
	if i < len(s) {
		i++
	}
	return i
}

// skipDollarConstruct advances past `${...}`, `$(...)`, `$((...))`, or a
// plain `$identifier` so brace expansion never touches parameter or
// command-substitution forms. The returned index is just past the construct.
func skipDollarConstruct(s string, i int) int {
	if i >= len(s) || s[i] != '$' {
		return i + 1
	}
	if i+1 >= len(s) {
		return i + 1
	}
	switch s[i+1] {
	case '{':
		return skipBalanced(s, i+1, '{', '}')
	case '(':
		if i+2 < len(s) && s[i+2] == '(' {
			return skipDoubleParen(s, i+1)
		}
		return skipBalanced(s, i+1, '(', ')')
	}
	return i + 1
}

// skipBalanced advances past a balanced construct starting at s[start]
// (which must equal open). Tracks nesting and respects quote regions.
func skipBalanced(s string, start int, open, close byte) int {
	if start >= len(s) || s[start] != open {
		return start + 1
	}
	depth := 1
	i := start + 1
	for i < len(s) {
		switch s[i] {
		case '\'':
			i = skipSingleQuoted(s, i)
			continue
		case '"':
			i = skipDoubleQuoted(s, i)
			continue
		case '\\':
			if i+1 < len(s) {
				i += 2
				continue
			}
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return i
}

// skipDoubleParen advances past `$((...))` (caller passes the index of the
// first `(`). The inner content is treated as opaque arithmetic.
func skipDoubleParen(s string, start int) int {
	depth := 0
	i := start
	for i < len(s) {
		switch s[i] {
		case '\'':
			i = skipSingleQuoted(s, i)
			continue
		case '"':
			i = skipDoubleQuoted(s, i)
			continue
		case '\\':
			if i+1 < len(s) {
				i += 2
				continue
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if i+1 < len(s) && s[i+1] == ')' {
					return i + 2
				}
				return i + 1
			}
		}
		i++
	}
	return i
}
