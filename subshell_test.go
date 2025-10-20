package gosh

import (
	"strings"
	"testing"
)

func TestSubshellParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Simple subshell",
			input:   "( echo hello )",
			wantErr: false,
		},
		{
			name:    "Subshell with multiple commands",
			input:   "( echo hello ; echo world )",
			wantErr: false,
		},
		{
			name:    "Nested subshells",
			input:   "( ( echo nested ) )",
			wantErr: false,
		},
		{
			name:    "Subshell with redirection",
			input:   "( echo hello ) > output.txt",
			wantErr: false,
		},
		{
			name:    "Command group",
			input:   "{ echo hello; }",
			wantErr: false,
		},
		{
			name:    "Command group with multiple commands",
			input:   "{ echo hello ; echo world; }",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == nil {
				t.Error("NewCommand() returned nil command")
			}
		})
	}
}

func TestSubshellIsolation(t *testing.T) {
	state := GetGlobalState()
	origCWD := state.GetCWD()
	defer state.UpdateCWD(origCWD)

	tests := []struct {
		name            string
		input           string
		wantCWDChanged  bool
		description     string
	}{
		{
			name:           "Subshell cd doesn't affect parent",
			input:          "( cd /tmp )",
			wantCWDChanged: false,
			description:    "cd in subshell should not change parent CWD",
		},
		{
			name:           "Command group cd affects parent",
			input:          "{ cd /tmp; }",
			wantCWDChanged: true,
			description:    "cd in command group should change parent CWD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset to original CWD before each test
			state.UpdateCWD(origCWD)

			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			currentCWD := state.GetCWD()
			cwdChanged := currentCWD != origCWD

			if cwdChanged != tt.wantCWDChanged {
				t.Errorf("%s: CWD changed = %v, want %v (orig: %s, current: %s)",
					tt.description, cwdChanged, tt.wantCWDChanged, origCWD, currentCWD)
			}
		})
	}
}

func TestSubshellExecution(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "Subshell with echo",
			input:      "( echo hello )",
			wantStdout: "hello\n",
			wantErr:    false,
		},
		{
			name:       "Subshell with multiple commands",
			input:      "( echo first ; echo second )",
			wantStdout: "first\nsecond\n",
			wantErr:    false,
		},
		{
			name:       "Command group with echo",
			input:      "{ echo grouped; }",
			wantStdout: "grouped\n",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("NewCommand() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Stdin = strings.NewReader("")

			cmd.Run()

			gotStdout := stdout.String()
			if gotStdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", gotStdout, tt.wantStdout)
			}
		})
	}
}

func TestSubshellReturnCode(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantReturnCode int
	}{
		{
			name:           "Subshell with successful command",
			input:          "( true )",
			wantReturnCode: 0,
		},
		{
			name:           "Subshell with failed command",
			input:          "( false )",
			wantReturnCode: 1,
		},
		{
			name:           "Command group with successful command",
			input:          "{ true; }",
			wantReturnCode: 0,
		},
		{
			name:           "Command group with failed command",
			input:          "{ false; }",
			wantReturnCode: 1,
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

			if cmd.ReturnCode != tt.wantReturnCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantReturnCode)
			}
		})
	}
}
