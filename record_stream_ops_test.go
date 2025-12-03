package gosh

import (
	"bytes"
	"strings"
	"testing"

	"gosh/parser"
)

// TestRtake tests the rtake command
func TestRtake(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
{"name":"charlie","age":35}
{"name":"dave","age":28}
`

	tests := []struct {
		name     string
		args     string
		expected []string
		excluded []string
	}{
		{
			name:     "Take 2",
			args:     "rtake 2",
			expected: []string{"alice", "bob"},
			excluded: []string{"charlie", "dave"},
		},
		{
			name:     "Take 0",
			args:     "rtake 0",
			expected: []string{},
			excluded: []string{"alice", "bob", "charlie", "dave"},
		},
		{
			name:     "Take more than available",
			args:     "rtake 10",
			expected: []string{"alice", "bob", "charlie", "dave"},
			excluded: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := createRecordTestCommandWithStdin(t, tt.args, strings.NewReader(input), &stdout, &stderr)
			cmd.Run()

			output := stdout.String()

			// Check expected items are present
			for _, item := range tt.expected {
				if !strings.Contains(output, item) {
					t.Errorf("Expected %q to be in output, but it wasn't", item)
				}
			}

			// Check excluded items are not present
			for _, item := range tt.excluded {
				if strings.Contains(output, item) {
					t.Errorf("Expected %q to NOT be in output, but it was", item)
				}
			}
		})
	}
}

// TestRdrop tests the rdrop command
func TestRdrop(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
{"name":"charlie","age":35}
{"name":"dave","age":28}
`

	tests := []struct {
		name     string
		args     string
		expected []string
		excluded []string
	}{
		{
			name:     "Drop 2",
			args:     "rdrop 2",
			expected: []string{"charlie", "dave"},
			excluded: []string{"alice", "bob"},
		},
		{
			name:     "Drop 0",
			args:     "rdrop 0",
			expected: []string{"alice", "bob", "charlie", "dave"},
			excluded: []string{},
		},
		{
			name:     "Drop more than available",
			args:     "rdrop 10",
			expected: []string{},
			excluded: []string{"alice", "bob", "charlie", "dave"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := createRecordTestCommandWithStdin(t, tt.args, strings.NewReader(input), &stdout, &stderr)
			cmd.Run()

			output := stdout.String()

			// Check expected items are present
			for _, item := range tt.expected {
				if !strings.Contains(output, item) {
					t.Errorf("Expected %q to be in output, but it wasn't", item)
				}
			}

			// Check excluded items are not present
			for _, item := range tt.excluded {
				if strings.Contains(output, item) {
					t.Errorf("Expected %q to NOT be in output, but it was", item)
				}
			}
		})
	}
}

