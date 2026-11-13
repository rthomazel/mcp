---
id: 2026-11-13-context
type: documentation
summary: Documents the context tool that reports the container environment.
author: Thom
created: 2026-11-13
agents: claude
---

# context

## Description

`context` returns a plain-text summary of the environment so an agent can orient itself at session start. It takes no parameters.

## Output

A single `metadata` block with:

- `os`, `arch`, `disk` (free of total for `/`), `path` (current `PATH`), `shell_exec_timeout`, `version`.
- `volumes`: one line per relevant mount, marked `persistent`, `ro`, or `rw`. Mounts at the mise directory and the home directory are persistent.
  The root mount, pseudo and overlay filesystems (proc, sysfs, tmpfs, devpts, cgroup, mqueue, overlay), and mounts under `/proc`, `/sys`, `/dev`, `/run`, and `/etc` are omitted.
- A note that the container is ephemeral and that installs under `$HOME/bin` persist.
- `installed`: versions of bash, git, jujutsu, mise, python3, pip3, rg, make, jq, and curl; `-` when a tool is missing or reports nothing.
- `auto-detected in path`: executables found on `PATH` that were absent from the startup snapshot, when there are any.

## Behavior

- OS, architecture, disk, and tool versions are gathered by running shell commands under the normal shell timeout.
- Mount parsing failures are logged and the volumes list is left empty or partial; the call still succeeds.
- Each call rescans `PATH` and diffs against the snapshot written at startup; see [config](../config/doc.md) for the snapshot.

## Refs

- doc: 2026-09-29-config
- doc: 2026-09-29-architecture
