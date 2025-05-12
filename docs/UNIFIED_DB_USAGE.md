# Using the Unified Database

This document describes how to integrate the unified database into the main application.

## Integration Steps

### 1. Update main.go

Replace the separate history manager and argument history with the unified database:

```go
// Create unified database manager
var unifiedDB *UnifiedDBProvider
var argHistory ArgHistoryStorage
var historyManager *HistoryManager

// Check if unified database is enabled (environment variable or config)
useUnifiedDB := os.Getenv("GOSH_USE_UNIFIED_DB") == "1"

if useUnifiedDB {
    // Initialize unified database
    var err error
    unifiedDB, err = NewUnifiedDBProvider()
    if err != nil {
        log.Printf("Warning: Could not initialize unified database: %v", err)
        // Fall back to separate databases
        useUnifiedDB = false
    } else {
        // Get interfaces from unified provider
        argHistory = unifiedDB.GetArgHistory()
        historyManager = unifiedDB.GetHistoryManager()
    }
}

// Fall back to separate databases if needed
if !useUnifiedDB {
    // Initialize argument history
    argHistory, err = NewArgHistory("")
    if err != nil {
        log.Printf("Warning: Could not initialize argument history: %v", err)
    }
    
    // Initialize history manager
    historyManager, err = NewHistoryManager("")
    if err != nil {
        log.Printf("Failed to create history manager: %v", err)
    }
}

// ... rest of the code remains the same
```

### 2. Add Cleanup on Exit

Ensure proper cleanup on application exit:

```go
// At the end of main() function
// Close the databases
if unifiedDB != nil {
    unifiedDB.Close()
} else {
    if argHistory != nil {
        argHistory.Save()
        argHistory.Close()
    }
    // History manager will be closed by deferred call
}
```

### 3. Add Migration Flag

Add a command-line flag to migrate from old databases:

```go
var migrateFlag bool
flag.BoolVar(&migrateFlag, "migrate-db", false, "Migrate from separate databases to unified database")

// ... after flag.Parse()
if migrateFlag {
    fmt.Println("Migrating databases...")
    err := MigrateAndUnify()
    if err != nil {
        log.Fatalf("Migration failed: %v", err)
    }
    fmt.Println("Migration completed successfully")
    return
}
```

## Configuration

The unified database uses the following environment variables:

1. `GOSH_UNIFIED_DB` - Custom path for the unified database file
2. `GOSH_USE_UNIFIED_DB` - Set to "1" to enable the unified database

## Gradual Transition

To ensure a smooth transition:

1. Make the unified database opt-in at first
2. Provide a migration flag
3. Later, make it the default with an opt-out flag
4. Eventually, remove the old database code

## Performance Considerations

The unified database provides better performance by:

1. Using a single database connection
2. Reducing disk I/O
3. Enabling more efficient queries
4. Better transaction management

## Integration Testing

Test the unified database integration with:

```bash
# Run with unified database
GOSH_USE_UNIFIED_DB=1 ./bin/gosh

# Migrate from separate databases
./bin/gosh -migrate-db
```