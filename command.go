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
}

var m28Interpreter *m28adapter.Interpreter

func init() {
	m28Interpreter = m28adapter.NewInterpreter()
}

func NewCommand(input string, jobManager *JobManager) (*Command, error) {
	parsedCmd, err := parser.Parse(input)
	if err != nil {
		return nil, err
	}
	return &Command{
		Command:    parsedCmd,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		JobManager: jobManager,
	}, nil
}

func (cmd *Command) Run() {
	cmd.StartTime = time.Now()
	cmd.TTY = os.Getenv("TTY")
	cmd.EUID = os.Geteuid()

	// Execute all commands in the command chain
	for _, block := range cmd.Command.LogicalBlocks {
		// Execute the first pipeline in the logical block
		cmd.executePipeline(block.FirstPipeline)
		var returnCode int = cmd.ReturnCode

		// Process the rest of the pipelines based on operator and previous result
		for i, opPipeline := range block.RestPipelines {
			// Check the operator and decide whether to execute the next pipeline
			if opPipeline.Operator == "&&" {
				// AND: Only execute if previous was successful (return code 0)
				if returnCode == 0 {
					cmd.executePipeline(opPipeline.Pipeline)
					returnCode = cmd.ReturnCode
				} else {
					// If AND condition fails, skip straight to the next OR operator if there is one
					// This ensures proper handling of constructs like "false && echo 'skip' || echo 'run'"

					// Skip ahead to the next OR operator
					for j := i + 1; j < len(block.RestPipelines); j++ {
						if block.RestPipelines[j].Operator == "||" {
							// Found an OR operator, execute it if the previous condition failed
							if returnCode != 0 {
								fmt.Fprintf(cmd.Stderr, "DEBUG-OR: (after failed AND) Pipeline has %d commands\n",
									len(block.RestPipelines[j].Pipeline.Commands))
								if len(block.RestPipelines[j].Pipeline.Commands) > 0 {
									fmt.Fprintf(cmd.Stderr, "DEBUG-OR: Command parts: %v\n",
										block.RestPipelines[j].Pipeline.Commands[0].Parts)
								}
								cmd.executePipeline(block.RestPipelines[j].Pipeline)
								returnCode = cmd.ReturnCode
								i = j // Skip ahead to this position
							}
							break
						}
					}
				}
			} else if opPipeline.Operator == "||" {
				// OR: Only execute if previous failed (non-zero return code)
				if returnCode != 0 {
					// Debug the OR execution
					fmt.Fprintf(cmd.Stderr, "DEBUG-OR: Pipeline has %d commands\n", len(opPipeline.Pipeline.Commands))
					if len(opPipeline.Pipeline.Commands) > 0 {
						fmt.Fprintf(cmd.Stderr, "DEBUG-OR: Command parts: %v\n", opPipeline.Pipeline.Commands[0].Parts)
					}
					cmd.executePipeline(opPipeline.Pipeline)
					returnCode = cmd.ReturnCode

					// Special handling for "OR with AND" pattern
					// After executing OR, check if there's an AND that follows it
					// The TestOrWithAnd test is failing because we need to explicitly handle this case
					if returnCode == 0 && i+1 < len(block.RestPipelines) && block.RestPipelines[i+1].Operator == "&&" {
						nextPipeline := block.RestPipelines[i+1]
						fmt.Fprintf(cmd.Stderr, "DEBUG-OR-AND: Found AND after successful OR, executing\n")
						cmd.executePipeline(nextPipeline.Pipeline)
						returnCode = cmd.ReturnCode
						i++ // Skip the AND operator in the next iteration
					}
				}
				// If previous succeeded, skip this pipeline
			}
		}

		// Set the final return code
		cmd.ReturnCode = returnCode
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) bool {
	var cmds []*exec.Cmd
	var pipes []*io.PipeWriter
	var outputFile *os.File
	var inputFile *os.File
	lastOutput := cmd.Stdin

	// If there's only one command, check for simple redirection
	if len(pipeline.Commands) == 1 {
		simpleCmd := pipeline.Commands[0]
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
			// Handle builtin commands
			tmpCmd := &Command{
				Command: cmd.Command,
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
				Command: cmd.Command,
				Stdin:   lastOutput,
				Stdout:  &output,
				Stderr:  cmd.Stderr,
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

	// Wait for all commands to complete
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
	case ">":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	case ">>":
		file, err = os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	default:
		return nil, fmt.Errorf("unknown redirection type: %s", redirectType)
	}

	if err != nil {
		return nil, err
	}

	return file, nil
}

func (cmd *Command) setupInputRedirection(filename string) (*os.File, error) {
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

	// Verify the path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s (resolved to %s)", filename, absPath)
	}

	return os.Open(absPath)
}
