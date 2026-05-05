package gosh

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ExpandParameterExpansions scans s for ${...} expressions and expands each,
// honoring balanced braces inside the expression so patterns like
// ${var/foo/bar} and substitutions containing further ${...} work correctly.
//
// Returns an error only for explicit failures such as ${var:?msg} on an unset
// variable, mirroring POSIX semantics.
func ExpandParameterExpansions(s string) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			b.WriteByte(c)
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c == '$' && i+1 < len(s) && s[i+1] == '{' {
			end := findMatchingBrace(s, i+1)
			if end == -1 {
				b.WriteByte(c)
				i++
				continue
			}
			expr := s[i+2 : end]
			expanded, err := expandParameterRef(expr)
			if err != nil {
				return "", err
			}
			b.WriteString(expanded)
			i = end + 1
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String(), nil
}

// findMatchingBrace returns the index of the } matching the { at openIdx.
// Returns -1 if no match exists. Skips escaped characters.
func findMatchingBrace(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// expandParameterRef expands the inside of a single ${...} expression
// (without the surrounding ${ and }).
func expandParameterRef(expr string) (string, error) {
	if expr == "" {
		return "", fmt.Errorf("${}: bad substitution")
	}

	// Length: ${#name}
	if expr[0] == '#' && len(expr) > 1 {
		name := expr[1:]
		if name == "@" || name == "*" {
			return strconv.Itoa(GetGlobalState().GetPositionalParamCount()), nil
		}
		val, _ := lookupShellVar(name)
		return strconv.Itoa(len(val)), nil
	}

	name, rest := splitParamName(expr)
	if name == "" {
		return "", fmt.Errorf("${%s}: bad substitution", expr)
	}

	val, isSet := lookupShellVar(name)

	if rest == "" {
		return val, nil
	}

	return applyParamOperator(name, val, isSet, rest)
}

// splitParamName extracts the leading parameter name from an expression and
// returns the remainder (the operator and its argument, if any).
func splitParamName(expr string) (name, rest string) {
	if expr == "" {
		return "", ""
	}
	switch expr[0] {
	case '@', '*', '?', '!':
		return expr[:1], expr[1:]
	}
	if expr[0] >= '0' && expr[0] <= '9' {
		i := 1
		for i < len(expr) && expr[i] >= '0' && expr[i] <= '9' {
			i++
		}
		return expr[:i], expr[i:]
	}
	if isParamIDStart(expr[0]) {
		i := 1
		for i < len(expr) && isParamIDCont(expr[i]) {
			i++
		}
		return expr[:i], expr[i:]
	}
	return "", expr
}

func isParamIDStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isParamIDCont(c byte) bool {
	return isParamIDStart(c) || (c >= '0' && c <= '9')
}

// lookupShellVar resolves a name against the shell's variable namespace,
// covering specials, positional parameters, and environment variables.
func lookupShellVar(name string) (string, bool) {
	state := GetGlobalState()
	switch name {
	case "?":
		return strconv.Itoa(state.GetLastExitStatus()), true
	case "!":
		return strconv.Itoa(state.GetLastBackgroundPID()), true
	case "#":
		return strconv.Itoa(state.GetPositionalParamCount()), true
	case "@", "*":
		return strings.Join(state.GetPositionalParams(), " "), true
	}
	if num, err := strconv.Atoi(name); err == nil {
		if num == 0 {
			return state.GetScriptName(), true
		}
		val := state.GetPositionalParam(num)
		return val, num <= state.GetPositionalParamCount()
	}
	val, ok := os.LookupEnv(name)
	return val, ok
}

// applyParamOperator dispatches on the operator that follows the name in a
// ${name<op>...} expression. rest begins with the operator character(s).
func applyParamOperator(name, val string, isSet bool, rest string) (string, error) {
	colonForm := false
	op := rest
	if op[0] == ':' && len(op) > 1 && strings.IndexByte("-=?+", op[1]) >= 0 {
		colonForm = true
		op = op[1:]
	}

	switch op[0] {
	case '-':
		word, err := ExpandSpecialVariablesE(op[1:])
		if err != nil {
			return "", err
		}
		if !isSet || (colonForm && val == "") {
			return word, nil
		}
		return val, nil
	case '=':
		word, err := ExpandSpecialVariablesE(op[1:])
		if err != nil {
			return "", err
		}
		if !isSet || (colonForm && val == "") {
			if err := assignParamDefault(name, word); err != nil {
				return "", err
			}
			return word, nil
		}
		return val, nil
	case '?':
		word, err := ExpandSpecialVariablesE(op[1:])
		if err != nil {
			return "", err
		}
		if !isSet || (colonForm && val == "") {
			if word == "" {
				word = "parameter null or not set"
			}
			return "", fmt.Errorf("%s: %s", name, word)
		}
		return val, nil
	case '+':
		word, err := ExpandSpecialVariablesE(op[1:])
		if err != nil {
			return "", err
		}
		if isSet && !(colonForm && val == "") {
			return word, nil
		}
		return "", nil
	}

	if colonForm {
		return "", fmt.Errorf("${%s%s}: bad substitution", name, rest)
	}

	switch op[0] {
	case '#':
		longest := strings.HasPrefix(op, "##")
		patStart := 1
		if longest {
			patStart = 2
		}
		pattern, err := ExpandSpecialVariablesE(op[patStart:])
		if err != nil {
			return "", err
		}
		return removePrefixPattern(val, pattern, longest), nil
	case '%':
		longest := strings.HasPrefix(op, "%%")
		patStart := 1
		if longest {
			patStart = 2
		}
		pattern, err := ExpandSpecialVariablesE(op[patStart:])
		if err != nil {
			return "", err
		}
		return removeSuffixPattern(val, pattern, longest), nil
	case '/':
		return substituteParam(val, op[1:])
	case '^':
		all := strings.HasPrefix(op, "^^")
		return modifyCase(val, op[boolToInt(all)+1:], unicode.ToUpper, all), nil
	case ',':
		all := strings.HasPrefix(op, ",,")
		return modifyCase(val, op[boolToInt(all)+1:], unicode.ToLower, all), nil
	case ':':
		return substringParam(val, op[1:])
	}

	return "", fmt.Errorf("${%s%s}: bad substitution", name, rest)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// assignParamDefault implements the ${var:=word} side effect, writing word
// into the appropriate scope (local if shadowing a function local, otherwise
// the environment).
func assignParamDefault(name, value string) error {
	gs := GetGlobalState()
	if gs.IsReadonly(name) {
		return fmt.Errorf("%s: readonly variable", name)
	}
	if gs.IsInFunction() {
		if _, isLocal := gs.GetLocalVar(name); isLocal {
			return gs.SetLocalVar(name, value)
		}
	}
	return gs.SetEnvVar(name, value)
}

// removePrefixPattern strips the matching leading portion of s. When longest
// is true, the longest match wins; otherwise the shortest. The prefix case
// can rely on regex greediness because anchoring is one-sided (start only).
func removePrefixPattern(s, pattern string, longest bool) string {
	if pattern == "" {
		return s
	}
	re, err := globToAnchoredRegex(pattern, true, false, longest)
	if err != nil {
		return s
	}
	if loc := re.FindStringIndex(s); loc != nil && loc[0] == 0 {
		return s[loc[1]:]
	}
	return s
}

// removeSuffixPattern strips the matching trailing portion of s. Anchoring
// pattern$ forces the regex engine to extend any match to the end, so a
// non-greedy * doesn't help here — we have to scan candidate split points
// directly.
func removeSuffixPattern(s, pattern string, longest bool) string {
	if pattern == "" {
		return s
	}
	re, err := globToAnchoredRegex(pattern, true, true, longest)
	if err != nil {
		return s
	}
	if longest {
		for i := 0; i <= len(s); i++ {
			if re.MatchString(s[i:]) {
				return s[:i]
			}
		}
	} else {
		for i := len(s); i >= 0; i-- {
			if re.MatchString(s[i:]) {
				return s[:i]
			}
		}
	}
	return s
}

// substituteParam implements ${var/pat/repl}, ${var//pat/repl},
// ${var/#pat/repl}, ${var/%pat/repl}.
func substituteParam(val, spec string) (string, error) {
	all := false
	anchor := byte(0)
	if len(spec) > 0 {
		switch spec[0] {
		case '/':
			all = true
			spec = spec[1:]
		case '#':
			anchor = '#'
			spec = spec[1:]
		case '%':
			anchor = '%'
			spec = spec[1:]
		}
	}
	pattern, replacement, hasRepl := splitPatternReplacement(spec)
	pattern, err := ExpandSpecialVariablesE(pattern)
	if err != nil {
		return "", err
	}
	if hasRepl {
		replacement, err = ExpandSpecialVariablesE(replacement)
		if err != nil {
			return "", err
		}
	}
	if pattern == "" {
		return val, nil
	}
	re, err := globToAnchoredRegex(pattern, anchor == '#', anchor == '%', true)
	if err != nil {
		return val, nil
	}
	if all {
		return re.ReplaceAllString(val, escapeReplacement(replacement)), nil
	}
	if loc := re.FindStringIndex(val); loc != nil {
		return val[:loc[0]] + replacement + val[loc[1]:], nil
	}
	return val, nil
}

// splitPatternReplacement splits at the first unescaped /, returning the
// pattern, the replacement, and whether a / was present at all.
func splitPatternReplacement(s string) (pattern, replacement string, hasRepl bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == '/' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// escapeReplacement escapes characters meaningful to regexp.ReplaceAllString
// so that the user's replacement text is treated literally.
func escapeReplacement(s string) string {
	return strings.ReplaceAll(s, "$", "$$")
}

// modifyCase applies fn to the matching characters in s. When all is true,
// every matching position is transformed; otherwise just the first character.
// pattern (currently * by default when empty) selects which characters match;
// other glob forms aren't supported here yet.
func modifyCase(s, pattern string, fn func(rune) rune, all bool) string {
	matches := func(r rune) bool {
		if pattern == "" || pattern == "*" || pattern == "?" {
			return true
		}
		// Character class like [aeiou]
		if len(pattern) >= 2 && pattern[0] == '[' && pattern[len(pattern)-1] == ']' {
			class := pattern[1 : len(pattern)-1]
			for _, c := range class {
				if c == r {
					return true
				}
			}
			return false
		}
		// Single literal character
		runes := []rune(pattern)
		return len(runes) == 1 && runes[0] == r
	}
	var b strings.Builder
	done := false
	for _, r := range s {
		if !done && matches(r) {
			b.WriteRune(fn(r))
			if !all {
				done = true
			}
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// substringParam implements ${var:offset[:length]}. Offsets and lengths can
// be negative; negative offsets count from the end, negative lengths count
// back from the end of the string.
func substringParam(val, spec string) (string, error) {
	expanded, err := ExpandSpecialVariablesE(spec)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(expanded, ":", 2)
	offset, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return "", fmt.Errorf("substring: %s: invalid offset", parts[0])
	}
	runes := []rune(val)
	n := len(runes)
	if offset < 0 {
		offset = n + offset
		if offset < 0 {
			offset = 0
		}
	}
	if offset > n {
		offset = n
	}
	if len(parts) == 1 {
		return string(runes[offset:]), nil
	}
	length, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", fmt.Errorf("substring: %s: invalid length", parts[1])
	}
	end := offset + length
	if length < 0 {
		end = n + length
	}
	if end < offset {
		return "", nil
	}
	if end > n {
		end = n
	}
	return string(runes[offset:end]), nil
}

// globToAnchoredRegex compiles a shell-style pattern (* ? [...]) into a Go
// regexp. anchorStart/anchorEnd anchor the match; longest selects greedy
// (longest) versus non-greedy (shortest) repetition for *.
func globToAnchoredRegex(pattern string, anchorStart, anchorEnd, longest bool) (*regexp.Regexp, error) {
	var b strings.Builder
	if anchorStart {
		b.WriteByte('^')
	}
	star := ".*"
	if !longest {
		star = ".*?"
	}
	question := "."
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			b.WriteString(star)
		case '?':
			b.WriteString(question)
		case '[':
			end := strings.IndexByte(pattern[i:], ']')
			if end == -1 {
				b.WriteString(regexp.QuoteMeta(string(c)))
				continue
			}
			b.WriteString(pattern[i : i+end+1])
			i += end
		case '\\':
			if i+1 < len(pattern) {
				b.WriteString(regexp.QuoteMeta(string(pattern[i+1])))
				i++
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if anchorEnd {
		b.WriteByte('$')
	}
	return regexp.Compile(b.String())
}
