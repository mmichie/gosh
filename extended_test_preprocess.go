package gosh

import (
	"strings"
	"unicode"
)

// PreprocessExtendedTest preprocesses [[ ... ]] constructs to quote shell operators
// so the parser treats them as regular words rather than shell operators.
// This function transforms:
//   [[ $a && $b ]] -> [[ $a '&&' $b ]]
//   [[ $a || $b ]] -> [[ $a '||' $b ]]
//   [[ $a < $b ]] -> [[ $a '<' $b ]]
//   [[ ( $a ) ]] -> [[ '(' $a ')' ]]
func PreprocessExtendedTest(input string) string {
	result := strings.Builder{}
	i := 0

	for i < len(input) {
		// Look for [[
		if i+1 < len(input) && input[i] == '[' && input[i+1] == '[' {
			// Find the matching ]]
			end := findMatchingCloseBracket(input, i+2)

			if end == -1 {
				// No matching ]], just copy the rest
				result.WriteString(input[i:])
				break
			}

			// Extract and process the content between [[ and ]]
			content := input[i+2 : end]
			processed := processExtendedTestContent(content)

			result.WriteString("[[")
			result.WriteString(processed)
			result.WriteString("]]")

			i = end + 2
			continue
		}

		result.WriteByte(input[i])
		i++
	}

	return result.String()
}

// findMatchingCloseBracket finds the index of ]] starting from pos
// Returns -1 if not found
func findMatchingCloseBracket(input string, pos int) int {
	inSingleQuote := false
	inDoubleQuote := false

	for i := pos; i < len(input)-1; i++ {
		c := input[i]

		// Handle quote state
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		// Look for ]] outside of quotes
		if !inSingleQuote && !inDoubleQuote {
			if c == ']' && input[i+1] == ']' {
				return i
			}
		}
	}

	return -1
}

// processExtendedTestContent quotes shell operators within [[ ]] content
func processExtendedTestContent(content string) string {
	result := strings.Builder{}
	i := 0

	for i < len(content) {
		c := content[i]

		// Skip single-quoted strings
		if c == '\'' {
			j := i + 1
			for j < len(content) && content[j] != '\'' {
				j++
			}
			if j < len(content) {
				result.WriteString(content[i : j+1])
				i = j + 1
				continue
			}
		}

		// Skip double-quoted strings
		if c == '"' {
			j := i + 1
			for j < len(content) && content[j] != '"' {
				if content[j] == '\\' && j+1 < len(content) {
					j++ // Skip escaped char
				}
				j++
			}
			if j < len(content) {
				result.WriteString(content[i : j+1])
				i = j + 1
				continue
			}
		}

		// Quote && operator
		if i+1 < len(content) && content[i] == '&' && content[i+1] == '&' {
			// Check if surrounded by whitespace or at boundaries
			if needsQuoting(content, i, 2) {
				result.WriteString("'&&'")
				i += 2
				continue
			}
		}

		// Quote || operator
		if i+1 < len(content) && content[i] == '|' && content[i+1] == '|' {
			if needsQuoting(content, i, 2) {
				result.WriteString("'||'")
				i += 2
				continue
			}
		}

		// Quote single | (pipe) - treat as literal in [[]]
		if content[i] == '|' && (i+1 >= len(content) || content[i+1] != '|') {
			if i == 0 || i+1 >= len(content) || content[i-1] == ' ' || content[i+1] == ' ' {
				result.WriteString("'|'")
				i++
				continue
			}
		}

		// Quote < operator (but not =~ which uses < in regex)
		if content[i] == '<' {
			// Check if this is part of a regex or standalone
			if needsQuoting(content, i, 1) {
				result.WriteString("'<'")
				i++
				continue
			}
		}

		// Quote > operator
		if content[i] == '>' {
			if needsQuoting(content, i, 1) {
				result.WriteString("'>'")
				i++
				continue
			}
		}

		// Quote ( and ) for grouping
		if content[i] == '(' {
			if needsQuoting(content, i, 1) {
				result.WriteString("'('")
				i++
				continue
			}
		}

		if content[i] == ')' {
			if needsQuoting(content, i, 1) {
				result.WriteString("')'")
				i++
				continue
			}
		}

		result.WriteByte(content[i])
		i++
	}

	return result.String()
}

// needsQuoting checks if an operator at position i needs quoting
// An operator needs quoting if it's surrounded by whitespace or at word boundaries
func needsQuoting(content string, i, length int) bool {
	// Check character before
	if i > 0 {
		prev := content[i-1]
		if !unicode.IsSpace(rune(prev)) && prev != '(' && prev != ')' {
			return false // Part of a word
		}
	}

	// Check character after
	end := i + length
	if end < len(content) {
		next := content[end]
		if !unicode.IsSpace(rune(next)) && next != '(' && next != ')' {
			return false // Part of a word
		}
	}

	return true
}
