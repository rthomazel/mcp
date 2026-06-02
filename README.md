# mcp

A collection of MCP servers.

[![License: BSD3](https://img.shields.io/badge/license-BSD3-green)](./LICENSE)

## Servers

| Directory | Image | Description |
| --------- | ----- | ----------- |
| [`bench/`](./bench/) | [![Docker](https://img.shields.io/badge/docker-ghcr.io%2Frthomazel%2Fbench--mcp-blue?logo=docker)](https://ghcr.io/rthomazel/mcp/bench) | Give your AI agent a real workbench — shell, file editing, background jobs, environment discovery, all inside Docker. |
| [`keys/`](./keys/) | [![Docker](https://img.shields.io/badge/docker-ghcr.io%2Frthomazel%2Fmcp%2Fkeys-blue?logo=docker)](https://ghcr.io/rthomazel/mcp/keys) | Configuration-driven MCP server that holds API credentials and exposes authenticated HTTP tools — secret values are injected server-side, never surfaced to the model. |
| [`postgres/`](./postgres/) | [![Docker](https://img.shields.io/badge/docker-ghcr.io%2Frthomazel%2Fmcp%2Fpostgres-blue?logo=docker)](https://ghcr.io/rthomazel/mcp/postgres) | Give your AI agent safe, structured access to a PostgreSQL database — schema introspection, read-only queries, guarded mutations, dry-run, and diagnostics. |

