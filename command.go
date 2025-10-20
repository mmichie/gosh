package gosh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gosh/m28adapter"
	"gosh/parser"
)

type Command struct {
	*parser.Command
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	TTY        string
	EUID       int
	ReturnCode int
	JobManager *JobManager
	HereDocs   HereDocMap
}

var m28Interpreter *m28adapter.Interpreter

func NewCommand(input string, jobManager *JobManager) (*Command, error) {
	// Store here-document data for later processing
	var hereDocs HereDocMap

	// Preprocess for here-documents
	// This extracts here-doc content and replaces it with input redirection placeholders
	processedInput, hereDocs, err := PreprocessHereDoc(input)
	if err != nil {
		return nil, fmt.Errorf("here-document error: %v", err)
	}

	// Perform command substitution before parsing
	// This replaces $(...) and `...` with their command output
	processedInput, err = PerformCommandSubstitution(processedInput, jobManager)
	if err != nil {
		return nil, fmt.Errorf("command substitution error: %v", err)
	}

	// Evaluate M28 Lisp expressions
	processedInput, err = evaluateM28InCommand(processedInput)
	if err != nil {
		return nil, fmt.Errorf("M28 evaluation error: %v", err)
	}

	// Parse the processed command string
	parsedCmd, err := parser.Parse(processedInput)
	if err != nil {
		return nil, err
	}

	return &Command{
		Command:    parsedCmd,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		JobManager: jobManager,
		HereDocs:   hereDocs,
	}, nil
}

// ParseCommand is a helper function to parse command strings for testing or external use
func ParseCommand(input string) (*parser.Command, error) {
	return parser.Parse(input)
}

