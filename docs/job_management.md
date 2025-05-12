# Job Management in Gosh Shell

This document describes the job management capabilities in Gosh Shell, including how to run background processes, manage them, and bring them to the foreground.

## Running Commands in the Background

To run a command in the background, append an ampersand (`&`) to the end of the command. For example:

```bash
sleep 60 &
```

This starts the `sleep` command as a background job and immediately returns control to the shell, allowing you to continue entering commands. When a job is started in the background, Gosh Shell will print its job ID and process ID.

Example output:
```
[1] 12345
```

Where `1` is the job ID (internal to the shell) and `12345` is the process ID (PID) in the operating system.

## Listing Jobs

To see a list of all background jobs that are currently active, use the `jobs` command:

```bash
jobs
```

This will list all background jobs, showing their job ID, status, and the command itself.

Example output:
```
[1]+ Running        sleep 60
```

The `+` symbol indicates the current (most recent) job, while `-` would indicate the previous job.

## Job Control

Gosh Shell supports the following job control commands:

### Bringing a Job to the Foreground

To bring a background job to the foreground (which means the shell will wait for it to complete), use the `fg` command followed by the job ID:

```bash
fg 1
```

If you don't specify a job ID, `fg` will bring the most recent job to the foreground.

### Resuming Jobs in the Background

If a job has been stopped (for example, by pressing `Ctrl+Z`), you can resume it in the background using the `bg` command:

```bash
bg 1
```

If you don't specify a job ID, `bg` will resume the most recently stopped job in the background.

## Signals and Job Control

Gosh Shell handles the following signals for job control:

- `Ctrl+C` (SIGINT): Sends the interrupt signal to the foreground process
- `Ctrl+Z` (SIGTSTP): Stops the foreground process
- `Ctrl+\` (SIGQUIT): Sends the quit signal to the foreground process

When a background job completes, Gosh Shell will notify you the next time it shows a prompt:

```
[1]+ Done           sleep 60
```

## Examples

Here are some examples of job management in Gosh Shell:

```bash
# Start a command in the background
sleep 60 &

# List all jobs
jobs

# Bring a job to the foreground
fg 1

# Stop the foreground job with Ctrl+Z

# Resume the stopped job in the background
bg 1

# Bring the job back to the foreground
fg
```

## Implementation Details

Job management in Gosh Shell is implemented via several components:

1. A `JobManager` that tracks all background processes
2. The `&` operator in the parser that marks a command to run in the background
3. Built-in commands (`jobs`, `fg`, `bg`) for job control
4. Signal handling to manage processes properly

Job IDs are assigned sequentially, starting from 1, as new background jobs are created.