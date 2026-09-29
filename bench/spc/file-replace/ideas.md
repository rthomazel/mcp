# Loose matching

Deferred. Extracted from the `FileReplace` spec so it stays out of the implemented scope. The mechanism that partially covers the same need is region targeting (`bench/spc/file-replace/spec.md`, §3).

> Not implemented. Deferred: loose matching overlaps with region targeting (#3) and adds a second resolution axis. Keep it out until the simpler capabilities land and telemetry confirms the need.

The current contract is byte-exact. Auto-formatters, import reordering, and generated-comment additions can invalidate a `find` block that was valid a moment before. For a non-deterministic writer, the probability of hitting those exact bytes on the first try is low.

The tool should accept a looser `find` — a short unique prefix, or a match tolerant of leading-whitespace and indentation drift — and apply it. The model gets to supply intent ("change the constant near this comment") rather than reproduce a whole indented block. The tool resolves the narrowest interpretation and reports what it matched.
