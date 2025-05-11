package gosh

import (
	"os"
	"sync"
)

type GlobalState struct {
	CWD         string
	PreviousDir string
	mu          sync.RWMutex
}

var globalState *GlobalState
var once sync.Once

func GetGlobalState() *GlobalState {
	once.Do(func() {
		cwd, _ := os.Getwd()
		// Initialize with different PreviousDir to ensure cd - works properly
		prevDir := os.Getenv("HOME")
		if prevDir == "" {
			prevDir = cwd
		}
		globalState = &GlobalState{
			CWD:         cwd,
			PreviousDir: prevDir,
		}
	})
	return globalState
}

func (gs *GlobalState) UpdateCWD(newCWD string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.PreviousDir = gs.CWD
	gs.CWD = newCWD
}

func (gs *GlobalState) GetCWD() string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.CWD
}

func (gs *GlobalState) GetPreviousDir() string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.PreviousDir
}
