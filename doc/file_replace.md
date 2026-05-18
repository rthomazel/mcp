# file_replace and file_replace_all

Find-and-replace tools for precise file editing, built on the same mental model as editor tooling.

- **`file_replace`** — replaces each `find` exactly once per item (unique match required)
- **`file_replace_all`** — replaces every occurrence of each `find`

Both accept a batch of replacements applied sequentially in memory with a single atomic write.

## Tool schemas

### file_replace

```json
{
  "name": "file_replace",
  "description": "Find and replace exact substrings in a file. Each item must match exactly once. All replacements applied in one atomic write. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Absolute path to the file."
      },
      "replacements": {
        "type": "array",
        "description": "One or more find/replace pairs applied in order.",
        "items": {
          "type": "object",
          "properties": {
            "find": {
              "type": "string",
              "description": "Exact substring to find, matched character-for-character including whitespace. Must occur exactly once (globally, or at line_number if given)."
            },
            "replace": {
              "type": "string",
              "description": "Replacement text. Subject to a configurable line limit. Empty string deletes the match."
            },
            "line_number": {
              "type": "integer",
              "description": "Optional. Narrows the match to occurrences spanning this line. Use when find alone is ambiguous across the file."
            }
          },
          "required": ["find", "replace"]
        }
      },
      "dry_run": {
        "type": "boolean",
        "description": "Optional. If true, validate and compute the diff without writing to disk. Returns the same unified diff as a real run."
      }
    },
    "required": ["path", "replacements"]
  }
}
```

### file_replace_all

```json
{
  "name": "file_replace_all",
  "description": "Find and replace all occurrences of substrings in a file. All replacements applied in one atomic write. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Absolute path to the file."
      },
      "replacements": {
        "type": "array",
        "description": "One or more find/replace pairs applied in order.",
        "items": {
          "type": "object",
          "properties": {
            "find": {
              "type": "string",
              "description": "Exact substring to find. All occurrences are replaced."
            },
            "replace": {
              "type": "string",
              "description": "Replacement text. Empty string deletes each match."
            },
            "start_line": {
              "type": "integer",
              "description": "Optional. Restrict replacements to this line range (inclusive)."
            },
            "end_line": {
              "type": "integer",
              "description": "Optional. Restrict replacements to this line range (inclusive)."
            }
          },
          "required": ["find", "replace"]
        }
      },
      "dry_run": {
        "type": "boolean",
        "description": "Optional. If true, validate and compute the diff without writing to disk. Returns the same unified diff as a real run."
      }
    },
    "required": ["path", "replacements"]
  }
}
```

## Limits

| Constraint | Value | Rationale |
| --- | --- | --- |
| `replace` max lines per item | **50** (env: `JAIL_MCP_EDIT_MAX_LINES`) | Keeps individual replacements surgical |
| `file_replace` match count | exactly 1 per item | Fails loudly on ambiguity |
| `file_replace_all` match count | ≥ 1 per item | Zero matches is an error |

## Error behavior

Both tools are **fail-fast**: if any item fails, nothing is written to disk. The error identifies which item failed and how many had been applied in memory:

> "Replacement 2 of 4 failed: [reason]. (1 applied in memory, nothing written.)"

### file_replace error matrix

All errors that identify match locations include 1 line of file context before and after each match.

| Matches | `line_number` | Error content |
| --- | --- | --- |
| 0 | omitted | Searches for first line of `find`; if found, reports line(s) with 1-line context; if not found, says so and points to whitespace/indentation or CRLF line endings |
| 0 | provided | "`find` not found at line N" + shows line N with 1-line context |
| >1 | omitted | Lists starting line of each match with 1-line context; suggests `line_number` or widening `find` |
| >1 | provided, spread | Lists starting line of each candidate with 1-line context; notes `line_number` N did not narrow to one |
| >1 | provided, same line | Char positions of each match + line content; suggests replacing the whole line |

### file_replace_all error cases

| Situation | Error content |
| --- | --- |
| 0 matches, no scope | "`find` not found in file" + first-line diagnostic with 1-line context |
| 0 matches, with scope | "`find` not found between lines X–Y" + shows that range |

## Execution flow

Both tools share the same outer structure; the inner match step differs.

