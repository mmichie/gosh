package gosh

import (
	"fmt"
	"os"
	"regexp"
)

var assignmentRegex = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// Assignment is a single NAME=VALUE pair extracted from a command line.
type Assignment struct {
	Name  string
	Value string
}

// IsAssignment reports whether s has the shape NAME=VALUE (POSIX identifier
// followed by = and any string, possibly empty).
func IsAssignment(s string) (name, value string, ok bool) {
	m := assignmentRegex.FindStringSubmatch(s)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// SplitAssignments separates leading NAME=VALUE words from a command's parts.
// It also handles a quoted RHS that the lexer split into two tokens: when a
// part has the shape NAME= (empty value) and the next part is not itself an
// assignment, the next part is consumed as the value (with surrounding quotes
// stripped).
//
// The remaining slice is the command and its arguments — empty if the entire
// input was assignments.
func SplitAssignments(parts []string) (assigns []Assignment, remaining []string) {
	i := 0
	for i < len(parts) {
		name, value, ok := IsAssignment(parts[i])
		if !ok {
			break
		}
		if value == "" && i+1 < len(parts) {
			next := parts[i+1]
			// Only glue when the next token is a quoted string — that's the
			// shape produced by the lexer for FOO="bar baz". A bare word is a
			// separate argument or command, not a continuation of the RHS.
			if isQuotedToken(next) {
				value = stripQuotes(next)
				assigns = append(assigns, Assignment{Name: name, Value: ExpandSpecialVariables(value)})
				i += 2
				continue
			}
		}
		assigns = append(assigns, Assignment{Name: name, Value: ExpandSpecialVariables(stripQuotes(value))})
		i++
	}
	return assigns, parts[i:]
}

// ApplyAssignmentsToShell applies assignments to the shell's persistent state.
// Honors readonly variables and the current function scope: when inside a
// function, assignments to names that already exist as locals update the local
// scope; otherwise they go to the environment.
func ApplyAssignmentsToShell(assigns []Assignment) error {
	gs := GetGlobalState()
	for _, a := range assigns {
		if gs.IsReadonly(a.Name) {
			return fmt.Errorf("%s: readonly variable", a.Name)
		}
		if gs.IsInFunction() {
			if _, isLocal := gs.GetLocalVar(a.Name); isLocal {
				if err := gs.SetLocalVar(a.Name, a.Value); err != nil {
					return err
				}
				continue
			}
		}
		if err := gs.SetEnvVar(a.Name, a.Value); err != nil {
			return err
		}
	}
	return nil
}

// isQuotedToken reports whether s is wrapped in a matching pair of single or
// double quotes, the shape participle's Quote token produces.
func isQuotedToken(s string) bool {
	if len(s) < 2 {
		return false
	}
	return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')
}

// envWithAssignments returns the current process env with the given
// assignments overlaid (later entries shadow earlier ones with the same name,
// which matches Go's exec.Cmd.Env semantics).
func envWithAssignments(assigns []Assignment) []string {
	env := os.Environ()
	for _, a := range assigns {
		env = append(env, a.Name+"="+a.Value)
	}
	return env
}

// snapshotAndApplyForCommand applies assignments and returns a function that
// restores the prior values. Use for prefix assignments (FOO=bar cmd) so the
// settings survive only for the command's lifetime.
func snapshotAndApplyForCommand(assigns []Assignment) (restore func(), err error) {
	gs := GetGlobalState()
	type prior struct {
		name    string
		value   string
		existed bool
	}
	saved := make([]prior, 0, len(assigns))
	for _, a := range assigns {
		if gs.IsReadonly(a.Name) {
			for i := len(saved) - 1; i >= 0; i-- {
				if saved[i].existed {
					_ = gs.SetEnvVar(saved[i].name, saved[i].value)
				} else {
					_ = gs.UnsetEnvVar(saved[i].name)
				}
			}
			return nil, fmt.Errorf("%s: readonly variable", a.Name)
		}
		v, exists := os.LookupEnv(a.Name)
		saved = append(saved, prior{name: a.Name, value: v, existed: exists})
		if err := gs.SetEnvVar(a.Name, a.Value); err != nil {
			return nil, err
		}
	}
	return func() {
		for i := len(saved) - 1; i >= 0; i-- {
			if saved[i].existed {
				_ = gs.SetEnvVar(saved[i].name, saved[i].value)
			} else {
				_ = gs.UnsetEnvVar(saved[i].name)
			}
		}
	}, nil
}
