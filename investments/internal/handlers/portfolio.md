# Functions

## Handler.HandleGetPositions(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_positions. Reconstructs active positions from the ledger.

Description shown to the agent: Reconstruct active positions, instruments with a positive quantity, from the ledger as of an effective date, or the whole ledger when no date is given. Returns the positions, each with its name and type, plus any integrity errors, instruments whose running quantity goes negative; report those to the user, they are never omitted. Quantities only, no prices or values: use get_portfolio for what the holdings are worth.

- date string, optional YYYY-MM-DD; positions as of that effective date, the whole ledger when absent

1. Parse date.
2. Read through store.GetPositions and list the instruments through store.ListInstruments for their names and types.
3. Return the positions, each with its name and type, and the integrity errors, never omitting the latter.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.

## Handler.HandleGetPortfolio(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_portfolio. Values active positions at real closing prices and reports their average-cost P&L. Results name the valuation as securities, excluding cash, and carry the cash balance and account equity beside it.

Description shown to the agent: Value active positions at real closing prices for a date, default today in the New York market timezone. The securities value excludes cash. Returns per-holding name, type, quantity, close, price date and market value, plus the total and each allocation, which are over securities only. Each holding also carries total cost, average cost, unrealized P&L in dollars and as a percent of cost, and cumulative realized P&L, dividends received, tax withheld and trade fees paid, each separate, plus a total return summing unrealized and realized P&L plus dividends minus tax. Fees are already inside cost and sale proceeds, so they are shown for information and never subtracted again. Fully sold positions are listed separately with their realized P&L and dividends. P&L totals add up every position, open and closed. Total return is a position total, not a time-weighted or money-weighted return. Beside them it returns the cash balance and buying power, and account equity, securities value plus cash; cash is null when no opening balance exists yet, and equity is withheld when the portfolio is incomplete or cash is not initialized. A holding with no price, or a price five or more days older than the date, makes the portfolio incomplete: the total, allocations, market value and unrealized P&L figures that need a price are withheld and the affected holdings are flagged missing or stale. Fix that with refresh_prices. Use get_positions when only quantities are needed, or to see holdings on a past date regardless of prices.

- date string, optional YYYY-MM-DD; today in the market timezone when absent

1. Parse date.
2. Read through store.GetPortfolio and list the instruments through store.ListInstruments for their names and types.
3. Return the portfolio, each holding with its name and type and its P&L, the closed positions with their names, the P&L totals, and the cash balance with account equity. An incomplete portfolio carries no total, allocations, equity, unrealized P&L or total return and lists the holdings with a missing or stale price.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
