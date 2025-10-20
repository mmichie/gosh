package gosh

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// testCommand implements the POSIX test command and [ builtin
// It evaluates conditional expressions and returns exit status 0 (true) or 1 (false)
func testCommand(cmd *Command) error {
	// Extract arguments from the command
	args := extractTestArgs(cmd)

	// Handle the [ command - must end with ]
	if len(args) > 0 && args[0] == "[" {
		args = args[1:] // Remove the leading [
		if len(args) == 0 || args[len(args)-1] != "]" {
			cmd.ReturnCode = 2 // Syntax error
			return fmt.Errorf("[: missing closing ]")
		}
		args = args[:len(args)-1] // Remove the trailing ]
	}

	// Empty test is false
	if len(args) == 0 {
		cmd.ReturnCode = 1
		return nil
	}

	// Evaluate the expression
	result, err := evaluateTest(args)
	if err != nil {
		cmd.ReturnCode = 2 // Syntax error
		return err
	}

	if result {
		cmd.ReturnCode = 0
	} else {
		cmd.ReturnCode = 1
	}

	return nil
}

// extractTestArgs extracts arguments from a Command structure
func extractTestArgs(cmd *Command) []string {
	if len(cmd.Command.LogicalBlocks) == 0 ||
		cmd.Command.LogicalBlocks[0].FirstPipeline == nil ||
		len(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands) == 0 {
		return []string{}
	}

	parts := cmd.Command.LogicalBlocks[0].FirstPipeline.Commands[0].Parts
	if len(parts) == 0 {
		return []string{}
	}

	// For [ command, we need to keep it to detect and validate closing ]
	// For test command, we skip the command name
	if len(parts) > 0 && parts[0] == "[" {
		return parts // Keep [ for validation
	}

	// For test command, skip the command name
	if len(parts) > 1 {
		return parts[1:]
	}

	return []string{}
}

// evaluateTest evaluates a test expression
func evaluateTest(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	// Handle single argument - non-empty string is true
	if len(args) == 1 {
		return args[0] != "", nil
	}

	// Handle negation (!)
	if args[0] == "!" {
		result, err := evaluateTest(args[1:])
		return !result, err
	}

	// Handle two arguments (unary operators)
	if len(args) == 2 {
		return evaluateUnary(args[0], args[1])
	}

	// Handle three or more arguments
	if len(args) >= 3 {
		// Check for binary operators in the middle
		return evaluateBinaryAndLogical(args)
	}

	return false, fmt.Errorf("test: invalid number of arguments")
}

// evaluateUnary evaluates unary test operators
func evaluateUnary(op, arg string) (bool, error) {
	switch op {
	// File tests
	case "-e":
		// File exists
		_, err := os.Stat(arg)
		return err == nil, nil

	case "-f":
		// Is a regular file
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode().IsRegular(), nil

	case "-d":
		// Is a directory
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.IsDir(), nil

	case "-r":
		// Is readable
		file, err := os.Open(arg)
		if err != nil {
			return false, nil
		}
		file.Close()
		return true, nil

	case "-w":
		// Is writable
		file, err := os.OpenFile(arg, os.O_WRONLY, 0)
		if err != nil {
			return false, nil
		}
		file.Close()
		return true, nil

	case "-x":
		// Is executable
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		mode := info.Mode()
		return mode&0111 != 0, nil

	case "-s":
		// File exists and size > 0
		info, err := os.Stat(arg)
		if err != nil {
			return false, nil
		}
		return info.Size() > 0, nil

	case "-L", "-h":
		// Is a symbolic link
		info, err := os.Lstat(arg)
		if err != nil {
			return false, nil
		}
		return info.Mode()&os.ModeSymlink != 0, nil

	// String tests
	case "-z":
		// String is empty
		return arg == "", nil

	case "-n":
		// String is not empty
		return arg != "", nil

	default:
		return false, fmt.Errorf("test: unknown unary operator: %s", op)
	}
}

// evaluateBinaryAndLogical evaluates binary operators and logical combinations
func evaluateBinaryAndLogical(args []string) (bool, error) {
	// Look for logical operators -a (AND) and -o (OR)
	// Process -o first (lower precedence)
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "-o" {
			left, err := evaluateTest(args[:i])
			if err != nil {
				return false, err
			}
			right, err := evaluateTest(args[i+1:])
			if err != nil {
				return false, err
			}
			return left || right, nil
		}
	}

	// Process -a (higher precedence)
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "-a" {
			left, err := evaluateTest(args[:i])
			if err != nil {
				return false, err
			}
			right, err := evaluateTest(args[i+1:])
			if err != nil {
				return false, err
			}
			return left && right, nil
		}
	}

	// Handle binary operators (exactly 3 arguments)
	if len(args) == 3 {
		return evaluateBinary(args[0], args[1], args[2])
	}

	// Handle parentheses (not implemented yet, but documented)
	if args[0] == "(" && args[len(args)-1] == ")" {
		return evaluateTest(args[1 : len(args)-1])
	}

	return false, fmt.Errorf("test: invalid expression")
}

// evaluateBinary evaluates binary test operators
func evaluateBinary(left, op, right string) (bool, error) {
	switch op {
	// String comparison
	case "=", "==":
		return left == right, nil

	case "!=":
		return left != right, nil

	case "<":
		return left < right, nil

	case ">":
		return left > right, nil

	// Numeric comparison
	case "-eq":
		return compareNumeric(left, right, func(a, b int) bool { return a == b })

	case "-ne":
		return compareNumeric(left, right, func(a, b int) bool { return a != b })

	case "-lt":
		return compareNumeric(left, right, func(a, b int) bool { return a < b })

	case "-le":
		return compareNumeric(left, right, func(a, b int) bool { return a <= b })

	case "-gt":
		return compareNumeric(left, right, func(a, b int) bool { return a > b })

	case "-ge":
		return compareNumeric(left, right, func(a, b int) bool { return a >= b })

	default:
		return false, fmt.Errorf("test: unknown binary operator: %s", op)
	}
}

// compareNumeric compares two strings as integers
func compareNumeric(left, right string, cmp func(int, int) bool) (bool, error) {
	leftNum, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil {
		return false, fmt.Errorf("test: integer expression expected: %s", left)
	}

	rightNum, err := strconv.Atoi(strings.TrimSpace(right))
	if err != nil {
		return false, fmt.Errorf("test: integer expression expected: %s", right)
	}

	return cmp(leftNum, rightNum), nil
}
