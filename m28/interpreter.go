package m28

import (
	"fmt"
	"strconv"
	"strings"
)

// NewInterpreter creates a new M28 Lisp interpreter
func NewInterpreter() *Interpreter {
	return &Interpreter{
		globalEnv: SetupGlobalEnvironment(),
	}
}

// Eval evaluates a LispValue in the interpreter's environment
func (i *Interpreter) Eval(expr LispValue) (LispValue, error) {
	i.envMutex.RLock()
	defer i.envMutex.RUnlock()
	return eval(expr, i.globalEnv)
}

// SetupGlobalEnvironment creates and populates the global environment
func SetupGlobalEnvironment() *Environment {
	env := NewEnvironment(nil)

	// Add basic arithmetic operations
	env.Set(LispSymbol("+"), LispFunc(add))
	env.Set(LispSymbol("-"), LispFunc(subtract))
	env.Set(LispSymbol("*"), LispFunc(multiply))
	env.Set(LispSymbol("/"), LispFunc(divide))
	env.Set(LispSymbol("<"), LispFunc(lessThan))

	// Add control structures
	env.Set(LispSymbol("loop"), LispFunc(loop))
	env.Set(LispSymbol("do"), LispFunc(do))
	env.Set(LispSymbol("when"), LispFunc(when))
	env.Set(LispSymbol("unless"), LispFunc(unless))

	return env
}

// Parse converts a string into a LispValue
func (i *Interpreter) Parse(input string) (LispValue, error) {
	return parse(input)
}

// Execute parses and evaluates a M28 Lisp expression
func (i *Interpreter) Execute(input string) (string, error) {
	expr, err := i.Parse(input)
	if err != nil {
		return "", err
	}
	result, err := i.Eval(expr)
	if err != nil {
		return "", err
	}
	return PrintValue(result), nil
}

// IsLispExpression checks if a given string is a Lisp expression
func IsLispExpression(cmdString string) bool {
	trimmed := strings.TrimSpace(cmdString)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}

func parse(input string) (LispValue, error) {
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

func eval(expr LispValue, env *Environment) (LispValue, error) {
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
			case "quote":
				if len(e) != 2 {
					return nil, fmt.Errorf("'quote' expects exactly one argument")
				}
				return e[1], nil
			case "if":
				return evalIf(e, env)
			case "define":
				return evalDefine(e, env)
			case "lambda":
				return evalLambda(e, env)
			case "begin":
				return evalBegin(e, env)
			}
		}

		// Function application
		fn, err := eval(first, env)
		if err != nil {
			return nil, err
		}

		args := make([]LispValue, len(e)-1)
		for i, arg := range e[1:] {
			if args[i], err = eval(arg, env); err != nil {
				return nil, err
			}
		}

		switch fn := fn.(type) {
		case LispFunc:
			return fn(args, env)
		case *Lambda:
			return callLambda(fn, args, env)
		default:
			return nil, fmt.Errorf("not a function: %v", fn)
		}
	default:
		return nil, fmt.Errorf("unknown expression type: %T", e)
	}
}

func evalIf(list LispList, env *Environment) (LispValue, error) {
	if len(list) != 4 {
		return nil, fmt.Errorf("'if' expects exactly three arguments")
	}
	condition, err := eval(list[1], env)
	if err != nil {
		return nil, err
	}
	if isTruthy(condition) {
		return eval(list[2], env)
	}
	return eval(list[3], env)
}

func evalDefine(list LispList, env *Environment) (LispValue, error) {
	if len(list) != 3 {
		return nil, fmt.Errorf("'define' expects exactly two arguments")
	}
	symbol, ok := list[1].(LispSymbol)
	if !ok {
		return nil, fmt.Errorf("first argument to 'define' must be a symbol")
	}
	value, err := eval(list[2], env)
	if err != nil {
		return nil, err
	}
	env.Set(symbol, value)
	return value, nil
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
		localEnv.Set(param, args[i])
	}

	return eval(lambda.Body, localEnv)
}

func evalBegin(list LispList, env *Environment) (LispValue, error) {
	if len(list) < 2 {
		return nil, fmt.Errorf("'begin' expects at least one form")
	}
	var result LispValue
	var err error
	for _, form := range list[1:] {
		result, err = eval(form, env)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
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
