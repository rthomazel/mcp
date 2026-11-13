---
id: 2026-11-13-shell
type: documentation
summary: Documents the current behavior of the shell tool, including command parsing, && splitting, and output format.
author: Thom
created: 2026-11-13
agents: claude
---

# shell

## Description

`shell` runs one or more bash commands synchronously and returns stdout, stderr, exit code, and duration for each.
Each command runs as `bash -c` with a timeout of `BENCH_MCP_TIMEOUT` (default 15s). No command filtering is applied;
the container is the security boundary.

## Parameters

| parameter | required | meaning |
| --- | --- | --- |
| `commands` | yes* | non-empty array of command strings |
| `cwd` | no | working directory for all commands; defaults to `/`; does not persist across calls |

*`BENCH_MCP_SHELL_COMMANDS_ALIASES` may name alternate parameters for `commands`. Exactly one of the canonical name
or an alias must be a non-empty array of strings. Supplying more than one, a non-array, or none is an error.

## Execution

Commands run sequentially in order. Execution stops at the first command that cannot start (a tool error is returned);
a non-zero exit does not stop later commands.

## Command expansion

Controlled by `BENCH_MCP_SHELL_EXPAND_COMMANDS` (default true). When enabled:

1. **cd prefix.** When `cwd` was not supplied, a command starting with `cd PATH && ` has the path used as its working directory and
   the rest as the command. The path must be non-empty and contain no quotes or whitespace; otherwise the command is left alone.
2. **&& splitting.** Unquoted ` && ` chains become independent commands with separate results. A later command runs even when an earlier one
   fails; there is no short-circuit. Splitting honors single and double quotes, backticks, `$(...)` subshells, and heredocs
   (whole heredoc, including body, is opaque; an unterminated heredoc consumes the rest of the command).

Each expanded command carries a hint in its metadata explaining the cd parse and/or the split. When disabled, commands run exactly as given.

## Output

Plain text with XML-style tags. A single result has a `metadata` block (`exit`, `duration`, optional `hint`), then `stdout` and `stderr`
blocks. When more than one command ran, each result is wrapped in a `command` tag with its index and the metadata also lists the command.
Trailing newlines are trimmed from stdout and stderr.

## Errors

A process that cannot start returns a tool error ("could not start process") and the call ends. A timeout is not a tool error; the command's
result is returned with the exit code of the killed process (typically -1) and a duration near the timeout. Each executed command is recorded in the stats history.

## Refs

- doc: 2026-11-13-shell-background
- doc: 2026-09-29-config
