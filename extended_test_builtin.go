package gosh

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// extendedTestCommand implements the bash [[ extended test command
// It provides pattern matching, regex support, and better operator handling than [ ]
func extendedTestCommand(cmd *Command) error {
	// Extract arguments from the command
	args := extractTestArgs(cmd)

	// Must start with [[ and end with ]]
	if len(args) < 1 || args[0] != "[[" {
		cmd.ReturnCode = 2
		return fmt.Errorf("[[: syntax error")
	}

	// Remove the leading [[
	args = args[1:]

	if len(args) == 0 || args[len(args)-1] != "]]" {
		cmd.ReturnCode = 2
		return fmt.Errorf("[[: missing closing ]]")
	}

	// Remove the trailing ]]
	args = args[:len(args)-1]

	// Strip quotes from operators that were quoted by the preprocessor
	for i, arg := range args {
		args[i] = stripQuotes(arg)
	}

	// Empty test is false
	if len(args) == 0 {
		cmd.ReturnCode = 1
		return nil
	}

	// Evaluate the expression
	result, err := evaluateExtendedTest(args)
	if err != nil {
		cmd.ReturnCode = 2
		return err
	}

	if result {
		cmd.ReturnCode = 0
	} else {
		cmd.ReturnCode = 1
	}

	return nil
}

// evaluateExtendedTest evaluates an extended test expression
// It handles && and || operators, parentheses, and pattern matching
func evaluateExtendedTest(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	// Parse with operator precedence: ! > && > ||
	return parseOrExpr(args)
}

// parseOrExpr handles || with lowest precedence
func parseOrExpr(args []string) (bool, error) {
	// Find top-level || (not inside parentheses)
	depth := 0
	for i := len(args) - 1; i >= 0; i-- {
		switch args[i] {
		case "(":
			depth--
		case ")":
			depth++
		case "||":
			if depth == 0 {
				left, err := parseOrExpr(args[:i])
				if err != nil {
					return false, err
				}
				// Short-circuit: if left is true, don't evaluate right
				if left {
					return true, nil
				}
				return parseOrExpr(args[i+1:])
			}
		}
	}
	return parseAndExpr(args)
}

// parseAndExpr handles && with higher precedence than ||
func parseAndExpr(args []string) (bool, error) {
	// Find top-level && (not inside parentheses)
	depth := 0
	for i := len(args) - 1; i >= 0; i-- {
		switch args[i] {
		case "(":
			depth--
		case ")":
			depth++
		case "&&":
			if depth == 0 {
				left, err := parseAndExpr(args[:i])
				if err != nil {
					return false, err
				}
				// Short-circuit: if left is false, don't evaluate right
				if !left {
					return false, nil
				}
				return parseAndExpr(args[i+1:])
			}
		}
	}
	return parseUnaryExpr(args)
}

// parseUnaryExpr handles ! and parentheses
func parseUnaryExpr(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	// Handle negation
	if args[0] == "!" {
		result, err := parseUnaryExpr(args[1:])
		return !result, err
	}

	// Handle parentheses
	if args[0] == "(" {
		// Find matching closing parenthesis
		depth := 1
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "(":
				depth++
			case ")":
				depth--
				if depth == 0 {
					// Evaluate the content inside parentheses
					result, err := evaluateExtendedTest(args[1:i])
					if err != nil {
						return false, err
					}
					// If there's more after the ), process it
					if i+1 < len(args) {
						// This shouldn't happen at this level; && and || should be at higher levels
						return false, fmt.Errorf("[[: unexpected token after )")
					}
					return result, nil
				}
			}
		}
		return false, fmt.Errorf("[[: missing closing )")
	}

	return parsePrimaryExpr(args)
}

// parsePrimaryExpr handles unary operators and binary comparisons
func parsePrimaryExpr(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	// Single argument: non-empty string is true
	if len(args) == 1 {
		return args[0] != "", nil
	}

	// Two arguments: unary operator
	if len(args) == 2 {
		return evaluateExtendedUnary(args[0], args[1])
	}

	// Three arguments: binary operator
	if len(args) == 3 {
		return evaluateExtendedBinary(args[0], args[1], args[2])
	}

	return false, fmt.Errorf("[[: syntax error: too many arguments")
}

