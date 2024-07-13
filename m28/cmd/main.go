package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mmichie/gosh/m28"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		// If no arguments are provided, start the REPL
		m28.RunREPL()
		return
	}

	commands := m28.GetCommands()
	for _, cmd := range commands {
		if cmd.Name == args[0] {
			err := cmd.Execute(args[1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// If the input is a Lisp expression, evaluate it
	if m28.IsLispExpression(strings.Join(args, " ")) {
		interpreter := m28.NewInterpreter()
		result, err := interpreter.Execute(strings.Join(args, " "))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
	fmt.Fprintf(os.Stderr, "Available commands:\n")
	for _, cmd := range commands {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", cmd.Name, cmd.Description)
	}
	os.Exit(1)
}
