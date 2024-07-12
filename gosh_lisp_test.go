package gosh

import (
	"reflect"
	"testing"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		input    string
		expected []Token
	}{
		{"(+ 1 2)", []Token{
			{Type: "paren", Value: "("},
			{Type: "identifier", Value: "+"},
			{Type: "number", Value: "1"},
			{Type: "number", Value: "2"},
			{Type: "paren", Value: ")"},
		}},
		{"(* 3.14 (- 5 2))", []Token{
			{Type: "paren", Value: "("},
			{Type: "identifier", Value: "*"},
			{Type: "number", Value: "3.14"},
			{Type: "paren", Value: "("},
			{Type: "identifier", Value: "-"},
			{Type: "number", Value: "5"},
			{Type: "number", Value: "2"},
			{Type: "paren", Value: ")"},
			{Type: "paren", Value: ")"},
		}},
	}

	for _, tt := range tests {
		result := Lexer(tt.input)
		if !reflect.DeepEqual(result, tt.expected) {
			t.Errorf("Lexer(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		input    []Token
		expected Node
	}{
		{
			input: []Token{
				{Type: "paren", Value: "("},
				{Type: "identifier", Value: "+"},
				{Type: "number", Value: "1"},
				{Type: "number", Value: "2"},
				{Type: "paren", Value: ")"},
			},
			expected: Node{
				Type: "root",
				Children: []Node{
					{
						Type: "expression",
						Children: []Node{
							{Type: "identifier", Value: "+"},
							{Type: "number", Value: "1"},
							{Type: "number", Value: "2"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		result, err := Parser(tt.input)
		if err != nil {
			t.Errorf("Parser(%v) returned error: %v", tt.input, err)
		}
		if !reflect.DeepEqual(result, tt.expected) {
			t.Errorf("Parser(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"(+ 1 2)", float64(3)},
		{"(- 5 3)", float64(2)},
		{"(* 2 3)", float64(6)},
		{"(/ 6 2)", float64(3)},
		{"(< 1 2)", true},
		{"(> 2 1)", true},
		{"(<= 2 2)", true},
		{"(>= 3 2)", true},
		{"(= 2 2)", true},
		{"(= 2 3)", false},
	}

	for _, tt := range tests {
		result, err := ExecuteGoshLisp(tt.input)
		if err != nil {
			t.Errorf("ExecuteGoshLisp(%q) returned error: %v", tt.input, err)
		}
		if result != tt.expected {
			t.Errorf("ExecuteGoshLisp(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsLispExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"(+ 1 2)", true},
		{"echo hello", false},
		{"(* 3 (+ 1 2))", true},
		{"()", true},
		{"( )", true},
		{"(", false},
		{")", false},
	}

	for _, tt := range tests {
		result := IsLispExpression(tt.input)
		if result != tt.expected {
			t.Errorf("IsLispExpression(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestErrorCases(t *testing.T) {
	tests := []struct {
		input       string
		expectedErr string
	}{
		{"(/ 1 0)", "division by zero (at argument 2)"},
		{"(+ 1 'a')", "cannot convert 'a' to float64"},
		{"(< 1 'a')", "cannot convert 'a' to float64"},
		{"(", "missing closing parenthesis"},
		{")", "unexpected closing parenthesis"},
		{"(unknown 1 2)", "unknown operator: unknown"},
	}

	for _, tt := range tests {
		_, err := ExecuteGoshLisp(tt.input)
		if err == nil {
			t.Errorf("ExecuteGoshLisp(%q) expected error, got nil", tt.input)
		} else if err.Error() != tt.expectedErr {
			t.Errorf("ExecuteGoshLisp(%q) error = %v, want %v", tt.input, err.Error(), tt.expectedErr)
		}
	}
}