func (cmd *Command) Run() {
	cmd.StartTime = time.Now()
	cmd.TTY = os.Getenv("TTY")
	cmd.EUID = os.Geteuid()

	// Execute all LogicalBlocks (commands separated by semicolons)
	for _, block := range cmd.Command.LogicalBlocks {

		// Execute the first pipeline in the block
		cmd.executePipeline(block.FirstPipeline)
		var returnCode int = cmd.ReturnCode

		// Process any logical operators (AND/OR) that follow
		for i := 0; i < len(block.RestPipelines); i++ {
			opPipeline := block.RestPipelines[i]

			// Process based on the operator type
			switch opPipeline.Operator {
			case "&&":
				// AND: Only execute if previous was successful (return code 0)
				if returnCode == 0 {
					cmd.executePipeline(opPipeline.Pipeline)
					returnCode = cmd.ReturnCode
				}
			case "||":
				// OR: Only execute if previous failed (non-zero return code)
				if returnCode != 0 {
					cmd.executePipeline(opPipeline.Pipeline)
					returnCode = cmd.ReturnCode
				}
			}
		}

		// Set the final return code for this block
		cmd.ReturnCode = returnCode
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)

	// Update the last exit status in global state
	GetGlobalState().SetLastExitStatus(cmd.ReturnCode)
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) bool {
	var outputFile *os.File
	var inputCleanup func()
	lastOutput := cmd.Stdin

	// If there's only one command, check for simple execution (non-pipeline)
	if len(pipeline.Commands) == 1 {
		cmdElem := pipeline.Commands[0]

		// Handle subshells
		if cmdElem.Subshell != nil {
			return cmd.executeSubshell(cmdElem.Subshell, lastOutput)
		}

		// Handle command groups
		if cmdElem.CommandGroup != nil {
			return cmd.executeCommandGroup(cmdElem.CommandGroup, lastOutput)
		}

		// Handle simple commands
		if cmdElem.Simple == nil {
			cmd.ReturnCode = 1
			return false
		}

		simpleCmd := cmdElem.Simple

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

		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename, stderrRedirectType, stderrFilename, fdDupType := parser.ProcessCommand(simpleCmd)

		// Expand variables in arguments (before wildcard expansion so variables can expand to patterns)
		args = ExpandVariablesInArgs(args)

		// Expand wildcards in arguments
		args = ExpandWildcards(args)

		// Handle input redirection
		if inputRedirectType == "<" && inputFilename != "" {
			var err error
			lastOutput, inputCleanup, err = cmd.setupInputRedirection(inputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error opening input file: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			defer inputCleanup()
		}

		// Handle file descriptor duplication (2>&1)
		var originalStderr io.Writer = cmd.Stderr
		if fdDupType == "2>&1" {
			// Set stderr to the same destination as stdout
			err := cmd.setupFileDescriptorDuplication(fdDupType)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error setting up file descriptor duplication: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			cmd.Stderr = cmd.Stdout
			defer func() {
				cmd.Stderr = originalStderr
			}()
		}

		// Handle stderr redirection
		var stderrFile *os.File
		if stderrRedirectType != "" && stderrFilename != "" && stderrRedirectType != "&>" && stderrRedirectType != ">&" {
			var err error
			stderrFile, err = cmd.setupOutputRedirection(stderrRedirectType, stderrFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error setting up stderr redirection: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}

			// Use the error file as stderr, but remember to close it at the end
			cmd.Stderr = stderrFile
			defer func() {
				if stderrFile != nil {
					stderrFile.Close()
					cmd.Stderr = originalStderr
				}
			}()
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

			// For combined redirection (&> or >&), also redirect stderr to the same file
			if outputRedirectType == "&>" || outputRedirectType == ">&" {
				cmd.Stderr = outputFile
				defer func() {
					cmd.Stderr = originalStderr
				}()
			}
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
							Commands: []*parser.CommandElement{{Simple: simpleCmd}},
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
			args = ExpandWildcards(args)
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
									Commands: []*parser.CommandElement{{Simple: simpleCmd}},
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

		// Restore original stdout and stderr if changed
		if outputFile != nil {
			cmd.Stdout = originalStdout
			outputFile.Close()
		}

		if handled {
			return cmd.ReturnCode == 0
		}
	}

	// Handle multi-command pipelines
	// Set up the pipeline
	var cmds []*exec.Cmd
	var pipes []*io.PipeWriter

	// Manual background detection: check if the last command has background flag
	if len(pipeline.Commands) > 0 {
		lastElem := pipeline.Commands[len(pipeline.Commands)-1]
		if (lastElem.Simple != nil && lastElem.Simple.Background) ||
			(lastElem.Subshell != nil && lastElem.Subshell.Background) ||
			(lastElem.CommandGroup != nil && lastElem.CommandGroup.Background) {
			pipeline.Background = true
		}
	}

	// Process each command in the pipeline
	for i, cmdElem := range pipeline.Commands {
		// For subshells and command groups in pipelines, we need special handling
		// For now, let's focus on simple commands in pipelines
		// TODO: Handle subshells and command groups in pipelines
		if cmdElem.Simple == nil {
			fmt.Fprintf(cmd.Stderr, "Subshells and command groups in pipelines not yet supported\n")
			cmd.ReturnCode = 1
			return false
		}

		simpleCmd := cmdElem.Simple
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
		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename, _, _, _ := parser.ProcessCommand(simpleCmd)

		// Expand variables in arguments (before wildcard expansion so variables can expand to patterns)
		args = ExpandVariablesInArgs(args)

		// Expand wildcards in arguments
		args = ExpandWildcards(args)

		// Handle input redirection for the first command
		if i == 0 && inputRedirectType == "<" && inputFilename != "" {
			var err error
			lastOutput, inputCleanup, err = cmd.setupInputRedirection(inputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error opening input file: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			defer inputCleanup()
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
							Commands: []*parser.CommandElement{{Simple: simpleCmd}},
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
			args = ExpandWildcards(args)
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

// findBalancedParens finds all balanced parentheses expressions in a string
func findBalancedParens(s string) []struct{ start, end int } {
	var results []struct{ start, end int }
	var stack []int

	for i, ch := range s {
		if ch == '(' {
			stack = append(stack, i)
		} else if ch == ')' && len(stack) > 0 {
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			// Only consider it a complete expression if we're back at depth 0
			if len(stack) == 0 {
				results = append(results, struct{ start, end int }{start, i + 1})
			}
		}
	}

	return results
}

func evaluateM28InCommand(cmdString string) (string, error) {
	// Initialize m28 interpreter if needed
	if m28Interpreter == nil {
		m28Interpreter = m28adapter.NewInterpreter()
	}

	// Find all balanced parentheses expressions
	expressions := findBalancedParens(cmdString)

	// Process from right to left to avoid index shifting issues
	var lastErr error
	result := cmdString

	for i := len(expressions) - 1; i >= 0; i-- {
		expr := expressions[i]
		match := cmdString[expr.start:expr.end]

		if m28adapter.IsLispExpression(match) {
			evalResult, err := m28Interpreter.Execute(match)
			if err != nil {
				lastErr = fmt.Errorf("in '%s': %v", match, err)
				continue // Keep the original expression if there's an error
			}
			// Replace the expression with its result
			result = result[:expr.start] + evalResult + result[expr.end:]
		}
	}

	return result, lastErr
}

func (cmd *Command) setupOutputRedirection(redirectType, filename string) (*os.File, error) {
	// Get absolute path based on the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = GetGlobalState().GetCWD()
	}

	var absPath string
	if filepath.IsAbs(filename) {
		absPath = filename
	} else {
		absPath = filepath.Join(currentDir, filename)
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(absPath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating parent directories: %v", err)
	}

	var file *os.File

	switch redirectType {
	// Standard output redirection (truncate)
	case ">":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)

	// Standard output redirection (append)
	case ">>":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	// Standard error redirection (truncate)
	case "2>":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)

	// Standard error redirection (append)
	case "2>>":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	// Combined output redirection (both stdout and stderr)
	case "&>", ">&":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)

	default:
		return nil, fmt.Errorf("unknown redirection type: %s", redirectType)
	}

	if err != nil {
		return nil, err
	}

	return file, nil
}

// setupInputRedirection sets up input redirection for a command
// Returns an io.Reader and a cleanup function that must be called when done
func (cmd *Command) setupInputRedirection(filename string) (io.Reader, func(), error) {
	// Check if this is a here-document identifier
	if cmd.HereDocs != nil {
		if hereDoc, ok := cmd.HereDocs[filename]; ok {
			// Process the here-document and return a reader with no-op cleanup
			return ProcessHereDoc(hereDoc), func() {}, nil
		}
	}

	// If not a here-document, handle regular file redirection
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = GetGlobalState().GetCWD()
	}

	var absPath string
	if filepath.IsAbs(filename) {
		absPath = filename
	} else {
		absPath = filepath.Join(currentDir, filename)
	}

	// Verify the path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, func() {}, fmt.Errorf("file not found: %s (resolved to %s)", filename, absPath)
	}

	// Open the file
	file, err := os.Open(absPath)
	if err != nil {
		return nil, func() {}, err
	}

	// Return the file and a cleanup function that closes it
	return file, func() { file.Close() }, nil
}

// setupFileDescriptorDuplication handles redirections like 2>&1 (stderr to stdout)
func (cmd *Command) setupFileDescriptorDuplication(dupType string) error {
	switch dupType {
	case "2>&1":
		// For 2>&1, redirect stderr to the same destination as stdout
		// This is handled in the command execution by setting cmd.Stderr = cmd.Stdout
		return nil
	default:
		return fmt.Errorf("unsupported file descriptor duplication: %s", dupType)
	}
}

// executeSubshell executes commands in a subshell with isolated environment
func (cmd *Command) executeSubshell(subshell *parser.Subshell, input io.Reader) bool {
	// Create a new command for the subshell
	subCmd := &Command{
		Command:    subshell.Command,
		Stdin:      input,
		Stdout:     cmd.Stdout,
		Stderr:     cmd.Stderr,
		JobManager: cmd.JobManager,
		HereDocs:   cmd.HereDocs,
	}

	// Subshells need environment isolation
	// For now, we'll execute in the same process but save/restore state
	// A full implementation would fork a new process

	// Save current environment state
	state := GetGlobalState()
	origCWD := state.GetCWD()
	origEnv := os.Environ()

	// Execute the subshell command
	subCmd.Run()

	// Restore environment state (subshell changes don't affect parent)
	state.UpdateCWD(origCWD)

	// Restore environment variables
	// Clear current env
	for _, envVar := range os.Environ() {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) > 0 {
			os.Unsetenv(parts[0])
		}
	}
	// Restore original env
	for _, envVar := range origEnv {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}

	// Set the return code from the subshell
	cmd.ReturnCode = subCmd.ReturnCode

	return cmd.ReturnCode == 0
}

// executeCommandGroup executes grouped commands without environment isolation
func (cmd *Command) executeCommandGroup(group *parser.CommandGroup, input io.Reader) bool {
	// Create a new command for the group
	groupCmd := &Command{
		Command:    group.Command,
		Stdin:      input,
		Stdout:     cmd.Stdout,
		Stderr:     cmd.Stderr,
		JobManager: cmd.JobManager,
		HereDocs:   cmd.HereDocs,
	}

	// Execute the grouped commands
	// Unlike subshells, command groups share the environment with the parent
	groupCmd.Run()

	// Set the return code from the group
	cmd.ReturnCode = groupCmd.ReturnCode

	return cmd.ReturnCode == 0
}
