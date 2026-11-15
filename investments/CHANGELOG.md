# CHANGELOG

## Unreleased

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
