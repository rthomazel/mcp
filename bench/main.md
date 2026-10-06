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
6. Build the server, registering every tool. Proposed additions: register file_insert with Handler.HandleFileInsert and file_delete with Handler.HandleFileDelete. Both require string path, integer line (minimum 1), and string anchor; optional boolean dry_run defaults to false. file_insert additionally requires string content (empty allowed); file_delete requires integer count (minimum 1). Descriptions explain first literal line-scoped anchor matching, empty-anchor start-of-line, unified diffs, and dry-run. Deletion description explicitly states Unicode code-point counting, cross-line deletion, EOF truncation, and successful EOF no-ops. Runtime parsing repeats schema validation.
7. Serve over stdio with the configured worker pool.

## shellCommandToolOptions(cfg, description) []mcp.ToolOption

1. Describe the command escaping rules.
2. Register the canonical commands parameter.
3. Register each configured alias.

#### Rationale

- The PATH prepends let agents discover mise-managed and ad-hoc tools without reinstalling.
- The worker pool size controls whether tool calls complete in submission order.
