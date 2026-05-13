package gosh

import "testing"

func TestExpandBracesCommaLists(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{a,b,c}", "a b c"},
		{"{a,b}", "a b"},
		{"{single}", "{single}"},
		{"{}", "{}"},
		{"{,}", " "},
		{"{a,}", "a "},
		{"{,b}", " b"},
		{"echo {a,b,c}", "echo a b c"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesNumericSequence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{1..5}", "1 2 3 4 5"},
		{"{5..1}", "5 4 3 2 1"},
		{"{10..1}", "10 9 8 7 6 5 4 3 2 1"},
		{"{0..10..2}", "0 2 4 6 8 10"},
		{"{10..0..-3}", "10 7 4 1"},
		{"{10..0..3}", "10 7 4 1"},
		{"{-2..2}", "-2 -1 0 1 2"},
		{"{3..3}", "3"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesZeroPadded(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{01..05}", "01 02 03 04 05"},
		{"{01..10}", "01 02 03 04 05 06 07 08 09 10"},
		{"{001..003}", "001 002 003"},
		{"{1..05}", "01 02 03 04 05"},
		{"{05..1}", "05 04 03 02 01"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesCharSequence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{a..e}", "a b c d e"},
		{"{e..a}", "e d c b a"},
		{"{A..E}", "A B C D E"},
		{"{Z..X}", "Z Y X"},
		{"{a..z..3}", "a d g j m p s v y"},
		{"{a..a}", "a"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesPreamblePostscript(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"pre{a,b}post", "preapost prebpost"},
		{"foo{1..3}.txt", "foo1.txt foo2.txt foo3.txt"},
		{"x{a,b,c}y", "xay xby xcy"},
		{"file_{01..03}.log", "file_01.log file_02.log file_03.log"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesCartesian(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{a,b}{1,2}", "a1 a2 b1 b2"},
		{"{a,b}-{x,y}", "a-x a-y b-x b-y"},
		{"{1..2}{a..b}", "1a 1b 2a 2b"},
		{"x{a,b}y{1,2}z", "xay1z xay2z xby1z xby2z"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesNested(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{a,b{1,2}}", "a b1 b2"},
		{"{{a,b},c}", "a b c"},
		{"{a,{b,c}d}", "a bd cd"},
		{"{a,b{c,d}e}", "a bce bde"},
		{"{{1..3},x}", "1 2 3 x"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesNonExpansionsLeftAlone(t *testing.T) {
	cases := []string{
		"{abc}",
		"{single}",
		"${var}",
		"${var:-default}",
		"$((1+2))",
		"'{a,b}'",
		`"{a,b}"`,
		"$(echo {a,b})",
		"{a..}",
		"{..b}",
		"{1..b}",
		"{a..2}",
	}
	for _, in := range cases {
		got := ExpandBraces(in)
		if got != in {
			t.Errorf("ExpandBraces(%q) should be unchanged, got %q", in, got)
		}
	}
}

func TestExpandBracesMixed(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"echo foo{1..3} bar", "echo foo1 foo2 foo3 bar"},
		{"cp file.{txt,bak} /tmp/", "cp file.txt file.bak /tmp/"},
		{"echo {a,b}; echo {1,2}", "echo a b; echo 1 2"},
		{"ls a/{x,y}/b", "ls a/x/b a/y/b"},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesQuotesAndDollarSafety(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`echo "{a,b}"`, `echo "{a,b}"`},
		{"echo '{a,b}'", "echo '{a,b}'"},
		{"echo ${x:-{a,b}}", "echo ${x:-{a,b}}"},
		{"echo $((1+2)) {a,b}", "echo $((1+2)) a b"},
		{"echo {a,b} ${var}", "echo a b ${var}"},
		{`echo {$x,$y}`, `echo $x $y`},
	}
	for _, c := range cases {
		got := ExpandBraces(c.in)
		if got != c.want {
			t.Errorf("ExpandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandBracesUnmatched(t *testing.T) {
	cases := []string{
		"{a,b",
		"a,b}",
		"{",
		"}",
		"{{a,b}",
	}
	for _, in := range cases {
		got := ExpandBraces(in)
		if in == "{{a,b}" {
			// Inner `{a,b}` is a valid group; outer `{` is left alone.
			want := "{a {b"
			if got != want {
				t.Errorf("ExpandBraces(%q) = %q, want %q", in, got, want)
			}
			continue
		}
		if got != in {
			t.Errorf("ExpandBraces(%q) should be unchanged, got %q", in, got)
		}
	}
}

func TestExpandBracesEndToEndCommand(t *testing.T) {
	cmd, err := NewCommand("echo foo{1..3}", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}
	if len(cmd.Command.LogicalBlocks) != 1 {
		t.Fatalf("expected 1 logical block, got %d", len(cmd.Command.LogicalBlocks))
	}
	pipeline := cmd.Command.LogicalBlocks[0].FirstPipeline
	if pipeline == nil {
		t.Fatalf("nil pipeline")
	}
	if len(pipeline.Commands) != 1 {
		t.Fatalf("expected 1 simple command, got %d", len(pipeline.Commands))
	}
	parts := getCommandParts(pipeline.Commands[0])
	want := []string{"echo", "foo1", "foo2", "foo3"}
	if len(parts) != len(want) {
		t.Fatalf("parts: got %v want %v", parts, want)
	}
	for i := range parts {
		if parts[i] != want[i] {
			t.Errorf("parts[%d]: got %q want %q", i, parts[i], want[i])
		}
	}
}

func TestExpandBracesEndToEndCartesian(t *testing.T) {
	cmd, err := NewCommand("echo {a,b}{1,2}", NewJobManager())
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}
	parts := getCommandParts(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands[0])
	want := []string{"echo", "a1", "a2", "b1", "b2"}
	if len(parts) != len(want) {
		t.Fatalf("parts: got %v want %v", parts, want)
	}
	for i := range parts {
		if parts[i] != want[i] {
			t.Errorf("parts[%d]: got %q want %q", i, parts[i], want[i])
		}
	}
}
