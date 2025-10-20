package gosh

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type GlobalState struct {
	CWD                string
	PreviousDir        string
	DirStack           []string // Directory stack for pushd/popd
	ShellPID           int      // $$ - Current shell PID
	LastBackgroundPID  int      // $! - Last background process PID
	LastExitStatus     int      // $? - Exit status of last command
	StartTime          time.Time // For calculating $SECONDS
	mu                 sync.RWMutex
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
			CWD:               cwd,
			PreviousDir:       prevDir,
			DirStack:          []string{cwd}, // Initialize with current directory
			ShellPID:          os.Getpid(),
			LastBackgroundPID: 0,
			LastExitStatus:    0,
			StartTime:         time.Now(),
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

	// Update the top of the directory stack
	if len(gs.DirStack) > 0 {
		gs.DirStack[0] = newCWD
	}

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

// PushDir pushes the current directory onto the stack and changes to the new directory
func (gs *GlobalState) PushDir(newDir string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Push current directory onto stack
	gs.DirStack = append(gs.DirStack, newDir)
}

// PopDir pops a directory from the stack and returns it
// Returns empty string if stack is empty
func (gs *GlobalState) PopDir() string {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if len(gs.DirStack) <= 1 {
		// Don't pop the last directory
		return ""
	}

	// Remove the top directory
	gs.DirStack = gs.DirStack[:len(gs.DirStack)-1]

	// Return the new top
	return gs.DirStack[len(gs.DirStack)-1]
}

// GetDirStack returns a copy of the directory stack
func (gs *GlobalState) GetDirStack() []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	// Return a copy to prevent external modification
	stack := make([]string, len(gs.DirStack))
	copy(stack, gs.DirStack)
	return stack
}

// RotateStack rotates the directory stack by n positions
func (gs *GlobalState) RotateStack(n int) string {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if len(gs.DirStack) == 0 {
		return ""
	}

	// Normalize n to be within stack bounds
	n = n % len(gs.DirStack)
	if n < 0 {
		n += len(gs.DirStack)
	}

	if n == 0 {
		return gs.DirStack[0]
	}

	// Rotate the stack
	rotated := append(gs.DirStack[n:], gs.DirStack[:n]...)
	gs.DirStack = rotated

	return gs.DirStack[0]
}

// UpdateDirStackTop updates the top of the directory stack
func (gs *GlobalState) UpdateDirStackTop(dir string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if len(gs.DirStack) > 0 {
		gs.DirStack[0] = dir
	}
}

// ResetDirStack resets the directory stack to contain only the current directory
func (gs *GlobalState) ResetDirStack() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.DirStack = []string{gs.CWD}
}

// RemoveStackElement removes the element at the specified index from the directory stack
// Returns the removed element, or empty string if index is out of bounds
func (gs *GlobalState) RemoveStackElement(index int) string {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if index < 0 || index >= len(gs.DirStack) {
		return ""
	}

	// Save the element to return
	removed := gs.DirStack[index]

	// Remove the element by rebuilding the slice
	gs.DirStack = append(gs.DirStack[:index], gs.DirStack[index+1:]...)

	return removed
}

// GetShellPID returns the shell's process ID ($$)
func (gs *GlobalState) GetShellPID() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.ShellPID
}

// SetLastBackgroundPID sets the PID of the last background process ($!)
func (gs *GlobalState) SetLastBackgroundPID(pid int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.LastBackgroundPID = pid
}

// GetLastBackgroundPID returns the PID of the last background process ($!)
func (gs *GlobalState) GetLastBackgroundPID() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.LastBackgroundPID
}

// SetLastExitStatus sets the exit status of the last command ($?)
func (gs *GlobalState) SetLastExitStatus(status int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.LastExitStatus = status
}

// GetLastExitStatus returns the exit status of the last command ($?)
func (gs *GlobalState) GetLastExitStatus() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.LastExitStatus
}

// GetSeconds returns the number of seconds since the shell started ($SECONDS)
func (gs *GlobalState) GetSeconds() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return int(time.Since(gs.StartTime).Seconds())
}
