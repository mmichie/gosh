package gosh

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestSubshellAndGroupInPipeline verifies that subshells and command groups
// can act as stages inside a multi-command pipeline (gosh-095d).
func TestSubshellAndGroupInPipeline(t *testing.T) {
	// Some sibling tests leave GlobalState.CWD pointing at a removed temp dir.
	// Pin both the process cwd and GlobalState to a known-good directory so
	// pipeline stages that spawn external commands have a valid working dir.
	gs := GetGlobalState()
	origGSCWD := gs.GetCWD()
	origCWD, _ := os.Getwd()
	wd := os.TempDir()
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("could not chdir to %s: %v", wd, err)
	}
	gs.UpdateCWD(wd)
	defer func() {
		if origCWD != "" {
			os.Chdir(origCWD)
		}
		gs.UpdateCWD(origGSCWD)
	}()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "subshell as first stage produces byte count",
			input: "( echo hello ) | wc -c",
			want:  "6",
		},
		{
			name:  "command group as first stage filtered by grep",
			input: "{ echo a; echo b; } | grep a",
			want:  "a",
		},
		{
			name:  "subshell emits two lines counted by wc -l",
			input: "( echo x; echo y ) | wc -l",
			want:  "2",
		},
		{
			name:  "subshell as last stage passes input through cat",
			input: "echo input | ( cat )",
			want:  "input",
		},
		{
			name:  "command group as last stage passes input through cat",
			input: "echo input | { cat; }",
			want:  "input",
		},
		{
			name:  "three-stage subshell to cat to wc -c",
			input: "( echo hello ) | cat | wc -c",
			want:  "6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCommand(tt.input, nil)
			if err != nil {
				t.Fatalf("NewCommand failed: %v", err)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			cmd.Run()

			if cmd.ReturnCode != 0 {
				t.Errorf("ReturnCode = %d, want 0 (stderr: %q)", cmd.ReturnCode, stderr.String())
			}

			if strings.Contains(stderr.String(), "not yet supported") {
				t.Errorf("regression: pipeline reported unsupported stage; stderr: %q", stderr.String())
			}

			got := strings.TrimSpace(stdout.String())
			if got != tt.want {
				t.Errorf("output = %q, want %q (stderr: %q)", got, tt.want, stderr.String())
			}
		})
	}
}
