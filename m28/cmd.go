package m28

import (
	"fmt"
)

// Command represents a command that can be executed
type Command struct {
	Name        string
	Description string
	Execute     func(args []string) error
}

// GetCommands returns a list of available M28 Lisp commands
func GetCommands() []Command {
	return []Command{
		{
			Name:        "repl",
			Description: "Start the M28 Lisp REPL",
			Execute: func(args []string) error {
				RunREPL()
				return nil
			},
		},
		{
			Name:        "eval",
			Description: "Evaluate a M28 Lisp expression",
			Execute: func(args []string) error {
				if len(args) == 0 {
					return fmt.Errorf("eval command requires an expression")
				}
				interpreter := NewInterpreter()
				result, err := interpreter.Execute(args[0])
				if err != nil {
					return err
				}
				fmt.Println(result)
				return nil
			},
		},
	}
}
