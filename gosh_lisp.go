package gosh

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Token represents a lexical token
type Token struct {
	Type  string
	Value string
}

// Lexer breaks input string into tokens
func Lexer(input string) []Token {
	var tokens []Token
	var current strings.Builder

	for _, ch := range input {
		if ch == '(' || ch == ')' {
			if current.Len() > 0 {
				tokens = append(tokens, classifyToken(current.String()))
				current.Reset()
			}
			tokens = append(tokens, Token{Type: "paren", Value: string(ch)})
		} else if unicode.IsSpace(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, classifyToken(current.String()))
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, classifyToken(current.String()))
	}

	return tokens
}

func classifyToken(token string) Token {
	if _, err := strconv.ParseFloat(token, 64); err == nil {
		return Token{Type: "number", Value: token}
	}
	return Token{Type: "identifier", Value: token}
}

// Node represents a node in the abstract syntax tree
type Node struct {
	Type     string
	Value    string
	Children []Node
}

// Parser converts tokens into an abstract syntax tree
func Parser(tokens []Token) (Node, error) {
	root := Node{Type: "root", Children: []Node{}}
	current := &root
	stack := []*Node{}

	for _, token := range tokens {
		if token.Type == "paren" && token.Value == "(" {
			newNode := Node{Type: "expression"}
			current.Children = append(current.Children, newNode)
			stack = append(stack, current)
			current = &current.Children[len(current.Children)-1]
		} else if token.Type == "paren" && token.Value == ")" {
			if len(stack) == 0 {
				return Node{}, fmt.Errorf("unexpected closing parenthesis")
			}
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		} else {
			current.Children = append(current.Children, Node{Type: token.Type, Value: token.Value})
		}
	}

	if len(stack) > 0 {
		return Node{}, fmt.Errorf("missing closing parenthesis")
	}

	return root, nil
}

// Evaluate interprets the AST
func Evaluate(node Node) (interface{}, error) {
	switch node.Type {
	case "root":
		if len(node.Children) != 1 {
			return nil, fmt.Errorf("expected single expression, got %d", len(node.Children))
		}
		return Evaluate(node.Children[0])
	case "number":
		return strconv.ParseFloat(node.Value, 64)
	case "identifier":
		return node.Value, nil
	case "expression":
		if len(node.Children) == 0 {
			return nil, nil
		}
		operator, err := Evaluate(node.Children[0])
		if err != nil {
			return nil, err
		}
		operatorStr, ok := operator.(string)
		if !ok {
			return nil, fmt.Errorf("operator must be a string")
		}
		args := make([]interface{}, len(node.Children)-1)
		for i, child := range node.Children[1:] {
			arg, err := Evaluate(child)
			if err != nil {
				return nil, err
			}
			args[i] = arg
		}
		return EvaluateFunction(operatorStr, args)
	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
}

// EvaluateFunction applies the operator to the arguments
func EvaluateFunction(operator string, args []interface{}) (interface{}, error) {
	switch operator {
	case "+":
		return add(args)
	case "-":
		return subtract(args)
	case "*":
		return multiply(args)
	case "/":
		return divide(args)
	case "=":
		return equal(args)
	case "<":
		return lessThan(args)
	case ">":
		return greaterThan(args)
	case "<=":
		return lessThanOrEqual(args)
	case ">=":
		return greaterThanOrEqual(args)
	default:
		return nil, fmt.Errorf("unknown operator: %s", operator)
	}
}

func add(args []interface{}) (float64, error) {
	result := 0.0
	for _, arg := range args {
		num, err := toFloat64(arg)
		if err != nil {
			return 0, err
		}
		result += num
	}
	return result, nil
}

func subtract(args []interface{}) (float64, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("'-' operator expects at least one argument")
	}
	result, err := toFloat64(args[0])
	if err != nil {
		return 0, err
	}
	if len(args) == 1 {
		return -result, nil
	}
	for _, arg := range args[1:] {
		num, err := toFloat64(arg)
		if err != nil {
			return 0, err
		}
		result -= num
	}
	return result, nil
}

func multiply(args []interface{}) (float64, error) {
	result := 1.0
	for _, arg := range args {
		num, err := toFloat64(arg)
		if err != nil {
			return 0, err
		}
		result *= num
	}
	return result, nil
}

func divide(args []interface{}) (float64, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("'/' operator expects at least two arguments")
	}
	result, err := toFloat64(args[0])
	if err != nil {
		return 0, err
	}
	for i, arg := range args[1:] {
		num, err := toFloat64(arg)
		if err != nil {
			return 0, err
		}
		if num == 0 {
			return 0, fmt.Errorf("division by zero (at argument %d)", i+2)
		}
		result /= num
	}
	return result, nil
}

func equal(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("'=' operator expects at least two arguments")
	}
	first := args[0]
	for _, arg := range args[1:] {
		if arg != first {
			return false, nil
		}
	}
	return true, nil
}

func lessThan(args []interface{}) (interface{}, error) {
	return compareNumbers(args, func(a, b float64) bool { return a < b })
}

func greaterThan(args []interface{}) (interface{}, error) {
	return compareNumbers(args, func(a, b float64) bool { return a > b })
}

func lessThanOrEqual(args []interface{}) (interface{}, error) {
	return compareNumbers(args, func(a, b float64) bool { return a <= b })
}

func greaterThanOrEqual(args []interface{}) (interface{}, error) {
	return compareNumbers(args, func(a, b float64) bool { return a >= b })
}

func compareNumbers(args []interface{}, compare func(float64, float64) bool) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("comparison operator expects at least two arguments")
	}
	prev, err := toFloat64(args[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range args[1:] {
		curr, err := toFloat64(arg)
		if err != nil {
			return nil, err
		}
		if !compare(prev, curr) {
			return false, nil
		}
		prev = curr
	}
	return true, nil
}

func toFloat64(v interface{}) (float64, error) {
	switch v := v.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %v to float64", v)
	}
}

// ExecuteGoshLisp takes a Gosh Lisp expression and evaluates it
func ExecuteGoshLisp(input string) (interface{}, error) {
	tokens := Lexer(input)
	ast, err := Parser(tokens)
	if err != nil {
		return nil, err
	}
	return Evaluate(ast)
}

// IsLispExpression checks if a given string is a Lisp expression
func IsLispExpression(cmdString string) bool {
	trimmed := strings.TrimSpace(cmdString)
	return strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")")
}
