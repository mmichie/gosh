package gosh

import (
	"strings"
	"testing"
)

func TestShellArrayIndexedBasics(t *testing.T) {
	a := NewIndexedArray()
	if a.Length() != 0 {
		t.Errorf("new array length: got %d want 0", a.Length())
	}
	if err := a.Set("0", "alpha"); err != nil {
		t.Fatalf("set 0: %v", err)
	}
	if err := a.Set("1", "beta"); err != nil {
		t.Fatalf("set 1: %v", err)
	}
	if err := a.Set("5", "epsilon"); err != nil {
		t.Fatalf("set 5: %v", err)
	}
	if v, ok := a.Get("0"); !ok || v != "alpha" {
		t.Errorf("get 0: got (%q,%v)", v, ok)
	}
	if v, ok := a.Get("5"); !ok || v != "epsilon" {
		t.Errorf("get 5: got (%q,%v)", v, ok)
	}
	if _, ok := a.Get("99"); ok {
		t.Errorf("get 99: should be unset")
	}
	if a.Length() != 3 {
		t.Errorf("length: got %d want 3", a.Length())
	}
	values := a.Values()
	if strings.Join(values, ",") != "alpha,beta,epsilon" {
		t.Errorf("values: got %v", values)
	}
}

func TestShellArrayAppend(t *testing.T) {
	a := NewIndexedArray()
	a.Append("x", "y", "z")
	if a.Length() != 3 {
		t.Errorf("length after append: got %d want 3", a.Length())
	}
	if v, _ := a.Get("0"); v != "x" {
		t.Errorf("first: got %q want x", v)
	}
	if v, _ := a.Get("2"); v != "z" {
		t.Errorf("last: got %q want z", v)
	}
	// Append after a sparse set picks max+1.
	_ = a.Set("10", "ten")
	a.Append("more")
	if v, _ := a.Get("11"); v != "more" {
		t.Errorf("append after sparse: got %q want more", v)
	}
}

func TestShellArrayAssociative(t *testing.T) {
	a := NewAssociativeArray()
	if err := a.Set("name", "gosh"); err != nil {
		t.Fatal(err)
	}
	if err := a.Set("kind", "shell"); err != nil {
		t.Fatal(err)
	}
	if v, ok := a.Get("name"); !ok || v != "gosh" {
		t.Errorf("get name: got (%q,%v)", v, ok)
	}
	if _, ok := a.Get("missing"); ok {
		t.Errorf("get missing: should be unset")
	}
	if a.Length() != 2 {
		t.Errorf("length: got %d want 2", a.Length())
	}
}

func TestParseArrayLiteral(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{"one two three", []string{"one", "two", "three"}},
		{"  spaced   words  ", []string{"spaced", "words"}},
		{`"with space" plain`, []string{"with space", "plain"}},
		{`'single quoted' next`, []string{"single quoted", "next"}},
		{`a\ b c`, []string{"a b", "c"}},
		{"", nil},
	}
	for _, c := range cases {
		got, err := ParseArrayLiteral(c.body)
		if err != nil {
			t.Fatalf("ParseArrayLiteral(%q): %v", c.body, err)
		}
		if len(got) != len(c.want) {
			t.Errorf("ParseArrayLiteral(%q): got %v want %v", c.body, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseArrayLiteral(%q) elem %d: got %q want %q", c.body, i, got[i], c.want[i])
			}
		}
	}
}

