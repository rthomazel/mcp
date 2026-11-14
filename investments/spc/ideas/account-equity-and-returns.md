---
id: 2026-11-14-account-equity-and-returns
type: idea
summary: Follow-up idea to show account equity beside securities equity, and to track dividends, returns, buying power and cash-like holdings such as T-bills.
author: Thom
created: 2026-11-14
agents: claude
---

# Account equity and returns

## Status

Idea only, not part of v1. The Investments MCP spec values securities only and
excludes cash from valuation, allocation and snapshots. This captures the
follow-ups that decision leaves open. Everything below is to be decided.

## Idea

Show account equity separately from securities equity, and use the cash side of the
ledger for things that are interesting in a personal portfolio: dividends, returns,
buying power and cash-like holdings.

## Why v1 excludes cash

The ledger records only the cash impact of events the operator chose to enter:
buys, sells, dividends and taxes. It has no opening balance, deposits, withdrawals,
interest or fees. A sum of it is a net recorded cash flow, not an account cash
balance, so folding it into the securities total would look like brokerage equity
and mislead.

## Directions

- **Account equity.** Securities value plus a real cash balance, shown beside the
  securities value and never merged with it. Needs deposits, withdrawals, interest
  and fees to be recordable, and an opening balance, so the figure can be trusted.
- **Dividends.** Income per instrument and per period from the dividend and tax
  entries already in the ledger, such as trailing twelve months and yield on current
  value.
- **Returns.** Performance over a period that accounts for cash flows, which means
  a method has to be chosen, for example money-weighted or time-weighted. Adjusted
  close, already stored, supports instrument-level total return.
- **Buying power.** Cash available to deploy, once a real cash balance exists.
- **T-bills and other cash-like holdings.** Treasury bills and similar instruments
  held as a cash substitute. They do not fit the stock and ETF registry cleanly:
  they mature, are bought at a discount and may have no quoted daily close.
- **Net recorded cash flow, as a stopgap.** A clearly labelled figure on the
  portfolio that sums the ledger's cash amounts up to a date, without claiming to be
  a balance. Cheap, but easy to misread.

## To be decided

- Whether a real cash balance is worth the extra bookkeeping for a personal system.
- How cash movements outside securities trades are recorded: new event types or a
  separate record.
- Which return method to use, and whether snapshots store cash so history can be
  shown without recalculating.
- How T-bills are modelled and priced, and whether the provider covers them.
- How this relates to the Brazil portfolio idea, which raises similar cash-flow and
  performance questions.
- Whether the v1 design needs any accommodation now. Today the ledger already keeps
  cash per event, so a later cash balance should not need a redesign.

The v1 spec should not grow to cover this. If a question here affects v1 choices,
bring it back to the spec explicitly.
