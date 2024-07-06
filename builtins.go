package gosh

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gosh/parser"
)

var builtins map[string]func(cmd *Command) error

func init() {
	builtins = make(map[string]func(cmd *Command) error)
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

func cd(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		return fmt.Errorf("cd: no arguments")
	}
	_, args, _, _, _, _ := parser.ProcessCommand(cmd.AndCommands[0].Pipelines[0].Commands[0])
	var dir string
	if len(args) == 0 {
		dir = os.Getenv("HOME")
	} else {
		dir = args[0]
	}
	return os.Chdir(dir)
}

func pwd(cmd *Command) error {
	dir, err := os.Getwd()
	if err == nil {
		_, err = fmt.Fprintln(cmd.Stdout, dir)
	}
	return err
}

func exitShell(cmd *Command) error {
	os.Exit(0)
	return nil
}

func echo(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		return nil
	}
	_, args, _, _, _, _ := parser.ProcessCommand(cmd.AndCommands[0].Pipelines[0].Commands[0])
	output := strings.Join(args, " ") + "\n"
	_, err := fmt.Fprint(cmd.Stdout, output)
	return err
}

func help(cmd *Command) error {
	_, err := fmt.Fprintln(cmd.Stdout, "Built-in commands:")
	if err != nil {
		return err
	}
	for name := range builtins {
		_, err = fmt.Fprintf(cmd.Stdout, "  %s\n", name)
		if err != nil {
			return err
		}
	}
	return nil
}

func history(cmd *Command) error {
	historyManager, err := NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("Failed to open history database: %v", err)
	}
	records, err := historyManager.Dump()
	if err != nil {
		return fmt.Errorf("Error retrieving history: %v", err)
	}
	for _, record := range records {
		_, err = fmt.Fprintln(cmd.Stdout, record)
		if err != nil {
			return err
		}
	}
	return nil
}

func env(cmd *Command) error {
	for _, env := range os.Environ() {
		_, err := fmt.Fprintln(cmd.Stdout, env)
		if err != nil {
			return err
		}
	}
	return nil
}

func export(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		return fmt.Errorf("Usage: export NAME=VALUE")
	}

	assignment := cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1]
	parts := strings.SplitN(assignment, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Invalid export syntax. Usage: export NAME=VALUE")
	}

	name, value := parts[0], parts[1]
	return os.Setenv(name, value)
}

func alias(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		// List all aliases
		for _, a := range ListAliases() {
			_, err := fmt.Fprintln(cmd.Stdout, a)
			if err != nil {
				return err
			}
		}
		return nil
	}

	parts := cmd.AndCommands[0].Pipelines[0].Commands[0].Parts
	if len(parts) < 2 {
		return fmt.Errorf("Usage: alias name='command'")
	}

	aliasDeclaration := strings.Join(parts[1:], " ")
	nameParts := strings.SplitN(aliasDeclaration, "=", 2)
	if len(nameParts) != 2 {
		return fmt.Errorf("Invalid alias syntax. Usage: alias name='command'")
	}

	name := strings.TrimSpace(nameParts[0])
	command := strings.Trim(strings.TrimSpace(nameParts[1]), "'\"")
	SetAlias(name, command)
	return nil
}

func unalias(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		return fmt.Errorf("Usage: unalias name")
	}

	name := cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1]
	RemoveAlias(name)
	return nil
}

func jobs(cmd *Command) error {
	jobList := cmd.JobManager.ListJobs()
	for _, job := range jobList {
		_, err := fmt.Fprintf(cmd.Stdout, "[%d] %s %s\n", job.ID, job.Status, job.Command)
		if err != nil {
			return err
		}
	}
	return nil
}

func fg(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		return fmt.Errorf("Usage: fg <job_id>")
	}
	jobID, err := strconv.Atoi(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1])
	if err != nil {
		return fmt.Errorf("Invalid job ID")
	}
	return cmd.JobManager.ForegroundJob(jobID)
}

func bg(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts) < 2 {
		return fmt.Errorf("Usage: bg <job_id>")
	}
	jobID, err := strconv.Atoi(cmd.AndCommands[0].Pipelines[0].Commands[0].Parts[1])
	if err != nil {
		return fmt.Errorf("Invalid job ID")
	}
	return cmd.JobManager.BackgroundJob(jobID)
}

// Builtins returns a copy of the builtins map
func Builtins() map[string]func(cmd *Command) error {
	copy := make(map[string]func(cmd *Command) error)
	for k, v := range builtins {
		copy[k] = v
	}
	return copy
}
