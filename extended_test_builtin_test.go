package gosh

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gosh/parser"
)

// runExtendedTestCommand executes the [[ command and returns the exit code
func runExtendedTestCommand(t *testing.T, args []string) int {
	// Build the command string
	cmdString := ""
	for i, arg := range args {
		if i > 0 {
			cmdString += " "
		}
		// Quote arguments that need it
		needsQuoting := false
		if arg == "" {
			needsQuoting = true
		}

		if needsQuoting {
			cmdString += "'" + arg + "'"
		} else {
			cmdString += arg
		}
	}

	// Preprocess extended test to handle shell operators
	preprocessed := PreprocessExtendedTest(cmdString)

	// Parse the command
	parsedCmd, err := parser.Parse(preprocessed)
	if err != nil {
		t.Fatalf("Failed to parse command %q: %v", cmdString, err)
	}

	// Create the command
	var stdout, stderr bytes.Buffer
	cmd := &Command{
		Command:    parsedCmd,
		Stdin:      os.Stdin,
		Stdout:     &stdout,
		Stderr:     &stderr,
		JobManager: NewJobManager(),
	}

	// Execute the command
	extendedTestCommand(cmd)

	return cmd.ReturnCode
}

func TestExtendedTestBasic(t *testing.T) {
	// Create temporary directory and files for tests
	tmpDir, err := os.MkdirTemp("", "gosh-extended-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	dir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	nonExistent := filepath.Join(tmpDir, "does-not-exist")

	tests := []struct {
		name       string
		args       []string
		wantResult int // 0 = true, 1 = false, 2 = error
	}{
		// Empty test
		{"empty test", []string{"[[", "]]"}, 1},

		// Single argument (non-empty string is true)
		{"single arg non-empty", []string{"[[", "hello", "]]"}, 0},
		{"single arg empty-looking", []string{"[[", "-n", "]]"}, 0}, // "-n" is non-empty

		// File existence tests
		{"file exists", []string{"[[", "-e", regularFile, "]]"}, 0},
		{"file not exists", []string{"[[", "-e", nonExistent, "]]"}, 1},

		// Regular file tests
		{"is regular file", []string{"[[", "-f", regularFile, "]]"}, 0},
		{"is not regular file (dir)", []string{"[[", "-f", dir, "]]"}, 1},

		// Directory tests
		{"is directory", []string{"[[", "-d", dir, "]]"}, 0},
		{"is not directory (file)", []string{"[[", "-d", regularFile, "]]"}, 1},

		// String empty/non-empty tests
		{"string is not empty (-z)", []string{"[[", "-z", "hello", "]]"}, 1},
		{"string is empty (-z)", []string{"[[", "-z", "", "]]"}, 0},
		{"string is not empty (-n)", []string{"[[", "-n", "hello", "]]"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestStringComparison(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// String equality
		{"strings equal (==)", []string{"[[", "hello", "==", "hello", "]]"}, 0},
		{"strings not equal (==)", []string{"[[", "hello", "==", "world", "]]"}, 1},
		{"strings equal (=)", []string{"[[", "hello", "=", "hello", "]]"}, 0},
		{"strings not equal (!=)", []string{"[[", "hello", "!=", "world", "]]"}, 0},
		{"strings equal (!=)", []string{"[[", "hello", "!=", "hello", "]]"}, 1},

		// Lexicographic comparison
		{"string less than", []string{"[[", "abc", "<", "def", "]]"}, 0},
		{"string not less than", []string{"[[", "def", "<", "abc", "]]"}, 1},
		{"string greater than", []string{"[[", "def", ">", "abc", "]]"}, 0},
		{"string not greater than", []string{"[[", "abc", ">", "def", "]]"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestNumericComparison(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		{"numbers equal", []string{"[[", "42", "-eq", "42", "]]"}, 0},
		{"numbers not equal", []string{"[[", "42", "-eq", "43", "]]"}, 1},
		{"numbers not equal (ne)", []string{"[[", "42", "-ne", "43", "]]"}, 0},
		{"less than", []string{"[[", "5", "-lt", "10", "]]"}, 0},
		{"not less than", []string{"[[", "10", "-lt", "5", "]]"}, 1},
		{"less or equal", []string{"[[", "5", "-le", "5", "]]"}, 0},
		{"greater than", []string{"[[", "10", "-gt", "5", "]]"}, 0},
		{"greater or equal", []string{"[[", "5", "-ge", "5", "]]"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestPatternMatching(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Glob pattern matching with ==
		{"match star pattern", []string{"[[", "hello", "==", "hel*", "]]"}, 0},
		{"match question pattern", []string{"[[", "hello", "==", "hell?", "]]"}, 0},
		{"no match star pattern", []string{"[[", "hello", "==", "world*", "]]"}, 1},
		{"match star at end", []string{"[[", "filename.txt", "==", "*.txt", "]]"}, 0},
		{"match star at start", []string{"[[", "filename.txt", "==", "filename.*", "]]"}, 0},
		{"match multiple stars", []string{"[[", "path/to/file.txt", "==", "*/to/*", "]]"}, 0},

		// Character class matching
		{"match char class", []string{"[[", "abc", "==", "[a-z]*", "]]"}, 0},
		{"no match char class", []string{"[[", "123", "==", "[a-z]*", "]]"}, 1},

		// Literal matching
		{"exact match", []string{"[[", "hello", "==", "hello", "]]"}, 0},
		{"no exact match", []string{"[[", "hello", "==", "world", "]]"}, 1},

		// Negated pattern matching
		{"negated pattern match", []string{"[[", "hello", "!=", "world*", "]]"}, 0},
		{"negated pattern no match", []string{"[[", "hello", "!=", "hel*", "]]"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestRegexMatching(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Regex matching with =~
		{"regex simple match", []string{"[[", "hello123", "=~", "hello[0-9]+", "]]"}, 0},
		{"regex no match", []string{"[[", "hello", "=~", "world", "]]"}, 1},
		{"regex anchor start", []string{"[[", "hello", "=~", "^hello", "]]"}, 0},
		{"regex anchor end", []string{"[[", "hello", "=~", "hello$", "]]"}, 0},
		{"regex anchor both", []string{"[[", "hello", "=~", "^hello$", "]]"}, 0},
		{"regex dot wildcard", []string{"[[", "hello", "=~", "h.llo", "]]"}, 0},
		{"regex plus quantifier", []string{"[[", "helllo", "=~", "hel+o", "]]"}, 0},
		{"regex question quantifier", []string{"[[", "helo", "=~", "hel?o", "]]"}, 0},
		// Note: regex groups with () require special handling as () are shell operators
		// Use patterns without bare () in the shell, or assign to a variable first
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestLogicalOperators(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gosh-extended-logic-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	nonExistent := filepath.Join(tmpDir, "does-not-exist")

	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Logical AND (&&)
		{"and both true", []string{"[[", "-f", regularFile, "&&", "-r", regularFile, "]]"}, 0},
		{"and first false", []string{"[[", "-f", nonExistent, "&&", "-r", regularFile, "]]"}, 1},
		{"and second false", []string{"[[", "-f", regularFile, "&&", "-r", nonExistent, "]]"}, 1},
		{"and both false", []string{"[[", "-f", nonExistent, "&&", "-r", nonExistent, "]]"}, 1},

		// Logical OR (||)
		{"or both true", []string{"[[", "-f", regularFile, "||", "-r", regularFile, "]]"}, 0},
		{"or first true", []string{"[[", "-f", regularFile, "||", "-r", nonExistent, "]]"}, 0},
		{"or second true", []string{"[[", "-f", nonExistent, "||", "-r", regularFile, "]]"}, 0},
		{"or both false", []string{"[[", "-f", nonExistent, "||", "-r", nonExistent, "]]"}, 1},

		// Negation (!)
		{"negation of true", []string{"[[", "!", "-f", regularFile, "]]"}, 1},
		{"negation of false", []string{"[[", "!", "-f", nonExistent, "]]"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestParentheses(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gosh-extended-paren-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	nonExistent := filepath.Join(tmpDir, "does-not-exist")

	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Grouping with parentheses
		{"simple grouping", []string{"[[", "(", "-f", regularFile, ")", "]]"}, 0},
		{"grouped and", []string{"[[", "(", "-f", regularFile, "&&", "-r", regularFile, ")", "]]"}, 0},
		{"grouped or", []string{"[[", "(", "-f", nonExistent, "||", "-f", regularFile, ")", "]]"}, 0},

		// Complex: (false || true) && true = true
		{"complex expr 1", []string{"[[", "(", "-f", nonExistent, "||", "-f", regularFile, ")", "&&", "-r", regularFile, "]]"}, 0},
		// Complex: (true && false) || true = true
		{"complex expr 2", []string{"[[", "(", "-f", regularFile, "&&", "-f", nonExistent, ")", "||", "-r", regularFile, "]]"}, 0},
		// Complex: true && (false || true) = true
		{"complex expr 3", []string{"[[", "-f", regularFile, "&&", "(", "-f", nonExistent, "||", "-r", regularFile, ")", "]]"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestFileComparisons(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gosh-extended-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create two files with different timestamps
	olderFile := filepath.Join(tmpDir, "older.txt")
	if err := os.WriteFile(olderFile, []byte("older"), 0644); err != nil {
		t.Fatalf("Failed to create older file: %v", err)
	}

	newerFile := filepath.Join(tmpDir, "newer.txt")
	if err := os.WriteFile(newerFile, []byte("newer"), 0644); err != nil {
		t.Fatalf("Failed to create newer file: %v", err)
	}

	// Create a symlink to test -ef
	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(olderFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// File time comparison (-nt, -ot)
		{"newer than", []string{"[[", newerFile, "-nt", olderFile, "]]"}, 0},
		{"older than", []string{"[[", olderFile, "-ot", newerFile, "]]"}, 0},

		// Same file (-ef)
		{"same file via symlink", []string{"[[", olderFile, "-ef", symlink, "]]"}, 0},
		{"different files", []string{"[[", olderFile, "-ef", newerFile, "]]"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestExtendedTestSyntaxErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult int
	}{
		// Missing closing ]]
		{"missing closing bracket", []string{"[[", "-f", "/tmp/test"}, 2},

		// Too many arguments without operators
		{"too many args", []string{"[[", "a", "b", "c", "d", "e", "]]"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runExtendedTestCommand(t, tt.args)
			if result != tt.wantResult {
				t.Errorf("test %v: got exit code %d, want %d", tt.args, result, tt.wantResult)
			}
		})
	}
}

func TestPreprocessExtendedTest(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Simple cases - no preprocessing needed
		{"echo hello", "echo hello"},
		{"[[ -f file ]]", "[[ -f file ]]"},
		{"[[ hello == world ]]", "[[ hello == world ]]"},

		// Logical operators should be quoted
		{"[[ a && b ]]", "[[ a '&&' b ]]"},
		{"[[ a || b ]]", "[[ a '||' b ]]"},

		// Comparison operators should be quoted
		{"[[ a < b ]]", "[[ a '<' b ]]"},
		{"[[ a > b ]]", "[[ a '>' b ]]"},

		// Parentheses should be quoted
		{"[[ ( a ) ]]", "[[ '(' a ')' ]]"},

		// Combined expressions
		{"[[ a && ( b || c ) ]]", "[[ a '&&' '(' b '||' c ')' ]]"},

		// Outside [[ ]] should not be affected
		{"echo && echo", "echo && echo"},
		{"(echo hello)", "(echo hello)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := PreprocessExtendedTest(tt.input)
			if result != tt.expected {
				t.Errorf("PreprocessExtendedTest(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob     string
		expected string
	}{
		{"*", ".*"},
		{"?", "."},
		{"*.txt", ".*\\.txt"},
		{"file?.txt", "file.\\.txt"},
		{"[abc]", "[abc]"},
		{"[a-z]", "[a-z]"},
		{"[!abc]", "[^abc]"},
		{"hello.world", "hello\\.world"},
		{"path/to/file", "path/to/file"},
		{"file\\*name", "file\\*name"},
	}

	for _, tt := range tests {
		t.Run(tt.glob, func(t *testing.T) {
			result := globToRegex(tt.glob)
			if result != tt.expected {
				t.Errorf("globToRegex(%q) = %q, want %q", tt.glob, result, tt.expected)
			}
		})
	}
}
