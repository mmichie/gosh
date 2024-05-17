package gosh

import (
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"gosh/parser"
)

type Redirection struct {
	Operator       string `parser:"@Operator"`
	Word           string `parser:"@Ident|@String"`
	FileDescriptor int
}

type Command struct {
	*parser.Command
	Redirections []*Redirection `parser:"@@*"`
	Background   bool           `parser:"@'&'"`
	Stdin        io.Reader
	Stdout       io.Writer
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	TTY          string
	EUID         int
	CWD          string
	ReturnCode   int
}

func NewCommand(input string) (*Command, error) {
	parsedCmd, err := parser.Parse(input)
	if err != nil {
		return nil, err
	}
	return &Command{Command: parsedCmd}, nil
}

func (cmd *Command) Read(p []byte) (n int, err error) {
	return cmd.Stdin.Read(p)
}

func (cmd *Command) Run() {
	cmd.StartTime = time.Now()
	if cmd.PipelineCommand != nil {
		cmd.pipelineExec()
	} else {
		cmd.simpleExec()
	}
	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) simpleExec() {
	executable, err := exec.LookPath(cmd.SimpleCommand.Command)
	if err != nil {
		log.Printf("Command not found: %s", cmd.SimpleCommand.Command)
		return
	}

	args := append(cmd.SimpleCommand.Options, cmd.SimpleCommand.Args...)
	process := exec.Command(executable, args...)
	setupRedirection(process, cmd)

	if cmd.Background {
		err = process.Start()
		if err != nil {
			log.Printf("Error executing %s in background: %v", cmd.SimpleCommand.Command, err)
			return
		}
		log.Printf("Started %s [PID: %d] in background", cmd.SimpleCommand.Command, process.Process.Pid)
	} else {
		err = process.Run()
		if err != nil {
			log.Printf("Error executing %s: %v", cmd.SimpleCommand.Command, err)
		}
	}
}

func setupRedirection(process *exec.Cmd, cmd *Command) {
	for _, redirection := range cmd.Redirections {
		var file *os.File
		var err error
		if redirection.Operator == ">" || redirection.Operator == ">>" {
			file, err = os.OpenFile(redirection.Word, os.O_CREATE|os.O_WRONLY|(func() int {
				if redirection.Operator == ">>" {
					return os.O_APPEND
				}
				return os.O_TRUNC
			}()), 0644)
		} else if redirection.Operator == "<" {
			file, err = os.Open(redirection.Word)
		}
		if err != nil {
			log.Printf("Error opening file %s for redirection: %v", redirection.Word, err)
			return
		}
		if redirection.FileDescriptor != 0 {
			process.ExtraFiles = append(process.ExtraFiles, file)
		} else {
			switch redirection.Operator {
			case ">":
				process.Stdout = file
			case "<":
				process.Stdin = file
			case ">>":
				process.Stdout = file
			}
		}
	}
}

func (cmd *Command) pipelineExec() {
	var commands []*exec.Cmd
	for _, shellCommand := range cmd.PipelineCommand.PipeCommands {
		executable, err := exec.LookPath(shellCommand.SimpleCommand.Command)
		if err != nil {
			log.Printf("Command not found: %s", shellCommand.SimpleCommand.Command)
			return
		}

		args := append(shellCommand.SimpleCommand.Options, shellCommand.SimpleCommand.Args...)
		command := exec.Command(executable, args...)
		commands = append(commands, command)
	}

	for i := 0; i < len(commands)-1; i++ {
		pipe, err := commands[i].StdoutPipe()
		if err != nil {
			log.Printf("Error creating pipe: %v", err)
			return
		}
		commands[i+1].Stdin = pipe
	}

	for _, command := range commands {
		err := command.Start()
		if err != nil {
			log.Printf("Error starting command: %v", err)
			return
		}
	}

	for _, command := range commands {
		err := command.Wait()
		if err != nil {
			log.Printf("Error waiting for command: %v", err)
		}
	}
}
