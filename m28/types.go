package m28

import "sync"

// LispValue represents any Lisp value
type LispValue interface{}

// LispSymbol represents a Lisp symbol
type LispSymbol string

// LispList represents a Lisp list
type LispList []LispValue

// LispFunc represents a Lisp function
type LispFunc func([]LispValue, *Environment) (LispValue, error)

// Lambda represents a lambda function
type Lambda struct {
	Params []LispSymbol
	Body   LispValue
	Env    *Environment
}

// Environment represents a Lisp environment
type Environment struct {
	vars  map[LispSymbol]LispValue
	outer *Environment
}

// Interpreter represents the M28 Lisp interpreter
type Interpreter struct {
	globalEnv *Environment
	envMutex  sync.RWMutex
}

// NewEnvironment creates a new environment
func NewEnvironment(outer *Environment) *Environment {
	return &Environment{
		vars:  make(map[LispSymbol]LispValue),
		outer: outer,
	}
}

// Get retrieves a value from the environment
func (e *Environment) Get(symbol LispSymbol) (LispValue, bool) {
	value, ok := e.vars[symbol]
	if !ok && e.outer != nil {
		return e.outer.Get(symbol)
	}
	return value, ok
}

// Set sets a value in the environment
func (e *Environment) Set(symbol LispSymbol, value LispValue) {
	e.vars[symbol] = value
}
