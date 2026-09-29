# replacement

The tool submits commands to run asynchronously, returning a job_id immediately.

# Functions

## Handler.HandleShellBackground(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Parse the command array, rejecting an invalid or empty request.
2. Default the working directory to the root.
3. Submit each command as a deprioritized background job.
4. Return the job IDs, tagged per command when more than one was submitted.
