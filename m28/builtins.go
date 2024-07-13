package m28

import (
	"fmt"
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
			_, err = eval(arg, env)
			if err != nil {
				return nil, err
			}
		}
	}
}

func do(args []LispValue, env *Environment) (LispValue, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'do' expects at least two arguments")
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
		initValue, err := eval(bindingList[1], localEnv)
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
		endResult, err := eval(endTest[0], localEnv)
		if err != nil {
			return nil, err
		}
		if isTruthy(endResult) {
			// Execute result forms and return
			var result LispValue
			for _, resultForm := range endTest[1:] {
				result, err = eval(resultForm, localEnv)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		}

		// Execute body
		for _, bodyForm := range args[2:] {
			_, err := eval(bodyForm, localEnv)
			if err != nil {
				return nil, err
			}
		}

		// Update bindings
		for _, binding := range bindings {
			bindingList := binding.(LispList)
			symbol := bindingList[0].(LispSymbol)
			if len(bindingList) > 2 {
				newValue, err := eval(bindingList[2], localEnv)
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
	condition, err := eval(args[0], env)
	if err != nil {
		return nil, err
	}
	if isTruthy(condition) {
		var result LispValue
		for _, arg := range args[1:] {
			result, err = eval(arg, env)
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
	condition, err := eval(args[0], env)
	if err != nil {
		return nil, err
	}
	if !isTruthy(condition) {
		var result LispValue
		for _, arg := range args[1:] {
			result, err = eval(arg, env)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return nil, nil
}