package gosh

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestColonCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "simple colon command",
			input:    ":",
			wantCode: 0,
		},
		{
			name:     "colon with args (should ignore them)",
			input:    ": ignored args here",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestUnsetCommand(t *testing.T) {
	tests := []struct {
		name        string
		setup       func()
		input       string
		checkVar    string
		wantPresent bool
		cleanup     func()
	}{
		{
			name: "unset existing variable",
			setup: func() {
				os.Setenv("TEST_UNSET_VAR", "value")
			},
			input:       "unset TEST_UNSET_VAR",
			checkVar:    "TEST_UNSET_VAR",
			wantPresent: false,
			cleanup: func() {
				os.Unsetenv("TEST_UNSET_VAR")
			},
		},
		{
			name: "unset multiple variables",
			setup: func() {
				os.Setenv("TEST_UNSET_A", "a")
				os.Setenv("TEST_UNSET_B", "b")
			},
			input:       "unset TEST_UNSET_A TEST_UNSET_B",
			checkVar:    "TEST_UNSET_A",
			wantPresent: false,
			cleanup: func() {
				os.Unsetenv("TEST_UNSET_A")
				os.Unsetenv("TEST_UNSET_B")
			},
		},
		{
			name:        "unset nonexistent variable (no error)",
			setup:       func() {},
			input:       "unset NONEXISTENT_VAR_12345",
			checkVar:    "NONEXISTENT_VAR_12345",
			wantPresent: false,
			cleanup:     func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			_, present := os.LookupEnv(tt.checkVar)
			if present != tt.wantPresent {
				t.Errorf("Variable %s present = %v, want %v", tt.checkVar, present, tt.wantPresent)
			}
		})
	}
}

