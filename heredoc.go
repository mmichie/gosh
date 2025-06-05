package gosh

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"time"
)

// HereDoc represents a here-document with its delimiter and content
type HereDoc struct {
	Delimiter   string
	Content     string
	StripTabs   bool
	ContentType string // "heredoc" or "herestring"
}

// HereDocMap stores here-documents with identifiers for retrieval during execution
type HereDocMap map[string]*HereDoc

// Regular expressions for here-document detection
var (
	// << DELIMITER
	hereDocPattern = regexp.MustCompile(`<<\s*(\-?)\s*([a-zA-Z0-9_]+|'[^']*'|"[^"]*")`)

	// <<< "string"
	hereStringPattern = regexp.MustCompile(`<<<\s+(.+)`)
)

// PreprocessHereDoc preprocess a command for here-documents
// This function extracts here-documents from command input to handle them separately
// It returns:
// - Modified command with GUID placeholder for here-docs
// - Map of here-doc GUIDs to content
// - Error if any occurred
func PreprocessHereDoc(input string) (string, HereDocMap, error) {
	// Map of here-docs for later retrieval
	hereDocs := make(HereDocMap)

	// Handle here-strings (<<<) first since they're more specific than <<
	var lastIndex int
	for lastIndex < len(input) {
		matches := hereStringPattern.FindStringSubmatchIndex(input[lastIndex:])
		if matches == nil {
			break
		}

		// Adjust indices to account for lastIndex
		for i := range matches {
			if matches[i] >= 0 {
				matches[i] += lastIndex
			}
		}

		// Extract matches
		_ = input[matches[0]:matches[1]] // fullMatch - not currently used
		rawContent := input[matches[2]:matches[3]] // The string content with possible quotes
		
		// Handle quoted and unquoted strings
		content := strings.TrimSpace(rawContent)
		if (strings.HasPrefix(content, "\"") && strings.HasSuffix(content, "\"")) ||
		   (strings.HasPrefix(content, "'") && strings.HasSuffix(content, "'")) {
			content = content[1 : len(content)-1]
		}

		// Generate a unique identifier for this here-string
		hereStringID := "herestring_" + NewGUID()

		// Create a here-doc object for here-string
		hereDoc := &HereDoc{
			Content:     content,
			ContentType: "herestring",
		}

		// Store the here-string
		hereDocs[hereStringID] = hereDoc

		// Replace the original here-string with a placeholder
		placeholder := "< " + hereStringID

		// Replace in the original input
		// For here-strings, we just replace the entire match
		input = input[:matches[0]] + placeholder + input[matches[1]:]

		// Update lastIndex to avoid re-matching the same here-string
		lastIndex = matches[0] + len(placeholder)
	}

	// Check for standard here-documents (<<DELIMITER)
	lastIndex = 0
	for lastIndex < len(input) {
		matches := hereDocPattern.FindStringSubmatchIndex(input[lastIndex:])
		if matches == nil {
			break
		}

		// Adjust indices to account for lastIndex
		for i := range matches {
			if matches[i] >= 0 {
				matches[i] += lastIndex
			}
		}

		// Extract matches
		_ = input[matches[0]:matches[1]] // fullMatch - not currently used
		dashFlag := strings.TrimSpace(input[matches[2]:matches[3]]) // "-" for tab stripping
		delimiterWithQuotes := input[matches[4]:matches[5]]         // The delimiter, possibly with quotes

		// Clean up the delimiter (remove quotes)
		delimiter := delimiterWithQuotes
		if strings.HasPrefix(delimiter, "'") && strings.HasSuffix(delimiter, "'") {
			delimiter = delimiter[1 : len(delimiter)-1]
		} else if strings.HasPrefix(delimiter, "\"") && strings.HasSuffix(delimiter, "\"") {
			delimiter = delimiter[1 : len(delimiter)-1]
		}

		// Look for the here-doc content (everything from end of << to the delimiter on its own line)
		endOfHereDocTag := matches[1]
		contentStart := endOfHereDocTag

		// Find the end delimiter (must be on a line by itself, optionally with tabs if dash specified)
		var endPattern string
		if dashFlag == "-" {
			endPattern = `(?m)^\s*` + regexp.QuoteMeta(delimiter) + `$`
		} else {
			endPattern = `(?m)^` + regexp.QuoteMeta(delimiter) + `$`
		}

		endRegexp := regexp.MustCompile(endPattern)
		contentEndMatch := endRegexp.FindStringIndex(input[contentStart:])
		if contentEndMatch == nil {
			// End delimiter not found, treat the rest of input as content
			contentEndMatch = []int{len(input) - contentStart, len(input) - contentStart}
		}

		contentEnd := contentStart + contentEndMatch[0]
		content := input[contentStart:contentEnd]

		// Generate a unique identifier for this here-doc
		hereDocID := "heredoc_" + delimiter + "_" + NewGUID()

		// Create a here-doc object
		hereDoc := &HereDoc{
			Delimiter:   delimiter,
			Content:     content,
			StripTabs:   dashFlag == "-",
			ContentType: "heredoc",
		}

		// Store the here-doc
		hereDocs[hereDocID] = hereDoc

		// Replace the original here-doc with a placeholder
		// Format: < [GUID] (appears like a file redirection to the parser)
		placeholder := "< " + hereDocID

		// Replace in the original input
		// We need to replace the full here-doc (delimiter line through content to end delimiter)
		endPos := contentEnd
		if contentEndMatch != nil && contentEndMatch[0] < len(input)-contentStart {
			// Include the delimiter line and any trailing newline
			endPos = contentStart + contentEndMatch[1]
		}
		if endPos > len(input) {
			endPos = len(input)
		}
		input = input[:matches[0]] + placeholder + input[endPos:]

		// Update lastIndex to avoid re-matching the same here-doc
		lastIndex = matches[0] + len(placeholder)
	}

	return input, hereDocs, nil
}

// ProcessHereDoc processes the here-document by:
// - Removing the delimiter lines
// - Stripping tabs if needed
// Returns a string reader with the processed content
func ProcessHereDoc(heredoc *HereDoc) io.Reader {
	content := heredoc.Content

	if heredoc.ContentType == "heredoc" {
		// Split into lines to process
		lines := strings.Split(content, "\n")

		// Process lines
		var processedLines []string
		for _, line := range lines {
			// If StripTabs is true, strip leading tabs
			if heredoc.StripTabs {
				line = strings.TrimLeft(line, "\t")
			}
			processedLines = append(processedLines, line)
		}

		// Join back into a string
		content = strings.Join(processedLines, "\n")
	}

	// Create a reader for the content
	return bytes.NewReader([]byte(content))
}

// NewGUID generates a simple unique identifier
// In a real implementation, you'd use a proper UUID library
func NewGUID() string {
	// This is a simplified version; in practice use a proper UUID library
	return time.Now().Format("20060102150405")
}
