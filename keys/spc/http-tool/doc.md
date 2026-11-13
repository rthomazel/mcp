---
id: 2026-11-13-keys-http-tool
type: documentation
summary: Documents the agent-facing HTTP tool and how requests are validated, executed, and returned.
author: Thom
created: 2026-11-13
agents: claude
---

# http tool

## Description

Each entry in `mcp_tools` registers one MCP tool with the same input schema. The tool description is the
configured `description`, followed by "Docs:" and the configured URLs when present.

## Tool schema

| parameter | required | meaning |
| --- | --- | --- |
| `path` | yes | relative API path, may include a query string |
| `method` | yes | any HTTP method |
| `body` | no | raw request body string |
| `headers` | no | object of non-secret headers; non-string values are ignored |
| `response_headers` | no | `NONE` (default), `ALL`, or comma-separated header names |

## Request handling

1. Empty `path` or `method` returns an error.
2. A body larger than `max_request_bytes` is rejected.
3. The path is validated and joined onto `base_url`:
   - a path containing `://`, or parsing with a scheme or host, is rejected;
   - a missing leading slash is added;
   - the base URL's path is kept as a prefix, and the combined path is cleaned (`..` and duplicate slashes collapsed);
   - the query string is preserved, any fragment dropped.
4. Headers are built from agent headers minus the blocked set, then injected headers overwrite.
   Blocked: `Host`, `Content-Length`, `Transfer-Encoding`, `Connection`, `Upgrade`, `Proxy-Authorization`, `TE`, `Trailer`.
5. The request runs with the configured timeout.
6. The response body is read up to `max_response_bytes`; a larger body is an error, not a truncation.

## Redirects

Redirects to the same scheme and host are followed, up to 5 hops. Anything else, including hitting the hop cap,
returns the redirect response as-is so the agent may decide whether to follow it.

## Response

The tool result text is JSON: `status`, `headers` (omitted when none), and `body`.

Response headers:
- `NONE` or empty: none returned.
- `ALL`: all except hop-by-hop headers (`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`).
- A comma-separated list: only those names (case-insensitive), also minus hop-by-hop.
- Only the first value of each header is returned; values containing a secret become `[redacted]`.

The body is UTF-8 text as received; no transformation, pagination, or retry is performed.

## Errors

Errors are MCP tool error results prefixed with the tool name, for example oversize request, invalid path,
request failure, or oversize response. A non-2xx HTTP status is not an error; it is returned as the response status.

## Refs

- doc: 2026-11-13-keys-secrets
- doc: 2026-11-13-keys-config
