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

// SetupGlobalEnvironment creates and populates the global environment
func SetupGlobalEnvironment() *Environment {
	env := NewEnvironment(nil)

	// Add basic arithmetic operations
	env.Set(LispSymbol("+"), LispFunc(add))
	env.Set(LispSymbol("-"), LispFunc(subtract))
	env.Set(LispSymbol("*"), LispFunc(multiply))
	env.Set(LispSymbol("/"), LispFunc(divide))
	env.Set(LispSymbol("<"), LispFunc(lessThan))
	env.Set(LispSymbol(">"), LispFunc(greaterThan))
	env.Set(LispSymbol("="), LispFunc(equals))
	env.Set(LispSymbol("nil"), nil)

	// Add control structures
	env.Set(LispSymbol("if"), LispFunc(ifFunc))
	env.Set(LispSymbol("define"), LispFunc(defineFunc))
	env.Set(LispSymbol("lambda"), LispFunc(lambdaFunc))
	env.Set(LispSymbol("begin"), LispFunc(beginFunc))
	env.Set(LispSymbol("quote"), LispFunc(quoteFunc))
	env.Set(LispSymbol("do"), LispFunc(do))

	// Add utility functions
	env.Set(LispSymbol("print"), LispFunc(printFunc))
	env.Set(LispSymbol("string-append"), LispFunc(stringAppend))
	env.Set(LispSymbol("number->string"), LispFunc(numberToString))

	return env
}

// Helper functions for special forms
func ifFunc(args []LispValue, env *Environment) (LispValue, error) {
	return evalIf(LispList(append([]LispValue{LispSymbol("if")}, args...)), env)
}

func defineFunc(args []LispValue, env *Environment) (LispValue, error) {
	return evalDefine(LispList(append([]LispValue{LispSymbol("define")}, args...)), env)
}

func lambdaFunc(args []LispValue, env *Environment) (LispValue, error) {
	return evalLambda(LispList(append([]LispValue{LispSymbol("lambda")}, args...)), env)
}

func beginFunc(args []LispValue, env *Environment) (LispValue, error) {
	return evalBegin(LispList(append([]LispValue{LispSymbol("begin")}, args...)), env)
}

func quoteFunc(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'quote' expects exactly one argument")
	}
	return args[0], nil
}

// Eval evaluates a LispValue in the interpreter's environment
func (i *Interpreter) Eval(expr LispValue) (LispValue, error) {
	i.envMutex.RLock()
	defer i.envMutex.RUnlock()
	return EvalExpression(expr, i.globalEnv)
}

// Parse converts a string into a LispValue
func (i *Interpreter) Parse(input string) (LispValue, error) {
	tokens := tokenize(input)
	expressions, err := parseMultiple(&tokens)
	if err != nil {
		return nil, err
	}
	if len(expressions) == 1 {
		return expressions[0], nil
	}
	return LispList(expressions), nil
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

	// Parse the entire file content
	expressions, err := i.Parse(string(content))
	if err != nil {
		return fmt.Errorf("error parsing file: %v", err)
	}

	// Evaluate each expression
	var result LispValue
	switch expr := expressions.(type) {
	case LispList:
		for _, e := range expr {
			result, err = i.Eval(e)
			if err != nil {
				return fmt.Errorf("error evaluating expression: %v", err)
			}
			// Only print non-nil results
			if result != nil {
				fmt.Println("=>", PrintValue(result))
			}
		}
	default:
		result, err = i.Eval(expr)
		if err != nil {
			return fmt.Errorf("error evaluating expression: %v", err)
		}
		if result != nil {
			fmt.Println("=>", PrintValue(result))
		}
	}

	return nil
}

// IsLispExpression checks if a given string is a Lisp expression
func IsLispExpression(cmdString string) bool {
	trimmed := strings.TrimSpace(cmdString)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}

func parseMultiple(tokens *[]string) ([]LispValue, error) {
	var expressions []LispValue
	for len(*tokens) > 0 {
		expr, err := parseTokens(tokens)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, expr)
	}
	return expressions, nil
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

	switch {
	case token == "(":
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
	case token == ")":
		return nil, fmt.Errorf("unexpected closing parenthesis")
	case token == "'":
		quoted, err := parseTokens(tokens)
		if err != nil {
			return nil, err
		}
		return LispList{LispSymbol("quote"), quoted}, nil
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
			case "do":
				return do(e[1:], env)
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
	case nil:
		return false
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
