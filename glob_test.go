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
		// 		{
		// 			name:     "Negated character class",
		// 			args:     []string{"file[!3].txt"},
		// 			expected: []string{"file1.txt", "file2.txt"},
		// 		},
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

func TestRecursiveGlobbing(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gosh-recursive-glob-test")
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

	// Create a more complex directory structure for recursive glob testing
	createRecursiveTestFiles(t, tempDir)

	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "Recursive glob all .txt files",
			args:     []string{"**/*.txt"},
			expected: []string{"file1.txt", "file2.txt", "src/main.txt", "src/lib/util.txt", "src/lib/deep/nested.txt"},
		},
		{
			name:     "Recursive glob all .go files",
			args:     []string{"**/*.go"},
			expected: []string{"main.go", "src/main.go", "src/lib/lib.go", "src/lib/deep/deep.go"},
		},
		{
			name:     "Recursive glob under specific directory",
			args:     []string{"src/**/*.go"},
			expected: []string{"src/main.go", "src/lib/lib.go", "src/lib/deep/deep.go"},
		},
		{
			name:     "Recursive glob under nested directory",
			args:     []string{"src/lib/**/*.go"},
			expected: []string{"src/lib/lib.go", "src/lib/deep/deep.go"},
		},
		{
			name:     "Recursive glob matching specific filename",
			args:     []string{"**/main.go"},
			expected: []string{"main.go", "src/main.go"},
		},
		{
			name:     "Recursive glob all files (no suffix)",
			args:     []string{"src/lib/**"},
			expected: []string{"src/lib/lib.go", "src/lib/util.txt", "src/lib/deep/deep.go", "src/lib/deep/nested.txt"},
		},
		{
			name:     "Recursive glob with question mark",
			args:     []string{"**/*.g?"},
			expected: []string{"main.go", "src/main.go", "src/lib/lib.go", "src/lib/deep/deep.go"},
		},
		{
			name:     "Recursive glob no matches",
			args:     []string{"**/*.xyz"},
			expected: []string{"**/*.xyz"},
		},
		{
			name:     "Recursive glob with directory wildcard prefix",
			args:     []string{"src/*/**/*.go"},
			expected: []string{"src/lib/lib.go", "src/lib/deep/deep.go"},
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

// createRecursiveTestFiles creates a deeper directory structure for recursive glob testing
func createRecursiveTestFiles(t *testing.T, baseDir string) {
	// Create directory structure:
	// .
	// ├── main.go
	// ├── file1.txt
	// ├── file2.txt
	// ├── src/
	// │   ├── main.go
	// │   ├── main.txt
	// │   └── lib/
	// │       ├── lib.go
	// │       ├── util.txt
	// │       └── deep/
	// │           ├── deep.go
	// │           └── nested.txt
	// └── .hidden/
	//     └── secret.txt

	files := map[string]bool{
		"main.go":                 false,
		"file1.txt":               false,
		"file2.txt":               false,
		"src/main.go":             false,
		"src/main.txt":            false,
		"src/lib/lib.go":          false,
		"src/lib/util.txt":        false,
		"src/lib/deep/deep.go":    false,
		"src/lib/deep/nested.txt": false,
		".hidden/secret.txt":      false, // Hidden directory should be skipped
	}

	for filePath := range files {
		fullPath := filepath.Join(baseDir, filePath)
		dir := filepath.Dir(fullPath)

		// Create directory if needed
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Create file
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
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
