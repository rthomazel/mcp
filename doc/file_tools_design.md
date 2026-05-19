# File Tools Design

Specification and implementation guide for `file_replace` and `file_replace_all`.

- **`file_replace`** — replaces each `find` exactly once per item (unique match required); accepts a batch
- **`file_replace_all`** — replaces every occurrence of a single `find`

## Tool schemas

### file_replace

```json
{
  "name": "file_replace",
  "description": "Find and replace unique substrings in a file. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Absolute path to the file."
      },
      "replacements": {
        "type": "array",
        "description": "One or more find/replace pairs. Order does not matter.",
        "items": {
          "type": "object",
          "properties": {
            "find": {
              "type": "string",
              "description": "Unique substring to find, matched by character including whitespace."
            },
            "replace": {
              "type": "string",
              "description": "Replacement text. Subject to a configurable line limit. Empty string deletes the match."
            },
            "line_number": {
              "type": "integer",
              "description": "Optional. Narrows the match to occurrences spanning this line (original-file line number). Use when find alone is ambiguous across the file."
            }
          },
          "required": ["find", "replace"]
        }
      },
      "dry_run": {
        "type": "boolean",
        "description": "Optional. If true, validate and compute the diff without writing to disk."
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
  "description": "Replace all occurrences of a substring in a file. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Absolute path to the file."
      },
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
        "description": "Optional. Restrict replacements to this line range, inclusive (original-file line numbers)."
      },
      "end_line": {
        "type": "integer",
        "description": "Optional. Restrict replacements to this line range, inclusive (original-file line numbers)."
      },
      "dry_run": {
        "type": "boolean",
        "description": "Optional. If true, validate and compute the diff without writing to disk."
      }
    },
    "required": ["path", "find", "replace"]
  }
}
```

## Limits

| Constraint | Value | Rationale |
| --- | --- | --- |
| Max newlines in `replace` | **50** (env: `JAIL_MCP_EDIT_MAX_LINES`) | Keeps individual replacements surgical |
| Max candidates in error output | **5** (env: `JAIL_MCP_MAX_CANDIDATES`) | Keeps error messages readable |
| `file_replace` match count | exactly 1 per item | Fails loudly on ambiguity |
| `file_replace_all` match count | ≥ 1 | Zero matches is an error |
| Chaining within a `file_replace` call | not supported | All `find` values are matched against the original file content before any replacement is applied. To target text produced by a prior replacement, issue a second call. |

## Error behavior

`file_replace` is **fail-fast**: all items are validated in a single pre-pass against the original file content before any edits are applied. If any item fails, nothing is written to disk.

`file_replace_all` validates its single find/replace pair and either applies all matches or writes nothing.

### file_replace error matrix

All errors that identify match locations include 1 line of file context before and after each match. Diagnostic output is capped at 5 matches; when more exist the error notes “showing first 5 of N.”

| Matches | `line_number` | Error content |
| --- | --- | --- |
| 0 | omitted | Searches for first non-empty line of `find`; if found, reports line(s) with 1-line context; if not found, says so and points to whitespace/indentation or CRLF line endings |
| 0 | provided | "`find` not found at line N" + shows line N with 1-line context |
| >1 | omitted | Lists starting line of each match with 1-line context (capped at 5); suggests `line_number` or widening `find` |
| >1 | provided, spread | Lists starting line of each candidate with 1-line context (capped at 5); notes `line_number` N did not narrow to one |
| >1 | provided, same line | Char positions of each match + line content; suggests replacing the whole line |

### file_replace_all error cases

| Situation | Error content |
| --- | --- |
| 0 matches, no scope | "`find` not found in file" + first non-empty line diagnostic with 1-line context |
| 0 matches, with scope | "`find` not found between lines X–Y" + up to 10 lines of that range |

## Execution flow

### handle_file_replace

