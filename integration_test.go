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

	// Simulate setting the home directory
	homeDir := "/fakehome"
	os.Setenv("HOME", homeDir)
	defer os.Unsetenv("HOME") // Clean up environment change at the end of tests

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
		{
			name:     "Environment variable",
			input:    "export TEST_VAR=hello && echo $TEST_VAR",
			expected: "hello\n",
		},
		{
			name:     "CD with dash",
			input:    "cd /tmp && pwd && cd - && pwd",
			expected: "/tmp\n" + homeDir + "\n",
		},
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

func TestPreviousDirectoryHandling(t *testing.T) {
	os.Setenv("HOME", "/fakehome")
	defer os.Unsetenv("HOME")

	jobManager := NewJobManager()
	cmd, _ := NewCommand("cd /tmp", jobManager)
	cmd.Run() // Change to /tmp, setting PreviousDir to initial dir

	cmd, _ = NewCommand("cd -", jobManager)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Run() // Should revert to initial dir

	expected := "/fakehome\n"
	if output.String() != expected {
		t.Errorf("Expected previous directory to be %s, got %s", expected, output.String())
	}
}

func TestCDWithDash(t *testing.T) {
	os.Setenv("HOME", "/fakehome")
	defer os.Unsetenv("HOME") // Ensure cleanup after test

	command := "cd /tmp && pwd && cd - && pwd"
	expectedOutput := "/tmp\n/fakehome\n"

	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	jobManager := NewJobManager()
	cmd, err := NewCommand(command, jobManager)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	cmd.Stdout = w
	cmd.Run()

	w.Close()
	os.Stdout = stdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()
	if output != expectedOutput {
		t.Errorf("Expected output %q but got %q", expectedOutput, output)
	}
}

func TestCDCommand(t *testing.T) {
	os.Setenv("HOME", "/fakehome")
	defer os.Unsetenv("HOME")

	jobManager := NewJobManager()

	cmd, _ := NewCommand("cd /tmp", jobManager)
	cmd.Run()

	cmd, _ = NewCommand("cd -", jobManager)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Run()

	expected := "/fakehome\n"
	if output.String() != expected {
		t.Errorf("Failed cd -: expected %s, got %s", expected, output.String())
	}
}
