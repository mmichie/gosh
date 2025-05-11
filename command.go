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

	// Check for redirection at the top level
	if len(cmd.AndCommands) == 1 && len(cmd.AndCommands[0].Pipelines) == 1 {
		pipeline := cmd.AndCommands[0].Pipelines[0]
		if len(pipeline.Commands) == 1 {
			simpleCmd := pipeline.Commands[0]
			_, _, inputRedirectType, inputFilename, outputRedirectType, outputFilename := parser.ProcessCommand(simpleCmd)

			// Setup input redirection
			var originalStdin io.Reader = cmd.Stdin
			if inputRedirectType == "<" && inputFilename != "" {
				inputFile, err := os.Open(inputFilename)
				if err != nil {
					fmt.Fprintf(cmd.Stderr, "Error opening input file: %v\n", err)
					cmd.ReturnCode = 1
					return
				}
				defer inputFile.Close()
				cmd.Stdin = inputFile
			}

			// Setup output redirection
			var originalStdout io.Writer = cmd.Stdout
			if outputRedirectType != "" && outputFilename != "" {
				var outputFile *os.File
				var err error
				if outputRedirectType == ">" {
					outputFile, err = os.OpenFile(outputFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				} else { // ">>"
					outputFile, err = os.OpenFile(outputFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				}

				if err != nil {
					fmt.Fprintf(cmd.Stderr, "Error opening output file: %v\n", err)
					cmd.ReturnCode = 1
					return
				}
				defer outputFile.Close()
				cmd.Stdout = outputFile
			}

			// Run with redirected I/O
			for _, andCommand := range cmd.AndCommands {
				success := true
				for _, pipeline := range andCommand.Pipelines {
					success = cmd.executePipeline(pipeline)
					if !success {
						break
					}
				}
				if !success {
					break
				}
			}

			// Restore original I/O
			cmd.Stdin = originalStdin
			cmd.Stdout = originalStdout
		} else {
			// Normal execution without redirection handling
			for _, andCommand := range cmd.AndCommands {
				success := true
				for _, pipeline := range andCommand.Pipelines {
					success = cmd.executePipeline(pipeline)
					if !success {
						break
					}
				}
				if !success {
					break
				}
			}
		}
	} else {
		// Normal execution without redirection handling
		for _, andCommand := range cmd.AndCommands {
			success := true
			for _, pipeline := range andCommand.Pipelines {
				success = cmd.executePipeline(pipeline)
				if !success {
					break
				}
			}
			if !success {
				break
			}
		}
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) bool {
	var cmds []*exec.Cmd
	var pipes []*io.PipeWriter
	var outputFile *os.File
	lastOutput := cmd.Stdin

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
		simpleCmd = parsedCmd.AndCommands[0].Pipelines[0].Commands[0]

		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename := parser.ProcessCommand(simpleCmd)

		// Handle input redirection
		var inputFile *os.File
		if inputRedirectType == "<" && inputFilename != "" {
			inputFile, err = cmd.setupInputRedirection(inputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error opening input file: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			defer inputFile.Close()
			lastOutput = inputFile
		}

		// Handle output redirection
		if outputRedirectType != "" && outputFilename != "" {
			fmt.Fprintf(cmd.Stderr, "Setting up output redirection: %s %s\n", outputRedirectType, outputFilename)
			outputFile, err = cmd.setupOutputRedirection(outputRedirectType, outputFilename)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Error setting up output redirection: %v\n", err)
				cmd.ReturnCode = 1
				return false
			}
			// Print out file info
			fileInfo, _ := outputFile.Stat()
			if fileInfo != nil {
				fmt.Fprintf(cmd.Stderr, "Output file created: %s, path: %s\n", fileInfo.Name(), outputFilename)
			}
			// Don't defer the close, we need to keep the file open until we write to it
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
			lastOutput = &output

			// Write the output of the built-in command
			if i == len(pipeline.Commands)-1 {
				if outputFile != nil {
					fmt.Fprintf(cmd.Stderr, "Writing to output file for builtin command\n")
					n, err := io.Copy(outputFile, &output)
					if err != nil {
						fmt.Fprintf(cmd.Stderr, "Error writing to file: %v\n", err)
					} else {
						fmt.Fprintf(cmd.Stderr, "Wrote %d bytes to file\n", n)
					}
					outputFile.Close() // Close the file after writing to it
					outputFile = nil   // Prevent closing again
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
		fmt.Fprintf(cmd.Stderr, "Closing output file for external command\n")
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
	// Get absolute path and ensure parent directories exist
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("error resolving path: %v", err)
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
	return os.Open(filename)
}
