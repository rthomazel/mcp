# Anchored file-edit orchestration

Proposed shared plumbing for file_insert and file_delete. Reuses existing file-edit locking, checksum protection, diff generation, and atomic writes. Model awaiting review; no implementation yet.

# Types

## cursorRequest

1. path string
2. line int, positive and 1-based
3. anchor string, required but may be empty
4. dryRun bool

# Functions

## parseCursorRequest(args map[string]any) (request cursorRequest, toolErr string)

1. Require string path and anchor and a numeric line. Distinguish missing anchor from an explicitly empty anchor.
2. Parse line with parsePositiveInteger. Reject non-boolean dry_run when supplied; default it to false.
3. Reject a non-absolute path, null bytes, or invalid UTF-8 in path or anchor. Reject CR or LF in anchor.
4. Return the parsed request; never access the filesystem here.

## parsePositiveInteger(value any, name string) (int, string)

1. Accept the numeric representation decoded by MCP only when finite, integral, positive, and representable as an int without overflow. Do not coerce strings, booleans, null, or fractional numbers.
2. Check bounds before conversion. For JSON float64 values, also reject values outside the exactly representable integer range.
3. Return the integer or a parameter-specific validation error. Shared by line and count.

## applyCursorEdit(request cursorRequest, transform func(string, int) string) (result, toolErr string)

1. Call openFileForEdit(request.path); on failure return its error. On success defer file.ReleaseLock for the entire remaining operation.
2. Resolve a byte cursor with file.ResolveCursor over the locked, validated original content. Return its bounded diagnostic on failure without invoking transform.
3. Invoke transform with original content and the resolved cursor. The two callers supply only pure insertion/deletion operations over valid UTF-8.
4. If working content equals original content, return an explicit successful no-op with an empty diff and no write. For dry-run label it no-op, not written. Validation and cursor resolution still precede this branch.
5. Otherwise call editedFile.commit(working, request.dryRun), preserving checksum checking, permissions, symlink target handling, and atomic replacement. Propagate commit errors before reporting success.
6. Return the unified diff; label dry-run output as not written.

## Handler.recordCursorEdit(tool string, started time.Time, request cursorRequest, errorKind string)

1. Record tool name, start time, elapsed duration, file path, dry-run flag, and error kind using the existing stats.ToolCall fields and Handler.record.
2. Use validation_error for parse/payload guards and edit_error for resolution/open/commit failures; callers supply the classified error kind rather than raw file contents. Successful no-ops have no error kind.
3. Do not record anchor or content, invent replacement counts, or add database columns. Tool names and outcomes support baseline usage/error analysis; retry inference remains separate.
