# file_replace

Forgiving file editing. Each replacement in a batch resolves independently: byte-exact hits apply always, loose hits (region, prefix, or indentation drift) apply only when `applyNearestCandidate` is set, and unresolvable hits are reported with a candidate diff instead of failing the whole batch.

## replacement

1. find string, optional — intended text; required unless a region is supplied
2. replace string
3. lineNumber int, 0 meaning not provided
4. startLine int, 0 meaning not provided
5. endLine int, 0 meaning not provided

## locatedReplacement

1. origIdx int, index of the replacement in the batch
2. r replacement, the parsed pair
3. m file.Match, the resolved exact match

## looseCandidate

1. startByte int
2. endByte int
3. line int, 1-based anchor line for reporting
4. kind string, "region" | "prefix" | "indent"

## hunkStatus

1. index int, 1-based position in the batch
2. outcome string, "applied" | "applied_candidate" | "not_matched"
3. located *locatedReplacement, present when outcome is "applied"
4. candidate *looseCandidate, present when outcome is "applied_candidate"
5. diagnostics string, present when outcome is "not_matched"

## Functions

## Handler.HandleFileReplace(ctx Context, req CallToolRequest) (*CallToolResult, error)

1. Extract path, dryRun, applyNearestCandidate, and the replacements array; reject an empty array.
2. For each replacement, extract find, replace, lineNumber, startLine, endLine; reject a non-integer coordinate.
3. Time the call, invoke handleFileReplace, and record stats with the usual path, count, byte lengths, and dryRun.
4. Return an error result when handleFileReplace fails, otherwise a text result.

## handleFileReplace(path, replacements, dryRun, applyNearestCandidate) (result, toolErr string)

1. Guard path is absolute, the batch is non-empty, and the file exists.
2. Open the file via openFileForEdit, releasing the lock on return.
3. Resolve each replacement via resolveReplacement, classifying it as exact, candidate, or unmatched.
4. Build the applied set: every exact match, plus every candidate when applyNearestCandidate is set.
5. Sort the applied spans by start byte and reject any that overlap.
6. Apply the applied hunks in descending byte order into the working content.
7. Compute the applied diff over the real path with file.ComputeDiff, honoring dryRun.
8. Build each replacement's hunkStatus from its resolution and its applied/not-applied role.
9. Concatenate the applied diff and the per-hunk status lines; append candidate previews for unmatched hunks.
10. Return the assembled text, or an error when a guard or overlap fails.

## renderStatus(statuses []hunkStatus) string

1. Prefix with the applied / applied_candidate / not_matched tally.
2. Emit one line per hunk: its outcome, index, and — for candidates — the anchor line and kind; for unmatched — the diagnostics.

#### Rationale

- Resolving every replacement independently means a bad guess on one hunk never discards the good guesses in the same batch.
- Applying only the applied set (exact, plus candidates when opted in) keeps confirmation mode from writing fuzzy locations while automatic mode absorbs them.
- Overlap is rejected across the applied set because applying one span invalidates the byte offsets of another; independent application is per-hunk, not per-byte.
