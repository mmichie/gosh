package gosh

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Aliases maps command names to their respective alias commands.
var Aliases = map[string]string{
	"ls": "ls -G",
}

// Command represents a command to be executed.
type Command struct {
	Text        string
	Parsed      []string
	Command     string
	Args        []string
	Alias       string
	CommandLine string
	TTY         string
	EUID        int
	CWD         string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	ReturnCode  int
}

// NewCommand creates a new Command instance from the given text.
func NewCommand(commandText string) *Command {
	cmd := &Command{Text: commandText}
	cmd.Parsed = parseCommand(commandText)
	if len(cmd.Parsed) == 0 {
		cmd.Command = ""
	} else {
		cmd.Command = cmd.Parsed[0]
	}
	if len(cmd.Parsed) > 1 {
		cmd.Args = cmd.Parsed[1:]
	}
	cmd.Alias = substituteAlias(cmd.Command, cmd.Args)
	cmd.TTY, _ = os.Readlink("/proc/self/fd/0")
	cmd.EUID = os.Geteuid()
	cmd.CWD, _ = os.Getwd()
	return cmd
}

// parseCommand splits the command text into parts.
func parseCommand(commandText string) []string {
	return strings.Fields(commandText)
}

// substituteAlias checks for aliases and substitutes them if found.
func substituteAlias(command string, args []string) string {
	if alias, found := Aliases[command]; found {
		return alias + " " + strings.Join(args, " ")
	}
	return strings.Join(append([]string{command}, args...), " ")
}

// Run executes the command based on whether it is a built-in or an external command.
func (cmd *Command) Run() {
	cmd.StartTime = time.Now()
	if isBuiltin(cmd.Command) {
		cmd.runBuiltin()
	} else {
		cmd.runExternal()
	}
	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
}

// isBuiltin checks if the command is a built-in shell command.
func isBuiltin(command string) bool {
	_, found := builtins[command]
	return found
}

// runBuiltin executes a built-in command.
func (cmd *Command) runBuiltin() {
	// Built-in command execution logic will be implemented here.
	if execFunc, found := builtins[cmd.Command]; found {
		execFunc(cmd)
	} else {
		log.Println("Command not recognized as a built-in")
	}
}

// runExternal executes an external command using the exec package.
func (cmd *Command) runExternal() {
	executable, err := exec.LookPath(cmd.Command)
	if err != nil {
		log.Println("Command not found:", cmd.Command)
		return
	}
	command := exec.Command(executable, cmd.Args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err != nil {
		log.Printf("Error executing %s: %v\n", cmd.CommandLine, err)
	}
}
