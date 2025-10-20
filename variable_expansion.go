package gosh

import (
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ExpandSpecialVariables expands shell special variables in the given string
// Supports: $$, $!, $?, $PPID, $RANDOM, $SECONDS, and regular environment variables
func ExpandSpecialVariables(s string) string {
	state := GetGlobalState()

	// Create a map of special variable replacements
	replacements := make(map[string]string)

	// $$ - Current shell PID
	replacements["$$"] = strconv.Itoa(state.GetShellPID())

	// $! - Last background process PID
	replacements["$!"] = strconv.Itoa(state.GetLastBackgroundPID())

	// $? - Exit status of last command
	replacements["$?"] = strconv.Itoa(state.GetLastExitStatus())

	// $PPID - Parent process PID
	replacements["$PPID"] = strconv.Itoa(os.Getppid())

	// $RANDOM - Random number (0-32767)
	replacements["$RANDOM"] = strconv.Itoa(rand.Intn(32768))

	// $SECONDS - Seconds since shell start
	replacements["$SECONDS"] = strconv.Itoa(state.GetSeconds())

	// Replace special variables
	result := s
	for varName, value := range replacements {
		result = strings.ReplaceAll(result, varName, value)
	}

	// Expand environment variables: $VAR and ${VAR}
	// Handle ${VAR} first
	braceRe := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	result = braceRe.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		return os.Getenv(varName)
	})

	// Handle $VAR (but not special variables we've already handled)
	simpleRe := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	result = simpleRe.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[1:] // Remove $
		// Skip if it's a special variable we've already handled
		if varName == "PPID" || varName == "RANDOM" || varName == "SECONDS" {
			// Already handled above, skip
			return match
		}
		return os.Getenv(varName)
	})

	return result
}

// ExpandVariablesInArgs expands special variables in all arguments
func ExpandVariablesInArgs(args []string) []string {
	expanded := make([]string, len(args))
	for i, arg := range args {
		expanded[i] = ExpandSpecialVariables(arg)
	}
	return expanded
}
