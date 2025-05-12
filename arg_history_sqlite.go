package gosh

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ArgHistoryDBSQLite stores command argument usage frequencies in a SQLite database
type ArgHistoryDBSQLite struct {
	db           *sql.DB
	dbLock       sync.RWMutex
	filePath     string
	lastModified time.Time
	dirty        bool
	stmtCache    map[string]*sql.Stmt
}

// NewArgHistoryDBSQLite creates a new SQLite-based argument history database
func NewArgHistoryDBSQLite(customPath string) (*ArgHistoryDBSQLite, error) {
	// Determine storage path
	var filePath string
	if customPath != "" {
		filePath = customPath
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		filePath = filepath.Join(home, ".gosh_arg_history.db")
	}

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for database: %v", err)
	}

	// Open or create the SQLite database
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %v", err)
	}

	// Initialize the database structure
	histDB := &ArgHistoryDBSQLite{
		db:           db,
		filePath:     filePath,
		lastModified: time.Now(),
		dirty:        false,
		stmtCache:    make(map[string]*sql.Stmt),
	}

	// Create tables if they don't exist
	if err := histDB.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	// Start background autocommit routine
	go histDB.backgroundCommit()

	return histDB, nil
}

// Close closes the database connection
func (db *ArgHistoryDBSQLite) Close() error {
	// Close any prepared statements
	for _, stmt := range db.stmtCache {
		stmt.Close()
	}
	return db.db.Close()
}

// initDB creates the database schema if it doesn't exist
func (db *ArgHistoryDBSQLite) initDB() error {
	// Create tables with indexes for performance
	schema := `
	CREATE TABLE IF NOT EXISTS commands (
		id INTEGER PRIMARY KEY,
		name TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS arguments (
		id INTEGER PRIMARY KEY,
		command_id INTEGER,
		text TEXT NOT NULL,
		count INTEGER DEFAULT 1,
		last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(command_id, text),
		FOREIGN KEY(command_id) REFERENCES commands(id)
	);

	CREATE INDEX IF NOT EXISTS idx_arguments_command_id ON arguments(command_id);
	CREATE INDEX IF NOT EXISTS idx_arguments_text ON arguments(text);
	CREATE INDEX IF NOT EXISTS idx_arguments_count ON arguments(count);
	`

	_, err := db.db.Exec(schema)
	return err
}

// RecordArgUsage records usage of an argument with a command
func (db *ArgHistoryDBSQLite) RecordArgUsage(command string, args []string) {
	if len(args) == 0 {
		return
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Begin a transaction for better performance
	tx, err := db.db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to start transaction: %v\n", err)
		return
	}

	// Define a cleanup function with its own error tracking
	// to avoid issues with the outer err variable
	succeeded := false
	defer func() {
		if !succeeded {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fmt.Fprintf(os.Stderr, "SQLite: Transaction rollback failed: %v\n", rollbackErr)
			}
		}
		// If we exit the function successfully, we've already committed
	}()

	// Get or create the command ID
	var commandID int64
	err = tx.QueryRow("SELECT id FROM commands WHERE name = ?", command).Scan(&commandID)
	if err == sql.ErrNoRows {
		// Command doesn't exist, create it
		result, err := tx.Exec("INSERT INTO commands (name) VALUES (?)", command)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: Failed to insert command: %v\n", err)
			return
		}
		commandID, err = result.LastInsertId()
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: Failed to get last insert ID: %v\n", err)
			return
		}
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Error querying command: %v\n", err)
		return
	}

	// Increment count for each argument - track if any failed
	var anyFailed bool
	for _, arg := range args {
		// Skip empty arguments
		if arg == "" {
			continue
		}

		// Try to update existing argument
		result, err := tx.Exec(`
			UPDATE arguments
			SET count = count + 1, last_used = CURRENT_TIMESTAMP
			WHERE command_id = ? AND text = ?
		`, commandID, arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: Update query failed: %v\n", err)
			anyFailed = true
			continue
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: RowsAffected failed: %v\n", err)
			anyFailed = true
			continue
		}

		if rowsAffected == 0 {
			// Argument doesn't exist for this command, insert it
			_, err := tx.Exec(`
				INSERT INTO arguments (command_id, text, count, last_used)
				VALUES (?, ?, 1, CURRENT_TIMESTAMP)
			`, commandID, arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "SQLite: Failed to insert new argument: %v\n", err)
				anyFailed = true
				continue
			}
		}
	}

	// Don't commit if any operations failed
	if anyFailed {
		fmt.Fprintf(os.Stderr, "SQLite: Some argument operations failed, rolling back transaction\n")
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Commit failed: %v\n", err)
		return
	}

	succeeded = true
	db.dirty = true
	db.lastModified = time.Now()
}

// GetArgumentsByFrequency returns arguments for a command sorted by frequency
func (db *ArgHistoryDBSQLite) GetArgumentsByFrequency(command string, prefix string) []string {
	db.dbLock.RLock()
	defer db.dbLock.RUnlock()

	var results []string

	// Look up command ID
	var commandID int64
	err := db.db.QueryRow("SELECT id FROM commands WHERE name = ?", command).Scan(&commandID)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return nil
	}

	// Query for matching arguments
	query := `
		SELECT text, count 
		FROM arguments 
		WHERE command_id = ? AND text LIKE ? 
		ORDER BY count DESC, last_used DESC
		LIMIT 50
	`

	rows, err := db.db.Query(query, commandID, prefix+"%")
	if err != nil {
		return nil
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		var text string
		var count int
		if err := rows.Scan(&text, &count); err != nil {
			continue
		}
		results = append(results, text)
	}

	return results
}

