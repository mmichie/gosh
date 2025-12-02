package m28adapter

import (
	"strings"
	"sync"
	"testing"
)

// MockEnvProvider implements EnvProvider for testing
type MockEnvProvider struct {
	mu            sync.Mutex
	cwd           string
	previousDir   string
	lastExitCode  int
	shellPID      int
	lastBgPID     int
	dirStack      []string
}

func (m *MockEnvProvider) GetCWD() string             { m.mu.Lock(); defer m.mu.Unlock(); return m.cwd }
func (m *MockEnvProvider) GetPreviousDir() string     { m.mu.Lock(); defer m.mu.Unlock(); return m.previousDir }
func (m *MockEnvProvider) GetLastExitStatus() int     { m.mu.Lock(); defer m.mu.Unlock(); return m.lastExitCode }
func (m *MockEnvProvider) GetShellPID() int           { m.mu.Lock(); defer m.mu.Unlock(); return m.shellPID }
func (m *MockEnvProvider) GetLastBackgroundPID() int  { m.mu.Lock(); defer m.mu.Unlock(); return m.lastBgPID }
func (m *MockEnvProvider) GetDirStack() []string      { m.mu.Lock(); defer m.mu.Unlock(); return m.dirStack }

// Test singleton interpreter to avoid M28 registration issues
var (
	testInterpreter     *Interpreter
	testInterpreterOnce sync.Once
	testMockEnv         *MockEnvProvider
	testMockJobs        func() []JobInfo
)

func getTestInterpreter() *Interpreter {
	testInterpreterOnce.Do(func() {
		testInterpreter = NewInterpreter()
		testMockEnv = &MockEnvProvider{
			cwd:          "/test/dir",
			previousDir:  "/test/old",
			lastExitCode: 0,
			shellPID:     12345,
			lastBgPID:    0,
			dirStack:     []string{"/test/dir"},
		}
		testMockJobs = func() []JobInfo {
			return []JobInfo{}
		}
		testInterpreter.RegisterGoshFunctions(DefaultShellExecutor, testMockJobs, testMockEnv)
	})
	return testInterpreter
}

func TestDefaultShellExecutor(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		wantStdout     string
		wantStderr     string
		wantExitCode   int
		wantErr        bool
	}{
		{
			name:         "simple echo",
			command:      "echo hello",
			wantStdout:   "hello\n",
			wantExitCode: 0,
		},
		{
			name:         "exit code 1",
			command:      "exit 1",
			wantExitCode: 1,
		},
		{
			name:         "exit code 42",
			command:      "exit 42",
			wantExitCode: 42,
		},
		{
			name:         "stderr output",
			command:      "echo error >&2",
			wantStderr:   "error\n",
			wantExitCode: 0,
		},
		{
			name:         "both stdout and stderr",
			command:      "echo out; echo err >&2",
			wantStdout:   "out\n",
			wantStderr:   "err\n",
			wantExitCode: 0,
		},
		{
			name:         "command not found",
			command:      "nonexistent_command_xyz",
			wantExitCode: 127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode, err := DefaultShellExecutor(tt.command)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantStdout != "" && stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}

			if tt.wantStderr != "" && stderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, tt.wantStderr)
			}

			if exitCode != tt.wantExitCode {
				t.Errorf("exitCode = %d, want %d", exitCode, tt.wantExitCode)
			}
		})
	}
}

func TestShellResult(t *testing.T) {
	interp := getTestInterpreter()

	// Test shell_result with a successful command
	result, err := interp.Execute(`(shell_result "echo hello")`)
	if err != nil {
		t.Fatalf("shell_result failed: %v", err)
	}

	// Result should be a dict representation
	if !strings.Contains(result, "stdout") || !strings.Contains(result, "hello") {
		t.Errorf("shell_result returned unexpected result: %s", result)
	}

	if !strings.Contains(result, "exit_code") || !strings.Contains(result, "0") {
		t.Errorf("shell_result should contain exit_code 0: %s", result)
	}
}

func TestShellPipeline(t *testing.T) {
	interp := getTestInterpreter()

	// Test shell_pipeline with multiple commands
	result, err := interp.Execute(`(shell_pipeline "echo hello world" "wc -w")`)
	if err != nil {
		t.Fatalf("shell_pipeline failed: %v", err)
	}

	// The word count of "hello world" should be 2
	if !strings.Contains(result, "2") {
		t.Errorf("shell_pipeline returned unexpected result: %s", result)
	}
}

