package gosh

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gosh/parser"

	_ "github.com/mattn/go-sqlite3"
)

// Database manages shell history and command arguments in a single SQLite database
type Database struct {
	db           *sql.DB
	dbLock       sync.RWMutex
	filePath     string
	lastModified time.Time
	dirty        bool
}

// NewDatabase creates a new database for the shell
func NewDatabase(customPath string) (*Database, error) {
	// Determine storage path
	var filePath string
	if customPath != "" {
		filePath = customPath
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		filePath = filepath.Join(home, ".gosh.db")
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

	// Enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %v", err)
	}

	// Initialize the database structure
	database := &Database{
		db:           db,
		filePath:     filePath,
		lastModified: time.Now(),
		dirty:        false,
	}

	// Create tables if they don't exist
	if err := database.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	// Start background optimization
	go database.backgroundOptimize()

	return database, nil
}

// Close closes the database connection
func (db *Database) Close() error {
	return db.db.Close()
}

// initDB creates the database schema if it doesn't exist
func (db *Database) initDB() error {
	// Create tables with indexes for performance
	schema := `
	-- Sessions table for tracking shell sessions
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time INTEGER NOT NULL,
		end_time INTEGER,
		user_id INTEGER NOT NULL,
		hostname TEXT NOT NULL,
		tty TEXT
	);

	-- Commands table for complete command history
	CREATE TABLE IF NOT EXISTS commands (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		cwd TEXT NOT NULL,
		full_command TEXT NOT NULL,
		base_command TEXT NOT NULL,
		start_time INTEGER NOT NULL,
		end_time INTEGER,
		duration INTEGER,
		return_code INTEGER,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);

	-- Command registry for argument tab completion
	CREATE TABLE IF NOT EXISTS command_registry (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		last_used INTEGER NOT NULL,
		use_count INTEGER DEFAULT 1
	);

	-- Arguments table for argument tab completion
	CREATE TABLE IF NOT EXISTS arguments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		command_id INTEGER NOT NULL,
		text TEXT NOT NULL,
		count INTEGER DEFAULT 1,
		last_used INTEGER NOT NULL,
		UNIQUE(command_id, text),
		FOREIGN KEY (command_id) REFERENCES command_registry(id)
	);

	-- Command arguments tracking
	CREATE TABLE IF NOT EXISTS command_arguments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		command_id INTEGER NOT NULL,
		position INTEGER NOT NULL,
		value TEXT NOT NULL,
		FOREIGN KEY (command_id) REFERENCES commands(id)
	);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_commands_base_command ON commands(base_command);
	CREATE INDEX IF NOT EXISTS idx_commands_session_id ON commands(session_id);
	CREATE INDEX IF NOT EXISTS idx_arguments_command_id ON arguments(command_id);
	CREATE INDEX IF NOT EXISTS idx_arguments_text ON arguments(text);
	CREATE INDEX IF NOT EXISTS idx_arguments_count ON arguments(count);
	`

	_, err := db.db.Exec(schema)
	return err
}

