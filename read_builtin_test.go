package gosh

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"gosh/parser"
)

func TestReadCommand(t *testing.T) {
	tests := []struct {
		name       string
		cmd        string
		input      string
		wantVars   map[string]string
		wantCode   int
		wantPrompt string
	}{
		{
			name:     "basic read single variable",
			cmd:      "read name",
			input:    "Alice\n",
			wantVars: map[string]string{"name": "Alice"},
			wantCode: 0,
		},
		{
			name:     "read with prompt",
			cmd:      "read -p 'Enter name: ' name",
			input:    "Bob\n",
			wantVars: map[string]string{"name": "Bob"},
			wantCode: 0,
			wantPrompt: "Enter name: ",
		},
		{
			name:     "read multiple variables",
			cmd:      "read first last",
			input:    "John Doe\n",
			wantVars: map[string]string{"first": "John", "last": "Doe"},
			wantCode: 0,
		},
		{
			name:     "read multiple variables - last gets remaining",
			cmd:      "read first middle last",
			input:    "John Michael Smith Jr\n",
			wantVars: map[string]string{"first": "John", "middle": "Michael", "last": "Smith Jr"},
			wantCode: 0,
		},
		{
			name:     "read fewer fields than variables",
			cmd:      "read first last email",
			input:    "Alice\n",
			wantVars: map[string]string{"first": "Alice", "last": "", "email": ""},
			wantCode: 0,
		},
		{
			name:     "read into REPLY when no variables",
			cmd:      "read",
			input:    "default value\n",
			wantVars: map[string]string{"REPLY": "default value"},
			wantCode: 0,
		},
		{
			name:     "read empty line",
			cmd:      "read value",
			input:    "\n",
			wantVars: map[string]string{"value": ""},
			wantCode: 0,
		},
		{
			name:     "read with custom IFS",
			cmd:      "read a b c",
			input:    "one:two:three\n",
			wantVars: map[string]string{"a": "one", "b": "two", "c": "three"},
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			for varName := range tt.wantVars {
				os.Unsetenv(varName)
			}

			// Set up custom IFS if test uses it
			oldIFS := os.Getenv("IFS")
			if strings.Contains(tt.input, ":") && strings.Contains(tt.cmd, "a b c") {
				os.Setenv("IFS", ":")
			}
			defer func() {
				if oldIFS == "" {
					os.Unsetenv("IFS")
				} else {
					os.Setenv("IFS", oldIFS)
				}
			}()

			// Parse command
			parsedCmd, err := parser.Parse(tt.cmd)
			if err != nil {
				t.Fatalf("Failed to parse command: %v", err)
			}

			// Create command with input
			var stdout bytes.Buffer
			cmd := &Command{
				Command:    parsedCmd,
				Stdin:      strings.NewReader(tt.input),
				Stdout:     &stdout,
				Stderr:     &stdout,
				JobManager: NewJobManager(),
			}

			// Execute
			err = readCommand(cmd)
			if err != nil {
				t.Fatalf("read command failed: %v", err)
			}

			// Check exit code
			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("got exit code %d, want %d", cmd.ReturnCode, tt.wantCode)
			}

			// Check prompt was written
			if tt.wantPrompt != "" {
				output := stdout.String()
				if !strings.Contains(output, tt.wantPrompt) {
					t.Errorf("prompt not found in output: got %q, want to contain %q", output, tt.wantPrompt)
				}
			}

			// Check variables
			for varName, wantValue := range tt.wantVars {
				gotValue := os.Getenv(varName)
				if gotValue != wantValue {
					t.Errorf("variable %s: got %q, want %q", varName, gotValue, wantValue)
				}
			}
		})
	}
}

func TestReadCommandNChars(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		input    string
		wantVar  string
		wantCode int
	}{
		{
			name:     "read 5 characters",
			cmd:      "read -n 5 code",
			input:    "12345extra\n",
			wantVar:  "12345",
			wantCode: 0,
		},
		{
			name:     "read 3 characters",
			cmd:      "read -n 3 short",
			input:    "abc",
			wantVar:  "abc",
			wantCode: 0,
		},
		{
			name:     "read 10 characters from short input",
			cmd:      "read -n 10 value",
			input:    "short",
			wantVar:  "short",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("code")
			os.Unsetenv("short")
			os.Unsetenv("value")

			parsedCmd, err := parser.Parse(tt.cmd)
			if err != nil {
				t.Fatalf("Failed to parse command: %v", err)
			}

			var stdout bytes.Buffer
			cmd := &Command{
				Command:    parsedCmd,
				Stdin:      strings.NewReader(tt.input),
				Stdout:     &stdout,
				Stderr:     &stdout,
				JobManager: NewJobManager(),
			}

			err = readCommand(cmd)
			if err != nil {
				t.Fatalf("read command failed: %v", err)
			}

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("got exit code %d, want %d", cmd.ReturnCode, tt.wantCode)
			}

			// Extract variable name from command
			varName := strings.Fields(tt.cmd)[len(strings.Fields(tt.cmd))-1]
			gotValue := os.Getenv(varName)
			if gotValue != tt.wantVar {
				t.Errorf("got %q, want %q", gotValue, tt.wantVar)
			}
		})
	}
}

