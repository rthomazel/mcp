# keys — design

## what it is

`keys` is an MCP server that holds API credentials and exposes configuration-driven,
authenticated HTTP tools to agents. The agent never sees secret values — only header
names and tool descriptions. `keys` injects credentials at call time server-side.

## responsibilities

- Load secrets at startup from Docker Secrets (`/run/secrets/`)
- Read a YAML config file that defines named tools, one per API
- Register those tools dynamically with the MCP protocol at startup
- On tool call: validate and construct the request, inject secret headers, execute, return response
- Return clear, structured errors without leaking secret values
- Crash on startup if config is invalid or any secret fails to load
- Expose rich tool descriptions so the agent is self-directed toward API docs

**Not responsible for:**

- Knowing anything about specific API endpoints or request shapes
- Pagination, retries, or response transformation
- Any protocol other than HTTPS (plain HTTP supported via per-tool `http: true` flag)
- Protecting against API data exfiltration through legitimate API calls — `keys` protects
  credentials from the model, not what the model does with access once granted

## config schema

See [`config.yaml`](../config.yaml) for a full annotated example.
The structs are defined in [`internal/config/config.go`](../internal/config/config.go).

```yaml
timeout_seconds: 30 # global request timeout; default 30
max_response_bytes: 1048576 # global response size cap; default 1MB
max_request_bytes: 102400 # agent-supplied body size cap; default 100KB

secrets:
  github_token:
    docker_secret: github_token # reads /run/secrets/github_token
  datadog_api_key:
    docker_secret: datadog_api_key
  datadog_app_key:
    docker_secret: datadog_app_key

mcp_tools:
  github:
    description: >
      Make authenticated requests to the GitHub REST and GraphQL API.
      Base URL: https://api.github.com
    docs:
      - https://docs.github.com/en/rest
      - https://docs.github.com/en/graphql
    base_url: https://api.github.com
    inject:
      Authorization:
        secret: github_token
        format: "Bearer {value}" # optional; if omitted, raw secret value is used as-is

  datadog:
    description: >
      Make authenticated requests to the Datadog API.
      Base URL: https://api.datadoghq.com
    docs:
      - https://docs.datadoghq.com/api/latest/
    base_url: https://api.datadoghq.com
    inject:
      DD-API-KEY:
        secret: datadog_api_key
      DD-APPLICATION-KEY:
        secret: datadog_app_key
```

### `http: true` flag

By default `base_url` must use HTTPS and must not contain a path component — only
scheme, host, and optional port are valid (e.g. `https://api.github.com`, not
`https://api.example.com/v1`). For local services or testing, set `http: true`
on the tool:

```yaml
mcp_tools:
  local_api:
    description: Local development API.
    base_url: http://localhost:8080
    http: true
    inject: ...
```

### secret types

v1 ships with `docker_secret` only. Docker Secrets mount at `/run/secrets/<name>` and
work in both production and development — no separate dev-only mechanism is needed.
The config schema is designed to accommodate additional secret types in future versions
without breaking changes:

| type                          | how it works                                                                   | unlocks                                                         |
| ----------------------------- | ------------------------------------------------------------------------------ | --------------------------------------------------------------- |
| `docker_secret` (v1)          | reads `/run/secrets/<name>`, trims trailing whitespace, rejects empty values   | any static token or key; works in prod and dev                  |
| `oauth2_client` (v2)          | client_id + client_secret → token endpoint → cached bearer                     | Salesforce, HubSpot, Zoom, Slack, most enterprise SaaS          |
| `google_service_account` (v2) | service account JSON → RS256 JWT → token exchange → cached bearer with refresh | BigQuery, GCS, Drive, Sheets, Pub/Sub, Vertex AI, Cloud Logging |
| `aws_sigv4` (v3)              | request signing — different primitive from header injection                    | all AWS services                                                |

## agent-facing tool interface

Every registered tool has the same input schema regardless of which API it targets:

```json
{
  "path": "string (required) — relative API path, e.g. /api/v2/logs/events/search",
  "method": "string (required) — any valid HTTP method: GET, POST, PUT, PATCH, DELETE, etc.",
  "body": "string (optional) — raw request body; set Content-Type in headers if needed",
  "headers": "object (optional) — non-secret headers, e.g. Content-Type: application/json"
}
```

The tool description is built from `description` + `docs` at startup:

```
Make authenticated requests to the Datadog API.
Base URL: https://api.datadoghq.com
Docs: https://docs.datadoghq.com/api/latest/
```

The agent knows where the API lives, where to find the docs, and that auth is handled.
It is the agent's responsibility to know which endpoints to call and with what body.

## request handling

### path validation

`path` must be a relative path: no scheme, no host, no `://`. The server rejects
absolute URLs and normalizes `..` segments before joining with `base_url`. The final
resolved URL's host must exactly match `base_url`'s host. Any path that fails these
checks returns an error to the agent before any network request is made.

### redirects

Redirects are disabled. The HTTP client does not follow redirects (`CheckRedirect`
returns `http.ErrUseLastResponse`). The agent receives the redirect response as-is
and may follow it with a second call.

### agent-supplied headers

