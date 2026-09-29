---
type: documentation
id: 2026-09-29-file-create
summary: Documents the current behavior of the file_create tool.
created: 2026-09-29
updated: 2026-09-29
---

# file_create

## Description

`file_create` writes `content` to `path`, creating any missing parent directories. It is the
successor to the old `file_replace` create-mode. A non-empty existing file is refused unless
`overwrite` is true; a missing file, or an existing empty one, is unaffected. Directories,
devices, and other non-regular targets are always rejected.

## Tool schema

```json
{
  "name": "file_create",
  "description": "Create a new file, or overwrite an existing regular file when overwrite is true. Creates any missing parent directories. Returns a short message on success; on dry_run it returns a unified diff.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": { "type": "string", "description": "Absolute path to the file." },
      "content": { "type": "string", "description": "Full contents of the new file." },
      "dry_run": { "type": "boolean", "description": "Optional. If true, validate and compute the diff without writing to disk." },
      "overwrite": { "type": "boolean", "description": "Optional. If true, replace an existing non-empty regular file. Defaults to false." }
    },
    "required": ["path", "content"]
  }
}
```

## Behavior

- **Missing file**: content is written, any missing parent directories are created.
- **Existing empty file**: content replaces the empty file; the overwrite guard does not fire
  for an empty target, only for a non-empty one.
- **Missing-file precedence**: a missing target is reported as "file does not exist." rather
  than a find/replace diagnostic, since the target is validated before the content is.
- **Existing non-empty file, `overwrite=false`** (default): refused. A message is returned and
  the file is left untouched.
- **Existing non-empty file, `overwrite=true`**: the file is atomically replaced.
- **Non-regular target** (directory, device, symlink target, etc.): always rejected.
- **`dry_run=true`**: returns a unified diff without writing.

## Execution flow

1. **Input guards**. Reject non-absolute paths, null bytes, invalid UTF-8, `content` exceeding
   the newline limit, and `dry_run`/`overwrite` type errors.
2. **Resolve symlinks** — lock and operate on the real path.
3. **Inspect** the target: determine whether it is missing, empty, non-empty regular, or a
   non-regular file.
4. **Decision**:
   - Missing or empty -> proceed to write.
   - Non-empty regular -> require `overwrite=true`, else refuse.
   - Non-regular -> reject.
5. **Acquire** an exclusive per-file lock.
6. **Dry-run exit** — return the diff without writing.
7. **Atomic write** — temp file in the same directory, chmod to original mode, POSIX rename.
8. **Return** a short one-line message on success, or a unified diff on `dry_run`.

## Constraints

- **Symlink resolution**: the path is resolved via `filepath.EvalSymlinks` before locking; the
  lock key and write target are the real path.
- **External modification**: an external-modification re-read and SHA-256 comparison guard the
  write, same as `file_replace`.
- **Atomic write**: temp file is written at `0600` before chmod to avoid a world-readable
  window; same-directory placement guarantees the same filesystem.

## Out of scope

- **Workspace boundary**: path confinement to an allowed root is enforced at the server level,
  not per-tool.
- **Durability guarantees**: a successful response means the rename completed; it does not
  guarantee durable persistence to stable storage.
