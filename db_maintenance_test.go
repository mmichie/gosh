package gosh

import (
	"os"
	"testing"
)

// TestDatabaseMaintenance tests the maintenance operations of the database
func TestDatabaseMaintenance(t *testing.T) {
	// Create a temporary database file
	dbPath := "/tmp/gosh_db_maintenance_test.db"
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

	// Record a lot of arguments for testing trimming
	for i := 0; i < 20; i++ {
		db.RecordArgUsage("test-cmd", []string{"-arg" + string(rune('a'+i))})
	}

	// Test Trim operation
	err = db.Trim(10)
	if err != nil {
		t.Fatalf("Failed to trim database: %v", err)
	}

	// Check that we now have 10 arguments
	args := db.GetArgumentsByFrequency("test-cmd", "")
	if len(args) != 10 {
		t.Errorf("Expected 10 arguments after trim, got %d", len(args))
	}

	// Test ApplyDecay
	cmd := "decay-test"
	db.RecordArgUsage(cmd, []string{"-a", "-b", "-c"})

	// Record extra usage of -a and -b
	for i := 0; i < 9; i++ {
		db.RecordArgUsage(cmd, []string{"-a"})
	}
	for i := 0; i < 4; i++ {
		db.RecordArgUsage(cmd, []string{"-b"})
	}

	// Apply decay
	err = db.ApplyDecay(0.5)
	if err != nil {
		t.Fatalf("Failed to apply decay: %v", err)
	}

	// Test ClearHistory
	err = db.ClearHistory()
	if err != nil {
		t.Fatalf("Failed to clear history: %v", err)
	}

	// Check that history is empty
	history, err := db.GetHistory(10)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected empty history after clear, got %d entries", len(history))
	}

	// End session
	err = db.EndSession(sessionID)
	if err != nil {
		t.Errorf("Failed to end session: %v", err)
	}
}
