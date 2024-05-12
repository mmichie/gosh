package gosh

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// HistoryManager manages the command history stored in SQLite.
type HistoryManager struct {
	db *sql.DB
}

// NewHistoryManager initializes a new history manager with a default or specified database path.
func NewHistoryManager(dbPath string) (*HistoryManager, error) {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir() // Get user home directory
		if err != nil {
			return nil, err // Handle errors retrieving the home directory
		}
		dbPath = filepath.Join(homeDir, ".gosh_history.sqlite") // Construct default path
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	return &HistoryManager{db: db}, nil
}

// Insert inserts a new command into the history.
func (h *HistoryManager) Insert(cmd *Command) error {
	_, err := h.db.Exec("INSERT INTO command (command, args, return_code) VALUES (?, ?, ?)",
		cmd.Command, strings.Join(cmd.Args, " "), cmd.ReturnCode)
	return err
}

// Dump returns the entire history of commands.
func (h *HistoryManager) Dump() ([]string, error) {
	rows, err := h.db.Query("SELECT command || ' ' || args AS cmd FROM command")
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
