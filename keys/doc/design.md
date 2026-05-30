# keys — design

## what it is

`keys` is an MCP server that holds API credentials and exposes configuration-driven,
authenticated HTTP tools to agents. The agent never sees secret values — only header
names and tool descriptions. `keys` injects credentials at call time server-side.

## responsibilities

- Load secrets at startup from Docker Secrets (or env vars in dev)
- Read a YAML config file that defines named tools, one per API
- Register those tools dynamically with the MCP protocol at startup
- On tool call: build the HTTP request, inject secret headers, execute, return response
- Return clear, structured errors without leaking secret values
- Expose rich tool descriptions so the agent is self-directed toward API docs

**Not responsible for:**
- Knowing anything about specific API endpoints or request shapes
- Pagination, retries, or response transformation
- Any protocol other than HTTP/HTTPS (v1)

## config schema

```yaml
secrets:
  github_token:
    docker_secret: github_token       # reads /run/secrets/github_token
  datadog_api_key:
    docker_secret: datadog_api_key
  datadog_app_key:
    docker_secret: datadog_app_key

tools:
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
        format: "Bearer {value}"

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

### secret types

v1 ships with `docker_secret` only. The config schema is designed to accommodate
additional secret types in future versions without breaking changes:

| type | how it works | unlocks |
| --- | --- | --- |
| `docker_secret` | reads `/run/secrets/<name>` | any static token or key |
| `env` | reads an env var (dev/testing only) | same as above, no file mount needed |
| `oauth2_client` (v2) | client_id + client_secret → token endpoint → cached bearer | Salesforce, HubSpot, Zoom, enterprise APIs |
| `google_service_account` (v2) | service account JSON → JWT → token exchange → cached bearer | BigQuery, GCS, Drive, Sheets, Pub/Sub, all Google Cloud |
| `aws_sigv4` (v3) | request signing, not just header injection — different primitive | all AWS services |

`oauth2_client` and `google_service_account` share the same shape from the agent's
perspective: a tool call injects an `Authorization: Bearer <token>` header. The server
handles token acquisition and caching internally. `aws_sigv4` requires a different
primitive (signing the full request) and is a separate design problem.

## agent-facing tool interface

Every registered tool has the same input schema regardless of which API it targets:

```json
{
  "path":    "string (required) — API path, e.g. /api/v2/logs/events/search",
  "method":  "string (required) — HTTP method: GET, POST, PATCH, DELETE",
  "body":    "string (optional) — JSON body",
  "headers": "object (optional) — non-secret headers, e.g. Content-Type"
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

## security model

### isolation
- `keys` runs as a dedicated non-root user (`uid 9999`) in its container
- Config file and Docker Secrets are owned by that user, mode `0400`
- The container has no shared volumes with `bench` or other containers
- `keys` sits on an internal Docker network; only the MCP broker can reach it
- No Docker socket mounted

### secrets
- All secrets are delivered via Docker Secrets (`/run/secrets/`), not env vars in production
- Secrets are read once at startup and held in memory only
- Secret values are never included in any response, log line, or error message
- Injected headers are never echoed back in the response

### compose config discipline
- Secret env vars are declared only on the `keys` service, never in shared env files
- `bench` cannot `exec` into `keys` — no Docker socket, no `pid: host`
- Process namespace isolation prevents reading `/proc/<pid>/environ` across containers

## layout

```
keys/
  main.go          server wiring, config loading, dynamic tool registration
  config.go        Config struct, YAML parsing, secret resolution
  server.go        per-tool HTTP handler, header injection, response formatting
  doc/
    design.md      this file
```

## auth machinery taxonomy

The complexity of `keys` grows by secret type, not by API. Every API that uses a given
authentication scheme gets the same implementation for free. From the agent's perspective
all secret types produce the same result: an injected header on an outbound request.

| secret type | how it works | products unlocked |
| --- | --- | --- |
| `docker_secret` / `env` | static value read from file or env | GitHub, Datadog, Stripe, OpenAI, Anthropic, ~80% of SaaS APIs |
| `oauth2_client` (v2) | client\_id + client\_secret → token endpoint → cached bearer | Salesforce, HubSpot, Zoom, Slack, most enterprise SaaS |
| `google_service_account` (v2) | service account JSON → RS256 JWT → token exchange → cached bearer with refresh | BigQuery, GCS, Drive, Sheets, Pub/Sub, Vertex AI, Cloud Logging — entire GCP |
| `aws_sigv4` (v3) | request signing (not just header injection) — different primitive | all AWS services |

v2 means the `google_service_account` type: one JWT library + one token cache + one
refresh goroutine. Ships the entire GCP ecosystem. `oauth2_client` is similar scope and
could be bundled in v2 as well.

`aws_sigv4` is architecturally different — it signs the full request, not just injects
a header — and is a separate design problem, deferred to v3 if ever.

## out of scope (v1)

- BigQuery and any Google Cloud API (requires `google_service_account` secret type — v2)
- AWS services (requires `aws_sigv4` — v3)
- Database wire protocols — Postgres, MySQL — belong in dedicated MCP servers
- Response transformation, pagination helpers, retries
- Any non-HTTP protocol

## postgres note

Postgres is a wire protocol (libpq), not HTTP. The credential model (DSN, SSL certs,
connection pooling) and the tool model (`query`, `explain`, `list_tables`) are both
fundamentally different from what `keys` does. The "secrets out of model context"
principle applies equally, but that is the only shared concern. Postgres belongs in
its own MCP server (idea [12]).
