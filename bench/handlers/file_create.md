# replacement

The tool writes content to a path, creating missing parent directories. Overwriting a non-empty file requires explicit permission.

# Types

# Functions

## Handler.HandleFileCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Extract path, content, dryRun, overwrite from the request arguments.
2. Reject a missing content parameter.
3. Time the operation, call handleFileCreate, and record a stats call with path, the dry-run flag, the overwrite flag, and the error kind.
4. Return an error result if the call produced one, otherwise return the result text.

## Handler.handleFileCreate(path, content, dryRun, overwrite) (result, toolErr)

1. Guard the path is absolute, reject null bytes, and reject invalid UTF-8.
2. Create any missing parent directories before resolving symlinks, so a deeply missing path is created in one shot.
3. Resolve symlinks and take the per-file lock so two concurrent creates cannot both proceed, releasing the lock on return.
4. Stat the target. Reject non-regular files. Refuse a non-empty target unless overwrite is set.
5. Hand the target to commitCreate, which returns the diff on dry-run or a one-line message on success.

## commitCreate(realPath, content, dryRun, overwrite, info) (result, toolErr)

1. Return the diff on dry-run without writing.
2. Report file did not exist, file was empty, or file was overwritten depending on the target state.
3. Atomically write content at 0644, reporting any write failure.

#### Rationale

- Validating the file exists before validating replacement contents gives the missing-file case a direct, honest error rather than a find-not-found error.
- Applying edits from the end of the file backward keeps earlier byte offsets valid without recomputing them after each insertion.
- The pre-pass locks every candidate against the original content before any write, so chaining between replacements in one call is impossible; a follow-up call is required to target text a prior replacement produced.
