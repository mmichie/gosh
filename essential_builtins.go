package gosh

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// colonCommand implements the : (null) command
// The : command does nothing and returns success
// It's useful in conditional expressions and as a placeholder
func colonCommand(cmd *Command) error {
	// Do nothing, just return success
	cmd.ReturnCode = 0
	return nil
}

// unsetCommand implements the unset builtin
// Usage: unset [-v|-f] name [name ...]
// -v: Treat each name as a shell variable (default)
// -f: Treat each name as a shell function
func unsetCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return nil // unset with no args does nothing
	}

	unsetFunctions := false
	varNames := []string{}

	for _, arg := range args {
		switch arg {
		case "-v":
			// Unset variables (default behavior)
			unsetFunctions = false
		case "-f":
			// Unset functions - not yet implemented
			unsetFunctions = true
		default:
			varNames = append(varNames, arg)
		}
	}

	if unsetFunctions {
		// TODO: Implement function unset when we have shell functions
		return fmt.Errorf("unset: -f: shell functions not yet implemented")
	}

	// Unset environment variables
	gs := GetGlobalState()
	for _, name := range varNames {
		if err := gs.UnsetEnvVar(name); err != nil {
			return fmt.Errorf("unset: %v", err)
		}
	}

	return nil
}

// sourceCommand implements the source/. builtin
// Usage: source filename [arguments]
// Reads and executes commands from the filename in the current shell environment
func sourceCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return fmt.Errorf("source: filename argument required")
	}

	filename := args[0]

	// Resolve the file path
	resolvedPath, err := resolveSourceFile(filename)
	if err != nil {
		return fmt.Errorf("source: %v", err)
	}

	// Read the file
	file, err := os.Open(resolvedPath)
	if err != nil {
		return fmt.Errorf("source: %v", err)
	}
	defer file.Close()

	// Save and set positional parameters if additional arguments are provided
	gs := GetGlobalState()
	oldParams := gs.GetPositionalParams()
	if len(args) > 1 {
		gs.SetPositionalParams(args[1:])
	}

	// Read and execute each line
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Execute the line using the shell's command execution
		execCmd, err := NewCommand(line, cmd.JobManager)
		if err != nil {
			gs.SetPositionalParams(oldParams)
			return fmt.Errorf("source: %s: line %d: %v", filename, lineNum, err)
		}
		execCmd.Stdin = cmd.Stdin
		execCmd.Stdout = cmd.Stdout
		execCmd.Stderr = cmd.Stderr

		execCmd.Run()

		// Check for errexit option
		if execCmd.ReturnCode != 0 && gs.GetOptions().Errexit {
			gs.SetPositionalParams(oldParams)
			cmd.ReturnCode = execCmd.ReturnCode
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		gs.SetPositionalParams(oldParams)
		return fmt.Errorf("source: error reading %s: %v", filename, err)
	}

	// Restore positional parameters
	gs.SetPositionalParams(oldParams)
	cmd.ReturnCode = 0
	return nil
}

