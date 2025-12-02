package gosh

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// colonCommand implements the : (null) command
// The : command does nothing and returns success
// It's useful in conditional expressions and as a placeholder
func colonCommand(cmd *Command) error {
	// Do nothing, just return success
	cmd.ReturnCode = 0
	return nil
}

// unsetCommand implements the unset builtin
// Usage: unset [-v|-f] name [name ...]
// -v: Treat each name as a shell variable (default)
// -f: Treat each name as a shell function
func unsetCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return nil // unset with no args does nothing
	}

	unsetFunctions := false
	varNames := []string{}

	for _, arg := range args {
		switch arg {
		case "-v":
			// Unset variables (default behavior)
			unsetFunctions = false
		case "-f":
			// Unset functions - not yet implemented
			unsetFunctions = true
		default:
			varNames = append(varNames, arg)
		}
	}

	if unsetFunctions {
		// TODO: Implement function unset when we have shell functions
		return fmt.Errorf("unset: -f: shell functions not yet implemented")
	}

	// Unset environment variables
	for _, name := range varNames {
		os.Unsetenv(name)
	}

	return nil
}

// sourceCommand implements the source/. builtin
// Usage: source filename [arguments]
// Reads and executes commands from the filename in the current shell environment
func sourceCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return fmt.Errorf("source: filename argument required")
	}

	filename := args[0]

	// Resolve the file path
	resolvedPath, err := resolveSourceFile(filename)
	if err != nil {
		return fmt.Errorf("source: %v", err)
	}

	// Read the file
	file, err := os.Open(resolvedPath)
	if err != nil {
		return fmt.Errorf("source: %v", err)
	}
	defer file.Close()

	// Save and set positional parameters if additional arguments are provided
	gs := GetGlobalState()
	oldParams := gs.GetPositionalParams()
	if len(args) > 1 {
		gs.SetPositionalParams(args[1:])
	}

	// Read and execute each line
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Execute the line using the shell's command execution
		execCmd, err := NewCommand(line, cmd.JobManager)
		if err != nil {
			gs.SetPositionalParams(oldParams)
			return fmt.Errorf("source: %s: line %d: %v", filename, lineNum, err)
		}
		execCmd.Stdin = cmd.Stdin
		execCmd.Stdout = cmd.Stdout
		execCmd.Stderr = cmd.Stderr

		execCmd.Run()

		// Check for errexit option
		if execCmd.ReturnCode != 0 && gs.GetOptions().Errexit {
			gs.SetPositionalParams(oldParams)
			cmd.ReturnCode = execCmd.ReturnCode
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		gs.SetPositionalParams(oldParams)
		return fmt.Errorf("source: error reading %s: %v", filename, err)
	}

	// Restore positional parameters
	gs.SetPositionalParams(oldParams)
	cmd.ReturnCode = 0
	return nil
}

// resolveSourceFile finds the source file, checking relative and PATH
func resolveSourceFile(filename string) (string, error) {
	// If it's an absolute path, use it directly
	if filepath.IsAbs(filename) {
		if _, err := os.Stat(filename); err != nil {
			return "", fmt.Errorf("%s: No such file or directory", filename)
		}
		return filename, nil
	}

	// Check if file exists in current directory or as relative path
	if _, err := os.Stat(filename); err == nil {
		return filename, nil
	}

	// Search in PATH for the file
	pathEnv := os.Getenv("PATH")
	if pathEnv != "" {
		paths := strings.Split(pathEnv, ":")
		for _, dir := range paths {
			fullPath := filepath.Join(dir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("%s: No such file or directory", filename)
}

// evalCommand implements the eval builtin
// Usage: eval [arg ...]
// Concatenates arguments and executes them as a shell command
func evalCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return nil // eval with no args does nothing
	}

	// Concatenate all arguments with spaces
	cmdString := strings.Join(args, " ")

	// Execute the command string
	execCmd, err := NewCommand(cmdString, cmd.JobManager)
	if err != nil {
		return fmt.Errorf("eval: %v", err)
	}
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	execCmd.Run()

	cmd.ReturnCode = execCmd.ReturnCode
	return nil
}

// getBuiltinArgs is a helper to extract arguments from a command
// Returns all arguments after the command name
func getBuiltinArgs(cmd *Command) []string {
	if cmd.Command == nil ||
		len(cmd.Command.LogicalBlocks) == 0 ||
		cmd.Command.LogicalBlocks[0].FirstPipeline == nil ||
		len(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands) == 0 {
		return nil
	}

	parts := getCommandParts(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands[0])
	if len(parts) > 1 {
		return parts[1:]
	}
	return nil
}
