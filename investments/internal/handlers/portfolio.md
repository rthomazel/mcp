# Functions

## Handler.HandleGetPositions(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_positions. Reconstructs active positions from the ledger.

Description shown to the agent: Reconstructs active positions, the instruments with a positive quantity, from the ledger, for today or as of a past effective date. Any ledger integrity errors, instruments whose running quantity goes negative, are always returned alongside and should be reported to the user. Errors returned: invalid input (malformed date); timed out.

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

Description shown to the agent: Values the active positions at real closing prices for today, in the New York market timezone, or a past date. The valuation covers securities only and excludes cash. When any holding has no price or a price five or more days old, the portfolio is marked incomplete: no total or allocation is returned, and the affected holdings are listed so the user can refresh prices. Errors returned: invalid input (malformed date); timed out.

- date string, optional YYYY-MM-DD; today in the market timezone when absent

1. Parse date.
2. Read through store.GetPortfolio.
3. Return the portfolio. An incomplete portfolio carries no total or allocations and lists the holdings with a missing or stale price.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
