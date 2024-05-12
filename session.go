package gosh

import (
	"os"
	"time"
)

// Session stores session-related information.
type Session struct {
	StartTime time.Time
	EndTime   time.Time
	UserID    int
	UserName  string
	MachineID string
	SessionID int
}

// NewSession initializes a new session with current environmental data.
func NewSession() *Session {
	return &Session{
		StartTime: time.Now(),
		UserID:    os.Getuid(),
		UserName:  os.Getenv("USER"),
		MachineID: os.Getenv("HOSTNAME"),
	}
}
