# gosh Shell TODO List

This document outlines the remaining features and improvements needed for the gosh shell implementation.

## Major Initiative: Record Streams with M28 Integration

Transform gosh into a data-oriented shell by implementing record streams. See [RECORD_STREAMS_DESIGN.md](RECORD_STREAMS_DESIGN.md) for detailed design.

### Phase 1: Core Record Infrastructure (High Priority)
□ **Record Protocol**
   - Define record types and serialization format
   - Implement RecordEmitter and RecordConsumer interfaces
   - Add --records flag to built-in commands (ls, ps, etc.)

□ **Basic I/O Commands**
   - from-json: Read JSON files/streams as records
   - from-csv: Parse CSV files with header detection
   - to-json: Output records as JSON
   - to-table: Pretty-print records in table format

□ **Pipeline Integration**
   - Modify pipeline executor to handle record streams
   - Add record-aware pipe operator
   - Environment variable for format negotiation

### Phase 2: M28 Stream Processing (High Priority)
□ **Lisp Stream Operators**
   - (map): Transform each record with a function
   - (filter): Select records matching a predicate
   - (reduce): Aggregate records to a single value
   - (take) / (drop): Limit record streams

□ **Inline Lisp Integration**
   - Parse inline Lisp expressions in pipelines
   - $ variable for current record access
   - Seamless conversion between shell and Lisp

□ **Stream Comprehensions**
   - (defstream): Define reusable stream processors
   - Stream composition operators
   - Lazy evaluation support

### Phase 3: Advanced Features (Medium Priority)
□ **Field Operations**
   - Nested field access with / notation
   - Array indexing with # notation
   - Fuzzy field matching with @ prefix
   - Wildcard field selection

□ **Aggregation Commands**
   - group-by: Group records by field values
   - aggregate: Compute statistics (sum, avg, count)
   - window: Time or count-based windows
   - pivot: Multi-dimensional aggregations

□ **Data Commands**
   - where: SQL-like filtering syntax
   - select: Project specific fields
   - join: Join multiple record streams
   - sort-by: Multi-field sorting

### Phase 4: Ecosystem (Low Priority)
□ **Format Support**
   - from-yaml / to-yaml
   - from-xml / to-xml  
   - from-log with pattern matching
   - Auto-detect format

□ **External Integration**
   - http command for API calls
   - to-sqlite / from-sqlite
   - to-chart for visualizations
   - Plugin system for custom formats

## Next Implementation Tasks

The following tasks have been identified as the next items to implement, based on examination of the codebase:

✅ **Fixed: CD with Dash**
   - Implemented a fix for the `cd -` command to correctly change to the previous directory
   - Updated `global_state.go` to properly initialize and maintain the previous directory
   - Enhanced `builtins.go` to handle edge cases in the `cd` function implementation
   - Added a dedicated test case in `cd_test.go` to verify proper functionality
   - Ensured cross-platform compatibility by handling symlinks (e.g., `/var` vs `/private/var` on macOS)

✅ **Fixed: File Redirection Issues**
   - Completely refactored the command execution logic in `command.go` to properly handle file redirection
   - Resolved issues with files being closed too early by reorganizing when and how files are opened and closed
   - Added proper file path handling by resolving absolute paths
   - Implemented a dedicated test case in `redirection_test.go` to verify the functionality
   - Fixed the integration test for file creation and content verification

✅ **Implemented OR Operator (||)**
   - Added "Or" operator to the lexer rules with pattern `\|\|`
   - Updated command structures to support OR conditional chains
   - Implemented command execution to run the second command only if the first fails
   - Added tests that verify OR operator functionality

## Core Shell Features

### High Priority

✅ **Implemented: Advanced Redirection**
   - Support for file descriptor redirection (2>, &>, etc.)
   - Append redirection for error output (2>>)
   - File descriptor duplication (2>&1)

### High Priority (Supporting Record Streams)

□ **Enhanced M28 Integration**
   - Extend M28 with record manipulation functions
   - Add stream processing primitives to M28 standard library
   - Create Lisp DSL for data transformations
   - Performance optimizations for Lisp stream operations

□ **Shell Script Support**
   - Add ability to execute shell scripts from files
   - Implement basic flow control structures (if/else, loops)  
   - Support for variables and basic script functions
   - Integration with record stream processing

□ **Array Support**
   - Implement array variables
   - Add array operations (indexing, slicing, iteration)
   - Arrays as lightweight record streams
   - Conversion between arrays and records

### Medium Priority

✅ **Implemented: Wildcard Expansion**
   - Improved glob pattern support for file matching
   - Added support for common wildcard patterns (`*`, `?`, `[...]`)
   - Added support for brace expansion (`{alt1,alt2,...}`)
   - Added home directory expansion (`~`)

