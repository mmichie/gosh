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

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS command(
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        session_id INTEGER NOT NULL,
        tty VARCHAR(20) NOT NULL,
        euid INT NOT NULL,
        cwd VARCHAR(256) NOT NULL,
        return_code INT NOT NULL,
        start_time INTEGER NOT NULL,
        end_time INTEGER NOT NULL,
        duration INTEGER NOT NULL,
        command VARCHAR(1000) NOT NULL,
        args VARCHAR(1000) NOT NULL
    );`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	return &HistoryManager{db: db}, nil
}

func (h *HistoryManager) Insert(cmd *Command, sessionID int) error {
	insertSQL := `INSERT INTO command (session_id, tty, euid, cwd, start_time, end_time, duration, command, args, return_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	command := cmd.SimpleCommand.Items[0].Value
	args := make([]string, len(cmd.SimpleCommand.Items)-1)
	for i, item := range cmd.SimpleCommand.Items[1:] {
		args[i] = item.Value
	}

	_, err := h.db.Exec(insertSQL, sessionID, cmd.TTY, cmd.EUID, cmd.CWD, cmd.StartTime.Unix(), cmd.EndTime.Unix(), int(cmd.Duration.Seconds()), command, strings.Join(args, " "), cmd.ReturnCode)
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
