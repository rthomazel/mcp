# commandResult

The tool runs shell commands and formats their output.

# Types

## commandResult

1. Command string
2. Stdout string
3. Stderr string
4. ExitCode int
5. Duration string
6. DurationMS int64
7. TimedOut bool
8. Hint string
9. err string

## expandedCmd

1. cmd string
2. cwd string
3. hint string

# Functions

## Handler.HandleShell(ctx, req) (*mcp.CallToolResult, error)

1. Parse the command array, rejecting an invalid or empty request.
2. Default the working directory to the root.
3. Expand the commands using the configured policy.
4. Run each command, record a stats call, and stop on the first error.
5. Return the combined results, tagging them as multi when more than one command ran.

## unchangedCommands(commands, cwd) []expandedCmd

1. Copy each command to a single expandedCmd carrying the default working directory.

## expandCommands(commands, cwd) []expandedCmd

1. Parse a leading cd prefix into the working directory when the command defaulted to the root.
2. Split && chains into independent commands.
3. Attach a hint noting the cwd parse and the && split.
4. Return the expanded commands.

## parseCWD(cmd) (path, remainder)

1. Strip a leading cd prefix, returning the original command unchanged when it is absent.
2. Locate the first && separator.
3. Reject the path when it is empty or contains quotes or whitespace.

## splitOnAndAnd(cmd) []string

1. Walk the command, honoring single and double quotes, backtick subshells, and $(...) subshells.
2. Treat heredoc introducers as opaque so && inside them never splits.
3. Split only on unquoted && and trim empty parts.

## formatCommandResults(results, multi) string

1. Wrap each result in a command tag when more than one ran.
2. Emit metadata with exit, duration, and hint.
3. Emit stdout and stderr tags.

## runCommand(ctx, cfg, command, cwd) *commandResult

1. Apply the shell timeout to the context.
2. Launch bash, capturing stdout and stderr.
3. Capture the exit code and timeout state, returning an error result when the process cannot start.

#### Rationale

- Splitting && chains into independent commands is a deliberate behavioral change; a later command runs even when an earlier one fails, so each result carries a hint explaining the split.
- Heredoc bodies are opaque to splitting because shell scripts frequently embed literal && that must not split.