// RemoveEntry removes a command entry and all its arguments from the history
func (db *ArgHistoryDBSQLite) RemoveEntry(command string) {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Begin a transaction
	tx, err := db.db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to start transaction: %v\n", err)
		return
	}

	succeeded := false
	defer func() {
		if !succeeded {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fmt.Fprintf(os.Stderr, "SQLite: Transaction rollback failed: %v\n", rollbackErr)
			}
		}
	}()

	// Get command ID
	var commandID int64
	err = tx.QueryRow("SELECT id FROM commands WHERE name = ?", command).Scan(&commandID)
	if err != nil {
		if err != sql.ErrNoRows {
			fmt.Fprintf(os.Stderr, "SQLite: Error querying command: %v\n", err)
		}
		return
	}

	// Delete all arguments for this command
	_, err = tx.Exec("DELETE FROM arguments WHERE command_id = ?", commandID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to delete arguments: %v\n", err)
		return
	}

	// Delete the command itself
	_, err = tx.Exec("DELETE FROM commands WHERE id = ?", commandID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to delete command: %v\n", err)
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Commit failed: %v\n", err)
		return
	}

	succeeded = true
	db.dirty = true
	db.lastModified = time.Now()
}

// ClearAllHistory clears all history
func (db *ArgHistoryDBSQLite) ClearAllHistory() {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Begin a transaction
	tx, err := db.db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to start transaction: %v\n", err)
		return
	}

	succeeded := false
	defer func() {
		if !succeeded {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fmt.Fprintf(os.Stderr, "SQLite: Transaction rollback failed: %v\n", rollbackErr)
			}
		}
	}()

	// Delete all data
	_, err = tx.Exec("DELETE FROM arguments")
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to delete arguments: %v\n", err)
		return
	}

	_, err = tx.Exec("DELETE FROM commands")
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to delete commands: %v\n", err)
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Commit failed: %v\n", err)
		return
	}

	succeeded = true
	db.dirty = true
	db.lastModified = time.Now()
}

// Trim reduces the database size by removing least-used arguments
func (db *ArgHistoryDBSQLite) Trim(maxArgsPerCommand int) {
	if maxArgsPerCommand <= 0 {
		return
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Use a transaction for all trim operations
	tx, err := db.db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to start transaction: %v\n", err)
		return
	}

	succeeded := false
	defer func() {
		if !succeeded {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fmt.Fprintf(os.Stderr, "SQLite: Transaction rollback failed: %v\n", rollbackErr)
			}
		}
	}()

	// Get all commands
	rows, err := tx.Query("SELECT id, name FROM commands")
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to query commands: %v\n", err)
		return
	}
	defer rows.Close()

	var commandsToTrim []struct {
		id   int64
		name string
	}

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: Failed to scan command row: %v\n", err)
			continue
		}

		// Count arguments for this command
		var argCount int
		err := tx.QueryRow("SELECT COUNT(*) FROM arguments WHERE command_id = ?", id).Scan(&argCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: Failed to count arguments: %v\n", err)
			continue
		}

		if argCount > maxArgsPerCommand {
			commandsToTrim = append(commandsToTrim, struct {
				id   int64
				name string
			}{id, name})
		}
	}

	// Now trim each command that has too many arguments
	for _, cmd := range commandsToTrim {
		// Delete all arguments beyond the top maxArgsPerCommand
		_, err := tx.Exec(`
			DELETE FROM arguments 
			WHERE command_id = ? AND id NOT IN (
				SELECT id FROM arguments 
				WHERE command_id = ? 
				ORDER BY count DESC, last_used DESC 
				LIMIT ?
			)
		`, cmd.id, cmd.id, maxArgsPerCommand)

		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLite: Error trimming command '%s': %v\n", cmd.name, err)
			continue
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Commit failed: %v\n", err)
		return
	}

	succeeded = true
	db.dirty = true
	db.lastModified = time.Now()
}

// ApplyDecay applies a decay factor to all counts to phase out old arguments
func (db *ArgHistoryDBSQLite) ApplyDecay(factor float64) {
	if factor <= 0 || factor >= 1 {
		return // Invalid factor
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Use a transaction for the decay operation
	tx, err := db.db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Failed to start transaction: %v\n", err)
		return
	}

	succeeded := false
	defer func() {
		if !succeeded {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fmt.Fprintf(os.Stderr, "SQLite: Transaction rollback failed: %v\n", rollbackErr)
			}
		}
	}()

	// Apply decay to all counts
	_, err = tx.Exec(`
		UPDATE arguments
		SET count = CASE 
			WHEN count * ? < 1 THEN 1 
			ELSE CAST(count * ? AS INTEGER) 
		END
	`, factor, factor)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Error applying decay: %v\n", err)
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "SQLite: Commit failed: %v\n", err)
		return
	}

	succeeded = true
	db.dirty = true
	db.lastModified = time.Now()
}

// Save forces a commit of any uncommitted changes
// This is mostly a no-op for SQLite as changes are committed as they're made
// But it does set the dirty flag to false
func (db *ArgHistoryDBSQLite) Save() error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Run a pragma to optimize the database
	_, err := db.db.Exec("PRAGMA optimize")
	db.dirty = false
	return err
}

// backgroundCommit periodically checks and optimizes the database
func (db *ArgHistoryDBSQLite) backgroundCommit() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		db.dbLock.RLock()
		dirty := db.dirty
		lastMod := db.lastModified
		db.dbLock.RUnlock()

		if dirty && time.Since(lastMod) > 5*time.Minute {
			db.Save()
		}
	}
}
