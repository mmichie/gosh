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

## Comparison with Zsh: Feature Delta Analysis

To make gosh a viable daily-driver shell, we need to understand what users expect from modern shells. Here's a comprehensive comparison with zsh, one of the most feature-rich shells available.

### ✅ Features Gosh Already Has
- Basic command execution and pipes
- Job control (fg, bg, jobs, &)
- I/O redirection (including 2>&1, &>)
- Command history with persistence
- Tab completion (basic)
- Environment variables
- Command substitution
- Wildcard expansion (*, ?, [...], {...})
- Here-documents and here-strings
- AND/OR operators (&&, ||)

### 🚀 Gosh's Unique Advantages
- **M28 Lisp Integration**: Full programming language embedded in shell
- **Record Streams** (planned): First-class structured data processing
- **Python-like Syntax**: More approachable than traditional shell scripting
- **Data-Oriented Design**: Built for modern JSON/API workflows

### ❌ Critical Features Missing (High Priority for Shell Parity)

#### 1. **Shell Scripting via M28** ✅ (Different Approach)
- [x] Control flow: M28 already has `if/elif/else`, `for`, `while`, `cond`, `case`
- [x] Functions: M28 has `def` with full Python-like capabilities
- [ ] Shell integration: Access to `$1`, `$2`, `$@`, `$*` from M28
- [ ] Exit status: Access to `$?` from M28 context
- [ ] Script debugging: M28 error messages and stack traces
- [ ] Source command: Load and execute .m28 files

#### 2. **Advanced Expansion and Substitution**
- [ ] Parameter expansion: `${var:-default}`, `${var:+alt}`, `${var:?error}`, `${var:=assign}`
- [ ] String manipulation: `${var#pattern}`, `${var%pattern}`, `${var/old/new}`
- [ ] Array expansion: `${array[@]}`, `${#array[@]}`
- [ ] Arithmetic expansion: `$((expression))`
- [ ] Brace expansion sequences: `{1..10}`, `{a..z}`

#### 3. **Directory Navigation**
- [ ] Directory stack (pushd, popd, dirs)
- [ ] CDPATH for quick navigation
- [ ] Auto-cd (type directory name to cd)
- [ ] Named directories (hash -d)
- [ ] Smart cd with fuzzy matching

#### 4. **Interactive Features**
- [ ] Command correction ("Did you mean...?")
- [ ] Shared history between sessions
- [ ] History substring search (Ctrl+R improvements)
- [ ] Syntax highlighting while typing
- [ ] Auto-suggestions based on history
- [ ] Programmable prompt with git status

#### 5. **Completion System**
- [ ] Context-aware completions
- [ ] Completion for command options/flags
- [ ] Customizable completion functions
- [ ] Remote file completion (scp, ssh)
- [ ] Git-aware completion
- [ ] Man page based completion

### 🔧 Nice-to-Have Features (Medium Priority)

#### 1. **Zsh Power Features**
- [ ] Associative arrays (hash maps)
- [ ] Floating-point arithmetic
- [ ] Extended globbing: `**/*.js`, `*.{jpg,png}`, `^pattern` (negation)
- [ ] Glob qualifiers: `*(.)` (files only), `*(/)` (dirs only), `*(m-7)` (modified in last week)
- [ ] Precommand modifiers: `noglob`, `nocorrect`
- [ ] Process substitution: `<()`, `>()`

#### 2. **Configuration and Customization**
- [ ] Multiple startup files (.goshenv, .goshrc, .goshlogin)
- [ ] Options system (setopt/unsetopt equivalents)
- [ ] Loadable modules
- [ ] Theme support
- [ ] Plugin architecture

#### 3. **Advanced Line Editing**
- [ ] Vi and Emacs modes with full keybindings
- [ ] Custom key bindings
- [ ] Multi-line editing with proper cursor movement
- [ ] Kill ring (yank/paste system)
- [ ] Undo/redo in command line

