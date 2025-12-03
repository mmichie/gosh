// Package gosh provides builtin commands for working with record streams.
//
// These commands form the foundation of gosh's data-oriented shell capabilities:
//   - from-json: Parse JSON lines into record stream
//   - to-json: Convert record stream to JSON lines
//   - from-csv: Parse CSV into record stream
//   - to-csv: Convert record stream to CSV
//   - to-table: Format record stream as a table
//   - select-fields: Select specific fields from records
//   - where: Filter records based on conditions
package gosh

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gosh/parser"
)

func init() {
	// Register record stream commands
	builtins["from-json"] = fromJSON
	builtins["to-json"] = toJSON
	builtins["from-csv"] = fromCSV
	builtins["to-csv"] = toCSV
	builtins["to-table"] = toTable
	builtins["select-fields"] = selectFields
	builtins["where"] = whereFilter
}

// Note: getBuiltinArgs is defined in essential_builtins.go

// fromJSON reads JSON lines from stdin and outputs them as a record stream.
// Usage: from-json [file...]
// If no files are specified, reads from stdin.
func fromJSON(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Determine input source
	var readers []io.Reader

	if len(args) > 0 {
		// Read from files
		for _, filename := range args {
			file, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("from-json: %v", err)
			}
			defer file.Close()
			readers = append(readers, file)
		}
	} else {
		// Read from stdin
		readers = append(readers, cmd.Stdin)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	// Process each reader
	for _, r := range readers {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Validate it's valid JSON
			var record Record
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				// Try to parse as array of records
				var records []Record
				if err2 := json.Unmarshal([]byte(line), &records); err2 == nil {
					// Output each record
					for _, rec := range records {
						data, _ := json.Marshal(rec)
						fmt.Fprintln(cmd.Stdout, string(data))
					}
					continue
				}
				// Skip invalid JSON lines
				fmt.Fprintf(cmd.Stderr, "from-json: skipping invalid JSON: %s\n", line)
				continue
			}

			// Output the valid JSON line
			fmt.Fprintln(cmd.Stdout, line)
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("from-json: %v", err)
		}
	}

	return nil
}

// toJSON converts a record stream to JSON lines output.
// Usage: to-json [-p|--pretty]
func toJSON(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	pretty := false
	for _, arg := range args {
		switch arg {
		case "-p", "--pretty":
			pretty = true
		}
	}

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("to-json: %v", err)
	}

	// Output records
	for _, record := range records {
		var data []byte
		if pretty {
			data, err = json.MarshalIndent(record, "", "  ")
		} else {
			data, err = json.Marshal(record)
		}
		if err != nil {
			return fmt.Errorf("to-json: %v", err)
		}
		fmt.Fprintln(cmd.Stdout, string(data))
	}

	return nil
}

// fromCSV reads CSV from stdin and outputs it as a record stream.
// Usage: from-csv [options] [file...]
// Options:
//
//	-H, --no-header   First line is data, not headers
//	-d, --delimiter   Field delimiter (default: comma)
func fromCSV(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	hasHeader := true
	delimiter := ','
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-H", "--no-header":
			hasHeader = false
		case "-d", "--delimiter":
			if i+1 < len(args) {
				i++
				if len(args[i]) > 0 {
					delimiter = rune(args[i][0])
				}
			}
		default:
			if !strings.HasPrefix(arg, "-") {
				files = append(files, arg)
			}
		}
	}

	// Determine input source
	var readers []io.Reader
	if len(files) > 0 {
		for _, filename := range files {
			file, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("from-csv: %v", err)
			}
			defer file.Close()
			readers = append(readers, file)
		}
	} else {
		readers = append(readers, cmd.Stdin)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	// Process each reader
	for _, r := range readers {
		csvReader := csv.NewReader(r)
		csvReader.Comma = delimiter

		var headers []string
		lineNum := 0

		for {
			line, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				// Handle variable field counts
				if strings.Contains(err.Error(), "wrong number of fields") {
					continue
				}
				return fmt.Errorf("from-csv: %v", err)
			}
			lineNum++

			// Handle headers
			if lineNum == 1 {
				if hasHeader {
					headers = line
					continue
				} else {
					// Generate headers like col1, col2, col3...
					headers = make([]string, len(line))
					for i := range headers {
						headers[i] = fmt.Sprintf("col%d", i+1)
					}
				}
			}

			// Create record
			record := make(Record)
			for i, header := range headers {
				if i < len(line) {
					record[header] = line[i]
				}
			}

			// Output as JSON
			data, err := json.Marshal(record)
			if err != nil {
				return fmt.Errorf("from-csv: %v", err)
			}
			fmt.Fprintln(cmd.Stdout, string(data))
		}
	}

	return nil
}

