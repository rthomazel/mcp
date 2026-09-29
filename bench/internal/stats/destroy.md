# Stats command processing

The package destroys the raw command string, preserving only the normalized result, base command, hash, and redacted byte counts.

# Types

## ProcessedCommand

1. Normalized string
2. BaseCmd string
3. Hash string, hex SHA-256 of the normalized command
4. RedactedByteCounts []int

# Functions

## ProcessCommand(cmd, userPatterns) ProcessedCommand

1. Run the redaction pass, collecting byte counts.
2. Run the structural normalization pass, appending its byte counts.
3. Collect the long-token byte counts.
4. Run the long-token normalization pass.
5. Return the normalized command, base command, hash, and byte counts.

## extractBaseCmd(cmd) string

1. Split on the first unquoted shell operator and take the first non-cd segment.
2. Strip leading environment assignments and wrappers.
3. Return the first remaining token.

## firstRealSegment(cmd) string

1. Split on shell operators.
2. Skip blank and cd segments.
3. Return the first real segment.

## splitOnOperators(cmd) []string

1. Track quote state and split on ||, &&, |, and ;.

## consumeWrapper(tokens) []string

1. Drop the wrapper and its leading flags.

#### Rationale

- The pipeline is destructive by design, so the raw command is never stored.
- Wrapper stripping lets the base command reflect the user's intent, not the scheduler.
