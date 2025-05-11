# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Running
```bash
# Compile and run the application
make compile
make run

# Or do both at once
make all
```

### Testing
```bash
# Run all tests
make test

# Generate test coverage
make coverage
```

### Code Formatting
```bash
# Format Go code
make format
```

### Cleaning
```bash
# Clean up build artifacts
make clean
```

## Architecture Overview

Gosh is a Go-based shell implementation that provides basic shell functionality with some additional features. It uses a custom parser along with an external M28 Lisp interpreter.

### Key Components

1. **Command Execution**: The `Command` type in `command.go` handles command execution, including parsing, running builtin commands, managing pipelines, and handling I/O redirection.

2. **Parser**: In the `parser` package, the shell parser uses the `participle` library to parse command strings into structured command objects, supporting features like pipes (`|`), logical AND (`&&`), and I/O redirection.

3. **Builtins**: Built-in shell commands (`cd`, `pwd`, `echo`, etc.) are implemented in `builtins.go` and include core shell functionality.

4. **Job Management**: The `JobManager` in `job.go` handles shell job control, including foreground and background processes, job status tracking, and signals.

5. **History**: Command history tracking is implemented in `history.go`, allowing for persistent command history across sessions.

6. **Completion**: Tab completion functionality is provided in `completion.go`.

7. **M28 Integration**: The shell integrates with the external `m28` Lisp interpreter via the `m28adapter` package, allowing Lisp expressions to be executed within the shell.

8. **Global State**: The `GlobalState` type in `global_state.go` manages environment variables and other shell state.

9. **Prompt Customization**: The shell supports customizable prompts with variables like username, hostname, working directory.

### Command Execution Flow

1. Command string is received from the user
2. `parser.Parse()` converts the string into a structured command
3. The `Command.Run()` method executes the command
4. If the command is a built-in, it's executed directly; otherwise, it's executed as an external command
5. Pipelines connect commands together, with output from one command feeding into the input of the next
6. M28 Lisp expressions are detected and evaluated when present

### Recent Changes

Based on the git history, recent changes include:
- Switching to an external M28 interpreter (from a previous internal implementation)
- Improvements to handle `cd -`, redirection, and Lisp loop functionality