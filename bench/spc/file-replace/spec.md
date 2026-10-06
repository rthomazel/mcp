---
id: 2026-09-29-file-replace
type: spec
summary: Makes file_replace easy to use for non-deterministic models through independent hunks, diff-on-failure, and line/region targeting.
author: Thom
created: 2026-09-29
updated: 2026-10-06
agents: merlin, rook2
---

# FileReplace

## Description

`file_replace` should be easy to use for models that are not deterministic in nature. Previously it demanded that a model reproduce a unique substring exactly — byte-for-byte, including tabs and indentation — and if any single item in a batch fails, the whole batch is rejected. Models respond by reaching for the shell: writing a Python or heredoc script with its own assertions, or running `sed -i`. The telemetry shows these escape hatches are common and usually succeed, so the tool is the problem, not the models.

The intent is to remove the reasons a model would choose the shell over `file_replace`, without sacrificing the surgical precision that makes the tool valuable. Three capabilities accomplish this. Each is optional on its own but they share one direction: reduce the context the model must reproduce and provide actionable feedback when an exact match fails.

### 1. Independent hunk application

The previous implementation failed fast: every item in a batch is validated against the original file before any edit is applied, and one failing item rejects the entire batch. For a non-deterministic writer this is hostile — a model may send a batch where three hunks are right and one is slightly off, losing all three good edits because of the one bad guess.

The tool should apply each item independently. A hunk that does not match contributes nothing; the hunks that do match are still written. The response reports each hunk's outcome — applied, not matched, invalid region, or dropped because of overlap — so the model gets the same per-item visibility it would have had from a successful atomic batch. Atomicity is not relied upon by models; per-hunk feedback is.

### 2. Diff on failure

On success `file_replace` returns a unified diff — immediate proof of what changed. On failure it returns only a text error. That asymmetry is the defect: the model that has to `cat` the file afterward is doing exactly the verification overhead the tool was designed to remove.

When a hunk does not match, the tool should render a diff against plausible diagnostic candidates when available, clearly marked as not applied. When no candidate can be identified, it reports that fact with a bounded excerpt rather than inventing a location. The model sees what it tried to do, in the same artifact shape as success, without guessing whether its `find` was even close. This reuses the existing dry-run diff machinery.

Only confirmation is in scope: candidate previews never write. The model resends with a corrected exact substring or a narrower region to commit. Automatic nearest-candidate application is deferred in ideas.md.

### 3. Line and region targeting

`file_replace` resolves a match by matching text. When the model cannot reproduce the exact text — because it drifted, or because it does not know the indentation — the match fails.

The tool should narrow exact matching by line region. A model can supply a short byte-exact substring and the region containing the intended occurrence instead of quoting surrounding code to distinguish duplicates. This does not tolerate indentation drift within the substring; loose matching remains deferred.

Either region bound may be omitted, defaulting to the corresponding file boundary. Bounds are clamped to the file; an empty resulting range rejects only that hunk and never becomes an unrestricted search. Matching uses original-file coordinates and retains occurrences whose line spans overlap the region.

## Relationships between the three

The three capabilities each absorb one axis of model non-determinism. Line targeting removes the need to reproduce extra surrounding context solely to distinguish repeated text. Independent hunking ensures a bad guess on one hunk does not discard the good guesses in the same batch. Diff-on-failure gives the model verification when its guess was still wrong. Together they mean a model rarely needs to shell out to patch a file.

## Use cases

- **Patch several places in one call.** A model wants to add a constant and edit the config block that uses it. It sends both as one batch. If one is slightly wrong, the other still applies, and the response shows which.
- **Disambiguate by region.** A model supplies an exact substring repeated in several config blocks and limits the search to the intended block's lines.
- **Correct-course in one shot.** A model's `find` does not match. Instead of a bare error and a follow-up `cat`, it receives not-applied candidate diffs when candidates exist, or an excerpt explaining that none were found, and can correct its request without leaving the tool.

