// Package gosh provides builtin commands with record stream output support.
//
// These enhanced builtins can output structured records when given the --records flag:
//   - ls --records    - List directory contents as records
//   - env --records   - List environment variables as records
//   - ps --records    - List processes as records
package gosh

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func init() {
	// Register record-enabled builtins
	builtins["ls"] = lsCommand
	builtins["ps"] = psCommand

	// Wrap existing env to support --records
	originalEnv := builtins["env"]
	builtins["env"] = func(cmd *Command) error {
		if hasRecordsFlag(cmd) {
			return envRecords(cmd)
		}
		return originalEnv(cmd)
	}
}

// hasRecordsFlag checks if --records flag is present in the command.
func hasRecordsFlag(cmd *Command) bool {
	args := getBuiltinArgs(cmd)
	for _, arg := range args {
		if arg == "--records" || arg == "-R" {
			return true
		}
	}
	return false
}

// removeRecordsFlag removes the --records flag from args.
func removeRecordsFlag(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--records" && arg != "-R" {
			result = append(result, arg)
		}
	}
	return result
}

// lsCommand lists directory contents, with optional record stream output.
// Usage: ls [options] [path]
// Options:
//
//	-a, --all        Include hidden files
//	-l               Long format (more detail)
//	--records, -R    Output as record stream
func lsCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	// Parse options
	recordMode := false
	showHidden := false
	longFormat := false
	var targetPath string

	for _, arg := range args {
		switch arg {
		case "-a", "--all":
			showHidden = true
		case "-l":
			longFormat = true
		case "--records", "-R":
			recordMode = true
		default:
			if !strings.HasPrefix(arg, "-") {
				targetPath = arg
			}
		}
	}

	// Determine path
	if targetPath == "" {
		targetPath = "."
	}

	// Make path absolute
	if !filepath.IsAbs(targetPath) {
		gs := GetGlobalState()
		targetPath = filepath.Join(gs.GetCWD(), targetPath)
	}

	// Check if path is a file
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("ls: %v", err)
	}

	// If it's a file, list just that file
	if !fileInfo.IsDir() {
		if recordMode {
			fmt.Fprint(cmd.Stdout, RecordMagic)
			record := Record{
				"name":    filepath.Base(targetPath),
				"size":    fileInfo.Size(),
				"mode":    fileInfo.Mode().String(),
				"modtime": fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				"isdir":   false,
				"perm":    fmt.Sprintf("%04o", fileInfo.Mode().Perm()),
			}
			data, _ := json.Marshal(record)
			fmt.Fprintln(cmd.Stdout, string(data))
			return nil
		}
		if longFormat {
			fmt.Fprintf(cmd.Stdout, "%s  %8d  %s  %s\n",
				fileInfo.Mode().String(),
				fileInfo.Size(),
				fileInfo.ModTime().Format("Jan 02 15:04"),
				filepath.Base(targetPath))
		} else {
			fmt.Fprintln(cmd.Stdout, filepath.Base(targetPath))
		}
		return nil
	}

	// Read directory
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return fmt.Errorf("ls: %v", err)
	}

	// Record mode
	if recordMode {
		fmt.Fprint(cmd.Stdout, RecordMagic)

		for _, entry := range entries {
			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			record := Record{
				"name":    name,
				"size":    info.Size(),
				"mode":    info.Mode().String(),
				"modtime": info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				"isdir":   entry.IsDir(),
				"perm":    fmt.Sprintf("%04o", info.Mode().Perm()),
			}

			data, _ := json.Marshal(record)
			fmt.Fprintln(cmd.Stdout, string(data))
		}
		return nil
	}

	// Text mode
	if longFormat {
		for _, entry := range entries {
			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			fmt.Fprintf(cmd.Stdout, "%s  %8d  %s  %s\n",
				info.Mode().String(),
				info.Size(),
				info.ModTime().Format("Jan 02 15:04"),
				name)
		}
	} else {
		// Simple list - one file per line
		for _, entry := range entries {
			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}
			fmt.Fprintln(cmd.Stdout, name)
		}
	}

	return nil
}

// envRecords outputs environment variables as records.
func envRecords(cmd *Command) error {
	fmt.Fprint(cmd.Stdout, RecordMagic)

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		record := Record{
			"name":  parts[0],
			"value": parts[1],
		}

		data, _ := json.Marshal(record)
		fmt.Fprintln(cmd.Stdout, string(data))
	}

	return nil
}

// psCommand lists processes with optional record stream output.
// This is a simplified version since Go doesn't have direct process listing.
// Usage: ps [options]
// Options:
//
//	--records, -R    Output as record stream
func psCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)
	recordMode := false

	for _, arg := range args {
		if arg == "--records" || arg == "-R" {
			recordMode = true
		}
	}

	// Get current process info
	pid := os.Getpid()
	ppid := os.Getppid()

	currentUser, _ := user.Current()
	username := ""
	if currentUser != nil {
		username = currentUser.Username
	}

	// For record mode, output process info as records
	if recordMode {
		fmt.Fprint(cmd.Stdout, RecordMagic)

		// Current shell process
		record := Record{
			"pid":     pid,
			"ppid":    ppid,
			"user":    username,
			"command": "gosh",
			"state":   "R",
		}
		data, _ := json.Marshal(record)
		fmt.Fprintln(cmd.Stdout, string(data))

		// List jobs if available
		if cmd.JobManager != nil {
			for _, job := range cmd.JobManager.ListJobs() {
				jobPid := 0
				if job.Cmd != nil && job.Cmd.Process != nil {
					jobPid = job.Cmd.Process.Pid
				}

				record := Record{
					"pid":     jobPid,
					"ppid":    pid,
					"user":    username,
					"command": job.Command,
					"state":   job.Status,
					"job_id":  job.ID,
				}
				data, _ := json.Marshal(record)
				fmt.Fprintln(cmd.Stdout, string(data))
			}
		}

		return nil
	}

	// Text mode output (simplified)
	fmt.Fprintf(cmd.Stdout, "%5s %5s %s\n", "PID", "PPID", "COMMAND")
	fmt.Fprintf(cmd.Stdout, "%5d %5d %s\n", pid, ppid, "gosh")

	// List jobs
	if cmd.JobManager != nil {
		for _, job := range cmd.JobManager.ListJobs() {
			jobPid := 0
			if job.Cmd != nil && job.Cmd.Process != nil {
				jobPid = job.Cmd.Process.Pid
			}
			fmt.Fprintf(cmd.Stdout, "%5d %5d %s\n", jobPid, pid, job.Command)
		}
	}

	return nil
}

// getNumCPUs returns the number of CPUs.
func getNumCPUs() int {
	return runtime.NumCPU()
}

// parseSize parses size strings like "100", "1K", "1M", "1G".
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	multiplier := int64(1)
	if len(s) > 0 {
		suffix := strings.ToUpper(s[len(s)-1:])
		switch suffix {
		case "K":
			multiplier = 1024
			s = s[:len(s)-1]
		case "M":
			multiplier = 1024 * 1024
			s = s[:len(s)-1]
		case "G":
			multiplier = 1024 * 1024 * 1024
			s = s[:len(s)-1]
		}
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}

	return val * multiplier, nil
}
