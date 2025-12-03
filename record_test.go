package gosh

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRecordBasics tests basic record operations.
func TestRecordBasics(t *testing.T) {
	record := Record{
		"name":  "test",
		"value": 42,
	}

	// Test field access
	if record["name"] != "test" {
		t.Errorf("Expected name='test', got %v", record["name"])
	}

	if record["value"] != 42 {
		t.Errorf("Expected value=42, got %v", record["value"])
	}
}

// TestRecordMagic tests the record magic header.
func TestRecordMagic(t *testing.T) {
	// Verify magic is the expected format
	if !strings.HasPrefix(RecordMagic, "\x1e\x1f") {
		t.Error("RecordMagic should start with ASCII control characters")
	}

	if !strings.HasSuffix(RecordMagic, "\n") {
		t.Error("RecordMagic should end with newline")
	}
}

// TestRecordReader tests reading records from JSON lines.
func TestRecordReader(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
`
	reader := NewRecordReader(strings.NewReader(input))

	// Read first record
	rec1, err := reader.Read()
	if err != nil {
		t.Fatalf("Failed to read first record: %v", err)
	}
	if rec1["name"] != "alice" {
		t.Errorf("Expected name='alice', got %v", rec1["name"])
	}

	// Read second record
	rec2, err := reader.Read()
	if err != nil {
		t.Fatalf("Failed to read second record: %v", err)
	}
	if rec2["name"] != "bob" {
		t.Errorf("Expected name='bob', got %v", rec2["name"])
	}

	// Read past end
	_, err = reader.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

// TestRecordWriter tests writing records in different formats.
func TestRecordWriter(t *testing.T) {
	records := []Record{
		{"name": "alice", "age": 30},
		{"name": "bob", "age": 25},
	}

	// Test JSON format
	t.Run("JSON", func(t *testing.T) {
		var buf bytes.Buffer
		err := WriteAllRecords(&buf, records, "json")
		if err != nil {
			t.Fatalf("WriteAllRecords failed: %v", err)
		}

		output := buf.String()
		if !strings.HasPrefix(output, RecordMagic) {
			t.Error("Output should start with RecordMagic")
		}

		// Read back and verify
		readRecords, err := ReadAllRecords(strings.NewReader(output))
		if err != nil {
			t.Fatalf("ReadAllRecords failed: %v", err)
		}

		if len(readRecords) != 2 {
			t.Errorf("Expected 2 records, got %d", len(readRecords))
		}
	})

	// Test CSV format
	t.Run("CSV", func(t *testing.T) {
		var buf bytes.Buffer
		err := WriteAllRecords(&buf, records, "csv")
		if err != nil {
			t.Fatalf("WriteAllRecords CSV failed: %v", err)
		}

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 2 {
			t.Error("CSV output should have header and data rows")
		}
	})

	// Test table format
	t.Run("Table", func(t *testing.T) {
		var buf bytes.Buffer
		err := WriteAllRecords(&buf, records, "table")
		if err != nil {
			t.Fatalf("WriteAllRecords table failed: %v", err)
		}

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 3 { // header, separator, data
			t.Error("Table output should have header, separator, and data rows")
		}
	})
}

// TestIsRecordStream tests detection of record streams.
func TestIsRecordStream(t *testing.T) {
	// Test with record stream
	t.Run("WithMagic", func(t *testing.T) {
		input := RecordMagic + `{"test": true}`
		isRecord, r := IsRecordStream(strings.NewReader(input))
		if !isRecord {
			t.Error("Should detect record stream")
		}

		// Read the rest to verify reader is still usable
		remaining, _ := io.ReadAll(r)
		if !strings.Contains(string(remaining), "test") {
			t.Error("Reader should still contain record data")
		}
	})

	// Test without record stream
	t.Run("WithoutMagic", func(t *testing.T) {
		input := `{"test": true}`
		isRecord, r := IsRecordStream(strings.NewReader(input))
		if isRecord {
			t.Error("Should not detect record stream without magic")
		}

		// Read to verify content is preserved
		remaining, _ := io.ReadAll(r)
		if !strings.Contains(string(remaining), "test") {
			t.Error("Reader should preserve original content")
		}
	})
}

// TestGetField tests nested field access.
func TestGetField(t *testing.T) {
	record := Record{
		"name": "test",
		"nested": map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": "deep value",
			},
		},
	}

	// Test simple field
	if GetField(record, "name") != "test" {
		t.Error("Failed to get simple field")
	}

	// Test nested field
	if GetField(record, "nested/level1/level2") != "deep value" {
		t.Error("Failed to get nested field")
	}

	// Test non-existent field
	if GetField(record, "nonexistent") != nil {
		t.Error("Non-existent field should return nil")
	}
}

// TestSetField tests nested field setting.
func TestSetField(t *testing.T) {
	record := make(Record)

	// Set simple field
	SetField(record, "name", "test")
	if record["name"] != "test" {
		t.Error("Failed to set simple field")
	}

	// Set nested field
	SetField(record, "nested/level1/value", "deep")
	if GetField(record, "nested/level1/value") != "deep" {
		t.Error("Failed to set nested field")
	}
}

// TestFromJSON tests the from-json command.
func TestFromJSON(t *testing.T) {
	// Create a temp file with JSON lines
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")
	content := `{"name":"alice","age":30}
{"name":"bob","age":25}
`
	err := os.WriteFile(jsonFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run from-json command
	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommand(t, "from-json "+jsonFile, &stdout, &stderr)
	cmd.Run()

	output := stdout.String()

	// Should have magic header
	if !strings.HasPrefix(output, RecordMagic) {
		t.Error("from-json output should start with RecordMagic")
	}

	// Should have records
	if !strings.Contains(output, "alice") || !strings.Contains(output, "bob") {
		t.Error("from-json output should contain record data")
	}
}

// TestToJSON tests the to-json command.
func TestToJSON(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
`

	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommandWithStdin(t, "to-json", strings.NewReader(input), &stdout, &stderr)
	cmd.Run()

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have 2 records
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	// Each line should be valid JSON
	for _, line := range lines {
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Errorf("Invalid JSON in output: %s", line)
		}
	}
}

