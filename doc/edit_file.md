# edit_file tool

Edit a file using a precise search and replace block. Designed to bypass shell quote escaping and offer guarantees.

## Tool schema

```json
{
  "name": "edit_file",
  "description": "Replace an exact substring in a file. Returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Absolute path to the file to edit."
      },
      "old_text": {
        "type": "string",
        "description": "Exact substring to replace, matched character-for-character including whitespace. Must occur exactly once (globally, or at line_number if given)."
      },
      "new_text": {
        "type": "string",
        "description": "The replacement text. Maximum 50 lines. Empty string deletes the matched block."
      },
      "line_number": {
        "type": "integer",
        "description": "Optional line number to narrow the match. Use when old_text alone is ambiguous across the file."
      }
    },
    "required": ["path", "old_text", "new_text"]
  }
}
```

## Limits (enforced server-side)

| Constraint | Value | Rationale |
| --- | --- | --- |
| `new_text` max lines | **50** (env: `JAIL_MCP_EDIT_MAX_LINES`) | Hard cap; keeps edits surgical regardless of file size |
| Match uniqueness | exactly 1 | Must resolve to a single occurrence; error content depends on how ambiguity manifests (see error matrix below) |

## Error matrix

All errors that identify match locations include 1 line of file context before and after each match.

| Matches | `line_number` | Error content |
| --- | --- | --- |
| 0 | omitted | Searches for first line of `old_text`; if found, reports line numbers with 1-line context around each hit; if not found anywhere, says so and points to whitespace/indentation |
| 0 | provided | "`old_text` not found at line N" + shows line N with 1-line context |
| >1 | omitted | Lists starting line of each match with 1-line context; suggests `line_number` or widening `old_text` |
| >1 | provided, spread | Lists starting line of each candidate with 1-line context; notes `line_number` N did not narrow to one |
| >1 | provided, same line | Char positions of each match + line content; suggests replacing the whole line |

## Execution flow

