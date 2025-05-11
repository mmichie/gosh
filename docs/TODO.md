# gosh Shell TODO List

This document outlines the remaining features and improvements needed for the gosh shell implementation.

## Next Implementation Tasks

The following tasks have been identified as the next items to implement, based on examination of the codebase:

✅ **Fixed: CD with Dash**
   - Implemented a fix for the `cd -` command to correctly change to the previous directory
   - Updated `global_state.go` to properly initialize and maintain the previous directory
   - Enhanced `builtins.go` to handle edge cases in the `cd` function implementation
   - Added a dedicated test case in `cd_test.go` to verify proper functionality
   - Ensured cross-platform compatibility by handling symlinks (e.g., `/var` vs `/private/var` on macOS)

□ **Fix File Redirection Issues**
   - Current issue: Problems with file creation and output redirection in tests
   - In `command.go`: Review the `defer outputFile.Close()` which might be closing files too early
   - Ensure file paths are resolved correctly before operations
   - Add better error handling for file operations

□ **Implement OR Operator (||)**
   - Current issue: Shell doesn't support OR operators for conditional execution
   - Update `parser/parser.go`: Add "Or" to the lexer rules with pattern `\|\|`
   - Modify the command structures to support OR conditional chains
   - Update command execution to run the second command only if the first fails

## Core Shell Features

### High Priority

□ **Command Separators**
   - Implement command separators (`;`) to allow multiple independent commands in a single line
   - Add proper parsing and execution of separated commands

### Medium Priority

□ **Background Job Improvements**
   - Enhance background job management with proper job control
   - Implement the `&` operator to run commands in the background
   - Add support for job listing, foreground/background switching

□ **Shell Script Support**
   - Add ability to execute shell scripts from files
   - Implement basic flow control structures (if/else, loops)
   - Support for variables and basic script functions

□ **Wildcard Expansion**
   - Improve glob pattern support for file matching
   - Add support for common wildcard patterns (`*`, `?`, `[...]`)

□ **Command Substitution**
   - Implement command substitution using backticks or `$(command)` syntax
   - Allow output of one command to be used as arguments for another

### Low Priority

□ **Environment Variable Expansion**
   - Enhance environment variable support
   - Add variable substitution in more contexts

□ **Signal Handling**
    - Improve handling of various signals (SIGINT, SIGTSTP, etc.)
    - Add custom signal handlers for better shell control

□ **Tab Completion Enhancements**
    - Expand tab completion to handle more complex scenarios
    - Add completion for command options and arguments

## M28 Lisp Integration

□ **Lisp Function Library**
    - Expand the standard library of Lisp functions
    - Add more shell integration functions

□ **Lisp Error Handling**
    - Improve error reporting for Lisp expressions
    - Add better debugging support for Lisp code

□ **Interactive Lisp Environment**
    - Add a REPL mode specifically for Lisp interaction

## User Experience

□ **Better Error Messages**
    - Improve error reporting with more context
    - Add suggestions for common errors

□ **Configuration System**
    - Add support for a config file (similar to .bashrc)
    - Allow customization of shell behavior through config

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

## Completed Features

- Basic command execution
- Pipe operator (`|`) for command chaining
- AND operator (`&&`) for conditional execution
- Simple I/O redirection (`>`, `>>`, `<`)
- Environment variables
- Command history
- Basic tab completion
- Built-in commands
- M28 Lisp integration