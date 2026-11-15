---
id: 2026-11-15-portfolio-returns
type: idea
summary: Follow-up idea for money-weighted and time-weighted portfolio returns built on cash transactions and account equity snapshots, the latter for benchmark comparison.
author: Thom
created: 2026-11-15
agents: claude
---

# Money-weighted and time-weighted returns

## Status

Idea only, not part of the cash and P&L round. The Investments MCP spec keeps
portfolio-level returns as a non-goal; this captures the intent. Everything below is to be decided.

## Idea

Report the portfolio's return over a period as a money-weighted return: the single annualized rate
that makes the dated external cash flows and the ending account equity equal the starting equity.
It answers "how did my money do" while accounting for when deposits and withdrawals happened,
which the value history deliberately does not.

Also report a time-weighted return, which removes the effect of deposit and withdrawal timing
and so measures the investments themselves. It is the one to compare with a benchmark, such as an
index fund over the same period. Money-weighted answers "how did I do", time-weighted answers "how did
the portfolio do".

## Time-weighted specifics

- Method: chain the returns of the sub-periods between external flows, each measured on account equity
  with the flow taken out. Daily equity snapshots make daily sub-periods possible, so no extra valuation
  is needed; a flow on a non-trading day is applied at the next snapshot date.
- Benchmark: needs benchmark prices over the same dates. An index ETF registered as an instrument would
  already be priced by the existing provider, but the base spec places benchmarks with the separate
  market-trends workflow, so ownership has to be settled.
- The same gaps apply: securities moved in without cash, snapshot gaps, and the start of history.
- Result reported as a period return, and annualized only for periods longer than a year.

## What the cash design already provides

- External flows: deposits and withdrawals, and the opening balance as the first inflow, are
  recorded as cash transactions with effective dates.
- Equity at any date: daily cash snapshots plus securities snapshots give account equity at the
  start and end of a period, preserved against later corrections.
- Internal flows are already separated: dividends, interest, taxes, fees and trades stay inside
  equity and are not external flows.

So the inputs exist once cash is initialized. No schema change is needed for a basic version.

## Gaps to resolve

- **Securities moved in or out without cash.** An adjustment that adds shares at zero cost, such as a
  transfer in, is an external flow of value the cash ledger does not see, so it would count as free gain.
  A way to value or flag such events is needed.
- **Start of history.** Equity exists only from the opening balance and the earliest snapshots, so the
  period cannot reach back further, and positions held before the opening balance are only valued at its date.
- **Snapshot gaps.** Equity exists only on snapshot dates and is withheld on incomplete ones; a period
  must start and end on dates that have both snapshots.
- **Computation.** The rate comes from an iterative root find, not a closed form. The project does
  decimal arithmetic in SQL only, so the method has to be settled: a SQL implementation, or an
  accepted floating point exception for a rate that is reported rounded.
- **No meaningful answer.** Zero or negative starting equity, or flows that never change sign,
  give no rate or several; those cases must be reported as not computable.
- **T-bills.** Cash-like holdings would sit in securities value and be priced as instruments, which
  keeps equity correct, but they need their own pricing first.

## To be decided

- Period choices: since inception, year to date, trailing windows, or arbitrary dates.
- Annualized rate only, or also the plain period return for short periods.
- Whether a simpler approximation, such as a Modified Dietz return, is enough for a personal system.
- Which benchmarks, and whether benchmark data lives in this server or in the market-trends workflow.
- Whether both returns ship together or time-weighted first, since it needs no iterative solve.
- How this relates to the Brazil portfolio idea, which raises the same performance questions with currency.
