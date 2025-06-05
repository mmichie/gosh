package gosh

import (
	"bytes"
	"strings"
	"testing"
)

func TestHereDocIntegration(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		expectedOutput string
	}{
		{
			name: "Simple here-doc",
			command: `cat << EOF
Hello, world!
This is a here-doc.
EOF`,
			expectedOutput: "Hello, world!\nThis is a here-doc.\n",
		},
		{
			name: "Here-doc with tab stripping",
			command: `cat <<- EOF
	Hello, world!
	This is a here-doc with tabs.
EOF`,
			expectedOutput: "Hello, world!\nThis is a here-doc with tabs.\n",
		},
		{
			name:           "Here-string",
			command:        `cat <<< "Hello, world!"`,
			expectedOutput: "Hello, world!",
		},
		{
			name: "Here-doc with quoted delimiter",
			command: `cat << "END"
This is a here-doc with a quoted delimiter.
END`,
			expectedOutput: "This is a here-doc with a quoted delimiter.\n",
		},
		{
			name: "Here-doc in a pipeline",
			command: `cat << EOF | grep Hello
Hello, world!
This is a here-doc.
EOF`,
			expectedOutput: "Hello, world!",
		},
		{
			name: "Multiple here-docs",
			command: `cat << EOF1 && cat << EOF2
Content 1
EOF1
Content 2
EOF2`,
			expectedOutput: "Content 1\nContent 2\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a job manager for the shell
			jobManager := NewJobManager()

			// Create a command with the test input
			cmd, err := NewCommand(tc.command, jobManager)
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}

			// Set up a buffer to capture output
			var outputBuffer bytes.Buffer
			cmd.Stdout = &outputBuffer
			cmd.Stderr = &outputBuffer

			// Run the command
			cmd.Run()

			// Get the captured output
			output := outputBuffer.String()

			// Clean up whitespace for more reliable comparison
			output = strings.TrimSpace(output)
			expected := strings.TrimSpace(tc.expectedOutput)

			// Check the output
			if output != expected {
				t.Errorf("Expected output: %q, got: %q", expected, output)
			}
		})
	}
}
