package gosh

import (
	"os"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	StartTime time.Time
	EndTime   time.Time
	UserID    int
	UserName  string
	MachineID string
	SessionID string
}

// NewSession initializes a new session with current environmental data.
func NewSession() *Session {
	return &Session{
		StartTime: time.Now(),
		UserID:    os.Getuid(),
		UserName:  os.Getenv("USER"),
		MachineID: os.Getenv("HOSTNAME"),
		SessionID: generateSessionID(),
	}
}

// generateSessionID generates a UUID for use as a unique session ID.
func generateSessionID() string {
	return uuid.New().String() // Generate and return a new UUID as a string.
}
