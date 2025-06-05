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

### Medium Priority

□ **Shell Script Support**
   - Add ability to execute shell scripts from files
   - Implement basic flow control structures (if/else, loops)
   - Support for variables and basic script functions

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

□ **Array Support**
   - Implement array variables
   - Add array operations (indexing, slicing, iteration)

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

## Completed Features

- Basic command execution
- Pipe operator (`|`) for command chaining
- AND operator (`&&`) for conditional execution
- OR operator (`||`) for conditional execution
- Simple I/O redirection (`>`, `>>`, `<`)
- Environment variables
- Command history
- Basic tab completion
- Built-in commands
- M28 Lisp integration
- Command separators (`;`) for multiple commands
- Background jobs management (`&`, `jobs`, `fg`, `bg`)