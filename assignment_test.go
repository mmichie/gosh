package gosh

import (
	"os"
	"reflect"
	"testing"
)

func TestIsAssignment(t *testing.T) {
	cases := []struct {
		in    string
		name  string
		value string
		ok    bool
	}{
		{"FOO=bar", "FOO", "bar", true},
		{"foo_bar=baz", "foo_bar", "baz", true},
		{"_X=1", "_X", "1", true},
		{"FOO=", "FOO", "", true},
		{"FOO=a=b=c", "FOO", "a=b=c", true},
		{"FOO=$BAR", "FOO", "$BAR", true},
		{"=foo", "", "", false},
		{"1FOO=bar", "", "", false},
		{"foo bar", "", "", false},
		{"FOO", "", "", false},
		{"a/b=foo", "", "", false},
	}
	for _, c := range cases {
		name, value, ok := IsAssignment(c.in)
		if ok != c.ok || name != c.name || value != c.value {
			t.Errorf("IsAssignment(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, name, value, ok, c.name, c.value, c.ok)
		}
	}
}

func TestSplitAssignments(t *testing.T) {
	cases := []struct {
		name      string
		parts     []string
		assigns   []Assignment
		remaining []string
	}{
		{
			name:      "single assignment, no command",
			parts:     []string{"FOO=bar"},
			assigns:   []Assignment{{"FOO", "bar"}},
			remaining: []string{},
		},
		{
			name:      "multiple assignments, no command",
			parts:     []string{"A=1", "B=2", "C=3"},
			assigns:   []Assignment{{"A", "1"}, {"B", "2"}, {"C", "3"}},
			remaining: []string{},
		},
		{
			name:      "prefix assignment with command",
			parts:     []string{"FOO=bar", "echo", "hi"},
			assigns:   []Assignment{{"FOO", "bar"}},
			remaining: []string{"echo", "hi"},
		},
		{
			name:      "no assignment",
			parts:     []string{"echo", "FOO=bar"},
			assigns:   nil,
			remaining: []string{"echo", "FOO=bar"},
		},
		{
			name:      "quoted RHS lexer split",
			parts:     []string{"FOO=", `"hello world"`, "echo"},
			assigns:   []Assignment{{"FOO", "hello world"}},
			remaining: []string{"echo"},
		},
		{
			name:      "embedded equals preserved",
			parts:     []string{"URL=http://x=y/z"},
			assigns:   []Assignment{{"URL", "http://x=y/z"}},
			remaining: []string{},
		},
		{
			name:      "empty value",
			parts:     []string{"X=", "cmd"},
			assigns:   []Assignment{{"X", ""}},
			remaining: []string{"cmd"},
		},
		{
			name:      "assignment after command is not stripped",
			parts:     []string{"A=1", "cmd", "B=2"},
			assigns:   []Assignment{{"A", "1"}},
			remaining: []string{"cmd", "B=2"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assigns, remaining := SplitAssignments(c.parts)
			if !reflect.DeepEqual(assigns, c.assigns) {
				t.Errorf("assigns: got %v, want %v", assigns, c.assigns)
			}
			if !reflect.DeepEqual(remaining, c.remaining) {
				t.Errorf("remaining: got %v, want %v", remaining, c.remaining)
			}
		})
	}
}

func TestApplyAssignmentsToShell_setsEnv(t *testing.T) {
	defer os.Unsetenv("GOSH_TEST_ASSIGN_A")
	defer os.Unsetenv("GOSH_TEST_ASSIGN_B")
	if err := ApplyAssignmentsToShell([]Assignment{
		{"GOSH_TEST_ASSIGN_A", "alpha"},
		{"GOSH_TEST_ASSIGN_B", "beta"},
	}); err != nil {
		t.Fatalf("ApplyAssignmentsToShell: %v", err)
	}
	if got := os.Getenv("GOSH_TEST_ASSIGN_A"); got != "alpha" {
		t.Errorf("A: got %q, want alpha", got)
	}
	if got := os.Getenv("GOSH_TEST_ASSIGN_B"); got != "beta" {
		t.Errorf("B: got %q, want beta", got)
	}
}

func TestApplyAssignmentsToShell_readonlyRejects(t *testing.T) {
	gs := GetGlobalState()
	gs.SetReadonly("GOSH_TEST_RO")
	defer func() {
		// Best-effort cleanup. There is no direct unset for readonly flags,
		// but the variable name is unique to this test.
		gs.mu.Lock()
		delete(gs.ReadonlyVars, "GOSH_TEST_RO")
		gs.mu.Unlock()
	}()
	err := ApplyAssignmentsToShell([]Assignment{{"GOSH_TEST_RO", "x"}})
	if err == nil {
		t.Fatal("expected readonly error, got nil")
	}
}

func TestSnapshotAndApplyForCommand_restores(t *testing.T) {
	const name = "GOSH_TEST_PREFIX_VAR"
	os.Setenv(name, "original")
	defer os.Unsetenv(name)

	restore, err := snapshotAndApplyForCommand([]Assignment{{name, "temp"}})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if got := os.Getenv(name); got != "temp" {
		t.Errorf("during: got %q, want temp", got)
	}
	restore()
	if got := os.Getenv(name); got != "original" {
		t.Errorf("after restore: got %q, want original", got)
	}
}

func TestSnapshotAndApplyForCommand_unsetsIfDidNotExist(t *testing.T) {
	const name = "GOSH_TEST_FRESH_VAR"
	os.Unsetenv(name)
	if _, exists := os.LookupEnv(name); exists {
		t.Fatalf("precondition: %s already set", name)
	}
	restore, err := snapshotAndApplyForCommand([]Assignment{{name, "temp"}})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if got := os.Getenv(name); got != "temp" {
		t.Errorf("during: got %q, want temp", got)
	}
	restore()
	if _, exists := os.LookupEnv(name); exists {
		t.Errorf("after restore: %s should be unset", name)
	}
}

func TestEnvWithAssignments_overlay(t *testing.T) {
	os.Setenv("GOSH_TEST_OVERLAY_BASE", "base")
	defer os.Unsetenv("GOSH_TEST_OVERLAY_BASE")

	env := envWithAssignments([]Assignment{
		{"GOSH_TEST_OVERLAY_BASE", "shadow"},
		{"GOSH_TEST_OVERLAY_NEW", "new"},
	})

	var sawShadow, sawNew bool
	for _, kv := range env {
		if kv == "GOSH_TEST_OVERLAY_BASE=shadow" {
			sawShadow = true
		}
		if kv == "GOSH_TEST_OVERLAY_NEW=new" {
			sawNew = true
		}
	}
	if !sawShadow {
		t.Error("did not find shadowed BASE entry")
	}
	if !sawNew {
		t.Error("did not find NEW entry")
	}
}
