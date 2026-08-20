# CHANGELOG

## [0.1.2](https://github.com/rthomazel/mcp/pull/44) fix: pin mcp SDK below 2.0

### fix

- **(Dockerfile)** pin `"mcp<2.0.0"` alongside `mcpo`/`mcp-proxy` in the `pip install` line — mirrors the fix already applied to `bench/Dockerfile` in #39, which this Dockerfile was missed by. `mcp` 2.0.0 (published 2026-07-28) removed/renamed lowlevel server internals (`request_ctx`) that `mcp-proxy` still imports directly, so any fresh image build without the pin crashes on startup under `mcp-proxy` transport with `ImportError: cannot import name 'request_ctx' from 'mcp.server.lowlevel.server'`.

## [0.1.1](https://github.com/rthomazel/mcp/pull/33) build: weekly Go dependency update

### build

- [`9a364d2`](https://github.com/rthomazel/mcp/commit/9a364d2) **(go.mod)** `github.com/mark3labs/mcp-go` v0.54.1 -> v0.57.0 — no source changes required. `CallToolParams.Arguments` widened from `map[string]any` to `any` in this range, but `handlers/transaction.go` already reads it via `req.GetArguments()`.
- [`9a364d2`](https://github.com/rthomazel/mcp/commit/9a364d2) **(go.mod)** `github.com/jackc/pgx/v5` v5.9.2 -> v5.10.0 — minor bump of the Postgres driver, no query or connection API changes on our call sites.

## [0.1.0](https://github.com/rthomazel/mcp/pull/24) feat: initial release

### feat

- **(server)** MCP server exposing PostgreSQL to AI agents via stdio, mcpo, or mcp-proxy
- **(introspection)** `list_schemas`, `list_tables`, `describe_table`, `list_indexes`, `list_foreign_keys`, `list_views`, `list_functions`, `table_stats`, `database_size`, `search_schema`, `er_diagram`
- **(query)** `query` — DQL (SELECT, SHOW, TABLE, WITH) in a `BEGIN READ ONLY` transaction
- **(mutate)** `mutate`, `mutate_schema`, `mutate_permissions` — DML/DDL/DCL gated by config flags
- **(transactions)** `mutate_batch` — multi-statement atomic transaction; `dry_run` — executes then unconditionally rolls back
- **(diagnostics)** `ping`, `explain`, `explain_analyze`, `active_connections`, `active_locks`
- **(sqlcheck)** comment stripping, multi-statement rejection, tx-control keyword block, trailing semicolon normalisation
- **(config)** env-var configuration (`POSTGRES_MCP_DSN`, `POSTGRES_MCP_ALLOW_*`, etc.); sensible defaults for all optional fields
- **(container)** multi-arch Docker image (`linux/amd64`, `linux/arm64`), Debian trixie-slim runtime, `postgresmcphttp` entrypoint supporting stdio / mcpo / mcp-proxy transports
- **(ci)** `postgres-pr` and `postgres-release` GitHub Actions workflows

<!--
  FORMAT GUIDE (for agents and humans)

  Entry heading:
    ## [version](PR URL) type: brief title
    - PR URL: run `git log --oneline` and look for "Merge pull request #N" or "(#N)" in the
      merge commit message, then use https://github.com/rthomazel/mcp/pull/N
    - type follows conventional commits:
      build | ci | docs | feat | fix | misc | perf | refactor | revert | style | test

  Section headings (only include sections that have entries):
    ### build | ci | docs | feat | fix | misc | perf | refactor | revert | style | test

  Bullets:
    - [`shortHash`](https://github.com/rthomazel/mcp/commit/shortHash) **(scope)** short label — longer description.
    - scope is the file, package, or area changed e.g. (config), (handlers/query), (sqlcheck).
    - Em dash (—) separates the short label from the explanation.
-->