// toCSV converts a record stream to CSV output.
// Usage: to-csv [options]
// Options:
//
//	-H, --no-header   Don't output header row
//	-d, --delimiter   Field delimiter (default: comma)
//	-f, --fields      Comma-separated list of fields to include
func toCSV(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	writeHeader := true
	delimiter := ','
	var selectedFields []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-H", "--no-header":
			writeHeader = false
		case "-d", "--delimiter":
			if i+1 < len(args) {
				i++
				if len(args[i]) > 0 {
					delimiter = rune(args[i][0])
				}
			}
		case "-f", "--fields":
			if i+1 < len(args) {
				i++
				selectedFields = strings.Split(args[i], ",")
			}
		}
	}

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("to-csv: %v", err)
	}

	if len(records) == 0 {
		return nil
	}

	// Determine fields
	var fields []string
	if len(selectedFields) > 0 {
		fields = selectedFields
	} else {
		fields = getRecordFields(records[0])
	}

	// Setup CSV writer
	csvWriter := csv.NewWriter(cmd.Stdout)
	csvWriter.Comma = delimiter
	defer csvWriter.Flush()

	// Write header
	if writeHeader {
		if err := csvWriter.Write(fields); err != nil {
			return fmt.Errorf("to-csv: %v", err)
		}
	}

	// Write data
	for _, record := range records {
		row := make([]string, len(fields))
		for i, field := range fields {
			if val, ok := record[field]; ok {
				row[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("to-csv: %v", err)
		}
	}

	return nil
}

// toTable formats a record stream as a table.
// Usage: to-table [options]
// Options:
//
//	-f, --fields     Comma-separated list of fields to include
//	-H, --no-header  Don't output header row
func toTable(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	writeHeader := true
	var selectedFields []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-H", "--no-header":
			writeHeader = false
		case "-f", "--fields":
			if i+1 < len(args) {
				i++
				selectedFields = strings.Split(args[i], ",")
			}
		}
	}

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("to-table: %v", err)
	}

	if len(records) == 0 {
		return nil
	}

	// Determine fields
	var fields []string
	if len(selectedFields) > 0 {
		fields = selectedFields
	} else {
		fields = getRecordFields(records[0])
	}

	// Calculate column widths
	widths := make([]int, len(fields))
	for i, field := range fields {
		widths[i] = len(field)
	}
	for _, record := range records {
		for i, field := range fields {
			val := fmt.Sprintf("%v", record[field])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	// Build format string
	formats := make([]string, len(fields))
	for i, w := range widths {
		formats[i] = fmt.Sprintf("%%-%ds", w)
	}
	formatStr := strings.Join(formats, "  ")

	// Write header
	if writeHeader {
		headerRow := make([]interface{}, len(fields))
		for i, field := range fields {
			headerRow[i] = field
		}
		fmt.Fprintln(cmd.Stdout, fmt.Sprintf(formatStr, headerRow...))

		// Write separator
		separators := make([]interface{}, len(fields))
		for i, w := range widths {
			separators[i] = strings.Repeat("-", w)
		}
		fmt.Fprintln(cmd.Stdout, fmt.Sprintf(formatStr, separators...))
	}

	// Write data rows
	for _, record := range records {
		row := make([]interface{}, len(fields))
		for i, field := range fields {
			if val, ok := record[field]; ok {
				row[i] = fmt.Sprintf("%v", val)
			} else {
				row[i] = ""
			}
		}
		fmt.Fprintln(cmd.Stdout, fmt.Sprintf(formatStr, row...))
	}

	return nil
}

// selectFields selects specific fields from records.
// Usage: select-fields field1 field2 ...
func selectFields(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return fmt.Errorf("select-fields: at least one field name required")
	}

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("select-fields: %v", err)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	// Output selected fields only
	for _, record := range records {
		newRecord := make(Record)
		for _, field := range args {
			if val, ok := record[field]; ok {
				newRecord[field] = val
			}
		}

		data, err := json.Marshal(newRecord)
		if err != nil {
			return fmt.Errorf("select-fields: %v", err)
		}
		fmt.Fprintln(cmd.Stdout, string(data))
	}

	return nil
}

