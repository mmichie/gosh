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

type Redirection struct {
	Operator string `parser:"@Operator"`
	File     string `parser:"@Ident|@String"`
}

type Command struct {
	Name        string       `parser:"@Ident"`
	Args        []string     `parser:"(@Ident | @String)*"`
	Redirection *Redirection `parser:"(@@)?"`
	Background  bool         `parser:"[@ \"&\"]"`
	Pipe        *Command     `parser:"( \"|\" @@ )?"`
	Stdin       io.Reader
	Stdout      io.Writer
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	TTY         string
	EUID        int
	CWD         string
	ReturnCode  int
}

var parser = participle.MustBuild[Command](
	participle.Lexer(lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Operator", Pattern: `[<>|&]`},
		{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
		{Name: "String", Pattern: `"(\\"|[^"])*"`},
		{Name: "Whitespace", Pattern: `\s+`},
	})),
)

// NewCommand parses the input command string and returns a Command struct instance.
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
	if cmd.Pipe != nil {
		cmd.pipeExec()
	} else {
		cmd.execute()
	}
	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) execute() {
	executable, err := exec.LookPath(cmd.Name)
	if err != nil {
		log.Printf("Command not found: %s", cmd.Name)
		return
	}

	process := exec.Command(executable, cmd.Args...)
	setupRedirection(process, cmd)

	if cmd.Background {
		err = process.Start()
		if err != nil {
			log.Printf("Error executing %s in background: %v", cmd.Name, err)
			return
		}
		log.Printf("Started %s [PID: %d] in background", cmd.Name, process.Process.Pid)
	} else {
		err = process.Run()
		if err != nil {
			log.Printf("Error executing %s: %v", cmd.Name, err)
		}
	}
}

func setupRedirection(process *exec.Cmd, cmd *Command) {
	if cmd.Redirection != nil && cmd.Redirection.Operator != "" && cmd.Redirection.File != "" {
		var file *os.File
		var err error
		if cmd.Redirection.Operator == ">" || cmd.Redirection.Operator == ">>" {
			file, err = os.OpenFile(cmd.Redirection.File, os.O_CREATE|os.O_WRONLY|(func() int {
				if cmd.Redirection.Operator == ">>" {
					return os.O_APPEND
				}
				return os.O_TRUNC
			}()), 0644)
		} else if cmd.Redirection.Operator == "<" {
			file, err = os.Open(cmd.Redirection.File)
		}
		if err != nil {
			log.Printf("Error opening file %s for redirection: %v", cmd.Redirection.File, err)
			return
		}
		switch cmd.Redirection.Operator {
		case ">":
			process.Stdout = file
		case "<":
			process.Stdin = file
		case ">>":
			process.Stdout = file
		}
	}
}

func (cmd *Command) pipeExec() {
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Pipe.Stdin = pr

	go cmd.Pipe.Run()
	cmd.execute()

	err := pw.Close()
	if err != nil {
		log.Printf("Error closing pipe writer: %v", err)
	}
	err = pr.Close()
	if err != nil {
		log.Printf("Error closing pipe reader: %v", err)
	}
}
