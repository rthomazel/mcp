# CHANGELOG

## [0.5.0](https://github.com/rthomazel/mcp/releases/tag/investments/v0.5.0) feat: eight new monitors, durations and configurable run times

### feat

- **(monitors)** `portfolio.ath` announces a new all-time high of the securities value, at most once per `interval` (15 days by default), as `notification.portfolio.ath`.
- **(monitors)** `portfolio.movement` announces a move of the securities value over a `window` (1 day by default) that prices alone explain, apart from trades, when it reaches `variation_pct`, in `direction` up, down or both, as `notification.portfolio.movement`. The message names the largest gainers and losers.
- **(monitors)** `allocation.drift` announces each time a group of instruments (`groups` with `instruments`, `min_pct`, `max_pct`) leaves its allocation band, once per stay outside it, as `notification.allocation.drift`.
- **(monitors)** `price.volatility` announces an instrument's price move over a `window` that is at least `variation_std_dev` standard deviations of its usual moves over a `lookback` (3 months by default), spaced by `interval`, as `notification.price.volatility`. Adjusted closes, and at least 20 moves in the look-back.
- **(monitors)** `price.target` announces a target `price` of an `instrument` when the latest daily unadjusted low to high range holds it, once, and again after a year if the price is in range then, as `notification.price.target`.
- **(monitors)** `instrument.drawdown` and `instrument.ath` do for an instrument's adjusted closes what `portfolio.drawdown` and `portfolio.ath` do for the securities value, as `notification.instrument.drawdown` and `notification.instrument.ath`. `instruments` is a list of tickers or `held`, every instrument held on the latest snapshot date.
- **(config)** durations in monitor settings are written with a unit, `h`, `d`, `w`, `mo` or `y`, for example `15d` or `1mo`. A bare number is rejected, and so is a span shorter than a day. Settings may also be lists and mappings.
- **(config)** the times of the daily jobs are server configuration: `INVESTMENTS_MCP_PRICE_REFRESH_RUN_TIMES` (a comma separated list, `06:00,20:30` by default) and `INVESTMENTS_MCP_MONITORS_RUN_TIME` (one time shared by every monitor, `07:00` by default), in the market timezone. The defaults are the previous fixed times.
- **(docs)** sample routing for every event type in the notifications spec and a sample monitors file with every monitor in the monitors configuration spec.

### fix

- **(notifications)** percentages in messages are rounded to one decimal and dollar amounts to cents in the renderers, never before, so the stored event keeps the exact figure. The portfolio drawdown message previously printed the raw percentage and shortens its note.
- **(notifications)** the portfolio drawdown message rounds its two securities values to the whole dollar.

### misc

- **(deps)** update `go.yaml.in/yaml/v3` to v3.0.6 and drop the unused `github.com/stretchr/objx` requirement.
- **(store)** the read models the monitors use move out of the generic store into a `MonitorStore`. Mocks are generated, so run `./run mocks` after pulling.

## [0.4.0](https://github.com/rthomazel/mcp/releases/tag/investments/v0.4.0) feat: monitors configuration and portfolio drawdown monitor

### feat

- **(monitors)** a separate monitors YAML file (`INVESTMENTS_MCP_MONITORS_CONFIG_FILE`) turns each monitor on or off with `disabled` and carries its settings; a monitor with no entry does not run. **Upgrade note:** the snapshot-incomplete monitor is now `snapshot.incomplete` and stops announcing until the file has an entry for it. The notifications and monitors files are loaded by a shared file loader and validated independently.
- **(monitors)** `portfolio.drawdown` announces, once per stay, each multiple of `increment_pct` the securities value falls below its all-time high, as `notification.portfolio.drawdown` with the peak and current values. Recovering above a level and falling back announces it again. Changes in value, not returns.

## [0.3.0](https://github.com/rthomazel/mcp/releases/tag/investments/v0.3.0) feat: notification outbox, monitors and Telegram channel

### feat