✅ **Implemented: Command Substitution**
   - Implemented command substitution using backticks or `$(command)` syntax
   - Added ability to use output of one command as arguments for another
   - Added support for nested command substitutions
   - Implemented error handling for failed command substitutions

✅ **Implemented: Here-Documents**
   - Support for here-docs (`<<EOF`) and here-strings (`<<<`)
   - Multi-line string input for commands
   - Tab stripping with `<<-EOF` syntax
   - Support for quoted delimiters

### Low Priority

□ **Environment Variable Expansion**
   - Enhance environment variable support
   - Add variable substitution in more contexts
   - Add parameter expansion (`${var:-default}`, `${var:=value}`, etc.)

□ **Process Substitution**
   - Implement process substitution (`<()` and `>()`)
   - Allow using command output as file input without temporary files

□ **Signal Handling**
    - Improve handling of various signals (SIGINT, SIGTSTP, etc.)
    - Add custom signal handlers for better shell control

□ **Tab Completion Enhancements**
    - Expand tab completion to handle more complex scenarios
    - Add completion for command options and arguments

□ **Shell Functions**
    - Add support for user-defined shell functions
    - Enable function arguments and return values

## M28 Lisp Integration (Critical for Record Streams)

□ **Record-Oriented Lisp Functions**
    - Record manipulation functions (get-field, set-field, update)
    - Collection operations (map, filter, reduce, group-by)
    - Statistical functions (sum, average, percentile)
    - Time/date functions for temporal data

□ **Stream Processing Library**  
    - Lazy evaluation for infinite streams
    - Parallel processing primitives
    - Stream combinators (merge, zip, partition)
    - Window and session functions

□ **Lisp/Shell Interop**
    - Seamless type conversion between shell and Lisp
    - Lisp functions callable as shell commands
    - Shell commands callable from Lisp
    - Shared variable namespace

□ **Performance & Debugging**
    - JIT compilation for hot code paths
    - Stream processing optimizations
    - Visual debugger for Lisp pipelines
    - Performance profiling tools

## User Experience

□ **Better Error Messages**
    - Improve error reporting with more context
    - Add suggestions for common errors

□ **Configuration System**
    - Add support for a config file (similar to .bashrc)
    - Allow customization of shell behavior through config

□ **Command Line Editing**
    - Enhanced line editing capabilities
    - Emacs/vi editing modes

□ **Prompt Customization**
    - More variables and expansion options for prompts
    - Support for ANSI colors and styling

□ **Documentation**
    - Add comprehensive documentation for all features
    - Create man pages for the shell and its builtins

## Testing and Stability

□ **Expand Test Coverage**
    - Add more integration tests for edge cases
    - Implement unit tests for all components

□ **Performance Optimization**
    - Profile and optimize command execution
    - Reduce memory usage for long-running sessions

## Example: Record Streams in Action

Once implemented, gosh will enable powerful data processing:

```bash
# System monitoring with alerts
ps --records | 
(filter #(> (:cpu %) 80)) |
group-by user |
select user cpu-total:{(sum cpu)} proc-count:{(count)} |
where {.cpu-total > 200} |
to-alert "High CPU usage by user"

# Log analysis with Lisp transformations  
from-log nginx.log |
(map #(assoc % 
  :response-time-ms (parse-float (:response-time %))
  :hour (time/hour (:timestamp %)))) |
group-by endpoint hour |
aggregate p95:{(percentile 95 response-time-ms)} avg:{(average response-time-ms)} |
where {.p95 > 1000} |
to-chart --x hour --y p95 --group-by endpoint

# Multi-source data join
parallel {
  docker ps --records | select container-id name cpu memory
  docker stats --records | select container-id network-io disk-io
} |
join container-id |
(map #(assoc % :efficiency (/ (:cpu %) (:memory %)))) |
sort-by efficiency desc |
to-table
```

## Completed Features

- Basic command execution
- Pipe operator (`|`) for command chaining
- AND operator (`&&`) for conditional execution
- OR operator (`||`) for conditional execution
- Simple I/O redirection (`>`, `>>`, `<`)
- Advanced redirection (2>, 2>&1, &>)
- Environment variables
- Command history
- Basic tab completion
- Built-in commands
- M28 Lisp integration
- Command separators (`;`) for multiple commands
- Background jobs management (`&`, `jobs`, `fg`, `bg`)
- Command substitution (`$(...)` and backticks)
- Wildcard expansion (*, ?, [...], {...})
- Here-documents (<<EOF) and here-strings (<<<)