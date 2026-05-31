# CHANGELOG

## [0.1.0](https://github.com/rthomazel/mcp/pull/24) feat: initial release

### feat

- **(server)** MCP server exposing PostgreSQL to AI agents via stdio, mcpo, or mcp-proxy
- **(introspection)** `list_schemas`, `list_tables`, `describe_table`, `list_indexes`, `list_foreign_keys`, `list_views`, `list_functions`, `table_stats`, `database_size`, `search_schema`, `er_diagram`
- **(query)** `query` — DQL (SELECT, SHOW, TABLE, WITH) in a `BEGIN READ ONLY` transaction
- **(mutate)** `mutate`, `mutate_schema`, `mutate_permissions` — DML/DDL/DCL gated by config flags
- **(transactions)** `mutate_batch` — multi-statement atomic transaction; `dry_run` — executes then unconditionally rolls back
- **(diagnostics)** `ping`, `explain`, `explain_analyze`, `active_connections`, `active_locks`
- **(sqlcheck)** comment stripping, multi-statement rejection, tx-control keyword block, trailing semicolon normalisation
- **(config)** YAML config file (`postgres-mcp.yaml`) with sensible defaults; `POSTGRES_MCP_CONFIG` env var for custom path
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
