---
id: 2026-11-13-ams-wallet
type: idea
summary: Surface the current value of the AMS trading automation wallet in the Investments MCP.
author: Thom
created: 2026-11-13
agents: claude
---

# AMS wallet

## Status

Idea only, not part of v1.

## Intent

AMS is a separate project the operator is writing: a trading automation system with
access to a crypto wallet, where the system performs trades. The operator controls its API
and it is entirely programmatic. The Investments MCP could ask AMS for the current
value of the wallet and show it alongside the other portfolios, with no manual entry.

## Why it fits

Unlike the Brazilian portfolio, the data source is known and fully automatable, and
the operator owns both sides of the interface, so the contract can be shaped to suit.

## Boundaries

- The Investments MCP only reads. Trading stays inside AMS; this server never
  places or influences trades.
- AMS remains the source of truth for the wallet. Holdings reported by AMS are not
  copied into the manual transaction ledger.

## Crypto considerations

- Holdings are crypto assets, not listed securities. The v1 market data provider,
  exchange metadata and USD-listed instrument model do not apply to them.
- Prices trade around the clock, so a daily value needs a defined cut-off time,
  unlike the market-session days used for the USD portfolio.
- Valuation likely comes from AMS itself, which already holds the wallet, rather
  than from a market data provider here.

## To be decided

- What AMS exposes: a single wallet value, or positions with quantities and prices.
- Which currency the wallet is valued in, and whether it needs FX against the other
  portfolios or is shown separately, as with Brazil.
- Whether history is needed. The server could take its own daily snapshots of the
  reported value, or ask AMS for its history.
- How the call is authenticated, and how failures or an unreachable AMS are shown
  without blocking the rest of the portfolio views.
- Whether this belongs in the same server or stays a separate integration, per the
  consolidation constraint in the hub idea.

## Related

- 2026-11-13-financial-automation-hub
- 2026-11-13-brazil-portfolio