// resolveSourceFile finds the source file, checking relative and PATH
func resolveSourceFile(filename string) (string, error) {
	// If it's an absolute path, use it directly
	if filepath.IsAbs(filename) {
		if _, err := os.Stat(filename); err != nil {
			return "", fmt.Errorf("%s: No such file or directory", filename)
		}
		return filename, nil
	}

	// Check if file exists in current directory or as relative path
	if _, err := os.Stat(filename); err == nil {
		return filename, nil
	}

	// Search in PATH for the file
	pathEnv := os.Getenv("PATH")
	if pathEnv != "" {
		paths := strings.Split(pathEnv, ":")
		for _, dir := range paths {
			fullPath := filepath.Join(dir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("%s: No such file or directory", filename)
}

// evalCommand implements the eval builtin
// Usage: eval [arg ...]
// Concatenates arguments and executes them as a shell command
func evalCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) == 0 {
		return nil // eval with no args does nothing
	}

	// Concatenate all arguments with spaces
	cmdString := strings.Join(args, " ")

	// Execute the command string
	execCmd, err := NewCommand(cmdString, cmd.JobManager)
	if err != nil {
		return fmt.Errorf("eval: %v", err)
	}
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	execCmd.Run()

	cmd.ReturnCode = execCmd.ReturnCode
	return nil
}

// getBuiltinArgs is a helper to extract arguments from a command
// Returns all arguments after the command name
func getBuiltinArgs(cmd *Command) []string {
	if cmd.Command == nil ||
		len(cmd.Command.LogicalBlocks) == 0 ||
		cmd.Command.LogicalBlocks[0].FirstPipeline == nil ||
		len(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands) == 0 {
		return nil
	}

	parts := getCommandParts(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands[0])
	if len(parts) > 1 {
		return parts[1:]
	}
	return nil
}

// execCommand implements the exec builtin
// Usage: exec [-c] [-l] [-a name] [command [arguments]]
// Replaces the shell with command without creating a new process
// If no command is given, any redirections take effect in the current shell
func execCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	var clearEnv bool
	var loginShell bool
	var argv0 string
	commandArgs := []string{}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "-c" {
			clearEnv = true
			i++
		} else if arg == "-l" {
			loginShell = true
			i++
		} else if arg == "-a" {
			i++
			if i >= len(args) {
				return fmt.Errorf("exec: -a: option requires an argument")
			}
			argv0 = args[i]
			i++
		} else if arg == "--" {
			i++
			commandArgs = args[i:]
			break
		} else if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("exec: %s: invalid option", arg)
		} else {
			commandArgs = args[i:]
			break
		}
	}

	// If no command is given, exec only handles redirections
	// (redirections are handled by the command execution layer)
	if len(commandArgs) == 0 {
		return nil
	}

	// Find the command in PATH
	commandPath, err := lookupCommand(commandArgs[0])
	if err != nil {
		return fmt.Errorf("exec: %s: %v", commandArgs[0], err)
	}

	// Prepare argv0 (the name the command sees as $0)
	if argv0 == "" {
		argv0 = commandArgs[0]
	}
	if loginShell && !strings.HasPrefix(argv0, "-") {
		argv0 = "-" + argv0
	}

	// Prepare environment
	var environ []string
	if clearEnv {
		environ = []string{}
	} else {
		environ = os.Environ()
	}

	// Prepare the full argv (argv0 followed by other arguments)
	argv := append([]string{argv0}, commandArgs[1:]...)

	// Replace the current process with the new command
	// This never returns on success
	return syscall.Exec(commandPath, argv, environ)
}

// lookupCommand finds the full path of a command
func lookupCommand(name string) (string, error) {
	// If it contains a slash, use it directly
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err != nil {
			return "", fmt.Errorf("no such file or directory")
		}
		return filepath.Abs(name)
	}

	// Search in PATH
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("not found")
	}

	paths := strings.Split(pathEnv, ":")
	for _, dir := range paths {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil {
			// Check if it's executable
			if info.Mode()&0111 != 0 {
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("not found")
}

// readonlyCommand implements the readonly builtin
// Usage: readonly [-p] [name[=value] ...]
// Marks variables as readonly, preventing their modification or unsetting
func readonlyCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	gs := GetGlobalState()

	// Parse options
	printMode := false
	varSpecs := []string{}

	for _, arg := range args {
		if arg == "-p" {
			printMode = true
		} else if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("readonly: %s: invalid option", arg)
		} else {
			varSpecs = append(varSpecs, arg)
		}
	}

	// If -p or no arguments, print readonly variables
	if printMode || len(varSpecs) == 0 {
		return printReadonlyVars(cmd, gs)
	}

	// Process each variable specification
	for _, spec := range varSpecs {
		// Check if it's an assignment (name=value)
		if idx := strings.Index(spec, "="); idx != -1 {
			name := spec[:idx]
			value := spec[idx+1:]

			// Check if already readonly before setting
			if gs.IsReadonly(name) {
				return fmt.Errorf("readonly: %s: readonly variable", name)
			}

			// Set the variable value
			os.Setenv(name, value)

			// Mark as readonly
			gs.SetReadonly(name)
		} else {
			// Just mark existing variable as readonly
			name := spec

			// Mark as readonly (even if variable doesn't exist, like bash)
			gs.SetReadonly(name)
		}
	}

	return nil
}

