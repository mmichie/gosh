package gosh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"gosh/parser"
)

func TestBackgroundJobExecution(t *testing.T) {
	// Skip in CI environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a job manager for the test
	jobManager := NewJobManager()

	// Set up a real process
	cmd := exec.Command("sleep", "1")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start background command: %v", err)
	}

	// Add the job to the job manager manually
	jobManager.AddJob("sleep 1", cmd)

	// Check that we have a job running
	jobs := jobManager.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}

	// Sleep briefly to allow job to complete
	time.Sleep(2 * time.Second)

	// The job might have completed by now, let's check
	jobManager.ReapChildren()

	// Wait for the process to finish
	cmd.Wait()
}

func TestJobsCommand(t *testing.T) {
	// Skip in CI environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a job manager for the test
	jobManager := NewJobManager()

	// Create a buffer to capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Set up a realistic test environment - use a real process
	cmd := exec.Command("sleep", "5")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start background command: %v", err)
	}

	// Add the job to the job manager manually
	jobManager.AddJob("sleep 5", cmd)

	// Call jobs function directly with a properly initialized Command
	jobsCmd := &Command{
		Stdout:     &stdout,
		Stderr:     &stderr,
		JobManager: jobManager,
	}

	// Call jobs directly
	err = jobs(jobsCmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check the output of the jobs command
	jobsOutput := stdout.String()
	if !strings.Contains(jobsOutput, "sleep 5") {
		t.Errorf("Expected jobs output to contain 'sleep 5', got: %s", jobsOutput)
	}
	if !strings.Contains(jobsOutput, "Running") {
		t.Errorf("Expected jobs output to contain 'Running', got: %s", jobsOutput)
	}

	// Clean up
	for _, job := range jobManager.ListJobs() {
		if job.Cmd != nil && job.Cmd.Process != nil {
			job.Cmd.Process.Kill()
		}
	}

	// Wait for the process to be killed
	cmd.Wait()
}

func TestBackgroundJobReporting(t *testing.T) {
	// Skip in CI or non-interactive environments
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "" {
		t.Skip("Skipping test in non-interactive environment")
	}

	// Create a job manager for the test
	jobManager := NewJobManager()

	// Create a buffer to capture output
	var stdout bytes.Buffer

	// Create a command directly
	cmd := exec.Command("sleep", "1")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start background command: %v", err)
	}

	// Add the job to the manager
	job := jobManager.AddJob("sleep 1", cmd)

	// Output the job info to the stdout buffer to simulate what the shell would do
	fmt.Fprintf(&stdout, "[%d] %d\n", job.ID, cmd.Process.Pid)

	// Check that a job ID was reported
	output := stdout.String()
	if !strings.Contains(output, "[1]") {
		t.Errorf("Expected job ID to be reported, got: %s", output)
	}

	// Sleep briefly to allow job to complete
	time.Sleep(2 * time.Second)

	// The job should have completed by now
	jobManager.ReapChildren()

	// Wait for the process to finish
	cmd.Wait()

	// The job should be removed from the job list
	jobs := jobManager.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs after completion, got %d", len(jobs))
	}
}

func TestJobsCommandWithNoJobs(t *testing.T) {
	// Create a buffer to capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create a jobs command - intentionally not setting JobManager to test nil handling
	jobsCmd := &Command{
		Command: &parser.Command{
			LogicalBlocks: []*parser.LogicalBlock{
				{
					FirstPipeline: &parser.Pipeline{
						Commands: []*parser.CommandElement{
							{Simple: &parser.SimpleCommand{
								Parts: []string{"jobs"},
							}},
						},
					},
				},
			},
		},
		Stdout: &stdout,
		Stderr: &stderr,
		// JobManager is nil - this tests our nil handling
	}

	// Call jobs command function directly
	err := jobs(jobsCmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check the output
	jobsOutput := stdout.String()
	if !strings.Contains(jobsOutput, "No background jobs") {
		t.Errorf("Expected 'No background jobs' message, got: %s", jobsOutput)
	}
}

func TestFgCommand(t *testing.T) {
	// Skip this test in automation as it requires terminal control
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "" {
		t.Skip("Skipping test that requires terminal control")
	}

	// Create a job manager for the test
	jobManager := NewJobManager()

	// Start a longer running background process
	cmd := exec.Command("sleep", "1") // Shorter time for faster tests
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start background command: %v", err)
	}

	// Add job to the manager
	jobManager.AddJob("sleep 1", cmd)

	// Sleep briefly to allow the job to be registered
	time.Sleep(100 * time.Millisecond)

	// Get the job ID
	jobs := jobManager.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}
	jobID := jobs[0].ID

	// Create a buffer to capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Call fg directly with a properly initialized Command
	fgCmd := &Command{
		Command: &parser.Command{
			LogicalBlocks: []*parser.LogicalBlock{
				{
					FirstPipeline: &parser.Pipeline{
						Commands: []*parser.CommandElement{
							{Simple: &parser.SimpleCommand{
								Parts: []string{"fg", fmt.Sprintf("%d", jobID)},
							}},
						},
					},
				},
			},
		},
		Stdout:     &stdout,
		Stderr:     &stderr,
		JobManager: jobManager,
	}

	// Run the fg command directly
	err = fg(fgCmd)
	if err != nil {
		// Clean up the process first
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		t.Fatalf("fg command failed: %v", err)
	}

	// The job should be gone from the job list
	jobs = jobManager.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs after fg, got %d", len(jobs))
	}
}
