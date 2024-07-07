package gosh

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestIntegration(t *testing.T) {
	// Set up logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set the temporary directory as HOME
	os.Setenv("HOME", tempDir)
	defer os.Unsetenv("HOME")

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
			expected: "/tmp\n" + tempDir + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.Printf("--- Starting test: %s ---", tt.name)
			log.Printf("Input command: %s", tt.input)

			// Reset global state for each test
			gs := GetGlobalState()
			gs.UpdateCWD(tempDir)

			// Capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stdout = w
			os.Stderr = w

			// Run the command
			jobManager := NewJobManager()
			cmds := strings.Split(tt.input, " && ")
			var output bytes.Buffer
			for _, cmdStr := range cmds {
				cmd, err := NewCommand(cmdStr, jobManager)
				if err != nil {
					t.Fatalf("Failed to create command: %v", err)
				}
				cmd.Stdout = &output
				cmd.Stderr = &output
				cmd.Run()
			}

			// Restore stdout and stderr
			w.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			// Read captured output
			capturedOutput := output.String()

			log.Printf("Captured output:\n%s", capturedOutput)

			// Compare output
			if tt.expected != "" && !strings.Contains(capturedOutput, tt.expected) {
				t.Errorf("Test case: %s\nCommand: %s\nExpected output to contain:\n%s\nBut got:\n%s\n", tt.name, tt.input, tt.expected, capturedOutput)
			}

			log.Printf("--- Finished test: %s ---\n", tt.name)
		})
	}
}

func TestPreviousDirectoryHandling(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)
	defer os.Unsetenv("HOME")

	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	jobManager := NewJobManager()

	cmd, _ := NewCommand("cd /tmp", jobManager)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Run()

	cmd, _ = NewCommand("cd -", jobManager)
	output.Reset()
	cmd.Stdout = &output
	cmd.Run()

	expected := tempDir + "\n"
	if output.String() != expected {
		t.Errorf("Expected previous directory to be %s, got %s", expected, output.String())
	}
}

func TestCDWithDash(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)
	os.Setenv("PWD", tempDir)
	defer os.Unsetenv("HOME")
	defer os.Unsetenv("PWD")

	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	jobManager := NewJobManager()
	var output bytes.Buffer

	// First cd to /tmp
	cmd, _ := NewCommand("cd /tmp", jobManager)
	cmd.Stdout = &output
	cmd.Run()

	// Then cd back using -
	cmd, _ = NewCommand("cd -", jobManager)
	cmd.Stdout = &output
	cmd.Run()

	// Check the output
	expectedOutput := "/tmp\n" + tempDir + "\n"
	if output.String() != expectedOutput {
		t.Errorf("Expected output %q but got %q", expectedOutput, output.String())
	}
}

func TestCDCommand(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)
	os.Setenv("PWD", tempDir)
	defer os.Unsetenv("HOME")
	defer os.Unsetenv("PWD")

	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	jobManager := NewJobManager()

	cmd, _ := NewCommand("cd /tmp", jobManager)
	cmd.Run()

	cmd, _ = NewCommand("cd -", jobManager)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Run()

	expected := tempDir + "\n"
	if output.String() != expected {
		t.Errorf("Failed cd -: expected %s, got %s", expected, output.String())
	}
}