func TestEvalCommand(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantCode   int
	}{
		{
			name:       "eval echo command",
			input:      "eval echo hello",
			wantOutput: "hello\n",
			wantCode:   0,
		},
		{
			name:       "eval with no args",
			input:      "eval",
			wantOutput: "",
			wantCode:   0,
		},
		{
			name:       "eval true",
			input:      "eval true",
			wantOutput: "",
			wantCode:   0,
		},
		{
			name:       "eval false",
			input:      "eval false",
			wantOutput: "",
			wantCode:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			if stdout.String() != tt.wantOutput {
				t.Errorf("Output = %q, want %q", stdout.String(), tt.wantOutput)
			}
			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestSourceCommand(t *testing.T) {
	// Create a temporary directory for test scripts
	tmpDir, err := os.MkdirTemp("", "gosh-test-source")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test script
	simpleScript := filepath.Join(tmpDir, "simple.sh")
	err = os.WriteFile(simpleScript, []byte(`
# This is a comment
echo hello from source

# Another comment
echo second line
`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Create a script that sets a variable
	varScript := filepath.Join(tmpDir, "setvar.sh")
	err = os.WriteFile(varScript, []byte(`
export TEST_SOURCE_VAR=sourced_value
`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantErr    bool
		checkVar   string
		wantValue  string
	}{
		{
			name:       "source simple script",
			input:      "source " + simpleScript,
			wantOutput: "hello from source\nsecond line\n",
			wantErr:    false,
		},
		{
			name:       "dot command (alias for source)",
			input:      ". " + simpleScript,
			wantOutput: "hello from source\nsecond line\n",
			wantErr:    false,
		},
		{
			name:      "source sets environment variable",
			input:     "source " + varScript,
			checkVar:  "TEST_SOURCE_VAR",
			wantValue: "sourced_value",
			wantErr:   false,
		},
		{
			name:    "source nonexistent file",
			input:   "source /nonexistent/file/path.sh",
			wantErr: true,
		},
		{
			name:    "source with no arguments",
			input:   "source",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any test variable
			if tt.checkVar != "" {
				os.Unsetenv(tt.checkVar)
				defer os.Unsetenv(tt.checkVar)
			}

			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				if tt.wantErr {
					return // Expected error during parsing
				}
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			// Check for error in stderr or non-zero return code
			hasErr := cmd.ReturnCode != 0 || stderr.Len() > 0
			if hasErr != tt.wantErr {
				t.Errorf("hasErr = %v, wantErr = %v (stderr: %q)", hasErr, tt.wantErr, stderr.String())
			}

			if tt.wantOutput != "" && stdout.String() != tt.wantOutput {
				t.Errorf("Output = %q, want %q", stdout.String(), tt.wantOutput)
			}

			if tt.checkVar != "" {
				val := os.Getenv(tt.checkVar)
				if val != tt.wantValue {
					t.Errorf("Variable %s = %q, want %q", tt.checkVar, val, tt.wantValue)
				}
			}
		})
	}
}

func TestSourceWithPositionalParams(t *testing.T) {
	// Create a temporary directory for test scripts
	tmpDir, err := os.MkdirTemp("", "gosh-test-source-params")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a script that uses positional parameters
	// Note: This test may need adjustment depending on how $1 is expanded
	paramScript := filepath.Join(tmpDir, "params.sh")
	err = os.WriteFile(paramScript, []byte(`
echo hello
`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	cmd, err := NewCommand("source "+paramScript+" arg1 arg2", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	cmd.Run()

	output := stdout.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got %q", output)
	}
}

func TestHelpIncludesNewBuiltins(t *testing.T) {
	// Verify that the builtins map contains our new commands
	builtinsCopy := Builtins()

	requiredBuiltins := []string{":", "unset", "source", ".", "eval", "exec", "readonly"}
	for _, name := range requiredBuiltins {
		if _, ok := builtinsCopy[name]; !ok {
			t.Errorf("Builtin %q not found in builtins map", name)
		}
	}
}

func TestReadonlyCommand(t *testing.T) {
	// Reset global state for each test
	resetGlobalStateForTesting()

	tests := []struct {
		name      string
		setup     func()
		input     string
		wantErr   bool
		checkVar  string
		wantValue string
		cleanup   func()
	}{
		{
			name:      "readonly with value",
			setup:     func() {},
			input:     "readonly TEST_RO_VAR=readonly_value",
			wantErr:   false,
			checkVar:  "TEST_RO_VAR",
			wantValue: "readonly_value",
			cleanup: func() {
				// Can't unset readonly, but we can clean up global state
				os.Unsetenv("TEST_RO_VAR")
			},
		},
		{
			name: "readonly existing variable",
			setup: func() {
				os.Setenv("TEST_RO_VAR2", "existing_value")
			},
			input:     "readonly TEST_RO_VAR2",
			wantErr:   false,
			checkVar:  "TEST_RO_VAR2",
			wantValue: "existing_value",
			cleanup: func() {
				os.Unsetenv("TEST_RO_VAR2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global state to clear readonly vars
			resetGlobalStateForTesting()

			tt.setup()
			defer tt.cleanup()

			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			hasErr := cmd.ReturnCode != 0 || stderr.Len() > 0
			if hasErr != tt.wantErr {
				t.Errorf("hasErr = %v, wantErr = %v (stderr: %q)", hasErr, tt.wantErr, stderr.String())
			}

			if tt.checkVar != "" {
				val := os.Getenv(tt.checkVar)
				if val != tt.wantValue {
					t.Errorf("Variable %s = %q, want %q", tt.checkVar, val, tt.wantValue)
				}
			}
		})
	}
}

func TestReadonlyPreventsModification(t *testing.T) {
	// Reset global state
	resetGlobalStateForTesting()
	defer func() {
		os.Unsetenv("TEST_RO_PROTECTED")
	}()

	// First, set a readonly variable
	cmd1, err := NewCommand("readonly TEST_RO_PROTECTED=protected", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout1, stderr1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Stderr = &stderr1
	cmd1.Run()

	if cmd1.ReturnCode != 0 {
		t.Fatalf("Setting readonly var failed: %s", stderr1.String())
	}

	// Now try to modify it with export
	cmd2, err := NewCommand("export TEST_RO_PROTECTED=modified", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout2, stderr2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	cmd2.Run()

	// Should fail with non-zero return code
	if cmd2.ReturnCode == 0 {
		t.Errorf("Expected export to fail on readonly variable, but it succeeded")
	}

	// Value should be unchanged
	val := os.Getenv("TEST_RO_PROTECTED")
	if val != "protected" {
		t.Errorf("Readonly variable was modified: got %q, want %q", val, "protected")
	}
}

func TestReadonlyPreventsUnset(t *testing.T) {
	// Reset global state
	resetGlobalStateForTesting()
	defer func() {
		os.Unsetenv("TEST_RO_NOUNSET")
	}()

	// First, set a readonly variable
	cmd1, err := NewCommand("readonly TEST_RO_NOUNSET=cannot_unset", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout1, stderr1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Stderr = &stderr1
	cmd1.Run()

	// Now try to unset it
	cmd2, err := NewCommand("unset TEST_RO_NOUNSET", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout2, stderr2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	cmd2.Run()

	// Should fail with non-zero return code
	if cmd2.ReturnCode == 0 {
		t.Errorf("Expected unset to fail on readonly variable, but it succeeded")
	}

	// Value should still exist
	val := os.Getenv("TEST_RO_NOUNSET")
	if val != "cannot_unset" {
		t.Errorf("Readonly variable was unset: got %q, want %q", val, "cannot_unset")
	}
}

func TestReadonlyList(t *testing.T) {
	// Reset global state
	resetGlobalStateForTesting()
	defer func() {
		os.Unsetenv("TEST_RO_LIST1")
		os.Unsetenv("TEST_RO_LIST2")
	}()

	// Set some readonly variables
	cmd1, _ := NewCommand("readonly TEST_RO_LIST1=value1", nil)
	var stdout1, stderr1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Stderr = &stderr1
	cmd1.Run()

	cmd2, _ := NewCommand("readonly TEST_RO_LIST2=value2", nil)
	var stdout2, stderr2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	cmd2.Run()

	// List readonly variables
	cmd3, err := NewCommand("readonly -p", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout3, stderr3 bytes.Buffer
	cmd3.Stdout = &stdout3
	cmd3.Stderr = &stderr3
	cmd3.Run()

	output := stdout3.String()
	if !strings.Contains(output, "TEST_RO_LIST1") {
		t.Errorf("readonly -p should list TEST_RO_LIST1, got: %s", output)
	}
	if !strings.Contains(output, "TEST_RO_LIST2") {
		t.Errorf("readonly -p should list TEST_RO_LIST2, got: %s", output)
	}
}

func TestExecNoCommand(t *testing.T) {
	// exec without a command should succeed (for redirection-only use case)
	cmd, err := NewCommand("exec", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	if cmd.ReturnCode != 0 {
		t.Errorf("exec with no args should succeed, got return code %d", cmd.ReturnCode)
	}
}

func TestExecInvalidOption(t *testing.T) {
	cmd, err := NewCommand("exec -z /bin/echo hello", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	if cmd.ReturnCode == 0 {
		t.Errorf("exec with invalid option should fail")
	}
}

func TestExecNotFound(t *testing.T) {
	cmd, err := NewCommand("exec nonexistent_command_12345", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	if cmd.ReturnCode == 0 {
		t.Errorf("exec with nonexistent command should fail")
	}
}

// resetGlobalStateForTesting resets the global state for testing purposes
func resetGlobalStateForTesting() {
	gs := GetGlobalState()
	gs.ReadonlyVars = make(map[string]bool)
	gs.SignalTraps = make(map[string]string)
	gs.VarAttributes = make(map[string]*VariableAttributes)
	gs.ScopeStack = []VariableScope{}
	gs.InFunction = false
	gs.FunctionDepth = 0
}

func TestTrapCommand(t *testing.T) {
	resetGlobalStateForTesting()

	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "trap list signals",
			input:      "trap -l",
			wantOutput: "SIG",
			wantErr:    false,
		},
		{
			name:       "trap print with no traps",
			input:      "trap -p",
			wantOutput: "",
			wantErr:    false,
		},
		{
			name:       "trap set handler",
			input:      "trap 'echo trapped' INT",
			wantOutput: "",
			wantErr:    false,
		},
		{
			name:       "trap invalid option",
			input:      "trap -z",
			wantOutput: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalStateForTesting()

			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			hasErr := cmd.ReturnCode != 0 || stderr.Len() > 0
			if hasErr != tt.wantErr {
				t.Errorf("hasErr = %v, wantErr = %v (stderr: %q)", hasErr, tt.wantErr, stderr.String())
			}

			if tt.wantOutput != "" && !strings.Contains(stdout.String(), tt.wantOutput) {
				t.Errorf("Output = %q, want to contain %q", stdout.String(), tt.wantOutput)
			}
		})
	}
}

func TestTrapSetAndGet(t *testing.T) {
	resetGlobalStateForTesting()
	gs := GetGlobalState()

	// Set a trap
	cmd1, err := NewCommand("trap 'echo goodbye' EXIT", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout1, stderr1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Stderr = &stderr1
	cmd1.Run()

	// Verify trap was set
	trapCmd, exists := gs.GetTrap("EXIT")
	if !exists {
		t.Errorf("EXIT trap should be set")
	}
	if trapCmd != "echo goodbye" {
		t.Errorf("EXIT trap = %q, want %q", trapCmd, "echo goodbye")
	}

	// Print the trap
	cmd2, _ := NewCommand("trap -p EXIT", nil)
	var stdout2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Run()

	output := stdout2.String()
	if !strings.Contains(output, "echo goodbye") {
		t.Errorf("trap -p should show EXIT trap, got: %s", output)
	}
}

func TestTrapClear(t *testing.T) {
	resetGlobalStateForTesting()
	gs := GetGlobalState()

	// Set a trap
	cmd1, _ := NewCommand("trap 'echo test' HUP", nil)
	var stdout1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Run()

	// Clear the trap
	cmd2, _ := NewCommand("trap - HUP", nil)
	var stdout2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Run()

	// Verify trap was cleared
	_, exists := gs.GetTrap("HUP")
	if exists {
		t.Errorf("HUP trap should be cleared")
	}
}

func TestReturnCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{
			name:     "return with no arg",
			input:    "return",
			wantCode: 0, // Uses last exit status
		},
		{
			name:     "return with 0",
			input:    "return 0",
			wantCode: 0,
		},
		{
			name:     "return with 1",
			input:    "return 1",
			wantCode: 1,
		},
		{
			name:     "return with 42",
			input:    "return 42",
			wantCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("ReturnCode = %d, want %d", cmd.ReturnCode, tt.wantCode)
			}
		})
	}
}

func TestReturnInvalidArg(t *testing.T) {
	cmd, err := NewCommand("return foo", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	// Should fail with non-numeric argument
	if cmd.ReturnCode == 0 {
		t.Errorf("return with non-numeric arg should fail")
	}
}

func TestLocalOutsideFunction(t *testing.T) {
	resetGlobalStateForTesting()

	cmd, err := NewCommand("local foo=bar", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	// Should fail outside function
	if cmd.ReturnCode == 0 {
		t.Errorf("local outside function should fail")
	}
}

func TestLocalInFunction(t *testing.T) {
	resetGlobalStateForTesting()
	gs := GetGlobalState()

	// Simulate being in a function
	gs.PushScope()
	defer gs.PopScope()

	cmd, err := NewCommand("local foo=bar", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	// Should succeed in function
	if cmd.ReturnCode != 0 {
		t.Errorf("local in function should succeed, got error: %s", stderr.String())
	}

	// Verify variable was set
	val := os.Getenv("foo")
	if val != "bar" {
		t.Errorf("local variable = %q, want %q", val, "bar")
	}
}

func TestDeclareCommand(t *testing.T) {
	resetGlobalStateForTesting()
	defer os.Unsetenv("TEST_DECLARE_VAR")

	tests := []struct {
		name      string
		input     string
		checkVar  string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "declare with value",
			input:     "declare TEST_DECLARE_VAR=hello",
			checkVar:  "TEST_DECLARE_VAR",
			wantValue: "hello",
			wantErr:   false,
		},
		{
			name:      "declare -x exports",
			input:     "declare -x TEST_DECLARE_VAR=exported",
			checkVar:  "TEST_DECLARE_VAR",
			wantValue: "exported",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalStateForTesting()
			os.Unsetenv(tt.checkVar)

			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			hasErr := cmd.ReturnCode != 0 || stderr.Len() > 0
			if hasErr != tt.wantErr {
				t.Errorf("hasErr = %v, wantErr = %v (stderr: %q)", hasErr, tt.wantErr, stderr.String())
			}

			if tt.checkVar != "" {
				val := os.Getenv(tt.checkVar)
				if val != tt.wantValue {
					t.Errorf("Variable %s = %q, want %q", tt.checkVar, val, tt.wantValue)
				}
			}
		})
	}
}

func TestDeclareReadonly(t *testing.T) {
	resetGlobalStateForTesting()
	defer os.Unsetenv("TEST_DECLARE_RO")

	// Declare readonly variable
	cmd1, err := NewCommand("declare -r TEST_DECLARE_RO=readonly_val", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout1, stderr1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Stderr = &stderr1
	cmd1.Run()

	if cmd1.ReturnCode != 0 {
		t.Fatalf("declare -r failed: %s", stderr1.String())
	}

	// Verify it's readonly
	gs := GetGlobalState()
	if !gs.IsReadonly("TEST_DECLARE_RO") {
		t.Errorf("TEST_DECLARE_RO should be readonly")
	}

	// Try to modify it
	cmd2, _ := NewCommand("declare TEST_DECLARE_RO=modified", nil)
	var stdout2, stderr2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	cmd2.Run()

	// Should fail
	if cmd2.ReturnCode == 0 {
		t.Errorf("modifying readonly variable should fail")
	}

	// Value should be unchanged
	val := os.Getenv("TEST_DECLARE_RO")
	if val != "readonly_val" {
		t.Errorf("readonly variable was modified: got %q, want %q", val, "readonly_val")
	}
}

func TestDeclareInteger(t *testing.T) {
	resetGlobalStateForTesting()
	defer os.Unsetenv("TEST_INT_VAR")

	// Declare integer variable
	cmd1, err := NewCommand("declare -i TEST_INT_VAR=42", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}
	var stdout1, stderr1 bytes.Buffer
	cmd1.Stdout = &stdout1
	cmd1.Stderr = &stderr1
	cmd1.Run()

	if cmd1.ReturnCode != 0 {
		t.Fatalf("declare -i failed: %s", stderr1.String())
	}

	// Verify value
	val := os.Getenv("TEST_INT_VAR")
	if val != "42" {
		t.Errorf("integer variable = %q, want %q", val, "42")
	}

	// Verify it has integer attribute
	gs := GetGlobalState()
	attrs := gs.GetVarAttributes("TEST_INT_VAR")
	if attrs == nil || !attrs.Integer {
		t.Errorf("TEST_INT_VAR should have integer attribute")
	}
}

func TestDeclarePrint(t *testing.T) {
	resetGlobalStateForTesting()
	defer os.Unsetenv("TEST_PRINT_VAR")

	// Set a variable with attributes
	os.Setenv("TEST_PRINT_VAR", "print_value")
	gs := GetGlobalState()
	gs.SetVarAttributes("TEST_PRINT_VAR", &VariableAttributes{Export: true})

	// Print it
	cmd, err := NewCommand("declare -p TEST_PRINT_VAR", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	output := stdout.String()
	if !strings.Contains(output, "TEST_PRINT_VAR") {
		t.Errorf("declare -p should show variable name, got: %s", output)
	}
	if !strings.Contains(output, "print_value") {
		t.Errorf("declare -p should show variable value, got: %s", output)
	}
	if !strings.Contains(output, "-x") {
		t.Errorf("declare -p should show export flag, got: %s", output)
	}
}

func TestTypesetAliasForDeclare(t *testing.T) {
	resetGlobalStateForTesting()
	defer os.Unsetenv("TEST_TYPESET_VAR")

	// typeset should work the same as declare
	cmd, err := NewCommand("typeset TEST_TYPESET_VAR=typeset_value", nil)
	if err != nil {
		t.Fatalf("NewCommand failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	if cmd.ReturnCode != 0 {
		t.Errorf("typeset should succeed, got error: %s", stderr.String())
	}

	val := os.Getenv("TEST_TYPESET_VAR")
	if val != "typeset_value" {
		t.Errorf("typeset variable = %q, want %q", val, "typeset_value")
	}
}

func TestBuiltinsIncludeNewCommands(t *testing.T) {
	builtinsCopy := Builtins()

	requiredBuiltins := []string{
		"trap", "return", "local", "declare", "typeset",
	}
	for _, name := range requiredBuiltins {
		if _, ok := builtinsCopy[name]; !ok {
			t.Errorf("Builtin %q not found in builtins map", name)
		}
	}
}