func TestPreprocessArrayAssignments(t *testing.T) {
	gs := GetGlobalState()
	gs.UnsetArray("colors")
	gs.UnsetArray("empty")

	cases := []struct {
		name    string
		input   string
		arr     string
		want    []string
		remains string
	}{
		{
			name:    "basic literal",
			input:   "colors=(red green blue)",
			arr:     "colors",
			want:    []string{"red", "green", "blue"},
			remains: ":",
		},
		{
			name:    "empty",
			input:   "empty=()",
			arr:     "empty",
			want:    nil,
			remains: ":",
		},
		{
			name:    "followed by another command",
			input:   "colors=(a b); echo done",
			arr:     "colors",
			want:    []string{"a", "b"},
			remains: ":; echo done",
		},
		{
			name:    "with quoted element",
			input:   `colors=("red one" green)`,
			arr:     "colors",
			want:    []string{"red one", "green"},
			remains: ":",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs.UnsetArray(c.arr)
			got, err := PreprocessArrayAssignments(c.input)
			if err != nil {
				t.Fatalf("preprocess: %v", err)
			}
			if strings.TrimSpace(got) != c.remains {
				t.Errorf("remains: got %q want %q", got, c.remains)
			}
			arr, ok := gs.GetArray(c.arr)
			if !ok {
				t.Fatalf("array %q not registered", c.arr)
			}
			values := arr.Values()
			if len(values) != len(c.want) {
				t.Errorf("values: got %v want %v", values, c.want)
				return
			}
			for i := range values {
				if values[i] != c.want[i] {
					t.Errorf("values[%d]: got %q want %q", i, values[i], c.want[i])
				}
			}
			gs.UnsetArray(c.arr)
		})
	}
}

func TestPreprocessArrayAssignmentsNoMatch(t *testing.T) {
	cases := []string{
		"echo foo",
		"FOO=bar",
		"FOO=bar echo",
	}
	for _, in := range cases {
		got, err := PreprocessArrayAssignments(in)
		if err != nil {
			t.Errorf("%q: unexpected err %v", in, err)
		}
		if got != in {
			t.Errorf("%q: should be unchanged, got %q", in, got)
		}
	}
}

func TestApplyIndexedAssignment(t *testing.T) {
	gs := GetGlobalState()
	gs.UnsetArray("arr")
	done, err := ApplyIndexedAssignment("arr[3]=value")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !done {
		t.Fatalf("not applied")
	}
	v, ok := gs.GetArrayElement("arr", "3")
	if !ok || v != "value" {
		t.Errorf("get arr[3]: (%q,%v)", v, ok)
	}

	// Non-array form returns done=false.
	done, _ = ApplyIndexedAssignment("FOO=bar")
	if done {
		t.Errorf("FOO=bar should not be array assignment")
	}
}

func TestArrayExpansion(t *testing.T) {
	gs := GetGlobalState()
	a := NewIndexedArray()
	a.Append("alpha", "beta", "gamma")
	gs.SetArray("words", a)
	defer gs.UnsetArray("words")

	cases := []struct {
		in   string
		want string
	}{
		{"${words[0]}", "alpha"},
		{"${words[1]}", "beta"},
		{"${words[@]}", "alpha beta gamma"},
		{"${words[*]}", "alpha beta gamma"},
		{"${#words[@]}", "3"},
		{"${#words[0]}", "5"}, // len("alpha")
		{"${words[@]:1}", "beta gamma"},
		{"${words[@]:1:1}", "beta"},
		{"${words[@]:0:2}", "alpha beta"},
		{"prefix-${words[1]}-suffix", "prefix-beta-suffix"},
	}
	for _, c := range cases {
		got, err := ExpandSpecialVariablesE(c.in)
		if err != nil {
			t.Errorf("expand %q: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("expand %q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestArrayExpansionAssoc(t *testing.T) {
	gs := GetGlobalState()
	a := NewAssociativeArray()
	_ = a.Set("red", "ff0000")
	_ = a.Set("green", "00ff00")
	gs.SetArray("colors", a)
	defer gs.UnsetArray("colors")

	cases := []struct {
		in   string
		want string
	}{
		{"${colors[red]}", "ff0000"},
		{"${colors[green]}", "00ff00"},
		{"${colors[missing]}", ""},
		{"${#colors[@]}", "2"},
	}
	for _, c := range cases {
		got, err := ExpandSpecialVariablesE(c.in)
		if err != nil {
			t.Errorf("expand %q: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("expand %q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestArraySubscriptArithmetic(t *testing.T) {
	gs := GetGlobalState()
	a := NewIndexedArray()
	a.Append("a", "b", "c", "d")
	gs.SetArray("items", a)
	defer gs.UnsetArray("items")
	t.Setenv("IDX", "2")

	got, err := ExpandSpecialVariablesE("${items[1+1]}")
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got != "c" {
		t.Errorf("items[1+1]: got %q want c", got)
	}
}
