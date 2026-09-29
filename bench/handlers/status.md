# job

The tool polls the state of one or more background jobs.

# Types

# Functions

## Handler.HandleStatus(_ context.Context, req) (*mcp.CallToolResult, error)

1. Require a job_ids array.
2. Report each job as multi when more than one was requested.
3. Format each job status and return the combined result.

## formatJobStatus(b, h, id, includeCommand)

1. Look up the job under a read lock, reporting a not-found error when it is missing.
2. Take the job lock and read its fields.
3. Emit the command when requested.
4. Emit the job ID, done state, exit code when done, duration, and error.
5. Emit stdout and stderr tags.

#### Rationale

- The read lock protects the jobs map; the job lock protects its buffers and state during the read.
