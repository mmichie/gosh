package gosh

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExpandWildcards expands wildcard patterns in command arguments
// Supports:
// - * (matches any number of any characters)
// - ? (matches exactly one character)
// - [...] (matches any one character in the brackets)
// - [!...] or [^...] (matches any one character not in the brackets)
// - {alt1,alt2,...} (matches any of the comma-separated alternatives)
// - ~ (expands to the user's home directory at the start of a path)
func ExpandWildcards(args []string) []string {
	var expandedArgs []string

	for _, arg := range args {
		// Quick check if expansion is needed
		if !strings.ContainsAny(arg, "*?[{~") {
			expandedArgs = append(expandedArgs, arg)
			continue
		}

		// Handle home directory expansion
		if strings.HasPrefix(arg, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				arg = strings.Replace(arg, "~", home, 1)
			}
		}

		// Handle brace expansion first if present (before globbing)
		if strings.ContainsAny(arg, "{") {
			braceExpanded := expandBraces(arg)
			for _, expandedArg := range braceExpanded {
				matches := expandPattern(expandedArg)
				if len(matches) == 0 {
					// If no matches, use the original argument
					expandedArgs = append(expandedArgs, expandedArg)
				} else {
					expandedArgs = append(expandedArgs, matches...)
				}
			}
		} else {
			// Regular glob expansion
			matches := expandPattern(arg)
			if len(matches) == 0 {
				// If no matches, use the original argument
				expandedArgs = append(expandedArgs, arg)
			} else {
				expandedArgs = append(expandedArgs, matches...)
			}
		}
	}

	return expandedArgs
}

// expandPattern expands a single glob pattern
func expandPattern(pattern string) []string {
	// Check if pattern contains character classes
	if strings.Contains(pattern, "[") && strings.Contains(pattern, "]") {
		// If using character classes, use filepath.Match with a walk function
		// since filepath.Glob doesn't fully support character classes
		return expandAdvancedPattern(pattern)
	}

	// For simple patterns, use built-in filepath.Glob
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return []string{}
	}
	return matches
}

// expandAdvancedPattern handles patterns with character classes [...]
func expandAdvancedPattern(pattern string) []string {
	// Convert glob pattern to base dir and pattern
	baseDir := "."
	patternOnly := pattern

	// Extract the directory part to start our walk from
	lastSlash := strings.LastIndex(pattern, "/")
	if lastSlash >= 0 {
		baseDir = pattern[:lastSlash]
		if baseDir == "" {
			baseDir = "/"
		}
		patternOnly = pattern[lastSlash+1:]
	}

	// Check if baseDir exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return []string{}
	}

	var matches []string
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		// Skip errors and don't walk into directories we don't need
		if err != nil || (d.IsDir() && path != baseDir && !strings.HasPrefix(pattern, path)) {
			return filepath.SkipDir
		}

		// Skip directories, we only want to match files
		if d.IsDir() && path != baseDir {
			return nil
		}

		// Check if the filename matches our pattern
		filename := filepath.Base(path)
		match, err := filepath.Match(patternOnly, filename)
		if err == nil && match {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return []string{}
	}

	return matches
}

// expandBraces expands {alt1,alt2,...} patterns
func expandBraces(pattern string) []string {
	// Check if there are even brace patterns
	if !strings.Contains(pattern, "{") || !strings.Contains(pattern, "}") {
		return []string{pattern}
	}

	// Use regex to find brace patterns
	r := regexp.MustCompile(`\{([^{}]*)\}`)
	matches := r.FindStringSubmatch(pattern)

	if len(matches) < 2 {
		return []string{pattern}
	}

	// Get the part to expand
	braceContent := matches[1]
	prefix := pattern[:strings.Index(pattern, "{")]
	suffix := pattern[strings.Index(pattern, "}")+1:]

	// Split alternatives
	alternatives := strings.Split(braceContent, ",")

	// Expand each alternative
	var results []string
	for _, alt := range alternatives {
		expanded := prefix + alt + suffix

		// Recursively expand if there are nested braces
		if strings.Contains(expanded, "{") {
			nestedExpanded := expandBraces(expanded)
			results = append(results, nestedExpanded...)
		} else {
			results = append(results, expanded)
		}
	}

	return results
}
