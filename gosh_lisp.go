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

		// Handle special forms
		if symbol, ok := first.(LispSymbol); ok {
			switch symbol {
			case "define":
				return evalDefine(e, env)
			case "if":
				return evalIf(e, env)
			case "lambda":
				return evalLambda(e, env)
			case "quote":
				if len(e) != 2 {
					return nil, fmt.Errorf("'quote' expects exactly one argument")
				}
				return e[1], nil
			case "set!":
				if len(e) != 3 {
					return nil, fmt.Errorf("'set!' expects exactly two arguments")
				}
				symbol, ok := e[1].(LispSymbol)
				if !ok {
					return nil, fmt.Errorf("first argument to 'set!' must be a symbol")
				}
				value, err := Eval(e[2], env)
				if err != nil {
					return nil, err
				}
				if _, ok := env.Get(symbol); !ok {
					return nil, fmt.Errorf("undefined symbol: %s", symbol)
				}
				env.Set(symbol, value)
				return value, nil
			case "begin":
				var lastVal LispValue
				for _, expr := range e[1:] {
					var err error
					lastVal, err = Eval(expr, env)
					if err != nil {
						return nil, err
					}
				}
				return lastVal, nil
			}
		}

		// Evaluate the first element
		fn, err := Eval(first, env)
		if err != nil {
			return nil, err
		}

		switch fn := fn.(type) {
		case LispFunc:
			return fn(e[1:], env)
		case *Lambda:
			return callLambda(fn, e[1:], env)
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

func evalIf(list LispList, env *Environment) (LispValue, error) {
	if len(list) != 4 {
		return nil, fmt.Errorf("'if' expects exactly three arguments")
	}
	condition, err := Eval(list[1], env)
	if err != nil {
		return nil, err
	}
	if isTruthy(condition) {
		return Eval(list[2], env)
	}
	return Eval(list[3], env)
}

func isTruthy(v LispValue) bool {
	switch v := v.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != ""
	case LispList:
		return len(v) > 0
	default:
		return true
	}
}

func evalLambda(list LispList, env *Environment) (LispValue, error) {
	if len(list) != 3 {
		return nil, fmt.Errorf("'lambda' expects exactly two arguments")
	}
	params, ok := list[1].(LispList)
	if !ok {
		return nil, fmt.Errorf("lambda parameters must be a list")
	}
	var paramSymbols []LispSymbol
	for _, param := range params {
		symbol, ok := param.(LispSymbol)
		if !ok {
			return nil, fmt.Errorf("lambda parameter must be a symbol")
		}
		paramSymbols = append(paramSymbols, symbol)
	}
	return &Lambda{
		Params: paramSymbols,
		Body:   list[2],
		Env:    env,
	}, nil
}

func callLambda(lambda *Lambda, args []LispValue, env *Environment) (LispValue, error) {
	if len(args) != len(lambda.Params) {
		return nil, fmt.Errorf("lambda called with wrong number of arguments")
	}

	localEnv := NewEnvironment(lambda.Env)
	for i, param := range lambda.Params {
		value, err := Eval(args[i], env)
		if err != nil {
			return nil, err
		}
		localEnv.Set(param, value)
	}

	return Eval(lambda.Body, localEnv)
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

	env.Set(LispSymbol("+"), LispFunc(func(args []LispValue, env *Environment) (LispValue, error) {
		result := 0.0
		for _, arg := range args {
			evaluated, err := Eval(arg, env)
			if err != nil {
				return nil, err
			}
			num, ok := evaluated.(float64)
			if !ok {
				return nil, fmt.Errorf("'+' expects numbers, got %T", evaluated)
			}
			result += num
		}
		return result, nil
	}))

	env.Set(LispSymbol("-"), LispFunc(func(args []LispValue, env *Environment) (LispValue, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("'-' expects at least one argument")
		}
		first, err := Eval(args[0], env)
		if err != nil {
			return nil, err
		}
		firstNum, ok := first.(float64)
		if !ok {
			return nil, fmt.Errorf("'-' expects numbers, got %T", first)
		}
		if len(args) == 1 {
			return -firstNum, nil
		}
		for _, arg := range args[1:] {
			evaluated, err := Eval(arg, env)
			if err != nil {
				return nil, err
			}
			num, ok := evaluated.(float64)
			if !ok {
				return nil, fmt.Errorf("'-' expects numbers, got %T", evaluated)
			}
			firstNum -= num
		}
		return firstNum, nil
	}))

	env.Set(LispSymbol("*"), LispFunc(func(args []LispValue, env *Environment) (LispValue, error) {
		result := 1.0
		for _, arg := range args {
			evaluated, err := Eval(arg, env)
			if err != nil {
				return nil, err
			}
			num, ok := evaluated.(float64)
			if !ok {
				return nil, fmt.Errorf("'*' expects numbers, got %T", evaluated)
			}
			result *= num
		}
		return result, nil
	}))

	env.Set(LispSymbol("/"), LispFunc(func(args []LispValue, env *Environment) (LispValue, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("'/' expects at least two arguments")
		}
		first, err := Eval(args[0], env)
		if err != nil {
			return nil, err
		}
		firstNum, ok := first.(float64)
		if !ok {
			return nil, fmt.Errorf("'/' expects numbers, got %T", first)
		}
		for _, arg := range args[1:] {
			evaluated, err := Eval(arg, env)
			if err != nil {
				return nil, err
			}
			num, ok := evaluated.(float64)
			if !ok {
				return nil, fmt.Errorf("'/' expects numbers, got %T", evaluated)
			}
			if num == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			firstNum /= num
		}
		return firstNum, nil
	}))

	env.Set(LispSymbol("<"), LispFunc(func(args []LispValue, env *Environment) (LispValue, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("'<' expects at least two arguments")
		}
		var prev float64
		for i, arg := range args {
			evaluated, err := Eval(arg, env)
			if err != nil {
				return nil, err
			}
			num, ok := evaluated.(float64)
			if !ok {
				return nil, fmt.Errorf("'<' expects numbers, got %T", evaluated)
			}
			if i > 0 && prev >= num {
				return false, nil
			}
			prev = num
		}
		return true, nil
	}))

	return env
}

// ExecuteGoshLisp takes a Gosh Lisp expression and evaluates it
func ExecuteGoshLisp(input string) (interface{}, error) {
	expr, err := Parse(input)
	if err != nil {
		return nil, err
	}

	envMutex.Lock()
	defer envMutex.Unlock()

	if globalEnv == nil {
		globalEnv = SetupGlobalEnvironment()
	}

	return Eval(expr, globalEnv)
}

// IsLispExpression checks if a given string is a Lisp expression
func IsLispExpression(cmdString string) bool {
	trimmed := strings.TrimSpace(cmdString)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}
