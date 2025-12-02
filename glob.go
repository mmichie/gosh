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
	// Check if pattern contains recursive glob (**)
	if strings.Contains(pattern, "**") {
		return expandRecursivePattern(pattern)
	}

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

// expandRecursivePattern handles patterns containing ** (recursive glob)
// Examples:
//   - **/*.go     matches any .go file at any depth
//   - src/**/*.go matches any .go file at any depth under src
//   - **/test     matches any file named "test" at any depth
//   - foo/**/bar  matches foo/bar, foo/x/bar, foo/x/y/bar, etc.
func expandRecursivePattern(pattern string) []string {
	// Find the position of **
	doubleStarIdx := strings.Index(pattern, "**")
	if doubleStarIdx == -1 {
		// No ** found, fall back to regular glob
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return []string{}
		}
		return matches
	}

	// Split pattern into prefix (before **) and suffix (after **)
	prefix := pattern[:doubleStarIdx]
	suffix := pattern[doubleStarIdx+2:]

	// Determine the base directory to start walking
	baseDir := "."
	if prefix != "" {
		// Remove trailing slash from prefix for baseDir
		baseDir = strings.TrimSuffix(prefix, "/")
		if baseDir == "" {
			baseDir = "/"
		}
	}

	// Check if baseDir itself contains wildcards
	if strings.ContainsAny(baseDir, "*?[{") {
		// Expand the base directory first
		baseDirs, err := filepath.Glob(baseDir)
		if err != nil || len(baseDirs) == 0 {
			return []string{}
		}
		var allMatches []string
		for _, dir := range baseDirs {
			// Reconstruct pattern with this expanded base
			newPattern := dir + "/" + "**" + suffix
			allMatches = append(allMatches, expandRecursivePattern(newPattern)...)
		}
		return allMatches
	}

	// Check if baseDir exists
	info, err := os.Stat(baseDir)
	if err != nil || !info.IsDir() {
		return []string{}
	}

	// Remove leading slash from suffix for matching
	suffixPattern := strings.TrimPrefix(suffix, "/")

	// Check if suffix contains another ** (nested recursive)
	if strings.Contains(suffixPattern, "**") {
		// Handle nested ** patterns by expanding the first ** and recursing
		return expandNestedRecursivePattern(baseDir, suffixPattern)
	}

	var matches []string
	err = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories (starting with .) unless explicitly requested
		if d.IsDir() && d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		// If there's no suffix pattern, ** matches all files
		if suffixPattern == "" {
			if !d.IsDir() {
				matches = append(matches, path)
			}
			return nil
		}

		// Check if the suffix contains directory separators
		if strings.Contains(suffixPattern, "/") {
			// Complex pattern with directories - need to match relative path
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return nil
			}

			// Try matching the relative path against the suffix pattern
			if matchRecursiveSuffix(relPath, suffixPattern, d.IsDir()) {
				matches = append(matches, path)
			}
		} else {
			// Simple pattern - just match the filename
			if !d.IsDir() {
				matched, err := filepath.Match(suffixPattern, d.Name())
				if err == nil && matched {
					matches = append(matches, path)
				}
			}
		}

		return nil
	})

	if err != nil {
		return []string{}
	}

	return matches
}

// expandNestedRecursivePattern handles patterns with multiple ** sequences
func expandNestedRecursivePattern(baseDir, suffixPattern string) []string {
	// Find the first ** in the suffix
	doubleStarIdx := strings.Index(suffixPattern, "**")
	if doubleStarIdx == -1 {
		// No more **, expand normally
		return expandRecursivePattern(baseDir + "/" + suffixPattern)
	}

	// Get the part before the first **
	beforeStar := suffixPattern[:doubleStarIdx]
	afterStar := suffixPattern[doubleStarIdx+2:]

	var matches []string
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories
		if d.IsDir() && d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		if d.IsDir() {
			// For each directory, try to match the remaining pattern
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return nil
			}

			// Check if the path matches the "before" part
			if beforeStar == "" || beforeStar == "/" || strings.HasSuffix(relPath+"/", beforeStar) || relPath == "." {
				// Recursively expand with the remaining pattern
				subMatches := expandRecursivePattern(path + "/**" + afterStar)
				matches = append(matches, subMatches...)
			}
		}

		return nil
	})

	if err != nil {
		return []string{}
	}

	return matches
}

// matchRecursiveSuffix checks if a path matches a pattern that may contain multiple path segments
func matchRecursiveSuffix(path, pattern string, isDir bool) bool {
	// Split both path and pattern by separator
	pathParts := strings.Split(path, string(filepath.Separator))
	patternParts := strings.Split(pattern, "/")

	// Remove empty parts
	pathParts = removeEmpty(pathParts)
	patternParts = removeEmpty(patternParts)

	if len(patternParts) == 0 {
		return !isDir
	}

	// For files, we need exact match in length or path must be longer
	if !isDir {
		// Try to match the last N parts of the path to the pattern
		if len(pathParts) < len(patternParts) {
			return false
		}

		// Match from the end
		pathOffset := len(pathParts) - len(patternParts)
		for i, patternPart := range patternParts {
			matched, err := filepath.Match(patternPart, pathParts[pathOffset+i])
			if err != nil || !matched {
				return false
			}
		}
		return true
	}

	return false
}

// removeEmpty filters out empty strings from a slice
func removeEmpty(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
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
