---
id: 2026-11-13-keys-secrets
type: documentation
summary: Documents how keys loads, holds, injects, and protects secret values.
author: Thom
created: 2026-11-13
agents: claude
---

# secrets

## Description

Secrets are read once at startup from files in the secrets directory (`/run/secrets`, the Docker Secrets mount)
and held in memory for the life of the process.

## Loading

- Each secret reads `<secrets dir>/<docker_secret>`.
- Leading and trailing whitespace is trimmed.
- An unreadable file or an empty trimmed value is fatal at startup.
- Startup logs secret names, never values.

## Use

For each tool call, every entry in the tool's `inject` map becomes a request header whose value is the secret,
optionally wrapped by `format`. Injected headers always win over agent-supplied headers of the same name.

## Protection of secret values

- Agent-supplied headers cannot override injected ones (see [http-tool](../http-tool/doc.md)).
- Redirects to a different origin are never followed, so injected headers cannot be sent to a foreign host.
- The response body is replaced with a redaction notice if it contains any secret value.
- A response header whose value contains a secret value is returned as `[redacted]`.
- Redaction is exact-substring matching only; encoded or transformed echoes are not detected.
- Errors and logs do not include secret values.

## Limits

Secrets protect credentials from the model, not what the model does with the access it is granted.

## Refs

- doc: 2026-11-13-keys-http-tool
- doc: 2026-11-13-keys-config
