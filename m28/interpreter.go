package m28

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
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
	return EvalExpression(expr, i.globalEnv)
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
	env.Set(LispSymbol("print"), LispFunc(printFunc))
	env.Set(LispSymbol("string-append"), LispFunc(stringAppend))

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

// ExecuteFile reads and executes M28 Lisp code from a file
func (i *Interpreter) ExecuteFile(filename string) error {
	// Check if the file has the .m28 extension
	if filepath.Ext(filename) != ".m28" {
		return fmt.Errorf("file must have .m28 extension")
	}

	// Read the file contents
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	// Execute each expression in the file
	expressions := strings.Split(string(content), "\n")
	for _, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" || strings.HasPrefix(expr, ";") {
			continue // Skip empty lines and comments
		}
		result, err := i.Execute(expr)
		if err != nil {
			return fmt.Errorf("error executing expression '%s': %v", expr, err)
		}
		fmt.Println("=>", result)
	}

	return nil
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
	var tokens []string
	var currentToken strings.Builder
	inString := false

	for _, char := range input {
		if inString {
			currentToken.WriteRune(char)
			if char == '"' {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
				inString = false
			}
		} else if char == '"' {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			currentToken.WriteRune(char)
			inString = true
		} else if unicode.IsSpace(char) {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
		} else if char == '(' || char == ')' {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			tokens = append(tokens, string(char))
		} else {
			currentToken.WriteRune(char)
		}
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
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
	// Check if it's a string literal
	if strings.HasPrefix(token, "\"") && strings.HasSuffix(token, "\"") {
		return token[1 : len(token)-1], nil
	}

	// Check if it's a number
	if num, err := strconv.ParseFloat(token, 64); err == nil {
		return num, nil
	}

	// If it's not a string or number, it's a symbol
	return LispSymbol(token), nil
}

func EvalExpression(expr LispValue, env *Environment) (LispValue, error) {
	switch e := expr.(type) {
	case LispSymbol:
		value, ok := env.Get(e)
		if !ok {
			return nil, fmt.Errorf("undefined symbol: %s", e)
		}
		return value, nil
	case float64:
		return e, nil
	case string:
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
		fn, err := EvalExpression(first, env)
		if err != nil {
			return nil, err
		}

		args := make([]LispValue, len(e)-1)
		for i, arg := range e[1:] {
			if args[i], err = EvalExpression(arg, env); err != nil {
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
	condition, err := EvalExpression(list[1], env)
	if err != nil {
		return nil, err
	}
	if IsTruthy(condition) {
		return EvalExpression(list[2], env)
	}
	return EvalExpression(list[3], env)
}

func evalDefine(list LispList, env *Environment) (LispValue, error) {
	if len(list) != 3 {
		return nil, fmt.Errorf("'define' expects exactly two arguments")
	}
	symbol, ok := list[1].(LispSymbol)
	if !ok {
		return nil, fmt.Errorf("first argument to 'define' must be a symbol")
	}
	value, err := EvalExpression(list[2], env)
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

	return EvalExpression(lambda.Body, localEnv)
}

func evalBegin(list LispList, env *Environment) (LispValue, error) {
	if len(list) < 2 {
		return nil, fmt.Errorf("'begin' expects at least one form")
	}
	var result LispValue
	var err error
	for _, form := range list[1:] {
		result, err = EvalExpression(form, env)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func IsTruthy(v LispValue) bool {
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
