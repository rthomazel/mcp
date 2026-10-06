---
id: 2026-10-06-file-delete
type: spec
summary: Dedicated line-anchored delete editing.
author: Thom
created: 2026-10-06
agents: merlin
---

# FileDelete

## Status

Behavioral contract approved by Thom. Specification only; not yet implemented.

## Intent

Allow agents to delete forward from a line-scoped anchor without reproducing
the deleted text. Keep the operation simple and deterministic.

## Interface

`file_delete(path, line, anchor, count, dry_run?)`

`count` is a positive integer. Delete up to `count` Unicode code points forward
from the cursor, stopping at EOF without error if fewer remain. The
anchor itself is not deleted. Deletion can cross line boundaries: line scoping
restricts anchor lookup, not the deletion range.

Count code points, not bytes or grapheme clusters. A tab counts as one; a
visually combined character may contain multiple code points. LF counts as
one, CRLF as two. Preserve remaining newline characters verbatim.

At EOF, including line 1 with an empty anchor in an empty file, a valid positive
count succeeds as a clearly reported no-op. Zero, negative, and noninteger counts are invalid.

## Cursor contract

`path` identifies an existing file. `line` is a positive, 1-based integer.
Find the first literal occurrence of `anchor` on that line and position the
cursor immediately after it. The anchor is retained. An empty anchor positions
the cursor at the start of the line. A nonempty anchor cannot contain a line
break or match across lines. Never search another line or guess a nearby match.
An empty file accepts line 1 with an empty anchor.

Invalid lines, missing anchors, invalid parameters, and inaccessible paths fail
without writing. Failure diagnostics include bounded relevant context rather
than the entire file. Content outside the edit is preserved, including newline
encoding; no indentation or newline normalization is performed.

## Results and safety

Return a unified diff of the change. `dry_run` defaults to false; when true,
perform the same validation and return the proposed diff without writing.
Report a successful no-op explicitly. Neither tool creates missing files.

## Scope

One operation per call. Batching, fuzzy matching, numeric character offsets,
and anchor occurrence selectors are out of scope. These are proposed tools,
not documentation of an existing implementation.

## Acceptance examples

- With `hello world`, line 1, anchor `hello`, and count 6, produce `hello`.
- The same cursor with count 100 also produces `hello`, stopping at EOF.
- An empty anchor and count 1 delete the first code point of the selected line.
- A cursor immediately before LF and count 1 remove that line break.
- A cursor immediately before CRLF and count 2 remove both code points.
- A multibyte Unicode code point is removed whole with count 1; combining
  sequences need not be removed as one visual character.
- A cursor at EOF, including line 1 with an empty anchor in an empty file,
  succeeds as a clearly reported no-op.
- Invalid counts, invalid lines, or missing anchors leave the file unchanged.
- Dry-run reports the proposed diff without changing the file.

## Changelog

- Initial agreed specification; positive `count` measures Unicode code points,
  deletion crosses lines and stops at EOF, and deletion at EOF succeeds as a no-op.
