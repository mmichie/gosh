package gosh

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestExpandWildcards(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-glob-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save the current directory to restore it later
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to the temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create test files and directories
	createTestFiles(t, tempDir)

	// Run test cases
	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "No wildcards",
			args:     []string{"file1.txt", "file2.txt"},
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "Simple star wildcard",
			args:     []string{"*.txt"},
			expected: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:     "Question mark wildcard",
			args:     []string{"file?.txt"},
			expected: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name:     "Character class wildcard",
			args:     []string{"file[1-2].txt"},
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "Negated character class",
			args:     []string{"file[!3].txt"},
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "Brace expansion",
			args:     []string{"file{1,3}.txt"},
			expected: []string{"file1.txt", "file3.txt"},
		},
		{
			name:     "Mixed wildcards",
			args:     []string{"*.txt", "dir1/*"},
			expected: []string{"file1.txt", "file2.txt", "file3.txt", "dir1/nested1.txt", "dir1/nested2.txt"},
		},
		{
			name:     "No matches",
			args:     []string{"nonexistent*.txt"},
			expected: []string{"nonexistent*.txt"},
		},
		{
			name:     "Directory with wildcards",
			args:     []string{"dir*/nested*.txt"},
			expected: []string{"dir1/nested1.txt", "dir1/nested2.txt", "dir2/nested3.txt"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExpandWildcards(tc.args)

			// Sort the results for reliable comparison
			sort.Strings(result)
			sort.Strings(tc.expected)

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// createTestFiles creates test files and directories for glob testing
func createTestFiles(t *testing.T, baseDir string) {
	// Create regular files
	files := []string{
		"file1.txt",
		"file2.txt",
		"file3.txt",
		"other.log",
	}

	for _, f := range files {
		path := filepath.Join(baseDir, f)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f, err)
		}
	}

	// Create directories with nested files
	dirs := map[string][]string{
		"dir1": {"nested1.txt", "nested2.txt"},
		"dir2": {"nested3.txt"},
	}

	for dir, nestedFiles := range dirs {
		dirPath := filepath.Join(baseDir, dir)
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		for _, f := range nestedFiles {
			path := filepath.Join(dirPath, f)
			if err := os.WriteFile(path, []byte("nested content"), 0644); err != nil {
				t.Fatalf("Failed to create nested file %s: %v", path, err)
			}
		}
	}
}
