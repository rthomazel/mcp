# Functions

## Handler.HandleGetPortfolioPnL(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_portfolio_pnl. Reports average-cost profit and loss for every position in the portfolio. Results say a total return is a position total, not a time-weighted or money-weighted return.

Description shown to the agent: Report profit and loss for every position as of a date, default today in the New York market timezone, under average cost. Each position has quantity, total cost, average cost, close, price date, market value, unrealized P&L in dollars and as a percent of cost, and cumulative realized P&L, dividends received, tax withheld and trade fees paid, each separate, plus a total return that sums unrealized and realized P&L plus dividends minus tax. Fees are already inside cost and sale proceeds, so they are shown for information and never subtracted again. Fully sold positions stay in the list with their realized P&L and dividends. A position with no price, or a price five or more days older than the date, is flagged missing or stale and withholds its market value and unrealized P&L, and the totals that need them; fix that with refresh_prices. Totals add up every position. Total return is a position total, not a time-weighted or money-weighted return, and says nothing about the timing of deposits. Integrity errors list instruments whose ledger goes negative; report those to the user, they are left out of the positions and never omitted from the response.

- date string, optional YYYY-MM-DD; today in the market timezone when absent

1. Parse date.
2. Read through store.GetPnL and list the instruments through store.ListInstruments for their names and types.
3. Return the positions, each with its name and type, the totals and the integrity errors.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