```python
def handle_file_replace(path, replacements, dry_run=False):
    # 1. Input guards (no lock needed — pure validation)
    if path == "":
        return Error("path must not be empty.")
    if len(replacements) == 0:
        return Error("replacements must not be empty.")
    for i, r in enumerate(replacements):
        label = f"Replacement {i+1}"
        if r.find == "":
            return Error(f"{label}: find must not be empty.")
        if r.find == r.replace:
            return Error(f"{label}: find and replace are identical — no change would be made.")
        if "\x00" in r.find or "\x00" in r.replace:
            return Error(f"{label}: null bytes detected; binary files are not supported.")
        if not is_valid_utf8(r.find) or not is_valid_utf8(r.replace):
            return Error(f"{label}: find and replace must be valid UTF-8.")
        if r.line_number is not None and r.line_number < 1:
            return Error(f"{label}: line_number must be ≥ 1.")

    # 2. Resolve symlinks — lock and operate on the real path
    path = resolve_symlinks(path)

    # 3. Acquire exclusive per-file lock (refcounted; entry removed when refcount reaches 0)
    lock = acquire_file_lock(path)

    # 4. Read file and snapshot checksum; reject binary content
    file_content = read_file(path)
    if contains_null_bytes(file_content):
        release(lock)
        return Error("Binary files are not supported.")
    checksum = sha256(file_content)
    total_lines = count_lines(file_content)

    # 5. Validate line_number ranges against actual file length
    for i, r in enumerate(replacements):
        if r.line_number is not None and r.line_number > total_lines:
            release(lock)
            return Error(f"Replacement {i+1}: line_number {r.line_number} out of range (file has {total_lines} lines).")

    # 6. Apply replacements sequentially in memory — fail-fast, nothing written on any error
    # line_number values are relative to the original file; line_offset tracks cumulative line
    # shift as prior replacements add or remove lines.
    working = file_content
    line_offset = 0
    for i, r in enumerate(replacements):
        label = f"Replacement {i+1} of {len(replacements)}"
        progress = f"({i} applied in memory, nothing written.)"

        if count_lines(r.replace) > MAX_LINES:
            release(lock)
            return Error(f"{label}: replace is {count_lines(r.replace)} lines. Max is {MAX_LINES}. {progress}")

        all_matches = find_substring_matches(working, r.find)
        candidates = (
            [m for m in all_matches if m.start_line <= r.line_number + line_offset <= m.end_line]
            if r.line_number is not None else all_matches
        )

        if len(candidates) == 0:
            release(lock)
            if r.line_number is not None:
                ctx = excerpt(working, r.line_number + line_offset, radius=1)
                return Error(f"{label} failed: find not found at line {r.line_number}.\n{ctx}\n{progress}")
            first_line = first_line_of(r.find)
            partial = find_substring_matches(working, first_line)
            if partial:
                snippets = [excerpt(working, m.start_line, radius=1) for m in partial]
                locs = [m.start_line for m in partial]
                return Error(
                    f"{label} failed: first line of find matched at {locs} but full find did not match"
                    f" (check indentation or whitespace).\n{join(snippets)}\n{progress}"
                )
            return Error(f"{label} failed: find not found in file (check whitespace or CRLF endings). {progress}")

        if len(candidates) > 1:
            release(lock)
            if r.line_number is not None:
                same_line = all(m.start_line == candidates[0].start_line for m in candidates)
                if same_line:
                    char_positions = [m.start_char for m in candidates]
                    ctx = excerpt(working, r.line_number + line_offset, radius=1)
                    return Error(
                        f"{label} failed: ambiguous at line {r.line_number}: find matched"
                        f" {len(candidates)} times at characters {char_positions}. Replace the whole line.\n{ctx}\n{progress}"
                    )
                locs = [m.start_line for m in candidates]
                snippets = [excerpt(working, m.start_line, radius=1) for m in candidates]
                return Error(
                    f"{label} failed: line_number {r.line_number} did not narrow to one match"
                    f" (at lines {locs}).\n{join(snippets)}\n{progress}"
                )
            locs = [m.start_line for m in candidates]
            snippets = [excerpt(working, m.start_line, radius=1) for m in candidates]
            return Error(
                f"{label} failed: find matched {len(candidates)} locations (lines {locs})."
                f" Provide line_number or widen find.\n{join(snippets)}\n{progress}"
            )

        working = replace_at(working, candidates[0], r.replace)
        line_offset += count_lines(r.replace) - count_lines(r.find)

    # 7. Dry-run exit — return diff without writing
    if dry_run:
        release(lock)
        return compute_myers_diff(path, file_content, working)

    # 8. External-modification check before committing
    if sha256(read_file(path)) != checksum:
        release(lock)
        return Error("Edit aborted: file was modified externally between read and write.")

    # 9. Atomic write — preserve original file permissions
    original_mode = stat(path).mode
    tmp_path = path + ".tmp." + current_time_ns()
    write_to_disk(tmp_path, working, mode=original_mode)
    rename(tmp_path, path)  # POSIX-atomic
    release(lock)

    # 10. Return unified diff
    return compute_myers_diff(path, file_content, working)
```

`handle_file_replace_all` is identical to `handle_file_replace` except:

**1.** The step 1 guard loop additionally validates `start_line` and `end_line`:

```python
        if r.start_line is not None and r.start_line < 1:
            return Error(f"{label}: start_line must be ≥ 1.")
        if r.end_line is not None and r.end_line < 1:
            return Error(f"{label}: end_line must be ≥ 1.")
        if r.start_line is not None and r.end_line is not None and r.end_line < r.start_line:
            return Error(f"{label}: end_line must be ≥ start_line.")
```

**2.** Step 5 additionally checks `end_line` against the file length:

```python
        if r.end_line is not None and r.end_line > total_lines:
            release(lock)
            return Error(f"Replacement {i+1}: end_line {r.end_line} out of range (file has {total_lines} lines).")
```

