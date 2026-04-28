package gosh

import (
	"os"
	"strings"
	"testing"
)

// Helper function to reset shell options to default state
func resetShellOptions() {
	gs := GetGlobalState()
	gs.SetOption("errexit", false)
	gs.SetOption("nounset", false)
	gs.SetOption("xtrace", false)
	gs.SetOption("pipefail", false)
	gs.SetOption("verbose", false)
	gs.SetOption("noclobber", false)
	gs.SetOption("allexport", false)
}

func TestSetBuiltinBasic(t *testing.T) {
	defer resetShellOptions()

	tests := []struct {
		name         string
		input        string
		checkOption  string
		wantEnabled  bool
		wantContains string
	}{
		{
			name:        "set -e enables errexit",
			input:       "set -e",
			checkOption: "errexit",
			wantEnabled: true,
		},
		{
			name:        "set +e disables errexit",
			input:       "set -e; set +e",
			checkOption: "errexit",
			wantEnabled: false,
		},
		{
			name:        "set -u enables nounset",
			input:       "set -u",
			checkOption: "nounset",
			wantEnabled: true,
		},
		{
			name:        "set -x enables xtrace",
			input:       "set -x",
			checkOption: "xtrace",
			wantEnabled: true,
		},
		{
			name:        "set -o pipefail enables pipefail",
			input:       "set -o pipefail",
			checkOption: "pipefail",
			wantEnabled: true,
		},
		{
			name:        "set +o pipefail disables pipefail",
			input:       "set -o pipefail; set +o pipefail",
			checkOption: "pipefail",
			wantEnabled: false,
		},
		{
			name:        "set -eu enables multiple options",
			input:       "set -eu",
			checkOption: "errexit",
			wantEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetShellOptions()

			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			gs := GetGlobalState()
			val, err := gs.GetOption(tt.checkOption)
			if err != nil {
				t.Fatalf("GetOption() error = %v", err)
			}

			if val != tt.wantEnabled {
				t.Errorf("option %s = %v, want %v", tt.checkOption, val, tt.wantEnabled)
			}
		})
	}
}

