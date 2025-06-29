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

## Bash Compatibility Gap

To understand what's needed for gosh to be a bash replacement, here's what's missing:

### 🔴 Critical Bash Features Not Yet in Gosh
1. **Parameter Expansion**: The `${var:-default}` family of expansions
2. **Arrays**: Both indexed `arr=(a b c)` and associative arrays
3. **History Expansion**: `!!`, `!$`, `^old^new`
4. **Arithmetic**: `$((2+2))`, `let`, `(( ))` constructs
5. **Aliases**: `alias` command for shortcuts
6. **Special Variables**: `$$`, `$!`, `$0`, positional parameters
7. **Startup Files**: `.bashrc`, `.bash_profile` equivalents

### 🟡 Bash Features with Different Approach in Gosh
1. **Control Flow**: Bash uses `if/then/fi`, gosh uses M28 Lisp
2. **Functions**: Bash functions vs M28 `def`
3. **Scripting**: Bash syntax vs M28 Python-like syntax

### 🟢 Bash Features Already in Gosh
- Pipes, redirection, job control
- Command substitution, wildcards
- Here-docs, logical operators
- Environment variables, history
- Directory navigation (cd, pushd/popd)

## Fish Compatibility Gap

Fish excels at user-friendliness with zero configuration. Here's what gosh needs to compete:

### 🔴 Critical Fish Features Not in Gosh
1. **Syntax Highlighting As You Type**: Immediate visual feedback
2. **Auto-suggestions**: Grayed-out completions from history
3. **Helpful Error Messages**: User-friendly, actionable errors
4. **Abbreviations**: Expand in-place (better than aliases)
5. **Web-based Configuration**: Visual config interface
6. **Universal Variables**: Settings that persist across sessions

### 🟡 Features Where Gosh Differs
1. **POSIX Compatibility**: Gosh maintains it, fish doesn't
2. **Scripting**: Fish's custom syntax vs M28's Python-like syntax
3. **Philosophy**: Data processing (gosh) vs interactive UX (fish)

### 🟢 Fish Features Already in Gosh
- Basic command execution and pipes
- Command history
- Tab completion (basic)
- Here-documents (fish added in v3.0)

## Zsh Compatibility Gap

To understand what's needed for gosh to compete with zsh, here's what's missing:

### 🔴 Critical Zsh Features Not in Gosh
1. **Advanced Globbing**: `**/*.js`, `^pattern`, `*(.)`, `*(m-7)`
2. **Extended Parameter Expansion**: Array operations, transformations
3. **Interactive Excellence**: Syntax highlighting, auto-suggestions, RPROMPT
4. **Smart Completion**: Context-aware, descriptions, menu selection
5. **Configuration System**: setopt/unsetopt, hooks, themes
6. **Advanced Line Editing**: ZLE with vi/emacs modes, widgets

### 🟡 Features Where Gosh Takes a Different Approach
1. **Scripting**: Zsh's complex syntax vs M28's Python-like clarity
2. **Data Processing**: Text-oriented (zsh) vs record-oriented (gosh)

### 🟢 Basic Features Already in Gosh
- Command execution, pipes, job control
- I/O redirection, command substitution
- Basic wildcards and completion
- Directory stack (pushd/popd)

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
- [ ] Floating-point arithmetic: `$(( 3.14 * 2 ))`
- [ ] Brace expansion sequences: `{1..10}`, `{a..z}`, `{01..20}`
- [ ] Special parameters: `$$` (PID), `$!` (last background PID), `$0` (script name)
- [ ] Recursive globbing: `**/*.js` (this is critical for modern development)

#### 3. **Shell Arrays** (Not replaced by M28)
- [ ] Indexed arrays: `arr=(one two three)`
- [ ] Array access: `${arr[0]}`, `${arr[@]}`, `${arr[*]}`
- [ ] Array slicing: `${arr[@]:1:2}`
- [ ] Array length: `${#arr[@]}`
- [ ] Array assignment: `arr[5]=value`
- [ ] Associative arrays: `declare -A map`

#### 4. **History Features**
- [ ] History expansion: `!!` (last command), `!$` (last arg), `!^` (first arg)
- [ ] History search: `!pattern` (last command matching pattern)
- [ ] History substitution: `^old^new` (replace in last command)
- [ ] Shared history between sessions
- [ ] History timestamps
- [ ] HISTCONTROL options (ignoredups, ignorespace)
- [ ] Ctrl+R reverse history search

