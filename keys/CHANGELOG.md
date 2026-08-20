# CHANGELOG

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
    - scope is the file, package, or area changed e.g. (config), (proxy), (secrets).
    - Em dash (—) separates the short label from the explanation.
-->

## [Unreleased](https://github.com/rthomazel/mcp/pull/43) fix: follow same-origin redirects

### fix

- **(proxy)** follow same-origin redirects automatically (up to 5 hops) — previously every redirect (`CheckRedirect` returning `http.ErrUseLastResponse`) was returned to the agent as-is, so a trailing-slash 301 like vast.ai's `/bundles` → `/bundles/` required the agent to reissue the call itself, without credentials, defeating the point of the proxy. Cross-origin redirects are still never followed, since Go's client copies injected secret headers onto any request it follows and a cross-host hop would leak credentials outside the tool's configured `base_url`.

## [0.1.1](https://github.com/rthomazel/mcp/pull/34) build: weekly Go dependency update

### build

- [`dfbaf99`](https://github.com/rthomazel/mcp/commit/dfbaf99) **(go.mod)** `github.com/mark3labs/mcp-go` v0.54.1 -> v0.57.0 — no source changes required. `CallToolParams.Arguments` widened from `map[string]any` to `any` in this range, but `main.go` already reads it via a type assertion rather than direct indexing.
- [`dfbaf99`](https://github.com/rthomazel/mcp/commit/dfbaf99) **(go.mod)** `go.yaml.in/yaml/v3` v3.0.4 -> v3.0.5 — patch bump, config parsing unaffected.

## [0.1.0](https://github.com/rthomazel/mcp/pull/23) feat: keys server

### feat

- [`0dd4558`](https://github.com/rthomazel/mcp/commit/0dd4558) **(keys)** new `keys` MCP server — config-driven HTTP proxy that holds API credentials and injects them at call time. Secret values are loaded from Docker Secrets and never surfaced to the model.
- [`0dd4558`](https://github.com/rthomazel/mcp/commit/0dd4558) **(config)** YAML config — `mcp_tools:` block defines tools with `name`, `description`, `base_url`, `auth`, and optional `method`/`headers` overrides; `secrets:` block maps logical secret names to Docker Secret file paths.
- [`0dd4558`](https://github.com/rthomazel/mcp/commit/0dd4558) **(proxy)** HTTP proxy — validates and joins paths, strips hop-by-hop headers, injects credentials from secrets, redacts secret values from response bodies, enforces request/response size limits. Redirects disabled.