```python
MAX_CANDIDATES = env("JAIL_MCP_MAX_CANDIDATES", default=5)  # max candidates shown in diagnostic output; shared by both handlers


def handle_file_replace(path, replacements, dry_run=False):
    # 1. Input guards (no lock needed — pure validation)
    if not is_absolute(path):
        return Error("path must be absolute.")
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
            return Error(f"{label}: line_number must be \u2265 1.")
        if count_newlines(r.replace) > MAX_LINES:
            return Error(f"{label}: replace exceeds the {MAX_LINES}-newline limit.")

    # 2. Resolve symlinks — lock and operate on the real path
    path = resolve_symlinks(path)

    # 3. Verify resolved path is a regular file
    if not is_regular_file(path):
        return Error(f"path must point to a regular file.")

    # 4. Acquire exclusive per-file lock
    lock = acquire_file_lock(path)

    # 5. Read file; reject binary content
    file_content = read_file(path)
    if contains_null_bytes(file_content) or not is_valid_utf8(file_content):
        release(lock)
        return Error("Binary files are not supported.")
    checksum = sha256(file_content)
    total_lines = count_lines(file_content)

    # 6. Validate line_number ranges against actual file length
    for i, r in enumerate(replacements):
        if r.line_number is not None and r.line_number > total_lines:
            release(lock)
            return Error(f"Replacement {i+1}: line_number {r.line_number} out of range (file has {total_lines} lines).")

    # 7. Pre-pass: locate each replacement's unique candidate in the original content.
    # All searches run on file_content. Chaining is not supported within a call;
    # use a second call to target text produced by a prior replacement.
    located = []  # list of (original_index, replacement, candidate_match)
    for i, r in enumerate(replacements):
        label = f"Replacement {i+1} of {len(replacements)}"

        all_matches = find_substring_matches(file_content, r.find)
        candidates = (
            [m for m in all_matches if m.start_line <= r.line_number <= m.end_line]
            if r.line_number is not None else all_matches
        )

        if len(candidates) == 0:
            release(lock)
            if r.line_number is not None:
                ctx = excerpt(file_content, r.line_number, radius=1)
                return Error(f"{label} failed: find not found at line {r.line_number}.\n{ctx}")
            first_line = first_nonempty_line_of(r.find)
            if first_line is not None:
                partial = find_substring_matches(file_content, first_line)
                if partial:
                    shown = partial[:MAX_CANDIDATES]
                    snippets = [excerpt(file_content, m.start_line, radius=1) for m in shown]
                    locs = [m.start_line for m in shown]
                    suffix = f" (showing first {MAX_CANDIDATES} of {len(partial)})" if len(partial) > MAX_CANDIDATES else ""
                    return Error(
                        f"{label} failed: first line of find matched at {locs}{suffix} but full find did not match"
                        f" (check indentation or whitespace).\n{join(snippets)}"
                    )
            return Error(f"{label} failed: find not found in file (check whitespace or CRLF endings).")

        if len(candidates) > 1:
            release(lock)
            if r.line_number is not None:
                same_line = all(m.start_line == candidates[0].start_line for m in candidates)
                if same_line:
                    char_positions = [m.start_char for m in candidates]
                    ctx = excerpt(file_content, r.line_number, radius=1)
                    return Error(
                        f"{label} failed: ambiguous at line {r.line_number}: find matched"
                        f" {len(candidates)} times at characters {char_positions}. Replace the whole line.\n{ctx}"
                    )
                shown = candidates[:MAX_CANDIDATES]
                locs = [m.start_line for m in shown]
                snippets = [excerpt(file_content, m.start_line, radius=1) for m in shown]
                suffix = f" (showing first {MAX_CANDIDATES} of {len(candidates)})" if len(candidates) > MAX_CANDIDATES else ""
                return Error(
                    f"{label} failed: line_number {r.line_number} did not narrow to one match"
                    f" (at lines {locs}{suffix}).\n{join(snippets)}"
                )
            shown = candidates[:MAX_CANDIDATES]
            locs = [m.start_line for m in shown]
            snippets = [excerpt(file_content, m.start_line, radius=1) for m in shown]
            suffix = f" (showing first {MAX_CANDIDATES} of {len(candidates)})" if len(candidates) > MAX_CANDIDATES else ""
            return Error(
                f"{label} failed: find matched {len(candidates)} locations (lines {locs}{suffix})."
                f" Provide line_number or widen find.\n{join(snippets)}"
            )

        located.append((i, r, candidates[0]))

    # 8. Sort by start_byte ascending; reject overlapping candidates
    located.sort(key=lambda x: x[2].start_byte)
    for j in range(1, len(located)):
        prev_idx, _, prev_m = located[j-1]
        curr_idx, _, curr_m = located[j]
        if prev_m.end_byte > curr_m.start_byte:
            release(lock)
            return Error(
                f"Replacements {prev_idx+1} and {curr_idx+1} target overlapping regions "
                f"in the original file."
            )

    # 9. Apply in descending byte order — later-in-file edits first so that
    # byte offsets of earlier edits remain valid throughout.
    working = file_content
    for _, r, m in reversed(located):
        working = working[:m.start_byte] + r.replace + working[m.end_byte:]

    # 10. Dry-run exit — return diff without writing
    if dry_run:
        release(lock)
        return compute_myers_diff(path, file_content, working)

    # 11. External-modification check before committing
    if sha256(read_file(path)) != checksum:
        release(lock)
        return Error("Edit aborted: file was modified externally between read and write.")

    # 12. Atomic write — preserve original file permissions
    original_mode = stat(path).mode
    tmp = os_create_temp(dir=dirname(path))  # O_CREATE|O_EXCL|0600; unique name guaranteed
    if err := write_to_disk(tmp, working); err != nil:
        remove(tmp)
        release(lock)
        return Error(f"Write failed: {err}")
    if err := chmod(tmp, original_mode); err != nil:
        remove(tmp)
        release(lock)
        return Error(f"Chmod failed: {err}")
    if err := rename(tmp, path); err != nil:  # POSIX-atomic
        remove(tmp)
        release(lock)
        return Error(f"Rename failed: {err}")
    release(lock)

    # 13. Return unified diff
    return compute_myers_diff(path, file_content, working)
```

