Proposed handler for the approved file-insert specification. Shares parsing and edit lifecycle with file_cursor.md. Awaiting model review.

# Functions

## Handler.HandleFileInsert(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, err error)

1. Start timing, initialize telemetry fields, and defer Handler.recordCursorEdit() so every return records exactly one call.
2. Call parseCursorRequest().
3. Extract and validate required content.
4. Call applyCursorEdit() with a pure transform returning original[:cursor] + content + original[cursor:].
5. Return mcp.NewToolResultText() with the edit result and nil transport error.

#### Errors

- **2.** if common parsing fails, set validation_error and return mcp.NewToolResultError() with its diagnostic and nil transport error.

---

- **3.** if content is missing or not a string, set validation_error and return mcp.NewToolResultError() with a parameter-specific diagnostic and nil transport error.
- **3.** if content contains null bytes or invalid UTF-8, set validation_error and return mcp.NewToolResultError() with the validation diagnostic and nil transport error.

---

- **4.** if editing fails, set edit_error and return mcp.NewToolResultError() with its diagnostic and nil transport error.

The deferred recording reads final telemetry fields at return, not values captured before parsing. Empty content is valid; file and cursor validation still precede no-op reporting. Content is otherwise preserved verbatim, with no insertion-specific newline cap. ctx is unused.
