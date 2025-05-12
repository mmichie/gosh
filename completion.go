package gosh

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CommandInfo stores information about a command
type CommandInfo struct {
	Name      string
	Path      string
	Frequency int // For tracking usage frequency
}

// Completer provides tab completion for the shell
type Completer struct {
	builtins     map[string]func(cmd *Command) error
	commandMap   map[string]*CommandInfo  // Map for quick lookup
	commandList  []*CommandInfo           // Sorted list for completion
	dirCache     map[string]time.Time     // Cache of directory modification times
	fileCache    map[string][]os.DirEntry // Cache of directory contents
	commonCmds   map[string]bool          // Set of common commands to prioritize
	commandsLock sync.RWMutex
	cacheLock    sync.RWMutex
	loaded       chan struct{} // Signal when initial loading is done
	ready        bool          // Fast path for checking if loaded
}

// NewCompleter creates a new tab completion handler
func NewCompleter(builtins map[string]func(cmd *Command) error) *Completer {
	c := &Completer{
		builtins:    builtins,
		commandMap:  make(map[string]*CommandInfo),
		commandList: make([]*CommandInfo, 0, 100),
		dirCache:    make(map[string]time.Time),
		fileCache:   make(map[string][]os.DirEntry),
		commonCmds:  initCommonCommands(),
		loaded:      make(chan struct{}),
		ready:       false,
	}

	// Add builtins to command map first (they're always available)
	for cmd := range builtins {
		c.commandMap[cmd] = &CommandInfo{
			Name:      cmd,
			Path:      "builtin",
			Frequency: 10, // Prioritize builtins
		}
	}

	// Start fast initialization of common commands
	go c.fastInit()

	// Start background indexing of all commands
	go c.backgroundIndexing()

	return c
}

// initCommonCommands returns a set of common commands to prioritize loading
func initCommonCommands() map[string]bool {
	common := map[string]bool{
		"ls": true, "cd": true, "grep": true, "cat": true,
		"git": true, "find": true, "echo": true, "mv": true,
		"cp": true, "rm": true, "mkdir": true, "touch": true,
		"vim": true, "nano": true, "python": true, "python3": true,
		"go": true, "make": true, "gcc": true, "node": true,
		"npm": true, "docker": true, "ssh": true, "curl": true,
	}
	return common
}

// fastInit quickly loads common commands for immediate completion
func (c *Completer) fastInit() {
	// Get PATH directories
	pathDirs := filepath.SplitList(os.Getenv("PATH"))

	// First pass: only look for common commands
	for _, dir := range pathDirs {
		// Skip directories that don't exist or can't be read
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		// Cache this directory's contents
		c.cacheLock.Lock()
		c.dirCache[dir] = time.Now()
		c.fileCache[dir] = files
		c.cacheLock.Unlock()

		// Process only common commands first
		var batch []*CommandInfo
		for _, file := range files {
			name := file.Name()
			// Only add common commands in this fast pass
			if file.Type().IsRegular() && file.Type().Perm()&0111 != 0 && c.commonCmds[name] {
				if _, exists := c.commandMap[name]; !exists {
					cmdInfo := &CommandInfo{
						Name:      name,
						Path:      filepath.Join(dir, name),
						Frequency: 5, // Common commands get priority
					}
					batch = append(batch, cmdInfo)
				}
			}
		}

		// Batch update the commands list
		if len(batch) > 0 {
			c.commandsLock.Lock()
			for _, cmd := range batch {
				c.commandMap[cmd.Name] = cmd
			}
			// We'll rebuild the sorted list at the end
			c.commandsLock.Unlock()
		}
	}

	// Build the initial sorted command list
	c.rebuildCommandList()

	// Mark as ready for basic completion
	c.ready = true

	// Signal that the fast initialization is complete
	select {
	case <-c.loaded:
		// Already closed, do nothing
	default:
		close(c.loaded)
	}
}

