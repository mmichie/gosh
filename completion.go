package gosh

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Completer struct {
	builtins     map[string]func(cmd *Command) error
	commands     []string
	commandsLock sync.RWMutex
	loaded       chan struct{}
}

func NewCompleter(builtins map[string]func(cmd *Command) error) *Completer {
	c := &Completer{
		builtins: builtins,
		commands: make([]string, 0, len(builtins)),
		loaded:   make(chan struct{}),
	}
	for cmd := range builtins {
		c.commands = append(c.commands, cmd)
	}
	go c.loadCommands()
	return c
}

func (c *Completer) loadCommands() {
	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	for _, dir := range pathDirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.Type().IsRegular() && file.Type().Perm()&0111 != 0 {
				c.commandsLock.Lock()
				c.commands = append(c.commands, file.Name())
				c.commandsLock.Unlock()
			}
		}
	}
	close(c.loaded)
}

func (c *Completer) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line[:pos])
	parts := strings.Fields(lineStr)

	if len(parts) == 0 {
		return c.completeCommands("", false)
	}

	lastPart := parts[len(parts)-1]
	if lastPart == "&&" {
		return c.completeCommands("", false)
	}
	// Complete filenames for arguments
	return c.completeFilenames(lineStr)
}

func (c *Completer) completeCommands(prefix string, partial bool) (newLine [][]rune, length int) {
	c.commandsLock.RLock()
	defer c.commandsLock.RUnlock()

	for _, cmd := range c.commands {
		if strings.HasPrefix(cmd, prefix) {
			newLine = append(newLine, []rune(cmd[len(prefix):]))
		}
	}

	if len(newLine) == 1 && !partial {
		newLine[0] = append(newLine[0], ' ')
	}

	return newLine, len(prefix)
}

func (c *Completer) completeFilenames(line string) (newLine [][]rune, length int) {
	lastWord := line[strings.LastIndex(line, " ")+1:]
	dir := filepath.Dir(lastWord)
	prefix := filepath.Base(lastWord)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, len(prefix)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			completion := name[len(prefix):]
			if entry.IsDir() {
				completion += "/"
			}
			newLine = append(newLine, []rune(completion))
		}
	}

	return newLine, len(prefix)
}
