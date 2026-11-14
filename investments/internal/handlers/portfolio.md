# Functions

## Handler.HandleGetPositions(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_positions. Reconstructs active positions from the ledger.

Description shown to the agent: Reconstruct active positions, instruments with a positive quantity, from the ledger as of an effective date, or the whole ledger when no date is given. Returns the positions plus any integrity errors, instruments whose running quantity goes negative; report those to the user, they are never omitted.

- date string, optional YYYY-MM-DD; positions as of that effective date, the whole ledger when absent

1. Parse date.
2. Read through store.GetPositions.
3. Return the positions and the integrity errors, never omitting the latter.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.

## Handler.HandleGetPortfolio(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_portfolio. Values active positions at real closing prices. Results name the valuation as securities, excluding cash.

Description shown to the agent: Value active positions at real closing prices for a date, default today in the New York market timezone. Securities only, cash excluded. Returns per-holding quantity, close, price date and market value, plus the total and each allocation. A holding with no price, or a price five or more days older than the date, makes the portfolio incomplete: the total and allocations are withheld and the affected holdings are flagged missing or stale. Fix that with refresh_prices.

- date string, optional YYYY-MM-DD; today in the market timezone when absent

1. Parse date.
2. Read through store.GetPortfolio.
3. Return the portfolio. An incomplete portfolio carries no total or allocations and lists the holdings with a missing or stale price.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