#### 5. **Interactive Shell Features**
- [ ] Command aliases: `alias ll='ls -la'`
- [ ] Shell options: `set -o` / `set +o` (errexit, nounset, pipefail, etc.)
- [ ] Prompt customization: PS1, PS2, PS3, PS4 variables
- [ ] PROMPT_COMMAND for dynamic prompts
- [ ] Command correction ("Did you mean...?")
- [ ] Syntax highlighting while typing (fish-style immediate feedback)
- [ ] Auto-suggestions based on history (fish-style grayed-out completions)
- [ ] Abbreviations (fish-style): Expand in-place, better than aliases
- [ ] Helpful error messages: User-friendly, actionable feedback
- [ ] Universal variables: Persist settings across sessions

#### 6. **Directory Navigation**
- [x] Directory stack (pushd, popd, dirs)
- [x] CDPATH for quick navigation
- [ ] Auto-cd (type directory name to cd)
- [ ] Named directories (hash -d)
- [ ] Smart cd with fuzzy matching
- [ ] `cd -` history (not just previous dir)

#### 7. **Completion System**
- [ ] Context-aware completions
- [ ] Completion for command options/flags
- [ ] Programmable completion (`complete` command)
- [ ] Remote file completion (scp, ssh)
- [ ] Git-aware completion
- [ ] Man page based completion
- [ ] Hostname completion from known_hosts
- [ ] Variable name completion

#### 8. **Process and Job Control**
- [ ] Process substitution: `<(command)`, `>(command)`
- [ ] Coprocesses: `coproc name { command; }`
- [ ] Disown command to detach jobs
- [ ] Wait command with job specs
- [ ] Job notifications for background jobs
- [ ] SIGCHLD handling

#### 9. **Startup and Configuration**
- [ ] Startup files: `.goshrc`, `.gosh_profile`, `.goshenv`
- [ ] System-wide config: `/etc/goshrc`
- [ ] Login vs non-login shell distinction
- [ ] Interactive vs non-interactive detection
- [ ] RC file sourcing order
- [ ] Custom module/plugin loading

### 🔧 Nice-to-Have Features (Medium Priority)

#### 1. **Zsh-Style Advanced Globbing**
- [ ] Recursive globbing: `**/*.js` (search all subdirectories)
- [ ] Negation patterns: `^*.txt` (all except .txt files)
- [ ] Exclusion patterns: `*~*.bak` (exclude backup files)
- [ ] Glob qualifiers: `*(.)` (files only), `*(/)` (dirs only)
- [ ] Time-based qualifiers: `*(m-7)` (modified in last week), `*(mh+2)` (modified 2+ hours ago)
- [ ] Size qualifiers: `*(Lm+100)` (files larger than 100MB)
- [ ] Permission qualifiers: `*(f755)` (specific permissions)
- [ ] Sorting qualifiers: `*(om)` (order by modification), `*(oL)` (order by size)
- [ ] Numeric ranges in braces: `{01..99}` (zero-padded sequences)

#### 2. **Zsh-Style Interactive Features**
- [ ] RPROMPT: Right-side prompt support
- [ ] Spelling correction with CORRECT_ALL option
- [ ] AUTO_CD: Type directory name without cd command
- [ ] Menu completion: Interactive selection from completions
- [ ] Completion descriptions: Help text for each completion
- [ ] Completion grouping: Organize completions by type
- [ ] List colors: Colored file listings (LS_COLORS support)

#### 3. **Zsh-Style Arrays and Expansions**
- [ ] Array transformations: `${(U)array}` (uppercase), `${(L)array}` (lowercase)
- [ ] Array joining: `${(j:,:)array}` (join with delimiter)
- [ ] String splitting: `${(s:,:)string}` (split by delimiter)
- [ ] Unique elements: `${(u)array}` (remove duplicates)
- [ ] Reverse array: `${(Oa)array}`
- [ ] Nested expansions: `${${var#prefix}%suffix}`
- [ ] Conditional expansions: `${var:+word}`, `${var:?error}`

#### 4. **Configuration and Customization**
- [ ] Multiple startup files (.goshenv, .goshrc, .goshlogin)
- [ ] Options system (setopt/unsetopt equivalents):
  - [ ] EXTENDED_GLOB: Enable advanced globbing
  - [ ] SHARE_HISTORY: Share history between sessions
  - [ ] HIST_IGNORE_DUPS: Don't save duplicate commands
  - [ ] AUTO_PUSHD: Make cd push to directory stack
  - [ ] CORRECT: Command correction
  - [ ] GLOB_DOTS: Include dotfiles in globs
- [ ] Hook functions:
  - [ ] precmd: Execute before each prompt
  - [ ] preexec: Execute before each command
  - [ ] chpwd: Execute on directory change
  - [ ] periodic: Execute periodically
- [ ] Loadable modules (datetime, regex, math functions)
- [ ] Theme support with prompt themes
- [ ] Plugin architecture

