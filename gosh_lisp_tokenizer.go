package gosh

import (
	"fmt"
	"regexp"
	"strings"
)

// Token represents a token in the Lisp language
type Token struct {
	Type  string
	Value string
}

// Node represents a node in the Lisp AST
type Node struct {
	Type     string
	Value    string
	Children []Node
}

// Lexer tokenizes a Lisp expression
func Lexer(input string) []Token {
	tokens := []Token{}
	
	// Define regex patterns
	parenRegex := regexp.MustCompile(`[\(\)]`)
	numberRegex := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	stringRegex := regexp.MustCompile(`^['"].*['"]$`)
	whitespaceRegex := regexp.MustCompile(`\s+`)
	
	// Split the input by whitespace but keep parentheses separate
	input = parenRegex.ReplaceAllStringFunc(input, func(match string) string {
		return " " + match + " "
	})
	
	fields := whitespaceRegex.Split(strings.TrimSpace(input), -1)
	
	for _, field := range fields {
		if field == "" {
			continue
		}
		
		if field == "(" || field == ")" {
			tokens = append(tokens, Token{Type: "paren", Value: field})
		} else if numberRegex.MatchString(field) {
			tokens = append(tokens, Token{Type: "number", Value: field})
		} else if stringRegex.MatchString(field) {
			tokens = append(tokens, Token{Type: "string", Value: field})
		} else {
			tokens = append(tokens, Token{Type: "identifier", Value: field})
		}
	}
	
	return tokens
}

// Parser parses tokens into an AST
func Parser(tokens []Token) (Node, error) {
	root := Node{Type: "root", Children: []Node{}}
	
	// The index of the current token being processed
	index := 0
	
	// Parse an expression and return the resulting node and updated token index
	var parseExpression func() (Node, int, error)
	parseExpression = func() (Node, int, error) {
		// Skip the opening parenthesis
		index++
		
		expr := Node{Type: "expression", Children: []Node{}}
		
		// Process tokens until we find the closing parenthesis or end of tokens
		for index < len(tokens) {
			token := tokens[index]
			
			if token.Type == "paren" && token.Value == "(" {
				// Recursively parse a nested expression
				nestedExpr, newIndex, err := parseExpression()
				if err != nil {
					return Node{}, 0, err
				}
				
				expr.Children = append(expr.Children, nestedExpr)
				index = newIndex
			} else if token.Type == "paren" && token.Value == ")" {
				// End of the current expression
				return expr, index + 1, nil
			} else {
				// Add the token as a node
				node := Node{Type: token.Type, Value: token.Value}
				expr.Children = append(expr.Children, node)
				index++
			}
		}
		
		return Node{}, 0, fmt.Errorf("missing closing parenthesis")
	}
	
	// Parse tokens into AST
	for index < len(tokens) {
		token := tokens[index]
		
		if token.Type == "paren" && token.Value == "(" {
			expr, newIndex, err := parseExpression()
			if err != nil {
				return Node{}, err
			}
			
			root.Children = append(root.Children, expr)
			index = newIndex
		} else if token.Type == "paren" && token.Value == ")" {
			return Node{}, fmt.Errorf("unexpected closing parenthesis")
		} else {
			// Add the token as a node at the root level
			node := Node{Type: token.Type, Value: token.Value}
			root.Children = append(root.Children, node)
			index++
		}
	}
	
	return root, nil
}