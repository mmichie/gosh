# Unified Database Design

This document outlines the design for a unified SQLite database to store both command history and argument usage history for gosh shell.

## Current State

Currently, gosh uses two separate SQLite databases:

1. `.gosh_history.sqlite` - Stores command execution history
2. `.gosh_arg_history.db` - Stores argument usage history for tab completion

## Proposed Schema

The unified database will be organized into the following tables with proper relationships:

### 1. `sessions` Table
Stores information about shell sessions:

```sql
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    start_time INTEGER NOT NULL,
    end_time INTEGER,
    user_id INTEGER NOT NULL,
    hostname TEXT NOT NULL,
    tty TEXT
);
```

### 2. `commands` Table
Stores complete command execution history:

```sql
CREATE TABLE commands (
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
```

### 3. `command_registry` Table
Tracks unique commands for tab completion:

```sql
CREATE TABLE command_registry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    last_used INTEGER NOT NULL,
    use_count INTEGER DEFAULT 1
);
```

### 4. `arguments` Table
Tracks arguments associated with commands for tab completion:

```sql
CREATE TABLE arguments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    command_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    count INTEGER DEFAULT 1,
    last_used INTEGER NOT NULL,
    UNIQUE(command_id, text),
    FOREIGN KEY (command_id) REFERENCES command_registry(id)
);
```

### 5. `command_arguments` Table (Optional - for exact command line tracking)
Tracks specific arguments used with specific command invocations:

```sql
CREATE TABLE command_arguments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    command_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    value TEXT NOT NULL,
    FOREIGN KEY (command_id) REFERENCES commands(id)
);
```

## Indexes

To ensure efficient lookups:

```sql
CREATE INDEX idx_commands_base_command ON commands(base_command);
CREATE INDEX idx_commands_session_id ON commands(session_id);
CREATE INDEX idx_arguments_command_id ON arguments(command_id);
CREATE INDEX idx_arguments_text ON arguments(text);
CREATE INDEX idx_arguments_count ON arguments(count);
```

## Implementation Plan

1. Create a new database implementation with the unified schema
2. Add migration support to move data from the existing databases
3. Update the existing HistoryManager and ArgHistoryStorage implementations to use the new schema
4. Add a configuration option to enable the unified storage or use legacy storage
5. Eventually deprecate the separate databases

## Benefits

1. **Data Integrity**: Relational constraints ensure consistency
2. **Performance**: Single connection pool and prepared statements
3. **Reduced File I/O**: One database file instead of two
4. **Enhanced Queries**: Can perform complex queries across related tables
5. **Maintenance**: Single schema to maintain and upgrade

## Migration Strategy

To migrate existing data:

1. Open both existing databases
2. Create the new unified database
3. Insert session data (generate placeholder sessions if needed)
4. Copy command history with references to sessions
5. Copy argument history with references to command registry
6. Validate the migrated data
7. Close all databases

The migration will be offered as an option when first launching with the new version, or through a command-line flag.