#### 5. **Advanced Line Editing (ZLE)**
- [ ] Vi and Emacs modes with full keybindings
- [ ] Custom widgets and key bindings
- [ ] Multi-line editing with proper cursor movement
- [ ] Kill ring (clipboard history)
- [ ] Undo/redo in command line
- [ ] Visual selection mode
- [ ] Incremental search in command line
- [ ] Transpose words/characters

#### 6. **Zsh-Style Aliases and Functions**
- [ ] Suffix aliases: `alias -s py=python` (auto-execute .py files)
- [ ] Global aliases: `alias -g L='| less'` (expand anywhere)
- [ ] Function autoloading from fpath
- [ ] Anonymous functions: `() { echo $1 } arg`
- [ ] Function tracing and profiling

#### 7. **Advanced Completion Features**
- [ ] Completion styles (menu, list, etc.)
- [ ] Completion matching control (case-insensitive, fuzzy)
- [ ] Completion caching for slow completions
- [ ] Dynamic completion updates
- [ ] Completion preview
- [ ] Smart case matching
- [ ] Partial word completion
- [ ] Approximate completion (typo correction)
- [ ] Completion descriptions (fish-style)
- [ ] Git-aware completions with status info

#### 8. **Fish-Inspired Features**
- [ ] Web-based configuration interface
- [ ] Event system: `--on-event`, `--on-variable`
- [ ] Built-in math command: `math "2 + 2"`
- [ ] String manipulation commands: `string split`, `string join`
- [ ] Random command for generating random values
- [ ] Status command for checking last command success
- [ ] Variable scoping: `-l` (local), `-g` (global), `-x` (export)
- [ ] List variables with indexing: `$list[1]`, `$list[2..-1]`
- [ ] Path variables without colons: `set PATH $PATH /new/path`
- [ ] Function autoloading from directories
- [ ] Key bindings system: `bind \cr 'command'`
- [ ] Right prompt support (fish_right_prompt)

### 📊 Implementation Priority Matrix

Based on user impact and implementation complexity:

**Immediate Priority (Makes gosh usable as primary shell):**
1. Parameter expansion (`${var:-default}`, `${var#pattern}`, etc.)
2. Shell arrays (indexed and associative)
3. History features (`!!`, `!$`, Ctrl+R search)
4. Syntax highlighting while typing (fish-style immediate feedback)
5. Auto-suggestions from history (fish-style grayed-out)
6. Command aliases and abbreviations
7. Arithmetic expansion `$((...))`
8. Recursive globbing `**/*.js` (essential for modern development)
9. M28 shell integration (access to $1, $2, $?, shell command execution)

**Second Wave (Improves daily use):**
1. Better completion system (programmable, git-aware, with descriptions)
2. Startup files (.goshrc, .gosh_profile)
3. Prompt customization (PS1, PROMPT_COMMAND, RPROMPT)
4. Helpful error messages (fish-style user-friendly)
5. History expansion and sharing
6. Brace expansion sequences `{1..10}`, `{01..20}`
7. Process substitution `<(...)`, `>(...)`
8. AUTO_CD option (type directory name to cd)
9. Basic glob qualifiers `*(.)`, `*(/)`

**Third Wave (Power user features):**
1. Universal variables (fish-style persistent settings)
2. Command spelling correction
3. Advanced glob qualifiers (time, size, permissions)
4. Advanced line editing (vi/emacs modes)
5. Shell options system (setopt/unsetopt)
6. Hook functions (precmd, preexec, chpwd)
7. Web-based configuration (fish-style)
8. Event system for automation

**Fourth Wave (Nice to have):**
1. Extended glob qualifiers
2. Named directories
3. Plugin architecture
4. Theme system
5. Remote file completion

### 🎯 Strategic Approach

Rather than copying all features from other shells, gosh should:
1. **Prioritize data processing**: Leverage M28 and record streams as the killer feature
2. **Modernize shell UX**: Better error messages, visual feedback, progressive disclosure
3. **Cherry-pick essentials**: Focus on the 20% of features that handle 80% of use cases
4. **Integrate with modern tools**: First-class JSON, API calls, cloud services
5. **M28 as the scripting language**: No need for bash/zsh scripting syntax - use M28's Python-like syntax for everything
6. **Learn from fish's UX philosophy**: 
   - Sane defaults that work out of the box
   - Helpful, user-friendly error messages
   - Visual feedback (syntax highlighting, auto-suggestions)
   - Zero-configuration productivity

This positions gosh not as "another zsh" but as "the shell for the API age" - combining fish's excellent UX, traditional shell power, and modern data processing capabilities.

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