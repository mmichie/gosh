package gosh

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
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
	gs := GetGlobalState()
	for _, name := range varNames {
		if err := gs.UnsetEnvVar(name); err != nil {
			return fmt.Errorf("unset: %v", err)
		}
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

// execCommand implements the exec builtin
// Usage: exec [-c] [-l] [-a name] [command [arguments]]
// Replaces the shell with command without creating a new process
// If no command is given, any redirections take effect in the current shell
func execCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	var clearEnv bool
	var loginShell bool
	var argv0 string
	commandArgs := []string{}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "-c" {
			clearEnv = true
			i++
		} else if arg == "-l" {
			loginShell = true
			i++
		} else if arg == "-a" {
			i++
			if i >= len(args) {
				return fmt.Errorf("exec: -a: option requires an argument")
			}
			argv0 = args[i]
			i++
		} else if arg == "--" {
			i++
			commandArgs = args[i:]
			break
		} else if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("exec: %s: invalid option", arg)
		} else {
			commandArgs = args[i:]
			break
		}
	}

	// If no command is given, exec only handles redirections
	// (redirections are handled by the command execution layer)
	if len(commandArgs) == 0 {
		return nil
	}

	// Find the command in PATH
	commandPath, err := lookupCommand(commandArgs[0])
	if err != nil {
		return fmt.Errorf("exec: %s: %v", commandArgs[0], err)
	}

	// Prepare argv0 (the name the command sees as $0)
	if argv0 == "" {
		argv0 = commandArgs[0]
	}
	if loginShell && !strings.HasPrefix(argv0, "-") {
		argv0 = "-" + argv0
	}

	// Prepare environment
	var environ []string
	if clearEnv {
		environ = []string{}
	} else {
		environ = os.Environ()
	}

	// Prepare the full argv (argv0 followed by other arguments)
	argv := append([]string{argv0}, commandArgs[1:]...)

	// Replace the current process with the new command
	// This never returns on success
	return syscall.Exec(commandPath, argv, environ)
}

// lookupCommand finds the full path of a command
func lookupCommand(name string) (string, error) {
	// If it contains a slash, use it directly
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err != nil {
			return "", fmt.Errorf("no such file or directory")
		}
		return filepath.Abs(name)
	}

	// Search in PATH
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("not found")
	}

	paths := strings.Split(pathEnv, ":")
	for _, dir := range paths {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil {
			// Check if it's executable
			if info.Mode()&0111 != 0 {
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("not found")
}

// readonlyCommand implements the readonly builtin
// Usage: readonly [-p] [name[=value] ...]
// Marks variables as readonly, preventing their modification or unsetting
func readonlyCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	gs := GetGlobalState()

	// Parse options
	printMode := false
	varSpecs := []string{}

	for _, arg := range args {
		if arg == "-p" {
			printMode = true
		} else if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("readonly: %s: invalid option", arg)
		} else {
			varSpecs = append(varSpecs, arg)
		}
	}

	// If -p or no arguments, print readonly variables
	if printMode || len(varSpecs) == 0 {
		return printReadonlyVars(cmd, gs)
	}

	// Process each variable specification
	for _, spec := range varSpecs {
		// Check if it's an assignment (name=value)
		if idx := strings.Index(spec, "="); idx != -1 {
			name := spec[:idx]
			value := spec[idx+1:]

			// Check if already readonly before setting
			if gs.IsReadonly(name) {
				return fmt.Errorf("readonly: %s: readonly variable", name)
			}

			// Set the variable value
			os.Setenv(name, value)

			// Mark as readonly
			gs.SetReadonly(name)
		} else {
			// Just mark existing variable as readonly
			name := spec

			// Mark as readonly (even if variable doesn't exist, like bash)
			gs.SetReadonly(name)
		}
	}

	return nil
}

// printReadonlyVars prints all readonly variables in a reusable format
func printReadonlyVars(cmd *Command, gs *GlobalState) error {
	readonlyVars := gs.GetReadonlyVars()

	// Sort for consistent output
	sort.Strings(readonlyVars)

	for _, name := range readonlyVars {
		value := os.Getenv(name)
		if value != "" {
			fmt.Fprintf(cmd.Stdout, "declare -r %s=%q\n", name, value)
		} else {
			fmt.Fprintf(cmd.Stdout, "declare -r %s\n", name)
		}
	}

	return nil
}