func TestShellCwd(t *testing.T) {
	interp := getTestInterpreter()

	// Update mock env for this test
	testMockEnv.mu.Lock()
	testMockEnv.cwd = "/test/my-directory"
	testMockEnv.mu.Unlock()

	result, err := interp.Execute(`(shell_cwd)`)
	if err != nil {
		t.Fatalf("shell_cwd failed: %v", err)
	}

	if result != "/test/my-directory" {
		t.Errorf("shell_cwd returned %s, expected /test/my-directory", result)
	}

	// Reset mock env
	testMockEnv.mu.Lock()
	testMockEnv.cwd = "/test/dir"
	testMockEnv.mu.Unlock()
}

func TestShellOldpwd(t *testing.T) {
	interp := getTestInterpreter()

	// Update mock env for this test
	testMockEnv.mu.Lock()
	testMockEnv.previousDir = "/test/previous-dir"
	testMockEnv.mu.Unlock()

	result, err := interp.Execute(`(shell_oldpwd)`)
	if err != nil {
		t.Fatalf("shell_oldpwd failed: %v", err)
	}

	if result != "/test/previous-dir" {
		t.Errorf("shell_oldpwd returned %s, expected /test/previous-dir", result)
	}

	// Reset mock env
	testMockEnv.mu.Lock()
	testMockEnv.previousDir = "/test/old"
	testMockEnv.mu.Unlock()
}

func TestShellExitStatus(t *testing.T) {
	interp := getTestInterpreter()

	// Update mock env for this test
	testMockEnv.mu.Lock()
	testMockEnv.lastExitCode = 42
	testMockEnv.mu.Unlock()

	result, err := interp.Execute(`(shell_exit_status)`)
	if err != nil {
		t.Fatalf("shell_exit_status failed: %v", err)
	}

	if result != "42" {
		t.Errorf("shell_exit_status returned %s, expected 42", result)
	}

	// Reset mock env
	testMockEnv.mu.Lock()
	testMockEnv.lastExitCode = 0
	testMockEnv.mu.Unlock()
}

func TestShellPid(t *testing.T) {
	interp := getTestInterpreter()

	// Update mock env for this test
	testMockEnv.mu.Lock()
	testMockEnv.shellPID = 99999
	testMockEnv.mu.Unlock()

	result, err := interp.Execute(`(shell_pid)`)
	if err != nil {
		t.Fatalf("shell_pid failed: %v", err)
	}

	if result != "99999" {
		t.Errorf("shell_pid returned %s, expected 99999", result)
	}

	// Reset mock env
	testMockEnv.mu.Lock()
	testMockEnv.shellPID = 12345
	testMockEnv.mu.Unlock()
}

func TestShellBgPid(t *testing.T) {
	interp := getTestInterpreter()

	// Update mock env for this test
	testMockEnv.mu.Lock()
	testMockEnv.lastBgPID = 88888
	testMockEnv.mu.Unlock()

	result, err := interp.Execute(`(shell_bg_pid)`)
	if err != nil {
		t.Fatalf("shell_bg_pid failed: %v", err)
	}

	if result != "88888" {
		t.Errorf("shell_bg_pid returned %s, expected 88888", result)
	}

	// Reset mock env
	testMockEnv.mu.Lock()
	testMockEnv.lastBgPID = 0
	testMockEnv.mu.Unlock()
}

func TestShellDirstack(t *testing.T) {
	interp := getTestInterpreter()

	// Update mock env for this test
	testMockEnv.mu.Lock()
	testMockEnv.dirStack = []string{"/test/dir", "/home", "/tmp"}
	testMockEnv.mu.Unlock()

	result, err := interp.Execute(`(shell_dirstack)`)
	if err != nil {
		t.Fatalf("shell_dirstack failed: %v", err)
	}

	// Should return a list representation
	if !strings.Contains(result, "/test/dir") || !strings.Contains(result, "/home") || !strings.Contains(result, "/tmp") {
		t.Errorf("shell_dirstack returned unexpected result: %s", result)
	}

	// Reset mock env
	testMockEnv.mu.Lock()
	testMockEnv.dirStack = []string{"/test/dir"}
	testMockEnv.mu.Unlock()
}

func TestShellJobs(t *testing.T) {
	// For this test, we need a different jobs provider, but we can't change
	// the one registered with the singleton. Test with the empty jobs provider.
	interp := getTestInterpreter()

	result, err := interp.Execute(`(shell_jobs)`)
	if err != nil {
		t.Fatalf("shell_jobs failed: %v", err)
	}

	// With empty jobs, result should be an empty list
	if result != "[]" {
		t.Errorf("shell_jobs with no jobs should return [], got: %s", result)
	}
}

func TestShellAsync(t *testing.T) {
	interp := getTestInterpreter()

	// Test shell_async returns a task
	result, err := interp.Execute(`(shell_async "echo async_test")`)
	if err != nil {
		t.Fatalf("shell_async failed: %v", err)
	}

	// The result should be a task representation
	if !strings.Contains(result, "Task") {
		t.Errorf("shell_async should return a Task: %s", result)
	}
}
