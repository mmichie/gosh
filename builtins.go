package gosh

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gosh/parser"
)

var builtins map[string]func(cmd *Command)

func init() {
	builtins = make(map[string]func(cmd *Command))
	builtins["cd"] = cd
	builtins["pwd"] = pwd
	builtins["exit"] = exitShell
	builtins["echo"] = echo
	builtins["help"] = help
	builtins["history"] = history
	builtins["env"] = env
	builtins["export"] = export
	builtins["alias"] = alias
	builtins["unalias"] = unalias
	builtins["jobs"] = jobs
	builtins["fg"] = fg
	builtins["bg"] = bg
}

// Builtins returns a copy of the builtins map
func Builtins() map[string]func(cmd *Command) {
	copy := make(map[string]func(cmd *Command))
	for k, v := range builtins {
		copy[k] = v
	}
	return copy
}

func help(cmd *Command) {
	fmt.Println("Built-in commands:")
	for name := range builtins {
		fmt.Printf("  %s\n", name)
	}
}

func cd(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		fmt.Println("cd: no arguments")
		return
	}
	_, args, _, _, _, _ := parser.ProcessCommand(cmd.AndCommands[0].Pipelines[0].Commands[0])
	var dir string
	if len(args) == 0 {
		dir = os.Getenv("HOME")
	} else {
		dir = args[0]
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Println("cd:", err)
	}
}

func pwd(cmd *Command) {
	if dir, err := os.Getwd(); err == nil {
		fmt.Println(dir)
	} else {
		fmt.Println("pwd:", err)
	}
}

func exitShell(cmd *Command) {
	os.Exit(0)
}

func echo(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		return
	}
	_, args, _, _, _, _ := parser.ProcessCommand(cmd.AndCommands[0].Pipelines[0].Commands[0])
	output := strings.Join(args, " ")
	fmt.Fprintln(cmd.Stdout, output)
}

func history(cmd *Command) {
	historyManager, err := NewHistoryManager("")
	if err != nil {
		fmt.Println("Failed to open history database:", err)
		return
	}
	records, err := historyManager.Dump()
	if err != nil {
		fmt.Println("Error retrieving history:", err)
		return
	}
	for _, record := range records {
		fmt.Println(record)
	}
}

func env(cmd *Command) {
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
}

func export(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: export NAME=VALUE")
		return
	}

	assignment := cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1]
	parts := strings.SplitN(assignment, "=", 2)
	if len(parts) != 2 {
		fmt.Fprintln(cmd.Stderr, "Invalid export syntax. Usage: export NAME=VALUE")
		return
	}

	name, value := parts[0], parts[1]
	os.Setenv(name, value)
}

func alias(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		// List all aliases
		for _, a := range ListAliases() {
			fmt.Println(a)
		}
		return
	}

	parts := cmd.AndCommands[0].Pipelines[0].Commands[0].Parts
	if len(parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: alias name='command'")
		return
	}

	aliasDeclaration := strings.Join(parts[1:], " ")
	nameParts := strings.SplitN(aliasDeclaration, "=", 2)
	if len(nameParts) != 2 {
		fmt.Fprintln(cmd.Stderr, "Invalid alias syntax. Usage: alias name='command'")
		return
	}

	name := strings.TrimSpace(nameParts[0])
	command := strings.Trim(strings.TrimSpace(nameParts[1]), "'\"")
	SetAlias(name, command)
}

func unalias(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: unalias name")
		return
	}

	name := cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1]
	RemoveAlias(name)
}

func jobs(cmd *Command) {
	jobList := cmd.JobManager.ListJobs()
	for _, job := range jobList {
		fmt.Printf("[%d] %s %s\n", job.ID, job.Status, job.Command)
	}
}

func fg(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: fg <job_id>")
		return
	}
	jobID, err := strconv.Atoi(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1])
	if err != nil {
		fmt.Fprintln(cmd.Stderr, "Invalid job ID")
		return
	}
	err = cmd.JobManager.ForegroundJob(jobID)
	if err != nil {
		fmt.Fprintln(cmd.Stderr, err)
	}
}

func bg(cmd *Command) {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: bg <job_id>")
		return
	}
	jobID, err := strconv.Atoi(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1])
	if err != nil {
		fmt.Fprintln(cmd.Stderr, "Invalid job ID")
		return
	}
	err = cmd.JobManager.BackgroundJob(jobID)
	if err != nil {
		fmt.Fprintln(cmd.Stderr, err)
	}
}
