package gosh

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExpandSpecialVariables(t *testing.T) {
	// Initialize global state
	state := GetGlobalState()

	tests := []struct {
		name     string
		input    string
		setup    func()
		validate func(string) bool
		desc     string
	}{
		{
			name:  "$$  - shell PID",
			input: "PID: $$",
			validate: func(result string) bool {
				expected := "PID: " + strconv.Itoa(os.Getpid())
				return result == expected
			},
			desc: "Should expand $$ to current shell PID",
		},
		{
			name:  "$PPID - parent PID",
			input: "Parent: $PPID",
			validate: func(result string) bool {
				expected := "Parent: " + strconv.Itoa(os.Getppid())
				return result == expected
			},
			desc: "Should expand $PPID to parent process PID",
		},
		{
			name:  "$? - exit status",
			input: "Status: $?",
			setup: func() {
				state.SetLastExitStatus(42)
			},
			validate: func(result string) bool {
				return result == "Status: 42"
			},
			desc: "Should expand $? to last exit status",
		},
		{
			name:  "$! - last background PID",
			input: "Last BG: $!",
			setup: func() {
				state.SetLastBackgroundPID(12345)
			},
			validate: func(result string) bool {
				return result == "Last BG: 12345"
			},
			desc: "Should expand $! to last background process PID",
		},
		{
			name:  "$RANDOM - random number",
			input: "Random: $RANDOM",
			validate: func(result string) bool {
				parts := strings.Split(result, " ")
				if len(parts) != 2 {
					return false
				}
				num, err := strconv.Atoi(parts[1])
				if err != nil {
					return false
				}
				// $RANDOM should be between 0 and 32767
				return num >= 0 && num < 32768
			},
			desc: "Should expand $RANDOM to a number between 0-32767",
		},
		{
			name:  "$SECONDS - shell uptime",
			input: "Uptime: $SECONDS",
			setup: func() {
				// Wait a brief moment to ensure SECONDS is > 0
				time.Sleep(10 * time.Millisecond)
			},
			validate: func(result string) bool {
				parts := strings.Split(result, " ")
				if len(parts) != 2 {
					return false
				}
				seconds, err := strconv.Atoi(parts[1])
				if err != nil {
					return false
				}
				// Should be >= 0 (we just started)
				return seconds >= 0
			},
			desc: "Should expand $SECONDS to seconds since shell start",
		},
		{
			name:  "Multiple special variables",
			input: "PID=$$ PPID=$PPID STATUS=$?",
			setup: func() {
				state.SetLastExitStatus(0)
			},
			validate: func(result string) bool {
				parts := strings.Fields(result)
				if len(parts) != 3 {
					return false
				}
				// Check PID
				if !strings.HasPrefix(parts[0], "PID=") {
					return false
				}
				pidStr := strings.TrimPrefix(parts[0], "PID=")
				pid, err := strconv.Atoi(pidStr)
				if err != nil || pid != os.Getpid() {
					return false
				}
				// Check PPID
				if !strings.HasPrefix(parts[1], "PPID=") {
					return false
				}
				ppidStr := strings.TrimPrefix(parts[1], "PPID=")
				ppid, err := strconv.Atoi(ppidStr)
				if err != nil || ppid != os.Getppid() {
					return false
				}
				// Check STATUS
				return parts[2] == "STATUS=0"
			},
			desc: "Should expand multiple special variables in one string",
		},
		{
			name:  "Environment variable expansion",
			input: "Home: $HOME",
			setup: func() {
				os.Setenv("HOME", "/home/test")
			},
			validate: func(result string) bool {
				return result == "Home: /home/test"
			},
			desc: "Should expand regular environment variables",
		},
		{
			name:  "Braced variable expansion",
			input: "Path: ${PATH}",
			setup: func() {
				os.Setenv("PATH", "/usr/bin:/bin")
			},
			validate: func(result string) bool {
				return result == "Path: /usr/bin:/bin"
			},
			desc: "Should expand ${VAR} syntax",
		},
		{
			name:  "No expansion for undefined variables",
			input: "Var: $UNDEFINED_VAR",
			validate: func(result string) bool {
				return result == "Var: "
			},
			desc: "Should expand undefined variables to empty string",
		},
		{
			name:  "Mixed special and environment variables",
			input: "PID=$$ HOME=$HOME",
			setup: func() {
				os.Setenv("HOME", "/home/user")
			},
			validate: func(result string) bool {
				expected := "PID=" + strconv.Itoa(os.Getpid()) + " HOME=/home/user"
				return result == expected
			},
			desc: "Should expand both special and environment variables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			result := ExpandSpecialVariables(tt.input)

			if !tt.validate(result) {
				t.Errorf("%s\nInput: %q\nGot: %q", tt.desc, tt.input, result)
			}
		})
	}
}

