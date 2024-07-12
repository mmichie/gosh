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
		result := 0.0
		for _, arg := range args {
			num, ok := arg.(float64)
			if !ok {
				return nil, fmt.Errorf("'+' operator expects numbers")
			}
			result += num
		}
		return result, nil
	case "-":
		if len(args) == 0 {
			return nil, fmt.Errorf("'-' operator expects at least one argument")
		}
		result, ok := args[0].(float64)
		if !ok {
			return nil, fmt.Errorf("'-' operator expects numbers")
		}
		for _, arg := range args[1:] {
			num, ok := arg.(float64)
			if !ok {
				return nil, fmt.Errorf("'-' operator expects numbers")
			}
			result -= num
		}
		return result, nil
	// Add more operators as needed
	default:
		return nil, fmt.Errorf("unknown operator: %s", operator)
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