func TestSetListOptions(t *testing.T) {
	defer resetShellOptions()

	// Test set -o to list options
	cmd, err := NewCommand("set -o", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &strings.Builder{}
	cmd.Run()

	output := stdout.String()
	expectedOptions := []string{"errexit", "nounset", "xtrace", "pipefail"}
	for _, opt := range expectedOptions {
		if !strings.Contains(output, opt) {
			t.Errorf("set -o output should contain %q, got: %s", opt, output)
		}
	}
}

func TestSetPositionalParams(t *testing.T) {
	defer resetShellOptions()

	gs := GetGlobalState()
	defer gs.SetPositionalParams([]string{})

	// Test set -- to set positional parameters
	cmd, err := NewCommand("set -- foo bar baz", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	cmd.Stdout = &strings.Builder{}
	cmd.Stderr = &strings.Builder{}
	cmd.Run()

	params := gs.GetPositionalParams()
	if len(params) != 3 {
		t.Errorf("positional params count = %d, want 3", len(params))
	}

	expected := []string{"foo", "bar", "baz"}
	for i, want := range expected {
		if i >= len(params) || params[i] != want {
			t.Errorf("positional param $%d = %q, want %q", i+1, params[i], want)
		}
	}
}

func TestSetResetPositionalParams(t *testing.T) {
	defer resetShellOptions()

	gs := GetGlobalState()
	gs.SetPositionalParams([]string{"one", "two", "three"})
	defer gs.SetPositionalParams([]string{})

	// Test set - to reset positional parameters
	cmd, err := NewCommand("set -", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	cmd.Stdout = &strings.Builder{}
	cmd.Stderr = &strings.Builder{}
	cmd.Run()

	params := gs.GetPositionalParams()
	if len(params) != 0 {
		t.Errorf("positional params should be empty after 'set -', got %v", params)
	}
}

func TestXtrace(t *testing.T) {
	defer resetShellOptions()

	// Enable xtrace and run a command
	gs := GetGlobalState()
	gs.SetOption("xtrace", true)

	cmd, err := NewCommand("echo hello", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()

	// Check that stderr contains the trace output
	if !strings.Contains(stderr.String(), "+ echo hello") {
		t.Errorf("xtrace should print '+ echo hello' to stderr, got: %s", stderr.String())
	}

	// Check that stdout still has the actual output
	if !strings.Contains(stdout.String(), "hello") {
		t.Errorf("stdout should contain 'hello', got: %s", stdout.String())
	}
}

func TestPipefail(t *testing.T) {
	defer resetShellOptions()

	tests := []struct {
		name     string
		pipefail bool
		input    string
		wantCode int
	}{
		{
			name:     "without pipefail, last command exit code",
			pipefail: false,
			input:    "false | true",
			wantCode: 0,
		},
		{
			name:     "with pipefail, rightmost failed exit code",
			pipefail: true,
			input:    "false | true",
			wantCode: 1,
		},
		{
			name:     "with pipefail, all succeed",
			pipefail: true,
			input:    "true | true",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetShellOptions()
			gs := GetGlobalState()
			gs.SetOption("pipefail", tt.pipefail)

			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			cmd.Stdout = &strings.Builder{}
			cmd.Stderr = &strings.Builder{}
			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestNounset(t *testing.T) {
	defer resetShellOptions()

	// Make sure UNSET_VAR_FOR_TEST is not set
	os.Unsetenv("UNSET_VAR_FOR_TEST")

	tests := []struct {
		name         string
		nounset      bool
		input        string
		wantCode     int
		wantContains string
	}{
		{
			name:     "without nounset, unset var expands to empty",
			nounset:  false,
			input:    "echo $UNSET_VAR_FOR_TEST",
			wantCode: 0,
		},
		{
			name:         "with nounset, unset var causes error",
			nounset:      true,
			input:        "echo $UNSET_VAR_FOR_TEST",
			wantCode:     1,
			wantContains: "unbound variable",
		},
		{
			name:     "with nounset, set var works",
			nounset:  true,
			input:    "echo $HOME",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetShellOptions()
			gs := GetGlobalState()
			gs.SetOption("nounset", tt.nounset)

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

			if tt.wantContains != "" {
				combined := stdout.String() + stderr.String()
				if !strings.Contains(combined, tt.wantContains) {
					t.Errorf("output should contain %q, got: %s", tt.wantContains, combined)
				}
			}
		})
	}
}

func TestShoptBuiltin(t *testing.T) {
	defer resetShellOptions()

	tests := []struct {
		name         string
		input        string
		checkOption  string
		wantEnabled  bool
		wantContains string
	}{
		{
			name:        "shopt -s pipefail enables pipefail",
			input:       "shopt -s pipefail",
			checkOption: "pipefail",
			wantEnabled: true,
		},
		{
			name:        "shopt -u pipefail disables pipefail",
			input:       "shopt -s pipefail; shopt -u pipefail",
			checkOption: "pipefail",
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetShellOptions()

			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			cmd.Stdout = &strings.Builder{}
			cmd.Stderr = &strings.Builder{}
			cmd.Run()

			gs := GetGlobalState()
			val, err := gs.GetOption(tt.checkOption)
			if err != nil {
				t.Fatalf("GetOption() error = %v", err)
			}

			if val != tt.wantEnabled {
				t.Errorf("option %s = %v, want %v", tt.checkOption, val, tt.wantEnabled)
			}
		})
	}
}

func TestGetOptionsString(t *testing.T) {
	defer resetShellOptions()

	gs := GetGlobalState()

	// No options set
	if got := gs.GetOptionsString(); got != "" {
		t.Errorf("GetOptionsString() with no options = %q, want empty", got)
	}

	// Set some options
	gs.SetOption("errexit", true)
	gs.SetOption("xtrace", true)

	got := gs.GetOptionsString()
	if !strings.Contains(got, "e") {
		t.Errorf("GetOptionsString() should contain 'e', got %q", got)
	}
	if !strings.Contains(got, "x") {
		t.Errorf("GetOptionsString() should contain 'x', got %q", got)
	}
}

func TestInvalidOptions(t *testing.T) {
	defer resetShellOptions()

	tests := []struct {
		name         string
		input        string
		wantContains string
	}{
		{
			name:         "invalid short option",
			input:        "set -z",
			wantContains: "invalid option",
		},
		{
			name:         "invalid named option",
			input:        "set -o nonexistent",
			wantContains: "unknown option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetShellOptions()

			cmd, err := NewCommand(tt.input, NewJobManager())
			if err != nil {
				t.Fatalf("NewCommand() error = %v", err)
			}

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			combined := stdout.String() + stderr.String()
			if !strings.Contains(combined, tt.wantContains) {
				t.Errorf("output should contain %q, got: %s", tt.wantContains, combined)
			}
		})
	}
}
