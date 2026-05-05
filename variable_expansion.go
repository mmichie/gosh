package gosh

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ExpandSpecialVariables expands shell special variables in the given string
// Supports: $$, $!, $?, $0-$9, ${...} parameter expansion, $#, $@, $*, $PPID,
// $RANDOM, $SECONDS, and regular environment variables. Errors from explicit
// failure forms like ${var:?msg} are swallowed; use ExpandSpecialVariablesE
// to surface them.
func ExpandSpecialVariables(s string) string {
	out, _ := ExpandSpecialVariablesE(s)
	return out
}

// ExpandSpecialVariablesE is the error-returning variant. Parameter expansion
// constructs like ${var:?message} report their error here.
func ExpandSpecialVariablesE(s string) (string, error) {
	// Brace-aware parameter expansion runs first so the operator-bearing forms
	// (${var:-default}, ${var/foo/bar}, etc.) are resolved before any of the
	// short-form replacements below could mangle them.
	result, err := ExpandParameterExpansions(s)
	if err != nil {
		return "", err
	}

	state := GetGlobalState()
	replacements := map[string]string{
		"$$":       strconv.Itoa(state.GetShellPID()),
		"$!":       strconv.Itoa(state.GetLastBackgroundPID()),
		"$?":       strconv.Itoa(state.GetLastExitStatus()),
		"$#":       strconv.Itoa(state.GetPositionalParamCount()),
		"$0":       state.GetScriptName(),
		"$PPID":    strconv.Itoa(os.Getppid()),
		"$RANDOM":  strconv.Itoa(rand.Intn(32768)),
		"$SECONDS": strconv.Itoa(state.GetSeconds()),
	}
	for i := 1; i <= 9; i++ {
		replacements["$"+strconv.Itoa(i)] = state.GetPositionalParam(i)
	}
	params := state.GetPositionalParams()
	replacements["$@"] = strings.Join(params, " ")
	replacements["$*"] = strings.Join(params, " ")

	for varName, value := range replacements {
		result = strings.ReplaceAll(result, varName, value)
	}

	simpleRe := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	result = simpleRe.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[1:]
		if varName == "PPID" || varName == "RANDOM" || varName == "SECONDS" {
			return match
		}
		return os.Getenv(varName)
	})

	return result, nil
}

// ExpandVariablesInArgs expands special variables in all arguments
func ExpandVariablesInArgs(args []string) []string {
	expanded := make([]string, len(args))
	for i, arg := range args {
		expanded[i] = ExpandSpecialVariables(arg)
	}
	return expanded
}

// ExpandVariablesInArgsWithNounset expands variables and returns an error if
// nounset is enabled and an unset variable is referenced
func ExpandVariablesInArgsWithNounset(args []string) ([]string, error) {
	state := GetGlobalState()
	opts := state.GetOptions()

	if !opts.Nounset {
		// If nounset is not enabled, use the regular expansion
		return ExpandVariablesInArgs(args), nil
	}

	expanded := make([]string, len(args))
	for i, arg := range args {
		result, err := expandWithNounsetCheck(arg)
		if err != nil {
			return nil, err
		}
		expanded[i] = result
	}
	return expanded, nil
}

// expandWithNounsetCheck expands variables and errors on unset variables
func expandWithNounsetCheck(s string) (string, error) {
	state := GetGlobalState()

	// First check for unset variables before doing any expansion

	// Check ${VAR} format for unset variables
	braceRe := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*|\d+)\}`)
	for _, match := range braceRe.FindAllStringSubmatch(s, -1) {
		varName := match[1]

		// Check if it's a numeric positional parameter
		if num, err := strconv.Atoi(varName); err == nil {
			if num == 0 {
				continue // $0 is always set
			}
			param := state.GetPositionalParam(num)
			if param == "" && num > state.GetPositionalParamCount() {
				return "", fmt.Errorf("unbound variable: %s", varName)
			}
			continue
		}

		// Check if it's an environment variable
		if _, exists := os.LookupEnv(varName); !exists {
			// Check if it's a special variable that's always set
			if !isSpecialVariable(varName) {
				return "", fmt.Errorf("unbound variable: %s", varName)
			}
		}
	}

	// Check $VAR format for unset variables
	simpleRe := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	for _, match := range simpleRe.FindAllStringSubmatch(s, -1) {
		varName := match[1]

		// Skip special variables that are always defined
		if isSpecialVariable(varName) {
			continue
		}

		// Check if it's an environment variable
		if _, exists := os.LookupEnv(varName); !exists {
			return "", fmt.Errorf("unbound variable: %s", varName)
		}
	}

	// All variables are set, do the regular expansion
	return ExpandSpecialVariables(s), nil
}

// isSpecialVariable returns true if the variable name is a special variable
// that is always defined (like PPID, RANDOM, SECONDS, etc.)
func isSpecialVariable(name string) bool {
	switch name {
	case "PPID", "RANDOM", "SECONDS", "PWD", "OLDPWD", "HOME", "USER", "SHELL", "PATH":
		return true
	default:
		return false
	}
}
