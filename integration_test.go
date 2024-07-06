package gosh

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestIntegration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple echo command",
			input:    "echo Hello, World!",
			expected: "Hello, World!\n",
		},
		{
			name:     "Change directory",
			input:    "cd /tmp && pwd",
			expected: "/tmp\n",
		},
		/*
				{
					name:     "Multiple commands",
					input:    "echo First && echo Second",
					expected: "First\nSecond\n",
				},
			{
				name:     "Pipe commands",
				input:    "echo Hello, World! | wc -w",
				expected: "2\n",
			},
		*/
		{
			name:     "Environment variable",
			input:    "export TEST_VAR=hello && echo $TEST_VAR",
			expected: "hello\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run the command
			jobManager := NewJobManager()
			cmd, err := NewCommand(tt.input, jobManager)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}
			cmd.Run()

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Compare output
			if tt.expected != "" && !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain %q, but got %q", tt.expected, output)
			}
		})
	}
}
