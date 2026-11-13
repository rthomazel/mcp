---
id: 2026-11-13-keys-config
type: documentation
summary: Documents the keys YAML config schema, defaults, and startup validation rules.
author: Thom
created: 2026-11-13
agents: claude
---

# config

## Description

Configuration is a single YAML file. Loading applies defaults, then validates. Every validation failure is fatal:
the server does not start with an invalid config.

## Schema

| key | type | default | meaning |
| --- | --- | --- | --- |
| `timeout_seconds` | int | 30 | HTTP client timeout per tool call |
| `max_response_bytes` | int | 1048576 | response body cap |
| `max_request_bytes` | int | 102400 | agent-supplied body cap |
| `secrets` | map name → `{docker_secret}` | required, non-empty | secret sources |
| `mcp_tools` | map name → tool | required, non-empty | one MCP tool per entry |

A zero or missing numeric value takes the default.

Tool entry:

| key | meaning |
| --- | --- |
| `description` | required; first part of the tool description shown to the agent |
| `docs` | optional list of URLs appended to the tool description under "Docs:" |
| `base_url` | required; scheme, host, optional port, and optional path prefix |
| `http` | optional; allows a plain `http` base URL |
| `inject` | map header name → `{secret, format}` |

`format` uses `{value}` as the placeholder (for example `Bearer {value}`); when empty the raw secret is used.

## Validation rules

- `secrets` and `mcp_tools` must not be empty.
- Every secret must have a non-empty `docker_secret`.
- Every tool must have a non-empty `description` and `base_url`, and `base_url` must parse.
- Without `http: true`, `base_url` must use `https`. With it, `http` or `https` is accepted.
- Every `inject` entry must name a secret that exists in `secrets`.
- Malformed YAML or an unreadable file is fatal.

`base_url` may include a path (for example `https://api.example.com/v1`); the path is kept as a prefix when joining request paths.

## Secret types

Only `docker_secret` exists. The schema is a map per secret so other types can be added without breaking existing files.

## Example

[`config.yaml`](../../config.yaml) is an annotated example.

## Refs

- doc: 2026-11-13-keys-secrets
