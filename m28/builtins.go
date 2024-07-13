package m28

import (
	"fmt"
	"strconv"
	"strings"
)

func add(args []LispValue, _ *Environment) (LispValue, error) {
	result := 0.0
	for _, arg := range args {
		num, ok := arg.(float64)
		if !ok {
			return nil, fmt.Errorf("'+' expects numbers, got %T", arg)
		}
		result += num
	}
	return result, nil
}

func subtract(args []LispValue, _ *Environment) (LispValue, error) {
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
}

func multiply(args []LispValue, _ *Environment) (LispValue, error) {
	result := 1.0
	for _, arg := range args {
		num, ok := arg.(float64)
		if !ok {
			return nil, fmt.Errorf("'*' expects numbers, got %T", arg)
		}
		result *= num
	}
	return result, nil
}

func divide(args []LispValue, _ *Environment) (LispValue, error) {
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
