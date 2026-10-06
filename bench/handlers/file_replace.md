# file_replace

Forgiving, per-hunk file editing. Each replacement resolves independently against original content. A unique byte-exact match is eligible for application, subject to overlap filtering. Failed matches receive not-applied candidate previews when candidates exist, otherwise a diagnostic excerpt. Region targeting disambiguates repeated text using start_line/end_line; it does not tolerate indentation drift. Loose matching and automatic candidate application are deferred.

# Types

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
2. outcome string, resolved, applied, would_apply, not_matched, invalid_region, or overlap
3. located *locatedReplacement, present for resolved, applied, and would_apply outcomes
4. diagnostics string, rejection reason and any not-applied preview

# Functions

## Handler.HandleFileReplace(ctx Context, req CallToolRequest) (*CallToolResult, error)

1. Extract path, dryRun, and the replacements array from the request arguments; reject an empty array.
2. For each raw replacement item, assert it is a map[string]any; extract find and replace, and the optional start_line and end_line coordinates, coercing each coordinate with a float64 assertion and rejecting any non-finite, fractional, or out-of-int-range coordinate before conversion.
3. Time the call, invoke handleFileReplace, and record a stats call with path, replacement count, per-item find/replace byte lengths, and the dryRun flag.
4. Return an error result when handleFileReplace returns an error, otherwise a text result.

## handleFileReplace(path string, replacements []replacement, dryRun bool) (result, toolErr string)

1. Guard the path is absolute, the replacements array is non-empty, and the file exists; return a file-does-not-exist error before validating replacement contents.
2. Open the file via openFileForEdit, resolving symlinks, acquiring the per-file lock, and rejecting binary content, releasing the lock on return.
3. Run validateFindReplace on each replacement and return on the first violation.
4. For each replacement, call resolveReplacement() with the real path and original content. Assign the returned status its 1-based batch position and any located replacement its 0-based origIdx.
5. Collect the located hunks into the applied set, preserving batch order.
6. Sort the located hunks by match start byte ascending, breaking ties by original batch position.
7. Keep each hunk only when its half-open byte span does not overlap a previously kept hunk. For each dropped hunk, set its status to overlap, clear located, and report the conflicting hunk position.
8. Apply only the kept hunks to a working copy of original content in descending byte order, later edits first so earlier byte offsets stay valid.
9. If any hunks remain, call commit(working, dryRun) for the diff and optional atomic write; return any error before reporting success. Otherwise skip commit and use an empty diff. Mark kept hunks would_apply for dry-run, otherwise applied.
10. Assemble the result from the applied diff followed by renderStatus of every hunk.
11. Return the assembled text, or an error when a guard, or commit fails.

## resolveReplacement(path string, r replacement, content string, fileLines int, maxCandidates int) (status hunkStatus)

1. If both bounds are zero (omitted), search the whole file. Otherwise default the omitted start to 1 and omitted end to fileLines, then call file.RegionSpan(). If it returns false, return an invalid_region status with the requested bounds and file line count; never fall back to whole-file search.
2. Gather all matches of r.find in content with file.FindMatches.
3. When a region was supplied, keep only matches whose line spans overlap the returned clamped bounds; otherwise keep all matches. All coordinates refer to original content.
4. Branch on the surviving matches:
   1. Zero matches: return a not_matched hunkStatus whose diagnostics are built by resolveMismatchDiagnostics.
   2. Exactly one match: return a resolved hunkStatus wrapping a locatedReplacement over the match.
   3. More than one match: return not_matched with the ambiguous locations and separate not-applied diffs for up to maxCandidates matches in byte order, noting omitted candidates. Each preview substitutes replace at that exact span in a fresh copy of original content and calls file.ComputeDiff(path, content, hypothetical); never commit previews.

## resolveMismatchDiagnostics(path string, r replacement, content string, startLine int, endLine int, maxCandidates int) (diagnostics string)

1. Receive the effective search bounds from resolveReplacement(), using the whole file when no region was supplied. Report those bounds and a capped file.ExcerptRange() snippet.
2. Find the first non-empty line of find, preserving whitespace. Search for it within the bounds; if no hits exist, retry with leading and trailing whitespace trimmed. An empty anchor yields no candidates.
3. For each anchor hit in byte order, infer the candidate starting line by subtracting the anchor's zero-based line index within find. Select file.CountLines(find) whole lines, preserving original line endings. Discard spans extending outside the effective bounds and deduplicate identical spans.
4. For up to maxCandidates spans, substitute replace for that span in a fresh copy of original content and call file.ComputeDiff(path, content, hypothetical). Label each diff not applied, approximate whole-line candidate, with its original line range; note omitted candidates. If hypothetical content is unchanged, explicitly report an empty preview.
5. If no candidates remain, report no preview candidate, retain the excerpt, and suggest checking whitespace, indentation, or CRLF endings. Never invent a location or write preview content.

Candidate selection is a deterministic diagnostic heuristic, not fuzzy matching or permission to apply.

## renderStatus(statuses []hunkStatus) string

1. Prefix the output with tallies for each final outcome; resolved is internal and must not appear in output.
2. For each hunk, in batch order, emit its outcome. Applied and would_apply hunks show original match start/end line and char offset; rejected hunks show diagnostics. Label dry-run output explicitly as not written.

#### Rationale

- Resolving every replacement independently means a bad guess on one hunk never discards the good hunks in the same batch.
- Region targeting disambiguates repeated byte-exact text without requiring surrounding context in find; loose matching remains deferred.
- When applied hunks overlap, the first hunk in sorted byte order wins and every later hunk crossing its span is dropped, so one edit never invalidates the byte offsets of another; independent application is per-hunk, not per-byte.
- A not_matched hunk receives diagnostic previews when candidates exist, otherwise an excerpt; neither case aborts unrelated hunks.

## Diagnostic implementation helpers

- ambiguousDiagnostics(path, r, matches, content, maxCandidates) string renders the multi-match branch of resolveReplacement: exact-span previews capped in byte order.
- writeCandidatePreview(builder, path, content, hypothetical, label) emits a label and unified diff, or explicitly reports an empty preview.
- findWithinBounds(content, needle, startLine, endLine) []file.Match filters matches by overlapping line spans.
- anchorLineIndex(find) int returns the zero-based first non-whitespace line index.
- dedupeSpans(hits, anchorIndex, nLines, content, startLine, endLine) [][2]int infers whole-line candidate ranges, discards out-of-bounds ranges, and deduplicates while preserving hit order.
- lineBoundaries(content) [][2]int retains byte boundaries including original line terminators; lineByteSpan(content, startLine, endLine) (int, int, bool) selects the corresponding half-open byte range.
- noPreviewCandidate() string supplies the no-candidate hint; indent(text, prefix) string formats diagnostic lines in per-hunk output. Each rejected status ends with a newline so adjacent statuses remain separate.

## Tool registration

Register required string path, required nonempty replacements array with required string find/replace per item, optional integer start_line/end_line, and optional boolean dry_run. Descriptions explain independent original-file resolution, not-applied previews, and byte-order overlap selection with input-order ties. There is no line_number or automatic-candidate option.

## Shared helpers (existing, not added here)

- openFileForEdit, validateFindReplace, and (editedFile).commit live in file_edit.go and are reused by handleFileReplace as-is.
- file.FindMatches, file.RegionSpan, file.ExcerptRange, and file.ComputeDiff live in the file package; RegionSpan is the only new primitive.
