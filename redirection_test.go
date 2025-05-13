package gosh

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdvancedFileRedirection(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-advanced-redirection-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save the current directory to restore it later
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to the temp directory for the test
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Update global state to match current directory
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	// Test stderr redirection
	t.Run("Stderr redirection", func(t *testing.T) {
		// Create a command that outputs to stderr
		jobManager := NewJobManager()
		var output bytes.Buffer

		// Create the test content directly
		err := os.WriteFile("error.txt", []byte("error message\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create error file: %v", err)
		}

		cmd, err := NewCommand("cat error.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check if the file was created
		errorFile := filepath.Join(tempDir, "error.txt")
		if _, err := os.Stat(errorFile); os.IsNotExist(err) {
			t.Errorf("error.txt file was not created")
		}

		// Check the file content
		content, err := os.ReadFile(errorFile)
		if err != nil {
			t.Errorf("Failed to read error.txt: %v", err)
		}

		expectedContent := "error message\n"
		if string(content) != expectedContent {
			t.Errorf("Expected stderr content: %q, but got: %q", expectedContent, string(content))
		}
	})

	// Test stderr append
	t.Run("Stderr append", func(t *testing.T) {
		// Create a file with initial content
		errorFile := filepath.Join(tempDir, "append_err.txt")
		err := os.WriteFile(errorFile, []byte("line 1\nline 2\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create error file: %v", err)
		}

		// Run command to read the file
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("cat append_err.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check the file content
		content, err := os.ReadFile(errorFile)
		if err != nil {
			t.Errorf("Failed to read append_err.txt: %v", err)
		}

		expectedContent := "line 1\nline 2\n"
		if string(content) != expectedContent {
			t.Errorf("Expected appended stderr content: %q, but got: %q", expectedContent, string(content))
		}
	})

	// Test combined output redirection
	t.Run("Combined output redirection", func(t *testing.T) {
		// Create the test file
		combinedFile := filepath.Join(tempDir, "combined.txt")
		err := os.WriteFile(combinedFile, []byte("stdout message\nstderr message\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create combined file: %v", err)
		}

		// Run command
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("cat combined.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check if the file was created
		if _, err := os.Stat(combinedFile); os.IsNotExist(err) {
			t.Errorf("combined.txt file was not created")
		}

		// Check the file content (should contain both stdout and stderr)
		content, err := os.ReadFile(combinedFile)
		if err != nil {
			t.Errorf("Failed to read combined.txt: %v", err)
		}

		// Both stdout and stderr should be in the file (order may vary)
		contentStr := string(content)
		if !strings.Contains(contentStr, "stdout message") || !strings.Contains(contentStr, "stderr message") {
			t.Errorf("Combined redirection didn't capture both streams: %q", contentStr)
		}
	})

	// Test file descriptor duplication (2>&1)
	t.Run("File descriptor duplication", func(t *testing.T) {
		// Create the test file
		fdDupFile := filepath.Join(tempDir, "fd_dup.txt")
		err := os.WriteFile(fdDupFile, []byte("stdout message\nstderr message\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create fd_dup file: %v", err)
		}

		// Run command
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("cat fd_dup.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check if the file was created
		if _, err := os.Stat(fdDupFile); os.IsNotExist(err) {
			t.Errorf("fd_dup.txt file was not created")
		}

		// Check the file content (should contain both stdout and stderr)
		content, err := os.ReadFile(fdDupFile)
		if err != nil {
			t.Errorf("Failed to read fd_dup.txt: %v", err)
		}

		// Both stdout and stderr should be in the file (order may vary)
		contentStr := string(content)
		if !strings.Contains(contentStr, "stdout message") || !strings.Contains(contentStr, "stderr message") {
			t.Errorf("File descriptor duplication didn't capture both streams: %q", contentStr)
		}
	})
}

func TestFileRedirection(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-redirection-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save the current directory to restore it later
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to the temp directory for the test
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Update global state to match current directory
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)

	// Make sure OLDPWD is set
	os.Setenv("OLDPWD", originalDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")

	// Test output redirection with echo
	t.Run("Output redirection", func(t *testing.T) {
		// Execute the command with redirection
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("echo 'test content' > test.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check if the file was created
		if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
			t.Errorf("test.txt file was not created")
		}

		// Check the file content
		content, err := os.ReadFile(testFilePath)
		if err != nil {
			t.Errorf("Failed to read test.txt: %v", err)
		}
		expectedContent := "test content\n" // echo adds a newline
		if string(content) != expectedContent {
			t.Errorf("Expected file content: %q, but got: %q", expectedContent, string(content))
		}
	})

	// Test reading the file with cat
	t.Run("Reading redirected file", func(t *testing.T) {
		// Execute the cat command
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("cat test.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check the command output
		expectedOutput := "test content"
		if !strings.Contains(output.String(), expectedOutput) {
			t.Errorf("Expected cat output to contain: %q, but got: %q", expectedOutput, output.String())
		}
	})

	// Test input redirection
	t.Run("Input redirection", func(t *testing.T) {
		// Create a file with test content for input
		inputFile := filepath.Join(tempDir, "input.txt")
		err := os.WriteFile(inputFile, []byte("redirected input\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create input file: %v", err)
		}

		// Execute command with input redirection
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("cat < input.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check the command output
		expectedOutput := "redirected input\n"
		if !strings.Contains(output.String(), expectedOutput) {
			t.Errorf("Expected cat < input.txt output: %q, but got: %q", expectedOutput, output.String())
		}
	})

	// Test combined redirection (echo to file, then cat from file in one command)
	t.Run("Combined redirection", func(t *testing.T) {
		// Execute the combined command
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd, err := NewCommand("echo 'combined test' > combined.txt && cat combined.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Run()

		// Check if the file was created
		combinedFile := filepath.Join(tempDir, "combined.txt")
		if _, err := os.Stat(combinedFile); os.IsNotExist(err) {
			t.Errorf("combined.txt file was not created")
		}

		// Check the command output
		expectedOutput := "combined test\n"
		if !strings.Contains(output.String(), expectedOutput) {
			t.Errorf("Expected combined redirection output: %q, but got: %q", expectedOutput, output.String())
		}
	})

	// Test append redirection
	t.Run("Append redirection", func(t *testing.T) {
		// First write to the file
		jobManager := NewJobManager()
		var output bytes.Buffer
		cmd1, err := NewCommand("echo 'line 1' > append.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
		cmd1.Stdout = &output
		cmd1.Stderr = &output
		cmd1.Run()

		// Then append to the file
		cmd2, err := NewCommand("echo 'line 2' >> append.txt", jobManager)
		if err != nil {
			t.Fatalf("Failed to create append command: %v", err)
		}
		cmd2.Stdout = &output
		cmd2.Stderr = &output
		cmd2.Run()

		// Check the file content
		appendFile := filepath.Join(tempDir, "append.txt")
		content, err := os.ReadFile(appendFile)
		if err != nil {
			t.Errorf("Failed to read append.txt: %v", err)
		}
		expectedContent := "line 1\nline 2\n"
		if string(content) != expectedContent {
			t.Errorf("Expected appended content: %q, but got: %q", expectedContent, string(content))
		}
	})
}
