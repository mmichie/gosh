package gosh

import (
	"fmt"
	"strings"
	"unicode"
)

// PreprocessFunctionDefinitions extracts shell function definitions from input
// and registers them in GlobalState. Supports both syntaxes:
//
//	name() { body }
//	function name { body }
//	function name() { body }
//
// Definitions are matched only when they appear at the start of a logical
// statement (start of input or after `;`/newline). The matched definition is
// replaced with `:` (the true builtin) so the remaining input still parses
// and runs cleanly when no following command is present.
func PreprocessFunctionDefinitions(input string) (string, error) {
	gs := GetGlobalState()
	var out strings.Builder
	i := 0
	atStart := true
	for i < len(input) {
		// Skip leading whitespace at the start of a statement.
		if atStart {
			for i < len(input) && (input[i] == ' ' || input[i] == '\t' || input[i] == '\n') {
				out.WriteByte(input[i])
				i++
			}
		}
		if i >= len(input) {
			break
		}
		if atStart {
			consumed, name, body, ok, err := matchFunctionDef(input, i)
			if err != nil {
				return "", err
			}
			if ok {
				gs.SetFunction(name, body)
				out.WriteByte(':')
				i += consumed
				atStart = false
				continue
			}
		}
		c := input[i]
		out.WriteByte(c)
		atStart = c == ';' || c == '\n'
		// Skip over quoted strings so a `;` or `{` inside a quote doesn't
		// reset our statement boundary.
		if c == '\'' {
			i++
			for i < len(input) && input[i] != '\'' {
				out.WriteByte(input[i])
				i++
			}
			if i < len(input) {
				out.WriteByte(input[i])
				i++
			}
			continue
		}
		if c == '"' {
			i++
			for i < len(input) && input[i] != '"' {
				if input[i] == '\\' && i+1 < len(input) {
					out.WriteByte(input[i])
					out.WriteByte(input[i+1])
					i += 2
					continue
				}
				out.WriteByte(input[i])
				i++
			}
			if i < len(input) {
				out.WriteByte(input[i])
				i++
			}
			continue
		}
		i++
	}
	return out.String(), nil
}

// matchFunctionDef tries to read a function definition starting at offset i in
// input. On match returns the number of bytes consumed, the function name, the
// body source (between the matching braces), and ok=true.
func matchFunctionDef(input string, i int) (consumed int, name, body string, ok bool, err error) {
	start := i
	// Optional "function" keyword.
	hasKeyword := false
	if hasPrefixAt(input, i, "function") {
		afterKw := i + len("function")
		if afterKw < len(input) && (input[afterKw] == ' ' || input[afterKw] == '\t') {
			hasKeyword = true
			i = afterKw
			i = skipSpaceTabs(input, i)
		}
	}

	// Identifier.
	nameStart := i
	for i < len(input) && isFuncNameByte(input[i]) {
		i++
	}
	if i == nameStart {
		return 0, "", "", false, nil
	}
	name = input[nameStart:i]
	i = skipSpaceTabs(input, i)

	// Optional ().
	hasParens := false
	if i+1 < len(input) && input[i] == '(' && input[i+1] == ')' {
		hasParens = true
		i += 2
		i = skipSpaceTabs(input, i)
	}

	// Without either keyword or (), it's not a function def.
	if !hasKeyword && !hasParens {
		return 0, "", "", false, nil
	}

	// Skip over any newlines between the header and the body.
	for i < len(input) && (input[i] == ' ' || input[i] == '\t' || input[i] == '\n') {
		i++
	}

	// Body must open with `{`.
	if i >= len(input) || input[i] != '{' {
		return 0, "", "", false, nil
	}
	bodyStart := i + 1
	closeIdx, err := findMatchingBraceQuoted(input, i)
	if err != nil {
		return 0, "", "", false, err
	}
	if closeIdx == -1 {
		return 0, "", "", false, fmt.Errorf("function %s: missing closing brace", name)
	}
	body = strings.TrimSpace(input[bodyStart:closeIdx])
	return closeIdx + 1 - start, name, body, true, nil
}

