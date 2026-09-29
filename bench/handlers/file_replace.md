# Replacement

1. find string
2. replace string
3. lineNumber int, 0 meaning not provided

# Types

## replacement

1. find string
2. replace string
3. lineNumber int, 0 meaning not provided

## locatedReplacement

1. origIdx int, the index of the replacement in the input batch
2. r replacement, the parsed find/replace pair
3. m file.Match, the resolved match location

# Functions

## Handler.HandleFileReplace(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Extract `path`, `dry_run`, and the `replacements` array from the request arguments; reject an empty array.
2. Iterate each replacement item, rejecting any non-object, and extract `find`, `replace`, and optional `line_number`; reject a non-integer `line_number`.
3. Time the operation, call `handleFileReplace`, and record a stats call with path, replacement count, per-item find/replace byte lengths, the dry-run flag, and the error kind.
4. If the call produced an error, return it as a tool error result.
5. Return the result string as a tool text result.

## Handler.handleFileReplace(path string, replacements []replacement, dryRun bool) (result, toolErr string)

1. Guard the path is absolute and the replacements array is non-empty.
2. Stat the path before validating replacements so a missing file reports "file does not exist." rather than a find-not-found error.
3. Open the file via openFileForEdit, which resolves symlinks, acquires the per-file lock, and rejects binary content, releasing the lock on return.
4. Run validateFindReplace on each replacement; reject on the first violation.
5. Reject any line_number below 1, then any line_number beyond the file length.
6. In a single pre-pass, resolve each replacement to exactly one candidate: gather matches, filter by line_number when given, and reject on zero or more-than-one match.
7. Sort candidates by start byte and reject any that overlap.
8. Apply each replacement in descending byte order, later edits first so earlier byte offsets stay valid.
9. Hand the result to commit for dry-run or atomic write, returning the unified diff and any error.

#### Rationale

- Validating the file exists before validating replacement contents gives the missing-file case a direct, honest error rather than a misleading find error.
- Applying edits from the end of the file backward keeps earlier byte offsets valid without recomputing them after each insertion.
- The pre-pass locks every candidate against the original content before any write, so chaining between replacements in one call is impossible; a follow-up call is required to target text a prior replacement produced.
