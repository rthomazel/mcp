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
