Proposed shared plumbing for file_insert and file_delete. Reuses existing file-edit locking, checksum protection, diff generation, and atomic writes. Model awaiting review; no implementation yet.

# Types

## cursorRequest

1. path string
2. line int, positive and 1-based
3. anchor string, required but may be empty
4. dryRun bool

# Functions

## parseCursorRequest(args map[string]any) (request cursorRequest, toolErr string)

1. Extract required path and anchor strings and the line value, retaining successfully parsed fields for telemetry.
2. Parse line with parsePositiveInteger().
3. Extract optional dry_run, defaulting to false.
4. Validate absolute path, UTF-8, null bytes, and anchor line boundaries.
5. Return request and an empty toolErr.

#### Errors

- **1.** if path or anchor is missing or not a string, return a parameter-specific error. An explicitly empty anchor is valid.
- **1.** if line is missing, return a required-parameter error.

---

- **2.** if parsing fails, return its error without filesystem access.

---

- **3.** if supplied dry_run is not boolean, return a type error.

---

- **4.** if path is not absolute, return a path error.
- **4.** if path or anchor contains null bytes or invalid UTF-8, return a validation error.
- **4.** if anchor contains CR or LF, return a single-line-anchor error.

## parsePositiveInteger(value any, name string) (parsed int, toolErr string)

1. Inspect the numeric representation decoded by MCP.
2. Validate positivity, integrality, finiteness, and representability before conversion.
3. Convert to int and return it with an empty toolErr.

#### Errors

- **1.** if value is missing, null, boolean, a string, or otherwise nonnumeric, return a parameter-specific numeric-type error; never coerce it.

---

- **2.** if value is nonfinite, fractional, zero, or negative, return a positive-integer error.
- **2.** if value is outside int bounds, return a range error before conversion.
- **2.** if a float64 value exceeds the exactly representable integer range, return a range error.

## applyCursorEdit(request cursorRequest, transform func(string, int) string) (result string, toolErr string)

1. Call openFileForEdit().
2. Defer file.ReleaseLock() for the opened file for every subsequent return.
3. Call file.ResolveCursor() over the validated original content.
4. Invoke transform with original content and the resolved byte cursor.
   1. if the result equals original content, return explicit successful no-op text and an empty diff without writing; label dry-run as not written.
5. Call editedFile.commit() with transformed content and request.dryRun.
6. Return the unified diff, labeling dry-run output as not written.

#### Errors

- **1.** if opening fails, return its error; openFileForEdit() releases any lock it acquired before failure.

---

- **3.** if cursor resolution fails, return its bounded diagnostic without invoking transform or writing.

---

- **5.** if checksum verification or writing fails, return the commit error rather than reporting success.

The supplied transforms are pure insertion/deletion over valid UTF-8. File and cursor validation precede the no-op branch.

## Handler.recordCursorEdit(tool string, started time.Time, request cursorRequest, errorKind string)

1. Construct stats.ToolCall with tool name, start time, duration, parsed path, dry-run flag, and caller-classified error kind.
2. Call Handler.record() exactly once per invocation, including validation failures.

Use validation_error for parse/payload guards and edit_error for open/resolution/commit failures. Successful no-ops have no error kind. Do not record anchor or content, invent replacement counts, or add database columns. Stats-disabled behavior remains delegated to Handler.record().
