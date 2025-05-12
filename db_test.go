package gosh

import (
	"os"
	"testing"
	"time"
)

// TestDatabase tests the basic functionality of the database
func TestDatabase(t *testing.T) {
	// Create a temporary database file
	dbPath := "/tmp/gosh_db_test.db"
	os.Remove(dbPath) // Clean up any existing file

	// Create the database
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() {
		db.Close()
		os.Remove(dbPath) // Clean up after test
	}()

	// Start a session
	sessionID, err := db.StartSession()
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// Record argument usage
	db.RecordArgUsage("ls", []string{"-l", "-a"})

	// Check argument retrieval
	args := db.GetArgumentsByFrequency("ls", "")
	if len(args) != 2 {
		t.Errorf("Expected 2 arguments for ls, got %d", len(args))
	}

	// Record more usage of -l to change ranking
	db.RecordArgUsage("ls", []string{"-l"})
	db.RecordArgUsage("ls", []string{"-l"})

	// Check that -l is now ranked first
	args = db.GetArgumentsByFrequency("ls", "")
	if len(args) == 0 {
		t.Fatal("No arguments returned")
	}

	if args[0] != "-l" {
		t.Errorf("Expected -l to be ranked first, got %s", args[0])
	}

	// Test command recording
	jobManager := NewJobManager()
	cmd, err := NewCommand("ls -la", jobManager)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Set execution timestamps
	cmd.StartTime = time.Now().Add(-1 * time.Second)
	cmd.EndTime = time.Now()
	cmd.Duration = cmd.EndTime.Sub(cmd.StartTime)
	cmd.ReturnCode = 0

	// Record the command
	err = db.RecordCommand(cmd, sessionID)
	if err != nil {
		t.Fatalf("Failed to record command: %v", err)
	}

	// Get command history
	history, err := db.GetHistory(10)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 command in history, got %d", len(history))
	}

	if len(history) > 0 && history[0] != "ls -la" {
		t.Errorf("Expected 'ls -la' in history, got '%s'", history[0])
	}

	// End the session
	err = db.EndSession(sessionID)
	if err != nil {
		t.Errorf("Failed to end session: %v", err)
	}

	// Test save
	err = db.Save()
	if err != nil {
		t.Errorf("Failed to save database: %v", err)
	}
}