```python
def handle_edit_file(path, old_text, new_text, line_number=None):
    # 1. Input guards (no lock needed — pure validation)
    if path == "":
        return Error("path must not be empty.")
    if old_text == "":
        return Error("old_text must not be empty.")
    if old_text == new_text:
        return Error("No changes: old_text and new_text are identical.")
    if "\x00" in old_text or "\x00" in new_text:
        return Error("Null bytes detected; binary files are not supported.")
    if not is_valid_utf8(old_text) or not is_valid_utf8(new_text):
        return Error("old_text and new_text must be valid UTF-8.")
    if line_number is not None and line_number < 1:
        return Error(f"line_number must be ≥ 1, got {line_number}.")

    # 2. Acquire exclusive per-file lock (held until after rename)
    lock = acquire_file_lock(path)  # refcounted; entry removed from map when refcount reaches 0

    # 3. Read file and snapshot checksum for external-modification detection
    file_content = read_file(path)
    checksum = sha256(file_content)
    total_lines = count_lines(file_content)

    # 4. Validate line_number range now that we know the file length
    if line_number is not None and line_number > total_lines:
        release(lock)
        return Error(f"line_number {line_number} is out of range (file has {total_lines} lines).")

    # 5. Enforce 50-line cap on new_text (what actually gets written)
    if count_lines(new_text) > MAX_LINES:
        release(lock)
        return Error(f"Edit rejected: new_text is {count_lines(new_text)} lines. Max is {MAX_LINES}.")

    # 6. Find all substring matches; each match records (start_line, end_line, start_char)
    all_matches = find_substring_matches(file_content, old_text)

    # 7. Narrow by line_number if provided
    if line_number is not None:
        candidates = [m for m in all_matches if m.start_line <= line_number <= m.end_line]
    else:
        candidates = all_matches

    # 8. Uniqueness enforcement with targeted diagnostics
    if len(candidates) == 0:
        if line_number is not None:
            release(lock)
            return Error(
                f"old_text not found at line {line_number}.\n"
                + excerpt(file_content, line_number, radius=1)
            )
        else:
            first_line = first_line_of(old_text)
            partial = find_substring_matches(file_content, first_line)
            if partial:
                snippets = [excerpt(file_content, m.start_line, radius=1) for m in partial]
                release(lock)
                return Error(
                    f"Patch failed: first line of old_text found at line(s) "
                    f"{[m.start_line for m in partial]} but surrounding block did not match.\n"
                    + join(snippets)
                )
            else:
                release(lock)
                return Error(
                    "Patch failed: first line of old_text not found in file. "
                    "Check for whitespace or indentation differences. "
                    "If the file originated on Windows, line endings may be CRLF (\\r\\n) "
                    "while old_text uses LF (\\n) — matching is byte-exact."
                )

    if len(candidates) > 1:
        release(lock)
        all_on_one_line = all(m.start_line == candidates[0].start_line for m in candidates)
        if line_number is not None and all_on_one_line:
            char_positions = [m.start_char for m in candidates]
            return Error(
                f"Ambiguous at line {line_number}: old_text matched {len(candidates)} times "
                f"at characters {char_positions}. Replace the whole line to target a specific occurrence.\n"
                + excerpt(file_content, line_number, radius=1)
            )
        else:
            snippets = [excerpt(file_content, m.start_line, radius=1) for m in candidates]
            hint = " Provide line_number to narrow the search." if line_number is None else ""
            return Error(
                f"Patch failed: old_text matched {len(candidates)} locations "
                f"(starting at lines {[m.start_line for m in candidates]}).{hint}\n"
                + join(snippets)
            )

    # 9. replace_exact: uniqueness pre-validated above; guaranteed single substitution
    updated_content = replace_exact(file_content, old_text, new_text)

    # 10. External-modification check — re-read and compare checksum before committing
    if sha256(read_file(path)) != checksum:
        release(lock)
        return Error("Edit aborted: file was modified externally between read and write.")

    # 11. Atomic write — copy original permissions, then temp file + rename
    original_mode = stat(path).mode
    tmp_path = path + ".tmp"
    write_to_disk(tmp_path, updated_content, mode=original_mode)
    rename(tmp_path, path)  # POSIX-atomic
    release(lock)

    # 12. Return unified diff — confirms the patch without a secondary read
    return compute_myers_diff(path, file_content, updated_content)
```

## Implementation notes

- **Diff algorithm:** Myers diff, returned as a standard unified diff string.
- **Atomic write:** write to `<filename>.tmp` in the same directory, then `os.Rename`. Same-directory placement guarantees both paths are on the same filesystem, making the rename atomic on POSIX. The temp file is created with the original file's mode (`os.Stat` before write, `os.Chmod` before rename) to preserve execute bits and other permissions.
- **Per-file locking:** a `sync.Map` of `{sync.Mutex, refcount}` keyed by absolute path. Refcount is incremented on acquire and decremented on release; the entry is deleted from the map when it reaches zero. This prevents unbounded map growth while avoiding the race where a waiting goroutine holds a reference to a mutex that was already removed.
- **External-modification detection:** SHA-256 of the file content is computed immediately after the read. Before writing, the file is re-read and its hash compared. A mismatch means an external process modified the file during the edit — the operation fails rather than silently overwriting unrelated changes.
- **Line endings:** matching is byte-exact. The server assumes LF (`\n`). Files with CRLF line endings will fail to match `old_text` supplied with LF — the not-found error surfaces this hint explicitly.
- **replace_exact** is a single-substitution call (`strings.Replace(s, old, new, 1)`). The name signals that uniqueness is pre-validated — it is not a "first match wins" fallback.
- **No shell involvement:** the entire operation is in-process Go. No `exec`, no escaping.

## Why not exec_sync?

The current file-write workflow uses `exec_sync` to shell out Python/bash and redirect text into a file. This creates two problems:

1. **Shell escaping:** quotes, backslashes, and special characters in generated content require careful escaping that is error-prone and model-unfriendly.
2. **Verification overhead:** the model must follow up with a `cat` or `grep` to confirm the write applied correctly, adding a round-trip.

`edit_file` eliminates both: writes happen in-process (no shell), and the returned diff is immediate proof of what changed.