// whereFilter filters records based on a condition.
// Usage: where field op value
// Examples:
//
//	where status == active
//	where count > 10
//	where name =~ "^test"
func whereFilter(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) < 3 {
		return fmt.Errorf("where: requires field, operator, and value (e.g., where status == active)")
	}

	field := args[0]
	op := strings.Trim(args[1], "\"'") // Remove quotes from operator
	value := strings.Join(args[2:], " ")

	// Remove quotes from value if present
	value = strings.Trim(value, "\"'")

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("where: %v", err)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	// Filter records
	for _, record := range records {
		fieldVal, ok := record[field]
		if !ok {
			continue
		}

		if matchCondition(fieldVal, op, value) {
			data, err := json.Marshal(record)
			if err != nil {
				return fmt.Errorf("where: %v", err)
			}
			fmt.Fprintln(cmd.Stdout, string(data))
		}
	}

	return nil
}

// matchCondition checks if a field value matches the condition.
func matchCondition(fieldVal interface{}, op string, value string) bool {
	fieldStr := fmt.Sprintf("%v", fieldVal)

	switch op {
	case "==", "=":
		return fieldStr == value

	case "!=", "<>":
		return fieldStr != value

	case ">":
		fv, err1 := strconv.ParseFloat(fieldStr, 64)
		v, err2 := strconv.ParseFloat(value, 64)
		if err1 != nil || err2 != nil {
			return fieldStr > value
		}
		return fv > v

	case ">=":
		fv, err1 := strconv.ParseFloat(fieldStr, 64)
		v, err2 := strconv.ParseFloat(value, 64)
		if err1 != nil || err2 != nil {
			return fieldStr >= value
		}
		return fv >= v

	case "<":
		fv, err1 := strconv.ParseFloat(fieldStr, 64)
		v, err2 := strconv.ParseFloat(value, 64)
		if err1 != nil || err2 != nil {
			return fieldStr < value
		}
		return fv < v

	case "<=":
		fv, err1 := strconv.ParseFloat(fieldStr, 64)
		v, err2 := strconv.ParseFloat(value, 64)
		if err1 != nil || err2 != nil {
			return fieldStr <= value
		}
		return fv <= v

	case "=~", "~=":
		// Contains check (simple substring match)
		return strings.Contains(strings.ToLower(fieldStr), strings.ToLower(value))

	case "^=":
		// Starts with
		return strings.HasPrefix(fieldStr, value)

	case "$=":
		// Ends with
		return strings.HasSuffix(fieldStr, value)

	default:
		return false
	}
}

// readRecordsFromInput reads records from an io.Reader.
// It handles both record streams (with magic header) and plain JSON lines.
func readRecordsFromInput(r io.Reader) ([]Record, error) {
	scanner := bufio.NewScanner(r)
	var records []Record
	firstLine := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip magic header
		if firstLine && line == strings.TrimSuffix(RecordMagic, "\n") {
			firstLine = false
			continue
		}
		firstLine = false

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse JSON
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			// Skip invalid JSON lines
			continue
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return records, err
	}

	return records, nil
}

// lsRecords implements ls with record output for use with --records flag.
func lsRecords(cmd *Command, args []string) error {
	dir := "."
	showHidden := false

	for _, arg := range args {
		if arg == "-a" || arg == "--all" {
			showHidden = true
		} else if !strings.HasPrefix(arg, "-") {
			dir = arg
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless -a flag
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		record := Record{
			"name":    name,
			"size":    info.Size(),
			"mode":    info.Mode().String(),
			"modtime": info.ModTime().Format("2006-01-02 15:04:05"),
			"isdir":   entry.IsDir(),
		}

		data, _ := json.Marshal(record)
		fmt.Fprintln(cmd.Stdout, string(data))
	}

	return nil
}

// Helper function to get the ls builtin args and check for --records flag
func checkRecordsFlag(cmd *Command) bool {
	args := getBuiltinArgs(cmd)
	for _, arg := range args {
		if arg == "--records" || arg == "-R" {
			return true
		}
	}
	return false
}

// Helper function to format a command with specific parts
func formatCmdParts(parts []string) string {
	return parser.FormatCommand(&parser.Command{
		LogicalBlocks: []*parser.LogicalBlock{
			{
				FirstPipeline: &parser.Pipeline{
					Commands: []*parser.CommandElement{
						{
							Simple: &parser.SimpleCommand{
								Parts: parts,
							},
						},
					},
				},
			},
		},
	})
}
