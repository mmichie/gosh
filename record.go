// Package gosh provides the core record stream types and interfaces for
// gosh's data-oriented shell features.
//
// Record streams allow structured data to flow through pipelines, enabling
// powerful data processing capabilities similar to PowerShell or Nushell.
package gosh

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// Record represents a single structured data record as a map of field names to values.
// Values can be strings, numbers, booleans, nested maps, or arrays.
type Record map[string]interface{}

// RecordMagic is the prefix used to identify record stream data in pipelines.
// When a command outputs data starting with this magic string, downstream
// commands know to parse it as records rather than plain text.
const RecordMagic = "\x1e\x1fRECORDS\x1f\x1e\n"

// RecordStream represents a stream of records that can be read from or written to.
type RecordStream interface {
	// Read returns the next record from the stream.
	// Returns io.EOF when there are no more records.
	Read() (Record, error)

	// Close closes the stream and releases any resources.
	Close() error
}

// RecordEmitter is implemented by commands that can output structured records.
type RecordEmitter interface {
	// EmitRecords writes records to the given writer.
	// If recordMode is true, outputs in record format; otherwise plain text.
	EmitRecords(w io.Writer, recordMode bool) error
}

// RecordConsumer is implemented by commands that can process structured records.
type RecordConsumer interface {
	// ConsumeRecords reads records from the given reader and processes them.
	ConsumeRecords(r io.Reader) error
}

// RecordReader reads records from an io.Reader containing JSON lines or record format.
type RecordReader struct {
	reader  *bufio.Reader
	scanner *bufio.Scanner
	started bool
}

// NewRecordReader creates a new RecordReader from an io.Reader.
func NewRecordReader(r io.Reader) *RecordReader {
	return &RecordReader{
		reader:  bufio.NewReader(r),
		scanner: bufio.NewScanner(r),
	}
}

// Read returns the next record from the stream.
func (rr *RecordReader) Read() (Record, error) {
	if !rr.scanner.Scan() {
		if err := rr.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	line := rr.scanner.Text()

	// Skip the magic header if present
	if !rr.started && line == strings.TrimSuffix(RecordMagic, "\n") {
		rr.started = true
		return rr.Read() // Read the next line
	}
	rr.started = true

	// Skip empty lines
	if strings.TrimSpace(line) == "" {
		return rr.Read()
	}

	// Parse as JSON
	var record Record
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return nil, fmt.Errorf("invalid record format: %v", err)
	}

	return record, nil
}

// Close closes the RecordReader.
func (rr *RecordReader) Close() error {
	return nil
}

// RecordWriter writes records to an io.Writer.
type RecordWriter struct {
	writer      io.Writer
	format      string // "json", "csv", "table"
	fields      []string
	wroteHeader bool
}

// NewRecordWriter creates a new RecordWriter.
func NewRecordWriter(w io.Writer, format string, fields []string) *RecordWriter {
	return &RecordWriter{
		writer: w,
		format: format,
		fields: fields,
	}
}

// WriteHeader writes the record stream header (magic bytes for record format).
func (rw *RecordWriter) WriteHeader() error {
	if rw.format == "json" || rw.format == "records" {
		_, err := rw.writer.Write([]byte(RecordMagic))
		return err
	}
	return nil
}

// Write writes a single record to the stream.
func (rw *RecordWriter) Write(record Record) error {
	switch rw.format {
	case "json", "records":
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(rw.writer, string(data))
		return err

	case "csv":
		return rw.writeCSV(record)

	case "table":
		return rw.writeTableRow(record)

	default:
		// Default to JSON
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(rw.writer, string(data))
		return err
	}
}

// writeCSV writes a record in CSV format.
func (rw *RecordWriter) writeCSV(record Record) error {
	// Initialize fields from record if not set
	if len(rw.fields) == 0 {
		rw.fields = getRecordFields(record)
	}

	// Write header if not written yet
	if !rw.wroteHeader {
		_, err := fmt.Fprintln(rw.writer, strings.Join(rw.fields, ","))
		if err != nil {
			return err
		}
		rw.wroteHeader = true
	}

	// Write values
	values := make([]string, len(rw.fields))
	for i, field := range rw.fields {
		if val, ok := record[field]; ok {
			values[i] = formatRecordValue(val)
		}
	}
	_, err := fmt.Fprintln(rw.writer, strings.Join(values, ","))
	return err
}

// writeTableRow writes a record as a table row.
func (rw *RecordWriter) writeTableRow(record Record) error {
	// Initialize fields from record if not set
	if len(rw.fields) == 0 {
		rw.fields = getRecordFields(record)
	}

	// For table format, we just write the values with tabs
	values := make([]string, len(rw.fields))
	for i, field := range rw.fields {
		if val, ok := record[field]; ok {
			values[i] = formatRecordValue(val)
		}
	}
	_, err := fmt.Fprintln(rw.writer, strings.Join(values, "\t"))
	return err
}

