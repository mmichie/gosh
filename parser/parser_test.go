package parser

import (
	"testing"
)

// Test parsing of valid command inputs.
func TestParseValidInputs(t *testing.T) {
	testCases := []struct {
		input   string
		wantErr bool
	}{
		{"ls -l", false},
		{"echo 'hello world'", false},
		{"cat myfile.txt", false},
		{"rm -rf /", false}, // Be cautious with commands like these in real scenarios.
		{"grep -i 'pattern' file.txt", false},
	}

	for _, tc := range testCases {
		_, err := Parse(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("Parse(%q) returned error: %v, wantErr %t", tc.input, err, tc.wantErr)
		}
	}
}

// Test parsing of invalid command inputs.
func TestParseInvalidInputs(t *testing.T) {
	testCases := []struct {
		input   string
		wantErr bool
	}{
		{"ls |", true},
		{"echo >", true},
		{"cat", true},
		{"| grep", true},
	}

	for _, tc := range testCases {
		_, err := Parse(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("Parse(%q) returned error: %v, wantErr %t", tc.input, err, tc.wantErr)
		}
	}
}