### handle_file_replace_all

```python
def handle_file_replace_all(path, find, replace, start_line=None, end_line=None, dry_run=False):
    # 1. Input guards (no lock needed — pure validation)
    if not is_absolute(path):
        return Error("path must be absolute.")
    if find == "":
        return Error("find must not be empty.")
    if find == replace:
        return Error("find and replace are identical — no change would be made.")
    if "\x00" in find or "\x00" in replace:
        return Error("null bytes detected; binary files are not supported.")
    if not is_valid_utf8(find) or not is_valid_utf8(replace):
        return Error("find and replace must be valid UTF-8.")
    if count_newlines(replace) > MAX_LINES:
        return Error(f"replace exceeds the {MAX_LINES}-newline limit.")
    if start_line is not None and start_line < 1:
        return Error("start_line must be \u2265 1.")
    if end_line is not None and end_line < 1:
        return Error("end_line must be \u2265 1.")
    if start_line is not None and end_line is not None and end_line < start_line:
        return Error("end_line must be \u2265 start_line.")

    # 2. Resolve symlinks — lock and operate on the real path
    path = resolve_symlinks(path)

    # 3. Verify resolved path is a regular file
    if not is_regular_file(path):
        return Error(f"path must point to a regular file.")

    # 4. Acquire exclusive per-file lock
    lock = acquire_file_lock(path)

    # 5. Read file; reject binary content
    file_content = read_file(path)
    if contains_null_bytes(file_content) or not is_valid_utf8(file_content):
        release(lock)
        return Error("Binary files are not supported.")
    checksum = sha256(file_content)
    total_lines = count_lines(file_content)

    # 6. Validate scope against file length
    if start_line is not None and start_line > total_lines:
        release(lock)
        return Error(f"start_line {start_line} out of range (file has {total_lines} lines).")
    if end_line is not None and end_line > total_lines:
        release(lock)
        return Error(f"end_line {end_line} out of range (file has {total_lines} lines).")

    # 7. Find all matches within scope.
    # A match is included only if it is fully contained within the scope:
    # m.start_line >= start_line AND m.end_line <= end_line.
    # Multi-line matches that cross either boundary are not replaced.
    all_matches = find_substring_matches(file_content, find)
    candidates = [
        m for m in all_matches
        if (start_line is None or m.start_line >= start_line) and
           (end_line is None or m.end_line <= end_line)
    ]

    if len(candidates) == 0:
        release(lock)
        if start_line is not None or end_line is not None:
            scope_start = start_line or 1
            scope_end = end_line or total_lines
            ctx = excerpt(file_content, scope_start, end_line=scope_end, max_lines=10)
            return Error(f"find not found between lines {scope_start}\u2013{scope_end}.\n{ctx}")
        first_line = first_nonempty_line_of(find)
        if first_line is not None:
            partial = find_substring_matches(file_content, first_line)
            if partial:
                shown = partial[:MAX_CANDIDATES]
                snippets = [excerpt(file_content, m.start_line, radius=1) for m in shown]
                locs = [m.start_line for m in shown]
                suffix = f" (showing first {MAX_CANDIDATES} of {len(partial)})" if len(partial) > MAX_CANDIDATES else ""
                return Error(
                    f"first line of find matched at {locs}{suffix} but full find did not match"
                    f" (check indentation or whitespace).\n{join(snippets)}"
                )
        return Error("find not found in file (check whitespace or CRLF endings).")

    # 8. Apply in descending byte order — candidates are non-overlapping by construction
    # (find_substring_matches returns non-overlapping results) so no overlap check is needed.
    # Replacement text is not re-searched; this matches strings.ReplaceAll semantics.
    working = file_content
    for m in reversed(candidates):
        working = working[:m.start_byte] + replace + working[m.end_byte:]

    # 9. Dry-run exit — return diff without writing
    if dry_run:
        release(lock)
        return compute_myers_diff(path, file_content, working)

    # 10. External-modification check before committing
    if sha256(read_file(path)) != checksum:
        release(lock)
        return Error("Edit aborted: file was modified externally between read and write.")

    # 11. Atomic write — preserve original file permissions
    original_mode = stat(path).mode
    tmp = os_create_temp(dir=dirname(path))  # O_CREATE|O_EXCL|0600; unique name guaranteed
    if err := write_to_disk(tmp, working); err != nil:
        remove(tmp)
        release(lock)
        return Error(f"Write failed: {err}")
    if err := chmod(tmp, original_mode); err != nil:
        remove(tmp)
        release(lock)
        return Error(f"Chmod failed: {err}")
    if err := rename(tmp, path); err != nil:  # POSIX-atomic
        remove(tmp)
        release(lock)
        return Error(f"Rename failed: {err}")
    release(lock)

    # 12. Return unified diff
    return compute_myers_diff(path, file_content, working)
```

