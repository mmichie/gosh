package gosh

import (
	"strings"
	"testing"
)

func TestExitCodeHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "true command returns 0",
			input:    "true",
			wantCode: 0,
		},
		{
			name:     "false command returns 1",
			input:    "false",
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}

			// Check that $? is updated
			state := GetGlobalState()
			if state.GetLastExitStatus() != tt.wantCode {
				t.Errorf("$? = %d, want %d", state.GetLastExitStatus(), tt.wantCode)
			}
		})
	}
}

func TestPipelineExitCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "Pipeline with all success",
			input:    "true | true | true",
			wantCode: 0,
		},
		{
			name:     "Pipeline with last command failing",
			input:    "true | true | false",
			wantCode: 1,
		},
		{
			name:     "Pipeline with first command failing",
			input:    "false | true | true",
			wantCode: 0, // Last command succeeds
		},
		{
			name:     "Pipeline with middle command failing",
			input:    "true | false | true",
			wantCode: 0, // Last command succeeds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d (stderr: %s)", cmd.ReturnCode, tt.wantCode, stderr.String())
			}
		})
	}
}

func TestLogicalOperatorExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "AND with both success",
			input:    "true && true",
			wantCode: 0,
		},
		{
			name:     "AND with first failure",
			input:    "false && true",
			wantCode: 1, // First command fails, second not executed
		},
		{
			name:     "AND with second failure",
			input:    "true && false",
			wantCode: 1, // Second command fails
		},
		{
			name:     "OR with first success",
			input:    "true || false",
			wantCode: 0, // First command succeeds, second not executed
		},
		{
			name:     "OR with first failure",
			input:    "false || true",
			wantCode: 0, // Second command succeeds
		},
		{
			name:     "OR with both failure",
			input:    "false || false",
			wantCode: 1, // Both fail
		},
		{
			name:     "Complex: AND then OR",
			input:    "true && false || true",
			wantCode: 0, // (true && false) fails, then true succeeds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestSubshellExitCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "Subshell with success",
			input:    "( true )",
			wantCode: 0,
		},
		{
			name:     "Subshell with failure",
			input:    "( false )",
			wantCode: 1,
		},
		{
			name:     "Subshell with multiple commands",
			input:    "( true ; false )",
			wantCode: 1, // Last command in subshell
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestBuiltinExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "cd to existing directory",
			input:    "cd /tmp",
			wantCode: 0,
		},
		{
			name:     "cd to non-existing directory",
			input:    "cd /nonexistent_directory_12345",
			wantCode: 1,
		},
		{
			name:     "echo always succeeds",
			input:    "echo hello",
			wantCode: 0,
		},
		{
			name:     "pwd always succeeds",
			input:    "pwd",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d (stderr: %s)", cmd.ReturnCode, tt.wantCode, stderr.String())
			}
		})
	}
}
