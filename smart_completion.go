package gosh

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// CompletionCandidate represents a single completion suggestion with its score
type CompletionCandidate struct {
	Text  string
	Score int
	IsDir bool
}

// CompletionCycle stores the state for cycling through completions
type CompletionCycle struct {
	Command    string
	Prefix     string
	Candidates []CompletionCandidate
	Index      int
}

// SmartCompleter extends the basic Completer with argument history and cycling
type SmartCompleter struct {
	*Completer
	ArgHistory    *ArgHistoryDB
	currentCycle  *CompletionCycle
	cycleLock     sync.Mutex
	lastLine      string // Store the last line to detect when input changes
	lastPos       int    // Store the last position to detect when input changes
	maxCandidates int
}

// NewSmartCompleter creates a new smart completer with argument history
func NewSmartCompleter(builtins map[string]func(cmd *Command) error, argHistory *ArgHistoryDB) *SmartCompleter {
	return &SmartCompleter{
		Completer:     NewCompleter(builtins),
		ArgHistory:    argHistory,
		currentCycle:  nil,
		lastLine:      "",
		lastPos:       0,
		maxCandidates: 50,
	}
}

// Do implements the completion interface for readline
func (c *SmartCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	// Convert line to string up to cursor position
	lineStr := string(line[:pos])
	parts := strings.Fields(lineStr)

	debug := os.Getenv("GOSH_ARG_DEBUG") != ""
	sameInput := lineStr == c.lastLine && pos == c.lastPos

	// Store current line and position for next time
	c.lastLine = lineStr
	c.lastPos = pos

	if debug {
		fmt.Fprintf(os.Stderr, "Do() called with lineStr='%s', pos=%d, sameInput=%v\n",
			lineStr, pos, sameInput)
	}

	// If line is truly empty, complete commands normally
	if len(parts) == 0 {
		// Reset cycle when starting a new completion
		c.resetCycle()
		return c.Completer.completeCommands("", false)
	}

	// If we have a command followed by a space, this is argument completion territory
	if len(parts) > 0 && len(lineStr) > 0 && lineStr[len(lineStr)-1] == ' ' {
		// We have a command and should suggest arguments based on history
		command := parts[0]
		return c.startNewCompletionCycle(command, "")
	}

	// Get the last part of the command line
	lastPart := parts[len(parts)-1]

	// Check if we're after a control operator and should complete commands
	if lastPart == "&&" || lastPart == "||" || lastPart == ";" || lastPart == "|" {
		// Reset cycle when starting a new completion
		c.resetCycle()
		return c.Completer.completeCommands("", false)
	}

	// If we're completing the first word, it's a command
	if len(parts) == 1 && !strings.Contains(strings.TrimRight(lineStr, " "), " ") {
		// Reset cycle when starting a new completion
		c.resetCycle()
		return c.Completer.completeCommands(lastPart, true)
	}

	// This case is now handled by the condition above

	// Otherwise, we're completing an argument
	command := parts[0]
	argPrefix := lastPart

	// Check if we're in a completion cycle for the same command and prefix
	c.cycleLock.Lock()
	defer c.cycleLock.Unlock()

	if debug {
		fmt.Fprintf(os.Stderr, "Checking for cycle continuation: command='%s', prefix='%s', sameInput=%v\n",
			command, argPrefix, sameInput)
		if c.currentCycle != nil {
			fmt.Fprintf(os.Stderr, "Current cycle: command='%s', prefix='%s', candidateCount=%d, index=%d\n",
				c.currentCycle.Command, c.currentCycle.Prefix,
				len(c.currentCycle.Candidates), c.currentCycle.Index)
		} else {
			fmt.Fprintf(os.Stderr, "No current cycle exists\n")
		}
	}

	if sameInput && c.currentCycle != nil &&
		c.currentCycle.Command == command &&
		c.currentCycle.Prefix == argPrefix &&
		len(c.currentCycle.Candidates) > 0 {
		// We're continuing a cycle, move to next candidate
		c.currentCycle.Index = (c.currentCycle.Index + 1) % len(c.currentCycle.Candidates)
		candidate := c.currentCycle.Candidates[c.currentCycle.Index]
		completion := candidate.Text[len(argPrefix):]

		// For directories, add trailing slash
		if candidate.IsDir {
			completion += "/"
		}

		if debug {
			fmt.Fprintf(os.Stderr, "Continuing cycle, suggesting: '%s' (candidate %d of %d)\n",
				candidate.Text, c.currentCycle.Index+1, len(c.currentCycle.Candidates))
		}

		return [][]rune{[]rune(completion)}, len(argPrefix)
	}

	// Start a new completion cycle
	return c.startNewCompletionCycle(command, argPrefix)
}