func TestReadCommandTimeout(t *testing.T) {
	// Test timeout - should timeout and return error
	t.Run("timeout expires", func(t *testing.T) {
		os.Unsetenv("value")

		parsedCmd, err := parser.Parse("read -t 0.1 value")
		if err != nil {
			t.Fatalf("Failed to parse command: %v", err)
		}

		// Use a reader that never provides data
		pr, _, _ := os.Pipe()
		defer pr.Close()

		var stdout bytes.Buffer
		cmd := &Command{
			Command:    parsedCmd,
			Stdin:      pr,
			Stdout:     &stdout,
			Stderr:     &stdout,
			JobManager: NewJobManager(),
		}

		start := time.Now()
		err = readCommand(cmd)
		duration := time.Since(start)

		// Should timeout after about 0.1 seconds
		if duration < 50*time.Millisecond || duration > 200*time.Millisecond {
			t.Errorf("timeout duration unexpected: %v", duration)
		}

		if err == nil {
			t.Error("expected timeout error, got nil")
		}

		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("expected timeout error, got: %v", err)
		}
	})

	// Test successful read before timeout
	t.Run("read completes before timeout", func(t *testing.T) {
		os.Unsetenv("value")

		parsedCmd, err := parser.Parse("read -t 5 value")
		if err != nil {
			t.Fatalf("Failed to parse command: %v", err)
		}

		var stdout bytes.Buffer
		cmd := &Command{
			Command:    parsedCmd,
			Stdin:      strings.NewReader("quick\n"),
			Stdout:     &stdout,
			Stderr:     &stdout,
			JobManager: NewJobManager(),
		}

		err = readCommand(cmd)
		if err != nil {
			t.Fatalf("read command failed: %v", err)
		}

		if cmd.ReturnCode != 0 {
			t.Errorf("got exit code %d, want 0", cmd.ReturnCode)
		}

		gotValue := os.Getenv("value")
		if gotValue != "quick" {
			t.Errorf("got %q, want %q", gotValue, "quick")
		}
	})
}

func TestReadCommandEOF(t *testing.T) {
	t.Run("EOF returns exit code 1", func(t *testing.T) {
		os.Unsetenv("value")

		parsedCmd, err := parser.Parse("read value")
		if err != nil {
			t.Fatalf("Failed to parse command: %v", err)
		}

		var stdout bytes.Buffer
		cmd := &Command{
			Command:    parsedCmd,
			Stdin:      strings.NewReader(""), // Empty = immediate EOF
			Stdout:     &stdout,
			Stderr:     &stdout,
			JobManager: NewJobManager(),
		}

		err = readCommand(cmd)
		if err != nil {
			t.Fatalf("read command should not error on EOF: %v", err)
		}

		if cmd.ReturnCode != 1 {
			t.Errorf("got exit code %d, want 1 for EOF", cmd.ReturnCode)
		}
	})
}

func TestReadCommandIFSSplitting(t *testing.T) {
	tests := []struct {
		name     string
		ifs      string
		input    string
		wantVars map[string]string
	}{
		{
			name:     "split by colon",
			ifs:      ":",
			input:    "one:two:three\n",
			wantVars: map[string]string{"a": "one", "b": "two", "c": "three"},
		},
		{
			name:     "split by comma",
			ifs:      ",",
			input:    "apple,banana,orange\n",
			wantVars: map[string]string{"a": "apple", "b": "banana", "c": "orange"},
		},
		{
			name:     "default IFS (space)",
			ifs:      "",
			input:    "word1 word2 word3\n",
			wantVars: map[string]string{"a": "word1", "b": "word2", "c": "word3"},
		},
		{
			name:     "multiple delimiters",
			ifs:      ":,",
			input:    "a:b,c:d\n",
			wantVars: map[string]string{"a": "a", "b": "b", "c": "c d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set IFS
			oldIFS := os.Getenv("IFS")
			if tt.ifs != "" {
				os.Setenv("IFS", tt.ifs)
			} else {
				os.Unsetenv("IFS")
			}
			defer func() {
				if oldIFS == "" {
					os.Unsetenv("IFS")
				} else {
					os.Setenv("IFS", oldIFS)
				}
			}()

			// Clear variables
			for varName := range tt.wantVars {
				os.Unsetenv(varName)
			}

			parsedCmd, err := parser.Parse("read a b c")
			if err != nil {
				t.Fatalf("Failed to parse command: %v", err)
			}

			var stdout bytes.Buffer
			cmd := &Command{
				Command:    parsedCmd,
				Stdin:      strings.NewReader(tt.input),
				Stdout:     &stdout,
				Stderr:     &stdout,
				JobManager: NewJobManager(),
			}

			err = readCommand(cmd)
			if err != nil {
				t.Fatalf("read command failed: %v", err)
			}

			// Check variables
			for varName, wantValue := range tt.wantVars {
				gotValue := os.Getenv(varName)
				if gotValue != wantValue {
					t.Errorf("variable %s: got %q, want %q", varName, gotValue, wantValue)
				}
			}
		})
	}
}

func TestReadCommandErrors(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr string
	}{
		{
			name:    "missing -p argument",
			cmd:     "read -p",
			wantErr: "-p requires an argument",
		},
		{
			name:    "missing -n argument",
			cmd:     "read -n",
			wantErr: "-n requires an argument",
		},
		{
			name:    "invalid -n value",
			cmd:     "read -n abc var",
			wantErr: "invalid count",
		},
		{
			name:    "missing -t argument",
			cmd:     "read -t",
			wantErr: "-t requires an argument",
		},
		{
			name:    "invalid -t value",
			cmd:     "read -t xyz var",
			wantErr: "invalid timeout",
		},
		{
			name:    "unknown option",
			cmd:     "read -x var",
			wantErr: "invalid option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedCmd, err := parser.Parse(tt.cmd)
			if err != nil {
				t.Fatalf("Failed to parse command: %v", err)
			}

			var stdout bytes.Buffer
			cmd := &Command{
				Command:    parsedCmd,
				Stdin:      strings.NewReader("test\n"),
				Stdout:     &stdout,
				Stderr:     &stdout,
				JobManager: NewJobManager(),
			}

			err = readCommand(cmd)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
