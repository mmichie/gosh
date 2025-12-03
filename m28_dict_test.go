package gosh

import (
	"testing"

	"github.com/mmichie/m28/core"
)

func TestM28DictSyntax(t *testing.T) {
	interpreter := GetM28Interpreter()

	// Test dict creation
	tests := []struct {
		name string
		expr string
	}{
		{"dict literal", `{"name": "test"}`},
		{"make-record", `(make-record "name" "test")`},
		{"empty dict", `{}`},
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

func TestM28PrintValue(t *testing.T) {
	dict := core.NewDict()
	dict.Set("name", core.StringValue("test"))
	dict.Set("value", core.NumberValue(42))

	printed := core.PrintValue(dict)
	t.Logf("PrintValue output: %s", printed)
}
