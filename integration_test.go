package gosh

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

func TestIntegration(t *testing.T) {
	// Set up logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

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
			name:     "Pipe commands",
			input:    "echo Hello, World! | wc -w",
			expected: "2\n",
		},
		{
			name:     "Multiple pipes",
			input:    "echo 'one two three four' | tr ' ' '\n' | sort | uniq -c | sort -nr",
			expected: "      1 two\n      1 three\n      1 one\n      1 four\n",
		},
		// ... (other test cases)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.Printf("--- Starting test: %s ---", tt.name)
			log.Printf("Input command: %s", tt.input)

			// Capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stdout = w
			os.Stderr = w

			// Run the command
			jobManager := NewJobManager()
			cmd, err := NewCommand(tt.input, jobManager)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}
			cmd.Stdout = w
			cmd.Stderr = w
			cmd.Run()

			// Restore stdout and stderr
			w.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			log.Printf("Captured output:\n%s", output)

			// Compare output
			if tt.expected != "" && !strings.Contains(output, tt.expected) {
				t.Errorf("Test case: %s\nCommand: %s\nExpected output to contain:\n%s\nBut got:\n%s\n", tt.name, tt.input, tt.expected, output)
			}

			log.Printf("--- Finished test: %s ---\n", tt.name)
		})
	}
}