- **(notifications)** outbox of events with per-handler retries, exponential backoff and give-up after 10 failures; notifications routed through a versioned YAML file (`INVESTMENTS_MCP_NOTIFICATIONS_CONFIG_FILE`) to Telegram or a stub channel, by category. A 07:00 New York monitor announces a latest price date without a completed snapshot once per date. `get_status` lists events given up on and expired ones; `remove_event` clears them. Adds the `events` table and promotes `go.yaml.in/yaml/v3` to a direct dependency.

### build

- **(container)** add OCI image labels (`org.opencontainers.image.title`, `source`, `version`, `revision`) to the runtime image. The release workflow passes `VERSION` and `REVISION` (commit SHA) as build args, so `docker image inspect` shows which release is running; local builds report `local` / `unknown`.

## [0.2.1](https://github.com/rthomazel/mcp/releases/tag/investments/v0.2.1) build: tidy go.mod; remove local lint

### build

- [`0e9e933`](https://github.com/rthomazel/mcp/commit/0e9e933) **(go.mod)** drop the unused indirect requirement `github.com/stretchr/objx` v0.5.3 (`go mod tidy`). Already on Go 1.27.4, so the weekly update changed nothing else.

### misc

- **(dev)** remove `./run lint`; golangci-lint runs in CI only, pinned in the PR workflow and bumped by the weekly Go update.

## [0.2.0](https://github.com/rthomazel/mcp/releases/tag/investments/v0.2.0) feat: cash, position P&L, status and value history

### feat

- **(cash)** explicit cash: `add_cash_transaction`, `list_cash_transactions` and `get_cash`, daily cash snapshots written with the securities snapshot, and account equity and buying power in `get_portfolio`. Record an opening balance first; cash is reported only from its date. Migration `cash`.
- **(pnl)** average-cost position P&L in `get_portfolio`: cost, unrealized and realized P&L, dividends, tax and fees, closed positions and totals. `add_transaction` takes an optional `fee`. Migration `trade_fee`.
- **(status)** `get_status` aggregates price freshness of held instruments, snapshot coverage and unresolved failed runs into one verdict.
- **(history)** `get_value_history` returns the securities value and account equity over a date range with `window_days` spacing, all-time high, drawdown, change over the range and net contributions. Changes in value, not returns.

### fix

- **(container)** pin `mcp-proxy` to 0.15.0 and allow the `mcpinvestments` Host header in mcp-proxy (`INVESTMENTS_MCP_ALLOWED_HOST` to override), so LiteLLM health checks stop failing with "Invalid Host header".

## [0.1.0](https://github.com/rthomazel/mcp/releases/tag/investments/v0.1.0) feat: first release

### ci

- **(release)** pushing a tag `investments/vX.Y.Z` builds and publishes `ghcr.io/rthomazel/mcp/investments:X.Y.Z` and `:latest` for linux/amd64 and linux/arm64.

### feat

- **(mcp)** eleven agent tools: add and list instruments, add and list transactions, positions, portfolio valuation, stored prices, price refresh, snapshots, snapshot rebuild and the updates log (stage 6). The refresh and the daily job now cover only held instruments, and opening a position fetches its prices right away.
- **(snapshots)** daily job inside the server (06:00 and 20:30 Eastern) that refreshes prices and writes idempotent, all-or-nothing daily snapshots, the updates log, and atomic snapshot rebuilds (stage 5). No agent tools yet.
- **(market data)** Tiingo price provider behind the provider-independent boundary, price storage with revised-day upserts, and a refresher with full-history backfill on registration (stage 4). Requires `INVESTMENTS_MCP_TIINGO_API_KEY`.
- **(foundations)** project structure, configuration, Postgres store and embedded migrations (stage 1).
- **(ledger)** instrument registry and transaction ledger with long-only validation and per-instrument write serialization (stage 2).
- **(valuation)** position reconstruction, securities valuation and allocation, with missing and stale price handling (stage 3).
