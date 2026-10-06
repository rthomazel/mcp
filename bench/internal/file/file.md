# File editing primitives

The package provides substring matching, line counting, diagnostic excerpts, diff, atomic writes, and per-file locking.

# Types

## Match

1. StartByte int
2. EndByte int
3. StartLine int, 1-based line of the first byte
4. EndLine int, 1-based line of the last byte; a trailing newline terminates its own line
5. StartChar int, byte offset from the start of StartLine

# Functions

## ResolveCursor(content string, line int, anchor string) (cursor int, toolErr string)

Proposed primitive for file_insert and file_delete; existing replacement matching is unchanged.

1. Require positive line and a valid UTF-8 anchor containing neither null bytes nor CR/LF. Content has already passed openFileForEdit validation.
2. Locate the requested line by scanning LF boundaries while retaining original byte offsets. Follow CountLines for nonempty files: a final LF terminates its line rather than creating an extra addressable line. Special-case an empty file to permit line 1. This line-addressing detail is proposed for model review.
3. Exclude LF and its preceding CR, when present, from the searchable line text. Preserve both in original content. A lone CR is not a line separator under this LF-based convention.
4. Select the cursor within the searchable line text.
   1. if anchor is empty, return the line's start byte.
5. Find the first literal match with strings.Index().
6. Return the byte immediately after the match.

#### Errors

- **1.** if line is nonpositive or anchor contains invalid UTF-8, null bytes, CR, or LF, return a validation error.

---

- **2.** if the line does not exist, return requested line and available line count; never clamp or search elsewhere.

---

- **5.** if no match exists, return a missing-anchor diagnostic with the selected line quoted and truncated to at most 200 Unicode code points, adding a truncation marker when needed. Do not echo an unbounded anchor or file.

No diagnostic changes the cursor or authorizes a write.

## AdvanceCodePoints(content string, cursor int, count int) (end int)

1. Receive valid UTF-8 content, a resolved byte-boundary cursor, and a positive count.
2. Starting at cursor, decode one rune at a time and advance by its byte width until count code points have been consumed or EOF is reached.
3. Return the end byte offset. Count tabs and LF as one each and CRLF as two. Never normalize, split a UTF-8 encoding, or allocate proportional to count. Combining marks are separate code points.

## FindMatches(content, find) []Match

1. Return nil when find is empty.
2. Search left-to-right, collecting non-overlapping matches.
3. Compute the start/end line and start character for each match.

## CountLines(s) int

1. Count newlines.
2. Add one when the string is non-empty and does not end with a newline.
3. Return zero for an empty string.

## CountNewlines(s) int

1. Count newlines directly.

## FirstNonEmptyLine(s) string

1. Scan the lines and return the first containing non-whitespace, stripped of its newline.
2. Return empty when no such line exists.

## SplitLines(content) []string

1. Split on newlines.
2. Drop the trailing empty element produced when content ends with a newline.

## Excerpt(content, lineNum, radius) string

1. Split the content into lines.
2. Center the radius around the line, clamping to the file bounds.
3. Prefix each output line with its 1-based number.

## ExcerptRange(content, startLine, endLine, maxLines) string

1. Split the content into lines.
2. Select the lines in range, capping the span to maxLines.
3. Prefix each output line with its 1-based number.

## RegionSpan(startLine, endLine, fileLines int) (start, end int, ok bool)

1. Clamp startLine to 1 when it is below 1.
2. Clamp endLine to fileLines when it exceeds fileLines.
3. If startLine exceeds endLine, return (0, 0, false): the clamped range is empty, including when the file has no lines.
4. Otherwise return (startLine, endLine, true).

#### Rationale

- The caller fills omitted bounds before calling RegionSpan and uses the returned bounds for filtering. False means an invalid, empty region, never permission to search the whole file.

## ComputeDiff(path, before, after) string

1. Compute the edits using the Myers algorithm.
2. Render them as a unified diff.

## AtomicWrite(path, content, mode) error

1. Create a temp file in the same directory, guaranteeing the same filesystem.
2. Write the content and close it.
3. chmod the temp file to the mode to avoid a world-readable window.
4. Rename the temp file over the target.
5. Remove the temp file on any failure after creation.

## AcquireLock(path) *LockEntry

1. Find an existing lock entry or allocate a new one.
2. Increment the reference count and acquire the lock.

## ReleaseLock(path, entry)

1. Decrement the reference count.
2. Remove the entry when no other goroutines hold a reference.

#### Rationale

- FindMatches is non-overlapping and left-to-right, matching the strings.Index behavior.
- AtomicWrite writes the temp file at 0600 before chmod to avoid a world-readable window.
