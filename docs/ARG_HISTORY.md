# Argument History

This document describes the argument history feature in gosh.

## Overview

The argument history feature tracks which arguments are commonly used with each command and provides intelligent tab completion based on usage frequency. This allows the shell to provide smarter suggestions during tab completion.

## Implementation

The argument history system has the following components:

1. **Storage Interface**: An abstract interface (`ArgHistoryStorage`) that defines the operations for storing and retrieving argument history.

2. **SQLite Implementation**: The primary storage implementation (`ArgHistoryDBSQLite`) that uses SQLite to efficiently store command and argument usage data.

3. **Tab Completion Integration**: The smart tab completion system that uses the argument history to provide usage-ranked suggestions.

## How It Works

1. Every time a command is executed with arguments, the shell records those arguments and their usage counts.
2. When a user presses Tab after typing a command, the system looks up the most frequently used arguments for that command.
3. Arguments are sorted by usage frequency (most used first).
4. Pressing Tab multiple times cycles through the available suggestions.

## File Structure

- `arg_history_interface.go`: Defines the abstract `ArgHistoryStorage` interface.
- `arg_history_sqlite.go`: Implements the SQLite-based storage backend.
- `smart_completion.go`: Implements the smart tab completion system with cycling behavior.

## Usage

No special configuration is needed - the argument history feature is enabled by default. As you use the shell, it will learn your common argument patterns.

### Debugging

To enable debug logging for the argument history system:

```bash
export GOSH_ARG_DEBUG=1
```

### Maintenance Operations

The following maintenance commands are available:

```bash
# Trim history to keep only the top 100 arguments per command
gosh -trim-history

# Apply decay to gradually phase out old argument history
gosh -decay-history
```

## Design Notes

1. **Transaction Safety**: All database operations use transactions to ensure data integrity.
2. **Error Handling**: Comprehensive error handling ensures the system degrades gracefully.
3. **Performance**: Indexes on key columns optimize query performance.
4. **Background Optimization**: A background thread periodically optimizes the database.