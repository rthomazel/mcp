# CHANGELOG

## Unreleased

### feat

- **(market data)** Tiingo price provider behind the provider-independent boundary, price storage with revised-day upserts, and a refresher with full-history backfill on registration (stage 4). Requires `INVESTMENTS_MCP_TIINGO_API_KEY`.
- **(foundations)** project structure, configuration, Postgres store and embedded migrations (stage 1).
- **(ledger)** instrument registry and transaction ledger with long-only validation and per-instrument write serialization (stage 2).
- **(valuation)** position reconstruction, securities valuation and allocation, with missing and stale price handling (stage 3).
