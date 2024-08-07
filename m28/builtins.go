package m28

import (
	"fmt"
	"strconv"
	"strings"
)

// arithmeticHelper handles the common logic for arithmetic operations
func arithmeticHelper(args []LispValue, op string) (float64, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("'%s' expects at least one argument", op)
	}

	var result float64
	switch op {
	case "+":
		result = 0.0
	case "-":
		result = args[0].(float64) // Start with the first argument
	case "*":
		result = 1.0
	case "/":
		result = args[0].(float64) // Start with the first argument
	}

	for i, arg := range args {
		num, ok := arg.(float64)
		if !ok {
			return 0, fmt.Errorf("'%s' expects numbers, got %T", op, arg)
		}

		switch op {
		case "+":
			result += num
		case "-":
			if i == 0 {
				continue // Skip the first element as it's already assigned
			}
			result -= num
		case "*":
			result *= num
		case "/":
			if i == 0 {
				continue // Skip the first element as it's already assigned
			}
			if num == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			result /= num
		}
	}

	return result, nil
}

func add(args []LispValue, _ *Environment) (LispValue, error) {
	return arithmeticHelper(args, "+")
}

func subtract(args []LispValue, _ *Environment) (LispValue, error) {
	return arithmeticHelper(args, "-")
}

func multiply(args []LispValue, _ *Environment) (LispValue, error) {
	return arithmeticHelper(args, "*")
}

func divide(args []LispValue, _ *Environment) (LispValue, error) {
	return arithmeticHelper(args, "/")
}

func lessThan(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'<' expects at least two arguments")
	}
	prev, ok := args[0].(float64)
	if !ok {
		return nil, fmt.Errorf("'<' expects numbers, got %T", args[0])
	}
	for _, arg := range args[1:] {
		num, ok := arg.(float64)
		if !ok {
			return nil, fmt.Errorf("'<' expects numbers, got %T", arg)
		}
		if prev >= num {
			return false, nil
		}
		prev = num
	}
	return true, nil
}

func loop(args []LispValue, env *Environment) (LispValue, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("'loop' expects at least one argument")
	}
	for {
		var err error
		for _, arg := range args {
			_, err = EvalExpression(arg, env)
			if err != nil {
				return nil, err
			}
		}
	}
}

func do(args []LispValue, env *Environment) (LispValue, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("'do' expects at least three arguments")
	}

	// Parse variable bindings
	bindings, ok := args[0].(LispList)
	if !ok {
		return nil, fmt.Errorf("first argument to 'do' must be a list of bindings")
	}

	// Create a new environment for the do loop
	localEnv := NewEnvironment(env)

	// Initialize variables
	for _, binding := range bindings {
		bindingList, ok := binding.(LispList)
		if !ok || len(bindingList) < 2 {
			return nil, fmt.Errorf("invalid binding in 'do'")
		}
		symbol, ok := bindingList[0].(LispSymbol)
		if !ok {
			return nil, fmt.Errorf("binding variable must be a symbol")
		}
		initValue, err := EvalExpression(bindingList[1], localEnv)
		if err != nil {
			return nil, err
		}
		localEnv.Set(symbol, initValue)
	}

	// Parse end test and result forms
	endTest, ok := args[1].(LispList)
	if !ok || len(endTest) < 1 {
		return nil, fmt.Errorf("invalid end test in 'do'")
	}

	// Main loop
	for {
		// Check end condition
		endResult, err := EvalExpression(endTest[0], localEnv)
		if err != nil {
			return nil, err
		}
		if IsTruthy(endResult) {
			// Execute result forms and return
			if len(endTest) > 1 {
				var result LispValue
				for _, resultForm := range endTest[1:] {
					result, err = EvalExpression(resultForm, localEnv)
					if err != nil {
						return nil, err
					}
				}
				return result, nil
			}
			return nil, nil // If no result forms, return nil
		}

		// Execute body
		for _, bodyForm := range args[2:] {
			_, err := EvalExpression(bodyForm, localEnv)
			if err != nil {
				return nil, err
			}
		}

		// Update bindings
		for _, binding := range bindings {
			bindingList := binding.(LispList)
			symbol := bindingList[0].(LispSymbol)
			if len(bindingList) > 2 {
				newValue, err := EvalExpression(bindingList[2], localEnv)
				if err != nil {
					return nil, err
				}
				localEnv.Set(symbol, newValue)
			}
		}
	}
}