// backgroundIndexing continuously indexes PATH directories for commands
func (c *Completer) backgroundIndexing() {
	// Wait a bit for the fast init to finish
	time.Sleep(100 * time.Millisecond)

	// Get PATH directories
	pathDirs := filepath.SplitList(os.Getenv("PATH"))

	// Process all commands in all directories
	for _, dir := range pathDirs {
		var files []os.DirEntry
		var err error

		// Check if we already cached this directory's contents
		c.cacheLock.RLock()
		cachedFiles, exists := c.fileCache[dir]
		c.cacheLock.RUnlock()

		if exists {
			files = cachedFiles
		} else {
			// Read directory if not cached
			files, err = os.ReadDir(dir)
			if err != nil {
				continue
			}

			// Cache the directory contents
			c.cacheLock.Lock()
			c.dirCache[dir] = time.Now()
			c.fileCache[dir] = files
			c.cacheLock.Unlock()
		}

		// Collect commands in batches to minimize lock contention
		var batch []*CommandInfo
		for _, file := range files {
			name := file.Name()
			if file.Type().IsRegular() && file.Type().Perm()&0111 != 0 {
				c.commandsLock.RLock()
				_, exists := c.commandMap[name]
				c.commandsLock.RUnlock()

				if !exists {
					cmdInfo := &CommandInfo{
						Name:      name,
						Path:      filepath.Join(dir, name),
						Frequency: 1,
					}
					batch = append(batch, cmdInfo)
				}
			}
		}

		// Batch update the commands list
		if len(batch) > 0 {
			c.commandsLock.Lock()
			for _, cmd := range batch {
				c.commandMap[cmd.Name] = cmd
			}
			c.commandsLock.Unlock()
		}

		// Don't hog the CPU
		time.Sleep(5 * time.Millisecond)
	}

	// Rebuild the sorted command list after all commands are indexed
	c.rebuildCommandList()

	// Start monitor routine to keep commands updated
	go c.monitorPathChanges()
}

// rebuildCommandList rebuilds the sorted list of commands for completion
func (c *Completer) rebuildCommandList() {
	c.commandsLock.Lock()
	defer c.commandsLock.Unlock()

	// Rebuild the list for sorted access
	c.commandList = make([]*CommandInfo, 0, len(c.commandMap))
	for _, cmd := range c.commandMap {
		c.commandList = append(c.commandList, cmd)
	}

	// Sort by frequency (higher first) then by name
	sort.Slice(c.commandList, func(i, j int) bool {
		if c.commandList[i].Frequency != c.commandList[j].Frequency {
			return c.commandList[i].Frequency > c.commandList[j].Frequency
		}
		return c.commandList[i].Name < c.commandList[j].Name
	})
}

// monitorPathChanges periodically checks for changes to PATH directories
func (c *Completer) monitorPathChanges() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pathDirs := filepath.SplitList(os.Getenv("PATH"))
		for _, dir := range pathDirs {
			// Skip directories that don't exist or can't be read
			info, err := os.Stat(dir)
			if err != nil {
				continue
			}

			// Check if directory was modified since last check
			c.cacheLock.RLock()
			lastMod, exists := c.dirCache[dir]
			c.cacheLock.RUnlock()

			if !exists || info.ModTime().After(lastMod) {
				// Update the cache with new directory contents
				files, err := os.ReadDir(dir)
				if err != nil {
					continue
				}

				// Update cache
				c.cacheLock.Lock()
				c.dirCache[dir] = time.Now()
				c.fileCache[dir] = files
				c.cacheLock.Unlock()

				// Process new executable files
				var newCommands []*CommandInfo
				for _, file := range files {
					name := file.Name()
					if file.Type().IsRegular() && file.Type().Perm()&0111 != 0 {
						c.commandsLock.RLock()
						_, exists := c.commandMap[name]
						c.commandsLock.RUnlock()

						if !exists {
							cmdInfo := &CommandInfo{
								Name:      name,
								Path:      filepath.Join(dir, name),
								Frequency: 1,
							}
							newCommands = append(newCommands, cmdInfo)
						}
					}
				}

				// Batch update
				if len(newCommands) > 0 {
					c.commandsLock.Lock()
					for _, cmd := range newCommands {
						c.commandMap[cmd.Name] = cmd
					}
					c.commandsLock.Unlock()
					c.rebuildCommandList()
				}
			}
		}
	}
}

