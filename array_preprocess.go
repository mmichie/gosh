package gosh

import (
	"fmt"
	"strings"
)

// PreprocessArrayAssignments rewrites `name=(elem1 elem2 ...)` array-literal
// assignments out of input. The literal is parsed, stored as an indexed array
// in GlobalState, and replaced with `:` (a no-op) so the rest of the line
// still parses. Element words go through standard variable and arithmetic
// expansion at registration time, matching bash's eager-evaluation behavior.
//
// Detection rule: a valid identifier followed by `=(` at the start of a
// statement (start of input, or after `;` / newline / `&&` / `||`).
func PreprocessArrayAssignments(input string) (string, error) {
	gs := GetGlobalState()
	var out strings.Builder
	i := 0
	atStart := true
	for i < len(input) {
		// Skip whitespace at the start of a statement so the leading
		// identifier can still trigger matching.
		if atStart {
			for i < len(input) && (input[i] == ' ' || input[i] == '\t') {
				out.WriteByte(input[i])
				i++
			}
		}
		if i >= len(input) {
			break
		}
		if atStart {
			consumed, name, body, ok := matchArrayAssignment(input, i)
			if ok {
				elements, err := ParseArrayLiteral(body)
				if err != nil {
					return "", err
				}
				arr := NewIndexedArray()
				for _, raw := range elements {
					expanded, err := ExpandSpecialVariablesE(raw)
					if err != nil {
						return "", err
					}
					arr.Append(expanded)
				}
				gs.SetArray(name, arr)
				out.WriteByte(':')
				i += consumed
				atStart = false
				continue
			}
		}
		c := input[i]
		out.WriteByte(c)
		switch c {
		case ';', '\n':
			atStart = true
		case '\'':
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
		case '"':
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
		case '&', '|':
			if i+1 < len(input) && input[i+1] == c {
				out.WriteByte(input[i+1])
				i += 2
				atStart = true
				continue
			}
			atStart = false
		default:
			atStart = false
		}
		i++
	}
	return out.String(), nil
}

// matchArrayAssignment looks for `IDENT=(...)` at offset i. Returns the
// number of bytes consumed (covering the trailing `)`), the variable name, the
// raw body between `(` and `)`, and ok=true on a match. The body may contain
// quoted strings; balanced parens are NOT supported here (bash itself doesn't
// allow nested () inside an array literal).
func matchArrayAssignment(input string, i int) (consumed int, name, body string, ok bool) {
	start := i
	nameStart := i
	for i < len(input) && isArrayNameByte(input[i]) {
		i++
	}
	if i == nameStart {
		return 0, "", "", false
	}
	if i >= len(input) || input[i] != '=' {
		return 0, "", "", false
	}
	if i+1 >= len(input) || input[i+1] != '(' {
		return 0, "", "", false
	}
	name = input[nameStart:i]
	bodyStart := i + 2
	j := bodyStart
	for j < len(input) {
		switch input[j] {
		case '\'':
			j++
			for j < len(input) && input[j] != '\'' {
				j++
			}
			if j < len(input) {
				j++
			}
			continue
		case '"':
			j++
			for j < len(input) && input[j] != '"' {
				if input[j] == '\\' && j+1 < len(input) {
					j += 2
					continue
				}
				j++
			}
			if j < len(input) {
				j++
			}
			continue
		case ')':
			body = input[bodyStart:j]
			return j + 1 - start, name, body, true
		}
		j++
	}
	return 0, "", "", false
}

func isArrayNameByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// PreprocessIndexedAssignment handles `name[subscript]=value` standalone or
// prefix assignments. The parser tokenizes the LHS as one Word, so detection
// happens here on a Parts slice.
//
// Returns (consumed bool, error). When consumed is true, the input was an
// indexed assignment and was applied to the array store.
func ApplyIndexedAssignment(part string) (bool, error) {
	eq := strings.IndexByte(part, '=')
	if eq < 0 {
		return false, nil
	}
	lhs := part[:eq]
	rhs := part[eq+1:]
	open := strings.IndexByte(lhs, '[')
	if open < 0 || !strings.HasSuffix(lhs, "]") {
		return false, nil
	}
	name := lhs[:open]
	subscript := lhs[open+1 : len(lhs)-1]
	if name == "" || !isValidArrayName(name) {
		return false, nil
	}
	// Strip surrounding quotes on RHS — the parser keeps them.
	rhs = stripQuotes(rhs)
	expanded, err := ExpandSpecialVariablesE(rhs)
	if err != nil {
		return false, err
	}
	gs := GetGlobalState()
	if err := gs.SetArrayElement(name, subscript, expanded); err != nil {
		return false, fmt.Errorf("array assignment: %v", err)
	}
	return true, nil
}

func isValidArrayName(s string) bool {
	if s == "" {
		return false
	}
	if s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isArrayNameByte(s[i]) {
			return false
		}
	}
	return true
}
