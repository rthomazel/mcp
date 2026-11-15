---
id: 2026-11-15-cash
type: spec
summary: Explicit cash transactions, cash snapshots, account equity and buying power for the Investments MCP.
author: Thom
created: 2026-11-15
agents: claude
---

# Cash

## Intent

Report cash and total account equity, not only securities value, by modelling cash
explicitly instead of treating it as a security. The base spec's valuation, allocation and
snapshots stay securities only; this feature adds cash beside them.

## Concepts

**Cash transactions** are account-level cash events that belong to no instrument: an
opening balance, deposits, withdrawals, interest and account fees. They are distinct from
transactions, the instrument ledger. Together with the cash impact of transactions they
make up the cash ledger.

**Cash snapshots** are daily cash balances, one record per snapshot date, so account
equity has a history that is preserved like the securities snapshots.

## Behavior

Cash is modelled explicitly and never treated as a security. Cash transactions are a
separate record of five kinds, each with a sign convention: an opening balance
(any sign, at most one), deposit and interest (positive), withdrawal and fee (negative).
Dividends, taxes and trades keep entering through the transaction ledger, and their
cash impact counts toward cash from there. Nothing is entered twice.

The cash balance on a date is the sum of the opening balance, all cash transactions
and all transaction cash amounts on or after the opening balance date and on or
before the date. Entries dated before the opening balance are not part of cash. With no opening balance, or a date
before it, cash is reported as not initialized and no balance, equity or buying power is
shown; the agent is told to record an opening balance. The current balance and the balance on any
requested date are computed from the cash ledger.

Cash snapshots preserve history. The same unit of work that writes a date's securities
snapshot also writes that date's cash snapshot, the balance computed as above, whenever cash is
initialized on that date. A date with no active positions still gets a cash snapshot, so
a portfolio that is entirely cash keeps an equity history. A rebuild replaces cash snapshots with the
securities snapshots over the same range, atomically, and the snapshot run's completion marker covers both. A
cash snapshot is never merged into the securities snapshot rows: those stay one row per instrument and
allocation stays over securities only.

Cash-like holdings such as T-bills are securities, not cash. They may be added soon, as a feature of their own: they
will be instruments with their own valuation in the securities snapshots, and cash and cash snapshots stay unchanged.
They need pricing and maturity handling that this feature does not cover.

Account equity is securities value plus cash. It is shown beside securities value and
never replaces it, and is withheld when the portfolio is incomplete or cash is not
initialized. Buying power is the cash balance, floored at zero; there is no margin.

A balance that is negative at any date is not rejected, because transactions were
recorded before cash existed. It is reported as a data-integrity error, like a
negative position, and never silently omitted.

Valuation, allocation and snapshots still represent securities only.
Dividends and withholding taxes are entered through the ledger; there is no
distributions table and no automatic dividend ingestion.

## Non-goals

- Margin and multiple accounts.
- T-bills and other cash-like instruments; they are securities and get their own feature.
- Returns; see the portfolio returns idea.

## Acceptance criteria

- Cash: with no opening balance, cash, equity and buying power are reported as not initialized. With one, the balance equals the opening balance plus cash transactions and transaction cash from its date, a trade's cash is counted once, and a negative balance is reported as an integrity error and never rejected.
- A snapshot run writes the cash snapshot with the securities snapshot when cash is initialized, including on a date with no active positions; a rebuild replaces both atomically; value history reports equity and net contributions from cash snapshots.
- Account equity equals securities value plus cash, is shown beside securities value, and is withheld when the portfolio is incomplete.

## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
