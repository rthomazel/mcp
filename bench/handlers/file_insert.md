# file_insert

Proposed handler for the approved file-insert specification. Shares request parsing, cursor resolution, edit lifecycle, no-op reporting, and telemetry with file_cursor.md. Awaiting model review.

# Functions

## Handler.HandleFileInsert(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Start timing before parsing; parse common arguments with parseCursorRequest.
2. Require content to be present and a string, allowing an empty string. Reject invalid UTF-8 and null bytes. Preserve all other content exactly, without indentation or newline normalization.
3. If validation succeeds, call applyCursorEdit with a pure transform returning original[:cursor] + content + original[cursor:].
4. Record exactly one stats call via recordCursorEdit, including validation failures. Classify parsing/payload errors as validation_error and applyCursorEdit failures as edit_error.
5. Return an MCP error result on failure, otherwise the returned text result. Tool failures use the MCP result rather than a transport error.

No replacement hunk or substring duplication is required. Empty insertion still validates the file and cursor before reporting no-op. No insertion-specific newline cap is introduced by this model; the approved contract permits multiline content.
