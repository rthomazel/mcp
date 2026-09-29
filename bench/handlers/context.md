# mount

The tool returns environment context by gathering OS, mounts, versions, and newly detected binaries.

# Types

## mount

1. mountpoint string
2. ro bool
3. persistent bool

# Functions

## Handler.HandleContext(ctx, _ mcp.CallToolRequest) (*mcp.CallToolResult, error)

1. Record the call on exit.
2. Gather OS name, architecture, free disk, and PATH from the shell.
3. Open /proc/mounts and parse mounts, logging but not failing on error.
4. Collect versions for the preinstalled tools.
5. Diff the PATH against the snapshot to detect newly installed binaries.
6. Return a formatted plain-text context.

## formatPlainTextContext(osName, arch, disk, path, timeout, version, home, mounts, versions, detected) string

1. Emit the metadata block with the scalar values.
2. List each mount labeled persistent, ro, or rw.
3. Print the ephemeral note telling the agent where to install to persist across sessions.
4. Print the preinstalled tool versions, aligned to the longest name.
5. Optionally print newly detected binaries.
6. Close the metadata block.

## parseMounts(r, home, miseDir) ([]mount, error)

1. Scan each mount line, skipping blank or malformed entries.
2. Skip the root mountpoint and known filesystem types.
3. Skip mounts under protected prefixes.
4. Mark ro when the options start with ro, persistent when the mountpoint equals home or miseDir.
5. Sort mounts by mountpoint and return them.

#### Rationale

- The mountpoint equals home or miseDir persists across sessions, so the agent knows where to install to survive.
- Skipped filesystems and prefixes exclude kernel and host noise from the context.
