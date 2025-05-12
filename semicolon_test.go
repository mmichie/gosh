package gosh

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSemicolonOperator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Strings that should appear in the output
	}{
		{
			name:     "Basic semicolon operator",
			input:    "echo first; echo second",
			expected: []string{"first", "second"},
		},
		{
			name:     "Three commands with semicolons",
			input:    "echo one; echo two; echo three",
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "Simple AND operator",
			input:    "true && echo success",
			expected: []string{"success"},
		},
		{
			name:     "Simple OR operator",
			input:    "false || echo failed",
			expected: []string{"failed"},
		},
		{
			name:     "Command with pipe and semicolon",
			input:    "echo hello | tr a-z A-Z; echo done",
			expected: []string{"HELLO", "done"},
		},
		{
			name:     "Return code isolation between commands",
			input:    "false; echo should be printed",
			expected: []string{"should be printed"},
		},
	}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-semicolon-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save the current directory to restore it later
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to the temp directory for the test
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Update global state to match current directory
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the command
			jobManager := NewJobManager()
			var output bytes.Buffer
			cmd, err := NewCommand(tt.input, jobManager)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}
			cmd.Stdout = &output
			cmd.Stderr = &output

			// Run the command
			cmd.Run()

			// Check the output for expected strings
			outputStr := output.String()
			t.Logf("Command output: %s", outputStr)

			for _, expected := range tt.expected {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it doesn't: %q",
						expected, outputStr)
				}
			}
		})
	}
}

// Test to ensure each command in a semicolon-separated list has its own return code
func TestSemicolonReturnCodes(t *testing.T) {
	// We'll manually check internal state since return codes aren't directly visible
	jobManager := NewJobManager()
	input := "true; false; true"

	cmd, err := NewCommand(input, jobManager)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Run the command
	cmd.Run()

	// The final return code should be from the last command (true = 0)
	if cmd.ReturnCode != 0 {
		t.Errorf("Expected final return code to be 0, got %d", cmd.ReturnCode)
	}

	// Test a different sequence where the last command fails
	input = "true; true; false"
	cmd, err = NewCommand(input, jobManager)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	cmd.Stdout = &output
	cmd.Stderr = &output

	// Run the command
	cmd.Run()

	// The final return code should be from the last command (false = 1)
	if cmd.ReturnCode != 1 {
		t.Errorf("Expected final return code to be 1, got %d", cmd.ReturnCode)
	}
}