## Constraints and non-goals

- **Precision is preserved.** Region targeting still applies a single, well-defined edit. The tool never guesses among wildly different interpretations silently; it reports what it matched and where, and the model reacts.
- **No shell involvement.** The operation stays in-process. The escape hatch that motivated this work — piping text through the shell — is not reintroduced.
- **Overlap semantics.** Process resolved hunks in starting-location order, breaking ties by input order. Keep each hunk unless it overlaps an already kept hunk; drop the conflicting hunk and report which earlier hunk won. Unrelated hunks still apply.
- **file_replace_all is unchanged.** Its single-match-per-find contract and its existing line-range scoping are sufficient. This work targets `file_replace` only.
- **file_create is out of scope.** Write-only heredocs (`cat > file << EOF`) migrate to `file_create` by adoption, not by feature changes here. That tool is new and climbing fast on its own.

## Input and safety contract

The target must be an absolute path to an existing regular UTF-8 text file. Symlinks are resolved and edits operate on the real target while preserving its permissions. There is no hidden create mode: missing files belong to `file_create`. Target existence is checked before replacement-content validation.

Each replacement supplies required `find` and `replace` strings. Empty replacement text deletes the match. Empty find, identical find and replace, null bytes, invalid UTF-8, and replacement text exceeding the configured newline limit reject the entire request before resolution. Independent outcomes apply to matching, region, and overlap failures—not malformed replacement contents. Malformed requests and file I/O failures remain tool errors.

Optional `start_line` and `end_line` are integers in original-file coordinates; omitted or zero bounds default to the corresponding file boundary. Fractional or unrepresentable coordinates are rejected rather than truncated. `line_number` is replaced by these bounds. Optional `dry_run` computes the proposed diff and outcomes without writing. All finds resolve against the original snapshot: targeting text produced by an earlier edit requires another call.

Defaults are 50 newlines per replacement (`BENCH_MCP_EDIT_MAX_LINES`) and 5 diagnostic candidates per failed hunk (`BENCH_MCP_MAX_CANDIDATES`). No-match excerpts contain at most 10 lines. Candidate limits bound the number of previews, not individual line lengths.

Substring occurrences are non-overlapping and searched left-to-right. A match's end line contains its last byte: a trailing newline belongs to the line it terminates, not the next line. Empty files have zero lines; a final newline does not create another addressable line. Character positions in outcomes are byte offsets within the starting line. Matching preserves line endings and remains byte-exact; LF find text does not match CRLF content containing different bytes.

The response contains the kept hunks' unified diff followed by outcome tallies and each hunk's result in input order. Final outcomes are `applied`, `would_apply`, `not_matched`, `invalid_region`, and `overlap`. Applied outcomes identify original locations; dry runs explicitly say not written. Diagnostic candidates are always labeled not applied and rendered independently against original content. When every hunk is rejected, nothing is written; the response still reports the outcomes rather than a whole-request match error.

Edits use a per-target lock and a checksum recheck before an atomic replacement of the target. External-modification detection is best-effort: external changes between the final re-read and rename are not detected. Successful writes mean the rename completed, not guaranteed durable persistence after a crash. Dry runs validate the captured snapshot without write-time checksum revalidation. These safeguards remain shared with the other file-editing tools.

## Acceptance criteria

- A batch where one hunk fails still writes the hunks that match, and the response reports each hunk's status.
- A failed match renders clearly labeled not-applied candidate diffs when candidates exist; otherwise it reports no candidate and provides a bounded excerpt.
- A call may narrow a required byte-exact substring by either or both region bounds without reproducing surrounding context.
- An invalid region rejects only its hunk and never permits whole-file fallback.
- Overlapping hunks follow the deterministic keep/drop policy and dropped hunks are never reported as applied.
- Dry-run responses distinguish would-apply outcomes from actual writes; candidate previews never write.