// StartSession creates a new session and returns its ID
func (db *Database) StartSession() (int64, error) {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Create a new session
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	userID := os.Geteuid()
	tty := os.Getenv("TTY")

	result, err := db.db.Exec(
		"INSERT INTO sessions (start_time, user_id, hostname, tty) VALUES (?, ?, ?, ?)",
		time.Now().Unix(), userID, hostname, tty,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// EndSession marks a session as ended
func (db *Database) EndSession(sessionID int64) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	_, err := db.db.Exec(
		"UPDATE sessions SET end_time = ? WHERE id = ?",
		time.Now().Unix(), sessionID,
	)
	return err
}

// extractCommandParts extracts the base command and arguments from a Command structure
func extractCommandParts(cmd *Command) (string, []string) {
	baseCommand := ""
	var args []string

	if len(cmd.Command.LogicalBlocks) > 0 {
		block := cmd.Command.LogicalBlocks[0]
		if len(block.FirstPipeline.Commands) > 0 {
			cmdParts := block.FirstPipeline.Commands[0].Parts
			if len(cmdParts) > 0 {
				baseCommand = cmdParts[0]
				if len(cmdParts) > 1 {
					args = cmdParts[1:]
				}
			}
		}
	}

	return baseCommand, args
}

// RecordCommand records a command in the history database
func (db *Database) RecordCommand(cmd *Command, sessionID int64) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	fullCommand := parser.FormatCommand(cmd.Command)
	baseCommand, args := extractCommandParts(cmd)

	// Insert command record
	result, err := tx.Exec(
		`INSERT INTO commands 
		(session_id, cwd, full_command, base_command, start_time, end_time, duration, return_code) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID,
		GetGlobalState().GetCWD(),
		fullCommand,
		baseCommand,
		cmd.StartTime.Unix(),
		cmd.EndTime.Unix(),
		int(cmd.Duration.Seconds()),
		cmd.ReturnCode,
	)
	if err != nil {
		return err
	}

	// Get the command ID
	commandID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// If there are arguments, add them as command_arguments
	if len(args) > 0 {
		for i, arg := range args {
			_, err = tx.Exec(
				"INSERT INTO command_arguments (command_id, position, value) VALUES (?, ?, ?)",
				commandID, i, arg,
			)
			if err != nil {
				return err
			}
		}
	}

	// Also update the command registry and arguments table for tab completion
	if baseCommand != "" {
		now := time.Now().Unix()

		// Update or insert the base command in the registry
		_, err = tx.Exec(
			`INSERT INTO command_registry (name, last_used, use_count) 
			VALUES (?, ?, 1) 
			ON CONFLICT(name) DO UPDATE SET 
			last_used=?, use_count=use_count+1`,
			baseCommand, now, now,
		)
		if err != nil {
			return err
		}

		// Also record any arguments for tab completion if there are any
		if len(args) > 0 {
			// Get the command registry ID
			var registryID int64
			err = tx.QueryRow("SELECT id FROM command_registry WHERE name = ?", baseCommand).Scan(&registryID)
			if err != nil {
				return err
			}

			// Record each argument
			for _, arg := range args {
				_, err = tx.Exec(
					`INSERT INTO arguments (command_id, text, last_used) 
					VALUES (?, ?, ?) 
					ON CONFLICT(command_id, text) DO UPDATE SET 
					count=count+1, last_used=?`,
					registryID, arg, now, now,
				)
				if err != nil {
					return err
				}
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	db.dirty = true
	db.lastModified = time.Now()

	return nil
}

// GetHistory returns the command history
func (db *Database) GetHistory(limit int) ([]string, error) {
	db.dbLock.RLock()
	defer db.dbLock.RUnlock()

	query := "SELECT full_command FROM commands ORDER BY start_time DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []string
	for rows.Next() {
		var cmd string
		if err := rows.Scan(&cmd); err != nil {
			return nil, err
		}
		history = append(history, cmd)
	}

	return history, nil
}

// RecordArgUsage records usage of arguments with a command
func (db *Database) RecordArgUsage(command string, args []string) {
	if len(args) == 0 {
		return
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Begin a transaction for better performance
	tx, err := db.db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start transaction: %v\n", err)
		return
	}

	// Define a cleanup function with its own error tracking
	succeeded := false
	defer func() {
		if !succeeded {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fmt.Fprintf(os.Stderr, "Transaction rollback failed: %v\n", rollbackErr)
			}
		}
	}()

	now := time.Now().Unix()

	// Update or create command entry in the registry
	_, err = tx.Exec(
		`INSERT INTO command_registry (name, last_used, use_count) 
		VALUES (?, ?, 1) 
		ON CONFLICT(name) DO UPDATE SET 
		last_used=?, use_count=use_count+1`,
		command, now, now,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update command registry: %v\n", err)
		return
	}

	// Get the command registry ID
	var registryID int64
	err = tx.QueryRow("SELECT id FROM command_registry WHERE name = ?", command).Scan(&registryID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying command: %v\n", err)
		return
	}

	// Increment count for each argument
	for _, arg := range args {
		// Skip empty arguments
		if arg == "" {
			continue
		}

		// Insert or update the argument
		_, err = tx.Exec(
			`INSERT INTO arguments (command_id, text, last_used) 
			VALUES (?, ?, ?) 
			ON CONFLICT(command_id, text) DO UPDATE SET 
			count=count+1, last_used=?`,
			registryID, arg, now, now,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to record argument usage: %v\n", err)
			continue
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "Commit failed: %v\n", err)
		return
	}

	succeeded = true
	db.dirty = true
	db.lastModified = time.Now()
}

// GetArgumentsByFrequency returns arguments for a command sorted by frequency
func (db *Database) GetArgumentsByFrequency(command string, prefix string) []string {
	db.dbLock.RLock()
	defer db.dbLock.RUnlock()

	var results []string

	// Look up command ID in the registry
	var commandID int64
	err := db.db.QueryRow("SELECT id FROM command_registry WHERE name = ?", command).Scan(&commandID)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return nil
	}

	// Query for matching arguments
	query := `
		SELECT text
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
		if err := rows.Scan(&text); err != nil {
			continue
		}
		results = append(results, text)
	}

	return results
}

// ClearHistory clears all command history
func (db *Database) ClearHistory() error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Begin a transaction
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Delete all data - start with tables that have foreign key constraints
	_, err = tx.Exec("DELETE FROM command_arguments")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM arguments")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM commands")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM command_registry")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM sessions")
	if err != nil {
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	db.dirty = true
	db.lastModified = time.Now()
	return nil
}

// Trim reduces the database size by removing least-used arguments
func (db *Database) Trim(maxArgsPerCommand int) error {
	if maxArgsPerCommand <= 0 {
		return nil
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Use a transaction for all trim operations
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Get all commands from registry
	rows, err := tx.Query("SELECT id, name FROM command_registry")
	if err != nil {
		return err
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
			continue
		}

		// Count arguments for this command
		var argCount int
		err := tx.QueryRow("SELECT COUNT(*) FROM arguments WHERE command_id = ?", id).Scan(&argCount)
		if err != nil {
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
			return err
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	db.dirty = true
	db.lastModified = time.Now()
	return nil
}

// ApplyDecay applies a decay factor to all counts to phase out old arguments
func (db *Database) ApplyDecay(factor float64) error {
	if factor <= 0 || factor >= 1 {
		return nil // Invalid factor
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Use a transaction for the decay operation
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
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
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	db.dirty = true
	db.lastModified = time.Now()
	return nil
}

// Save forces a commit of any uncommitted changes
func (db *Database) Save() error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	// Run pragmas to optimize the database
	_, err := db.db.Exec("PRAGMA optimize")
	db.dirty = false
	return err
}

// backgroundOptimize periodically optimizes the database
func (db *Database) backgroundOptimize() {
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