**3.** The inner match/replace block in step 6:

```python
        all_matches = find_substring_matches(working, r.find)
        # start_line/end_line are relative to the original file; adjust to working's coordinate space
        adjusted_start = (r.start_line + line_offset) if r.start_line is not None else None
        adjusted_end = (r.end_line + line_offset) if r.end_line is not None else None
        candidates = [
            m for m in all_matches
            if (adjusted_start is None or m.start_line >= adjusted_start) and
               (adjusted_end is None or m.end_line <= adjusted_end)
        ]

        if len(candidates) == 0:
            release(lock)
            if r.start_line is not None or r.end_line is not None:
                scope_start = adjusted_start if adjusted_start is not None else 1
                scope_end = adjusted_end if adjusted_end is not None else total_lines + line_offset
                ctx = excerpt(working, scope_start, end_line=scope_end)
                return Error(
                    f"{label} failed: find not found between lines {r.start_line or 1}–{r.end_line or total_lines}.\n{ctx}\n{progress}"
                )
            first_line = first_line_of(r.find)
            partial = find_substring_matches(working, first_line)
            if partial:
                snippets = [excerpt(working, m.start_line, radius=1) for m in partial]
                locs = [m.start_line for m in partial]
                return Error(
                    f"{label} failed: first line of find matched at {locs} but full find did not match"
                    f" (check indentation or whitespace).\n{join(snippets)}\n{progress}"
                )
            return Error(f"{label} failed: find not found in file (check whitespace or CRLF endings). {progress}")

        # Replace all candidates (descending position order to preserve offsets)
        working = replace_all_occurrences(working, r.find, r.replace, scope=(adjusted_start, adjusted_end))
        line_offset += len(candidates) * (count_lines(r.replace) - count_lines(r.find))
```

## Implementation notes

- **Diff algorithm:** Myers diff, returned as a standard unified diff string.
- **Atomic write:** the temp file uses a unique name (`<filename>.tmp.<nanosecond timestamp>`) to avoid colliding with an existing file or stale temp. After writing, `os.Rename` makes the replacement atomic on POSIX (same-directory placement guarantees both paths are on the same filesystem). The temp file is created with the original file's mode (`os.Stat` before write, `os.Chmod` before rename) to preserve execute bits and other permissions. On failure after the temp file is written, the temp file is removed.
- **Per-file locking:** a `sync.Map` of `{sync.Mutex, refcount}` keyed by absolute path. Refcount is incremented on acquire and decremented on release; the entry is deleted from the map when it reaches zero. Prevents unbounded map growth while avoiding the race where a waiting goroutine holds a reference to a mutex that was already removed.
- **External-modification detection:** SHA-256 of the file content is computed immediately after the read. Before writing, the file is re-read and its hash compared. A mismatch means an external process modified the file during the edit — the operation fails rather than silently overwriting unrelated changes.
- **Line endings:** matching is byte-exact. The server assumes LF (`\n`). Files with CRLF line endings will fail to match `find` supplied with LF — the not-found error surfaces this hint explicitly.
- **Substring matching:** `find_substring_matches` returns non-overlapping matches in left-to-right order, consistent with Go's `strings.Index`/`strings.Count` behavior. Example: `"aa"` in `"aaa"` yields one match at offset 0, not two overlapping matches.
- **replace_at** performs a positional splice using the matched candidate's byte offsets: `s[:m.start_byte] + replace + s[m.end_byte:]`. This guarantees the correct occurrence is replaced even when `find` appears multiple times and `line_number` was used to narrow to one candidate.
- **Symlink handling:** the path is resolved via `filepath.EvalSymlinks` before locking. The lock key and write target are the real path. This ensures two calls through different symlinks to the same file serialize correctly, and that `rename` operates on the real file rather than replacing the symlink itself.
- **No shell involvement:** the entire operation is in-process Go. No `exec`, no escaping.

## Why not exec_sync?

The existing file-write workflow shells out Python/bash and redirects text into a file, creating two problems:

1. **Shell escaping:** quotes, backslashes, and special characters require careful escaping that is error-prone and model-unfriendly.
2. **Verification overhead:** the model must follow up with `cat` or `grep` to confirm the write applied correctly, adding a round-trip.

`file_replace` and `file_replace_all` eliminate both: writes happen in-process (no shell), and the returned diff is immediate proof of what changed.

## Known limitations

**Substring matching is structurally fragile**
Both tools match exact substrings — byte-for-byte, including indentation and whitespace. Small formatting changes (reordered imports, auto-formatter runs, generated comment additions) can invalidate a `find` block that was valid moments before. This is an accepted V1 tradeoff. Future extensions worth considering: indentation-aware matching, line-based patches, or AST-aware edits for structured languages.

**Durability guarantees**
A successful response means the rename completed — the replacement is visible to subsequent reads on the same filesystem. It does not guarantee durable persistence to stable storage (e.g. after a crash on a network-mounted filesystem). For the target use case (local or container-mounted filesystems in agentic loops) this is sufficient.
