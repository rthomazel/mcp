# file_delete

Proposed handler for the approved file-delete specification. Shares request parsing, cursor resolution, edit lifecycle, no-op reporting, and telemetry with file_cursor.md. Awaiting model review.

# Functions

## Handler.HandleFileDelete(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Start timing before parsing; parse common arguments with parseCursorRequest.
2. Require count and parse it with parsePositiveInteger. Zero, negative, fractional, missing, and nonnumeric counts fail even at EOF.
3. If validation succeeds, call applyCursorEdit with a pure transform: compute end = file.AdvanceCodePoints(original, cursor, count), then return original[:cursor] + original[end:].
4. Record exactly one stats call via recordCursorEdit, including validation failures. Classify parsing errors as validation_error and applyCursorEdit failures as edit_error.
5. Return an MCP error result on failure, otherwise the returned text result. Tool failures use the MCP result rather than a transport error.

The anchor is retained. Deletion may cross line boundaries and stops at EOF without error. A resolved cursor at EOF produces a successful, clearly reported no-op. No count-sized allocation or count-to-byte arithmetic is performed.
