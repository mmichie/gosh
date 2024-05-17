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
		{"rm -rf /", false},
		{"grep -i 'pattern' file.txt", false},
		{"cd /path/to/directory", false},
		{"pwd", false},
		{"mkdir new_directory", false},
		{"touch new_file.txt", false},
		{"cp file1.txt file2.txt", false},
		{"mv old_name.txt new_name.txt", false},
		{"find . -name '*.txt'", false},
		{"sed 's/old/new/g' file.txt", false},
		{"awk '{print $1}' file.txt", false},
		{"sort file.txt", false},
		{"uniq -c file.txt", false},
		{"head -n 10 file.txt", false},
		{"tail -f /var/log/syslog", false},
		{"tar -czvf archive.tar.gz directory/", false},
		{"gzip file.txt", false},
		{"ping google.com", false},
		{"nslookup example.com", false},
		{"ssh user@remote.host", false},
		{"scp file.txt user@remote.host:/path/", false},
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
		{"cp file1.txt", true},
		{"mv old_name.txt", true},
		{"find .", true},
		{"sed 's/old/new/g'", true},
		{"awk '{print $1}'", true},
		{"head -n", true},
		{"tail -f", true},
		{"tar -czvf", true},
		{"gzip", true},
		{"ssh", true},
		{"scp file.txt", true},
	}

	for _, tc := range testCases {
		_, err := Parse(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("Parse(%q) returned error: %v, wantErr %t", tc.input, err, tc.wantErr)
		}
	}
}