// printReadonlyVars prints all readonly variables in a reusable format
func printReadonlyVars(cmd *Command, gs *GlobalState) error {
	readonlyVars := gs.GetReadonlyVars()

	// Sort for consistent output
	sort.Strings(readonlyVars)

	for _, name := range readonlyVars {
		value := os.Getenv(name)
		if value != "" {
			fmt.Fprintf(cmd.Stdout, "declare -r %s=%q\n", name, value)
		} else {
			fmt.Fprintf(cmd.Stdout, "declare -r %s\n", name)
		}
	}

	return nil
}

// trapCommand implements the trap builtin
// Usage: trap [-lp] [[arg] signal_spec ...]
// -l: List signal names
// -p: Print trap commands
// trap '' signal: Ignore signal
// trap - signal: Reset to default
// trap command signal: Execute command on signal
func trapCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	gs := GetGlobalState()

	// Parse options
	listSignals := false
	printTraps := false
	i := 0

	for i < len(args) {
		if args[i] == "-l" {
			listSignals = true
			i++
		} else if args[i] == "-p" {
			printTraps = true
			i++
		} else if args[i] == "--" {
			i++
			break
		} else if args[i] == "-" {
			// Single dash is the command to reset trap, not an option
			break
		} else if strings.HasPrefix(args[i], "-") && len(args[i]) > 1 {
			return fmt.Errorf("trap: %s: invalid option", args[i])
		} else {
			break
		}
	}

	// -l: List all signal names
	if listSignals {
		signals := []string{
			"EXIT", "HUP", "INT", "QUIT", "ILL", "TRAP", "ABRT", "BUS",
			"FPE", "KILL", "USR1", "SEGV", "USR2", "PIPE", "ALRM", "TERM",
			"STKFLT", "CHLD", "CONT", "STOP", "TSTP", "TTIN", "TTOU", "URG",
			"XCPU", "XFSZ", "VTALRM", "PROF", "WINCH", "IO", "PWR", "SYS",
		}
		for j, sig := range signals {
			fmt.Fprintf(cmd.Stdout, "%2d) SIG%-8s", j, sig)
			if (j+1)%4 == 0 {
				fmt.Fprintln(cmd.Stdout)
			}
		}
		if len(signals)%4 != 0 {
			fmt.Fprintln(cmd.Stdout)
		}
		return nil
	}

	// -p or no args: Print current traps
	if printTraps || len(args) == i {
		return printTraps2(cmd, gs, args[i:])
	}

	// Remaining args: set traps
	// trap command signal [signal ...]
	if len(args)-i < 2 {
		// Just signal name(s) - print those specific traps
		return printTraps2(cmd, gs, args[i:])
	}

	command := args[i]
	// Strip outer quotes from command if present
	if len(command) >= 2 {
		if (command[0] == '\'' && command[len(command)-1] == '\'') ||
			(command[0] == '"' && command[len(command)-1] == '"') {
			command = command[1 : len(command)-1]
		}
	}
	signals := args[i+1:]

	for _, sig := range signals {
		sigName := normalizeSignalName(sig)

		// Register the trap
		gs.SetTrap(sigName, command)

		// Set up actual signal handler for supported signals
		if command == "" || command == "-" {
			// Reset to default
			resetSignalHandler(sigName)
		} else {
			setupSignalHandler(sigName, command, cmd)
		}
	}

	return nil
}