func when(args []LispValue, env *Environment) (LispValue, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'when' expects at least two arguments")
	}
	condition, err := EvalExpression(args[0], env)
	if err != nil {
		return nil, err
	}
	if IsTruthy(condition) {
		var result LispValue
		for _, arg := range args[1:] {
			result, err = EvalExpression(arg, env)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return nil, nil
}

func unless(args []LispValue, env *Environment) (LispValue, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'unless' expects at least two arguments")
	}
	condition, err := EvalExpression(args[0], env)
	if err != nil {
		return nil, err
	}
	if !IsTruthy(condition) {
		var result LispValue
		for _, arg := range args[1:] {
			result, err = EvalExpression(arg, env)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return nil, nil
}

func printFunc(args []LispValue, _ *Environment) (LispValue, error) {
	for _, arg := range args {
		fmt.Print(PrintValue(arg), " ")
	}
	fmt.Println()
	return nil, nil
}

func stringAppend(args []LispValue, _ *Environment) (LispValue, error) {
	var parts []string
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			parts = append(parts, v)
		case LispSymbol:
			parts = append(parts, string(v))
		default:
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return strings.Join(parts, ""), nil
}

func greaterThan(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'>' expects at least two arguments")
	}
	prev, ok := args[0].(float64)
	if !ok {
		return nil, fmt.Errorf("'>' expects numbers, got %T", args[0])
	}
	for _, arg := range args[1:] {
		num, ok := arg.(float64)
		if !ok {
			return nil, fmt.Errorf("'>' expects numbers, got %T", arg)
		}
		if prev <= num {
			return false, nil
		}
		prev = num
	}
	return true, nil
}

func numberToString(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'number->string' expects exactly one argument")
	}
	num, ok := args[0].(float64)
	if !ok {
		return nil, fmt.Errorf("'number->string' expects a number, got %T", args[0])
	}
	return strconv.FormatFloat(num, 'f', -1, 64), nil
}

func equals(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'=' expects at least two arguments")
	}
	first := args[0]
	for _, arg := range args[1:] {
		if !equalValues(first, arg) {
			return false, nil
		}
	}
	return true, nil
}

func equalValues(a, b LispValue) bool {
	switch va := a.(type) {
	case float64:
		if vb, ok := b.(float64); ok {
			return va == vb
		}
	case string:
		if vb, ok := b.(string); ok {
			return va == vb
		}
	case LispSymbol:
		if vb, ok := b.(LispSymbol); ok {
			return va == vb
		}
	case LispList:
		if vb, ok := b.(LispList); ok {
			if len(va) != len(vb) {
				return false
			}
			for i := range va {
				if !equalValues(va[i], vb[i]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func nullFunc(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'null?' expects exactly one argument")
	}
	switch arg := args[0].(type) {
	case LispList:
		return len(arg) == 0, nil
	case nil:
		return true, nil
	default:
		return false, nil
	}
}

func car(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'car' expects exactly one argument")
	}
	list, ok := args[0].(LispList)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("'car' expects a non-empty list")
	}
	return list[0], nil
}

func cdr(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'cdr' expects exactly one argument")
	}
	list, ok := args[0].(LispList)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("'cdr' expects a non-empty list")
	}
	return LispList(list[1:]), nil
}

func cons(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("'cons' expects exactly two arguments")
	}
	head, tail := args[0], args[1]
	switch t := tail.(type) {
	case LispList:
		// Prepend head to the existing list
		return LispList(append([]LispValue{head}, t...)), nil
	default:
		// Return a pair (head . tail) if tail is not a list
		return LispList{head, tail}, nil
	}
}

func and(args []LispValue, _ *Environment) (LispValue, error) {
	for _, arg := range args {
		if !IsTruthy(arg) {
			return false, nil
		}
	}
	return true, nil
}

func or(args []LispValue, _ *Environment) (LispValue, error) {
	for _, arg := range args {
		if IsTruthy(arg) {
			return true, nil
		}
	}
	return false, nil
}

func not(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'not' expects exactly one argument")
	}
	return !IsTruthy(args[0]), nil
}

func appendFunc(args []LispValue, _ *Environment) (LispValue, error) {
	var result LispList
	for _, arg := range args {
		if list, ok := arg.(LispList); ok {
			result = append(result, list...)
		} else {
			result = append(result, arg)
		}
	}
	return result, nil
}

func list(args []LispValue, _ *Environment) (LispValue, error) {
	return LispList(args), nil
}

func length(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'length' expects exactly one argument")
	}
	list, ok := args[0].(LispList)
	if !ok {
		return nil, fmt.Errorf("'length' expects a list, got %T", args[0])
	}
	return float64(len(list)), nil
}

func isNumber(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'isNumber' expects exactly one argument")
	}
	_, ok := args[0].(float64)
	return ok, nil
}

func isString(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'isString' expects exactly one argument")
	}
	_, ok := args[0].(string)
	return ok, nil
}

func isSymbol(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'isSymbol' expects exactly one argument")
	}
	_, ok := args[0].(LispSymbol)
	return ok, nil
}

func isList(args []LispValue, _ *Environment) (LispValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("'isList' expects exactly one argument")
	}
	_, ok := args[0].(LispList)
	return ok, nil
}
