---
type: documentation
id: 2026-09-29-file-replace-all
summary: Documents the current behavior of the file_replace_all tool.
created: 2026-09-29
updated: 2026-09-29
---

# file_replace_all

## Description

`file_replace_all` replaces every occurrence of a single `find` in a file and returns a unified
diff. Unlike `file_replace`, it intentionally matches all occurrences rather than requiring a
unique match.

## Tool schema

```json
{
  "name": "file_replace_all",
  "description": "Replace all occurrences of a substring in a file. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": { "type": "string", "description": "Absolute path to the file." },
      "find": { "type": "string", "description": "Exact substring to find. All occurrences are replaced." },
      "replace": { "type": "string", "description": "Replacement text. Empty string deletes each match." },
      "start_line": { "type": "integer", "description": "Optional. Restrict replacements to this line range, inclusive (original-file line numbers)." },
      "end_line": { "type": "integer", "description": "Optional. Restrict replacements to this line range, inclusive (original-file line numbers)." },
      "dry_run": { "type": "boolean", "description": "Optional. If true, validate and compute the diff without writing to disk." }
    },
    "required": ["path", "find", "replace"]
  }
}
```

## Limits

| Constraint | Value | Rationale |
| --- | --- | --- |
| Max newlines in `replace` | 50 (`BENCH_MCP_EDIT_MAX_LINES`) | Keeps individual replacements surgical |
| Max candidates shown in error output | 5 (`BENCH_MCP_MAX_CANDIDATES`) | Keeps error messages readable |
| `file_replace_all` match count | >= 1 | Zero matches is an error |

## Scope containment

With `start_line`/`end_line`, a match is selected only if it is **fully contained** within the
range: `m.start_line >= start_line AND m.end_line <= end_line`. A multi-line match that crosses
either boundary is not replaced. This matches the least-surprising interpretation of
"restrict to this range."

## Error behavior

`file_replace_all` validates its single find/replace pair and either applies all matches or
writes nothing.

| Situation | Error content |
| --- | --- |
| 0 matches, no scope | "`find` not found in file" + first non-empty line diagnostic with 1-line context |
| 0 matches, with scope | "`find` not found between lines X-Y" + up to 10 lines of that range |

## Execution flow

1. **Input guards** (no lock needed). Reject empty `find`, identical `find`/`replace`, null
   bytes, invalid UTF-8, and `start_line`/`end_line` < 1 or `end_line < start_line`.
2. **Resolve symlinks** — lock and operate on the real path.
3. **Verify** the resolved path is a regular file.
4. **Acquire** an exclusive per-file lock.
5. **Read** the file; reject binary content; compute checksum and line count.
6. **Validate** `start_line`/`end_line` ranges against actual file length.
7. **Find** all matches within scope. A match is included only if fully contained in the range.
8. **Apply** in descending byte order. Candidates are non-overlapping by construction
   (`find_substring_matches` returns non-overlapping results), so no overlap check is needed.
9. **Dry-run exit** — return the diff without writing.
10. **External-modification check** — re-read and compare SHA-256.
11. **Atomic write** — temp file in the same directory, chmod to original mode, POSIX rename.
12. **Return** the unified diff.

## Constraints

- **Non-recursive**: replacement text is not re-searched even if it contains `find`. This
  matches `strings.ReplaceAll` semantics and is consistent with the pre-pass approach used by
  `file_replace`.
- **Substring matching is byte-exact**, including indentation and whitespace.
- **Line endings**: matching is byte-exact. The server assumes LF (`\n`); CRLF files fail to
  match `find` supplied with LF.
- **No shell involvement**: the entire operation is in-process Go. No `exec`, no escaping.

## Out of scope

- **Workspace boundary**: path confinement to an allowed root is enforced at the server level,
  not per-tool.
- **Durability guarantees**: a successful response means the rename completed; it does not
  guarantee durable persistence to stable storage.
