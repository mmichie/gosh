package gosh

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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

	for _, pipeline := range cmd.Pipelines {
		cmd.executePipeline(pipeline)
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)

	historyManager, err := NewHistoryManager("")
	if err != nil {
		log.Printf("Failed to create history manager: %v", err)
	} else {
		err = historyManager.Insert(cmd, 0)
		if err != nil {
			log.Printf("Failed to insert command into history: %v", err)
		}
	}
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) {
	var prevOut io.ReadCloser = nil
	var cmds []*exec.Cmd

	for i, simpleCmd := range pipeline.Commands {
		// Expand aliases
		expandedCmd := ExpandAlias(strings.Join(simpleCmd.Parts, " "))
		expandedSimpleCmd, err := parser.Parse(expandedCmd)
		if err != nil {
			log.Printf("Error parsing expanded command: %v", err)
			fmt.Fprintf(cmd.Stderr, "Error parsing expanded command: %v\n", err)
			cmd.ReturnCode = 1
			return
		}
		simpleCmd = expandedSimpleCmd.Pipelines[0].Commands[0]

		cmdName, args, redirectType, filename := parser.ProcessCommand(simpleCmd)

		if builtinCmd, ok := builtins[cmdName]; ok {
			log.Println("Executing builtin command")
			builtinCmd(cmd)
			return
		}

		execCmd := exec.Command(cmdName, args...)
		cmds = append(cmds, execCmd)

		if i == 0 {
			execCmd.Stdin = cmd.Stdin
		} else {
			execCmd.Stdin = prevOut
		}

		if i == len(pipeline.Commands)-1 {
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
			} else {
				execCmd.Stdout = cmd.Stdout
			}
		} else {
			var err error
			prevOut, err = execCmd.StdoutPipe()
			if err != nil {
				log.Printf("Error creating pipe: %v", err)
				fmt.Fprintf(cmd.Stderr, "Error creating pipe: %v\n", err)
				cmd.ReturnCode = 1
				return
			}
		}

		execCmd.Stderr = cmd.Stderr
	}

	for _, c := range cmds {
		err := c.Start()
		if err != nil {
			log.Printf("Error starting command: %v", err)
			fmt.Fprintf(cmd.Stderr, "Error starting command: %v\n", err)
			cmd.ReturnCode = 1
			return
		}
	}

	for _, c := range cmds {
		err := c.Wait()
		if err != nil {
			log.Printf("Command execution error: %v", err)
			fmt.Fprintf(cmd.Stderr, "%s: %v\n", c.Path, err)
			cmd.ReturnCode = 1
		}
	}

	if len(cmds) > 0 {
		cmd.ReturnCode = cmds[len(cmds)-1].ProcessState.ExitCode()
	}
	log.Printf("Pipeline executed, last command exit code: %d", cmd.ReturnCode)
}
