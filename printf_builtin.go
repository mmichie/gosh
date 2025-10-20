package gosh

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// printfCommand implements the printf builtin command
// Supports C-style format strings with format specifiers and escape sequences
func printfCommand(cmd *Command) error {
	args := extractCommandArgs(cmd, "printf")

	if len(args) == 0 {
		return fmt.Errorf("printf: usage: printf format [arguments]")
	}

	// Remove quotes from arguments (parser preserves them)
	for i := range args {
		args[i] = removeQuotes(args[i])
	}

	format := args[0]
	values := args[1:]

	// Process escape sequences in the format string
	format = processEscapeSequences(format)

	// Find all format specifiers in the format string
	specs := findFormatSpecifiers(format)

	if len(specs) == 0 {
		// No format specifiers, just print the format string
		fmt.Fprint(cmd.Stdout, format)
		cmd.ReturnCode = 0
		return nil
	}

	// If we have format specifiers but no values, use defaults
	if len(values) == 0 {
		output := format
		for _, spec := range specs {
			output = strings.Replace(output, spec.full, getDefaultValue(spec.specifier), 1)
		}
		fmt.Fprint(cmd.Stdout, output)
		cmd.ReturnCode = 0
		return nil
	}

	// Process values against format specifiers
	// If there are more values than specifiers, repeat the format
	output := strings.Builder{}
	valueIndex := 0

	for valueIndex < len(values) {
		currentFormat := format
		for _, spec := range specs {
			if valueIndex >= len(values) {
				// No more values, use default
				currentFormat = strings.Replace(currentFormat, spec.full, getDefaultValue(spec.specifier), 1)
			} else {
				// Format the current value
				formatted, err := formatValue(values[valueIndex], spec)
				if err != nil {
					return err
				}
				currentFormat = strings.Replace(currentFormat, spec.full, formatted, 1)
				valueIndex++
			}
		}
		output.WriteString(currentFormat)

		// If we still have values left, we'll repeat the format
		// but only if we consumed at least one value in this iteration
		if valueIndex < len(values) && len(specs) == 0 {
			break
		}
	}

	fmt.Fprint(cmd.Stdout, output.String())
	cmd.ReturnCode = 0
	return nil
}

// formatSpec represents a parsed format specifier
type formatSpec struct {
	full      string // The full format specifier (e.g., "%5.2f")
	flags     string // Flags (-, +, space, 0, #)
	width     int    // Field width
	precision int    // Precision
	hasPrecision bool
	specifier string // The type specifier (s, d, f, x, o, c, %)
}

// findFormatSpecifiers finds all format specifiers in the format string
func findFormatSpecifiers(format string) []formatSpec {
	// Regex to match printf format specifiers
	// %[flags][width][.precision]specifier
	re := regexp.MustCompile(`%([#0 +-]*)(\d*)(?:\.(\d+))?([sdifxXocb%])`)
	matches := re.FindAllStringSubmatch(format, -1)

	specs := make([]formatSpec, 0, len(matches))
	for _, match := range matches {
		spec := formatSpec{
			full:      match[0],
			flags:     match[1],
			specifier: match[4],
		}

		if match[2] != "" {
			spec.width, _ = strconv.Atoi(match[2])
		}

		if match[3] != "" {
			spec.precision, _ = strconv.Atoi(match[3])
			spec.hasPrecision = true
		}

		specs = append(specs, spec)
	}

	return specs
}