// printTraps2 prints trap commands (named printTraps2 to avoid conflict)
func printTraps2(cmd *Command, gs *GlobalState, signals []string) error {
	traps := gs.GetAllTraps()

	if len(signals) > 0 {
		// Print specific signals
		for _, sig := range signals {
			sigName := normalizeSignalName(sig)
			if trapCmd, exists := traps[sigName]; exists {
				fmt.Fprintf(cmd.Stdout, "trap -- %q %s\n", trapCmd, sigName)
			}
		}
	} else {
		// Print all traps
		sigNames := make([]string, 0, len(traps))
		for sig := range traps {
			sigNames = append(sigNames, sig)
		}
		sort.Strings(sigNames)

		for _, sig := range sigNames {
			fmt.Fprintf(cmd.Stdout, "trap -- %q %s\n", traps[sig], sig)
		}
	}
	return nil
}

// normalizeSignalName converts signal specifications to standard names
func normalizeSignalName(sig string) string {
	sig = strings.ToUpper(sig)
	sig = strings.TrimPrefix(sig, "SIG")

	// Handle numeric signals
	if num, err := strconv.Atoi(sig); err == nil {
		signalNames := map[int]string{
			0:  "EXIT",
			1:  "HUP",
			2:  "INT",
			3:  "QUIT",
			6:  "ABRT",
			9:  "KILL",
			14: "ALRM",
			15: "TERM",
		}
		if name, ok := signalNames[num]; ok {
			return name
		}
	}
	return sig
}

// signalFromName converts a signal name to os.Signal
func signalFromName(name string) syscall.Signal {
	signalMap := map[string]syscall.Signal{
		"HUP":    syscall.SIGHUP,
		"INT":    syscall.SIGINT,
		"QUIT":   syscall.SIGQUIT,
		"ILL":    syscall.SIGILL,
		"TRAP":   syscall.SIGTRAP,
		"ABRT":   syscall.SIGABRT,
		"BUS":    syscall.SIGBUS,
		"FPE":    syscall.SIGFPE,
		"KILL":   syscall.SIGKILL,
		"USR1":   syscall.SIGUSR1,
		"SEGV":   syscall.SIGSEGV,
		"USR2":   syscall.SIGUSR2,
		"PIPE":   syscall.SIGPIPE,
		"ALRM":   syscall.SIGALRM,
		"TERM":   syscall.SIGTERM,
		"CHLD":   syscall.SIGCHLD,
		"CONT":   syscall.SIGCONT,
		"STOP":   syscall.SIGSTOP,
		"TSTP":   syscall.SIGTSTP,
		"TTIN":   syscall.SIGTTIN,
		"TTOU":   syscall.SIGTTOU,
		"URG":    syscall.SIGURG,
		"XCPU":   syscall.SIGXCPU,
		"XFSZ":   syscall.SIGXFSZ,
		"VTALRM": syscall.SIGVTALRM,
		"PROF":   syscall.SIGPROF,
		"WINCH":  syscall.SIGWINCH,
		"IO":     syscall.SIGIO,
		"SYS":    syscall.SIGSYS,
	}
	if sig, ok := signalMap[name]; ok {
		return sig
	}
	return 0
}

// setupSignalHandler sets up a signal handler for a trap
func setupSignalHandler(sigName, command string, cmd *Command) {
	// EXIT is special - handled at shell exit, not via signal
	if sigName == "EXIT" {
		return
	}

	sig := signalFromName(sigName)
	if sig == 0 {
		return
	}

	// Create channel for signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig)

	// Handle signal in goroutine
	go func() {
		for range ch {
			// Execute trap command
			gs := GetGlobalState()
			if trapCmd, exists := gs.GetTrap(sigName); exists && trapCmd != "" {
				execCmd, err := NewCommand(trapCmd, cmd.JobManager)
				if err == nil {
					execCmd.Stdin = cmd.Stdin
					execCmd.Stdout = cmd.Stdout
					execCmd.Stderr = cmd.Stderr
					execCmd.Run()
				}
			}
		}
	}()
}

// resetSignalHandler resets a signal to its default behavior
func resetSignalHandler(sigName string) {
	if sigName == "EXIT" {
		return
	}

	sig := signalFromName(sigName)
	if sig == 0 {
		return
	}

	signal.Reset(sig)
}

