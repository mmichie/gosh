package m28

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// REPL starts a Read-Eval-Print Loop for the M28 Lisp interpreter
func (i *Interpreter) REPL() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("M28 Lisp REPL")
	fmt.Println("Type 'exit' or 'quit' to exit the REPL")
	for {
		fmt.Print("m28> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "exit" || input == "quit" {
			break
		}

		result, err := i.Execute(input)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			fmt.Println("=>", result)
		}
	}
	fmt.Println("Exiting M28 Lisp REPL")
}

// RunREPL creates a new interpreter and starts the REPL
func RunREPL() {
	interpreter := NewInterpreter()
	interpreter.REPL()
}

// PrintValue converts a LispValue to a string representation
func PrintValue(val LispValue) string {
	switch v := val.(type) {
	case LispSymbol:
		return string(v)
	case float64:
		return fmt.Sprintf("%g", v)
	case LispList:
		elements := make([]string, len(v))
		for i, elem := range v {
			elements[i] = PrintValue(elem)
		}
		return "(" + strings.Join(elements, " ") + ")"
	case LispFunc:
		return "#<function>"
	case *Lambda:
		return "#<lambda>"
	default:
		return fmt.Sprintf("%v", v)
	}
}
