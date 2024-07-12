package gosh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

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

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) bool {
	var cmds []*exec.Cmd
	var pipes []*io.PipeWriter
	lastOutput := cmd.Stdin

	for i, simpleCmd := range pipeline.Commands {
		cmdString := strings.Join(simpleCmd.Parts, " ")

		// Check if the command is a Lisp expression
		if IsLispExpression(cmdString) {
			result, err := ExecuteGoshLisp(cmdString)
			if err != nil {
				fmt.Fprintf(cmd.Stderr, "Lisp error in '%s': %v\n", cmdString, err)
				cmd.ReturnCode = 1
				return false
			}
			output := fmt.Sprintf("%v\n", result)
			if i < len(pipeline.Commands)-1 {
				lastOutput = strings.NewReader(output)
			} else {
				fmt.Fprint(cmd.Stdout, output)
			}
			continue
		}

		// Evaluate any embedded Lisp expressions
		evaluatedCmd, err := evaluateLispInCommand(cmdString)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "Lisp error in '%s': %v\n", cmdString, err)
			cmd.ReturnCode = 1
			return false
		}

		// Re-parse the command after Lisp evaluation
		parsedCmd, err := parser.Parse(evaluatedCmd)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "Parse error: %v\n", err)
			cmd.ReturnCode = 1
			return false
		}
		simpleCmd = parsedCmd.AndCommands[0].Pipelines[0].Commands[0]

		cmdName, args, _, _, _, _ := parser.ProcessCommand(simpleCmd)

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

			// Write the output of the built-in command to cmd.Stdout
			if i == len(pipeline.Commands)-1 {
				io.Copy(cmd.Stdout, &output)
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

	cmd.ReturnCode = 0
	return true
}

func evaluateLispInCommand(cmdString string) (string, error) {
	re := regexp.MustCompile(`\((.*?)\)`)
	var lastErr error
	result := re.ReplaceAllStringFunc(cmdString, func(match string) string {
		if IsLispExpression(match) {
			result, err := ExecuteGoshLisp(match)
			if err != nil {
				lastErr = fmt.Errorf("in '%s': %v", match, err)
				return match // Keep the original expression if there's an error
			}
			return fmt.Sprintf("%v", result)
		}
		return match
	})
	return result, lastErr
}

func (cmd *Command) setupOutputRedirection(redirectType, filename string) (*os.File, error) {
	switch redirectType {
	case ">":
		return os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	case ">>":
		return os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	default:
		return nil, fmt.Errorf("unknown redirection type: %s", redirectType)
	}
}
