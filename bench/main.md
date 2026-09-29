# Server entry point

The package wires the configuration, the tool server, and the stdio transport.

# Types

# Functions

## run() error

1. Load the configuration.
2. Prepend the mise shims and home bin to the PATH.
3. Install a panic recovery that logs and exits.
4. Diff the PATH snapshot.
5. Construct the handler and defer its close.
6. Build the server, registering every tool.
7. Serve over stdio with the configured worker pool.

## shellCommandToolOptions(cfg, description) []mcp.ToolOption

1. Describe the command escaping rules.
2. Register the canonical commands parameter.
3. Register each configured alias.

#### Rationale

- The PATH prepends let agents discover mise-managed and ad-hoc tools without reinstalling.
- The worker pool size controls whether tool calls complete in submission order.
