---
id: 2026-11-13-keys-architecture
type: documentation
summary: Documents the layout, startup sequence, and transports of keys-mcp.
author: Thom
created: 2026-11-13
agents: claude
---

# architecture

## Description

`keys` is an MCP server that holds API credentials and exposes one authenticated HTTP tool per configured API.
The agent supplies a path, method, and optionally a body and headers; the server adds secret headers, performs
the request, and returns the response. Secret values never reach the agent.

## Layout

```
main.go                        flag parsing, startup, dynamic tool registration, stdio serving
internal/config/config.go      Config types, YAML loading, defaults, validation
internal/secrets/secrets.go    Docker Secret loading into an in-memory Store
internal/proxy/proxy.go        URL validation, header injection, request execution, redaction
bin/keysmcphttp                container entrypoint selecting the transport
```

## Startup sequence

1. Parse the `--config` flag (default `config.yaml`, resolved against the working directory).
2. Load and validate the config. Any failure is fatal.
3. Load all secrets from the secrets directory. Any failure is fatal.
4. Log version, timeout, tool names, and secret names (never values).
5. Build the proxy and register one MCP tool per entry in `mcp_tools`.
6. Serve on stdio.

## Request flow

Tool call → argument extraction → `path` and `method` presence check → `Proxy.Do` → JSON-encoded response as the
tool result text. Any error becomes an MCP tool error result carrying the message; errors do not contain secret values.
See [http-tool](../http-tool/doc.md) for request handling.

## Transports

The server itself speaks stdio only. The container entrypoint wraps it:

| `KEYS_MCP_TRANSPORT` / `KEYS_MCP_HTTP` | behavior |
| --- | --- |
| unset | stdio |
| `mcpo` or `KEYS_MCP_HTTP=true` | `mcpo` on port 8001 (OpenAI-compatible HTTP) |
| `mcp-proxy` | `mcp-proxy` on port 8001 (native MCP over SSE) |

## Container

Runs as non-root uid/gid 9999 with working directory `/config`, where the config file is expected to be bind-mounted.

## Refs

- doc: 2026-11-13-keys-config
- doc: 2026-11-13-keys-http-tool
- doc: 2026-11-13-keys-secrets
