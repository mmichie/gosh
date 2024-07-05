package gosh

import (
	"fmt"
	"strings"
	"sync"
)

var (
	aliases = make(map[string]string)
	aliasMu sync.RWMutex
)

func SetAlias(name, command string) {
	aliasMu.Lock()
	defer aliasMu.Unlock()
	aliases[name] = command
}

func GetAlias(name string) (string, bool) {
	aliasMu.RLock()
	defer aliasMu.RUnlock()
	command, exists := aliases[name]
	return command, exists
}

func RemoveAlias(name string) {
	aliasMu.Lock()
	defer aliasMu.Unlock()
	delete(aliases, name)
}

func ListAliases() []string {
	aliasMu.RLock()
	defer aliasMu.RUnlock()
	var result []string
	for name, command := range aliases {
		result = append(result, fmt.Sprintf("%s='%s'", name, command))
	}
	return result
}

func ExpandAlias(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command
	}

	expanded, exists := GetAlias(parts[0])
	if !exists {
		return command
	}

	if len(parts) > 1 {
		expanded += " " + strings.Join(parts[1:], " ")
	}
	return expanded
}