Injected headers (from `inject` config) always win — agent-supplied headers with the
same canonical name are silently dropped before the request is sent. The following
headers are always rejected: `Host`, `Content-Length`, `Transfer-Encoding`,
`Connection`, `Upgrade`, `Proxy-Authorization`, `TE`, `Trailer`.

### response shape

The tool returns a structured object:

```json
{
  "status": 200,
  "headers": { "Content-Type": "application/json", "X-RateLimit-Remaining": "59" },
  "body": "..."
}
```

All response headers are passed through except hop-by-hop headers (`Connection`,
`Keep-Alive`, `Transfer-Encoding`, `Upgrade`, `TE`, `Trailer`, `Proxy-Authenticate`,
`Proxy-Authorization`). Any header whose value contains a known secret is replaced
with `[redacted]` using the same value-scan used for the response body.

### response scrubbing

Injected header names are stripped from response headers before return. Known secret
values (held in memory at startup) are scanned for in the response body; if detected,
a warning is logged and the body is replaced with a redaction notice. This is
best-effort — it does not cover encoded or nested representations. Only configure APIs
that do not echo request headers in their responses.

### operational limits

Both limits apply per tool call and return a structured error if exceeded.

| limit             | default | config key           |
| ----------------- | ------- | -------------------- |
| request timeout   | 30s     | `timeout_seconds`    |
| max response body | 1 MB    | `max_response_bytes` |
| max request body  | 100 KB  | `max_request_bytes`  |

## security model

### isolation

- `keys` runs as a dedicated non-root user (`uid 9999`) in its container
- Config file should be owned by that user, mode `0400`; Docker Secret file ownership
  depends on compose configuration and should be verified per environment
- The container has no shared volumes with `bench` or other containers
- `keys` sits on an internal Docker network; only the MCP broker can reach it
- No Docker socket mounted

### secrets

- All secrets are delivered via Docker Secrets (`/run/secrets/`)
- Secrets are read once at startup and held in memory only
- Secret values are never included in any response, log line, or error message
- Trailing whitespace is trimmed from all secret values on load
- Empty values after trimming are a fatal startup error

### logging

Safe fields only: tool name, HTTP method, normalized path (no query string), response status, duration,
request ID. No headers (injected or agent-supplied), no request body, no response body.
Docker Secret names are logged at startup (not values). Error messages include tool
name and status code only.

### compose config discipline

- Secrets are declared only on the `keys` service in compose, never in shared env files
- `bench` cannot `exec` into `keys` — no Docker socket, no `pid: host`
- Process namespace isolation prevents reading `/proc/<pid>/environ` across containers

## layout

```
keys/
  main.go            server wiring, MCP protocol, dynamic tool registration
  internal/
    config/
      config.go      Config struct, YAML parsing, validation
    secrets/
      secrets.go     secret resolution, Docker Secret loading, value trimming
    proxy/
      proxy.go       HTTP client, path validation, header injection, response scrubbing
  doc/
    design.md        this file
```

## auth machinery taxonomy

The complexity of `keys` grows by secret type, not by API. Every API that uses a given
authentication scheme gets the same implementation for free. From the agent's perspective
all secret types produce the same result: an injected header on an outbound request.

| secret type                   | how it works                                                                   | products unlocked                                                            |
| ----------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------------- |
| `docker_secret` (v1)          | static value from `/run/secrets/`                                              | GitHub, Datadog, Stripe, OpenAI, Anthropic, ~80% of SaaS APIs                |
| `oauth2_client` (v2)          | client_id + client_secret → token endpoint → cached bearer                     | Salesforce, HubSpot, Zoom, Slack, most enterprise SaaS                       |
| `google_service_account` (v2) | service account JSON → RS256 JWT → token exchange → cached bearer with refresh | BigQuery, GCS, Drive, Sheets, Pub/Sub, Vertex AI, Cloud Logging — entire GCP |
| `aws_sigv4` (v3)              | request signing (not just header injection) — different primitive              | all AWS services                                                             |

v2 adds OAuth2 and/or Google service account secret types. Both require token
acquisition, caching, and refresh logic — the implementation complexity is bounded
but real. OAuth2 scopes, audiences, and token endpoint styles differ across providers.
`google_service_account` requires RS256 JWT construction and Google's specific token
exchange flow. From the agent's perspective both still produce
`Authorization: Bearer <token>`; the difference is entirely internal to `keys`.

`aws_sigv4` signs the full request rather than injecting a header — architecturally
different and a separate design problem, deferred to v3 if ever.

See [testing.md](./testing.md) for the full test plan.

## out of scope (v1)

- BigQuery and any Google Cloud API (requires `google_service_account` secret type — v2)
- AWS services (requires `aws_sigv4` — v3)
- Database wire protocols — Postgres, MySQL — belong in dedicated MCP servers
- Response transformation, pagination helpers, retries
- Any non-HTTP/HTTPS protocol
- Binary and non-UTF-8 responses (UTF-8 text only in v1)
- Per-tool policy knobs (method allowlists, path prefix restrictions, per-tool timeout overrides)

## postgres note

Postgres is a wire protocol (libpq), not HTTP. The credential model (DSN, SSL certs,
connection pooling) and the tool model (`query`, `explain`, `list_tables`) are both
fundamentally different from what `keys` does. The "secrets out of model context"
principle applies equally, but that is the only shared concern. Postgres belongs in
its own MCP server (idea [12]).
