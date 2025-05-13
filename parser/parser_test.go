package parser

import (
	"reflect"
	"testing"
)

func TestParseValidInputs(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "Simple command",
			input: "ls -l",
			expected: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"ls", "-l"}},
							},
						},
					},
				},
			},
		},
		{
			name:  "Pipeline",
			input: "cat file.txt | grep pattern",
			expected: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"cat", "file.txt"}},
								{Parts: []string{"grep", "pattern"}},
							},
						},
					},
				},
			},
		},
		{
			name:  "AND command",
			input: "mkdir test && cd test",
			expected: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"mkdir", "test"}},
							},
						},
						RestPipelines: []*OpPipeline{
							{
								Operator: "&&",
								Pipeline: &Pipeline{
									Commands: []*SimpleCommand{
										{Parts: []string{"cd", "test"}},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "Command with redirections",
			input: "echo 'Hello' > output.txt",
			expected: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{
									Parts: []string{"echo", "'Hello'"},
									Redirects: []*Redirect{
										{Type: ">", File: "output.txt"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tc.input, err)
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Parse(%q) = %+v, want %+v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParseInvalidInputs(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"Empty input", ""},
		{"Whitespace only", "   "},
		{"Incomplete pipeline", "ls |"},
		{"Incomplete AND", "ls &&"},
		{"Invalid redirection", "cat file.txt >"},
		{"Unmatched quote", "echo 'hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.input)
			if err == nil {
				t.Errorf("Parse(%q) did not return an error, want error", tc.input)
			}
		})
	}
}

func TestProcessCommand(t *testing.T) {
	testCases := []struct {
		name                string
		input               *SimpleCommand
		expectedCommand     string
		expectedArgs        []string
		expectedInputRedir  string
		expectedInputFile   string
		expectedOutputRedir string
		expectedOutputFile  string
	}{
		{
			name: "Simple command",
			input: &SimpleCommand{
				Parts: []string{"ls", "-l"},
			},
			expectedCommand:     "ls",
			expectedArgs:        []string{"-l"},
			expectedInputRedir:  "",
			expectedInputFile:   "",
			expectedOutputRedir: "",
			expectedOutputFile:  "",
		},
		{
			name: "Command with input redirection",
			input: &SimpleCommand{
				Parts: []string{"cat"},
				Redirects: []*Redirect{
					{Type: "<", File: "input.txt"},
				},
			},
			expectedCommand:     "cat",
			expectedArgs:        []string{},
			expectedInputRedir:  "<",
			expectedInputFile:   "input.txt",
			expectedOutputRedir: "",
			expectedOutputFile:  "",
		},
		{
			name: "Command with output redirection",
			input: &SimpleCommand{
				Parts: []string{"echo", "hello"},
				Redirects: []*Redirect{
					{Type: ">", File: "output.txt"},
				},
			},
			expectedCommand:     "echo",
			expectedArgs:        []string{"hello"},
			expectedInputRedir:  "",
			expectedInputFile:   "",
			expectedOutputRedir: ">",
			expectedOutputFile:  "output.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command, args, inputRedir, inputFile, outputRedir, outputFile, _, _, _ := ProcessCommand(tc.input)
			if command != tc.expectedCommand {
				t.Errorf("ProcessCommand() command = %v, want %v", command, tc.expectedCommand)
			}
			if !reflect.DeepEqual(args, tc.expectedArgs) {
				t.Errorf("ProcessCommand() args = %#v, want %#v", args, tc.expectedArgs)
				t.Logf("Input: %#v", tc.input)
				t.Logf("Input Parts: %#v", tc.input.Parts)
				t.Logf("Input Redirects: %#v", tc.input.Redirects)
			}
			if inputRedir != tc.expectedInputRedir {
				t.Errorf("ProcessCommand() inputRedir = %v, want %v", inputRedir, tc.expectedInputRedir)
			}
			if inputFile != tc.expectedInputFile {
				t.Errorf("ProcessCommand() inputFile = %v, want %v", inputFile, tc.expectedInputFile)
			}
			if outputRedir != tc.expectedOutputRedir {
				t.Errorf("ProcessCommand() outputRedir = %v, want %v", outputRedir, tc.expectedOutputRedir)
			}
			if outputFile != tc.expectedOutputFile {
				t.Errorf("ProcessCommand() outputFile = %v, want %v", outputFile, tc.expectedOutputFile)
			}
		})
	}
}

func TestFormatCommandNew(t *testing.T) {
	testCases := []struct {
		name     string
		input    *Command
		expected string
	}{
		{
			name: "Simple command",
			input: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"ls", "-l"}},
							},
						},
					},
				},
			},
			expected: "ls -l",
		},
		{
			name: "Pipeline",
			input: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"cat", "file.txt"}},
								{Parts: []string{"grep", "pattern"}},
							},
						},
					},
				},
			},
			expected: "cat file.txt | grep pattern",
		},
		{
			name: "AND command",
			input: &Command{
				LogicalBlocks: []*LogicalBlock{
					{
						FirstPipeline: &Pipeline{
							Commands: []*SimpleCommand{
								{Parts: []string{"mkdir", "test"}},
							},
						},
						RestPipelines: []*OpPipeline{
							{
								Operator: "&&",
								Pipeline: &Pipeline{
									Commands: []*SimpleCommand{
										{Parts: []string{"cd", "test"}},
									},
								},
							},
						},
					},
				},
			},
			expected: "mkdir test && cd test",
		},
		{
			name: "OR command",
			input: &Command{
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
										{Parts: []string{"echo", "failed"}},
									},
								},
							},
						},
					},
				},
			},
			expected: "false || echo failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatCommand(tc.input)
			if result != tc.expected {
				t.Errorf("FormatCommand() = %v, want %v", result, tc.expected)
			}
		})
	}
}
