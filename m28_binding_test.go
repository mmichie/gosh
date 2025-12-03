package gosh

import (
	"testing"
)

func TestM28Binding(t *testing.T) {
	interpreter := GetM28Interpreter()

	tests := []struct {
		name string
		expr string
	}{
		{"define", `(define x 42)`},
		{"set", `(set! x 42)`},
		{"let", `(let ((x 42)) x)`},
		{"lambda", `((lambda (x) x) 42)`},
		{"def", `(def foo 42)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := interpreter.Execute(tt.expr)
			if err != nil {
				t.Logf("%s error: %v", tt.name, err)
			} else {
				t.Logf("%s result: %s", tt.name, result)
			}
		})
	}
}
