package gosh

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestPreprocessHereDoc(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectedCount  int
	}{
		{
			name: "Simple here-doc",
			input: `cat << EOF
Hello, world!
This is a here-doc.
EOF`,
			expectedOutput: `cat < heredoc_EOF_guid_`,
			expectedCount:  1,
		},
		{
			name: "Here-doc with tab stripping",
			input: `cat <<- EOF
	Hello, world!
	This is a here-doc with tabs.
EOF`,
			expectedOutput: `cat < heredoc_EOF_guid_`,
			expectedCount:  1,
		},
		{
			name:           "Here-string",
			input:          `cat <<< "Hello, world!"`,
			expectedOutput: `cat < herestring_guid_`,
			expectedCount:  1,
		},
		{
			name: "Multiple here-docs",
			input: `cat << EOF1
Content 1
EOF1
cat << EOF2
Content 2
EOF2`,
			expectedOutput: `cat < heredoc_EOF1_guid_
cat < heredoc_EOF2_guid_`,
			expectedCount: 2,
		},
		{
			name: "Here-doc within a pipeline",
			input: `cat << EOF | grep "Hello"
Hello, world!
EOF`,
			expectedOutput: `cat < heredoc_EOF_guid_ | grep "Hello"`,
			expectedCount:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, hereDocs, err := PreprocessHereDoc(tc.input)
			if err != nil {
				t.Fatalf("PreprocessHereDoc error: %v", err)
			}

			// Check that the output contains the expected pattern
			if !strings.HasPrefix(output, tc.expectedOutput) {
				t.Errorf("Expected output to start with %q, got %q", tc.expectedOutput, output)
			}

			// Check that we have the expected number of here-docs
			if len(hereDocs) != tc.expectedCount {
				t.Errorf("Expected %d here-docs, got %d", tc.expectedCount, len(hereDocs))
			}
		})
	}
}

func TestProcessHereDoc(t *testing.T) {
	tests := []struct {
		name          string
		hereDoc       *HereDoc
		expectedLines []string
	}{
		{
			name: "Standard here-doc",
			hereDoc: &HereDoc{
				Delimiter:   "EOF",
				Content:     "Line 1\nLine 2\nLine 3\n",
				StripTabs:   false,
				ContentType: "heredoc",
			},
			expectedLines: []string{"Line 1", "Line 2", "Line 3", ""},
		},
		{
			name: "Tab-stripped here-doc",
			hereDoc: &HereDoc{
				Delimiter:   "EOF",
				Content:     "\tLine 1\n\t\tLine 2\n\tLine 3\n",
				StripTabs:   true,
				ContentType: "heredoc",
			},
			expectedLines: []string{"Line 1", "\tLine 2", "Line 3", ""},
		},
		{
			name: "Here-string",
			hereDoc: &HereDoc{
				Content:     "This is a here-string",
				ContentType: "herestring",
			},
			expectedLines: []string{"This is a here-string"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := ProcessHereDoc(tc.hereDoc)

			// Read the processed content
			content, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to read processed content: %v", err)
			}

			// Split into lines
			lines := strings.Split(string(content), "\n")

			// Check that the processed content matches the expected lines
			if !reflect.DeepEqual(lines, tc.expectedLines) {
				t.Errorf("Expected lines %v, got %v", tc.expectedLines, lines)
			}
		})
	}
}