func TestExpandVariablesInArgs(t *testing.T) {
	state := GetGlobalState()
	state.SetLastExitStatus(0)
	state.SetLastBackgroundPID(999)

	os.Setenv("USER", "testuser")
	os.Setenv("SHELL", "/bin/gosh")

	args := []string{
		"echo",
		"PID:$$",
		"$USER",
		"${SHELL}",
		"status=$?",
		"bg=$!",
	}

	expanded := ExpandVariablesInArgs(args)

	// Check that we got the right number of args
	if len(expanded) != len(args) {
		t.Fatalf("Expected %d args, got %d", len(args), len(expanded))
	}

	// Check each expansion
	if expanded[0] != "echo" {
		t.Errorf("arg[0]: got %q, want %q", expanded[0], "echo")
	}

	expectedPID := "PID:" + strconv.Itoa(os.Getpid())
	if expanded[1] != expectedPID {
		t.Errorf("arg[1]: got %q, want %q", expanded[1], expectedPID)
	}

	if expanded[2] != "testuser" {
		t.Errorf("arg[2]: got %q, want %q", expanded[2], "testuser")
	}

	if expanded[3] != "/bin/gosh" {
		t.Errorf("arg[3]: got %q, want %q", expanded[3], "/bin/gosh")
	}

	if expanded[4] != "status=0" {
		t.Errorf("arg[4]: got %q, want %q", expanded[4], "status=0")
	}

	if expanded[5] != "bg=999" {
		t.Errorf("arg[5]: got %q, want %q", expanded[5], "bg=999")
	}
}

func TestSpecialVariablesIntegration(t *testing.T) {
	// Test that variables are properly tracked through command execution
	state := GetGlobalState()

	// Test $$ is set
	if state.GetShellPID() != os.Getpid() {
		t.Errorf("ShellPID: got %d, want %d", state.GetShellPID(), os.Getpid())
	}

	// Test $? starts at 0
	if state.GetLastExitStatus() != 0 {
		t.Errorf("Initial LastExitStatus: got %d, want 0", state.GetLastExitStatus())
	}

	// Test setting and getting exit status
	state.SetLastExitStatus(42)
	if state.GetLastExitStatus() != 42 {
		t.Errorf("After set LastExitStatus: got %d, want 42", state.GetLastExitStatus())
	}

	// Test setting and getting background PID
	state.SetLastBackgroundPID(12345)
	if state.GetLastBackgroundPID() != 12345 {
		t.Errorf("After set LastBackgroundPID: got %d, want 12345", state.GetLastBackgroundPID())
	}

	// Test $SECONDS increases over time
	seconds1 := state.GetSeconds()
	time.Sleep(1 * time.Second)
	seconds2 := state.GetSeconds()
	if seconds2 <= seconds1 {
		t.Errorf("$SECONDS should increase over time: got %d then %d", seconds1, seconds2)
	}
}

func TestRANDOMUniqueness(t *testing.T) {
	// Test that $RANDOM generates different values
	values := make(map[string]bool)
	for i := 0; i < 100; i++ {
		result := ExpandSpecialVariables("$RANDOM")
		values[result] = true
	}

	// We should get at least some unique values (not all 100 will be unique due to randomness)
	if len(values) < 50 {
		t.Errorf("$RANDOM should generate diverse values, got only %d unique values in 100 tries", len(values))
	}
}
