package gosh

// ArgHistoryStorage defines the interface for argument history storage
type ArgHistoryStorage interface {
	// RecordArgUsage records the usage of arguments with a command
	RecordArgUsage(command string, args []string)

	// GetArgumentsByFrequency returns arguments for a command sorted by frequency
	GetArgumentsByFrequency(command string, prefix string) []string

	// RemoveEntry removes a command entry from the history
	RemoveEntry(command string)

	// ClearAllHistory clears all history
	ClearAllHistory()

	// Trim reduces the database size by removing least-used arguments
	Trim(maxArgsPerCommand int)

	// ApplyDecay applies a decay factor to all counts to phase out old arguments
	ApplyDecay(factor float64)

	// Save persists the history to storage
	Save() error

	// Close releases any resources used by the storage
	Close() error
}

// NewArgHistory creates a new argument history database with SQLite
func NewArgHistory(path string) (ArgHistoryStorage, error) {
	// Direct SQLite implementation
	return NewArgHistoryDBSQLite(path)
}