## Implementation notes

- **I/O error handling:** All I/O primitives (`resolve_symlinks`, `is_regular_file`, `read_file`, `stat`, `os_create_temp`, and the external-modification re-read) can fail. Any such failure is returned immediately as a tool error. If the lock is held at the time of failure, it is released before returning.
- **`count_lines`:** editor-style line count — `strings.Count(s, "\n")`, plus 1 if `s` is non-empty and does not end with `\n`. Returns 0 for an empty string. Used for range validation (`total_lines`); distinct from `count_newlines` which is used only for the `replace` line-limit guard.
- **Diff algorithm:** Myers diff, returned as a standard unified diff string.
- **Atomic write:** `os.CreateTemp` in the same directory creates a uniquely named temp file with `O_CREATE|O_EXCL` at mode `0600`. Contents are written and flushed, then `os.Chmod` sets the original file’s mode, then `os.Rename` replaces the original atomically on POSIX. Writing at `0600` before chmod prevents a window where the temp file is world-readable. On any write, chmod, or rename failure, the temp file is removed before returning the error. Same-directory placement guarantees both paths are on the same filesystem.
- **Per-file locking:** a `sync.Map` of `{sync.Mutex, refcount}` keyed by absolute path. `acquire`: lock the map, get or create the entry and increment refcount, unlock the map, then lock the entry mutex. `release`: unlock the entry mutex, then lock the map, decrement refcount, delete the entry if refcount reaches zero, unlock the map. Decrement-then-delete happens under the map lock so that a new goroutine cannot grab a stale pointer to an entry that is being removed.
- **External-modification detection:** SHA-256 of the file content is computed immediately after the read. Before writing, the file is re-read and its hash compared. A mismatch means an external process modified the file during the edit — the operation fails rather than silently overwriting unrelated changes. This is best-effort: a modification occurring between the second read and the rename would not be detected.
- **Line endings:** matching is byte-exact. The server assumes LF (`\n`). Files with CRLF line endings will fail to match `find` supplied with LF — the not-found error surfaces this hint explicitly.
- **Substring matching:** `find_substring_matches` returns non-overlapping matches in left-to-right order, consistent with Go’s `strings.Index`/`strings.Count` behavior. Example: `"aa"` in `"aaa"` yields one match at offset 0, not two overlapping matches.
- **Line-span semantics:** a match’s `end_line` is the line on which its last byte resides. A trailing newline is the terminator for its own line: `"foo\n"` on line 3 has `end_line == 3`, not `4`. `count_newlines` used in the line-limit guard is `strings.Count(s, "\n")`.
- **`file_replace_all` scope containment:** a match is selected only if it falls fully within `[start_line, end_line]`. A multi-line match that crosses either boundary is not replaced. This matches the least-surprising interpretation of “restrict to this range.”
- **`file_replace_all` non-recursive:** replacement text is not re-searched even if it contains `find`. This matches `strings.ReplaceAll` semantics and is consistent with the pre-pass approach used by `file_replace`.
- **Symlink handling:** the path is resolved via `filepath.EvalSymlinks` before locking. The resolved target is then verified to be a regular file (`os.Stat` + `Mode().IsRegular()`). The lock key and write target are the real path. This ensures two calls through different symlinks to the same file serialize correctly, and that `rename` operates on the real file rather than replacing the symlink itself.
- **No shell involvement:** the entire operation is in-process Go. No `exec`, no escaping.