// ReturnError is used to signal a return from a function or sourced script
type ReturnError struct {
	Code int
}

func (e *ReturnError) Error() string {
	return fmt.Sprintf("return: %d", e.Code)
}

// returnCommand implements the return builtin
// Usage: return [n]
// Returns from a function or sourced script with exit status n
func returnCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	gs := GetGlobalState()

	// Check if we're in a function or sourced script context
	// For now, we'll allow return anywhere but it's most meaningful in functions
	if !gs.IsInFunction() {
		// In bash, return outside a function in a sourced script exits the script
		// We'll simulate this behavior
	}

	// Default return code is 0, or the exit status of the last command
	returnCode := gs.GetLastExitStatus()

	if len(args) > 0 {
		var err error
		returnCode, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("return: %s: numeric argument required", args[0])
		}
	}

	cmd.ReturnCode = returnCode

	// Signal a return using a special error type
	return &ReturnError{Code: returnCode}
}

// localCommand implements the local builtin
// Usage: local [name[=value] ...]
// Creates local variables in the current function scope
func localCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	gs := GetGlobalState()

	// Check if we're in a function
	if !gs.IsInFunction() {
		return fmt.Errorf("local: can only be used in a function")
	}

	// If no args, list local variables
	if len(args) == 0 {
		return nil
	}

	// Process each variable specification
	for _, spec := range args {
		// Check if it's an assignment (name=value)
		if idx := strings.Index(spec, "="); idx != -1 {
			name := spec[:idx]
			value := spec[idx+1:]

			if err := gs.SetLocalVar(name, value); err != nil {
				return err
			}
		} else {
			// Just declare the variable as local with empty value
			if err := gs.SetLocalVar(spec, ""); err != nil {
				return err
			}
		}
	}

	return nil
}

// declareCommand implements the declare/typeset builtin
// Usage: declare [-aAfFgilnrtux] [-p] [name[=value] ...]
// -r: readonly
// -x: export
// -i: integer
// -p: print variables
// -g: global (when in function)
func declareCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	gs := GetGlobalState()

	// Parse options
	readonly := false
	export := false
	integer := false
	printVars := false
	global := false
	unsetAttrs := false
	i := 0

	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "+") {
			unset := strings.HasPrefix(arg, "+")
			flags := arg[1:]
			for _, flag := range flags {
				switch flag {
				case 'r':
					readonly = !unset
				case 'x':
					export = !unset
				case 'i':
					integer = !unset
				case 'p':
					printVars = true
				case 'g':
					global = true
				case 'a', 'A', 'f', 'F', 'l', 'n', 't', 'u':
					// These options are not yet implemented
					return fmt.Errorf("declare: -%c: not yet implemented", flag)
				default:
					return fmt.Errorf("declare: -%c: invalid option", flag)
				}
			}
			unsetAttrs = unset
			i++
		} else {
			break
		}
	}

	varSpecs := args[i:]

	// -p: print variables
	if printVars {
		return printDeclaredVars(cmd, gs, varSpecs, readonly, export, integer)
	}

	// If no variable specs and no options, print all
	if len(varSpecs) == 0 && !readonly && !export && !integer {
		return printAllDeclaredVars(cmd, gs)
	}

	// Process each variable specification
	for _, spec := range varSpecs {
		var name, value string
		hasValue := false

		// Check if it's an assignment (name=value)
		if idx := strings.Index(spec, "="); idx != -1 {
			name = spec[:idx]
			value = spec[idx+1:]
			hasValue = true
		} else {
			name = spec
		}

		// Check if variable is already readonly (before we potentially add readonly attr)
		wasReadonly := gs.IsReadonly(name)

		// Get or create attributes
		attrs := gs.GetVarAttributes(name)
		if attrs == nil {
			attrs = &VariableAttributes{}
		}

		if unsetAttrs {
			// Remove attributes
			if readonly {
				// Can't unset readonly
				if attrs.Readonly {
					return fmt.Errorf("%s: readonly variable", name)
				}
			}
			if export {
				attrs.Export = false
			}
			if integer {
				attrs.Integer = false
			}
		} else {
			// Set value if provided (before marking readonly!)
			if hasValue {
				// Only fail if was already readonly before this command
				if wasReadonly {
					return fmt.Errorf("%s: readonly variable", name)
				}

				// Handle integer attribute
				if integer || attrs.Integer {
					// Evaluate as arithmetic expression
					// For now, just try to parse as integer
					if _, err := strconv.Atoi(value); err != nil {
						// Try to evaluate simple expressions later
						// For now, default to 0 for non-numeric
						value = "0"
					}
				}

				// Set in appropriate scope
				if global || !gs.IsInFunction() {
					os.Setenv(name, value)
				} else {
					gs.SetLocalVar(name, value)
				}

				// Export if -x was specified
				if export || attrs.Export {
					os.Setenv(name, value)
				}
			}

			// Now set attributes (including readonly)
			if readonly {
				attrs.Readonly = true
				gs.SetReadonly(name)
			}
			if export {
				attrs.Export = true
			}
			if integer {
				attrs.Integer = true
			}
		}

		gs.SetVarAttributes(name, attrs)
	}

	return nil
}

