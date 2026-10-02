---
id: 2026-09-29-file-replace
type: spec
summary: Makes file_replace easy to use for non-deterministic models through independent hunks, diff-on-failure, and line/region targeting.
author: Thom
created: 2026-09-29
agents: merlin, rook2
---

# FileReplace

## Description

`file_replace` should be easy to use for models that are not deterministic in nature. Today it demands that a model reproduce a unique substring exactly — byte-for-byte, including tabs and indentation — and if any single item in a batch fails, the whole batch is rejected. Models respond by reaching for the shell: writing a Python or heredoc script with its own assertions, or running `sed -i`. The telemetry shows these escape hatches are common and usually succeed, so the tool is the problem, not the models.

The intent is to remove the reasons a model would choose the shell over `file_replace`, without sacrificing the surgical precision that makes the tool valuable. Three capabilities accomplish this. Each is optional on its own but they share one direction: reduce the context the model must reproduce and provide actionable feedback when an exact match fails.

### 1. Independent hunk application

`file_replace` currently fails fast: every item in a batch is validated against the original file before any edit is applied, and one failing item rejects the entire batch. For a non-deterministic writer this is hostile — a model may send a batch where three hunks are right and one is slightly off, losing all three good edits because of the one bad guess.

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

## Acceptance criteria

- A batch where one hunk fails still writes the hunks that match, and the response reports each hunk's status.
- A failed match renders clearly labeled not-applied candidate diffs when candidates exist; otherwise it reports no candidate and provides a bounded excerpt.
- A call may narrow a required byte-exact substring by either or both region bounds without reproducing surrounding context.
- An invalid region rejects only its hunk and never permits whole-file fallback.
- Overlapping hunks follow the deterministic keep/drop policy and dropped hunks are never reported as applied.
- Dry-run responses distinguish would-apply outcomes from actual writes; candidate previews never write.
