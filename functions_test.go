package gosh

import (
	"bytes"
	"strings"
	"testing"
)

func TestPreprocessFunctionDefinitions(t *testing.T) {
	gs := GetGlobalState()
	clear := func(name string) { gs.UnsetFunction(name) }

	cases := []struct {
		name    string
		input   string
		fnName  string
		body    string
		remains string
	}{
		{
			name:    "paren form, single line",
			input:   "greet() { echo hi; }",
			fnName:  "greet",
			body:    "echo hi;",
			remains: ":",
		},
		{
			name:    "function keyword, no parens",
			input:   "function greet { echo hi; }",
			fnName:  "greet",
			body:    "echo hi;",
			remains: ":",
		},
		{
			name:    "function keyword with parens",
			input:   "function greet() { echo hi; }",
			fnName:  "greet",
			body:    "echo hi;",
			remains: ":",
		},
		{
			name:    "definition followed by call",
			input:   "greet() { echo hi; }; greet",
			fnName:  "greet",
			body:    "echo hi;",
			remains: ":; greet",
		},
		{
			name:    "nested braces in body",
			input:   "f() { if true; then { echo nested; }; fi; }",
			fnName:  "f",
			body:    "if true; then { echo nested; }; fi;",
			remains: ":",
		},
		{
			name:    "braces inside double quotes",
			input:   `f() { echo "}"; }`,
			fnName:  "f",
			body:    `echo "}";`,
			remains: ":",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clear(c.fnName)
			got, err := PreprocessFunctionDefinitions(c.input)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if strings.TrimSpace(got) != c.remains {
				t.Errorf("remains: got %q want %q", got, c.remains)
			}
			body, ok := gs.GetFunction(c.fnName)
			if !ok {
				t.Fatalf("function %q not registered", c.fnName)
			}
			if body != c.body {
				t.Errorf("body: got %q want %q", body, c.body)
			}
			clear(c.fnName)
		})
	}
}

func TestPreprocessFunctionDefinitionsNoMatch(t *testing.T) {
	gs := GetGlobalState()
	cases := []string{
		"echo foo",
		"x=5 echo $x",
		"true; false",
		"not-a-def() echo broken",       // missing braces
		"function () { echo nameless }", // missing name
	}
	for _, in := range cases {
		gs.UnsetFunction("foo")
		got, err := PreprocessFunctionDefinitions(in)
		if err != nil {
			t.Errorf("%q: unexpected err %v", in, err)
		}
		if got != in {
			t.Errorf("%q: should be unchanged, got %q", in, got)
		}
	}
}

func TestPreprocessFunctionDefinitionsUnclosed(t *testing.T) {
	_, err := PreprocessFunctionDefinitions("f() { echo hi")
	if err == nil {
		t.Errorf("expected error for unclosed body")
	}
}

func TestSplitFunctionBody(t *testing.T) {
	cases := []struct {
		body  string
		parts []string
	}{
		{"echo a; echo b", []string{"echo a", "echo b"}},
		{"echo a\necho b", []string{"echo a", "echo b"}},
		{"echo 'a; b'; echo c", []string{"echo 'a; b'", "echo c"}},
		{"{ echo a; echo b; }; echo c", []string{"{ echo a; echo b; }", "echo c"}},
		{`echo "x;y"; echo z`, []string{`echo "x;y"`, "echo z"}},
	}
	for _, c := range cases {
		got := splitFunctionBody(c.body)
		if len(got) != len(c.parts) {
			t.Errorf("split %q -> %v, want %v", c.body, got, c.parts)
			continue
		}
		for i := range got {
			if got[i] != c.parts[i] {
				t.Errorf("split %q part %d: got %q want %q", c.body, i, got[i], c.parts[i])
			}
		}
	}
}

func TestCallUserFunctionPositionalParams(t *testing.T) {
	gs := GetGlobalState()
	gs.SetFunction("echoArgs", `echo "args: $#"; echo "first: $1"; echo "all: $@"`)
	defer gs.UnsetFunction("echoArgs")

	var out bytes.Buffer
	cmd := &Command{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, JobManager: NewJobManager()}
	if err := CallUserFunction("echoArgs", []string{"alpha", "beta", "gamma"}, cmd); err != nil {
		t.Fatalf("call error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"args: 3", "first: alpha", "all: alpha beta gamma"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %s", want, got)
		}
	}
}

func TestCallUserFunctionLocalShadow(t *testing.T) {
	gs := GetGlobalState()
	gs.SetFunction("shadow", `local x=inner; echo "in: $x"`)
	defer gs.UnsetFunction("shadow")

	// Set outer value via env so lookup sees it both before and after.
	t.Setenv("X_SHADOW_TEST", "outer")
	// the function uses literal "x" though — use a unique env var
	gs.SetFunction("shadow", `local SHADOWVAR=inner; echo "in: $SHADOWVAR"`)
	t.Setenv("SHADOWVAR", "outer")

	var out bytes.Buffer
	cmd := &Command{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, JobManager: NewJobManager()}
	if err := CallUserFunction("shadow", nil, cmd); err != nil {
		t.Fatalf("call error: %v", err)
	}
	if !strings.Contains(out.String(), "in: inner") {
		t.Errorf("expected inner value, got: %s", out.String())
	}
}

func TestCallUserFunctionReturnCode(t *testing.T) {
	gs := GetGlobalState()
	gs.SetFunction("retSeven", `return 7; echo "should not print"`)
	defer gs.UnsetFunction("retSeven")

	var out bytes.Buffer
	cmd := &Command{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, JobManager: NewJobManager()}
	if err := CallUserFunction("retSeven", nil, cmd); err != nil {
		t.Fatalf("call error: %v", err)
	}
	if cmd.ReturnCode != 7 {
		t.Errorf("return code: got %d want 7", cmd.ReturnCode)
	}
	if strings.Contains(out.String(), "should not print") {
		t.Errorf("statements after return executed: %s", out.String())
	}
}

func TestCallUserFunctionNested(t *testing.T) {
	gs := GetGlobalState()
	gs.SetFunction("inner", `echo "inner: $1"`)
	gs.SetFunction("outer", `inner "$1-wrapped"`)
	defer gs.UnsetFunction("inner")
	defer gs.UnsetFunction("outer")

	var out bytes.Buffer
	cmd := &Command{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, JobManager: NewJobManager()}
	if err := CallUserFunction("outer", []string{"hello"}, cmd); err != nil {
		t.Fatalf("call error: %v", err)
	}
	if !strings.Contains(out.String(), "inner: hello-wrapped") {
		t.Errorf("nested call output: %s", out.String())
	}
}

func TestIsValidFunctionName(t *testing.T) {
	good := []string{"foo", "_foo", "foo123", "foo-bar", "a.b"}
	bad := []string{"", "1foo", "foo bar", "foo$bar"}
	for _, n := range good {
		if !IsValidFunctionName(n) {
			t.Errorf("%q should be valid", n)
		}
	}
	for _, n := range bad {
		if IsValidFunctionName(n) {
			t.Errorf("%q should be invalid", n)
		}
	}
}
