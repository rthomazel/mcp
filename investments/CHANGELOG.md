# CHANGELOG

## Unreleased

### feat

- **(mcp)** eleven agent tools: add and list instruments, add and list transactions, positions, portfolio valuation, stored prices, price refresh, snapshots, snapshot rebuild and the updates log (stage 6). The refresh and the daily job now cover only held instruments, and opening a position fetches its prices right away.
- **(snapshots)** daily job inside the server (06:00 and 20:30 Eastern) that refreshes prices and writes idempotent, all-or-nothing daily snapshots, the updates log, and atomic snapshot rebuilds (stage 5). No agent tools yet.
- **(market data)** Tiingo price provider behind the provider-independent boundary, price storage with revised-day upserts, and a refresher with full-history backfill on registration (stage 4). Requires `INVESTMENTS_MCP_TIINGO_API_KEY`.
- **(foundations)** project structure, configuration, Postgres store and embedded migrations (stage 1).
- **(ledger)** instrument registry and transaction ledger with long-only validation and per-instrument write serialization (stage 2).
- **(valuation)** position reconstruction, securities valuation and allocation, with missing and stale price handling (stage 3).
