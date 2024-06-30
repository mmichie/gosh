package gosh

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
	CWD        string
	ReturnCode int
}

func NewCommand(input string) (*Command, error) {
	parsedCmd, err := parser.Parse(input)
	if err != nil {
		return nil, err
	}
	return &Command{
		Command: parsedCmd,
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}, nil
}

func (cmd *Command) Run() {
	cmd.StartTime = time.Now()
	cmd.TTY = os.Getenv("TTY")
	cmd.EUID = os.Geteuid()
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current working directory: %v", err)
	}
	cmd.CWD = cwd

	if cmd.SimpleCommand != nil {
		cmd.simpleExec()
	} else if cmd.PipelineCommand != nil {
		cmd.pipelineExec()
	}
	// Add code to handle other command types (ForLoop, IfCondition, CaseStatement) if needed

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)

	// Log the command execution details
	historyManager, err := NewHistoryManager("")
	if err != nil {
		log.Printf("Failed to create history manager: %v", err)
	} else {
		err = historyManager.Insert(cmd, 0) // Replace 0 with the actual session ID
		if err != nil {
			log.Printf("Failed to insert command into history: %v", err)
		}
	}
}

func (cmd *Command) simpleExec() {
	if cmd.SimpleCommand == nil || len(cmd.SimpleCommand.Items) == 0 {
		return
	}

	cmdName := cmd.SimpleCommand.Items[0].Value

	// Check for built-in commands
	if builtinCmd, ok := builtins[cmdName]; ok {
		builtinCmd(cmd)
		return
	}

	// External command execution
	cmdArgs := make([]string, len(cmd.SimpleCommand.Items)-1)
	for i, item := range cmd.SimpleCommand.Items[1:] {
		cmdArgs[i] = item.Value
	}

	execCmd := exec.Command(cmdName, cmdArgs...)
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	err := execCmd.Run()
	if err != nil {
		fmt.Fprintf(cmd.Stderr, "%s: command not found\n", cmdName)
		cmd.ReturnCode = 1
	} else {
		cmd.ReturnCode = execCmd.ProcessState.ExitCode()
	}
}

func (cmd *Command) pipelineExec() {
	// Implementation for pipeline execution
	// This is more complex and involves creating pipes between commands
	// For now, we'll leave this as a placeholder
	fmt.Fprintln(cmd.Stderr, "Pipeline execution not implemented yet")
}
