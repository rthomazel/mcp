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

The intent is to remove the reasons a model would choose the shell over `file_replace`, without sacrificing the surgical precision that makes the tool valuable. Three capabilities accomplish this. Each is optional on its own but they share one direction: stop requiring the model to guess the file's exact bytes, and let the tool resolve the model's intent.

### 1. Independent hunk application

`file_replace` currently fails fast: every item in a batch is validated against the original file before any edit is applied, and one failing item rejects the entire batch. For a non-deterministic writer this is hostile — a model may send a batch where three hunks are right and one is slightly off, losing all three good edits because of the one bad guess.

The tool should apply each item independently. A hunk that does not match contributes nothing; the hunks that do match are still written. The response reports each hunk's outcome — applied, or not matched — so the model gets the same per-item visibility it would have had from a successful atomic batch. Atomicity is not relied upon by models; per-hunk feedback is.

### 2. Diff on failure

On success `file_replace` returns a unified diff — immediate proof of what changed. On failure it returns only a text error. That asymmetry is the defect: the model that has to `cat` the file afterward is doing exactly the verification overhead the tool was designed to remove.

When a hunk does not match, the tool should still render a diff. The diff is rendered against the nearest candidate — the closest thing to what the model asked for — and marked as not applied. The model sees what it tried to do, in the same artifact shape as success, without guessing whether its `find` was even close. This reuses the existing dry-run diff machinery.

Two ways to resolve a diff-on-failure exist, and the tool must support both:

- **Confirmation model.** The tool renders the candidate diff but does not write. The model resends with a tighter `find` or a narrower region to commit. Nothing is written to the wrong location.
- **Automatic model.** The tool applies to the nearest candidate and returns the real diff, flagged with which location it picked. The model can see the change and react.

The confirmation model is the safe default; the automatic model is the more forgiving option that absorbs the model's "didn't know which to pick" noise entirely.

### 3. Line and region targeting

`file_replace` resolves a match by matching text. When the model cannot reproduce the exact text — because it drifted, or because it does not know the indentation — the match fails.

The tool should also resolve by location. A model may target a region — "the block in `buildSessionConfig`" — or narrow by line number, without supplying a matching substring at all. This kills ambiguity outright: if `MaxStreamWindowSize` appears three times, "the one in `buildSessionConfig`" resolves without the model quoting surrounding code. `file_replace_all` already supports `start_line`/`end_line` scoping; this extends the same axis to `file_replace`.

## Relationships between the three

The three capabilities each absorb one axis of model non-determinism. Line targeting removes the pressure to reproduce exact bytes in the first place. Independent hunking ensures a bad guess on one hunk does not discard the good guesses in the same batch. Diff-on-failure gives the model verification when its guess was still wrong. Together they mean a model rarely needs to shell out to patch a file.

## Use cases

- **Patch several places in one call.** A model wants to add a constant and edit the config block that uses it. It sends both as one batch. If one is slightly wrong, the other still applies, and the response shows which.
- **Target by region, not by exact text.** A model knows the intent ("add the accept-backlog field to this yamux config") but not the exact indentation. It targets the region and the tool resolves it.
- **Correct-course in one shot.** A model's `find` does not match. Instead of a bare error and a follow-up `cat`, it receives a rendered diff against the nearest candidate and can resend narrowed, without leaving the tool.

## Constraints and non-goals

- **Precision is preserved.** Region targeting still applies a single, well-defined edit. The tool never guesses among wildly different interpretations silently; it reports what it matched and where, and the model reacts.
- **No shell involvement.** The operation stays in-process. The escape hatch that motivated this work — piping text through the shell — is not reintroduced.
- **Overlap and atomicity semantics.** Overlapping hunks within one call remain a rejection, because applying one would invalidate the other's byte offsets. Independent application is per-hunk, not per-byte.
- **file_replace_all is unchanged.** Its single-match-per-find contract and its existing line-range scoping are sufficient. This work targets `file_replace` only.
- **file_create is out of scope.** Write-only heredocs (`cat > file << EOF`) migrate to `file_create` by adoption, not by feature changes here. That tool is new and climbing fast on its own.

## Acceptance criteria

- A batch where one hunk fails still writes the hunks that match, and the response reports each hunk's status.
- A single-hunk call whose `find` does not match renders a unified diff against the nearest candidate and marks it not applied, rather than returning a bare text error.
- A call may target a region by start/end line, narrowing a byte-exact `find` to the intended location as the resolution basis, without the model reproducing exact bytes.
