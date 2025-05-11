package gosh

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCdWithDash(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-cd-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Get the real path (resolving any symlinks like /var -> /private/var on macOS)
	realTempDir, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks: %v", err)
	}

	// Set up the initial environment
	originalDir, _ := os.Getwd() // Store original directory to restore later
	defer os.Chdir(originalDir)  // Restore original directory at the end

	// Create a subdirectory for testing
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Get the real path of subDir
	realSubDir, _ := filepath.EvalSymlinks(subDir)

	// Initialize the GlobalState
	gs := GetGlobalState()

	// Change to the temp directory and set up state
	os.Chdir(tempDir)
	gs.UpdateCWD(realTempDir)

	// Explicitly set OLDPWD to original directory
	os.Setenv("OLDPWD", originalDir)

	// Test first cd to subdir
	cmd1 := &Command{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	// Prepare a command string for "cd subdir"
	commandStr := "cd subdir"
	parsedCmd, _ := NewCommand(commandStr, NewJobManager())
	cmd1 = parsedCmd
	cmd1.Stdout = &bytes.Buffer{}
	cmd1.Stderr = &bytes.Buffer{}

	// Execute the cd command
	err = cd(cmd1)
	if err != nil {
		t.Fatalf("cd to subdir failed: %v", err)
	}

	// Verify we're in the subdirectory (compare real paths)
	currentDir, _ := os.Getwd()
	currentDirReal, _ := filepath.EvalSymlinks(currentDir)
	if currentDirReal != realSubDir {
		t.Errorf("Expected to be in %s, but got %s", realSubDir, currentDirReal)
	}

	// Now test cd -
	cmd2 := &Command{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	// Prepare a command string for "cd -"
	commandStr = "cd -"
	parsedCmd, _ = NewCommand(commandStr, NewJobManager())
	cmd2 = parsedCmd
	cmd2.Stdout = &bytes.Buffer{}
	cmd2.Stderr = &bytes.Buffer{}

	// Execute cd -
	err = cd(cmd2)
	if err != nil {
		t.Fatalf("cd - failed: %v", err)
	}

	// Verify we're back in the original temp directory (compare real paths)
	currentDir, _ = os.Getwd()
	currentDirReal, _ = filepath.EvalSymlinks(currentDir)
	if currentDirReal != realTempDir {
		t.Errorf("After cd -, expected to be in %s, but got %s", realTempDir, currentDirReal)
	}

	// Check output of cd - command (should print the directory)
	output := cmd2.Stdout.(*bytes.Buffer).String()
	output = strings.TrimSpace(output)
	if output != realTempDir && output != tempDir {
		t.Errorf("Expected cd - to output directory %s or %s, but got %s", realTempDir, tempDir, output)
	}

	log.Printf("cd - test successful, returned to %s", currentDir)
}
