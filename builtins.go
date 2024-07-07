package gosh

import (
	"fmt"
	"log"
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
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("cd error: failed to get current directory: %v", err)
		return err
	}

	var targetDir string

	// Assuming we always deal with the first command and its parts
	if len(cmd.AndCommands) > 0 && len(cmd.AndCommands[0].Pipelines) > 0 && len(cmd.AndCommands[0].Pipelines[0].Commands) > 0 {
		firstCommand := cmd.AndCommands[0].Pipelines[0].Commands[0]
		if len(firstCommand.Parts) > 1 {
			targetDir = firstCommand.Parts[1] // Getting the first argument
		}
	}

	if targetDir == "" {
		targetDir = os.Getenv("HOME") // Default to HOME if no argument given
	} else if targetDir == "-" {
		previousDir := os.Getenv("PREVIOUS_DIR")
		if previousDir == "" {
			log.Printf("cd error: no previous directory to return to")
			return fmt.Errorf("cd: no previous directory")
		}
		targetDir = previousDir
	}

	if targetDir != currentDir {
		err = os.Chdir(targetDir)
		if err != nil {
			log.Printf("cd error: unable to change directory to %s: %v", targetDir, err)
			return fmt.Errorf("cd: %v", err)
		}

		// Update the environment variable for the previous directory
		os.Setenv("PREVIOUS_DIR", currentDir)
		log.Printf("cd: changed from %s to %s. Previous directory updated to %s", currentDir, targetDir, currentDir)
	} else {
		log.Printf("cd: no change needed, already in %s", currentDir)
	}

	return nil
}

func pwd(cmd *Command) error {
	_, err := fmt.Fprintln(cmd.Stdout, cmd.State.CWD)
	return err
}

func echo(cmd *Command) error {
	if len(cmd.AndCommands) == 0 || len(cmd.AndCommands[0].Pipelines) == 0 || len(cmd.AndCommands[0].Pipelines[0].Commands) == 0 {
		return nil
	}
	_, args, _, _, _, _ := parser.ProcessCommand(cmd.AndCommands[0].Pipelines[0].Commands[0])

	// Remove quotes and expand environment variables
	for i, arg := range args {
		arg = strings.Trim(arg, "'\"")
		if strings.HasPrefix(arg, "$") {
			varName := strings.TrimPrefix(arg, "$")
			args[i] = os.Getenv(varName)
		} else {
			args[i] = arg
		}
	}

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
	err := os.Setenv(name, value)
	if err != nil {
		return fmt.Errorf("export: %v", err)
	}

	_, err = fmt.Fprintf(cmd.Stdout, "export %s=%s\n", name, value)
	return err
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

func exitShell(cmd *Command) error {
	os.Exit(0)
	return nil
}
