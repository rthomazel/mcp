Handler for the approved file-delete specification. Shares parsing and edit lifecycle with file_cursor.md. Model approved through merged PR #55.

# Functions

## Handler.HandleFileDelete(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, err error)

1. Start timing, initialize telemetry fields, and defer Handler.recordCursorEdit() so every return records exactly one call.
2. Call parseCursorRequest().
3. Extract count and call parsePositiveInteger().
4. Call applyCursorEdit() with a pure transform computing end via file.AdvanceCodePoints() and returning original[:cursor] + original[end:].
5. Return mcp.NewToolResultText() with the edit result and nil transport error.

#### Errors

- **2.** if common parsing fails, set validation_error and return mcp.NewToolResultError() with its diagnostic and nil transport error.

---

- **3.** if count is missing or parsing fails, set validation_error and return mcp.NewToolResultError() with the parameter diagnostic and nil transport error, even at EOF.

---

- **4.** if editing fails, set edit_error and return mcp.NewToolResultError() with its diagnostic and nil transport error.

The deferred recording reads final telemetry fields at return, not values captured before parsing. The anchor is retained; traversal stops at EOF without error, including a successful no-op when already at EOF. No count-sized allocation is performed. ctx is unused.
