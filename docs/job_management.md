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

## Running Pipelines in the Background

You can also run entire pipelines in the background. Just add the ampersand (`&`) at the end of the pipeline. For example:

```bash
cat large_file.txt | grep "pattern" | sort -u > results.txt &
```

This starts the entire pipeline as a background job. All commands in the pipeline will be connected properly, and the shell will immediately return control to you while the pipeline runs in the background.

Pipeline background execution fully supports:
- Multiple commands in a pipeline (e.g., `cmd1 | cmd2 | cmd3 &`)
- File redirection (e.g., `cmd1 | cmd2 > output.txt &`)
- Complex pipelines with various commands and arguments

When the pipeline completes, you'll see a notification message the next time the shell displays a prompt, just like with simple background commands.

### Technical Details

In Gosh Shell, the background operator (`&`) at the end of a pipeline applies to the entire pipeline, not just the last command. Internally, when the background flag is detected on the last command, it is automatically propagated to the entire pipeline to ensure all commands in the pipeline are properly executed in the background as a single job.

The implementation handles several important aspects of pipeline background execution:

1. **Flag Propagation**: When a pipeline ends with `&`, the background flag is automatically propagated from the last command to the entire pipeline.

2. **Resource Management**: All pipes and file descriptors are properly managed, even when the pipeline runs in the background, preventing resource leaks.

3. **Pipeline Integrity**: Commands in the pipeline remain properly connected, ensuring data flows correctly between them while running in the background.

4. **Builtin Command Support**: The pipeline can include both external commands and shell builtins like `echo`, and everything will work correctly in the background.

5. **Redirection Support**: File redirection (`>`, `>>`, `<`) works properly with background pipelines, creating and writing to files as expected.

6. **Job Tracking**: The entire pipeline is tracked as a single job, with proper status updates when the pipeline completes.

This allows pipelines to maintain their data flow connections while running in the background, and ensures proper job management for the entire pipeline.

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
5. Enhanced pipeline executor that handles multi-command pipelines in background
6. Synchronized goroutines to monitor background job completion
7. Resource management to properly handle pipes and file descriptors

For pipeline background execution, the implementation works by:

1. Detecting the `&` symbol at the end of a pipeline during parsing
2. Setting up the pipeline with proper pipe connections between commands
3. Starting all commands in the pipeline
4. Adding the pipeline to the job manager as a single job
5. Launching a dedicated goroutine to monitor pipeline completion
6. Properly closing all resources when the pipeline completes
7. Notifying the user when the background pipeline job completes

This architecture ensures reliable execution of complex pipelines in the background while maintaining proper isolation, resource management, and job tracking.

Job IDs are assigned sequentially, starting from 1, as new background jobs are created.