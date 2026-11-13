---
id: 2026-11-13-file-replace-nearest-candidate
type: spec
summary: Deferred idea to let file_replace optionally apply a replacement to the nearest diagnostic candidate.
author: Thom
created: 2026-11-13
agents: claude
---

# FileReplace nearest-candidate application

## Status

Deferred. Not approved, not modelled, not implemented. Promoted from the original ideas note.

## Intent

An optional automatic mode could apply a replacement to the nearest candidate when a `find` does not match, and return the actual diff, explicitly identifying the selected location.

The implemented scope supports diagnostic previews only: approximate candidates are never written, and the caller must resend a corrected exact `find` or narrower region to commit.

## Prerequisites

Before introducing automatic application, define separately:

- candidate ranking;
- ambiguity handling; and
- explicit opt-in.

These must not be derived from the diagnostic preview heuristic.

## Refs

- spec: 2026-09-29-file-replace
