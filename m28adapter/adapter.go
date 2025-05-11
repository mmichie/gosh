// Package m28adapter provides an adapter to use the external M28 implementation
package m28adapter

import (
	"strings"
	"sync"

	"github.com/mmichie/m28/core"
	"github.com/mmichie/m28/embed"
)

// Interpreter wraps the external M28 engine
type Interpreter struct {
	engine *embed.M28Engine
	mutex  sync.RWMutex
}

// NewInterpreter creates a new M28 interpreter using the external M28 engine
func NewInterpreter() *Interpreter {
	return &Interpreter{
		engine: embed.NewM28Engine(),
	}
}

// Execute evaluates an M28 expression and returns the result as a string
func (i *Interpreter) Execute(input string) (string, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	// Execute the expression using the external M28 engine
	result, err := i.engine.Evaluate(input)
	if err != nil {
		return "", err
	}

	// Convert the LispValue to a string representation
	return core.PrintValue(result), nil
}

// ExecuteFile executes M28 code from a file
func (i *Interpreter) ExecuteFile(filename string) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	return i.engine.ExecuteFile(filename)
}

// IsLispExpression checks if a given string is a Lisp expression
func IsLispExpression(cmdString string) bool {
	trimmed := strings.TrimSpace(cmdString)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}

// StringValue converts a value to its string representation
func StringValue(val interface{}) string {
	return core.PrintValue(val)
}
