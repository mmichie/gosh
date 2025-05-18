package gosh

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandSubstitution(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-cmd-sub-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save the current directory to restore it later
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to the temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Update global state to match current directory
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a job manager for testing
	jobManager := NewJobManager()

	testCases := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "Dollar parenthesis syntax",
			input:          "echo $(echo hello)",
			expectedOutput: "hello\n",
		},
		{
			name:           "Backtick syntax",
			input:          "echo `echo hello`",
			expectedOutput: "hello\n",
		},
		{
			name:           "Multiple substitutions",
			input:          "echo $(echo hello) `echo world`",
			expectedOutput: "hello world\n",
		},
		{
			name:           "Nested substitutions",
			input:          "echo $(echo `echo nested`)",
			expectedOutput: "nested\n",
		},
		{
			name:           "Substitution with file redirection",
			input:          "echo $(cat test.txt)",
			expectedOutput: "hello world\n",
		},
		{
			name:           "Substitution with command arguments",
			input:          "echo $(echo -n prefix-)suffix",
			expectedOutput: "prefix-suffix\n",
		},
		{
			name:           "Substitution with exit code handling",
			input:          "echo $(false || echo fallback)",
			expectedOutput: "fallback\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var outputBuffer bytes.Buffer

			cmd, err := NewCommand(tc.input, jobManager)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}

			cmd.Stdout = &outputBuffer
			cmd.Stderr = &outputBuffer
			cmd.Run()

			output := outputBuffer.String()
			if output != tc.expectedOutput {
				t.Errorf("Expected output %q, got %q", tc.expectedOutput, output)
			}
		})
	}
}

func TestCommandSubstitutionErrors(t *testing.T) {
	jobManager := NewJobManager()

	testCases := []struct {
		name          string
		input         string
		shouldSucceed bool
	}{
		{
			name:          "Command not found",
			input:         "echo $(nonexistentcommand)",
			shouldSucceed: false,
		},
		{
			name:          "Syntax error in substituted command",
			input:         "echo $(echo 'unclosed quote)",
			shouldSucceed: false,
		},
		{
			name:          "Unmatched $(",
			input:         "echo $(echo hello",
			shouldSucceed: false,
		},
		{
			name:          "Unmatched backtick",
			input:         "echo `echo hello",
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var outputBuffer bytes.Buffer

			cmd, err := NewCommand(tc.input, jobManager)
			if err != nil {
				if tc.shouldSucceed {
					t.Fatalf("Command creation failed but should have succeeded: %v", err)
				}
				return
			}

			cmd.Stdout = &outputBuffer
			cmd.Stderr = &outputBuffer
			cmd.Run()

			if tc.shouldSucceed && cmd.ReturnCode != 0 {
				t.Errorf("Command failed with exit code %d: %s", cmd.ReturnCode, outputBuffer.String())
			} else if !tc.shouldSucceed && cmd.ReturnCode == 0 {
				t.Errorf("Command succeeded but should have failed: %s", outputBuffer.String())
			}
		})
	}
}

func TestPerformCommandSubstitution(t *testing.T) {
	jobManager := NewJobManager()

	testCases := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "Simple dollar parenthesis substitution",
			input:    "echo $(echo hello)",
			expected: "echo hello",
			hasError: false,
		},
		{
			name:     "Simple backtick substitution",
			input:    "echo `echo hello`",
			expected: "echo hello",
			hasError: false,
		},
		{
			name:     "No substitution",
			input:    "echo hello",
			expected: "echo hello",
			hasError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := PerformCommandSubstitution(tc.input, jobManager)

			if tc.hasError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tc.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tc.hasError && !strings.Contains(result, tc.expected) {
				t.Errorf("Expected result to contain %q, but got %q", tc.expected, result)
			}
		})
	}
}
