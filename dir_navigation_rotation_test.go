package gosh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushdRotation(t *testing.T) {
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

	// Build a stack: tempDir, dir1, dir2, dir3
	gs.PushDir(dir1)
	gs.PushDir(dir2)
	gs.PushDir(dir3)

	// Test pushd +1 (should rotate stack by 1)
	commandStr := "pushd +1"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	output := &strings.Builder{}
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := pushd(cmd)
	if err != nil {
		t.Fatalf("pushd +1 failed: %v", err)
	}

	// Check we're in the rotated directory (should be dir1)
	cwd, _ := os.Getwd()
	if !strings.HasSuffix(cwd, "dir1") {
		t.Errorf("Expected to be in dir1 after pushd +1, but in %s", cwd)
	}

	// Check stack order is rotated
	stack := gs.GetDirStack()
	if len(stack) != 4 {
		t.Fatalf("Expected stack length 4, got %d", len(stack))
	}

	// After rotation by 1, order should be: dir1, dir2, dir3, tempDir
	if !strings.HasSuffix(stack[0], "dir1") {
		t.Errorf("Expected dir1 at position 0, got %s", stack[0])
	}
	if !strings.HasSuffix(stack[1], "dir2") {
		t.Errorf("Expected dir2 at position 1, got %s", stack[1])
	}
	if !strings.HasSuffix(stack[2], "dir3") {
		t.Errorf("Expected dir3 at position 2, got %s", stack[2])
	}
	if stack[3] != tempDir {
		t.Errorf("Expected tempDir at position 3, got %s", stack[3])
	}

	// Test pushd -1 (should rotate backwards)
	commandStr = "pushd -1"
	cmd, _ = NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err = pushd(cmd)
	if err != nil {
		t.Fatalf("pushd -1 failed: %v", err)
	}

	// Should be back in tempDir
	cwd, _ = os.Getwd()
	if cwd != tempDir {
		t.Errorf("Expected to be in tempDir after pushd -1, but in %s", cwd)
	}
}

func TestPopdWithIndex(t *testing.T) {
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

	// Build a stack: tempDir, dir1, dir2, dir3
	gs.PushDir(dir1)
	gs.PushDir(dir2)
	gs.PushDir(dir3)

	// Test popd +2 (should remove dir2 from stack)
	commandStr := "popd +2"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	output := &strings.Builder{}
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := popd(cmd)
	if err != nil {
		t.Fatalf("popd +2 failed: %v", err)
	}

	// Should still be in tempDir
	cwd, _ := os.Getwd()
	if cwd != tempDir {
		t.Errorf("Expected to still be in tempDir after popd +2, but in %s", cwd)
	}

	// Check stack no longer contains dir2
	stack := gs.GetDirStack()
	if len(stack) != 3 {
		t.Fatalf("Expected stack length 3 after popd +2, got %d", len(stack))
	}

	// Check dir2 is removed
	for _, dir := range stack {
		if strings.HasSuffix(dir, "dir2") {
			t.Errorf("dir2 should have been removed from stack, but found: %s", dir)
		}
	}

	// Test popd +0 (should act like normal popd)
	// The stack currently has [tempDir, dir1, dir3] after removing dir2
	// We're in tempDir, so popd +0 should remove tempDir and change to dir1

	commandStr = "popd +0"
	cmd, _ = NewCommand(commandStr, NewJobManager())
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err = popd(cmd)
	if err != nil {
		t.Fatalf("popd +0 failed: %v", err)
	}

	// Should be in the new top directory (dir1)
	cwd, _ = os.Getwd()
	if !strings.HasSuffix(cwd, "dir1") {
		t.Errorf("Expected to be in dir1 after popd +0, but in %s", cwd)
	}
}

func TestPopdNegativeIndex(t *testing.T) {
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

	// Build a stack: tempDir, dir1, dir2, dir3
	gs.PushDir(dir1)
	gs.PushDir(dir2)
	gs.PushDir(dir3)

	// Test popd -1 (should remove the last element)
	commandStr := "popd -1"
	cmd, _ := NewCommand(commandStr, NewJobManager())
	output := &strings.Builder{}
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := popd(cmd)
	if err != nil {
		t.Fatalf("popd -1 failed: %v", err)
	}

	// Should still be in tempDir
	cwd, _ := os.Getwd()
	if cwd != tempDir {
		t.Errorf("Expected to still be in tempDir after popd -1, but in %s", cwd)
	}

	// Check stack no longer contains dir3
	stack := gs.GetDirStack()
	if len(stack) != 3 {
		t.Fatalf("Expected stack length 3 after popd -1, got %d", len(stack))
	}

	// Check dir3 is removed
	for _, dir := range stack {
		if strings.HasSuffix(dir, "dir3") {
			t.Errorf("dir3 should have been removed from stack, but found: %s", dir)
		}
	}
}
