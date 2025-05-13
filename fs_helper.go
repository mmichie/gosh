package gosh

import (
	"fmt"
	"os"
	"path/filepath"
)

// PrepareFileForRedirection prepares a file for redirection, ensuring parent directories exist
func PrepareFileForRedirection(filename string, mode string) (*os.File, error) {
	// Get absolute path if needed
	var path string
	if !filepath.IsAbs(filename) {
		gs := GetGlobalState()
		path = filepath.Join(gs.GetCWD(), filename)
	} else {
		path = filename
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creating directories: %v", err)
	}

	// Open the file based on the redirection mode
	var file *os.File
	var err error

	// Handle different redirection modes
	switch {
	// Standard output truncate
	case mode == ">" || mode == "2>" || mode == "&>":
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)

	// Standard output append
	case mode == ">>" || mode == "2>>" || mode == ">>&":
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	// Unsupported modes
	default:
		return nil, fmt.Errorf("unsupported redirection mode: %s", mode)
	}

	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}

	return file, nil
}
