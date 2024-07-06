package gosh

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"gosh/parser"
)

// DebugMode controls whether debug output is printed
var DebugMode bool

// debugPrint prints debug output if DebugMode is true
func debugPrint(format string, v ...interface{}) {
	if DebugMode {
		log.Printf(format, v...)
	}
}

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
	CWD        string
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
	cwd, err := os.Getwd()
	if err != nil {
		debugPrint("Error getting current working directory: %v", err)
	}
	cmd.CWD = cwd

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
	debugPrint("Executing pipeline: %v", pipeline)
	var cmds []*exec.Cmd

	// Create commands
	for i, simpleCmd := range pipeline.Commands {
		cmdName, args, _, _, _, _ := parser.ProcessCommand(simpleCmd)
		debugPrint("Command %d: %s %v", i, cmdName, args)

		// Handle quotes in arguments
		var processedArgs []string
		for _, arg := range args {
			if strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'") {
				// Remove surrounding single quotes
				processedArgs = append(processedArgs, arg[1:len(arg)-1])
			} else {
				processedArgs = append(processedArgs, arg)
			}
		}

		execCmd := exec.Command(cmdName, processedArgs...)
		cmds = append(cmds, execCmd)
	}

	// Connect commands with pipes
	for i := 0; i < len(cmds)-1; i++ {
		reader, writer := io.Pipe()
		cmds[i].Stdout = writer
		cmds[i+1].Stdin = reader
		debugPrint("Connected pipe between command %d and %d", i, i+1)
	}

	// Capture the output of the last command
	var output bytes.Buffer
	cmds[len(cmds)-1].Stdout = io.MultiWriter(&output, cmd.Stdout)

	// Set stderr for all commands
	for i, execCmd := range cmds {
		execCmd.Stderr = cmd.Stderr
		debugPrint("Set stderr for command %d", i)
	}

	// Start all commands
	for i, execCmd := range cmds {
		debugPrint("Starting command %d: %v", i, execCmd.Args)
		err := execCmd.Start()
		if err != nil {
			debugPrint("Error starting command %d: %v", i, err)
			fmt.Fprintf(cmd.Stderr, "Error starting command: %v\n", err)
			cmd.ReturnCode = 1
			return false
		}
	}

	// Wait for all commands to complete
	for i, execCmd := range cmds {
		debugPrint("Waiting for command %d to complete", i)
		err := execCmd.Wait()
		if err != nil {
			debugPrint("Command %d execution error: %v", i, err)
			fmt.Fprintf(cmd.Stderr, "%s: %v\n", execCmd.Path, err)
			cmd.ReturnCode = 1
			if i == len(cmds)-1 {
				return false
			}
		}
		debugPrint("Command %d completed", i)
		if i < len(cmds)-1 {
			cmds[i].Stdout.(io.WriteCloser).Close()
		}
	}

	debugPrint("Pipeline execution completed")
	debugPrint("Captured output: %s", output.String())

	if len(cmds) > 0 {
		cmd.ReturnCode = cmds[len(cmds)-1].ProcessState.ExitCode()
	}
	debugPrint("Pipeline return code: %d", cmd.ReturnCode)
	return cmd.ReturnCode == 0
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
