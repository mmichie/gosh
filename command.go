package gosh

import (
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"gosh/parser"
)

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

type Redirection struct {
	Operator       string `parser:"@Operator"`
	Word           string `parser:"@Word"`
	FileDescriptor int    `parser:"@Int"`
}

func NewCommand(input string) (*Command, error) {
	parsedCmd, err := parser.Parse(input)
	if err != nil {
		return nil, err
	}
	return &Command{Command: parsedCmd}, nil
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

// Update simple execution to handle command structures
func (cmd *Command) simpleExec() {
	if cmd.SimpleCommand == nil {
		return
	}
	executable, err := exec.LookPath(cmd.SimpleCommand.Items[0].Value)
	if err != nil {
		log.Printf("Command not found: %s", cmd.SimpleCommand.Items[0].Value)
		return
	}
	args := make([]string, len(cmd.SimpleCommand.Items)-1)
	for i, item := range cmd.SimpleCommand.Items[1:] {
		args[i] = item.Value
	}
	process := exec.Command(executable, args...)
	setupRedirection(process, cmd)

	if cmd.Background {
		err = process.Start()
		if err != nil {
			log.Printf("Error executing %s in background: %v", executable, err)
			return
		}
		log.Printf("Started %s [PID: %d] in background", executable, process.Process.Pid)
	} else {
		err = process.Run()
		if err != nil {
			log.Printf("Error executing %s: %v", executable, err)
		}
	}
	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
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
	for _, simpleCommand := range cmd.PipelineCommand.Commands {
		executable, err := exec.LookPath(simpleCommand.Items[0].Value)
		if err != nil {
			log.Printf("Command not found: %s", simpleCommand.Items[0].Value)
			return
		}

		args := make([]string, len(simpleCommand.Items)-1)
		for i, item := range simpleCommand.Items[1:] {
			args[i] = item.Value
		}
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
