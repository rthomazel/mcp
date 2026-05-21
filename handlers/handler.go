package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os/exec"
	"sync"
	"time"

	"github.com/rthomazel/bench-mcp/internal"
)

type job struct {
	id       string
	cmd      string
	started  time.Time
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	exitCode int
	done     bool
	err      string
	mu       sync.Mutex
}

type Handler struct {
	cfg     *internal.Config
	version string
	jobs    map[string]*job
	mu      sync.RWMutex
}

func New(cfg *internal.Config, version string) *Handler {
	h := &Handler{
		cfg:     cfg,
		version: version,
		jobs:    make(map[string]*job),
	}
	go h.removeJobsOlderThan(time.Hour)
	return h
}

func (h *Handler) removeJobsOlderThan(deadline time.Duration) {
	for range time.Tick(5 * time.Minute) {
		h.mu.Lock()
		for id, j := range h.jobs {
			j.mu.Lock()
			done, started := j.done, j.started
			j.mu.Unlock()
			if done && time.Since(started) > deadline {
				delete(h.jobs, id)
			}
		}
		h.mu.Unlock()
	}
}

// addJob assigns an ID and stores the job. ID generation and storage are done
// under a single write lock to prevent two concurrent calls from claiming the same ID.
func (h *Handler) addJob(j *job) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for {
		id := fmt.Sprintf("%04d", rand.IntN(10000))
		if _, exists := h.jobs[id]; !exists {
			j.id = id
			h.jobs[id] = j
			return
		}
	}
}

func (h *Handler) startJob(command, cwd string) *job {
	job := &job{
		cmd:     command,
		started: time.Now(),
	}
	h.addJob(job)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.cfg.BackgroundTimeout)
		defer cancel()

		slog.Info("job start", "job", job.id, "cmd", command, "cwd", cwd)

		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Dir = cwd

		job.mu.Lock()
		cmd.Stdout = &job.stdout
		cmd.Stderr = &job.stderr
		job.mu.Unlock()

		err := cmd.Run()

		job.mu.Lock()
		defer job.mu.Unlock()

		job.done = true

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				job.exitCode = exitErr.ExitCode()
			} else {
				job.err = fmt.Sprintf("could not start process: %v", err)
				job.exitCode = -1
			}
		}

		slog.Info("job done", "job", job.id, "exit_code", job.exitCode, "duration", time.Since(job.started).Round(time.Millisecond))
	}()

	return job
}