// printDeclaredVars prints variables matching the given criteria
func printDeclaredVars(cmd *Command, gs *GlobalState, names []string, readonly, export, integer bool) error {
	if len(names) > 0 {
		// Print specific variables
		for _, name := range names {
			attrs := gs.GetVarAttributes(name)
			value := os.Getenv(name)
			printDeclare(cmd, name, value, attrs, gs.IsReadonly(name))
		}
	} else {
		// Print all variables matching criteria
		allAttrs := gs.GetAllVarAttributes()
		readonlyVars := gs.GetReadonlyVars()

		// Collect and sort names
		varNames := make([]string, 0)
		seen := make(map[string]bool)

		for name := range allAttrs {
			seen[name] = true
			varNames = append(varNames, name)
		}
		for _, name := range readonlyVars {
			if !seen[name] {
				varNames = append(varNames, name)
			}
		}
		sort.Strings(varNames)

		for _, name := range varNames {
			attrs := gs.GetVarAttributes(name)
			isRo := gs.IsReadonly(name)
			value := os.Getenv(name)

			// Filter by requested attributes
			if readonly && !isRo {
				continue
			}
			if export && (attrs == nil || !attrs.Export) {
				continue
			}
			if integer && (attrs == nil || !attrs.Integer) {
				continue
			}

			printDeclare(cmd, name, value, attrs, isRo)
		}
	}
	return nil
}

// printDeclare prints a single variable in declare format
func printDeclare(cmd *Command, name, value string, attrs *VariableAttributes, isReadonly bool) {
	var flags strings.Builder
	flags.WriteString("declare ")

	if isReadonly {
		flags.WriteString("-r ")
	}
	if attrs != nil {
		if attrs.Export {
			flags.WriteString("-x ")
		}
		if attrs.Integer {
			flags.WriteString("-i ")
		}
	}

	if value != "" {
		fmt.Fprintf(cmd.Stdout, "%s%s=%q\n", flags.String(), name, value)
	} else {
		fmt.Fprintf(cmd.Stdout, "%s%s\n", flags.String(), name)
	}
}

// printAllDeclaredVars prints all declared variables
func printAllDeclaredVars(cmd *Command, gs *GlobalState) error {
	// Print all environment variables
	envVars := os.Environ()
	sort.Strings(envVars)

	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			name := parts[0]
			value := parts[1]
			attrs := gs.GetVarAttributes(name)
			isRo := gs.IsReadonly(name)
			printDeclare(cmd, name, value, attrs, isRo)
		}
	}
	return nil
}

// typesetCommand is an alias for declare (for compatibility)
func typesetCommand(cmd *Command) error {
	return declareCommand(cmd)
}
