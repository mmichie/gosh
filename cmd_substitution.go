package gosh

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Command substitution patterns
var (
	// Match commands in $(...) format
	dollarParenPattern = regexp.MustCompile(`\$\((.*?)\)`)

	// Match commands in backtick format
	backtickPattern = regexp.MustCompile("`([^`]*)`")
)

// PerformCommandSubstitution processes a command string for command substitutions
// It supports both $(...) and backtick ` ` syntax
// Returns the processed command string with substitutions applied
func PerformCommandSubstitution(cmdString string, jobManager *JobManager) (string, error) {
	var err error

	// First handle $(...) substitutions
	cmdString, err = substituteCommandsInPattern(cmdString, dollarParenPattern, jobManager)
	if err != nil {
		return cmdString, err
	}

	// Then handle backtick substitutions
	cmdString, err = substituteCommandsInPattern(cmdString, backtickPattern, jobManager)
	if err != nil {
		return cmdString, err
	}

	return cmdString, nil
}

// substituteCommandsInPattern finds command substitutions matching the given pattern
// and replaces them with their output
func substituteCommandsInPattern(cmdString string, pattern *regexp.Regexp, jobManager *JobManager) (string, error) {
	// Find all matching command substitutions
	for {
		// Find the first match in the current string
		match := pattern.FindStringSubmatchIndex(cmdString)
		if match == nil {
			break // No more matches
		}

		// Extract the inner command to execute
		innerCmdStart, innerCmdEnd := match[2], match[3]
		innerCmd := cmdString[innerCmdStart:innerCmdEnd]

		// Execute the inner command and capture its output
		output, err := executeSubstitutedCommand(innerCmd, jobManager)
		if err != nil {
			return cmdString, fmt.Errorf("error in command substitution '%s': %v", innerCmd, err)
		}

		// Replace the command substitution with the output
		// We need to replace the entire match, including $() or `` characters
		fullMatchStart, fullMatchEnd := match[0], match[1]
		cmdString = cmdString[:fullMatchStart] + output + cmdString[fullMatchEnd:]
	}

	return cmdString, nil
}

// executeSubstitutedCommand runs a command in a subshell and returns its output
func executeSubstitutedCommand(cmdString string, jobManager *JobManager) (string, error) {
	// Create a buffer to capture command output
	var outputBuffer bytes.Buffer

	// To prevent infinite recursion, we need to parse and execute the command directly
	// without calling NewCommand which would trigger more command substitution
	parsedCmd, err := parser.Parse(cmdString)
	if err != nil {
		return "", err
	}

	// Create a command with the parsed command
	cmd := &Command{
		Command:    parsedCmd,
		Stdin:      strings.NewReader(""),
		Stdout:     &outputBuffer,
		Stderr:     &outputBuffer,
		JobManager: jobManager,
	}

	// Run the command
	cmd.Run()

	// Check if the command was successful
	if cmd.ReturnCode != 0 {
		return "", fmt.Errorf("command returned non-zero exit code: %d", cmd.ReturnCode)
	}

	// Get the output and clean it up (remove trailing newlines)
	output := strings.TrimRight(outputBuffer.String(), "\n")

	return output, nil
}
