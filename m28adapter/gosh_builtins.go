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

// RegisterRecordStreamFunctions registers M28 functions for record stream processing.
// These functions enable Lisp-based manipulation of record streams in gosh pipelines.
func (i *Interpreter) RegisterRecordStreamFunctions() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	// get-field: Get a field value from a record (dict)
	// (get-field record "fieldname")
	// (get-field record "nested/path") - supports nested access with /
	i.engine.DefineFunction("get-field", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("get-field: expected 2 arguments (record, field), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("get-field: first argument must be a dict, got %T", args[0])
		}

		fieldName, ok := args[1].(core.StringValue)
		if !ok {
			return nil, fmt.Errorf("get-field: second argument must be a string, got %T", args[1])
		}

		// Support nested field access with /
		parts := strings.Split(string(fieldName), "/")
		var current core.Value = dict

		for _, part := range parts {
			d, ok := current.(*core.DictValue)
			if !ok {
				return nil, nil
			}
			val, found := d.Get(part)
			if !found {
				return nil, nil
			}
			current = val
		}

		return current, nil
	})

	// set-field: Set a field value in a record, returning a new record
	// (set-field record "fieldname" value)
	i.engine.DefineFunction("set-field", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("set-field: expected 3 arguments (record, field, value), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("set-field: first argument must be a dict, got %T", args[0])
		}

		fieldName, ok := args[1].(core.StringValue)
		if !ok {
			return nil, fmt.Errorf("set-field: second argument must be a string, got %T", args[1])
		}

		// Create a copy of the dict with the new field
		newDict := core.NewDict()

		// Copy existing fields using Keys() and Get()
		for _, key := range dict.Keys() {
			val, _ := dict.Get(key)
			newDict.Set(key, val)
		}

		// Set the new/updated field
		newDict.Set(string(fieldName), args[2])

		return newDict, nil
	})

	// make-record: Create a new record (dict) from key-value pairs
	// (make-record "key1" val1 "key2" val2 ...)
	i.engine.DefineFunction("make-record", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args)%2 != 0 {
			return nil, fmt.Errorf("make-record: expected even number of arguments (key-value pairs), got %d", len(args))
		}

		result := core.NewDict()
		for i := 0; i < len(args); i += 2 {
			key, ok := args[i].(core.StringValue)
			if !ok {
				return nil, fmt.Errorf("make-record: key at position %d must be a string, got %T", i, args[i])
			}
			result.Set(string(key), args[i+1])
		}

		return result, nil
	})

	// record-keys: Get all keys from a record as a list
	// (record-keys record)
	i.engine.DefineFunction("record-keys", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("record-keys: expected 1 argument (record), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("record-keys: argument must be a dict, got %T", args[0])
		}

		result := core.NewList()
		for _, key := range dict.Keys() {
			result.Append(core.StringValue(key))
		}

		return result, nil
	})

	// record-values: Get all values from a record as a list
	// (record-values record)
	i.engine.DefineFunction("record-values", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("record-values: expected 1 argument (record), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("record-values: argument must be a dict, got %T", args[0])
		}

		result := core.NewList()
		for _, key := range dict.Keys() {
			val, _ := dict.Get(key)
			result.Append(val)
		}

		return result, nil
	})

	// record-has: Check if a record has a field
	// (record-has record "fieldname")
	i.engine.DefineFunction("record-has", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("record-has: expected 2 arguments (record, field), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("record-has: first argument must be a dict, got %T", args[0])
		}

		fieldName, ok := args[1].(core.StringValue)
		if !ok {
			return nil, fmt.Errorf("record-has: second argument must be a string, got %T", args[1])
		}

		_, found := dict.Get(string(fieldName))
		return core.BoolValue(found), nil
	})

	// record-remove: Remove a field from a record, returning a new record
	// (record-remove record "fieldname")
	i.engine.DefineFunction("record-remove", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("record-remove: expected 2 arguments (record, field), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("record-remove: first argument must be a dict, got %T", args[0])
		}

		fieldName, ok := args[1].(core.StringValue)
		if !ok {
			return nil, fmt.Errorf("record-remove: second argument must be a string, got %T", args[1])
		}

		fieldToRemove := string(fieldName)
		newDict := core.NewDict()

		for _, key := range dict.Keys() {
			if key != fieldToRemove {
				val, _ := dict.Get(key)
				newDict.Set(key, val)
			}
		}

		return newDict, nil
	})

	// record-merge: Merge two or more records into one
	// (record-merge record1 record2 ...)
	i.engine.DefineFunction("record-merge", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("record-merge: expected at least 2 arguments, got %d", len(args))
		}

		result := core.NewDict()

		for i, arg := range args {
			dict, ok := arg.(*core.DictValue)
			if !ok {
				return nil, fmt.Errorf("record-merge: argument %d must be a dict, got %T", i, arg)
			}

			for _, key := range dict.Keys() {
				val, _ := dict.Get(key)
				result.Set(key, val)
			}
		}

		return result, nil
	})

	// record-select: Select specific fields from a record
	// (record-select record "field1" "field2" ...)
	i.engine.DefineFunction("record-select", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("record-select: expected at least 2 arguments (record, field...), got %d", len(args))
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("record-select: first argument must be a dict, got %T", args[0])
		}

		// Build set of fields to select
		selectFields := make(map[string]bool)
		for i := 1; i < len(args); i++ {
			fieldName, ok := args[i].(core.StringValue)
			if !ok {
				return nil, fmt.Errorf("record-select: field name at position %d must be a string, got %T", i, args[i])
			}
			selectFields[string(fieldName)] = true
		}

		result := core.NewDict()
		for _, key := range dict.Keys() {
			if selectFields[key] {
				val, _ := dict.Get(key)
				result.Set(key, val)
			}
		}

		return result, nil
	})

	// record-update: Update multiple fields in a record at once
	// (record-update record "field1" val1 "field2" val2 ...)
	i.engine.DefineFunction("record-update", func(args []core.Value, ctx *core.Context) (core.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("record-update: expected at least 1 argument (record), got %d", len(args))
		}

		if (len(args)-1)%2 != 0 {
			return nil, fmt.Errorf("record-update: expected record followed by key-value pairs, got odd number of update args")
		}

		dict, ok := args[0].(*core.DictValue)
		if !ok {
			return nil, fmt.Errorf("record-update: first argument must be a dict, got %T", args[0])
		}

		// Create a copy of the dict
		result := core.NewDict()
		for _, key := range dict.Keys() {
			val, _ := dict.Get(key)
			result.Set(key, val)
		}

		// Apply updates
		for i := 1; i < len(args); i += 2 {
			key, ok := args[i].(core.StringValue)
			if !ok {
				return nil, fmt.Errorf("record-update: key at position %d must be a string, got %T", i, args[i])
			}
			result.Set(string(key), args[i+1])
		}

		return result, nil
	})
}
