package gosh

import (
	"fmt"
	"os"
	"strings"

	"gosh/parser"
)

// Define builtins as a variable without initializing it immediately.
var builtins map[string]func(cmd *Command)

func init() {
	builtins = make(map[string]func(cmd *Command))
	builtins["cd"] = cd
	builtins["pwd"] = pwd
	builtins["exit"] = exitShell
	builtins["echo"] = echo
	builtins["help"] = help
	builtins["env"] = env
	builtins["export"] = export
	builtins["alias"] = alias
	builtins["unalias"] = unalias
}

// help displays help for built-in commands.
func help(cmd *Command) {
	fmt.Println("Built-in commands:")
	for name := range builtins {
		fmt.Printf("  %s\n", name)
	}
}

// cd changes the current working directory.
func cd(cmd *Command) {
	if len(cmd.Pipelines) == 0 || len(cmd.Pipelines[0].Commands) == 0 {
		fmt.Println("cd: no arguments")
		return
	}
	_, args, _, _ := parser.ProcessCommand(cmd.Pipelines[0].Commands[0])
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

// pwd prints the current working directory.
func pwd(cmd *Command) {
	if dir, err := os.Getwd(); err == nil {
		fmt.Println(dir)
	} else {
		fmt.Println("pwd:", err)
	}
}

// exitShell exits the shell.
func exitShell(cmd *Command) {
	os.Exit(0)
}

func echo(cmd *Command) {
	if len(cmd.Pipelines) == 0 || len(cmd.Pipelines[0].Commands) == 0 {
		return
	}
	_, args, _, _ := parser.ProcessCommand(cmd.Pipelines[0].Commands[0])
	fmt.Println(strings.Join(args, " "))
}

// history dumps the command history.
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
	if len(cmd.Pipelines) == 0 || len(cmd.Pipelines[0].Commands) == 0 || len(cmd.Pipelines[0].Commands[0].Parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: export NAME=VALUE")
		return
	}

	assignment := cmd.Pipelines[0].Commands[0].Parts[1]
	parts := strings.SplitN(assignment, "=", 2)
	if len(parts) != 2 {
		fmt.Fprintln(cmd.Stderr, "Invalid export syntax. Usage: export NAME=VALUE")
		return
	}

	name, value := parts[0], parts[1]
	os.Setenv(name, value)
}

func alias(cmd *Command) {
	if len(cmd.Pipelines) == 0 || len(cmd.Pipelines[0].Commands) == 0 {
		// List all aliases
		for _, a := range ListAliases() {
			fmt.Println(a)
		}
		return
	}

	parts := cmd.Pipelines[0].Commands[0].Parts
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
	if len(cmd.Pipelines) == 0 || len(cmd.Pipelines[0].Commands) == 0 || len(cmd.Pipelines[0].Commands[0].Parts) < 2 {
		fmt.Fprintln(cmd.Stderr, "Usage: unalias name")
		return
	}

	name := cmd.Pipelines[0].Commands[0].Parts[1]
	RemoveAlias(name)
}
