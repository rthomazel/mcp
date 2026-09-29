# replacement

The tool replaces every occurrence of a single find string with a replace string, optionally restricted to a line range.

# Types

# Functions

## Handler.HandleFileReplaceAll(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Extract path, find, replace, dryRun, and optional start_line, end_line from the request arguments, coercing floats to ints.
2. Reject a non-integer start_line or end_line.
3. Time the operation, call handleFileReplaceAll, and record a stats call with path, replacement count of one, the find/replace byte lengths, the dry-run flag, and the error kind.
4. Return an error result if the call produced one, otherwise return the result text.

## Handler.handleFileReplaceAll(path, find, replace, startLine, endLine, dryRun) (result, toolErr)

1. Read EditMaxLines and MaxCandidates from the configuration.
2. Guard the path is absolute and validate find/replace with validateFindReplace, rejecting an out-of-range or inverted start_line/end_line.
3. Open the file via openFileForEdit, which resolves symlinks, acquires the per-file lock, and rejects binary content, releasing the lock on return.
4. Reject a start_line or end_line beyond the file length.
5. Gather all matches of find, keeping only those fully contained within the scope when a range is given.
6. Reject when no candidate remains, offering a scoped excerpt when a range was given or a partial-match diagnostic otherwise.
7. Apply the replacement in descending byte order so earlier byte offsets stay valid.
8. Hand the result to commit for dry-run or atomic write, returning the unified diff and any error.

#### Rationale

- Scoping keeps a match only if fully contained in the range, matching the least-surprising interpretation of restrict to this range.
- Non-overlapping matches come from FindMatches, so no overlap check is needed before applying.
- Chaining is not supported; find is matched only against the original content.
