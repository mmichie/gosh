package gosh

import (
	"strconv"
	"strings"
	"testing"
)

func TestPositionalParameters(t *testing.T) {
	state := GetGlobalState()

	// Save original state
	origParams := state.GetPositionalParams()
	origScriptName := state.GetScriptName()

	// Restore after test
	defer func() {
		state.SetPositionalParams(origParams)
		state.SetScriptName(origScriptName)
	}()

	tests := []struct {
		name       string
		scriptName string
		params     []string
		input      string
		want       string
	}{
		{
			name:       "$0 - script name",
			scriptName: "test.sh",
			params:     []string{},
			input:      "$0",
			want:       "test.sh",
		},
		{
			name:       "$1 - first parameter",
			scriptName: "script",
			params:     []string{"arg1", "arg2", "arg3"},
			input:      "$1",
			want:       "arg1",
		},
		{
			name:       "$2 - second parameter",
			scriptName: "script",
			params:     []string{"arg1", "arg2", "arg3"},
			input:      "$2",
			want:       "arg2",
		},
		{
			name:       "$9 - ninth parameter",
			scriptName: "script",
			params:     []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			input:      "$9",
			want:       "9",
		},
		{
			name:       "${10} - tenth parameter",
			scriptName: "script",
			params:     []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			input:      "${10}",
			want:       "10",
		},
		{
			name:       "${11} - eleventh parameter",
			scriptName: "script",
			params:     []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			input:      "${11}",
			want:       "11",
		},
		{
			name:       "$# - parameter count",
			scriptName: "script",
			params:     []string{"a", "b", "c"},
			input:      "$#",
			want:       "3",
		},
		{
			name:       "$# - zero parameters",
			scriptName: "script",
			params:     []string{},
			input:      "$#",
			want:       "0",
		},
		{
			name:       "$@ - all parameters",
			scriptName: "script",
			params:     []string{"one", "two", "three"},
			input:      "$@",
			want:       "one two three",
		},
		{
			name:       "$* - all parameters",
			scriptName: "script",
			params:     []string{"one", "two", "three"},
			input:      "$*",
			want:       "one two three",
		},
		{
			name:       "Multiple positional params",
			scriptName: "test.sh",
			params:     []string{"alice", "bob", "charlie"},
			input:      "$0 $1 $2 $3",
			want:       "test.sh alice bob charlie",
		},
		{
			name:       "Mix with count",
			scriptName: "script",
			params:     []string{"a", "b"},
			input:      "Count: $# Args: $@",
			want:       "Count: 2 Args: a b",
		},
		{
			name:       "Missing parameter - empty string",
			scriptName: "script",
			params:     []string{"arg1"},
			input:      "$1 $2 $3",
			want:       "arg1  ",
		},
		{
			name:       "${0} - braced script name",
			scriptName: "test.sh",
			params:     []string{},
			input:      "${0}",
			want:       "test.sh",
		},
		{
			name:       "Complex substitution",
			scriptName: "deploy.sh",
			params:     []string{"prod", "v1.0"},
			input:      "Deploying $2 to $1 from $0",
			want:       "Deploying v1.0 to prod from deploy.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.SetScriptName(tt.scriptName)
			state.SetPositionalParams(tt.params)

			result := ExpandSpecialVariables(tt.input)

			if result != tt.want {
				t.Errorf("got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestGetPositionalParam(t *testing.T) {
	state := GetGlobalState()

	// Save and restore
	origParams := state.GetPositionalParams()
	defer func() {
		state.SetPositionalParams(origParams)
	}()

	state.SetPositionalParams([]string{"a", "b", "c"})

	tests := []struct {
		index int
		want  string
	}{
		{1, "a"},
		{2, "b"},
		{3, "c"},
		{0, ""},  // Invalid - 0 is not a positional parameter
		{4, ""},  // Out of bounds
		{-1, ""}, // Invalid
	}

	for _, tt := range tests {
		t.Run("param_"+strconv.Itoa(tt.index), func(t *testing.T) {
			got := state.GetPositionalParam(tt.index)
			if got != tt.want {
				t.Errorf("GetPositionalParam(%d) = %q, want %q", tt.index, got, tt.want)
			}
		})
	}
}

func TestGetPositionalParamCount(t *testing.T) {
	state := GetGlobalState()

	// Save and restore
	origParams := state.GetPositionalParams()
	defer func() {
		state.SetPositionalParams(origParams)
	}()

	tests := []struct {
		params []string
		want   int
	}{
		{[]string{}, 0},
		{[]string{"a"}, 1},
		{[]string{"a", "b", "c"}, 3},
		{[]string{"1", "2", "3", "4", "5"}, 5},
	}

	for _, tt := range tests {
		t.Run("count_"+strconv.Itoa(tt.want), func(t *testing.T) {
			state.SetPositionalParams(tt.params)
			got := state.GetPositionalParamCount()
			if got != tt.want {
				t.Errorf("GetPositionalParamCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestShiftPositionalParams(t *testing.T) {
	state := GetGlobalState()

	// Save and restore
	origParams := state.GetPositionalParams()
	defer func() {
		state.SetPositionalParams(origParams)
	}()

	tests := []struct {
		name     string
		initial  []string
		shift    int
		wantErr  bool
		expected []string
	}{
		{
			name:     "shift by 1",
			initial:  []string{"a", "b", "c"},
			shift:    1,
			wantErr:  false,
			expected: []string{"b", "c"},
		},
		{
			name:     "shift by 2",
			initial:  []string{"a", "b", "c", "d"},
			shift:    2,
			wantErr:  false,
			expected: []string{"c", "d"},
		},
		{
			name:     "shift all",
			initial:  []string{"a", "b"},
			shift:    2,
			wantErr:  false,
			expected: []string{},
		},
		{
			name:     "shift more than available",
			initial:  []string{"a", "b"},
			shift:    5,
			wantErr:  false,
			expected: []string{},
		},
		{
			name:     "shift 0",
			initial:  []string{"a", "b", "c"},
			shift:    0,
			wantErr:  false,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "shift negative - error",
			initial:  []string{"a", "b"},
			shift:    -1,
			wantErr:  true,
			expected: []string{"a", "b"}, // Unchanged on error
		},
		{
			name:     "shift empty array",
			initial:  []string{},
			shift:    1,
			wantErr:  false,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.SetPositionalParams(tt.initial)

			err := state.ShiftPositionalParams(tt.shift)

			if (err != nil) != tt.wantErr {
				t.Errorf("ShiftPositionalParams(%d) error = %v, wantErr %v", tt.shift, err, tt.wantErr)
			}

			got := state.GetPositionalParams()
			if !equalSlices(got, tt.expected) {
				t.Errorf("After shift(%d), params = %v, want %v", tt.shift, got, tt.expected)
			}
		})
	}
}

func TestShiftBuiltin(t *testing.T) {
	state := GetGlobalState()

	// Save and restore
	origParams := state.GetPositionalParams()
	defer func() {
		state.SetPositionalParams(origParams)
	}()

	tests := []struct {
		name     string
		initial  []string
		cmd      string
		wantCode int
		expected []string
	}{
		{
			name:     "shift without args (default 1)",
			initial:  []string{"a", "b", "c"},
			cmd:      "shift",
			wantCode: 0,
			expected: []string{"b", "c"},
		},
		{
			name:     "shift 2",
			initial:  []string{"a", "b", "c", "d"},
			cmd:      "shift 2",
			wantCode: 0,
			expected: []string{"c", "d"},
		},
		{
			name:     "shift invalid argument",
			initial:  []string{"a", "b"},
			cmd:      "shift abc",
			wantCode: 1,
			expected: []string{"a", "b"}, // Unchanged on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.SetPositionalParams(tt.initial)

			// Parse and execute the command
			cmd := createTestCommand(t, tt.cmd)
			_ = shiftCommand(cmd) // Errors are handled via ReturnCode

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}

			got := state.GetPositionalParams()
			if !equalSlices(got, tt.expected) {
				t.Errorf("After %q, params = %v, want %v", tt.cmd, got, tt.expected)
			}
		})
	}
}

func TestPositionalParamsInArgs(t *testing.T) {
	state := GetGlobalState()

	// Save and restore
	origParams := state.GetPositionalParams()
	origScriptName := state.GetScriptName()
	defer func() {
		state.SetPositionalParams(origParams)
		state.SetScriptName(origScriptName)
	}()

	state.SetScriptName("test.sh")
	state.SetPositionalParams([]string{"alice", "bob", "charlie"})

	args := []string{"echo", "User:", "$1", "Script:", "$0", "Count:", "$#"}
	expanded := ExpandVariablesInArgs(args)

	expected := []string{"echo", "User:", "alice", "Script:", "test.sh", "Count:", "3"}

	if !equalSlices(expanded, expected) {
		t.Errorf("ExpandVariablesInArgs() = %v, want %v", expanded, expected)
	}
}

// Helper functions
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func createTestCommand(t *testing.T, cmdStr string) *Command {
	t.Helper()

	cmd, err := NewCommand(cmdStr, NewJobManager())
	if err != nil {
		t.Fatalf("Failed to parse command %q: %v", cmdStr, err)
	}

	cmd.Stdout = &strings.Builder{}
	cmd.Stderr = &strings.Builder{}
	cmd.Stdin = strings.NewReader("")

	return cmd
}
