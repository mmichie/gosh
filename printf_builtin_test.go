package gosh

import (
	"bytes"
	"strings"
	"testing"

	"gosh/parser"
)

func TestPrintfCommand(t *testing.T) {
	tests := []struct {
		name       string
		cmd        string
		wantOutput string
		wantCode   int
	}{
		{
			name:       "basic string",
			cmd:        `printf "Hello, World!"`,
			wantOutput: "Hello, World!",
			wantCode:   0,
		},
		{
			name:       "string with newline escape",
			cmd:        `printf "Hello\nWorld"`,
			wantOutput: "Hello\nWorld",
			wantCode:   0,
		},
		{
			name:       "string with tab escape",
			cmd:        `printf "Name:\tValue"`,
			wantOutput: "Name:\tValue",
			wantCode:   0,
		},
		{
			name:       "%s format specifier",
			cmd:        `printf "Hello %s" World`,
			wantOutput: "Hello World",
			wantCode:   0,
		},
		{
			name:       "%d decimal format",
			cmd:        `printf "Number: %d" 42`,
			wantOutput: "Number: 42",
			wantCode:   0,
		},
		{
			name:       "%f float format",
			cmd:        `printf "Pi: %.2f" 3.14159`,
			wantOutput: "Pi: 3.14",
			wantCode:   0,
		},
		{
			name:       "%x hexadecimal format",
			cmd:        `printf "Hex: %x" 255`,
			wantOutput: "Hex: ff",
			wantCode:   0,
		},
		{
			name:       "%X hexadecimal uppercase",
			cmd:        `printf "Hex: %X" 255`,
			wantOutput: "Hex: FF",
			wantCode:   0,
		},
		{
			name:       "%o octal format",
			cmd:        `printf "Octal: %o" 64`,
			wantOutput: "Octal: 100",
			wantCode:   0,
		},
		{
			name:       "multiple format specifiers",
			cmd:        `printf "%s is %d years old" Alice 25`,
			wantOutput: "Alice is 25 years old",
			wantCode:   0,
		},
		{
			name:       "width specifier",
			cmd:        `printf "%10s" Hello`,
			wantOutput: "     Hello",
			wantCode:   0,
		},
		{
			name:       "left-justify with width",
			cmd:        `printf "%-10s" Hello`,
			wantOutput: "Hello     ",
			wantCode:   0,
		},
		{
			name:       "zero-padded number",
			cmd:        `printf "%05d" 42`,
			wantOutput: "00042",
			wantCode:   0,
		},
		{
			name:       "precision for string",
			cmd:        `printf "%.3s" Hello`,
			wantOutput: "Hel",
			wantCode:   0,
		},
		{
			name:       "precision for float",
			cmd:        `printf "%.3f" 3.14159`,
			wantOutput: "3.142",
			wantCode:   0,
		},
		{
			name:       "width and precision",
			cmd:        `printf "%8.2f" 3.14159`,
			wantOutput: "    3.14",
			wantCode:   0,
		},
		{
			name:       "literal percent",
			cmd:        `printf "100%% complete"`,
			wantOutput: "100% complete",
			wantCode:   0,
		},
		{
			name:       "character format",
			cmd:        `printf "%c" A`,
			wantOutput: "A",
			wantCode:   0,
		},
		{
			name:       "character from ASCII code",
			cmd:        `printf "%c" 65`,
			wantOutput: "A",
			wantCode:   0,
		},
		{
			name:       "multiple escape sequences",
			cmd:        `printf "Line1\nLine2\tTabbed\rReturn"`,
			wantOutput: "Line1\nLine2\tTabbed\rReturn",
			wantCode:   0,
		},
		{
			name:       "backslash escape",
			cmd:        `printf "Path: C:\\Users\\Home"`,
			wantOutput: "Path: C:\\Users\\Home",
			wantCode:   0,
		},
		{
			name:       "quote escapes",
			cmd:        `printf "Say %s" "Hello"`,
			wantOutput: `Say Hello`,
			wantCode:   0,
		},
		{
			name:       "no arguments - just format",
			cmd:        `printf "Hello"`,
			wantOutput: "Hello",
			wantCode:   0,
		},
		{
			name:       "more values than format specs - repeat format",
			cmd:        `printf "%s\n" one two three`,
			wantOutput: "one\ntwo\nthree\n",
			wantCode:   0,
		},
		{
			name:       "format spec with no value - use default",
			cmd:        `printf "Number: %d"`,
			wantOutput: "Number: 0",
			wantCode:   0,
		},
		{
			name:       "invalid number defaults to 0",
			cmd:        `printf "%d" abc`,
			wantOutput: "0",
			wantCode:   0,
		},
		{
			name:       "signed integer format",
			cmd:        `printf "%+d" 42`,
			wantOutput: "+42",
			wantCode:   0,
		},
		{
			name:       "negative number",
			cmd:        `printf "%d" -42`,
			wantOutput: "-42",
			wantCode:   0,
		},
		{
			name:       "complex format string",
			cmd:        `printf "%-10s | %5d | %8.2f\n" Item 42 3.14`,
			wantOutput: "Item       |    42 |     3.14\n",
			wantCode:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedCmd, err := parser.Parse(tt.cmd)
			if err != nil {
				t.Fatalf("Failed to parse command: %v", err)
			}

			var stdout bytes.Buffer
			cmd := &Command{
				Command:    parsedCmd,
				Stdin:      strings.NewReader(""),
				Stdout:     &stdout,
				Stderr:     &stdout,
				JobManager: NewJobManager(),
			}

			err = printfCommand(cmd)
			if err != nil {
				t.Fatalf("printf command failed: %v", err)
			}

			if cmd.ReturnCode != tt.wantCode {
				t.Errorf("got exit code %d, want %d", cmd.ReturnCode, tt.wantCode)
			}

			gotOutput := stdout.String()
			if gotOutput != tt.wantOutput {
				t.Errorf("got output %q, want %q", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestPrintfEscapeSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"newline", `\n`, "\n"},
		{"tab", `\t`, "\t"},
		{"carriage return", `\r`, "\r"},
		{"backspace", `\b`, "\b"},
		{"alert", `\a`, "\a"},
		{"form feed", `\f`, "\f"},
		{"vertical tab", `\v`, "\v"},
		{"backslash", `\\`, "\\"},
		{"double quote", `\"`, "\""},
		{"single quote", `\'`, "'"},
		{"multiple escapes", `\n\t\r`, "\n\t\r"},
		{"mixed text and escapes", `Hello\nWorld\t!`, "Hello\nWorld\t!"},
		{"no escape", `plain text`, "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processEscapeSequences(tt.input)
			if got != tt.want {
				t.Errorf("processEscapeSequences(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindFormatSpecifiers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // Number of format specifiers expected
	}{
		{"no specifiers", "plain text", 0},
		{"single %s", "Hello %s", 1},
		{"multiple specifiers", "%s is %d years old", 2},
		{"with width", "%10s", 1},
		{"with precision", "%.2f", 1},
		{"with width and precision", "%8.2f", 1},
		{"with flags", "%+d", 1},
		{"literal percent", "100%%", 1},
		{"complex", "%s: %+5.2f%%", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := findFormatSpecifiers(tt.input)
			if len(specs) != tt.want {
				t.Errorf("findFormatSpecifiers(%q) found %d specifiers, want %d", tt.input, len(specs), tt.want)
			}
		})
	}
}

func TestPrintfErrors(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{
			name:    "no arguments",
			cmd:     "printf",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedCmd, err := parser.Parse(tt.cmd)
			if err != nil {
				t.Fatalf("Failed to parse command: %v", err)
			}

			var stdout bytes.Buffer
			cmd := &Command{
				Command:    parsedCmd,
				Stdin:      strings.NewReader(""),
				Stdout:     &stdout,
				Stderr:     &stdout,
				JobManager: NewJobManager(),
			}

			err = printfCommand(cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("printfCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrintfRepeatedFormat(t *testing.T) {
	// Test that printf repeats the format string for extra arguments
	parsedCmd, err := parser.Parse(`printf "%s:%d\n" a 1 b 2 c 3`)
	if err != nil {
		t.Fatalf("Failed to parse command: %v", err)
	}

	var stdout bytes.Buffer
	cmd := &Command{
		Command:    parsedCmd,
		Stdin:      strings.NewReader(""),
		Stdout:     &stdout,
		Stderr:     &stdout,
		JobManager: NewJobManager(),
	}

	err = printfCommand(cmd)
	if err != nil {
		t.Fatalf("printf command failed: %v", err)
	}

	want := "a:1\nb:2\nc:3\n"
	got := stdout.String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
