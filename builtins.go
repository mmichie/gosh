package gosh

import (
	"fmt"
	"os"
	"strings"
)

// Define builtins as a variable without initializing it immediately.
var builtins map[string]func(cmd *Command)

func init() {
	builtins = make(map[string]func(cmd *Command))
	builtins["cd"] = cd
	builtins["pwd"] = pwd
	builtins["exit"] = exitShell
	builtins["echo"] = echo
	// Delay adding 'help' to avoid initialization cycle
	builtins["help"] = help
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
	if len(cmd.Args) == 0 {
		os.Chdir(os.Getenv("HOME"))
	} else {
		if err := os.Chdir(cmd.Args[0]); err != nil {
			fmt.Println("cd:", err)
		}
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

// echo prints its arguments.
func echo(cmd *Command) {
	fmt.Println(strings.Join(cmd.Args, " "))
}
