# Config

The tool loads configuration from environment variables only, applying defaults for unset values.

# Types

## Config

1. Timeout time.Duration
2. BackgroundTimeout time.Duration
3. Home string
4. MiseDir string
5. EditMaxLines int
6. MaxCandidates int
7. ToolCallWorkers int
8. ShellExpandCommands bool
9. ShellCommandsAliases []string
10. BackgroundNice int
11. StatsRedactPatterns []*regexp.Regexp

# Functions

## LoadConfig() (*Config, error)

1. Resolve the home directory, overriding it with BENCH_MCP_HOME when set.
2. Set the mise directory, overriding it with BENCH_MCP_MISE_DIR.
3. Fill the defaults.
4. Parse BENCH_MCP_SHELL_COMMANDS_ALIASES, rejecting reserved and invalid names.
5. Parse each remaining variable, rejecting malformed values.
6. Compile each redaction pattern, logging and skipping invalid regexes.
7. Return the configuration.

## parseShellCommandAliases(raw) []string

1. Split the comma-separated list and trim each alias.
2. Skip empty, reserved, and duplicates, logging warnings for reserved and invalid names.
3. Return the valid aliases.

#### Rationale

- ToolCallWorkers defaults to 1 so mcp-go's stdio transport drains the queue strictly FIFO, restoring deterministic completion order matching submission order.
- Reserved aliases commands and cwd are skipped because they are the canonical shell command parameters.
