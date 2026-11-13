---
id: 2026-11-13-status
type: documentation
summary: Documents the status tool that reports background job state.
author: Thom
created: 2026-11-13
agents: claude
---

# status

## Description

`status` reports the state of background jobs started by `shell_background` or `setup`.

## Parameters

`job_ids`: required, non-empty array of strings. A missing, empty, or non-string-array value is an error.

## Output

One block per job, each with a `metadata` section, then `stdout` and `stderr` blocks holding output captured so far.

Metadata fields: `job_id`, `done` (true or false), `exit` (only once done), `duration` (elapsed since start, still counting while running),
and `error` when present. When more than one ID was requested, each job is wrapped in a `command` tag with its index and the metadata
also lists the job's command.

An unknown ID, including a job already removed after the one-hour retention, yields metadata with `error: job not found`
and does not fail the whole call.

## Concurrency

Reading a job takes the job lock, so a snapshot is consistent even while the job is writing output.

## Refs

- doc: 2026-11-13-shell-background
