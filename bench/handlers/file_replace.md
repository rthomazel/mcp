# file_replace

Forgiving, per-hunk file editing. Each replacement in a batch resolves independently: a replacement whose `find` matches exactly once within its region applies; a replacement that matches zero or more than once within its region does not apply and is reported with a rendered preview. Region targeting narrows a byte-exact `find` to a single location via `start_line`/`end_line` instead of reproducing exact bytes, so indentation drift no longer defeats a plain find. A hunk that fails to match is rendered as a not-applied preview (diff-on-failure) instead of aborting the batch; unrelated hunks in the same batch still apply. Loose matching (a prefix/indent-drift tolerant `find`) is intentionally out of scope.

## replacement

1. find string, the intended substring to match; required
2. replace string, the replacement text, subject to the configurable newline limit
3. startLine int, 0 meaning not provided
4. endLine int, 0 meaning not provided

## locatedReplacement

1. origIdx int, the 0-based index of the replacement in the batch
2. r replacement, the parsed find/replace pair
3. m file.Match, the resolved single match location

## hunkStatus

1. pos int, the 1-based position of the replacement in the batch
2. outcome string, "applied" or "not_matched"
3. located *locatedReplacement, present only when outcome is "applied"
4. diagnostics string, the rendered preview present only when outcome is "not_matched"

## Functions

## Handler.HandleFileReplace(ctx Context, req CallToolRequest) (*CallToolResult, error)

1. Extract path, dryRun, and the replacements array from the request arguments; reject an empty array.
2. For each raw replacement item, assert it is a map[string]any; extract find and replace, and the optional start_line and end_line coordinates, coercing each coordinate with a float64 assertion and rejecting any non-integer coordinate.
3. Time the call, invoke handleFileReplace, and record a stats call with path, replacement count, per-item find/replace byte lengths, and the dryRun flag.
4. Return an error result when handleFileReplace returns an error, otherwise a text result.

## handleFileReplace(path string, replacements []replacement, dryRun bool) (result, toolErr string)

1. Guard the path is absolute, the replacements array is non-empty, and the file exists; return a file-does-not-exist error before validating replacement contents.
2. Open the file via openFileForEdit, resolving symlinks, acquiring the per-file lock, and rejecting binary content, releasing the lock on return.
3. Run validateFindReplace on each replacement and return on the first violation.
4. For each replacement, resolve it independently with resolveReplacement, producing one hunkStatus indexed by position.
5. Collect the located hunks into the applied set, preserving batch order.
6. Sort the applied hunks by match start byte ascending.
7. Walk the sorted applied hunks and reject any pair whose spans overlap.
8. Apply the applied hunks to the working content in descending byte order, later edits first so earlier byte offsets stay valid.
9. Compute the diff over the real path with file.ComputeDiff; on a non-dry-run call, write atomically via commit, then recompute the diff.
10. Assemble the result from the applied diff followed by renderStatus of every hunk.
11. Return the assembled text, or an error when a guard, overlap, or commit fails.

#### Errors

- **7.** if any applied spans overlap, return an error naming the two replacement indices and explaining that overlapping hunks cannot both apply.

## resolveReplacement(r replacement, content string, fileLines int, maxCandidates int) hunkStatus

1. Determine the region span: when startLine and endLine are both non-zero, call file.RegionSpan(startLine, endLine, fileLines); mark the region set only when it returns ok.
2. Gather all matches of r.find in content with file.FindMatches.
3. When the region is set, keep only those matches that overlap [startLine, endLine]; otherwise keep all matches.
4. Branch on the surviving matches:
   1. Zero matches: return a not_matched hunkStatus whose diagnostics are built by resolveMismatchDiagnostics.
   2. Exactly one match: return an applied hunkStatus wrapping a locatedReplacement over the match.
   3. More than one match: return a not_matched hunkStatus whose diagnostics list the ambiguous locations.

## resolveMismatchDiagnostics(label string, r replacement, content string, matches file.Match, maxCandidates int) string

1. When the region is set, report the region span and append a file.ExcerptRange snippet over it.
2. When the first non-empty line of find matches somewhere in content, append the partial-match hint from partialMatchDiagnostic.
3. Otherwise append a line stating find did not match, suggesting whitespace, indentation, or CRLF line endings.

## renderStatus(statuses []hunkStatus) string

1. Prefix the output with the applied and not_matched tallies.
2. For each hunk, in batch order, emit a line: applied hunks show the match start/end line and char offset; not_matched hunks show the diagnostics.

#### Rationale

- Resolving every replacement independently means a bad guess on one hunk never discards the good hunks in the same batch.
- Region targeting narrows a byte-exact find to a single location without the model reproducing exact bytes, absorbing the indentation drift that would otherwise defeat a plain find.
- The applied set is checked for overlap once, up front, because applying one span invalidates the byte offsets of any span that crosses it; independent application is per-hunk, not per-byte.
- A not_matched hunk is rendered as a preview rather than aborting the batch, giving the model the diff-shaped feedback it expects on failure.

## Shared helpers (existing, not added here)

- openFileForEdit, validateFindReplace, partialMatchDiagnostic, and (editedFile).commit live in file_edit.go and are reused by handleFileReplace as-is.
- file.FindMatches, file.RegionSpan, file.ExcerptRange, and file.ComputeDiff live in the file package; RegionSpan is the only new primitive.
