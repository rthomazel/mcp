---
id: 2026-11-13-shell-background
type: documentation
summary: Documents the shell_background tool and the shared background job lifecycle.
author: Thom
created: 2026-11-13
agents: claude
---

# shell_background

## Description

`shell_background` starts one or more bash commands as background jobs and returns a job ID per command immediately.
Results are read with [status](../status/doc.md). The same job machinery runs `setup`.

## Parameters

Same `commands` parameter and aliases as [shell](../shell/doc.md). `cwd` is optional and defaults to `/`.
Commands are not expanded: no cd-prefix parsing and no && splitting, so a chain runs as one job under normal bash semantics.

## Response

A `metadata` block. A single command yields `job_id: NNNN`. Several yield `command_N` and `job_id_N` lines per command, in order.

## Job lifecycle

- A job gets a random 4-digit ID, unique among currently stored jobs.
- It runs as `bash -c` in the given directory with a timeout of `BENCH_MCP_BACKGROUND_TIMEOUT` (default 5m).
- Output is buffered in memory and readable while the job runs.
- Jobs started by `shell_background` run under `nice -n <BENCH_MCP_BACKGROUND_NICE>` (default 10) so they do not starve foreground
  commands; a nice level of 0 disables the wrapper. This does not bound memory or the combined CPU of several jobs.
- A job that cannot start is marked done with exit `-1` and an error.
- A timed-out job is marked done with the exit code of the killed process (typically -1) and is recorded as a timeout in stats.
- Finished jobs are removed from memory once older than one hour, checked every five minutes. Jobs are lost on server restart.

## Refs

- doc: 2026-11-13-status
- doc: 2026-11-13-shell
- doc: 2026-09-29-setup
