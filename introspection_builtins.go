package gosh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// typeCommand implements the type builtin command
// Shows the type of each command (builtin, alias, file, or not found)
func typeCommand(cmd *Command) error {
	args := extractCommandArgs(cmd, "type")
	if len(args) == 0 {
		return fmt.Errorf("type: usage: type name [name ...]")
	}

	// Track if any command was not found
	allFound := true

	for _, name := range args {
		found := false

		// Check if it's a builtin
		if _, isBuiltin := builtins[name]; isBuiltin {
			fmt.Fprintf(cmd.Stdout, "%s is a shell builtin\n", name)
			found = true
			continue
		}

		// Check if it's an alias
		if aliasCmd := GetAlias(name); aliasCmd != "" {
			fmt.Fprintf(cmd.Stdout, "%s is aliased to `%s'\n", name, aliasCmd)
			found = true
			continue
		}

		// Check if it's an executable in PATH
		path, err := exec.LookPath(name)
		if err == nil {
			// Get absolute path
			absPath, err := filepath.Abs(path)
			if err == nil {
				path = absPath
			}
			fmt.Fprintf(cmd.Stdout, "%s is %s\n", name, path)
			found = true
			continue
		}

		// Not found
		if !found {
			fmt.Fprintf(cmd.Stderr, "type: %s: not found\n", name)
			allFound = false
		}
	}

	if !allFound {
		cmd.ReturnCode = 1
	} else {
		cmd.ReturnCode = 0
	}
	return nil
}

// whichCommand implements the which builtin command
// Shows the full path to executables
func whichCommand(cmd *Command) error {
	args := extractCommandArgs(cmd, "which")
	if len(args) == 0 {
		return fmt.Errorf("which: usage: which name [name ...]")
	}

	// Track if any command was not found
	allFound := true

	for _, name := range args {
		// Check if it's an executable in PATH
		path, err := exec.LookPath(name)
		if err == nil {
			// Get absolute path
			absPath, err := filepath.Abs(path)
			if err == nil {
				path = absPath
			}
			fmt.Fprintf(cmd.Stdout, "%s\n", path)
		} else {
			// Not found - which typically doesn't print error messages to stderr,
			// it just doesn't print anything and returns non-zero
			allFound = false
		}
	}

	if !allFound {
		cmd.ReturnCode = 1
	} else {
		cmd.ReturnCode = 0
	}
	return nil
}

// commandCommand implements the command builtin
// With -v flag, it acts like 'type -P' showing the path to executables
func commandCommand(cmd *Command) error {
	args := extractCommandArgs(cmd, "command")

	// Parse flags
	vFlag := false
	var cmdArgs []string

	for _, arg := range args {
		if arg == "-v" {
			vFlag = true
		} else if arg == "-V" {
			// -V is verbose, similar to type
			vFlag = true
		} else if !strings.HasPrefix(arg, "-") {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("command: usage: command [-v] name [name ...]")
	}

	if vFlag {
		// Act like type, but only show the path for executables
		allFound := true
		for _, name := range cmdArgs {
			found := false

			// Check if it's a builtin
			if _, isBuiltin := builtins[name]; isBuiltin {
				fmt.Fprintf(cmd.Stdout, "%s\n", name)
				found = true
				continue
			}

			// Check if it's an alias (command -v shows aliases too)
			if aliasCmd := GetAlias(name); aliasCmd != "" {
				fmt.Fprintf(cmd.Stdout, "alias %s='%s'\n", name, aliasCmd)
				found = true
				continue
			}

			// Check if it's an executable in PATH
			path, err := exec.LookPath(name)
			if err == nil {
				absPath, err := filepath.Abs(path)
				if err == nil {
					path = absPath
				}
				fmt.Fprintf(cmd.Stdout, "%s\n", path)
				found = true
				continue
			}

			if !found {
				allFound = false
			}
		}

		if !allFound {
			cmd.ReturnCode = 1
		} else {
			cmd.ReturnCode = 0
		}
		return nil
	}

	// Without -v, execute the command bypassing aliases
	// This is a simplified implementation - we'd need to actually execute the command
	// For now, just return an error indicating it's not fully implemented
	return fmt.Errorf("command: execution mode not yet implemented (use -v flag)")
}

// findInPath searches for an executable in PATH
// Returns the full path if found, empty string otherwise
func findInPath(name string) string {
	// Check if it's already an absolute or relative path
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err == nil {
			absPath, err := filepath.Abs(name)
			if err == nil {
				return absPath
			}
			return name
		}
		return ""
	}

	// Search in PATH
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err == nil {
		return absPath
	}
	return path
}
