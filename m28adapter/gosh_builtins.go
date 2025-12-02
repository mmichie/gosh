// Package m28adapter provides gosh-specific M28 builtins
package m28adapter

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/mmichie/m28/core"
)

// ShellExecutor is a function that executes a shell command and returns stdout, stderr, and exit code
type ShellExecutor func(command string) (stdout string, stderr string, exitCode int, err error)

// JobInfo represents information about a shell job
type JobInfo struct {
	ID      int
	Command string
	Status  string
	PID     int
}

// JobsProvider provides access to shell job information
type JobsProvider func() []JobInfo

// EnvProvider provides access to shell environment
type EnvProvider interface {
	GetCWD() string
	GetPreviousDir() string
	GetLastExitStatus() int
	GetShellPID() int
	GetLastBackgroundPID() int
	GetDirStack() []string
}

// RegisterGoshFunctions registers gosh-specific functions with the M28 interpreter.
// This allows M28 expressions to interact with gosh's shell functionality.
func (i *Interpreter) RegisterGoshFunctions(executor ShellExecutor, jobs JobsProvider, env EnvProvider) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	// shell_result: Execute command and return dict with stdout, stderr, exit_code
	i.engine.DefineFunction("shell_result", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("shell_result: expected 1 argument, got %d", len(args))
		}

		cmdStr, ok := args[0].(core.StringValue)
		if !ok {
			return nil, fmt.Errorf("shell_result: expected string argument, got %T", args[0])
		}

		stdout, stderr, exitCode, err := executor(string(cmdStr))
		if err != nil && exitCode == 0 {
			// If there's an error but no exit code, return the error
			return nil, fmt.Errorf("shell_result: %v", err)
		}

		// Create result dictionary
		result := core.NewDict()
		result.Set(core.ValueToKey(core.StringValue("stdout")), core.StringValue(strings.TrimSuffix(stdout, "\n")))
		result.Set(core.ValueToKey(core.StringValue("stderr")), core.StringValue(strings.TrimSuffix(stderr, "\n")))
		result.Set(core.ValueToKey(core.StringValue("exit_code")), core.NumberValue(exitCode))

		return result, nil
	})

	// shell_async: Execute command asynchronously and return a Task
	i.engine.DefineFunction("shell_async", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("shell_async: expected 1 argument, got %d", len(args))
		}

		cmdStr, ok := args[0].(core.StringValue)
		if !ok {
			return nil, fmt.Errorf("shell_async: expected string argument, got %T", args[0])
		}

		command := string(cmdStr)

		// Create a function that will execute the shell command
		shellFunc := core.NewBuiltinFunction(func(innerArgs []core.Value, innerCtx *core.Context) (core.Value, error) {
			stdout, stderr, exitCode, err := executor(command)
			if err != nil && exitCode == 0 {
				return nil, fmt.Errorf("shell_async execution error: %v", err)
			}

			// Return the result dict
			result := core.NewDict()
			result.Set(core.ValueToKey(core.StringValue("stdout")), core.StringValue(strings.TrimSuffix(stdout, "\n")))
			result.Set(core.ValueToKey(core.StringValue("stderr")), core.StringValue(strings.TrimSuffix(stderr, "\n")))
			result.Set(core.ValueToKey(core.StringValue("exit_code")), core.NumberValue(exitCode))
			return result, nil
		})

		// Create and start a task
		task := core.NewTask(fmt.Sprintf("shell: %s", command), shellFunc, []core.Value{})
		task.Start(ctx)

		return task, nil
	})

	// shell_pipeline: Execute a pipeline of commands
	i.engine.DefineFunction("shell_pipeline", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("shell_pipeline: expected at least 1 argument")
		}

		// Collect commands from arguments (can be individual strings or a list)
		var commands []string

		// If single argument is a list, use that
		if len(args) == 1 {
			if list, ok := args[0].(*core.ListValue); ok {
				for i := 0; i < list.Len(); i++ {
					item, _ := list.GetItem(i)
					if str, ok := item.(core.StringValue); ok {
						commands = append(commands, string(str))
					} else {
						return nil, fmt.Errorf("shell_pipeline: list item %d is not a string", i)
					}
				}
			} else if str, ok := args[0].(core.StringValue); ok {
				// Single command - just execute it
				commands = []string{string(str)}
			} else {
				return nil, fmt.Errorf("shell_pipeline: expected string or list, got %T", args[0])
			}
		} else {
			// Multiple arguments - each should be a string command
			for i, arg := range args {
				if str, ok := arg.(core.StringValue); ok {
					commands = append(commands, string(str))
				} else {
					return nil, fmt.Errorf("shell_pipeline: argument %d is not a string", i)
				}
			}
		}

		if len(commands) == 0 {
			return nil, fmt.Errorf("shell_pipeline: no commands provided")
		}

		// Build the pipeline using shell piping
		pipelineCmd := strings.Join(commands, " | ")
		stdout, stderr, exitCode, err := executor(pipelineCmd)
		if err != nil && exitCode == 0 {
			return nil, fmt.Errorf("shell_pipeline: %v", err)
		}

		// Create result dictionary
		result := core.NewDict()
		result.Set(core.ValueToKey(core.StringValue("stdout")), core.StringValue(strings.TrimSuffix(stdout, "\n")))
		result.Set(core.ValueToKey(core.StringValue("stderr")), core.StringValue(strings.TrimSuffix(stderr, "\n")))
		result.Set(core.ValueToKey(core.StringValue("exit_code")), core.NumberValue(exitCode))

		return result, nil
	})

	// shell_jobs: Get list of current jobs
	i.engine.DefineFunction("shell_jobs", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_jobs: expected 0 arguments, got %d", len(args))
		}

		jobList := jobs()
		result := core.NewList()

		for _, job := range jobList {
			jobDict := core.NewDict()
			jobDict.Set(core.ValueToKey(core.StringValue("id")), core.NumberValue(job.ID))
			jobDict.Set(core.ValueToKey(core.StringValue("command")), core.StringValue(job.Command))
			jobDict.Set(core.ValueToKey(core.StringValue("status")), core.StringValue(job.Status))
			jobDict.Set(core.ValueToKey(core.StringValue("pid")), core.NumberValue(job.PID))
			result.Append(jobDict)
		}

		return result, nil
	})

	// shell_cwd: Get current working directory
	i.engine.DefineFunction("shell_cwd", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_cwd: expected 0 arguments, got %d", len(args))
		}
		return core.StringValue(env.GetCWD()), nil
	})

	// shell_oldpwd: Get previous working directory
	i.engine.DefineFunction("shell_oldpwd", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_oldpwd: expected 0 arguments, got %d", len(args))
		}
		return core.StringValue(env.GetPreviousDir()), nil
	})

	// shell_exit_status: Get last exit status ($?)
	i.engine.DefineFunction("shell_exit_status", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_exit_status: expected 0 arguments, got %d", len(args))
		}
		return core.NumberValue(env.GetLastExitStatus()), nil
	})

	// shell_pid: Get shell PID ($$)
	i.engine.DefineFunction("shell_pid", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_pid: expected 0 arguments, got %d", len(args))
		}
		return core.NumberValue(env.GetShellPID()), nil
	})

	// shell_bg_pid: Get last background PID ($!)
	i.engine.DefineFunction("shell_bg_pid", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_bg_pid: expected 0 arguments, got %d", len(args))
		}
		return core.NumberValue(env.GetLastBackgroundPID()), nil
	})

	// shell_dirstack: Get directory stack
	i.engine.DefineFunction("shell_dirstack", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("shell_dirstack: expected 0 arguments, got %d", len(args))
		}

		stack := env.GetDirStack()
		result := core.NewList()
		for _, dir := range stack {
			result.Append(core.StringValue(dir))
		}
		return result, nil
	})
}

// DefaultShellExecutor provides a default implementation that uses sh -c
func DefaultShellExecutor(command string) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.Command("sh", "-c", command)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	// Extract exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = 1
			}
			// Don't return the error since we have the exit code
			err = nil
		} else {
			// For other errors (command not found, etc.)
			exitCode = 127
		}
	}

	return stdout, stderr, exitCode, err
}
