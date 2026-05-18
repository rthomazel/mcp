# edit_file tool

Surgically modify a file using a search-and-replace block. Designed to bypass shell quote-escaping traps and eliminate secondary `cat` verification calls by returning a unified diff of the applied change.

## Tool schema

```json
{
  "name": "edit_file",
  "description": "Surgically modify a file with a search-and-replace. Returns a unified diff confirming the applied change.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Absolute path to the file to edit."
      },
      "old_text": {
        "type": "string",
        "description": "The exact block of text to find and replace. Must match the file character-for-character including whitespace. Must be unique in the file — include 1-2 lines of unchanged surrounding context if needed. Minimum 5 lines (minimum 1 line if the file has fewer than 5 lines total). The server rejects edits where old_text matches zero or more than one location."
      },
      "new_text": {
        "type": "string",
        "description": "The replacement text. Maximum 50 lines. Can be empty to delete the matched block."
      }
    },
    "required": ["path", "old_text", "new_text"]
  }
}
```

## Limits (enforced server-side)

| Constraint | Value | Rationale |
| --- | --- | --- |
| `new_text` max lines | **50** | Hard cap; keeps edits surgical regardless of file size |
| `old_text` min lines | **5** (or **1** if file < 5 lines) | Forces surrounding context; prevents single-line ambiguity |
| Match uniqueness | exactly 1 | Ambiguous matches are rejected; error lists the line number of each match |

## Execution flow

```python
def handle_edit_file(path, old_text, new_text):
    # 1. No-op guard
    if old_text == new_text:
        return Error("No changes: old_text and new_text are identical.")

    # 2. Acquire exclusive per-file lock (held until after rename)
    lock = acquire_file_lock(path)

    # 3. Read file
    file_content = read_file(path)
    total_lines  = count_lines(file_content)

    # 4. Enforce minimum context lines in old_text
    min_context = 1 if total_lines < 5 else 5
    if count_lines(old_text) < min_context:
        release(lock)
        return Error(f"old_text must be at least {min_context} lines for this file.")

    # 5. Enforce 50-line cap on new_text (what actually gets written)
    if count_lines(new_text) > 50:
        release(lock)
        return Error(f"Edit rejected: new_text is {count_lines(new_text)} lines. Max is 50.")

    # 6. Exact-match uniqueness enforcement
    matches = find_exact_matches(file_content, old_text)  # returns list of (start_line, end_line)
    if len(matches) == 0:
        # Help the model diagnose the mismatch: search for the first line of old_text alone.
        # If it appears, the surrounding block diverged; report where and show context.
        # If it doesn't appear at all, say so explicitly.
        first_line = first_line_of(old_text)
        partial = find_line_matches(file_content, first_line)
        if partial:
            context_snippets = [excerpt(file_content, ln, radius=3) for ln in partial]
            release(lock)
            return Error(
                f"Patch failed: first line of old_text found at line(s) {partial} "
                f"but surrounding block did not match. File context around each:\n"
                + join(context_snippets)
            )
        else:
            release(lock)
            return Error(
                f"Patch failed: first line of old_text not found anywhere in file. "
                f"Check for whitespace or indentation differences."
            )
    if len(matches) > 1:
        release(lock)
        return Error(
            f"Patch failed: old_text matched {len(matches)} locations "
            f"(starting at lines {[m.start_line for m in matches]}). "
            f"Add more unique surrounding context to make the block unambiguous."
        )

    # 7. replace_exact: uniqueness pre-validated above; guaranteed single substitution
    updated_content = replace_exact(file_content, old_text, new_text)

    # 8. Atomic write — temp file + rename prevents partial-write corruption
    tmp_path = path + ".tmp"
    write_to_disk(tmp_path, updated_content)
    rename(tmp_path, path)  # POSIX-atomic
    release(lock)

    # 9. Return unified diff — confirms the patch without a secondary read
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
