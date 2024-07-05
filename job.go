package gosh

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
)

type Job struct {
	ID      int
	Command string
	Cmd     *exec.Cmd
	Status  string
}

type JobManager struct {
	jobs    map[int]*Job
	nextID  int
	mu      sync.Mutex
	fgJob   *Job
	fgJobMu sync.Mutex
}

func NewJobManager() *JobManager {
	return &JobManager{
		jobs:   make(map[int]*Job),
		nextID: 1,
	}
}

func (jm *JobManager) AddJob(command string, cmd *exec.Cmd) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job := &Job{
		ID:      jm.nextID,
		Command: command,
		Cmd:     cmd,
		Status:  "Running",
	}
	jm.jobs[job.ID] = job
	jm.nextID++

	return job
}

func (jm *JobManager) ListJobs() []*Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jobs := make([]*Job, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (jm *JobManager) GetJob(id int) (*Job, bool) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, exists := jm.jobs[id]
	return job, exists
}

func (jm *JobManager) RemoveJob(id int) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	delete(jm.jobs, id)
}

func (jm *JobManager) SetForegroundJob(job *Job) {
	jm.fgJobMu.Lock()
	defer jm.fgJobMu.Unlock()
	jm.fgJob = job
}

func (jm *JobManager) GetForegroundJob() *Job {
	jm.fgJobMu.Lock()
	defer jm.fgJobMu.Unlock()
	return jm.fgJob
}

func (jm *JobManager) StopForegroundJob() {
	jm.fgJobMu.Lock()
	defer jm.fgJobMu.Unlock()

	if jm.fgJob != nil {
		fmt.Printf("\nStopping job: [%d] %s\n", jm.fgJob.ID, jm.fgJob.Command)
		err := jm.fgJob.Cmd.Process.Signal(syscall.SIGTSTP)
		if err != nil {
			fmt.Printf("Error stopping job: %v\n", err)
		} else {
			jm.fgJob.Status = "Stopped"
			fmt.Printf("[%d]+ Stopped %s\n", jm.fgJob.ID, jm.fgJob.Command)
		}
		jm.fgJob = nil
	}
}

func (jm *JobManager) ForegroundJob(id int) error {
	job, exists := jm.GetJob(id)
	if !exists {
		return fmt.Errorf("job %d not found", id)
	}

	jm.SetForegroundJob(job)
	job.Status = "Foreground"

	fmt.Printf("Bringing job to foreground: [%d] %s\n", job.ID, job.Command)

	err := job.Cmd.Process.Signal(syscall.SIGCONT)
	if err != nil {
		return err
	}

	state, err := job.Cmd.Process.Wait()
	if err != nil {
		return err
	}

	jm.SetForegroundJob(nil)

	if state.Exited() {
		jm.RemoveJob(id)
		fmt.Printf("[%d]+ Done %s\n", job.ID, job.Command)
	} else {
		job.Status = "Stopped"
		fmt.Printf("[%d]+ Stopped %s\n", job.ID, job.Command)
	}

	return nil
}

func (jm *JobManager) BackgroundJob(id int) error {
	job, exists := jm.GetJob(id)
	if !exists {
		return fmt.Errorf("job %d not found", id)
	}

	job.Status = "Running"
	return job.Cmd.Process.Signal(syscall.SIGCONT)
}

func (jm *JobManager) ReapChildren() {
	for {
		pid, _ := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
		if pid <= 0 {
			break
		}

		jm.mu.Lock()
		for id, job := range jm.jobs {
			if job.Cmd.Process.Pid == pid {
				delete(jm.jobs, id)
				fmt.Printf("[%d]+ Done %s\n", job.ID, job.Command)
				break
			}
		}
		jm.mu.Unlock()
	}
}