// TestRfilter tests the rfilter command by directly calling the builtin
func TestRfilter(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30,"status":"active"}
{"name":"bob","age":25,"status":"inactive"}
{"name":"charlie","age":35,"status":"active"}
`

	tests := []struct {
		name       string
		expression string
		expected   []string
		excluded   []string
	}{
		{
			name:       "Filter by numeric greater than",
			expression: `(> (get-field rec "age") 28)`,
			expected:   []string{"alice", "charlie"},
			excluded:   []string{"bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			// Create a mock Command with the expression as args
			// We need to call the builtin directly to avoid M28 preprocessing
			cmd := &Command{
				Command: &parser.Command{
					LogicalBlocks: []*parser.LogicalBlock{
						{
							FirstPipeline: &parser.Pipeline{
								Commands: []*parser.CommandElement{
									{
										Simple: &parser.SimpleCommand{
											Parts: []string{"rfilter", tt.expression},
										},
									},
								},
							},
						},
					},
				},
				Stdin:  strings.NewReader(input),
				Stdout: &stdout,
				Stderr: &stderr,
			}

			err := rfilterCommand(cmd)
			if err != nil {
				t.Fatalf("rfilterCommand failed: %v", err)
			}

			output := stdout.String()

			// Check expected items are present
			for _, item := range tt.expected {
				if !strings.Contains(output, item) {
					t.Errorf("Expected %q to be in output, but it wasn't. Output: %s", item, output)
				}
			}

			// Check excluded items are not present
			for _, item := range tt.excluded {
				if strings.Contains(output, item) {
					t.Errorf("Expected %q to NOT be in output, but it was. Output: %s", item, output)
				}
			}
		})
	}
}

// TestRreduce tests the rreduce command by directly calling the builtin
func TestRreduce(t *testing.T) {
	input := RecordMagic + `{"amount":10}
{"amount":20}
{"amount":30}
`

	tests := []struct {
		name         string
		initialValue string
		expression   string
		expected     string
	}{
		{
			name:         "Sum amounts",
			initialValue: "0",
			expression:   `(+ acc (get-field rec "amount"))`,
			expected:     "60",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			cmd := &Command{
				Command: &parser.Command{
					LogicalBlocks: []*parser.LogicalBlock{
						{
							FirstPipeline: &parser.Pipeline{
								Commands: []*parser.CommandElement{
									{
										Simple: &parser.SimpleCommand{
											Parts: []string{"rreduce", tt.initialValue, tt.expression},
										},
									},
								},
							},
						},
					},
				},
				Stdin:  strings.NewReader(input),
				Stdout: &stdout,
				Stderr: &stderr,
			}

			err := rreduceCommand(cmd)
			if err != nil {
				t.Fatalf("rreduceCommand failed: %v", err)
			}

			output := strings.TrimSpace(stdout.String())

			if output != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, output)
			}
		})
	}
}

// TestRecordStreamPipelineIntegration tests record stream operations in a pipeline
func TestRecordStreamPipelineIntegration(t *testing.T) {
	// Create test data
	input := RecordMagic + `{"name":"alice","age":30,"score":85}
{"name":"bob","age":25,"score":92}
{"name":"charlie","age":35,"score":78}
{"name":"dave","age":28,"score":88}
`

	// Test: rtake 3 | rdrop 1 -> should give middle 2 records (bob, charlie)
	t.Run("TakeThenDrop", func(t *testing.T) {
		// Step 1: rtake 3
		var buf1 bytes.Buffer
		cmd1 := createRecordTestCommandWithStdin(t, "rtake 3", strings.NewReader(input), &buf1, &bytes.Buffer{})
		cmd1.Run()

		// Step 2: rdrop 1
		var buf2 bytes.Buffer
		cmd2 := createRecordTestCommandWithStdin(t, "rdrop 1", &buf1, &buf2, &bytes.Buffer{})
		cmd2.Run()

		output := buf2.String()
		if !strings.Contains(output, "bob") || !strings.Contains(output, "charlie") {
			t.Errorf("Expected bob and charlie in output, got: %s", output)
		}
		if strings.Contains(output, "alice") || strings.Contains(output, "dave") {
			t.Errorf("Expected alice and dave NOT in output, got: %s", output)
		}
	})
}

// TestM28RecordFunctions tests the M28 record helper functions
func TestM28RecordFunctions(t *testing.T) {
	// Initialize the M28 interpreter
	GetM28Interpreter()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "make-record",
			expr:     `(make-record "name" "test" "value" 42)`,
			expected: "test",
		},
		{
			name:     "get-field",
			expr:     `(get-field (make-record "name" "test") "name")`,
			expected: "test",
		},
		{
			name:     "record-has true",
			expr:     `(record-has (make-record "name" "test") "name")`,
			expected: "True",
		},
		{
			name:     "record-has false",
			expr:     `(record-has (make-record "name" "test") "missing")`,
			expected: "False",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := GetM28Interpreter()
			result, err := interpreter.Execute(tt.expr)
			if err != nil {
				t.Fatalf("Failed to execute expression: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected result to contain %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestRtakeErrors tests error handling for rtake
func TestRtakeErrors(t *testing.T) {
	tests := []struct {
		name        string
		args        string
		expectError bool
	}{
		{
			name:        "No arguments",
			args:        "rtake",
			expectError: true,
		},
		{
			name:        "Invalid number",
			args:        "rtake abc",
			expectError: true,
		},
		{
			name:        "Negative number",
			args:        "rtake -5",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd, err := NewCommand(tt.args, nil)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Stdin = strings.NewReader("")
			cmd.Run()

			if tt.expectError && cmd.ReturnCode == 0 {
				t.Errorf("Expected non-zero return code for error case")
			}
		})
	}
}

// TestRdropErrors tests error handling for rdrop
func TestRdropErrors(t *testing.T) {
	tests := []struct {
		name        string
		args        string
		expectError bool
	}{
		{
			name:        "No arguments",
			args:        "rdrop",
			expectError: true,
		},
		{
			name:        "Invalid number",
			args:        "rdrop xyz",
			expectError: true,
		},
		{
			name:        "Negative number",
			args:        "rdrop -3",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd, err := NewCommand(tt.args, nil)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Stdin = strings.NewReader("")
			cmd.Run()

			if tt.expectError && cmd.ReturnCode == 0 {
				t.Errorf("Expected non-zero return code for error case")
			}
		})
	}
}

// TestReach tests the reach command by directly calling the builtin
func TestReach(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
`

	var stdout, stderr bytes.Buffer

	cmd := &Command{
		Command: &parser.Command{
			LogicalBlocks: []*parser.LogicalBlock{
				{
					FirstPipeline: &parser.Pipeline{
						Commands: []*parser.CommandElement{
							{
								Simple: &parser.SimpleCommand{
									Parts: []string{"reach", `(get-field rec "name")`},
								},
							},
						},
					},
				},
			},
		},
		Stdin:  strings.NewReader(input),
		Stdout: &stdout,
		Stderr: &stderr,
	}

	err := reachCommand(cmd)
	if err != nil {
		t.Fatalf("reachCommand failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "alice") || !strings.Contains(output, "bob") {
		t.Errorf("Expected names in output, got: %s", output)
	}
}

// TestRecordConversion tests conversion between Go values and M28 values
func TestRecordConversion(t *testing.T) {
	// Test recordToM28Value
	record := Record{
		"name":   "test",
		"age":    float64(30),
		"active": true,
	}

	m28Val := recordToM28Value(record)
	if m28Val == nil {
		t.Fatal("Expected non-nil M28 DictValue")
	}

	// Convert back
	goVal := m28ValueToGo(m28Val)
	result, ok := goVal.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", goVal)
	}

	if result["name"] != "test" {
		t.Errorf("Expected name='test', got %v", result["name"])
	}
}

// Note: createRecordTestCommandWithStdin is defined in record_test.go
