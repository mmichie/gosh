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

	cmd.simpleExec()

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
	cmdName, args, redirectType, filename := parser.ProcessCommand(cmd.Command)

	log.Printf("Executing command: %s", cmdName)
	log.Printf("Command arguments: %v", args)
	log.Printf("Redirection: %s %s", redirectType, filename)

	// Check for built-in commands
	if builtinCmd, ok := builtins[cmdName]; ok {
		log.Println("Executing builtin command")
		builtinCmd(cmd)
		return
	}

	// External command execution
	execCmd := exec.Command(cmdName, args...)
	execCmd.Stdin = cmd.Stdin
	execCmd.Stderr = cmd.Stderr

	if redirectType != "" {
		var file *os.File
		var err error

		switch redirectType {
		case ">":
			file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		case ">>":
			file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		default:
			err = fmt.Errorf("unknown redirection type: %s", redirectType)
		}

		if err != nil {
			log.Printf("Error opening output file: %v", err)
			fmt.Fprintf(cmd.Stderr, "Error opening output file: %v\n", err)
			cmd.ReturnCode = 1
			return
		}
		defer file.Close()
		execCmd.Stdout = file
		log.Println("Redirection set up successfully")
	} else {
		execCmd.Stdout = cmd.Stdout
		log.Println("No redirection, using standard output")
	}

	err := execCmd.Run()
	if err != nil {
		log.Printf("Command execution error: %v", err)
		fmt.Fprintf(cmd.Stderr, "%s: %v\n", cmdName, err)
		cmd.ReturnCode = 1
	} else {
		log.Printf("Command executed successfully, exit code: %d", execCmd.ProcessState.ExitCode())
		cmd.ReturnCode = execCmd.ProcessState.ExitCode()
	}
}
