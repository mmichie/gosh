package gosh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"gosh/m28adapter"
	"gosh/parser"
)

// executePipelineImproved is an enhanced version of executePipeline
// that properly handles logical operators and command separators
func (cmd *Command) executePipelineImproved(pipeline *parser.Pipeline) bool {
	var outputFile *os.File
	var inputFile *os.File
	lastOutput := cmd.Stdin

	// If there's only one command, check for simple redirection
	if len(pipeline.Commands) == 1 {
		simpleCmd := pipeline.Commands[0]

		// Check if the command is an M28 Lisp expression first
		cmdString := strings.Join(simpleCmd.Parts, " ")
		if m28adapter.IsLispExpression(cmdString) {
			result, err := m28Interpreter.Execute(cmdString)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "M28 error in '%s': %v\n", cmdString, err)
				cmd.ReturnCode = 1
				return false
			}
			fmt.Fprintln(cmd.Stdout, result)
			cmd.ReturnCode = 0
			return true
		}

		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename := parser.ProcessCommand(simpleCmd)

		// Handle input redirection
		if inputRedirectType == "<" && inputFilename != "" {
			var err error
			inputFile, err = cmd.setupInputRedirection(inputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error opening input file: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			defer func() {
				if inputFile != nil {
					inputFile.Close()
				}
			}()
			lastOutput = inputFile
		}

		// Handle output redirection
		var originalStdout io.Writer = cmd.Stdout
		if outputRedirectType != "" && outputFilename != "" {
			var err error
			outputFile, err = cmd.setupOutputRedirection(outputRedirectType, outputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error setting up output redirection: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}

			// Use the output file as stdout, but remember to close it at the end
			cmd.Stdout = outputFile
		}

		// Execute the command
		var handled bool
		if builtin, ok := builtins[cmdName]; ok {
			// Handle builtin commands with a properly scoped command
			// Create a temporary command that only contains this single command
			singleCmd := &parser.Command{
				LogicalBlocks: []*parser.LogicalBlock{
					{
						FirstPipeline: &parser.Pipeline{
							Commands: []*parser.SimpleCommand{simpleCmd},
						},
					},
				},
			}

			tmpCmd := &Command{
				Command: singleCmd, // Use the scoped command
				Stdin:   lastOutput,
				Stdout:  cmd.Stdout,
				Stderr:  cmd.Stderr,
			}

			err := builtin(tmpCmd)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "%s: %v\n", cmdName, err)
				cmd.ReturnCode = 1
			} else {
				// If the builtin explicitly set a return code (e.g., false command), use it
				if tmpCmd.ReturnCode != 0 {
					cmd.ReturnCode = tmpCmd.ReturnCode
				} else {
					cmd.ReturnCode = 0
				}
			}
			handled = true
		} else {
			// Handle external command
			execCmd := exec.Command(cmdName, args...)
			gs := GetGlobalState()
			execCmd.Dir = gs.GetCWD()
			execCmd.Stdin = lastOutput
			execCmd.Stdout = cmd.Stdout
			execCmd.Stderr = cmd.Stderr

			err := execCmd.Run()
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error executing command: %v\n", err)
				cmd.ReturnCode = 1
			} else {
				cmd.ReturnCode = 0
			}
			handled = true
		}

		// Restore original stdout if changed
		if outputFile != nil {
			cmd.Stdout = originalStdout
			outputFile.Close()
		}

		if handled {
			return cmd.ReturnCode == 0
		}
	}

	// Handle multi-command pipelines (similar to the original executePipeline)
	// (This could be enhanced in the future, but it's not as critical for the
	// logical operator handling which is the current focus)

	// For now, delegate to the original implementation for multi-command pipelines
	return cmd.executePipeline(pipeline)
}
