package gosh

import (
	"os"
	"path/filepath"
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
		cwd, err := os.Getwd()
		if err != nil {
			// Default to home directory if we can't get current directory
			cwd = os.Getenv("HOME")
			if cwd == "" {
				cwd = "/"
			}
		}

		// For the previous directory, try OLDPWD env var first
		prevDir := os.Getenv("OLDPWD")
		if prevDir == "" {
			// If OLDPWD not set, use HOME as fallback
			prevDir = os.Getenv("HOME")
			if prevDir == "" || prevDir == cwd {
				// If HOME not set or same as CWD, use parent directory
				prevDir = filepath.Dir(cwd)
				if prevDir == cwd { // Root directory case
					prevDir = cwd
				}
			}
		}

		// Initialize the global state
		globalState = &GlobalState{
			CWD:         cwd,
			PreviousDir: prevDir,
		}

		// Also ensure environment variables are set
		os.Setenv("PWD", cwd)
		os.Setenv("OLDPWD", prevDir)
	})
	return globalState
}

func (gs *GlobalState) UpdateCWD(newCWD string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Only update the previous directory if we're changing to a different directory
	if gs.CWD != newCWD {
		gs.PreviousDir = gs.CWD
	}
	gs.CWD = newCWD

	// Also update OLDPWD and PWD environment variables for consistency
	os.Setenv("OLDPWD", gs.PreviousDir)
	os.Setenv("PWD", gs.CWD)
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

func (gs *GlobalState) SetPreviousDir(prevDir string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.PreviousDir = prevDir
}
