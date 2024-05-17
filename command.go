// command.go
package gosh

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var shellLexer = lexer.MustSimple([]lexer.SimpleRule{
	{"Ident", `[a-zA-Z_][a-zA-Z0-9_]*`},
	{"Path", `/?[\w./-]+`},
	{"Option", `-\w+`},
	{"String", `"(\\\\"|[^"])*"`},
	{"SingleQuotedString", `'(\\\\'|[^'])*'`},
	{"Operator", `[<>|&;]+`},
	{"Whitespace", `\s+`},
})

type SimpleCommandElement struct {
	Word string `parser:"@Ident | @Path | @Option | @String"`
}

type SimpleCommand struct {
	Command  string                  `parser:"@Ident"`
	Options  []string                `parser:"(@Whitespace @Option)*"`
	Args     []string                `parser:"(@Whitespace (@Ident | @Path | @String))*"`
	Elements []*SimpleCommandElement `parser:"@@*"`
}

type ShellCommand struct {
	SimpleCommand *SimpleCommand `parser:"@@"`
}

type PipelineCommand struct {
	Bang         bool            `parser:"@'!'?"`
	Timespec     *Timespec       `parser:"@@?"`
	PipeCommands []*ShellCommand `parser:"@@ ( '|' @@ )*"`
}

type Timespec struct {
	Opt string `parser:"'time' @Ident?"`
}

type Redirection struct {
	Operator       string `parser:"@Operator"`
	Word           string `parser:"@Ident|@String"`
	FileDescriptor int
}

type Command struct {
	SimpleCommand   *SimpleCommand   `parser:"@@"`
	PipelineCommand *PipelineCommand `parser:"| @@"`
	Redirections    []*Redirection   `parser:"@@*"`
	Background      bool             `parser:"@'&'"`
	Stdin           io.Reader
	Stdout          io.Writer
	StartTime       time.Time
	EndTime         time.Time
	Duration        time.Duration
	TTY             string
	EUID            int
	CWD             string
	ReturnCode      int
}

var parser = participle.MustBuild[Command](
	participle.Lexer(shellLexer),
	participle.Unquote(),
)

func NewCommand(input string) (*Command, error) {
	command, err := parser.ParseString("", input)
	if err != nil {
		log.Printf("Failed to parse command string: %s, error: %v", input, err)
		return nil, fmt.Errorf("parse error: %v", err)
	}
	log.Printf("Parsed command: %+v", command)
	return command, nil
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
