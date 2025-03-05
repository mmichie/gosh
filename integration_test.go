package gosh

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestIntegration(t *testing.T) {
	// Set up logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set the temporary directory as HOME
	os.Setenv("HOME", tempDir)
	defer os.Unsetenv("HOME")

	tests := []struct {
		name     string
		input    string
		expected interface{}
		setup    func() error
		cleanup  func() error
	}{
		{
			name:     "Simple echo command",
			input:    "echo Hello, World!",
			expected: "Hello, World!\n",
		},
		{
			name:     "Change directory",
			input:    "cd /tmp && pwd",
			expected: "/tmp\n",
		},
		{
			name:     "Pipe commands",
			input:    "echo Hello, World! | wc -w",
			expected: regexp.MustCompile(`\s*2\s*`),
		},
		{
			name:  "Multiple pipes",
			input: "echo 'one two three four' | tr ' ' '\n' | sort | uniq -c | sort -nr",
			expected: regexp.MustCompile(`\s*1\s+two\s*
\s*1\s+three\s*
\s*1\s+one\s*
\s*1\s+four\s*`),
		},
		{
			name:     "Environment variable",
			input:    "export TEST_VAR=hello && echo $TEST_VAR",
			expected: "hello\n",
		},
		{
			name:  "CD with dash",
			input: "cd /tmp && pwd && cd - && pwd",
			expected: regexp.MustCompile(`(?:/private)?/tmp\n` +
				`(?:/private)?` + regexp.QuoteMeta(tempDir) + "\n"),
		},
		{
			name:     "File creation and content verification",
			input:    "echo 'test content' > test.txt && cat test.txt && ls test.txt",
			expected: "test content\ntest.txt",
			setup: func() error {
				return os.Chdir(tempDir)
			},
			cleanup: func() error {
				filePath := filepath.Join(tempDir, "test.txt")
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					return fmt.Errorf("file %s does not exist after test execution: %v", filePath, err)
				}
				return os.Remove(filePath)
			},
		},
		{
			name:     "File permissions modification",
			input:    "touch testfile && chmod 755 testfile && ls -l testfile",
			expected: regexp.MustCompile(`-rwx`), // Just check for the rwx permissions
			setup: func() error {
				return os.Chdir(tempDir)
			},
			cleanup: func() error {
				return os.Remove(filepath.Join(tempDir, "testfile"))
			},
		},
		{
			name:     "Process listing and filtering",
			input:    "ps aux | grep bash | wc -l",
			expected: regexp.MustCompile(`[0-9]+`), // Just check for any number in the output
		},
		{
			name:     "File searching",
			input:    "touch file1.txt file2.txt file3.dat && ls",
			expected: regexp.MustCompile(`(?s)file1\.txt.*file2\.txt.*file3\.dat`), // (?s) makes dot match newlines too
			setup: func() error {
				return os.Chdir(tempDir)
			},
			cleanup: func() error {
				os.Remove(filepath.Join(tempDir, "file1.txt"))
				os.Remove(filepath.Join(tempDir, "file2.txt"))
				os.Remove(filepath.Join(tempDir, "file3.dat"))
				return nil
			},
		},
		{
			name:     "Text processing with sed",
			input:    "echo Hello, World! | tr 'W' 'U'",
			expected: regexp.MustCompile(`Hello, Uorld!`),
		},
		{
			name:     "Archive creation and extraction",
			input:    "mkdir testdir && touch testdir/file1 testdir/file2 && tar -czf archive.tar.gz testdir && rm -r testdir && tar -xzf archive.tar.gz && ls testdir",
			expected: "file1\nfile2\n",
			setup: func() error {
				return os.Chdir(tempDir)
			},
			cleanup: func() error {
				os.RemoveAll(filepath.Join(tempDir, "testdir"))
				os.Remove(filepath.Join(tempDir, "archive.tar.gz"))
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.Printf("--- Starting test: %s ---", tt.name)
			log.Printf("Input command: %s", tt.input)

			// Reset global state for each test
			gs := GetGlobalState()
			gs.UpdateCWD(tempDir)

			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			// Run the command
			jobManager := NewJobManager()
			cmds := strings.Split(tt.input, " && ")
			var output bytes.Buffer
			for _, cmdStr := range cmds {
				cmd, err := NewCommand(cmdStr, jobManager)
				if err != nil {
					t.Fatalf("Failed to create command: %v", err)
				}
				cmd.Stdout = &output
				cmd.Stderr = &output
				cmd.Run()
				// Check if there was an error in command execution
				if cmd.ReturnCode != 0 {
					log.Printf("Command execution error: Return code %d", cmd.ReturnCode)
				}
			}

			// Read captured output
			capturedOutput := output.String()

			log.Printf("Captured output:\n%s", capturedOutput)

			// Compare output
			switch expected := tt.expected.(type) {
			case string:
				if !strings.Contains(capturedOutput, expected) {
					t.Errorf("Test case: %s\nCommand: %s\nExpected output to contain:\n%s\nBut got:\n%s\n", tt.name, tt.input, expected, capturedOutput)
				}
			case *regexp.Regexp:
				if !expected.MatchString(capturedOutput) {
					t.Errorf("Test case: %s\nCommand: %s\nExpected output to match regex:\n%s\nBut got:\n%s\n", tt.name, tt.input, expected, capturedOutput)
				}
			default:
				t.Errorf("Test case: %s has an invalid expectation type", tt.name)
			}

			if tt.cleanup != nil {
				if err := tt.cleanup(); err != nil {
					t.Errorf("Cleanup failed: %v", err)
				}
			}

			log.Printf("--- Finished test: %s ---\n", tt.name)
		})
	}
}