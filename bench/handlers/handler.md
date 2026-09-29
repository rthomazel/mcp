# job

The tool tracks background jobs and records tool-call statistics.

# Types

## job

1. id string
2. cmd string
3. tool string
4. cwd string
5. setupPaths []string
6. started time.Time
7. stdout bytes.Buffer
8. stderr bytes.Buffer
9. exitCode int
10. done bool
11. err string
12. mu sync.Mutex

## jobOpts

1. tool string
2. setupPaths []string
3. nice bool

## Handler

1. cfg *internal.Config
2. version string
3. jobs map[string]*job
4. mu sync.RWMutex
5. stats *stats.Writer, nil when stats are disabled

# Functions

## New(cfg, version) *Handler

1. Allocate the handler and launch removeJobsOlderThan.
2. Create the stats database directory, logging and returning a disabled handler on failure.
3. Load the encryption key, returning a disabled handler on failure.
4. Open the stats database and start the write goroutine, returning a disabled handler on failure.
5. Store the writer and return the handler.

## Handler.Close()

1. Close the stats writer when present.

## Handler.Record(tc)

1. Return immediately when the writer is nil.
2. Otherwise enqueue the tool call.

## Handler.removeJobsOlderThan(deadline)

1. Loop on a fixed interval.
2. Lock the handler, iterate jobs, and delete any done job older than the deadline.

## Handler.addJob(j)

1. Take a write lock.
2. Generate a random 4-digit ID until one is not already present, store the job, and release the lock.

## buildJobCommand(ctx, cfg, command, nice) *exec.Cmd

1. Wrap the command in nice when nice is set and BackgroundNice is positive, so the whole process tree runs at a lower priority.
2. Otherwise run the command directly.

## Handler.startJob(command, cwd, opts) *job

1. Allocate the job, assign an ID, and store it.
2. Launch a goroutine that builds the process, sets its working directory, and waits for it to finish.
3. Capture the exit code, error, and duration, marking the job done.
4. Record a stats call with the job metadata and return the job.

#### Rationale

- Deprioritizing background jobs keeps heavy work from starving foreground shell commands, while setup jobs intentionally stay at default priority.
- Generating IDs under a write lock prevents two concurrent calls from claiming the same ID.
