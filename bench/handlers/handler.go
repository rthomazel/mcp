package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rthomazel/mcp/bench/internal"
	"github.com/rthomazel/mcp/bench/internal/stats"
)

type job struct {
	id         string
	cmd        string
	tool       string
	cwd        string
	setupPaths []string
	started    time.Time
	stdout     bytes.Buffer
	stderr     bytes.Buffer
	exitCode   int
	done       bool
	err        string
	mu         sync.Mutex
}

// jobOpts carries optional per-tool metadata for background jobs.
type jobOpts struct {
	tool       string
	setupPaths []string
}

type Handler struct {
	cfg     *internal.Config
	version string
	jobs    map[string]*job
	mu      sync.RWMutex
	stats   *stats.Writer // nil if stats are disabled
}

func New(cfg *internal.Config, version string) *Handler {
	h := &Handler{
		cfg:     cfg,
		version: version,
		jobs:    make(map[string]*job),
	}
	go h.removeJobsOlderThan(time.Hour)

	dbPath := filepath.Join(cfg.Home, "bench-mcp-stats.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		slog.Error("stats disabled: could not create db directory", "err", err)
		return h
	}
	key, err := stats.LoadKey()
	if err != nil {
		slog.Error("stats disabled: key load failed", "err", err)
		return h
	}
	w, err := stats.Open(dbPath, stats.WriterConfig{
		ServerVersion:  version,
		EncryptionKey:  key,
		RedactPatterns: cfg.StatsRedactPatterns,
	})
	if err != nil {
		slog.Error("stats disabled", "err", err)
		return h
	}
	h.stats = w
	return h
}

// Close flushes pending stats writes and closes the database.
func (h *Handler) Close() {
	if h.stats != nil {
		h.stats.Close()
	}
}

// record enqueues a tool call for async stats recording.
// A nil stats writer is silently ignored.
func (h *Handler) record(tc stats.ToolCall) {
	if h.stats == nil {
		return
	}
	h.stats.Record(tc)
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

func (h *Handler) startJob(command, cwd string, opts jobOpts) *job {
	job := &job{
		cmd:        command,
		tool:       opts.tool,
		cwd:        cwd,
		setupPaths: opts.setupPaths,
		started:    time.Now(),
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
		duration := time.Since(job.started)

		job.mu.Lock()
		defer job.mu.Unlock()

		job.done = true

		var errorKind string
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				job.exitCode = exitErr.ExitCode()
			} else {
				job.err = fmt.Sprintf("could not start process: %v", err)
				job.exitCode = -1
				errorKind = "start_failed"
			}
		}
		timedOut := ctx.Err() != nil
		if timedOut {
			errorKind = "timeout"
		}

		slog.Info("job done", "job", job.id, "exit_code", job.exitCode, "duration", duration.Round(time.Millisecond))

		exitCode := job.exitCode
		h.record(stats.ToolCall{
			Tool:       job.tool,
			StartedAt:  job.started,
			Duration:   duration,
			ErrorKind:  errorKind,
			Command:    job.cmd,
			ExitCode:   &exitCode,
			TimedOut:   timedOut,
			CWD:        job.cwd,
			JobID:      job.id,
			SetupPaths: job.setupPaths,
		})
	}()

	return job
}
