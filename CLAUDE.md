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

# Run specific test file
go test ./... -run TestName

# Generate test coverage
make coverage

# Run background job tests
make test-background
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

2. **Parser**: In the `parser` package, the shell parser uses the `participle` library to parse command strings into structured command objects, supporting features like pipes (`|`), logical operators (`&&`, `||`), semicolons (`;`), and I/O redirection.

3. **Builtins**: Built-in shell commands (`cd`, `pwd`, `echo`, etc.) are implemented in `builtins.go` and include core shell functionality.

4. **Job Management**: The `JobManager` in `job.go` handles shell job control, including foreground and background processes, job status tracking, and signals.

5. **Database Storage**: The `Database` type in `db.go` uses SQLite to store command history, session information, and argument usage for smart completion.

6. **Smart Completion**: Tab completion functionality combines filesystem suggestions with argument history tracking (`smart_completion.go` and `arg_history.go`) to provide context-aware completions.

7. **M28 Integration**: The shell integrates with the external `m28` Lisp interpreter via the `m28adapter` package, allowing Lisp expressions (like `(+ 1 2)`) to be evaluated directly in the shell.

8. **Here-Documents**: Support for here-documents (`<<`) and here-strings (`<<<`) is implemented in `heredoc.go` with preprocessing before parsing.

9. **Command Substitution**: Command substitution using `$(...)` and backticks is handled in `cmd_substitution.go`.

10. **Global State**: The `GlobalState` type in `global_state.go` manages environment variables, current working directory, and other shell state.

11. **Directory Navigation**: Advanced directory navigation with `pushd`/`popd`/`dirs` and CDPATH support for quick directory switching.

### Command Execution Flow

1. Command string is received from the user
2. Here-documents are preprocessed and extracted (`PreprocessHereDoc`)
3. Command substitution is performed (`PerformCommandSubstitution`)
4. M28 Lisp expressions are evaluated (`evaluateM28InCommand`)
5. `parser.Parse()` converts the string into a structured command with:
   - LogicalBlocks (separated by `;`)
   - Pipelines (separated by `|`)
   - Logical operators (`&&`, `||`)
6. The `Command.Run()` method executes each logical block sequentially
7. For each block, pipelines are executed with logical operator short-circuiting
8. If the command is a built-in, it's executed directly; otherwise, it's executed as an external command
9. Command completion is recorded in the database for smart completion

### Important Implementation Details

- **Parser Structure**: Commands use a hierarchical structure: `Command` → `LogicalBlock` → `Pipeline` → `SimpleCommand`. Each level handles different operators (`;`, `&&`/`||`, `|`).

- **M28 Evaluation**: M28 Lisp expressions are evaluated in two places:
  1. During command preprocessing (before parsing) - embedded expressions like `echo (+ 1 2)`
  2. During pipeline execution - standalone Lisp commands

- **Pipeline Execution**: The `executePipeline` method in `command.go` handles all pipeline execution, including:
  - Single commands with redirections
  - Multi-command pipelines with pipes
  - stderr redirection (2>, 2>>)
  - File descriptor duplication (2>&1)
  - Combined redirections (&>, >&)
  - Background job execution

- **Smart Completion**: Uses frequency-based ranking from SQLite database. Tab completion cycles through candidates combining both filesystem entries and historical arguments.

- **Background Jobs**: Commands ending with `&` are tracked by `JobManager`. The shell monitors their completion and reports status asynchronously.