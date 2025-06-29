package gosh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushdPopd(t *testing.T) {
	// Create test directories
	tempDir := t.TempDir()
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")
	dir3 := filepath.Join(tempDir, "dir3")

	os.Mkdir(dir1, 0755)
	os.Mkdir(dir2, 0755)
	os.Mkdir(dir3, 0755)

	// Save original directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Change to temp directory
	os.Chdir(tempDir)

	// Initialize global state
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)
	gs.ResetDirStack()

	// Test pushd to dir1
	commandStr := "pushd dir1"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	output := &strings.Builder{}
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := pushd(cmd)
	if err != nil {
		t.Fatalf("pushd failed: %v", err)
	}

	// Check we're in dir1
	cwd, _ := os.Getwd()
	if !strings.HasSuffix(cwd, "dir1") {
		t.Errorf("Expected to be in dir1, but in %s", cwd)
	}

	// Check stack has two entries
	stack := gs.GetDirStack()
	if len(stack) != 2 {
		t.Errorf("Expected stack length 2, got %d", len(stack))
	}

	// Test pushd to dir2
	commandStr = "pushd ../dir2"
	cmd, _ = NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	err = pushd(cmd)
	if err != nil {
		t.Fatalf("pushd failed: %v", err)
	}

	// Check we're in dir2
	cwd, _ = os.Getwd()
	if !strings.HasSuffix(cwd, "dir2") {
		t.Errorf("Expected to be in dir2, but in %s", cwd)
	}

	// Check stack has three entries
	stack = gs.GetDirStack()
	if len(stack) != 3 {
		t.Errorf("Expected stack length 3, got %d", len(stack))
	}

	// Test popd
	commandStr = "popd"
	cmd, _ = NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	err = popd(cmd)
	if err != nil {
		t.Fatalf("popd failed: %v", err)
	}

	// Check we're back in dir1
	cwd, _ = os.Getwd()
	if !strings.HasSuffix(cwd, "dir1") {
		t.Errorf("Expected to be in dir1 after popd, but in %s", cwd)
	}

	// Check stack has two entries
	stack = gs.GetDirStack()
	if len(stack) != 2 {
		t.Errorf("Expected stack length 2 after popd, got %d", len(stack))
	}
}

func TestDirs(t *testing.T) {
	// Create test directories
	tempDir := t.TempDir()
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")

	os.Mkdir(dir1, 0755)
	os.Mkdir(dir2, 0755)

	// Save original directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Change to temp directory
	os.Chdir(tempDir)

	// Initialize global state
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)
	gs.ResetDirStack()

	// Push some directories
	gs.PushDir(dir1)
	gs.PushDir(dir2)

	// Test dirs command
	output := &strings.Builder{}
	commandStr := "dirs"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := dirs(cmd)
	if err != nil {
		t.Fatalf("dirs failed: %v", err)
	}

	// Check output contains all directories
	out := output.String()
	if !strings.Contains(out, tempDir) {
		t.Errorf("dirs output missing tempDir: %s", out)
	}
	if !strings.Contains(out, dir1) {
		t.Errorf("dirs output missing dir1: %s", out)
	}
	if !strings.Contains(out, dir2) {
		t.Errorf("dirs output missing dir2: %s", out)
	}

	// Test dirs -v
	output.Reset()
	commandStr = "dirs -v"
	cmd, _ = NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	err = dirs(cmd)
	if err != nil {
		t.Fatalf("dirs -v failed: %v", err)
	}

	// Check verbose output has indices
	out = output.String()
	if !strings.Contains(out, "0\t") {
		t.Errorf("dirs -v output missing index 0: %s", out)
	}
	if !strings.Contains(out, "1\t") {
		t.Errorf("dirs -v output missing index 1: %s", out)
	}
	if !strings.Contains(out, "2\t") {
		t.Errorf("dirs -v output missing index 2: %s", out)
	}
}

func TestCDPATH(t *testing.T) {
	// Create test directories
	tempDir := t.TempDir()
	parentDir := filepath.Join(tempDir, "parent")
	subDir := filepath.Join(parentDir, "subdir")
	otherDir := filepath.Join(tempDir, "other")

	os.Mkdir(parentDir, 0755)
	os.Mkdir(subDir, 0755)
	os.Mkdir(otherDir, 0755)

	// Save original directory and CDPATH
	origDir, _ := os.Getwd()
	origCDPATH := os.Getenv("CDPATH")
	defer func() {
		os.Chdir(origDir)
		os.Setenv("CDPATH", origCDPATH)
	}()

	// Set CDPATH
	os.Setenv("CDPATH", parentDir)

	// Change to other directory
	os.Chdir(otherDir)

	// Initialize global state
	gs := GetGlobalState()
	gs.UpdateCWD(otherDir)
	gs.ResetDirStack()

	// Test cd with CDPATH
	output := &strings.Builder{}
	commandStr := "cd subdir"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := cd(cmd)
	if err != nil {
		t.Fatalf("cd with CDPATH failed: %v", err)
	}

	// Check we're in subdir
	cwd, _ := os.Getwd()
	if !strings.HasSuffix(cwd, "subdir") {
		t.Errorf("Expected to be in subdir, but in %s", cwd)
	}

	// Check that cd printed the directory
	out := output.String()
	if !strings.Contains(out, subDir) {
		t.Errorf("cd with CDPATH should print target directory, got: %s", out)
	}

	// Test cd with multiple CDPATH entries
	os.Setenv("CDPATH", tempDir+":"+parentDir)
	os.Chdir(tempDir)

	output.Reset()
	err = cd(cmd)
	if err != nil {
		t.Fatalf("cd with multiple CDPATH failed: %v", err)
	}

	// Should still find subdir in parentDir
	cwd, _ = os.Getwd()
	if !strings.HasSuffix(cwd, "subdir") {
		t.Errorf("Expected to be in subdir with multiple CDPATH, but in %s", cwd)
	}
}

func TestPushdNoArgs(t *testing.T) {
	// Create test directories
	tempDir := t.TempDir()
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")

	os.Mkdir(dir1, 0755)
	os.Mkdir(dir2, 0755)

	// Save original directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Change to tempDir first
	os.Chdir(tempDir)

	// Initialize global state
	gs := GetGlobalState()
	gs.UpdateCWD(tempDir)
	gs.ResetDirStack()

	// Now change to dir1 and push dir2
	os.Chdir(dir1)
	gs.UpdateCWD(dir1)
	gs.PushDir(dir2)

	// Check the stack before pushd
	stack := gs.GetDirStack()
	t.Logf("Stack before pushd: %v", stack)

	// Test pushd with no args (should swap top two)
	commandStr := "pushd"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	output := &strings.Builder{}
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := pushd(cmd)
	if err != nil {
		t.Fatalf("pushd with no args failed: %v", err)
	}

	// Check we're in dir2
	cwd, _ := os.Getwd()
	if !strings.HasSuffix(cwd, "dir2") {
		t.Errorf("Expected to be in dir2 after pushd, but in %s", cwd)
	}

	// Check stack order is swapped
	stack = gs.GetDirStack()
	if len(stack) < 2 {
		t.Fatalf("Stack too small: %d", len(stack))
	}

	if !strings.HasSuffix(stack[0], "dir2") {
		t.Errorf("Expected dir2 at top of stack, got %s", stack[0])
	}
	if !strings.HasSuffix(stack[1], "dir1") {
		t.Errorf("Expected dir1 at position 1, got %s", stack[1])
	}
}
