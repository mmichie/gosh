package gosh

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// LispValue represents any Lisp value
type LispValue interface{}

// LispSymbol represents a Lisp symbol
type LispSymbol string

// LispList represents a Lisp list
type LispList []LispValue

// LispFunc represents a Lisp function
type LispFunc func([]LispValue, *Environment) (LispValue, error)

// Environment represents a Lisp environment
type Environment struct {
	vars  map[LispSymbol]LispValue
	outer *Environment
}

var (
	globalEnv *Environment
	envMutex  sync.RWMutex
)

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

// Parse converts a string into a LispValue
func Parse(input string) (LispValue, error) {
	tokens := tokenize(input)
	return parseTokens(&tokens)
}

func tokenize(input string) []string {
	input = strings.ReplaceAll(input, "(", " ( ")
	input = strings.ReplaceAll(input, ")", " ) ")
	return strings.Fields(input)
}

func parseTokens(tokens *[]string) (LispValue, error) {
	if len(*tokens) == 0 {
		return nil, fmt.Errorf("unexpected EOF")
	}
	token := (*tokens)[0]
	*tokens = (*tokens)[1:]

	switch token {
	case "(":
		var list LispList
		for len(*tokens) > 0 && (*tokens)[0] != ")" {
			val, err := parseTokens(tokens)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		}
		if len(*tokens) == 0 {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		*tokens = (*tokens)[1:] // consume the closing parenthesis
		return list, nil
	case ")":
		return nil, fmt.Errorf("unexpected closing parenthesis")
	default:
		return parseAtom(token)
	}
}

func parseAtom(token string) (LispValue, error) {
	if num, err := strconv.ParseFloat(token, 64); err == nil {
		return num, nil
	}
	return LispSymbol(token), nil
}

// Eval evaluates a LispValue in a given environment
func Eval(expr LispValue, env *Environment) (LispValue, error) {
	switch e := expr.(type) {
	case LispSymbol:
		value, ok := env.Get(e)
		if !ok {
			return nil, fmt.Errorf("undefined symbol: %s", e)
		}
		return value, nil
	case float64:
		return e, nil
	case LispList:
		if len(e) == 0 {
			return nil, fmt.Errorf("empty list")
		}
		first := e[0]
		if symbol, ok := first.(LispSymbol); ok && symbol == "define" {
			return evalDefine(e, env)
		}
		first, err := Eval(first, env)
		if err != nil {
			return nil, err
		}
		switch fn := first.(type) {
		case LispFunc:
			args := make([]LispValue, len(e)-1)
			for i, arg := range e[1:] {
				args[i], err = Eval(arg, env)
				if err != nil {
					return nil, err
				}
			}
			return fn(args, env)
		default:
			return nil, fmt.Errorf("not a function: %v", fn)
		}
	default:
		return nil, fmt.Errorf("unknown expression type: %T", e)
	}
}

func evalDefine(list LispList, env *Environment) (LispValue, error) {
	if len(list) != 3 {
		return nil, fmt.Errorf("'define' expects exactly two arguments")
	}
	symbol, ok := list[1].(LispSymbol)
	if !ok {
		return nil, fmt.Errorf("first argument to 'define' must be a symbol")
	}
	value, err := Eval(list[2], env)
	if err != nil {
		return nil, err
	}
	env.Set(symbol, value)
	return value, nil
}

// InitGlobalEnvironment initializes the global Lisp environment
func InitGlobalEnvironment() {
	envMutex.Lock()
	defer envMutex.Unlock()

	globalEnv = SetupGlobalEnvironment()
}

// GetGlobalEnvironment returns the global Lisp environment
func GetGlobalEnvironment() *Environment {
	envMutex.RLock()
	defer envMutex.RUnlock()

	return globalEnv
}

// SetupGlobalEnvironment creates and populates the global environment
func SetupGlobalEnvironment() *Environment {
	env := NewEnvironment(nil)

	env.Set(LispSymbol("+"), LispFunc(func(args []LispValue, _ *Environment) (LispValue, error) {
		result := 0.0
		for _, arg := range args {
			num, ok := arg.(float64)
			if !ok {
				return nil, fmt.Errorf("'+' expects numbers, got %T", arg)
			}
			result += num
		}
		return result, nil
	}))

	env.Set(LispSymbol("-"), LispFunc(func(args []LispValue, _ *Environment) (LispValue, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("'-' expects at least one argument")
		}
		first, ok := args[0].(float64)
		if !ok {
			return nil, fmt.Errorf("'-' expects numbers, got %T", args[0])
		}
		if len(args) == 1 {
			return -first, nil
		}
		for _, arg := range args[1:] {
			num, ok := arg.(float64)
			if !ok {
				return nil, fmt.Errorf("'-' expects numbers, got %T", arg)
			}
			first -= num
		}
		return first, nil
	}))

	env.Set(LispSymbol("*"), LispFunc(func(args []LispValue, _ *Environment) (LispValue, error) {
		result := 1.0
		for _, arg := range args {
			num, ok := arg.(float64)
			if !ok {
				return nil, fmt.Errorf("'*' expects numbers, got %T", arg)
			}
			result *= num
		}
		return result, nil
	}))

	env.Set(LispSymbol("/"), LispFunc(func(args []LispValue, _ *Environment) (LispValue, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("'/' expects at least two arguments")
		}
		first, ok := args[0].(float64)
		if !ok {
			return nil, fmt.Errorf("'/' expects numbers, got %T", args[0])
		}
		for _, arg := range args[1:] {
			num, ok := arg.(float64)
			if !ok {
				return nil, fmt.Errorf("'/' expects numbers, got %T", arg)
			}
			if num == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			first /= num
		}
		return first, nil
	}))

	env.Set(LispSymbol("define"), LispFunc(func(args []LispValue, env *Environment) (LispValue, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("'define' expects exactly two arguments")
		}
		symbol, ok := args[0].(LispSymbol)
		if !ok {
			return nil, fmt.Errorf("first argument to 'define' must be a symbol")
		}
		value, err := Eval(args[1], env)
		if err != nil {
			return nil, err
		}
		env.Set(symbol, value)
		return value, nil
	}))

	return env
}

// ExecuteGoshLisp takes a Gosh Lisp expression and evaluates it
func ExecuteGoshLisp(input string) (interface{}, error) {
	expr, err := Parse(input)
	if err != nil {
		return nil, err
	}

	return Eval(expr, globalLispEnv)
}

// IsLispExpression checks if a given string is a Lisp expression
func IsLispExpression(cmdString string) bool {
	trimmed := strings.TrimSpace(cmdString)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}
