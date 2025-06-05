package gosh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func init() {
	m28Interpreter = m28adapter.NewInterpreter()
}

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
		cmd.executePipelineImproved(block.FirstPipeline)
		var returnCode int = cmd.ReturnCode

		// Process any logical operators (AND/OR) that follow
		for i := 0; i < len(block.RestPipelines); i++ {
			opPipeline := block.RestPipelines[i]

			// Process based on the operator type
			switch opPipeline.Operator {
			case "&&":
				// AND: Only execute if previous was successful (return code 0)
				if returnCode == 0 {
					cmd.executePipelineImproved(opPipeline.Pipeline)
					returnCode = cmd.ReturnCode
				}
			case "||":
				// OR: Only execute if previous failed (non-zero return code)
				if returnCode != 0 {
					cmd.executePipelineImproved(opPipeline.Pipeline)
					returnCode = cmd.ReturnCode
				}
			}
		}

		// Set the final return code for this block
		cmd.ReturnCode = returnCode
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

// RunWithSemicolons is a legacy function, replaced by the improved Run implementation
// This function is kept for reference only and shouldn't be called directly
func (cmd *Command) RunWithSemicolons() {
	cmd.StartTime = time.Now()
	cmd.TTY = os.Getenv("TTY")
	cmd.EUID = os.Geteuid()

	// Execute all LogicalBlocks (commands separated by semicolons)
	for _, block := range cmd.Command.LogicalBlocks {
		// Process the logical block using the existing logic in command.go
		// but scope it to just this block

		// This is a critical part: for the builtins to work properly,
		// we need to put the current block as the ONLY logical block in the command
		singleBlockCmd := &parser.Command{
			LogicalBlocks: []*parser.LogicalBlock{block},
		}

		// Create a scoped command for this logical block
		blockCmd := &Command{
			Command:    singleBlockCmd,
			Stdin:      cmd.Stdin,
			Stdout:     cmd.Stdout,
			Stderr:     cmd.Stderr,
			JobManager: cmd.JobManager,
		}

		// For simpler commands, don't use our complex scoping - fallback to original logic
		// which is known to work for simple commands and logical operators
		if len(cmd.Command.LogicalBlocks) == 1 {
			// Execute the block directly with the original logic
			// Execute the first pipeline in the logical block
			cmd.executePipeline(block.FirstPipeline)
			var returnCode int = cmd.ReturnCode

			// Process the rest of the pipelines based on operator and previous result
			for _, opPipeline := range block.RestPipelines {
				// Check the operator and decide whether to execute the next pipeline
				if opPipeline.Operator == "&&" {
					// AND: Only execute if previous was successful (return code 0)
					if returnCode == 0 {
						cmd.executePipeline(opPipeline.Pipeline)
						returnCode = cmd.ReturnCode
					}
				} else if opPipeline.Operator == "||" {
					// OR: Only execute if previous failed (non-zero return code)
					if returnCode != 0 {
						cmd.executePipeline(opPipeline.Pipeline)
						returnCode = cmd.ReturnCode
					}
				}
			}

			// Set the return code for this block
			cmd.ReturnCode = returnCode

			// Skip the remainder of the logic for simple cases
			continue
		}

		// For multi-block commands (with semicolons), use the scoped approach
		// Execute the first pipeline in the logical block
		blockCmd.executePipeline(block.FirstPipeline)
		var returnCode int = blockCmd.ReturnCode

		// Process the rest of the pipelines based on operator and previous result
		for _, opPipeline := range block.RestPipelines {
			// Check the operator and decide whether to execute the next pipeline
			if opPipeline.Operator == "&&" {
				// AND: Only execute if previous was successful (return code 0)
				if returnCode == 0 {
					blockCmd.executePipeline(opPipeline.Pipeline)
					returnCode = blockCmd.ReturnCode
				}
			} else if opPipeline.Operator == "||" {
				// OR: Only execute if previous failed (non-zero return code)
				if returnCode != 0 {
					blockCmd.executePipeline(opPipeline.Pipeline)
					returnCode = blockCmd.ReturnCode
				}
			}
		}

		// Set the return code for this block and propagate to parent command
		blockCmd.ReturnCode = returnCode
		cmd.ReturnCode = returnCode

		// Each command separated by semicolons is independently executed
		// The final return code will be from the last command
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) bool {
	var cmds []*exec.Cmd
	var pipes []*io.PipeWriter
	var outputFile *os.File
	var inputCleanup func()
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

		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename, _, _, _ := parser.ProcessCommand(simpleCmd)

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
			// Handle builtin commands
			tmpCmd := &Command{
				Command:    cmd.Command,
				Stdin:      lastOutput,
				Stdout:     cmd.Stdout,
				Stderr:     cmd.Stderr,
				JobManager: cmd.JobManager, // Pass JobManager to builtins
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
			if simpleCmd.Background && cmd.JobManager != nil {
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
				// Run command in foreground
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
	for i, simpleCmd := range pipeline.Commands {
		cmdString := strings.Join(simpleCmd.Parts, " ")

		// Check if the command is an M28 expression
		if m28adapter.IsLispExpression(cmdString) {
			result, err := m28Interpreter.Execute(cmdString)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "M28 error in '%s': %v\n", cmdString, err)
				cmd.ReturnCode = 1
				return false
			}
			output := result + "\n"
			if i < len(pipeline.Commands)-1 {
				lastOutput = strings.NewReader(output)
			} else {
				fmt.Fprint(cmd.Stdout, output)
			}
			continue
		}

		// Evaluate any embedded M28 expressions
		evaluatedCmd, err := evaluateM28InCommand(cmdString)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "M28 error in '%s': %v\n", cmdString, err)
			cmd.ReturnCode = 1
			return false
		}

		// Re-parse the command after M28 evaluation
		parsedCmd, err := parser.Parse(evaluatedCmd)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "Parse error: %v\n", err)
			cmd.ReturnCode = 1
			return false
		}
		simpleCmd = parsedCmd.LogicalBlocks[0].FirstPipeline.Commands[0]

		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename, _, _, _ := parser.ProcessCommand(simpleCmd)

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

		// Handle output redirection
		if outputRedirectType != "" && outputFilename != "" {
			var err error
			outputFile, err = cmd.setupOutputRedirection(outputRedirectType, outputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error setting up output redirection: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			// Don't defer the close, we'll close it explicitly after using it
		}

		if builtin, ok := builtins[cmdName]; ok {
			// Handle builtin commands
			var output bytes.Buffer
			tmpCmd := &Command{
				Command:    cmd.Command,
				Stdin:      lastOutput,
				Stdout:     &output,
				Stderr:     cmd.Stderr,
				JobManager: cmd.JobManager, // Pass JobManager to builtins
			}
			err := builtin(tmpCmd)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "%s: %v\n", cmdName, err)
				cmd.ReturnCode = 1
				return false
			}

			// Propagate return code from builtin
			if tmpCmd.ReturnCode != 0 {
				cmd.ReturnCode = tmpCmd.ReturnCode
			}

			lastOutput = &output

			// Write the output of the built-in command
			if i == len(pipeline.Commands)-1 {
				if outputFile != nil {
					io.Copy(outputFile, &output)
					outputFile.Close()
					outputFile = nil
				} else {
					io.Copy(cmd.Stdout, &output)
				}
			}
		} else {
			// Handle external commands
			args = ExpandWildcards(args)
			execCmd := exec.Command(cmdName, args...)
			gs := GetGlobalState()
			execCmd.Dir = gs.GetCWD()
			execCmd.Stdin = lastOutput
			execCmd.Stderr = cmd.Stderr

			if i < len(pipeline.Commands)-1 {
				r, w := io.Pipe()
				execCmd.Stdout = w
				lastOutput = r
				pipes = append(pipes, w)
			} else if outputFile != nil {
				execCmd.Stdout = outputFile
			} else {
				execCmd.Stdout = cmd.Stdout
			}

			cmds = append(cmds, execCmd)
		}
	}

	// Start all commands
	for _, execCmd := range cmds {
		err := execCmd.Start()
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "Error starting command: %v\n", err)
			cmd.ReturnCode = 1
			return false
		}
	}

	// Manual background detection: if the last command in a pipeline has the background flag,
	// treat the entire pipeline as background
	if len(pipeline.Commands) > 0 && pipeline.Commands[len(pipeline.Commands)-1].Background {
		pipeline.Background = true
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

		// Add the first command to the job manager to track the pipeline
		job := cmd.JobManager.AddJob(pipelineDesc, cmds[0])

		// Print job information
		fmt.Fprintf(cmd.Stdout, "[%d] %d\n", job.ID, cmds[0].Process.Pid)

		// Launch a goroutine to manage the pipeline execution
		go func(cmds []*exec.Cmd, pipes []*io.PipeWriter, job *Job) {
			// Wait for all commands to complete
			var err error
			for i, execCmd := range cmds {
				err = execCmd.Wait()
				if err != nil {
					// Don't report error for background processes
					if i < len(cmds)-1 {
						pipes[i].Close()
					}
				} else {
					if i < len(cmds)-1 {
						pipes[i].Close()
					}
				}
			}

			// Update job status on completion
			cmd.JobManager.mu.Lock()
			job.Status = "Done"
			cmd.JobManager.mu.Unlock()

			// Notify when the job completes (next prompt)
			fmt.Printf("\n[%d]+ Done %s\n", job.ID, job.Command)

			// Close any output files
			if outputFile != nil {
				outputFile.Close()
			}
		}(cmds, pipes, job)

		cmd.ReturnCode = 0
		return true
	}

	// For foreground pipelines, wait for all commands to complete
	for i, execCmd := range cmds {
		err := execCmd.Wait()
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "Error executing command: %v\n", err)
			cmd.ReturnCode = 1
			return false
		}
		if i < len(cmds)-1 {
			pipes[i].Close()
		}
	}

	// Close any output files
	if outputFile != nil {
		outputFile.Close()
	}

	cmd.ReturnCode = 0
	return true
}

func evaluateM28InCommand(cmdString string) (string, error) {
	re := regexp.MustCompile(`\((.*?)\)`)
	var lastErr error
	result := re.ReplaceAllStringFunc(cmdString, func(match string) string {
		if m28adapter.IsLispExpression(match) {
			result, err := m28Interpreter.Execute(match)
			if err != nil {
				lastErr = fmt.Errorf("in '%s': %v", match, err)
				return match // Keep the original expression if there's an error
			}
			return result
		}
		return match
	})
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
