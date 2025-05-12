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
				Command:    singleCmd, // Use the scoped command
				Stdin:      lastOutput,
				Stdout:     cmd.Stdout,
				Stderr:     cmd.Stderr,
				JobManager: cmd.JobManager, // Pass the JobManager to ensure builtins can access it
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

			// Check if the command should run in the background
			if simpleCmd.Background {
				// Start the command but don't wait for it to complete
				err := execCmd.Start()
				if err != nil {
					fmt.Fprintf(cmd.Stderr, "Error starting background command: %v\n", err)
					cmd.ReturnCode = 1
				} else {
					// Add the command to the job manager
					job := cmd.JobManager.AddJob(parser.FormatCommand(&parser.Command{
						LogicalBlocks: []*parser.LogicalBlock{
							{
								FirstPipeline: &parser.Pipeline{
									Commands: []*parser.SimpleCommand{simpleCmd},
								},
							},
						},
					}), execCmd)

					// Print job information
					fmt.Fprintf(cmd.Stdout, "[%d] %d\n", job.ID, execCmd.Process.Pid)

					// Launch a goroutine to monitor the command completion
					go func(job *Job) {
						err := execCmd.Wait()
						if err != nil {
							// Update job status on completion
							cmd.JobManager.mu.Lock()
							job.Status = "Done"
							cmd.JobManager.mu.Unlock()
						} else {
							// Update job status on completion
							cmd.JobManager.mu.Lock()
							job.Status = "Done"
							cmd.JobManager.mu.Unlock()
						}

						// Notify when the job completes (next prompt)
						fmt.Printf("\n[%d]+ Done %s\n", job.ID, job.Command)
					}(job)

					cmd.ReturnCode = 0
				}
			} else {
				// Run the command in the foreground (normal execution)
				err := execCmd.Run()
				if err != nil {
					fmt.Fprintf(cmd.Stderr, "Error executing command: %v\n", err)
					cmd.ReturnCode = 1
				} else {
					cmd.ReturnCode = 0
				}
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

	// Handle multi-command pipelines
	// We'll implement proper handling here rather than delegating to the original implementation
	// to ensure consistent behavior with background execution

	// Set up the pipeline
	var cmds []*exec.Cmd
	var pipes []*io.PipeWriter

	// Manual background detection: check if the last command has background flag
	if len(pipeline.Commands) > 0 && pipeline.Commands[len(pipeline.Commands)-1].Background {
		pipeline.Background = true
	}

	// Process each command in the pipeline
	for i, simpleCmd := range pipeline.Commands {
		cmdString := strings.Join(simpleCmd.Parts, " ")

		// Check if the command is an M28 Lisp expression
		if m28adapter.IsLispExpression(cmdString) {
			result, err := m28Interpreter.Execute(cmdString)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "M28 error in '%s': %v\n", cmdString, err)
				cmd.ReturnCode = 1
				return false
			}
			// For Lisp expressions in pipelines, we want to pass their output to the next command
			lastOutput = strings.NewReader(result + "\n")
			continue
		}

		// Process the command
		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename := parser.ProcessCommand(simpleCmd)

		// Handle input redirection for the first command
		if i == 0 && inputRedirectType == "<" && inputFilename != "" {
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

		// Handle output redirection for the last command
		if i == len(pipeline.Commands)-1 && outputRedirectType != "" && outputFilename != "" {
			var err error
			outputFile, err = cmd.setupOutputRedirection(outputRedirectType, outputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error setting up output redirection: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			// Don't defer the close - we'll close it explicitly after using it
		}

		// Handle builtins differently - they need special handling in pipelines
		if builtin, ok := builtins[cmdName]; ok {
			// Create a buffer for capturing output if this is not the last command
			var output strings.Builder
			outWriter := cmd.Stdout
			if i < len(pipeline.Commands)-1 {
				outWriter = &output
			} else if outputFile != nil {
				outWriter = outputFile
			}

			// Create a temporary command for the builtin
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
				Command:    singleCmd,
				Stdin:      lastOutput,
				Stdout:     outWriter,
				Stderr:     cmd.Stderr,
				JobManager: cmd.JobManager,
			}

			err := builtin(tmpCmd)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "%s: %v\n", cmdName, err)
				cmd.ReturnCode = 1

				// Close any resources
				if outputFile != nil {
					outputFile.Close()
				}

				return false
			}

			// Propagate return code
			cmd.ReturnCode = tmpCmd.ReturnCode

			// If this is not the last command, set up the output for the next command
			if i < len(pipeline.Commands)-1 {
				lastOutput = strings.NewReader(output.String())
			}
		} else {
			// Handle external commands
			execCmd := exec.Command(cmdName, args...)
			gs := GetGlobalState()
			execCmd.Dir = gs.GetCWD()
			execCmd.Stdin = lastOutput
			execCmd.Stderr = cmd.Stderr

			// Set up stdout appropriately based on position in pipeline
			if i < len(pipeline.Commands)-1 {
				// Not the last command - connect to the next via pipe
				r, w := io.Pipe()
				execCmd.Stdout = w
				lastOutput = r
				pipes = append(pipes, w)
			} else if outputFile != nil {
				// Last command with redirection
				execCmd.Stdout = outputFile
			} else {
				// Last command, standard output
				execCmd.Stdout = cmd.Stdout
			}

			cmds = append(cmds, execCmd)
		}
	}

	// If no external commands in the pipeline, we're done
	if len(cmds) == 0 {
		return cmd.ReturnCode == 0
	}

	// Start all commands
	for _, execCmd := range cmds {
		err := execCmd.Start()
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "Error starting command: %v\n", err)
			cmd.ReturnCode = 1

			// Close any resources
			if outputFile != nil {
				outputFile.Close()
			}

			return false
		}
	}

	// Check if the pipeline should run in the background
	if pipeline.Background && cmd.JobManager != nil && len(cmds) > 0 {
		// Create a descriptive command name for the pipeline
		pipelineDesc := parser.FormatCommand(&parser.Command{
			LogicalBlocks: []*parser.LogicalBlock{
				{
					FirstPipeline: pipeline,
				},
			},
		})

		// Add the job to the job manager
		job := cmd.JobManager.AddJob(pipelineDesc, cmds[0])

		// Print job information
		fmt.Fprintf(cmd.Stdout, "[%d] %d\n", job.ID, cmds[0].Process.Pid)

		// Launch a goroutine to manage the pipeline execution
		go func(cmds []*exec.Cmd, pipes []*io.PipeWriter, job *Job, outputFile *os.File) {
			// Wait for all commands to complete
			for i, execCmd := range cmds {
				execCmd.Wait() // Ignore errors for background processes

				// Close the pipe after the command completes
				if i < len(cmds)-1 && i < len(pipes) {
					pipes[i].Close()
				}
			}

			// Update job status on completion
			cmd.JobManager.mu.Lock()
			job.Status = "Done"
			cmd.JobManager.mu.Unlock()

			// Notify when the job completes (next prompt)
			// Use a synchronized approach to avoid race conditions
			cmd.JobManager.mu.Lock()
			notification := fmt.Sprintf("\n[%d]+ Done %s\n", job.ID, job.Command)
			cmd.JobManager.mu.Unlock()

			// Print notification - should ideally use a synchronized writer
			fmt.Print(notification)

			// Close the output file if it's still open
			if outputFile != nil {
				outputFile.Close()
			}
		}(cmds, pipes, job, outputFile)

		cmd.ReturnCode = 0
		return true
	}

	// For foreground pipelines, wait for all commands to complete
	var lastErr error
	for i, execCmd := range cmds {
		err := execCmd.Wait()
		if err != nil {
			lastErr = err
			fmt.Fprintf(cmd.Stderr, "Error executing command: %v\n", err)
		}

		// Close the pipe after the command completes
		if i < len(cmds)-1 && i < len(pipes) {
			pipes[i].Close()
		}
	}

	// Close the output file if it's still open
	if outputFile != nil {
		outputFile.Close()
	}

	// Set return code based on the last error encountered
	if lastErr != nil {
		cmd.ReturnCode = 1
		return false
	}

	cmd.ReturnCode = 0
	return true
}