// TestFromCSV tests the from-csv command.
func TestFromCSV(t *testing.T) {
	// Create a temp file with CSV
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	content := `name,age
alice,30
bob,25
`
	err := os.WriteFile(csvFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommand(t, "from-csv "+csvFile, &stdout, &stderr)
	cmd.Run()

	output := stdout.String()

	// Should have magic header
	if !strings.HasPrefix(output, RecordMagic) {
		t.Error("from-csv output should start with RecordMagic")
	}

	// Should have records with name and age fields
	if !strings.Contains(output, `"name":"alice"`) {
		t.Error("from-csv output should contain alice record")
	}
}

// TestToCSV tests the to-csv command.
func TestToCSV(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
`

	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommandWithStdin(t, "to-csv", strings.NewReader(input), &stdout, &stderr)
	cmd.Run()

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 2 data rows
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines (header + 2 data), got %d", len(lines))
	}

	// Header should contain field names
	if !strings.Contains(lines[0], "name") || !strings.Contains(lines[0], "age") {
		t.Error("CSV header should contain field names")
	}
}

// TestToTable tests the to-table command.
func TestToTable(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
`

	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommandWithStdin(t, "to-table", strings.NewReader(input), &stdout, &stderr)
	cmd.Run()

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + separator + 2 data rows
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines (header + separator + 2 data), got %d: %v", len(lines), lines)
	}
}

