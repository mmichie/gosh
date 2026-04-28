package gosh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ShellOptions contains all shell option flags (set -e, -u, -x, -o pipefail, etc.)
type ShellOptions struct {
	Errexit   bool // -e: Exit immediately if a command exits with non-zero status
	Nounset   bool // -u: Treat unset variables as an error
	Xtrace    bool // -x: Print commands and their arguments as they are executed
	Pipefail  bool // -o pipefail: Return exit status of rightmost failed command in pipeline
	Verbose   bool // -v: Print shell input lines as they are read
	Noclobber bool // -C: Don't overwrite existing files with >
	Allexport bool // -a: Export all variables assigned to
}

// SignalTrap holds a command string to execute when a signal is received
type SignalTrap struct {
	Command string // Command to execute
	Signal  string // Signal name (e.g., "EXIT", "INT", "TERM")
}

// VariableScope represents a scope for local variables (used by functions)
type VariableScope struct {
	LocalVars map[string]string // Local variable values
	ParentEnv map[string]string // Snapshot of environment at scope entry
}

// VariableAttributes holds attributes for declared variables
type VariableAttributes struct {
	Readonly bool // -r: readonly
	Export   bool // -x: export
	Integer  bool // -i: integer (arithmetic evaluation)
}

type GlobalState struct {
	CWD               string
	PreviousDir       string
	DirStack          []string                       // Directory stack for pushd/popd
	ShellPID          int                            // $$ - Current shell PID
	LastBackgroundPID int                            // $! - Last background process PID
	LastExitStatus    int                            // $? - Exit status of last command
	StartTime         time.Time                      // For calculating $SECONDS
	ScriptName        string                         // $0 - Script/shell name
	PositionalParams  []string                       // $1, $2, ... - Positional parameters
	Options           ShellOptions                   // Shell options (set -e, -u, etc.)
	ReadonlyVars      map[string]bool                // Variables marked as readonly
	SignalTraps       map[string]string              // Signal handlers: signal name -> command
	ScopeStack        []VariableScope                // Stack of variable scopes for functions
	VarAttributes     map[string]*VariableAttributes // Variable attributes from declare
	InFunction        bool                           // Whether we're currently in a function
	FunctionDepth     int                            // Nesting depth of function calls
	mu                sync.RWMutex
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
			ScriptName:        "gosh",     // Default shell name
			PositionalParams:  []string{}, // No arguments initially
			ReadonlyVars:      make(map[string]bool),
			SignalTraps:       make(map[string]string), // No traps initially
			ScopeStack:        []VariableScope{},       // No scopes initially
			VarAttributes:     make(map[string]*VariableAttributes),
			InFunction:        false,
			FunctionDepth:     0,
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

// SetScriptName sets the script name ($0)
func (gs *GlobalState) SetScriptName(name string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.ScriptName = name
}

// GetScriptName returns the script name ($0)
func (gs *GlobalState) GetScriptName() string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.ScriptName
}

// SetPositionalParams sets all positional parameters ($1, $2, ...)
func (gs *GlobalState) SetPositionalParams(params []string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.PositionalParams = make([]string, len(params))
	copy(gs.PositionalParams, params)
}

// GetPositionalParams returns all positional parameters
func (gs *GlobalState) GetPositionalParams() []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	params := make([]string, len(gs.PositionalParams))
	copy(params, gs.PositionalParams)
	return params
}

// GetPositionalParam returns a specific positional parameter (1-indexed)
// Returns empty string if index is out of bounds
func (gs *GlobalState) GetPositionalParam(index int) string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	if index < 1 || index > len(gs.PositionalParams) {
		return ""
	}
	return gs.PositionalParams[index-1]
}

// GetPositionalParamCount returns the number of positional parameters ($#)
func (gs *GlobalState) GetPositionalParamCount() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return len(gs.PositionalParams)
}

// ShiftPositionalParams shifts positional parameters left by n positions
// Returns error if n is negative or larger than the number of parameters
func (gs *GlobalState) ShiftPositionalParams(n int) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if n < 0 {
		return fmt.Errorf("shift: invalid count: %d", n)
	}

	if n > len(gs.PositionalParams) {
		// Bash allows shifting more than available, just clears all
		gs.PositionalParams = []string{}
		return nil
	}

	gs.PositionalParams = gs.PositionalParams[n:]
	return nil
}

// GetOptions returns a copy of the shell options
func (gs *GlobalState) GetOptions() ShellOptions {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.Options
}

// SetOption sets a single shell option by name
func (gs *GlobalState) SetOption(name string, value bool) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	switch name {
	case "e", "errexit":
		gs.Options.Errexit = value
	case "u", "nounset":
		gs.Options.Nounset = value
	case "x", "xtrace":
		gs.Options.Xtrace = value
	case "pipefail":
		gs.Options.Pipefail = value
	case "v", "verbose":
		gs.Options.Verbose = value
	case "C", "noclobber":
		gs.Options.Noclobber = value
	case "a", "allexport":
		gs.Options.Allexport = value
	default:
		return fmt.Errorf("unknown option: %s", name)
	}
	return nil
}

// GetOption gets a single shell option by name
func (gs *GlobalState) GetOption(name string) (bool, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	switch name {
	case "e", "errexit":
		return gs.Options.Errexit, nil
	case "u", "nounset":
		return gs.Options.Nounset, nil
	case "x", "xtrace":
		return gs.Options.Xtrace, nil
	case "pipefail":
		return gs.Options.Pipefail, nil
	case "v", "verbose":
		return gs.Options.Verbose, nil
	case "C", "noclobber":
		return gs.Options.Noclobber, nil
	case "a", "allexport":
		return gs.Options.Allexport, nil
	default:
		return false, fmt.Errorf("unknown option: %s", name)
	}
}

