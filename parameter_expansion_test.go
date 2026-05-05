package gosh

import (
	"os"
	"strings"
	"testing"
)

func TestParameterExpansion_DefaultForms(t *testing.T) {
	os.Unsetenv("GOSH_PE_UNSET")
	os.Setenv("GOSH_PE_SET", "value")
	os.Setenv("GOSH_PE_EMPTY", "")
	defer os.Unsetenv("GOSH_PE_SET")
	defer os.Unsetenv("GOSH_PE_EMPTY")

	cases := []struct {
		input    string
		expected string
	}{
		{"${GOSH_PE_UNSET:-fallback}", "fallback"},
		{"${GOSH_PE_SET:-fallback}", "value"},
		{"${GOSH_PE_EMPTY:-fallback}", "fallback"},
		{"${GOSH_PE_EMPTY-fallback}", ""},
		{"${GOSH_PE_UNSET-fallback}", "fallback"},
		{"${GOSH_PE_SET:+alt}", "alt"},
		{"${GOSH_PE_EMPTY:+alt}", ""},
		{"${GOSH_PE_UNSET:+alt}", ""},
		{"${GOSH_PE_EMPTY+alt}", "alt"},
		{"prefix-${GOSH_PE_UNSET:-default}-suffix", "prefix-default-suffix"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ExpandSpecialVariablesE(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParameterExpansion_AssignDefault(t *testing.T) {
	os.Unsetenv("GOSH_PE_ASSIGN")
	defer os.Unsetenv("GOSH_PE_ASSIGN")

	got, err := ExpandSpecialVariablesE("${GOSH_PE_ASSIGN:=initial}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "initial" {
		t.Errorf("got %q, want %q", got, "initial")
	}
	if v := os.Getenv("GOSH_PE_ASSIGN"); v != "initial" {
		t.Errorf("env not set: got %q, want %q", v, "initial")
	}

	// Second call must not overwrite.
	got, err = ExpandSpecialVariablesE("${GOSH_PE_ASSIGN:=other}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "initial" {
		t.Errorf("got %q, want %q", got, "initial")
	}
}

func TestParameterExpansion_ErrorForm(t *testing.T) {
	os.Unsetenv("GOSH_PE_NOTSET")

	_, err := ExpandSpecialVariablesE("${GOSH_PE_NOTSET:?must be set}")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must be set") {
		t.Errorf("unexpected error message: %v", err)
	}

	os.Setenv("GOSH_PE_NOTSET", "ok")
	defer os.Unsetenv("GOSH_PE_NOTSET")
	got, err := ExpandSpecialVariablesE("${GOSH_PE_NOTSET:?must be set}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("got %q, want %q", got, "ok")
	}
}

func TestParameterExpansion_Length(t *testing.T) {
	os.Setenv("GOSH_PE_LEN", "hello")
	defer os.Unsetenv("GOSH_PE_LEN")

	got, err := ExpandSpecialVariablesE("${#GOSH_PE_LEN}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "5" {
		t.Errorf("got %q, want %q", got, "5")
	}

	os.Unsetenv("GOSH_PE_LEN")
	got, err = ExpandSpecialVariablesE("${#GOSH_PE_LEN}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0" {
		t.Errorf("unset length: got %q, want %q", got, "0")
	}
}

func TestParameterExpansion_Substring(t *testing.T) {
	os.Setenv("GOSH_PE_STR", "hello world")
	defer os.Unsetenv("GOSH_PE_STR")

	cases := []struct {
		input    string
		expected string
	}{
		{"${GOSH_PE_STR:0}", "hello world"},
		{"${GOSH_PE_STR:6}", "world"},
		{"${GOSH_PE_STR:0:5}", "hello"},
		{"${GOSH_PE_STR:6:5}", "world"},
		{"${GOSH_PE_STR:-5}", "hello world"}, // -5 is the default-form, not negative offset
		{"${GOSH_PE_STR: -5}", "world"},      // explicit negative offset (with space)
		{"${GOSH_PE_STR:0:-6}", "hello"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ExpandSpecialVariablesE(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParameterExpansion_PrefixSuffixRemoval(t *testing.T) {
	os.Setenv("GOSH_PE_PATH", "/usr/local/bin/program.tar.gz")
	defer os.Unsetenv("GOSH_PE_PATH")

	cases := []struct {
		input    string
		expected string
	}{
		{"${GOSH_PE_PATH#*/}", "usr/local/bin/program.tar.gz"},
		{"${GOSH_PE_PATH##*/}", "program.tar.gz"},
		{"${GOSH_PE_PATH%.*}", "/usr/local/bin/program.tar"},
		{"${GOSH_PE_PATH%%.*}", "/usr/local/bin/program"},
		{"${GOSH_PE_PATH#/usr/}", "local/bin/program.tar.gz"},
		{"${GOSH_PE_PATH%.gz}", "/usr/local/bin/program.tar"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ExpandSpecialVariablesE(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParameterExpansion_Substitution(t *testing.T) {
	os.Setenv("GOSH_PE_SUB", "foo bar foo baz")
	defer os.Unsetenv("GOSH_PE_SUB")

	cases := []struct {
		input    string
		expected string
	}{
		{"${GOSH_PE_SUB/foo/qux}", "qux bar foo baz"},
		{"${GOSH_PE_SUB//foo/qux}", "qux bar qux baz"},
		{"${GOSH_PE_SUB/#foo/qux}", "qux bar foo baz"},
		{"${GOSH_PE_SUB/%baz/qux}", "foo bar foo qux"},
		{"${GOSH_PE_SUB//bar/}", "foo  foo baz"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ExpandSpecialVariablesE(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParameterExpansion_CaseModification(t *testing.T) {
	os.Setenv("GOSH_PE_CASE", "hello WORLD")
	defer os.Unsetenv("GOSH_PE_CASE")

	cases := []struct {
		input    string
		expected string
	}{
		{"${GOSH_PE_CASE^}", "Hello WORLD"},
		{"${GOSH_PE_CASE^^}", "HELLO WORLD"},
		{"${GOSH_PE_CASE,}", "hello WORLD"}, // already lowercase first
		{"${GOSH_PE_CASE,,}", "hello world"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ExpandSpecialVariablesE(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}

	os.Setenv("GOSH_PE_CASE", "MIXED case")
	got, err := ExpandSpecialVariablesE("${GOSH_PE_CASE,}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mIXED case" {
		t.Errorf("got %q, want %q", got, "mIXED case")
	}
}

func TestParameterExpansion_NestedAndPositional(t *testing.T) {
	state := GetGlobalState()
	state.SetPositionalParams([]string{"alpha", "beta", "gamma"})
	defer state.SetPositionalParams(nil)

	cases := []struct {
		input    string
		expected string
	}{
		{"${1}", "alpha"},
		{"${#1}", "5"},
		{"${1:0:3}", "alp"},
		{"${10:-fallback}", "fallback"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ExpandSpecialVariablesE(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParameterExpansion_BalancedBraces(t *testing.T) {
	os.Setenv("GOSH_PE_BB", "abcdef")
	defer os.Unsetenv("GOSH_PE_BB")

	// Replacement contains a literal brace pair (no nesting hazard since they
	// balance).
	got, err := ExpandSpecialVariablesE("${GOSH_PE_BB/abc/{x}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "{x}def" {
		t.Errorf("got %q, want %q", got, "{x}def")
	}
}
