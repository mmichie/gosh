package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gosh/m28"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		m28.RunREPL()
		return
	}

	interpreter := m28.NewInterpreter()

	// Check if the first argument is a file
	if filepath.Ext(args[0]) == ".m28" {
		err := interpreter.ExecuteFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// If not a file, proceed with existing command handling
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

	if m28.IsLispExpression(strings.Join(args, " ")) {
		result, err := interpreter.Execute(strings.Join(args, " "))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command or invalid file: %s\n", args[0])
	fmt.Fprintf(os.Stderr, "Available commands:\n")
	for _, cmd := range commands {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", cmd.Name, cmd.Description)
	}
	fmt.Fprintf(os.Stderr, "Or provide a .m28 file to execute\n")
	os.Exit(1)
}