// GetOptionsString returns the current shell flags in $- format
func (gs *GlobalState) GetOptionsString() string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	var flags strings.Builder
	if gs.Options.Errexit {
		flags.WriteByte('e')
	}
	if gs.Options.Nounset {
		flags.WriteByte('u')
	}
	if gs.Options.Xtrace {
		flags.WriteByte('x')
	}
	if gs.Options.Verbose {
		flags.WriteByte('v')
	}
	if gs.Options.Noclobber {
		flags.WriteByte('C')
	}
	if gs.Options.Allexport {
		flags.WriteByte('a')
	}
	// Note: pipefail doesn't have a single-letter flag
	return flags.String()
}

// IsReadonly checks if a variable is marked as readonly
func (gs *GlobalState) IsReadonly(name string) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.ReadonlyVars[name]
}

// SetReadonly marks a variable as readonly
func (gs *GlobalState) SetReadonly(name string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.ReadonlyVars[name] = true
}

// GetReadonlyVars returns a list of all readonly variable names
func (gs *GlobalState) GetReadonlyVars() []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	vars := make([]string, 0, len(gs.ReadonlyVars))
	for name := range gs.ReadonlyVars {
		vars = append(vars, name)
	}
	return vars
}

// SetEnvVar sets an environment variable, checking for readonly status
// Returns an error if the variable is readonly
func (gs *GlobalState) SetEnvVar(name, value string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.ReadonlyVars[name] {
		return fmt.Errorf("%s: readonly variable", name)
	}
	return os.Setenv(name, value)
}

// UnsetEnvVar unsets an environment variable, checking for readonly status
// Returns an error if the variable is readonly
func (gs *GlobalState) UnsetEnvVar(name string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.ReadonlyVars[name] {
		return fmt.Errorf("%s: readonly variable", name)
	}
	return os.Unsetenv(name)
}

// SetTrap sets a signal trap handler
func (gs *GlobalState) SetTrap(signal, command string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if command == "" || command == "-" {
		delete(gs.SignalTraps, signal)
	} else {
		gs.SignalTraps[signal] = command
	}
}

// GetTrap gets the trap command for a signal
func (gs *GlobalState) GetTrap(signal string) (string, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	cmd, exists := gs.SignalTraps[signal]
	return cmd, exists
}

// GetAllTraps returns a copy of all signal traps
func (gs *GlobalState) GetAllTraps() map[string]string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	traps := make(map[string]string)
	for k, v := range gs.SignalTraps {
		traps[k] = v
	}
	return traps
}

// ClearTrap removes a trap handler
func (gs *GlobalState) ClearTrap(signal string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	delete(gs.SignalTraps, signal)
}

// PushScope creates a new variable scope (called when entering a function)
func (gs *GlobalState) PushScope() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Capture current environment
	parentEnv := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			parentEnv[parts[0]] = parts[1]
		}
	}

	scope := VariableScope{
		LocalVars: make(map[string]string),
		ParentEnv: parentEnv,
	}
	gs.ScopeStack = append(gs.ScopeStack, scope)
	gs.FunctionDepth++
	gs.InFunction = true
}

// PopScope removes the current variable scope (called when exiting a function)
func (gs *GlobalState) PopScope() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if len(gs.ScopeStack) == 0 {
		return
	}

	// Get the scope being popped
	scope := gs.ScopeStack[len(gs.ScopeStack)-1]

	// Restore any overwritten environment variables
	for name := range scope.LocalVars {
		if origVal, exists := scope.ParentEnv[name]; exists {
			os.Setenv(name, origVal)
		} else {
			os.Unsetenv(name)
		}
	}

	gs.ScopeStack = gs.ScopeStack[:len(gs.ScopeStack)-1]
	gs.FunctionDepth--
	gs.InFunction = len(gs.ScopeStack) > 0
}

// SetLocalVar sets a local variable in the current scope
func (gs *GlobalState) SetLocalVar(name, value string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if len(gs.ScopeStack) == 0 {
		return fmt.Errorf("local: can only be used in a function")
	}

	// Set in current scope
	currentScope := &gs.ScopeStack[len(gs.ScopeStack)-1]
	currentScope.LocalVars[name] = value

	// Also set in environment so it's visible
	os.Setenv(name, value)
	return nil
}

// GetLocalVar gets a local variable from the current scope
func (gs *GlobalState) GetLocalVar(name string) (string, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	if len(gs.ScopeStack) == 0 {
		return "", false
	}

	currentScope := gs.ScopeStack[len(gs.ScopeStack)-1]
	val, exists := currentScope.LocalVars[name]
	return val, exists
}

// IsInFunction returns whether we're currently in a function
func (gs *GlobalState) IsInFunction() bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.InFunction
}

// GetFunctionDepth returns the current function nesting depth
func (gs *GlobalState) GetFunctionDepth() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.FunctionDepth
}

// SetVarAttributes sets attributes for a variable
func (gs *GlobalState) SetVarAttributes(name string, attrs *VariableAttributes) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.VarAttributes[name] = attrs
}

// GetVarAttributes gets attributes for a variable
func (gs *GlobalState) GetVarAttributes(name string) *VariableAttributes {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.VarAttributes[name]
}

// GetAllVarAttributes returns all variables with attributes
func (gs *GlobalState) GetAllVarAttributes() map[string]*VariableAttributes {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	attrs := make(map[string]*VariableAttributes)
	for k, v := range gs.VarAttributes {
		attrs[k] = v
	}
	return attrs
}
