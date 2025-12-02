package gosh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestScriptExecution tests basic script execution
func TestScriptExecution(t *testing.T) {
	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test_script.sh")

	scriptContent := `#!/usr/bin/env gosh
echo Hello from script
echo Line 2
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	// Build the gosh binary if it doesn't exist
	goshBin := buildGoshBinary(t)

	// Execute the script
	cmd := exec.Command(goshBin, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Hello from script") {
		t.Errorf("Expected 'Hello from script' in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Line 2") {
		t.Errorf("Expected 'Line 2' in output, got: %s", outputStr)
	}
}

// TestScriptPositionalParams tests positional parameters in scripts
func TestScriptPositionalParams(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "params_script.sh")

	scriptContent := `#!/usr/bin/env gosh
echo Script name: $0
echo First arg: $1
echo Second arg: $2
echo All args: $@
echo Arg count: $#
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	goshBin := buildGoshBinary(t)

	// Execute with arguments
	cmd := exec.Command(goshBin, scriptPath, "arg1", "arg2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, scriptPath) {
		t.Errorf("Expected script path in $0, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "First arg: arg1") {
		t.Errorf("Expected 'First arg: arg1', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Second arg: arg2") {
		t.Errorf("Expected 'Second arg: arg2', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Arg count: 2") {
		t.Errorf("Expected 'Arg count: 2', got: %s", outputStr)
	}
}

// TestScriptExitCode tests script exit codes
func TestScriptExitCode(t *testing.T) {
	tests := []struct {
		name           string
		scriptContent  string
		expectedCode   int
		args           []string
	}{
		{
			name: "success exit code",
			scriptContent: `#!/usr/bin/env gosh
echo Success
true
`,
			expectedCode: 0,
		},
		{
			name: "failure exit code",
			scriptContent: `#!/usr/bin/env gosh
echo Before failure
false
echo This should still run
`,
			expectedCode: 1,
		},
		{
			name: "explicit exit code from test",
			scriptContent: `#!/usr/bin/env gosh
test -f /nonexistent/file
`,
			expectedCode: 1,
		},
	}

	goshBin := buildGoshBinary(t)
	tmpDir := t.TempDir()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name+".sh")
			if err := os.WriteFile(scriptPath, []byte(tt.scriptContent), 0755); err != nil {
				t.Fatalf("Failed to create script file: %v", err)
			}

			cmd := exec.Command(goshBin, append([]string{scriptPath}, tt.args...)...)
			err := cmd.Run()

			// Get the exit code
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				}
			}

			if exitCode != tt.expectedCode {
				t.Errorf("Expected exit code %d, got %d", tt.expectedCode, exitCode)
			}
		})
	}
}

// TestScriptComments tests that comments are properly ignored
func TestScriptComments(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "comments_script.sh")

	scriptContent := `#!/usr/bin/env gosh
# This is a comment
echo First line
# Another comment
echo Second line
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	goshBin := buildGoshBinary(t)
	cmd := exec.Command(goshBin, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "First line") {
		t.Errorf("Expected 'First line' in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Second line") {
		t.Errorf("Expected 'Second line' in output, got: %s", outputStr)
	}
	if strings.Contains(outputStr, "This is a comment") {
		t.Errorf("Comment should not appear in output, got: %s", outputStr)
	}
}

// TestScriptWithVariables tests variable usage in scripts
func TestScriptWithVariables(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "variables_script.sh")

	scriptContent := `#!/usr/bin/env gosh
export MY_VAR=hello
echo Variable value: $MY_VAR
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	goshBin := buildGoshBinary(t)
	cmd := exec.Command(goshBin, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Variable value: hello") {
		t.Errorf("Expected 'Variable value: hello', got: %s", outputStr)
	}
}

// TestScriptWithPipeline tests pipelines in scripts
func TestScriptWithPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "pipeline_script.sh")

	scriptContent := `#!/usr/bin/env gosh
echo -e "line1\nline2\nline3"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	goshBin := buildGoshBinary(t)
	cmd := exec.Command(goshBin, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "line1") {
		t.Errorf("Expected 'line1' in output, got: %s", outputStr)
	}
}

// TestScriptWithSubshell tests subshells in scripts
func TestScriptWithSubshell(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "subshell_script.sh")

	scriptContent := `#!/usr/bin/env gosh
export VAR=outer
(export VAR=inner; echo Subshell: $VAR)
echo After subshell: $VAR
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	goshBin := buildGoshBinary(t)
	cmd := exec.Command(goshBin, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Subshell: inner") {
		t.Errorf("Expected 'Subshell: inner', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "After subshell: outer") {
		t.Errorf("Expected 'After subshell: outer', got: %s", outputStr)
	}
}

// TestScriptWithCommandSubstitution tests command substitution in scripts
func TestScriptWithCommandSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "cmdsub_script.sh")

	scriptContent := `#!/usr/bin/env gosh
export RESULT=$(echo hello)
echo Result: $RESULT
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	goshBin := buildGoshBinary(t)
	cmd := exec.Command(goshBin, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Script execution failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Result: hello") {
		t.Errorf("Expected 'Result: hello', got: %s", outputStr)
	}
}

// TestScriptNonexistent tests error handling for nonexistent scripts
func TestScriptNonexistent(t *testing.T) {
	goshBin := buildGoshBinary(t)
	cmd := exec.Command(goshBin, "/nonexistent/script.sh")
	err := cmd.Run()

	if err == nil {
		t.Error("Expected error for nonexistent script, got nil")
	}
}

// buildGoshBinary builds the gosh binary for testing and returns the path
func buildGoshBinary(t *testing.T) string {
	t.Helper()

	// Build the binary in a temporary directory
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "gosh")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build gosh binary: %v\nOutput: %s", err, output)
	}

	return binaryPath
}
