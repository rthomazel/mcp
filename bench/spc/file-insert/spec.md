---
id: 2026-10-06-file-insert
type: spec
summary: Dedicated line-anchored insert editing.
author: Thom
created: 2026-10-06
agents: merlin
---

# FileInsert

## Status

Behavioral contract approved by Thom. Specification only; not yet implemented.

## Intent

Allow agents to add text without reproducing surrounding text in a replacement.
The hypothesis is that line-scoped anchors reduce effort and retries compared
with counting offsets or restating existing text. Tool selection alone does not
prove usability; evaluate successful edits, failures, and retries as well.

## Tool description

Insert content verbatim into an existing file after the first literal occurrence of anchor on the specified 1-based line. An empty anchor inserts at the start of the line; an empty file accepts line 1 with an empty anchor. No indentation or newlines are added automatically. Returns a unified diff; dry_run previews without writing. Invalid lines or missing anchors fail without writing.

## Interface

`file_insert(path, line, anchor, content, dry_run?)`

Insert `content` verbatim at the cursor. Content may span multiple lines.
No automatic indentation or newline is added. Empty content is a successful
no-op after validation.

## Cursor contract

`path` is an absolute path identifying an existing regular text file.
Paths, anchors, insertion content, and existing file content must be valid UTF-8
without null characters; unsupported inputs fail without writing. `line` is a positive, 1-based integer.
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
validate the inputs and captured file snapshot and return the proposed diff without
writing. Write-time external-modification checks apply only when an actual change
is committed; previews and successful no-ops do not guarantee the file remains
unchanged by another process.
Report a successful no-op explicitly. Neither tool creates missing files.

## Scope

One operation per call. Batching, fuzzy matching, numeric character offsets,
and anchor occurrence selectors are out of scope. These are proposed tools,
not documentation of an existing implementation.

## Acceptance examples

- With `hello world`, line 1, anchor `hello`, and content ` brave`, produce
  `hello brave world`.
- An empty anchor inserts before all existing text on the selected line.
- On an empty file, line 1 and an empty anchor insert the supplied content.
- When an anchor occurs twice on a line, insert after the first occurrence.
- A missing anchor or invalid line leaves the file unchanged, including in dry-run.
- Dry-run shows the same edit as a real call but leaves the file unchanged.

## Changelog

- Initial agreed specification for dedicated anchored insertion.
- Clarify the supported text/path domain and snapshot-based dry-run validation
  following review in [PR #55](https://github.com/rthomazel/mcp/pull/55).
