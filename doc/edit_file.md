# edit_file tool

Surgically modify a file using a search-and-replace block. Designed to bypass shell quote-escaping traps and eliminate secondary `cat` verification calls by returning a unified diff of the applied change.

## Tool schema

```json
{
  "name": "edit_file",
  "description": "Surgically modify a file with a search-and-replace. Finds the exact old_text block, replaces it with new_text, and returns a unified diff confirming the change. Limits: old_text must be ≥5 lines (≥1 if the file has <5 lines total); new_text must be ≤50 lines; old_text must match exactly once in the file.",
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
| Match uniqueness | exactly 1 | Ambiguous matches are rejected outright |

## Execution flow

```python
def handle_edit_file(path, old_text, new_text):
    # 1. No-op guard
    if old_text == new_text:
        return Error("No changes: old_text and new_text are identical.")

    # 2. Read file
    file_content = read_file(path)
    total_lines  = count_lines(file_content)

    # 3. Enforce minimum context lines in old_text
    min_context = 1 if total_lines < 5 else 5
    if count_lines(old_text) < min_context:
        return Error(f"old_text must be at least {min_context} lines for this file.")

    # 4. Enforce 50-line cap on new_text (what actually gets written)
    if count_lines(new_text) > 50:
        return Error(f"Edit rejected: new_text is {count_lines(new_text)} lines. Max is 50.")

    # 5. Exact-match uniqueness enforcement
    matches = find_exact_matches(file_content, old_text)
    if len(matches) == 0:
        return Error("Patch failed: exact old_text not found. Check whitespace and context.")
    if len(matches) > 1:
        return Error("Patch failed: ambiguous match. Add more unique surrounding context.")

    # 6. replace_exact: uniqueness pre-validated above; guaranteed single substitution
    updated_content = replace_exact(file_content, old_text, new_text)

    # 7. Atomic write — temp file + rename prevents partial-write corruption
    tmp_path = path + ".jail-mcp.tmp"
    write_to_disk(tmp_path, updated_content)
    rename(tmp_path, path)  # POSIX-atomic

    # 8. Return unified diff — confirms the patch without a secondary read
    return compute_myers_diff(path, file_content, updated_content)
```

## Implementation notes

- **Diff library:** `github.com/hexops/gotextdiff` (Myers algorithm), same as planned for other diff output in the server.
- **Atomic write:** `os.WriteFile` to a `.jail-mcp.tmp` sibling, then `os.Rename`. Both must be on the same filesystem (same directory) for rename to be atomic.
- **replace_exact** is a single-substitution call (`strings.Replace(s, old, new, 1)`). The name signals that uniqueness is pre-validated — it is not a "first match wins" fallback.
- **No shell involvement:** the entire operation is in-process Go. No `exec`, no escaping.

## Why not exec_sync?

The current file-write workflow uses `exec_sync` to shell out Python/bash and redirect text into a file. This creates two problems:

1. **Shell escaping:** quotes, backslashes, and special characters in generated content require careful escaping that is error-prone and model-unfriendly.
2. **Verification overhead:** the model must follow up with a `cat` or `grep` to confirm the write applied correctly, adding a round-trip.

`edit_file` eliminates both: writes happen in-process (no shell), and the returned diff is immediate proof of what changed.