// formatValue formats a value according to the format specifier
func formatValue(value string, spec formatSpec) (string, error) {
	switch spec.specifier {
	case "%":
		return "%", nil

	case "s":
		// String format
		result := value
		if spec.hasPrecision && spec.precision < len(value) {
			result = value[:spec.precision]
		}
		if spec.width > 0 {
			if strings.Contains(spec.flags, "-") {
				// Left-justify
				result = fmt.Sprintf("%-*s", spec.width, result)
			} else {
				// Right-justify (default)
				result = fmt.Sprintf("%*s", spec.width, result)
			}
		}
		return result, nil

	case "d", "i":
		// Decimal integer
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			// If parsing fails, treat as 0 (bash behavior)
			num = 0
		}
		formatStr := buildNumericFormat(spec, 'd')
		return fmt.Sprintf(formatStr, num), nil

	case "f":
		// Floating point
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			num = 0.0
		}
		formatStr := buildNumericFormat(spec, 'f')
		return fmt.Sprintf(formatStr, num), nil

	case "x":
		// Hexadecimal (lowercase)
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			num = 0
		}
		formatStr := buildNumericFormat(spec, 'x')
		return fmt.Sprintf(formatStr, num), nil

	case "X":
		// Hexadecimal (uppercase)
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			num = 0
		}
		formatStr := buildNumericFormat(spec, 'X')
		return fmt.Sprintf(formatStr, num), nil

	case "o":
		// Octal
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			num = 0
		}
		formatStr := buildNumericFormat(spec, 'o')
		return fmt.Sprintf(formatStr, num), nil

	case "c":
		// Character
		if len(value) == 0 {
			return "", nil
		}
		// If it's a number, treat it as ASCII code
		if num, err := strconv.Atoi(value); err == nil {
			if num >= 0 && num <= 127 {
				return string(rune(num)), nil
			}
		}
		// Otherwise, use first character
		return string(value[0]), nil

	case "b":
		// Binary (non-standard but useful)
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			num = 0
		}
		return fmt.Sprintf("%b", num), nil

	default:
		return value, nil
	}
}

// buildNumericFormat builds a format string for numeric types
func buildNumericFormat(spec formatSpec, baseSpec rune) string {
	var format strings.Builder
	format.WriteRune('%')

	// Add flags
	if spec.flags != "" {
		format.WriteString(spec.flags)
	}

	// Add width
	if spec.width > 0 {
		format.WriteString(strconv.Itoa(spec.width))
	}

	// Add precision
	if spec.hasPrecision {
		format.WriteRune('.')
		format.WriteString(strconv.Itoa(spec.precision))
	} else if baseSpec == 'f' {
		// Default precision for float is 6
		format.WriteString(".6")
	}

	// Add the base specifier
	format.WriteRune(baseSpec)

	return format.String()
}

// getDefaultValue returns the default value for a format specifier
func getDefaultValue(specifier string) string {
	switch specifier {
	case "s", "c":
		return ""
	case "d", "i", "x", "X", "o", "b":
		return "0"
	case "f":
		return fmt.Sprintf("%.6f", 0.0)
	case "%":
		return "%"
	default:
		return ""
	}
}

// processEscapeSequences processes escape sequences in a string
func processEscapeSequences(s string) string {
	result := strings.Builder{}
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteRune('\n')
				i += 2
			case 't':
				result.WriteRune('\t')
				i += 2
			case 'r':
				result.WriteRune('\r')
				i += 2
			case 'b':
				result.WriteRune('\b')
				i += 2
			case 'a':
				result.WriteRune('\a')
				i += 2
			case 'f':
				result.WriteRune('\f')
				i += 2
			case 'v':
				result.WriteRune('\v')
				i += 2
			case '\\':
				result.WriteRune('\\')
				i += 2
			case '"':
				result.WriteRune('"')
				i += 2
			case '\'':
				result.WriteRune('\'')
				i += 2
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// Octal escape sequence
				octal := ""
				j := i + 1
				for j < len(s) && j < i+4 && s[j] >= '0' && s[j] <= '7' {
					octal += string(s[j])
					j++
				}
				if num, err := strconv.ParseInt(octal, 8, 32); err == nil {
					result.WriteRune(rune(num))
				}
				i = j
			case 'x':
				// Hexadecimal escape sequence
				if i+3 < len(s) {
					hex := s[i+2 : i+4]
					if num, err := strconv.ParseInt(hex, 16, 32); err == nil {
						result.WriteRune(rune(num))
						i += 4
						continue
					}
				}
				result.WriteRune('\\')
				i++
			default:
				// Unknown escape, keep the backslash
				result.WriteRune('\\')
				i++
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// Helper function to check if a value is NaN or Inf
func isInvalidFloat(f float64) bool {
	return math.IsNaN(f) || math.IsInf(f, 0)
}

// removeQuotes removes surrounding quotes from a string
func removeQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