// Close closes the RecordWriter.
func (rw *RecordWriter) Close() error {
	return nil
}

// getRecordFields returns the field names from a record in sorted order.
func getRecordFields(record Record) []string {
	fields := make([]string, 0, len(record))
	for field := range record {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

// formatRecordValue formats a record value for text output.
func formatRecordValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		// Escape commas and quotes for CSV
		if strings.ContainsAny(val, ",\"\n") {
			return fmt.Sprintf("%q", val)
		}
		return val
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// IsRecordStream checks if the input starts with the record magic header.
func IsRecordStream(r io.Reader) (bool, io.Reader) {
	// Read enough bytes to check for magic
	buf := make([]byte, len(RecordMagic))
	n, err := io.ReadFull(r, buf)
	if err != nil || n < len(RecordMagic) {
		// Not enough data or error - create a new reader with what we read
		return false, io.MultiReader(strings.NewReader(string(buf[:n])), r)
	}

	// Check if it matches the magic
	if string(buf) == RecordMagic {
		return true, r // Return original reader positioned after magic
	}

	// Not a record stream - prepend what we read
	return false, io.MultiReader(strings.NewReader(string(buf)), r)
}

// ReadAllRecords reads all records from an io.Reader.
func ReadAllRecords(r io.Reader) ([]Record, error) {
	rr := NewRecordReader(r)
	defer rr.Close()

	var records []Record
	for {
		record, err := rr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return records, err
		}
		records = append(records, record)
	}
	return records, nil
}

// WriteAllRecords writes all records to an io.Writer in the specified format.
func WriteAllRecords(w io.Writer, records []Record, format string) error {
	if len(records) == 0 {
		return nil
	}

	switch format {
	case "json", "records":
		// Write magic header
		fmt.Fprint(w, RecordMagic)
		for _, record := range records {
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, string(data))
		}
		return nil

	case "csv":
		return writeCSVRecords(w, records)

	case "table":
		return writeTableRecords(w, records)

	default:
		// Default to JSON lines
		for _, record := range records {
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, string(data))
		}
		return nil
	}
}

// writeCSVRecords writes records in CSV format.
func writeCSVRecords(w io.Writer, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	// Get fields from first record
	fields := getRecordFields(records[0])

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	// Write header
	if err := csvWriter.Write(fields); err != nil {
		return err
	}

	// Write data rows
	for _, record := range records {
		row := make([]string, len(fields))
		for i, field := range fields {
			if val, ok := record[field]; ok {
				row[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeTableRecords writes records in a formatted table.
func writeTableRecords(w io.Writer, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	// Get fields from first record
	fields := getRecordFields(records[0])

	// Use tabwriter for aligned columns
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Write header
	fmt.Fprintln(tw, strings.Join(fields, "\t"))

	// Write separator
	separators := make([]string, len(fields))
	for i, field := range fields {
		separators[i] = strings.Repeat("-", len(field))
	}
	fmt.Fprintln(tw, strings.Join(separators, "\t"))

	// Write data rows
	for _, record := range records {
		values := make([]string, len(fields))
		for i, field := range fields {
			if val, ok := record[field]; ok {
				values[i] = fmt.Sprintf("%v", val)
			}
		}
		fmt.Fprintln(tw, strings.Join(values, "\t"))
	}

	return nil
}

// ParseJSONLine parses a single JSON line into a Record.
func ParseJSONLine(line string) (Record, error) {
	var record Record
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return nil, err
	}
	return record, nil
}

// ParseCSVLine parses a CSV line with headers into a Record.
func ParseCSVLine(line string, headers []string) (Record, error) {
	reader := csv.NewReader(strings.NewReader(line))
	values, err := reader.Read()
	if err != nil {
		return nil, err
	}

	record := make(Record)
	for i, header := range headers {
		if i < len(values) {
			record[header] = values[i]
		}
	}
	return record, nil
}

// GetField returns a field value from a record, supporting nested access with "/" notation.
// For example: GetField(record, "user/name") returns record["user"]["name"]
func GetField(record Record, field string) interface{} {
	parts := strings.Split(field, "/")
	var current interface{} = record

	for _, part := range parts {
		switch v := current.(type) {
		case Record:
			current = v[part]
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

// SetField sets a field value in a record, supporting nested access with "/" notation.
func SetField(record Record, field string, value interface{}) {
	parts := strings.Split(field, "/")
	if len(parts) == 1 {
		record[field] = value
		return
	}

	// Navigate to the parent
	current := record
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if next, ok := current[part]; ok {
			if m, ok := next.(map[string]interface{}); ok {
				current = m
			} else {
				// Create nested map
				newMap := make(map[string]interface{})
				current[part] = newMap
				current = newMap
			}
		} else {
			// Create nested map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}