// TestSelectFields tests the select-fields command.
func TestSelectFields(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30,"city":"nyc"}
{"name":"bob","age":25,"city":"la"}
`

	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommandWithStdin(t, "select-fields name city", strings.NewReader(input), &stdout, &stderr)
	cmd.Run()

	output := stdout.String()

	// Should have magic header
	if !strings.HasPrefix(output, RecordMagic) {
		t.Error("select-fields output should start with RecordMagic")
	}

	// Should have name and city but not age
	if !strings.Contains(output, "name") || !strings.Contains(output, "city") {
		t.Error("select-fields should include selected fields")
	}

	// Should not have age field
	if strings.Contains(output, `"age"`) {
		t.Error("select-fields should not include non-selected fields")
	}
}

// TestWhere tests the where filter command.
func TestWhere(t *testing.T) {
	input := RecordMagic + `{"name":"alice","age":30}
{"name":"bob","age":25}
{"name":"charlie","age":35}
`

	// Test equality
	t.Run("Equality", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cmd := createRecordTestCommandWithStdin(t, "where name == alice", strings.NewReader(input), &stdout, &stderr)
		cmd.Run()

		output := stdout.String()
		if !strings.Contains(output, "alice") || strings.Contains(output, "bob") {
			t.Error("where == should filter to matching records only")
		}
	})

	// Test greater than
	t.Run("GreaterThan", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		// Note: operator must be quoted since > would be parsed as redirection
		cmd := createRecordTestCommandWithStdin(t, `where age ">" 26`, strings.NewReader(input), &stdout, &stderr)
		cmd.Run()

		output := stdout.String()
		if strings.Contains(output, "bob") {
			t.Error("where > should exclude records that don't match")
		}
		if !strings.Contains(output, "alice") || !strings.Contains(output, "charlie") {
			t.Error("where > should include records that match")
		}
	})
}

// TestLsRecords tests the ls --records command.
func TestLsRecords(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("test data"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	// Test with --records flag
	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommand(t, "ls --records "+tmpDir, &stdout, &stderr)
	cmd.Run()

	output := stdout.String()

	// Should have magic header
	if !strings.HasPrefix(output, RecordMagic) {
		t.Error("ls --records output should start with RecordMagic")
	}

	// Should contain file info as records
	if !strings.Contains(output, "file1.txt") {
		t.Error("ls --records should include file1.txt")
	}

	// Should have size field
	if !strings.Contains(output, `"size"`) {
		t.Error("ls --records should include size field")
	}

	// Should have isdir field
	if !strings.Contains(output, `"isdir"`) {
		t.Error("ls --records should include isdir field")
	}
}

// TestEnvRecords tests the env --records command.
func TestEnvRecords(t *testing.T) {
	// Set a test environment variable
	os.Setenv("GOSH_TEST_VAR", "test_value")
	defer os.Unsetenv("GOSH_TEST_VAR")

	var stdout, stderr bytes.Buffer
	cmd := createRecordTestCommand(t, "env --records", &stdout, &stderr)
	cmd.Run()

	output := stdout.String()

	// Should have magic header
	if !strings.HasPrefix(output, RecordMagic) {
		t.Error("env --records output should start with RecordMagic")
	}

	// Should contain our test variable
	if !strings.Contains(output, "GOSH_TEST_VAR") {
		t.Error("env --records should include test variable")
	}
}

// TestRecordPipeline tests piping records between commands.
func TestRecordPipeline(t *testing.T) {
	// Create a temp file with JSON
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")
	content := `{"name":"alice","age":30,"status":"active"}
{"name":"bob","age":25,"status":"inactive"}
{"name":"charlie","age":35,"status":"active"}
`
	err := os.WriteFile(jsonFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test: from-json | where | select-fields | to-table
	// This simulates a pipeline by running commands sequentially

	// Step 1: from-json
	var buf1 bytes.Buffer
	cmd1 := createRecordTestCommand(t, "from-json "+jsonFile, &buf1, &bytes.Buffer{})
	cmd1.Run()

	// Step 2: where status == active
	var buf2 bytes.Buffer
	cmd2 := createRecordTestCommandWithStdin(t, "where status == active", &buf1, &buf2, &bytes.Buffer{})
	cmd2.Run()

	// Step 3: select-fields name age
	var buf3 bytes.Buffer
	cmd3 := createRecordTestCommandWithStdin(t, "select-fields name age", &buf2, &buf3, &bytes.Buffer{})
	cmd3.Run()

	// Step 4: to-table
	var buf4 bytes.Buffer
	cmd4 := createRecordTestCommandWithStdin(t, "to-table", &buf3, &buf4, &bytes.Buffer{})
	cmd4.Run()

	output := buf4.String()

	// Should have alice and charlie but not bob
	if !strings.Contains(output, "alice") || !strings.Contains(output, "charlie") {
		t.Error("Pipeline should include active users")
	}
	if strings.Contains(output, "bob") {
		t.Error("Pipeline should exclude inactive users")
	}

	// Should not have status field (not selected)
	if strings.Contains(output, "status") {
		t.Error("Pipeline should not include non-selected fields")
	}
}

// Helper functions for creating test commands

func createRecordTestCommand(t *testing.T, cmdStr string, stdout, stderr *bytes.Buffer) *Command {
	cmd, err := NewCommand(cmdStr, nil)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = strings.NewReader("")
	return cmd
}

func createRecordTestCommandWithStdin(t *testing.T, cmdStr string, stdin io.Reader, stdout, stderr *bytes.Buffer) *Command {
	cmd, err := NewCommand(cmdStr, nil)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	return cmd
}

// TestMatchCondition tests the where filter matching logic.
func TestMatchCondition(t *testing.T) {
	tests := []struct {
		fieldVal interface{}
		op       string
		value    string
		expected bool
	}{
		// Equality
		{"test", "==", "test", true},
		{"test", "==", "other", false},
		{"test", "=", "test", true},

		// Inequality
		{"test", "!=", "other", true},
		{"test", "!=", "test", false},

		// Numeric comparisons
		{30, ">", "25", true},
		{30, ">", "35", false},
		{30, ">=", "30", true},
		{30, "<", "35", true},
		{30, "<=", "30", true},

		// String comparisons (lexicographic)
		{"bob", ">", "alice", true},
		{"alice", "<", "bob", true},

		// Contains
		{"hello world", "=~", "world", true},
		{"hello world", "=~", "foo", false},

		// Starts with
		{"hello world", "^=", "hello", true},
		{"hello world", "^=", "world", false},

		// Ends with
		{"hello world", "$=", "world", true},
		{"hello world", "$=", "hello", false},
	}

	for _, test := range tests {
		result := matchCondition(test.fieldVal, test.op, test.value)
		if result != test.expected {
			t.Errorf("matchCondition(%v, %q, %q) = %v, expected %v",
				test.fieldVal, test.op, test.value, result, test.expected)
		}
	}
}