// startNewCompletionCycle begins a new completion cycle with merged candidates
func (c *SmartCompleter) startNewCompletionCycle(command, prefix string) ([][]rune, int) {
	debug := os.Getenv("GOSH_ARG_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "Starting new completion cycle: command='%s', prefix='%s'\n", command, prefix)
	}

	// Get history suggestions
	var historySuggestions []string
	if c.ArgHistory != nil {
		historySuggestions = c.ArgHistory.GetArgumentsByFrequency(command, prefix)
		if debug {
			fmt.Fprintf(os.Stderr, "Got %d history suggestions for command '%s'\n", len(historySuggestions), command)
		}
	}

	// Get file system suggestions
	fileCompletions := c.getFileCompletions(prefix)

	// Combine both sets of suggestions with proper scoring
	var candidates []CompletionCandidate
	seenItems := make(map[string]bool)

	// First add history items with their scores from frequency
	for i, arg := range historySuggestions {
		if _, exists := seenItems[arg]; !exists {
			// Higher importance for earlier items
			score := 1000 - i
			if score < 1 {
				score = 1
			}

			// Check if it's also a directory
			isDir := false
			for _, fc := range fileCompletions {
				if fc.Text == arg && fc.IsDir {
					isDir = true
					break
				}
			}

			candidates = append(candidates, CompletionCandidate{
				Text:  arg,
				Score: score,
				IsDir: isDir,
			})
			seenItems[arg] = true
		}
	}

	// Then add filesystem items not already added
	for _, fc := range fileCompletions {
		if _, exists := seenItems[fc.Text]; !exists {
			candidates = append(candidates, fc)
			seenItems[fc.Text] = true
		}
	}

	// Sort by score (highest first)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Limit to max candidates
	if len(candidates) > c.maxCandidates {
		candidates = candidates[:c.maxCandidates]
	}

	// Create new cycle
	c.currentCycle = &CompletionCycle{
		Command:    command,
		Prefix:     prefix,
		Candidates: candidates,
		Index:      0,
	}

	// If no candidates, fall back to filesystem completion
	if len(candidates) == 0 {
		if debug {
			fmt.Fprintf(os.Stderr, "No candidates found from history or filesystem, returning empty\n")
		}
		return nil, len(prefix)
	}

	// Return the first candidate
	candidate := candidates[0]
	completion := candidate.Text[len(prefix):]

	// For directories, add trailing slash
	if candidate.IsDir {
		completion += "/"
	}

	if debug {
		fmt.Fprintf(os.Stderr, "Returning first candidate: '%s'\n", candidate.Text)
	}

	return [][]rune{[]rune(completion)}, len(prefix)
}

// getFileCompletions returns file system completions with scores
func (c *SmartCompleter) getFileCompletions(prefix string) []CompletionCandidate {
	var candidates []CompletionCandidate

	// Handle empty path
	if prefix == "" {
		entries, err := os.ReadDir(".")
		if err != nil {
			return nil
		}

		for _, entry := range entries {
			name := entry.Name()
			// Skip hidden files unless explicitly looking for them
			if strings.HasPrefix(name, ".") {
				continue
			}

			candidates = append(candidates, CompletionCandidate{
				Text:  name,
				Score: 1, // Base score for filesystem entries
				IsDir: entry.IsDir(),
			})
		}
		return candidates
	}

	// Handle tilde expansion
	if strings.HasPrefix(prefix, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			prefix = filepath.Join(home, prefix[2:])
		}
	}

	// Get directory and base to match
	dir := filepath.Dir(prefix)
	base := filepath.Base(prefix)

	// Handle relative paths
	if !filepath.IsAbs(dir) && dir != "." {
		cwd, err := os.Getwd()
		if err == nil && dir != "." {
			dir = filepath.Join(cwd, dir)
		}
	}

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Add matching entries
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files unless explicitly looking for them
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}

		if strings.HasPrefix(name, base) {
			// For the filesystem completion, use the original prefix and not the expanded one
			// to keep the completion looking natural
			fullPath := filepath.Join(dir, name)
			relativePath := getRelativePath(prefix, fullPath)

			candidates = append(candidates, CompletionCandidate{
				Text:  relativePath,
				Score: 1, // Base score for filesystem entries
				IsDir: entry.IsDir(),
			})
		}
	}

	return candidates
}

// getRelativePath gets a completion-friendly version of the path
func getRelativePath(prefix, fullPath string) string {
	// If the prefix starts with ~/, preserve that in the completion
	if strings.HasPrefix(prefix, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && strings.HasPrefix(fullPath, home) {
			return "~" + fullPath[len(home):]
		}
	}

	// If we're using a relative path in the prefix, return a relative result
	if !filepath.IsAbs(prefix) {
		cwd, err := os.Getwd()
		if err == nil && strings.HasPrefix(fullPath, cwd) {
			rel, err := filepath.Rel(cwd, fullPath)
			if err == nil {
				return rel
			}
		}
	}

	return fullPath
}

// resetCycle clears the current completion cycle
func (c *SmartCompleter) resetCycle() {
	c.cycleLock.Lock()
	defer c.cycleLock.Unlock()
	c.currentCycle = nil
}