## Why not exec_sync?

The existing file-write workflow shells out Python/bash and redirects text into a file, creating two problems:

1. **Shell escaping:** quotes, backslashes, and special characters require careful escaping that is error-prone and model-unfriendly.
2. **Verification overhead:** the model must follow up with `cat` or `grep` to confirm the write applied correctly, adding a round-trip.

`file_replace` and `file_replace_all` eliminate both: writes happen in-process (no shell), and the returned diff is immediate proof of what changed.

## Out of scope

**Workspace boundary**
Path confinement to an allowed root (e.g. the jail volume) is enforced at the server level by existing middleware, not per-tool. These tools operate on any regular file the server process can reach after symlink resolution. Adding a per-tool allowlist would duplicate that logic.

**Substring matching is byte-exact**
Both tools match substrings byte-for-byte, including indentation and whitespace. Auto-formatter runs, import reordering, or generated comment additions can invalidate a `find` block that was valid moments before. This is the intended behavior for a precise surgical tool — indentation-aware matching, line-based patches, or AST-aware edits are different tools for a different purpose.

**Durability guarantees**
A successful response means the rename completed — the replacement is visible to subsequent reads on the same filesystem. It does not guarantee durable persistence to stable storage (e.g. after a crash on a network-mounted filesystem). For the target use case (local or container-mounted filesystems in agentic loops) this is sufficient.
