# keys

[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Frthomazel%2Fmcp%2Fkeys-blue?logo=docker)](https://ghcr.io/rthomazel/mcp/keys)

A configuration-driven MCP server that holds API credentials and exposes named, authenticated HTTP tools to agents. Secret values are never surfaced to the model — injected server-side at call time.

## How it works

You define tools in a YAML config file. Each tool maps to an external API and lists which headers to inject from Docker Secrets. At startup, `keys` reads the secrets and registers one MCP tool per configured API. The agent calls tools by name with `path`, `method`, `body`, and `headers` — no credentials required.

## Config

Copy [`doc/config-sample.yaml`](./doc/config-sample.yaml) as a starting point:

```yaml
secrets:
  github_token:
    docker_secret: github_token   # reads /run/secrets/github_token

tools:
  github:
    description: >
      Make authenticated requests to the GitHub REST and GraphQL API.
      Base URL: https://api.github.com
    docs:
      - https://docs.github.com/en/rest
    base_url: https://api.github.com
    inject:
      Authorization:
        secret: github_token
        format: "Bearer {value}"
```

See [`doc/design.md`](./doc/design.md) for the full config schema and security model.

## Running

```yaml
# compose.yml
secrets:
  github_token:
    file: ./secrets/github_token

services:
  keys:
    image: ghcr.io/rthomazel/mcp/keys:latest
    user: "9999:9999"
    volumes:
      - ./config.yaml:/etc/keys/config.yaml:ro
    secrets:
      - github_token
    networks:
      - mcp-internal
```

The default config path is `/etc/keys/config.yaml`. Override with `--config <path>`.

## Development

```bash
# start server (requires .env with KEYS_CONFIG pointing to a local config)
./run dev

# run tests
./run test

# lint
./run lint
```