// evaluateExtendedUnary evaluates unary test operators for [[
// Uses the same operators as [ but with consistent behavior
func evaluateExtendedUnary(op, arg string) (bool, error) {
	switch op {
	// File tests
	case "-e":
		_, err := os.Stat(arg)
		return err == nil, nil

	case "-f":
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode().IsRegular(), nil

	case "-d":
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.IsDir(), nil

	case "-r":
		file, err := os.Open(arg)
		if err != nil {
			return false, nil
		}
		file.Close()
		return true, nil

	case "-w":
		file, err := os.OpenFile(arg, os.O_WRONLY, 0)
		if err != nil {
			return false, nil
		}
		file.Close()
		return true, nil

	case "-x":
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		mode := info.Mode()
		return mode&0111 != 0, nil

	case "-s":
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Size() > 0, nil

	case "-L", "-h":
		info, err := os.Lstat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeSymlink != 0, nil

	case "-b":
		// Block special file
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeDevice != 0 && info.Mode()&os.ModeCharDevice == 0, nil

	case "-c":
		// Character special file
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeCharDevice != 0, nil

	case "-p":
		// Named pipe (FIFO)
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeNamedPipe != 0, nil

	case "-S":
		// Socket
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeSocket != 0, nil

	case "-g":
		// Set-group-ID bit
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeSetgid != 0, nil

	case "-u":
		// Set-user-ID bit
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeSetuid != 0, nil

	case "-k":
		// Sticky bit
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeSticky != 0, nil

	case "-O":
		// Owned by effective UID
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		// Note: This is a simplified check; proper implementation would use syscall
		_ = info
		return true, nil // Placeholder

	case "-G":
		// Owned by effective GID
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		_ = info
		return true, nil // Placeholder

	case "-N":
		// Modified since last read
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		_ = info
		return true, nil // Placeholder

	// String tests
	case "-z":
		return arg == "", nil

	case "-n":
		return arg != "", nil

	case "-v":
		// Variable is set (check environment)
		_, exists := os.LookupEnv(arg)
		return exists, nil

	default:
		return false, fmt.Errorf("[[: unknown unary operator: %s", op)
	}
}

// evaluateExtendedBinary evaluates binary test operators for [[
// This includes pattern matching (==), regex matching (=~), and comparisons
func evaluateExtendedBinary(left, op, right string) (bool, error) {
	switch op {
	// Pattern matching (glob patterns)
	case "==":
		return matchPattern(left, right)

	case "=":
		// In [[ ]], = is the same as ==
		return matchPattern(left, right)

	case "!=":
		matched, err := matchPattern(left, right)
		return !matched, err

	// Regex matching
	case "=~":
		return matchRegex(left, right)

	// Lexicographic comparison
	case "<":
		return left < right, nil

	case ">":
		return left > right, nil

	// Numeric comparison
	case "-eq":
		return compareNumericExt(left, right, func(a, b int) bool { return a == b })

	case "-ne":
		return compareNumericExt(left, right, func(a, b int) bool { return a != b })

	case "-lt":
		return compareNumericExt(left, right, func(a, b int) bool { return a < b })

	case "-le":
		return compareNumericExt(left, right, func(a, b int) bool { return a <= b })

	case "-gt":
		return compareNumericExt(left, right, func(a, b int) bool { return a > b })

	case "-ge":
		return compareNumericExt(left, right, func(a, b int) bool { return a >= b })

	// File comparisons
	case "-nt":
		// Left is newer than right
		return fileNewerThan(left, right)

	case "-ot":
		// Left is older than right
		newer, err := fileNewerThan(left, right)
		return !newer, err

	case "-ef":
		// Same file (same device and inode)
		return sameFile(left, right)

	default:
		return false, fmt.Errorf("[[: unknown binary operator: %s", op)
	}
}

