---
type: documentation
id: 2026-09-29-file-replace
summary: Documents the current behavior of the file_replace tool.
created: 2026-09-29
updated: 2026-09-29
---

# file_replace

## Description

`file_replace` finds and replaces unique substrings in a file and returns a unified diff.
The target file must already exist; creating missing files is out of scope (that is the job
of `file_create`).

## Tool schema

```json
{
  "name": "file_replace",
  "description": "Find and replace unique substrings in a file. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": { "type": "string", "description": "Absolute path to the file." },
      "replacements": {
        "type": "array",
        "description": "One or more find/replace pairs. Order does not matter.",
        "items": {
          "type": "object",
          "properties": {
            "find": { "type": "string", "description": "Unique substring to find, matched by character including whitespace." },
            "replace": { "type": "string", "description": "Replacement text. Subject to a configurable line limit. Empty string deletes the match." },
            "line_number": { "type": "integer", "description": "Optional. Narrows the match to occurrences spanning this line (original-file line number). Use when find alone is ambiguous across the file." }
          },
          "required": ["find", "replace"]
        }
      },
      "dry_run": { "type": "boolean", "description": "Optional. If true, validate and compute the diff without writing to disk." }
    },
    "required": ["path", "replacements"]
  }
}
```

## Limits

| Constraint | Value | Rationale |
| --- | --- | --- |
| Max newlines in `replace` | 50 (`BENCH_MCP_EDIT_MAX_LINES`) | Keeps individual replacements surgical |
| Max candidates shown in error output | 5 (`BENCH_MCP_MAX_CANDIDATES`) | Keeps error messages readable |
| `file_replace` match count | exactly 1 per item | Fails loudly on ambiguity |
| Chaining within a call | not supported | All `find` values are matched against the original file before any replacement is applied. To target text produced by a prior replacement, issue a second call. |

## Error behavior

`file_replace` is **fail-fast**. All items are validated in a single pre-pass against the
original file content before any edits are applied. If any item fails, nothing is written to
disk.

All errors that identify match locations include 1 line of file context before and after each
match. Diagnostic output is capped at 5 matches; when more exist the error notes
"showing first 5 of N."

| Matches | `line_number` | Error content |
| --- | --- | --- |
| 0 | omitted | Searches for first non-empty line of `find`; if found, reports line(s) with 1-line context; if not found, says so and points to whitespace/indentation or CRLF line endings |
| 0 | provided | "`find` not found at line N" + shows line N with 1-line context |
| >1 | omitted | Lists starting line of each match with 1-line context (capped at 5); suggests `line_number` or widening `find` |
| >1 | provided, spread | Lists starting line of each candidate with 1-line context (capped at 5); notes `line_number` N did not narrow to one |
| >1 | provided, same line | Char positions of each match + line content; suggests replacing the whole line |

## Execution flow

1. **Input guards** (no lock needed). Reject empty `find`, identical `find`/`replace`, null
   bytes, invalid UTF-8, `line_number` < 1, and `replace` exceeding the newline limit.
2. **Create mode** (hidden): if there is exactly one replacement and the file is missing or
   empty, `file_replace` falls back to creating the file. Otherwise it proceeds to in-place
   edit mode.
3. **Resolve symlinks** — lock and operate on the real path.
4. **Verify** the resolved path is a regular file.
5. **Acquire** an exclusive per-file lock.
6. **Read** the file; reject binary content; compute checksum and line count.
7. **Validate** `line_number` ranges against actual file length.
8. **Pre-pass** — locate each replacement's unique candidate in the original content. All
   searches run on the original content. Chaining is not supported within a call.
9. **Sort** by start byte ascending; reject overlapping candidates.
10. **Apply** in descending byte order (later-in-file edits first so earlier byte offsets stay
    valid).
11. **Dry-run exit** — return the diff without writing.
12. **External-modification check** — re-read and compare SHA-256.
13. **Atomic write** — temp file in the same directory, chmod to original mode, POSIX rename.
14. **Return** the unified diff.

## Constraints

- **Substring matching is byte-exact**, including indentation and whitespace. Auto-formatter
  runs, import reordering, or generated-comment additions can invalidate a `find` block that
  was valid moments before.
- **Line endings**: matching is byte-exact. The server assumes LF (`\n`). Files with CRLF line
  endings fail to match `find` supplied with LF — the not-found error surfaces this hint
  explicitly.
- **No shell involvement**: the entire operation is in-process. No `exec`, no escaping.

## Why not shell?

The file-write workflow shelling out Python/bash and redirecting text into a file has two
problems: shell escaping (quotes, backslashes, special characters) and verification overhead
(the model must follow up with `cat`/`grep` to confirm the write). `file_replace` eliminates
both — writes happen in-process, and the returned diff is immediate proof of what changed.

## Out of scope

- **Workspace boundary**: path confinement to an allowed root is enforced at the server level,
  not per-tool.
- **Indentation-aware matching, line-based patches, or AST-aware edits**: byte-exact matching is
  the intended behavior for a precise surgical tool.
- **Durability guarantees**: a successful response means the rename completed; it does not
  guarantee durable persistence to stable storage (e.g. after a crash on a network-mounted
  filesystem).
