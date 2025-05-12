package parser

import (
	"testing"
)

func TestParseSemicolon(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		expectedBlocks   int
		shouldParseError bool
	}{
		{
			name:           "Simple command",
			input:          "echo hello",
			expectedBlocks: 1,
		},
		{
			name:           "Two commands with semicolon",
			input:          "echo hello; echo world",
			expectedBlocks: 2,
		},
		{
			name:           "Three commands with semicolons",
			input:          "echo one; echo two; echo three",
			expectedBlocks: 3,
		},
		{
			name:           "Semicolon with logical operators",
			input:          "true && echo success; false || echo failed",
			expectedBlocks: 2,
		},
		{
			name:           "Complex command with semicolons",
			input:          "echo start; grep pattern file | sort; echo done",
			expectedBlocks: 3,
		},
		{
			name:           "Trailing semicolon",
			input:          "echo hello;",
			expectedBlocks: 1, // Should ignore trailing semicolon
		},
		{
			name:             "Only semicolon",
			input:            ";",
			shouldParseError: true, // Should fail to parse
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Parse(tc.input)

			if tc.shouldParseError {
				if err == nil {
					t.Fatalf("Expected parse error for input %q, but got none", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tc.input, err)
			}

			if len(result.LogicalBlocks) != tc.expectedBlocks {
				t.Errorf("Expected %d logical blocks, got %d for input %q",
					tc.expectedBlocks, len(result.LogicalBlocks), tc.input)
			}

			// Format the command back to string and re-parse to test consistency
			formatted := FormatCommand(result)
			t.Logf("Formatted command: %s", formatted)

			reparsed, err := Parse(formatted)
			if err != nil {
				t.Fatalf("Reparsing formatted command returned error: %v", err)
			}

			if len(reparsed.LogicalBlocks) != tc.expectedBlocks {
				t.Errorf("After formatting, expected %d logical blocks, got %d",
					tc.expectedBlocks, len(reparsed.LogicalBlocks))
			}
		})
	}
}

func TestSemicolonStructure(t *testing.T) {
	// Test a compound command with semicolons to ensure structure is preserved
	input := "echo hello; echo world"

	result, err := Parse(input)
	if err != nil {
		t.Fatalf("Failed to parse input %q: %v", input, err)
	}

	if len(result.LogicalBlocks) != 2 {
		t.Fatalf("Expected 2 logical blocks, got %d", len(result.LogicalBlocks))
	}

	// Verify first command is "echo hello"
	firstCmd := result.LogicalBlocks[0].FirstPipeline.Commands[0]
	if len(firstCmd.Parts) != 2 || firstCmd.Parts[0] != "echo" || firstCmd.Parts[1] != "hello" {
		t.Errorf("First command doesn't match expected structure: %v", firstCmd.Parts)
	}

	// Verify second command is "echo world"
	secondCmd := result.LogicalBlocks[1].FirstPipeline.Commands[0]
	if len(secondCmd.Parts) != 2 || secondCmd.Parts[0] != "echo" || secondCmd.Parts[1] != "world" {
		t.Errorf("Second command doesn't match expected structure: %v", secondCmd.Parts)
	}
}
