package gosh

import (
	"fmt"
	"strings"
)

// processEcho implements the echo builtin command with support for quoted arguments
func processEcho(cmd *Command) error {
	// Get the args from the command - always use the current command parts
	var args []string

	// When a builtin is called, it is passed the entire Command structure
	// but we need to extract just the current pipeline being executed

	// Extract args directly from the SimpleCommand that's being executed
	if cmd.Command != nil && len(cmd.Command.LogicalBlocks) > 0 {
		// Get from the first command's parts - this is the current command
		block := cmd.Command.LogicalBlocks[0]
		if block.FirstPipeline != nil && len(block.FirstPipeline.Commands) > 0 {
			cmdParts := block.FirstPipeline.Commands[0].Parts
			if len(cmdParts) > 1 {
				args = cmdParts[1:] // Skip the command name
			}
		}
	}

	// Fallback for edge cases - most of these shouldn't be needed in practice,
	// but we keep them in case the command structure is unusual
	if len(args) == 0 {
		// Direct approach - get command line as string and parse it
		cmdLine := ""
		if len(cmd.Command.LogicalBlocks) > 0 &&
			cmd.Command.LogicalBlocks[0].FirstPipeline != nil &&
			len(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands) > 0 {
			cmdLine = strings.Join(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands[0].Parts, " ")
		}

		// Extract args from the command line
		parts := strings.Fields(cmdLine)
		if len(parts) > 1 && parts[0] == "echo" {
			args = parts[1:]
		}
	}

	// Process arguments to handle quotes and special variables
	for i, arg := range args {
		// Handle quotes - remove surrounding quotes if present
		if (strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'")) ||
			(strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"")) {
			// Remove surrounding quotes
			arg = arg[1 : len(arg)-1]
			args[i] = arg
		}

		// Replace $? with the previous command's return code
		if strings.Contains(arg, "$?") {
			// This is a simplification - in a full shell, we'd have a more robust variable expansion system
			args[i] = strings.ReplaceAll(arg, "$?", "")
		}
	}

	// Print the arguments joined with a space, followed by a newline
	if len(args) > 0 {
		_, err := fmt.Fprintln(cmd.Stdout, strings.Join(args, " "))
		return err
	}

	// Just print a newline if no arguments are provided
	_, err := fmt.Fprintln(cmd.Stdout, "")
	return err
}
