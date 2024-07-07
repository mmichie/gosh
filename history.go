package gosh

import (
	"database/sql"
	"os"
	"path/filepath"

	"gosh/parser"

	_ "github.com/mattn/go-sqlite3"
)

// HistoryManager manages the command history stored in SQLite.
type HistoryManager struct {
	db *sql.DB
}

func NewHistoryManager(dbPath string) (*HistoryManager, error) {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(homeDir, ".gosh_history.sqlite")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Check if the table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='command'").Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			// Table doesn't exist, create it
			createTableSQL := `
			CREATE TABLE command(
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				session_id INTEGER NOT NULL,
				tty VARCHAR(20) NOT NULL,
				euid INT NOT NULL,
				cwd VARCHAR(256) NOT NULL,
				return_code INT NOT NULL,
				start_time INTEGER NOT NULL,
				end_time INTEGER NOT NULL,
				duration INTEGER NOT NULL,
				command VARCHAR(1000) NOT NULL
			);`
			_, err = db.Exec(createTableSQL)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &HistoryManager{db: db}, nil
}

func (h *HistoryManager) Insert(cmd *Command, sessionID int) error {
	// Check if 'args' column exists
	var argsColumnExists bool
	err := h.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('command') WHERE name='args'").Scan(&argsColumnExists)
	if err != nil {
		return err
	}

	var insertSQL string
	var args []interface{}

	fullCommand := parser.FormatCommand(cmd.Command)
	gs := GetGlobalState()

	if argsColumnExists {
		insertSQL = `INSERT INTO command (session_id, tty, euid, cwd, start_time, end_time, duration, command, args, return_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		args = []interface{}{sessionID, cmd.TTY, cmd.EUID, gs.GetCWD(), cmd.StartTime.Unix(), cmd.EndTime.Unix(), int(cmd.Duration.Seconds()), fullCommand, "", cmd.ReturnCode}
	} else {
		insertSQL = `INSERT INTO command (session_id, tty, euid, cwd, start_time, end_time, duration, command, return_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		args = []interface{}{sessionID, cmd.TTY, cmd.EUID, gs.GetCWD(), cmd.StartTime.Unix(), cmd.EndTime.Unix(), int(cmd.Duration.Seconds()), fullCommand, cmd.ReturnCode}
	}

	_, err = h.db.Exec(insertSQL, args...)
	return err
}

// Dump returns the entire history of commands.
func (h *HistoryManager) Dump() ([]string, error) {
	rows, err := h.db.Query("SELECT command FROM command")
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