// findMatchingBraceQuoted returns the index of the matching `}` for the `{`
// at openIdx, accounting for quoted strings and comments. Returns -1 if the
// brace is unmatched.
func findMatchingBraceQuoted(input string, openIdx int) (int, error) {
	if input[openIdx] != '{' {
		return -1, fmt.Errorf("findMatchingBraceQuoted: expected '{' at offset %d", openIdx)
	}
	depth := 1
	i := openIdx + 1
	for i < len(input) {
		c := input[i]
		switch c {
		case '\\':
			i += 2
			continue
		case '\'':
			i++
			for i < len(input) && input[i] != '\'' {
				i++
			}
			if i < len(input) {
				i++
			}
			continue
		case '"':
			i++
			for i < len(input) && input[i] != '"' {
				if input[i] == '\\' && i+1 < len(input) {
					i += 2
					continue
				}
				i++
			}
			if i < len(input) {
				i++
			}
			continue
		case '#':
			// Comment runs to end of line, but only when # starts a token.
			if i == 0 || input[i-1] == ' ' || input[i-1] == '\t' || input[i-1] == '\n' || input[i-1] == ';' {
				for i < len(input) && input[i] != '\n' {
					i++
				}
				continue
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
		i++
	}
	return -1, nil
}

func hasPrefixAt(s string, i int, prefix string) bool {
	return i+len(prefix) <= len(s) && s[i:i+len(prefix)] == prefix
}

func skipSpaceTabs(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func isFuncNameByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '.'
}

// CallUserFunction invokes a user-defined function by name. It pushes a new
// scope, sets positional parameters, executes the body, and restores prior
// state. Returns ReturnError unwrapped: a `return N` inside the body just
// sets the exit code.
func CallUserFunction(name string, args []string, parent *Command) error {
	gs := GetGlobalState()
	body, ok := gs.GetFunction(name)
	if !ok {
		return fmt.Errorf("%s: function not defined", name)
	}

	oldParams := gs.GetPositionalParams()
	gs.SetPositionalParams(args)
	gs.PushScope()
	defer func() {
		gs.PopScope()
		gs.SetPositionalParams(oldParams)
	}()

	// Split the body into statements separated by `;` or newline, respecting
	// quotes and braces (so subshells and nested groups stay intact).
	stmts := splitFunctionBody(body)
	parent.ReturnCode = 0
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		execCmd, err := NewCommand(stmt, parent.JobManager)
		if err != nil {
			parent.ReturnCode = 1
			return fmt.Errorf("%s: %v", name, err)
		}
		execCmd.Stdin = parent.Stdin
		execCmd.Stdout = parent.Stdout
		execCmd.Stderr = parent.Stderr
		execCmd.Run()
		parent.ReturnCode = execCmd.ReturnCode

		// If the body invoked `return`, propagate the code and stop.
		if execCmd.ReturnRequested {
			return nil
		}
	}
	return nil
}

// splitFunctionBody walks body and splits on top-level statement separators
// (`;` and newline) without breaking quoted strings or nested braces.
func splitFunctionBody(body string) []string {
	var parts []string
	var cur strings.Builder
	depth := 0
	parenDepth := 0
	flush := func() {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			parts = append(parts, s)
		}
		cur.Reset()
	}
	i := 0
	for i < len(body) {
		c := body[i]
		switch c {
		case '\\':
			cur.WriteByte(c)
			if i+1 < len(body) {
				cur.WriteByte(body[i+1])
				i += 2
				continue
			}
		case '\'':
			cur.WriteByte(c)
			i++
			for i < len(body) && body[i] != '\'' {
				cur.WriteByte(body[i])
				i++
			}
			if i < len(body) {
				cur.WriteByte(body[i])
				i++
			}
			continue
		case '"':
			cur.WriteByte(c)
			i++
			for i < len(body) && body[i] != '"' {
				if body[i] == '\\' && i+1 < len(body) {
					cur.WriteByte(body[i])
					cur.WriteByte(body[i+1])
					i += 2
					continue
				}
				cur.WriteByte(body[i])
				i++
			}
			if i < len(body) {
				cur.WriteByte(body[i])
				i++
			}
			continue
		case '{':
			depth++
			cur.WriteByte(c)
		case '}':
			depth--
			cur.WriteByte(c)
		case '(':
			parenDepth++
			cur.WriteByte(c)
		case ')':
			parenDepth--
			cur.WriteByte(c)
		case ';', '\n':
			if depth == 0 && parenDepth == 0 {
				flush()
			} else {
				cur.WriteByte(c)
			}
		default:
			cur.WriteByte(c)
		}
		i++
	}
	flush()
	return parts
}

// IsValidFunctionName returns true if name is a syntactically valid shell
// function name (used by the `unset` and reflection paths).
func IsValidFunctionName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if i == 0 && (c >= '0' && c <= '9') {
			return false
		}
		if !isFuncNameByte(c) {
			return false
		}
	}
	return !unicode.IsDigit(rune(name[0]))
}