// Do implements the completion interface for readline
func (c *Completer) Do(line []rune, pos int) (newLine [][]rune, length int) {
	// Convert line to string up to cursor position
	lineStr := string(line[:pos])
	parts := strings.Fields(lineStr)

	// If line is empty or ends with a space, complete commands
	if len(parts) == 0 || (len(lineStr) > 0 && lineStr[len(lineStr)-1] == ' ') {
		return c.completeCommands("", false)
	}

	// Get the last part of the command line
	lastPart := parts[len(parts)-1]

	// Check if we're after a control operator and should complete commands
	if lastPart == "&&" || lastPart == "||" || lastPart == ";" || lastPart == "|" {
		return c.completeCommands("", false)
	}

	// If we're completing the first word, it's a command
	if len(parts) == 1 && !strings.Contains(lineStr, " ") {
		return c.completeCommands(lastPart, true)
	}

	// Otherwise, complete filenames for arguments
	return c.completeFilenames(lineStr)
}

// completeCommands returns command completions matching the prefix
func (c *Completer) completeCommands(prefix string, partial bool) (newLine [][]rune, length int) {
	// Wait until at least fast initialization is done
	if !c.ready {
		select {
		case <-c.loaded:
			// Initialization complete
		case <-time.After(50 * time.Millisecond):
			// Timeout - return empty if initialization is taking too long
			return nil, len(prefix)
		}
	}

	c.commandsLock.RLock()
	defer c.commandsLock.RUnlock()

	// Use the sorted commandList for predictable, frequency-based completion
	completions := make([][]rune, 0, 10)
	for _, cmdInfo := range c.commandList {
		if strings.HasPrefix(cmdInfo.Name, prefix) {
			suffix := cmdInfo.Name[len(prefix):]
			completions = append(completions, []rune(suffix))

			// Limit to reasonable number of suggestions
			if len(completions) >= 50 {
				break
			}
		}
	}

	// Add a space if there's only one completion and we're not in a partial word
	if len(completions) == 1 && !partial {
		completions[0] = append(completions[0], ' ')
	}

	return completions, len(prefix)
}

// completeFilenames handles file and directory name completion
func (c *Completer) completeFilenames(line string) (newLine [][]rune, length int) {
	// Extract the last word from the line
	lastSpaceIdx := strings.LastIndex(line, " ")
	lastWord := line[lastSpaceIdx+1:]

	// Handle empty path
	if lastWord == "" {
		entries, err := os.ReadDir(".")
		if err != nil {
			return nil, 0
		}

		completions := make([][]rune, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			// Skip hidden files unless explicitly looking for them
			if strings.HasPrefix(name, ".") && !strings.HasPrefix(lastWord, ".") {
				continue
			}

			if entry.IsDir() {
				completions = append(completions, []rune(name+"/"))
			} else {
				completions = append(completions, []rune(name))
			}
		}
		return completions, 0
	}

	// Handle tilde expansion
	if strings.HasPrefix(lastWord, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			lastWord = filepath.Join(home, lastWord[2:])
		}
	}

	// Get directory and prefix
	dir := filepath.Dir(lastWord)
	prefix := filepath.Base(lastWord)

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
		return nil, len(prefix)
	}

	// Generate completions
	completions := make([][]rune, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files unless explicitly looking for them
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		if strings.HasPrefix(name, prefix) {
			suffix := name[len(prefix):]
			if entry.IsDir() {
				completions = append(completions, []rune(suffix+"/"))
			} else {
				completions = append(completions, []rune(suffix))
			}
		}
	}

	return completions, len(prefix)
}

// RecordCommandUsage increases the frequency count for commands
// This should be called when a command is successfully executed
func (c *Completer) RecordCommandUsage(cmdName string) {
	c.commandsLock.Lock()
	defer c.commandsLock.Unlock()

	if cmdInfo, exists := c.commandMap[cmdName]; exists {
		cmdInfo.Frequency++

		// Periodically rebuild the sorted list when frequency changes
		if cmdInfo.Frequency%5 == 0 {
			go c.rebuildCommandList() // Rebuild in background
		}
	}
}
