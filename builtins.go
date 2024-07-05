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
	builtins["history"] = history
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
	_, args, _, _ := parser.ProcessCommand(cmd.Command)
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
	_, args, _, _ := parser.ProcessCommand(cmd.Command)
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
