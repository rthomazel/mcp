# edit_file tool

Surgically modify a file using a search-and-replace block. Designed to bypass shell quote-escaping traps and eliminate secondary `cat` verification calls by returning a unified diff of the applied change.

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

| Matches | `line_number` | Error content |
| --- | --- | --- |
| 0 | omitted | First line of `old_text` searched across file; if found, reports which lines and shows surrounding context; if not found, says so and points to whitespace/indentation |
| 0 | provided | "`old_text` not found at line N" + shows what is actually on line N |
| >1 | omitted | "matched at lines X, Y, Z — add `line_number` or widen `old_text` for uniqueness" |
| >1 | provided, spread across multiple lines | "matched at lines X, Y — `line_number` N did not narrow to one match" |
| >1 | provided, all on same line | "ambiguous at line N, matched at chars X and Y — replace the whole line to target a specific occurrence" |

## Execution flow

```python
def handle_edit_file(path, old_text, new_text, line_number=None):
    # 1. No-op guard
    if old_text == new_text:
        return Error("No changes: old_text and new_text are identical.")

    # 2. Acquire exclusive per-file lock (held until after rename)
    lock = acquire_file_lock(path)

    # 3. Read file
    file_content = read_file(path)

    # 4. Enforce 50-line cap on new_text (what actually gets written)
    if count_lines(new_text) > 50:
        release(lock)
        return Error(f"Edit rejected: new_text is {count_lines(new_text)} lines. Max is 50.")

    # 5. Find all substring matches; each match records (start_line, end_line, start_char)
    all_matches = find_substring_matches(file_content, old_text)

    # 6. Narrow by line_number if provided
    if line_number is not None:
        candidates = [m for m in all_matches if m.start_line <= line_number <= m.end_line]
    else:
        candidates = all_matches

    # 7. Uniqueness enforcement with targeted diagnostics
    if len(candidates) == 0:
        if line_number is not None:
            release(lock)
            return Error(
                f"old_text not found at line {line_number}. "
                f"Line {line_number} contains:\n{get_line(file_content, line_number)}"
            )
        else:
            # Search for first line of old_text to help diagnose the mismatch
            first_line = first_line_of(old_text)
            partial = find_substring_matches(file_content, first_line)
            if partial:
                snippets = [excerpt(file_content, m.start_line, radius=3) for m in partial]
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
                    "Check for whitespace or indentation differences."
                )

    if len(candidates) > 1:
        release(lock)
        all_on_one_line = all(m.start_line == candidates[0].start_line for m in candidates)
        if line_number is not None and all_on_one_line:
            char_positions = [m.start_char for m in candidates]
            return Error(
                f"Ambiguous at line {line_number}: old_text matched {len(candidates)} times "
                f"at characters {char_positions}. Replace the whole line to target a specific occurrence."
            )
        else:
            match_lines = [m.start_line for m in candidates]
            hint = " Provide line_number to narrow the search." if line_number is None else ""
            return Error(
                f"Patch failed: old_text matched {len(candidates)} locations "
                f"(starting at lines {match_lines}).{hint}"
            )

    # 8. replace_exact: uniqueness pre-validated above; guaranteed single substitution
    updated_content = replace_exact(file_content, old_text, new_text)

    # 9. Atomic write — temp file + rename prevents partial-write corruption
    tmp_path = path + ".tmp"
    write_to_disk(tmp_path, updated_content)
    rename(tmp_path, path)  # POSIX-atomic
    release(lock)

    # 10. Return unified diff — confirms the patch without a secondary read
    return compute_myers_diff(path, file_content, updated_content)
```

## Implementation notes

- **Diff library:** `github.com/hexops/gotextdiff` (Myers algorithm), same as planned for other diff output in the server.
- **Atomic write:** write to `<filename>.tmp` in the same directory, then `os.Rename`. Same-directory placement guarantees both paths are on the same filesystem, making the rename atomic on POSIX.
- **Per-file locking:** a `sync.Map` of `sync.Mutex` keyed by absolute path, acquired before the read and released after the rename. Prevents two concurrent clients from racing on the same file.
- **replace_exact** is a single-substitution call (`strings.Replace(s, old, new, 1)`). The name signals that uniqueness is pre-validated — it is not a "first match wins" fallback.
- **No shell involvement:** the entire operation is in-process Go. No `exec`, no escaping.

## Why not exec_sync?

The current file-write workflow uses `exec_sync` to shell out Python/bash and redirect text into a file. This creates two problems:

1. **Shell escaping:** quotes, backslashes, and special characters in generated content require careful escaping that is error-prone and model-unfriendly.
2. **Verification overhead:** the model must follow up with a `cat` or `grep` to confirm the write applied correctly, adding a round-trip.

`edit_file` eliminates both: writes happen in-process (no shell), and the returned diff is immediate proof of what changed.
