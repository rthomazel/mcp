---
id: 2026-11-13-file-replace-loose-matching
type: spec
summary: Deferred idea to let file_replace accept a looser find tolerant of whitespace and indentation drift.
author: Thom
created: 2026-11-13
agents: claude
---

# FileReplace loose matching

## Status

Deferred. Not approved, not modelled, not implemented. Promoted from the original ideas note; the intent below is as recorded there.

## Intent

The current contract is byte-exact. Auto-formatters, import reordering, and generated-comment additions can invalidate a `find` block that was valid a moment before. For a non-deterministic writer, the probability of reproducing the exact bytes on the first try is low.

The tool could accept a looser `find`: a short unique prefix, or a match tolerant of leading-whitespace and indentation drift. The model would supply intent ("change the constant near this comment") rather than reproduce a whole indented block. The tool would resolve the narrowest interpretation and report what it matched.

## Why deferred

Loose matching overlaps with region targeting, which is implemented ([file_replace spec](spec.md), capability 3), and adds a second resolution axis. It stays out until telemetry confirms the need after the simpler capabilities.

## Open questions

- How ambiguity between several loose candidates is reported and whether any is applied.
- Whether looseness is opt-in per hunk or a tool-wide mode.
- How outcomes distinguish an exact match from a loose one.

## Refs

- spec: 2026-09-29-file-replace
