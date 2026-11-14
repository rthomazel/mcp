# Functions

## Handler.HandleGetPositions(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_positions. Reconstructs active positions from the ledger.

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

- date string, optional YYYY-MM-DD; today in the market timezone when absent

1. Parse date.
2. Read through store.GetPortfolio.
3. Return the portfolio. An incomplete portfolio carries no total or allocations and lists the holdings with a missing or stale price.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
