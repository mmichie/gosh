package gosh

import (
	"bytes"
	"strings"
	"testing"
)

func TestOrOperator(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		notExpected string // Text that should NOT appear in output (as part of actual command output)
	}{
		{
			name:     "Basic OR operator",
			input:    "false || echo success",
			expected: "success",
		},
		{
			name:        "OR operator with success in first command",
			input:       "true || echo should-not-see-this",
			expected:    "",
			notExpected: "should-not-see-this",
		},
		{
			name:        "Complex OR with AND operators",
			input:       "true && echo first-succeeded || echo first-failed",
			expected:    "first-succeeded",
			notExpected: "first-failed",
		},
		{
			name:        "Complex AND with OR operators",
			input:       "false && echo nothing-here || echo this-should-be-printed",
			expected:    "this-should-be-printed", // Correct shell behavior - first command fails, second runs
			notExpected: "nothing-here",
		},
		{
			name:     "Chain of operators",
			input:    "false || false || echo third-command-runs",
			expected: "third-command-runs",
		},
		{
			name:        "Chain with mix of success and failure",
			input:       "false || true || echo should-not-see-this",
			expected:    "",
			notExpected: "should-not-see-this",
		},
		{
			name:     "OR followed by AND",
			input:    "false || echo second-succeeded && echo both-succeeded",
			expected: "second-succeeded",
		},
	}

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

			// We're using builtin implementations of true/false commands for testing
			cmd.Run()

			// Check the output
			outputStr := output.String()
			t.Logf("Command output: %s", outputStr)

			// Clean output from debug messages
			debugLines := []string{"DEBUG-", "ECHO DEBUG:"}
			cleanOutput := ""
			for _, line := range strings.Split(outputStr, "\n") {
				isDebug := false
				for _, prefix := range debugLines {
					if strings.HasPrefix(strings.TrimSpace(line), prefix) {
						isDebug = true
						break
					}
				}
				if !isDebug && line != "" {
					cleanOutput += line + "\n"
				}
			}
			cleanOutput = strings.TrimSpace(cleanOutput)
			t.Logf("Clean output: %s", cleanOutput)

			// Check expected output is present
			if tt.expected != "" && !strings.Contains(cleanOutput, tt.expected) {
				t.Errorf("Expected output to contain %q, but got %q", tt.expected, cleanOutput)
			}

			// Check not-expected text is absent
			if tt.notExpected != "" && strings.Contains(cleanOutput, tt.notExpected) {
				t.Errorf("Output should NOT contain %q, but it does: %q", tt.notExpected, cleanOutput)
			}

			// If expected is empty, verify output is actually empty
			if tt.expected == "" && tt.notExpected == "" && cleanOutput != "" {
				t.Errorf("Expected empty output, but got: %q", cleanOutput)
			}
		})
	}
}

// Test that helps explicitly debug the parser structure for OR commands
func TestOrOperatorParsing(t *testing.T) {
	// Create a simple command with OR
	orCommand := "false || echo or-worked"
	jobManager := NewJobManager()

	cmd, err := NewCommand(orCommand, jobManager)
	if err != nil {
		t.Fatalf("Error creating command: %v", err)
	}

	// Dump the parsed command structure
	t.Logf("Parsed command: %#v", cmd.Command)
	if len(cmd.Command.LogicalBlocks) > 0 && len(cmd.Command.LogicalBlocks[0].RestPipelines) > 0 {
		t.Logf("Operator found: %s", cmd.Command.LogicalBlocks[0].RestPipelines[0].Operator)
	} else {
		t.Fatalf("No operator or pipeline found in parsed command")
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Run()

	// Check the result
	result := output.String()
	t.Logf("OR output: %q", result)
	if !strings.Contains(result, "or-worked") {
		t.Errorf("OR operator failed, expected to contain %q, got %q", "or-worked", result)
	}
}
