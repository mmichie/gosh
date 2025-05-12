package gosh

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

// TestLogicalOperators directly tests the AND (&&) and OR (||) operators
func TestLogicalOperators(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectedCode   int
	}{
		{
			name:           "Simple AND with success",
			input:          "true && echo success",
			expectedOutput: "success\n",
			expectedCode:   0,
		},
		{
			name:           "Simple AND with failure",
			input:          "false && echo should-not-print",
			expectedOutput: "",
			expectedCode:   1,
		},
		{
			name:           "Simple OR with failure",
			input:          "false || echo or-worked",
			expectedOutput: "or-worked\n",
			expectedCode:   0,
		},
		{
			name:           "Simple OR with success",
			input:          "true || echo should-not-print",
			expectedOutput: "",
			expectedCode:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Enable debug mode for detailed diagnosis
			os.Setenv("GOSH_DEBUG", "1")
			defer os.Unsetenv("GOSH_DEBUG")

			// Create a new command with the test input
			jobManager := NewJobManager()
			var output bytes.Buffer
			var errOutput bytes.Buffer

			cmd, err := NewCommand(tt.input, jobManager)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}

			// Capture stdout and stderr separately for better diagnostics
			cmd.Stdout = &output
			cmd.Stderr = &errOutput

			// Run the command
			cmd.Run()

			// Print debug info
			t.Logf("Command executed: %s", tt.input)
			t.Logf("STDOUT: %q", output.String())
			t.Logf("STDERR: %q", errOutput.String())
			t.Logf("Return code: %d", cmd.ReturnCode)

			// Check the output matches what we expect
			if output.String() != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, output.String())
			}

			// Check the return code
			if cmd.ReturnCode != tt.expectedCode {
				t.Errorf("Expected return code %d, got %d", tt.expectedCode, cmd.ReturnCode)
			}
		})
	}
}

// Helper function to debug command structure
func logCommandStructure(cmd *Command) string {
	var output bytes.Buffer

	fmt.Fprintf(&output, "Command Structure:\n")
	fmt.Fprintf(&output, "  LogicalBlocks: %d\n", len(cmd.Command.LogicalBlocks))

	for i, block := range cmd.Command.LogicalBlocks {
		fmt.Fprintf(&output, "  Block %d:\n", i)
		fmt.Fprintf(&output, "    FirstPipeline Commands: %d\n", len(block.FirstPipeline.Commands))

		for j, command := range block.FirstPipeline.Commands {
			fmt.Fprintf(&output, "      Command %d: %v\n", j, command.Parts)
		}

		fmt.Fprintf(&output, "    RestPipelines: %d\n", len(block.RestPipelines))
		for j, pipeline := range block.RestPipelines {
			fmt.Fprintf(&output, "      Pipeline %d Operator: %s\n", j, pipeline.Operator)
			fmt.Fprintf(&output, "      Pipeline %d Commands: %d\n", j, len(pipeline.Pipeline.Commands))

			for k, command := range pipeline.Pipeline.Commands {
				fmt.Fprintf(&output, "        Command %d: %v\n", k, command.Parts)
			}
		}
	}

	return output.String()
}
