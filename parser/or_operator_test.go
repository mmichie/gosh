package parser

import (
	"testing"
)

func TestParseOrOperator(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Simple OR operator",
			input: "false || echo 'Command failed'",
		},
		{
			name:  "OR with multiple commands",
			input: "false || false || echo 'All commands failed'",
		},
		{
			name:  "Mixed AND and OR",
			input: "true && echo 'First succeeded' || echo 'First failed'",
		},
		{
			name:  "Complex conditional execution",
			input: "true && echo 'First true' || echo 'First false'; false && echo 'Second true' || echo 'Second false'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tc.input, err)
			}

			// Verify we can parse the command successfully
			if result == nil {
				t.Fatalf("Parse(%q) returned nil result", tc.input)
			}

			// Format the command back to a string and reparse to verify consistency
			formatted := FormatCommand(result)
			reparsed, err := Parse(formatted)
			if err != nil {
				t.Fatalf("Parse(FormatCommand(result)) returned unexpected error: %v", err)
			}

			// Only check that the structure is valid by checking if we have logical blocks
			if len(reparsed.LogicalBlocks) == 0 {
				t.Errorf("Reparsed command has no logical blocks")
			}
		})
	}
}

func TestOrOperatorStructure(t *testing.T) {
	// Test a simple OR operator to ensure the structure is correct
	input := "false || echo 'failed'"
	expected := &Command{
		LogicalBlocks: []*LogicalBlock{
			{
				FirstPipeline: &Pipeline{
					Commands: []*SimpleCommand{
						{Parts: []string{"false"}},
					},
				},
				RestPipelines: []*OpPipeline{
					{
						Operator: "||",
						Pipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"echo", "'failed'"}},
							},
						},
					},
				},
			},
		},
	}

	result, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q) returned unexpected error: %v", input, err)
	}

	// Compare the structure of the parsed command
	if len(result.LogicalBlocks) != len(expected.LogicalBlocks) {
		t.Fatalf("Result has %d LogicalBlocks, expected %d",
			len(result.LogicalBlocks), len(expected.LogicalBlocks))
	}

	block := result.LogicalBlocks[0]

	// Check first pipeline has the correct "false" command
	if len(block.FirstPipeline.Commands) != 1 ||
		len(block.FirstPipeline.Commands[0].Parts) != 1 ||
		block.FirstPipeline.Commands[0].Parts[0] != "false" {
		t.Errorf("FirstPipeline doesn't match expected structure")
	}

	// Check that we have one RestPipeline with "||" operator
	if len(block.RestPipelines) != 1 ||
		block.RestPipelines[0].Operator != "||" {
		t.Errorf("RestPipelines doesn't contain the expected OR operator")
	}

	// Check that the second pipeline has the echo command
	echoPipeline := block.RestPipelines[0].Pipeline
	if len(echoPipeline.Commands) != 1 ||
		len(echoPipeline.Commands[0].Parts) != 2 ||
		echoPipeline.Commands[0].Parts[0] != "echo" {
		t.Errorf("Echo command in OR operator pipeline doesn't match expected structure")
	}
}
