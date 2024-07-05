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
		log.Printf("Error getting current working directory: %v", err)
	}
	cmd.CWD = cwd

	for _, pipeline := range cmd.Pipelines {
		cmd.executePipeline(pipeline)
	}

	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

func (cmd *Command) executePipeline(pipeline *parser.Pipeline) {
	var prevOut io.ReadCloser = nil
	var cmds []*exec.Cmd

	lastCmd := pipeline.Commands[len(pipeline.Commands)-1]
	isBackground := false
	if len(lastCmd.Parts) > 0 && lastCmd.Parts[len(lastCmd.Parts)-1] == "&" {
		isBackground = true
		lastCmd.Parts = lastCmd.Parts[:len(lastCmd.Parts)-1]
	}

	for i, simpleCmd := range pipeline.Commands {
		cmdName, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename := parser.ProcessCommand(simpleCmd)

		if builtinCmd, ok := builtins[cmdName]; ok {
			log.Println("Executing builtin command")
			builtinCmd(cmd)
			return
		}

		execCmd := exec.Command(cmdName, args...)
		cmds = append(cmds, execCmd)

		if i == 0 {
			if inputRedirectType == "<" {
				inputFile, err := os.Open(inputFilename)
				if err != nil {
					log.Printf("Error opening input file: %v", err)
					fmt.Fprintf(cmd.Stderr, "Error opening input file: %v\n", err)
					cmd.ReturnCode = 1
					return
				}
				defer inputFile.Close()
				execCmd.Stdin = inputFile
			} else {
				execCmd.Stdin = cmd.Stdin
			}
		} else {
			execCmd.Stdin = prevOut
		}

		if i == len(pipeline.Commands)-1 {
			if outputRedirectType != "" {
				var file *os.File
				var err error

				switch outputRedirectType {
				case ">":
					file, err = os.OpenFile(outputFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				case ">>":
					file, err = os.OpenFile(outputFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				default:
					err = fmt.Errorf("unknown redirection type: %s", outputRedirectType)
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

	lastExecCmd := cmds[len(cmds)-1]

	if isBackground {
		err := lastExecCmd.Start()
		if err != nil {
			log.Printf("Error starting background command: %v", err)
			fmt.Fprintf(cmd.Stderr, "Error starting background command: %v\n", err)
			cmd.ReturnCode = 1
			return
		}
		job := cmd.JobManager.AddJob(strings.Join(lastCmd.Parts, " "), lastExecCmd)
		fmt.Printf("[%d] %d\n", job.ID, job.Cmd.Process.Pid)
		go func() {
			err := lastExecCmd.Wait()
			if err != nil {
				log.Printf("Background command execution error: %v", err)
			}
			cmd.JobManager.RemoveJob(job.ID)
		}()
	} else {
		job := cmd.JobManager.AddJob(strings.Join(lastCmd.Parts, " "), lastExecCmd)
		cmd.JobManager.SetForegroundJob(job)
		defer cmd.JobManager.SetForegroundJob(nil)

		err := lastExecCmd.Run()
		if err != nil {
			log.Printf("Command execution error: %v", err)
			fmt.Fprintf(cmd.Stderr, "%s: %v\n", lastExecCmd.Path, err)
			cmd.ReturnCode = 1
		} else {
			cmd.ReturnCode = lastExecCmd.ProcessState.ExitCode()
		}

		cmd.JobManager.RemoveJob(job.ID)
	}

	log.Printf("Pipeline executed, last command exit code: %d", cmd.ReturnCode)
}