// matchPattern matches a string against a glob pattern
// In [[ ]], the right side of == is treated as a pattern unless quoted
func matchPattern(str, pattern string) (bool, error) {
	// Convert glob pattern to regex
	// Handle *, ?, and [...] character classes
	regexPattern := globToRegex(pattern)

	re, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		return false, fmt.Errorf("[[: invalid pattern: %s", pattern)
	}

	return re.MatchString(str), nil
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(pattern string) string {
	var result strings.Builder

	inCharClass := false
	i := 0
	for i < len(pattern) {
		c := pattern[i]

		switch {
		case c == '[' && !inCharClass:
			inCharClass = true
			result.WriteByte('[')
			// Handle negation: [! or [^
			if i+1 < len(pattern) && (pattern[i+1] == '!' || pattern[i+1] == '^') {
				result.WriteByte('^')
				i++
			}
		case c == ']' && inCharClass:
			inCharClass = false
			result.WriteByte(']')
		case inCharClass:
			// Inside character class, most chars are literal
			if c == '\\' && i+1 < len(pattern) {
				result.WriteByte('\\')
				i++
				result.WriteByte(pattern[i])
			} else {
				result.WriteByte(c)
			}
		case c == '*':
			result.WriteString(".*")
		case c == '?':
			result.WriteByte('.')
		case c == '\\' && i+1 < len(pattern):
			// Escape next character
			result.WriteByte('\\')
			i++
			result.WriteByte(pattern[i])
		case isRegexMeta(c):
			// Escape regex metacharacters
			result.WriteByte('\\')
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
		i++
	}

	return result.String()
}

// isRegexMeta returns true if c is a regex metacharacter (excluding glob chars we handle)
func isRegexMeta(c byte) bool {
	return c == '.' || c == '+' || c == '^' || c == '$' || c == '(' || c == ')' ||
		c == '{' || c == '}' || c == '|'
}

// matchRegex matches a string against a regular expression
// Sets BASH_REMATCH array on successful match
func matchRegex(str, pattern string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("[[: invalid regex: %s", pattern)
	}

	matches := re.FindStringSubmatch(str)
	if matches == nil {
		// Clear BASH_REMATCH on no match
		gs := GetGlobalState()
		gs.SetEnvVar("BASH_REMATCH", "")
		return false, nil
	}

	// Set BASH_REMATCH array
	// BASH_REMATCH[0] is the entire match
	// BASH_REMATCH[1], [2], etc. are capture groups
	gs := GetGlobalState()

	// Set BASH_REMATCH[0] to the full match
	gs.SetEnvVar("BASH_REMATCH", matches[0])

	// Set individual capture groups as BASH_REMATCH_1, BASH_REMATCH_2, etc.
	// Note: Bash uses arrays, but we'll use indexed variables for now
	for i, match := range matches {
		gs.SetEnvVar(fmt.Sprintf("BASH_REMATCH_%d", i), match)
	}

	return true, nil
}

// compareNumericExt compares two strings as integers for [[ ]]
func compareNumericExt(left, right string, cmp func(int, int) bool) (bool, error) {
	leftNum, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil {
		return false, fmt.Errorf("[[: integer expression expected: %s", left)
	}

	rightNum, err := strconv.Atoi(strings.TrimSpace(right))
	if err != nil {
		return false, fmt.Errorf("[[: integer expression expected: %s", right)
	}

	return cmp(leftNum, rightNum), nil
}

// fileNewerThan returns true if file1 is newer than file2
func fileNewerThan(file1, file2 string) (bool, error) {
	info1, err := os.Stat(file1)
	if err != nil {
		return false, nil
	}

	info2, err := os.Stat(file2)
	if err != nil {
		return false, nil
	}

	return info1.ModTime().After(info2.ModTime()), nil
}

// sameFile returns true if two paths refer to the same file
func sameFile(file1, file2 string) (bool, error) {
	// Resolve symlinks and get absolute paths
	abs1, err := filepath.EvalSymlinks(file1)
	if err != nil {
		return false, nil
	}

	abs2, err := filepath.EvalSymlinks(file2)
	if err != nil {
		return false, nil
	}

	// On Unix, we'd compare device and inode, but filepath comparison works for most cases
	info1, err := os.Stat(abs1)
	if err != nil {
		return false, nil
	}

	info2, err := os.Stat(abs2)
	if err != nil {
		return false, nil
	}

	return os.SameFile(info1, info2), nil
}

// stripQuotes removes surrounding single or double quotes from a string
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
