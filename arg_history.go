package gosh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ArgHistoryDB stores command argument usage frequencies
type ArgHistoryDB struct {
	// Map of command name to map of argument to usage count
	History     map[string]map[string]int
	historyLock sync.RWMutex
	filePath    string
	lastSave    time.Time
	dirty       bool
}

// NewArgHistoryDB creates a new argument history database
func NewArgHistoryDB(customPath string) (*ArgHistoryDB, error) {
	// Determine storage path
	var filePath string
	if customPath != "" {
		filePath = customPath
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		filePath = filepath.Join(home, ".gosh_arg_history")
	}

	db := &ArgHistoryDB{
		History:  make(map[string]map[string]int),
		filePath: filePath,
		lastSave: time.Now(),
		dirty:    false,
	}

	// Try to load existing history
	err := db.Load()
	if err != nil && !os.IsNotExist(err) {
		// Only return error if it's not just a missing file
		return nil, err
	}

	// Start background save routine
	go db.backgroundSave()

	return db, nil
}

// RecordArgUsage records usage of an argument with a command
func (db *ArgHistoryDB) RecordArgUsage(command string, args []string) {
	if len(args) == 0 {
		return
	}

	// Debug logging can be enabled with the environment variable
	debug := os.Getenv("GOSH_ARG_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "Recording args for command '%s': %v\n", command, args)
	}

	db.historyLock.Lock()
	defer db.historyLock.Unlock()

	// Create command entry if it doesn't exist
	if _, exists := db.History[command]; !exists {
		db.History[command] = make(map[string]int)
		if debug {
			fmt.Fprintf(os.Stderr, "Created new entry for command '%s'\n", command)
		}
	}

	// Increment count for each argument
	for _, arg := range args {
		// Include flags in history as they're often reused
		// If we don't want flags, uncomment this
		// if strings.HasPrefix(arg, "-") {
		//     continue
		// }
		db.History[command][arg]++

		if debug {
			fmt.Fprintf(os.Stderr, "Incremented usage count for '%s %s' to %d\n",
				command, arg, db.History[command][arg])
		}
	}

	db.dirty = true
}

// GetArgumentsByFrequency returns arguments for a command sorted by frequency
func (db *ArgHistoryDB) GetArgumentsByFrequency(command string, prefix string) []string {
	debug := os.Getenv("GOSH_ARG_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "Getting arguments for command '%s' with prefix '%s'\n", command, prefix)
	}

	db.historyLock.RLock()
	defer db.historyLock.RUnlock()

	// Get command entry
	cmdArgs, exists := db.History[command]
	if !exists {
		if debug {
			fmt.Fprintf(os.Stderr, "No history entry found for command '%s'\n", command)
		}
		return nil
	}

	if debug {
		fmt.Fprintf(os.Stderr, "Found %d argument entries for command '%s'\n", len(cmdArgs), command)
	}

	// Collect matching arguments
	type argFreq struct {
		arg   string
		count int
	}
	var matches []argFreq

	for arg, count := range cmdArgs {
		if strings.HasPrefix(arg, prefix) {
			matches = append(matches, argFreq{arg, count})
			if debug {
				fmt.Fprintf(os.Stderr, "Matched argument '%s' with count %d\n", arg, count)
			}
		}
	}

	// Sort by frequency (highest first)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].count != matches[j].count {
			return matches[i].count > matches[j].count
		}
		return matches[i].arg < matches[j].arg // Alphabetical as tiebreaker
	})

	// Convert to simple string slice
	result := make([]string, len(matches))
	for i, match := range matches {
		result[i] = match.arg
	}

	if debug && len(result) > 0 {
		fmt.Fprintf(os.Stderr, "Returning %d arguments, top one is '%s'\n", len(result), result[0])
	}

	return result
}

// RemoveEntry removes a command entry from the history
func (db *ArgHistoryDB) RemoveEntry(command string) {
	db.historyLock.Lock()
	defer db.historyLock.Unlock()

	delete(db.History, command)
	db.dirty = true
}

// ClearAllHistory clears all history
func (db *ArgHistoryDB) ClearAllHistory() {
	db.historyLock.Lock()
	defer db.historyLock.Unlock()

	db.History = make(map[string]map[string]int)
	db.dirty = true
}

// Trim reduces the database size by removing least-used arguments
func (db *ArgHistoryDB) Trim(maxArgsPerCommand int) {
	if maxArgsPerCommand <= 0 {
		return
	}

	db.historyLock.Lock()
	defer db.historyLock.Unlock()

	for cmd, args := range db.History {
		if len(args) <= maxArgsPerCommand {
			continue
		}

		// Sort arguments by frequency
		type argFreq struct {
			arg   string
			count int
		}
		sorted := make([]argFreq, 0, len(args))
		for arg, count := range args {
			sorted = append(sorted, argFreq{arg, count})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].count > sorted[j].count
		})

		// Keep only the most frequent args
		newArgs := make(map[string]int)
		for i := 0; i < maxArgsPerCommand && i < len(sorted); i++ {
			newArgs[sorted[i].arg] = sorted[i].count
		}
		db.History[cmd] = newArgs
	}

	db.dirty = true
}

// ApplyDecay applies a decay factor to all counts to phase out old arguments
func (db *ArgHistoryDB) ApplyDecay(factor float64) {
	if factor <= 0 || factor >= 1 {
		return // Invalid factor
	}

	db.historyLock.Lock()
	defer db.historyLock.Unlock()

	for cmd, args := range db.History {
		for arg, count := range args {
			newCount := int(float64(count) * factor)
			if newCount < 1 {
				delete(args, arg)
			} else {
				args[arg] = newCount
			}
		}
		// Remove command if all arguments were deleted
		if len(args) == 0 {
			delete(db.History, cmd)
		}
	}

	db.dirty = true
}

// Save persists the history database to disk
func (db *ArgHistoryDB) Save() error {
	db.historyLock.RLock()
	defer db.historyLock.RUnlock()

	// Marshal to JSON
	data, err := json.MarshalIndent(db.History, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(db.filePath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	// Write to file
	tempFile := db.filePath + ".tmp"
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		return err
	}

	// Rename for atomic update
	err = os.Rename(tempFile, db.filePath)
	if err != nil {
		os.Remove(tempFile) // Clean up on error
		return err
	}

	db.lastSave = time.Now()
	db.dirty = false
	return nil
}

// Load loads the history database from disk
func (db *ArgHistoryDB) Load() error {
	data, err := os.ReadFile(db.filePath)
	if err != nil {
		return err
	}

	db.historyLock.Lock()
	defer db.historyLock.Unlock()

	return json.Unmarshal(data, &db.History)
}

// backgroundSave periodically saves the history database if it's dirty
func (db *ArgHistoryDB) backgroundSave() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if !db.dirty {
			continue
		}

		err := db.Save()
		if err != nil {
			// Log error but continue
			// In a real implementation, you might want proper error handling
		}
	}
}