### 📊 Implementation Priority Matrix

Based on user impact and implementation complexity:

**Immediate Priority (Makes gosh usable as primary shell):**
1. M28 shell integration (access to $1, $2, $?, shell command execution)
2. Parameter expansion
3. Better completion system
4. Shared history
5. Directory stack

**Second Wave (Improves daily use):**
1. Command correction
2. Syntax highlighting
3. Advanced globbing
4. Git prompt integration
5. Arithmetic expansion

**Third Wave (Power user features):**
1. Associative arrays
2. Extended glob qualifiers
3. Loadable modules
4. Theme system
5. Advanced line editing

### 🎯 Strategic Approach

Rather than copying all zsh features, gosh should:
1. **Prioritize data processing**: Leverage M28 and record streams as the killer feature
2. **Modernize shell UX**: Better error messages, visual feedback, progressive disclosure
3. **Cherry-pick essentials**: Focus on the 20% of features that handle 80% of use cases
4. **Integrate with modern tools**: First-class JSON, API calls, cloud services
5. **M28 as the scripting language**: No need for bash/zsh scripting syntax - use M28's Python-like syntax for everything

This positions gosh not as "another zsh" but as "the shell for the API age" - combining traditional shell power with modern data processing capabilities.

### 📝 M28 Shell Scripting Examples

Instead of traditional shell scripts, gosh users will write M28:

```lisp
#!/usr/bin/env gosh -m

# Traditional shell script functionality in M28
(def main (args)
  # Access command line arguments
  (= script-name (first args))
  (= params (rest args))
  
  # Control flow
  (if (empty? params)
    (print "Usage: {script-name} <file>...")
    (for file in params
      (process-file file)))
  
  # Exit status
  (if success 0 1))

# Shell command execution from M28
(def process-file (file)
  (try
    # Execute shell commands and capture output
    (= result (shell "grep -n TODO {file}"))
    (if result.stdout
      (print "TODOs in {file}:\n{result.stdout}")
      (print "No TODOs in {file}"))
    (except ShellError as e
      (print "Error processing {file}: {e.message}")
      (= success False))))
```

This eliminates the need for learning arcane shell scripting syntax while providing more power and better error handling.

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
# System monitoring with alerts using M28's Python-like syntax
ps --records | 
[r for r in records if r["cpu"] > 80] |
group-by user |
(def summarize (groups)
  (for (user, procs) in (items groups)
    (yield {"user": user, 
            "cpu_total": (sum [p["cpu"] for p in procs]),
            "proc_count": (len procs)}))) |
[r for r in records if r["cpu_total"] > 200] |
to-alert "High CPU usage by user"

# Log analysis with M28 transformations  
from-log nginx.log |
(def parse_log (r)
  {**r,
   "response_time_ms": (float r["response_time"]),
   "hour": (datetime.strptime r["timestamp"] "%Y-%m-%d %H:%M:%S").hour}) |
(map parse_log records) |
group-by endpoint hour |
(def percentile (p values)
  (= sorted_vals (sorted values))
  (= index (int (len sorted_vals) * p / 100))
  sorted_vals[index]) |
aggregate p95:{(percentile 95 response_time_ms)} avg:{(average response_time_ms)} |
[r for r in records if r["p95"] > 1000] |
to-chart --x hour --y p95 --group-by endpoint

# Multi-source data join with M28 classes
(class RecordJoiner
  (def join (self key records1 records2)
    (= index {})
    (for r in records2
      (= index[r[key]] r))
    (for r in records1
      (if r[key] in index
        (yield {**r, **index[r[key]]}))))) |

parallel {
  docker ps --records | select container-id name cpu memory
  docker stats --records | select container-id network-io disk-io
} |
(= joiner (RecordJoiner)) |
(joiner.join "container-id") |
(map (lambda (r) {**r, "efficiency": r["cpu"] / r["memory"] if r["memory"] > 0 else 0}) records) |
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