package gosh

import (
	"strings"
	"testing"
)

func TestTypeCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string
		wantCode     int
	}{
		{
			name:         "type for builtin",
			input:        "type cd",
			wantContains: "cd is a shell builtin",
			wantCode:     0,
		},
		{
			name:         "type for multiple builtins",
			input:        "type cd pwd echo",
			wantContains: "builtin",
			wantCode:     0,
		},
		{
			name:         "type for external command",
			input:        "type ls",
			wantContains: "/",
			wantCode:     0,
		},
		{
			name:         "type for non-existent command",
			input:        "type nonexistent_command_12345",
			wantContains: "not found",
			wantCode:     1,
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

			output := stdout.String() + stderr.String()
			if !strings.Contains(output, tt.wantContains) {
				t.Errorf("output = %q, want to contain %q", output, tt.wantContains)
			}

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestTypeWithAlias(t *testing.T) {
	// Set up an alias
	SetAlias("ll", "ls -la")
	defer RemoveAlias("ll")

	cmd, err := NewCommand("type ll", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &strings.Builder{}
	cmd.Run()

	output := stdout.String()
	if !strings.Contains(output, "aliased") || !strings.Contains(output, "ls -la") {
		t.Errorf("output = %q, want to contain 'aliased' and 'ls -la'", output)
	}

	if cmd.ReturnCode != 0 {
		t.Errorf("ReturnCode = %d, want 0", cmd.ReturnCode)
	}
}

func TestWhichCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string
		wantCode     int
	}{
		{
			name:         "which for external command",
			input:        "which ls",
			wantContains: "/",
			wantCode:     0,
		},
		{
			name:         "which for non-existent command",
			input:        "which nonexistent_command_12345",
			wantContains: "",
			wantCode:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &strings.Builder{}
			cmd.Run()

			output := stdout.String()
			if tt.wantContains != "" && !strings.Contains(output, tt.wantContains) {
				t.Errorf("output = %q, want to contain %q", output, tt.wantContains)
			}

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestCommandWithVFlag(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string
		wantCode     int
	}{
		{
			name:         "command -v for builtin",
			input:        "command -v cd",
			wantContains: "cd",
			wantCode:     0,
		},
		{
			name:         "command -v for external",
			input:        "command -v ls",
			wantContains: "/",
			wantCode:     0,
		},
		{
			name:         "command -v for non-existent",
			input:        "command -v nonexistent_command_12345",
			wantContains: "",
			wantCode:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &strings.Builder{}
			cmd.Run()

			output := stdout.String()
			if tt.wantContains != "" && !strings.Contains(output, tt.wantContains) {
				t.Errorf("output = %q, want to contain %q", output, tt.wantContains)
			}

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestCommandWithVFlagAndAlias(t *testing.T) {
	// Set up an alias
	SetAlias("ll", "ls -la")
	defer RemoveAlias("ll")

	cmd, err := NewCommand("command -v ll", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &strings.Builder{}
	cmd.Run()

	output := stdout.String()
	if !strings.Contains(output, "alias") || !strings.Contains(output, "ll") {
		t.Errorf("output = %q, want to contain 'alias' and 'll'", output)
	}

	if cmd.ReturnCode != 0 {
		t.Errorf("ReturnCode = %d, want 0", cmd.ReturnCode)
	}
}
