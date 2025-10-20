package gosh

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gosh/parser"
)

func TestTestCommand(t *testing.T) {
	// Create temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "gosh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	execFile := filepath.Join(tmpDir, "executable.sh")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create executable file: %v", err)
	}

	dir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(regularFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	nonExistent := filepath.Join(tmpDir, "does-not-exist")

	tests := []struct {
		name       string
		args       []string
		wantResult int // 0 = true, 1 = false, 2 = error
	}{
		// Empty test
		{"empty test", []string{"test"}, 1},

		// Single argument (non-empty string is true)
		{"single arg non-empty", []string{"test", "hello"}, 0},
		// Note: testing empty strings via parser is tricky, so we skip this case
		// Real usage: [ -z "$var" ] or [ "$var" ] works correctly

		// File existence tests
		{"file exists", []string{"test", "-e", regularFile}, 0},
		{"file not exists", []string{"test", "-e", nonExistent}, 1},

		// Regular file tests
		{"is regular file", []string{"test", "-f", regularFile}, 0},
		{"is not regular file (dir)", []string{"test", "-f", dir}, 1},
		{"is not regular file (nonexistent)", []string{"test", "-f", nonExistent}, 1},

		// Directory tests
		{"is directory", []string{"test", "-d", dir}, 0},
		{"is not directory (file)", []string{"test", "-d", regularFile}, 1},
		{"is not directory (nonexistent)", []string{"test", "-d", nonExistent}, 1},

		// Readable tests
		{"is readable", []string{"test", "-r", regularFile}, 0},
		{"is not readable (nonexistent)", []string{"test", "-r", nonExistent}, 1},

		// Writable tests
		{"is writable", []string{"test", "-w", regularFile}, 0},
		{"is not writable (nonexistent)", []string{"test", "-w", nonExistent}, 1},

		// Executable tests
		{"is executable", []string{"test", "-x", execFile}, 0},
		{"is not executable", []string{"test", "-x", regularFile}, 1},

		// Size tests
		{"file has size", []string{"test", "-s", regularFile}, 0},
		{"file empty", []string{"test", "-s", emptyFile}, 1},
		{"file not exists", []string{"test", "-s", nonExistent}, 1},

		// Symlink tests
		{"is symlink", []string{"test", "-L", symlink}, 0},
		{"is not symlink", []string{"test", "-L", regularFile}, 1},

		// String empty/non-empty tests
		// Note: Testing with actual empty strings requires special handling
		{"string is not empty (-z)", []string{"test", "-z", "hello"}, 1},
		{"string is not empty (-n)", []string{"test", "-n", "hello"}, 0},

		// String equality tests
		{"strings equal (=)", []string{"test", "hello", "=", "hello"}, 0},
		{"strings not equal (=)", []string{"test", "hello", "=", "world"}, 1},
		{"strings equal (==)", []string{"test", "hello", "==", "hello"}, 0},
		{"strings not equal (!=)", []string{"test", "hello", "!=", "world"}, 0},
		{"strings equal (!=)", []string{"test", "hello", "!=", "hello"}, 1},

		// String ordering (Note: < and > are not POSIX standard test operators)
		// These are bash extensions and conflict with redirection, so we skip them
		// Use numeric -lt, -gt instead for comparisons

		// Numeric equality tests
		{"numbers equal", []string{"test", "42", "-eq", "42"}, 0},
		{"numbers not equal", []string{"test", "42", "-eq", "43"}, 1},
		{"numbers not equal (ne)", []string{"test", "42", "-ne", "43"}, 0},
		{"numbers equal (ne)", []string{"test", "42", "-ne", "42"}, 1},

		// Numeric comparison tests
		{"less than", []string{"test", "5", "-lt", "10"}, 0},
		{"not less than", []string{"test", "10", "-lt", "5"}, 1},
		{"less or equal", []string{"test", "5", "-le", "5"}, 0},
		{"less or equal (less)", []string{"test", "5", "-le", "10"}, 0},
		{"not less or equal", []string{"test", "10", "-le", "5"}, 1},
		{"greater than", []string{"test", "10", "-gt", "5"}, 0},
		{"not greater than", []string{"test", "5", "-gt", "10"}, 1},
		{"greater or equal", []string{"test", "5", "-ge", "5"}, 0},
		{"greater or equal (greater)", []string{"test", "10", "-ge", "5"}, 0},
		{"not greater or equal", []string{"test", "5", "-ge", "10"}, 1},

		// Negation tests
		{"negation of true", []string{"test", "!", "-f", regularFile}, 1},
		{"negation of false", []string{"test", "!", "-f", nonExistent}, 0},
		{"negation of empty string", []string{"test", "!", "-z", "hello"}, 0},

		// Logical AND tests
		{"and both true", []string{"test", "-f", regularFile, "-a", "-r", regularFile}, 0},
		{"and first false", []string{"test", "-f", nonExistent, "-a", "-r", regularFile}, 1},
		{"and second false", []string{"test", "-f", regularFile, "-a", "-r", nonExistent}, 1},
		{"and both false", []string{"test", "-f", nonExistent, "-a", "-r", nonExistent}, 1},

		// Logical OR tests
		{"or both true", []string{"test", "-f", regularFile, "-o", "-r", regularFile}, 0},
		{"or first true", []string{"test", "-f", regularFile, "-o", "-r", nonExistent}, 0},
		{"or second true", []string{"test", "-f", nonExistent, "-o", "-r", regularFile}, 0},
		{"or both false", []string{"test", "-f", nonExistent, "-o", "-r", nonExistent}, 1},

		// Complex expressions (AND has higher precedence than OR)
		{"complex or-and", []string{"test", "-f", nonExistent, "-o", "-f", regularFile, "-a", "-r", regularFile}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestBracketCommand(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gosh-bracket-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Basic bracket tests
		{"bracket file exists", []string{"[", "-f", tmpFile.Name(), "]"}, 0},
		{"bracket file not exists", []string{"[", "-f", "/nonexistent", "]"}, 1},
		{"bracket string equal", []string{"[", "hello", "=", "hello", "]"}, 0},
		{"bracket string not equal", []string{"[", "hello", "=", "world", "]"}, 1},
		{"bracket number equal", []string{"[", "42", "-eq", "42", "]"}, 0},
		{"bracket number not equal", []string{"[", "42", "-eq", "43", "]"}, 1},

		// Missing closing bracket should return error (exit code 2)
		{"missing closing bracket", []string{"[", "-f", tmpFile.Name()}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

// runTestCommand executes the test command and returns the exit code
func runTestCommand(t *testing.T, args []string) int {
	// Build the command string
	cmdString := ""
	for i, arg := range args {
		if i > 0 {
			cmdString += " "
		}
		// Quote arguments that need it
		needsQuoting := false
		if arg == "" {
			needsQuoting = true
		} else if arg == "<" || arg == ">" {
			// These are redirection operators in shell, need quoting
			needsQuoting = true
		}

		if needsQuoting {
			cmdString += "'" + arg + "'"
		} else {
			cmdString += arg
		}
	}

	// Parse the command
	parsedCmd, err := parser.Parse(cmdString)
	if err != nil {
		t.Fatalf("Failed to parse command %q: %v", cmdString, err)
	}

	// Create the command
	var stdout, stderr bytes.Buffer
	cmd := &Command{
		Command:    parsedCmd,
		Stdin:      os.Stdin,
		Stdout:     &stdout,
		Stderr:     &stderr,
		JobManager: NewJobManager(),
	}

	// Execute the command
	testCommand(cmd)

	return cmd.ReturnCode
}

func TestTestCommandEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Edge cases
		// In POSIX test, a single argument is tested for non-emptiness
		{"single arg treated as string", []string{"test", "-invalid"}, 0}, // "-invalid" is non-empty, returns true
		{"single arg -f is string", []string{"test", "-f"}, 0},             // "-f" is non-empty, returns true
		{"too many arguments no operators", []string{"test", "a", "b", "c", "d"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}
