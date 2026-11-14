# Functions

## Handler.HandleGetSnapshots(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_snapshots. Reads stored daily valuation snapshot rows, newest date first.

Description shown to the agent: Reads the stored daily valuation snapshots, newest date first, grouped by date with each date's securities total and each holding's quantity, closing price, price date, market value and allocation. Snapshots exist only for sessions on which every active holding had a price, and are never recalculated by later changes. Use it for history and trends; use get_portfolio for the current value. Errors returned: invalid input (malformed date, negative limit, from after to); instrument not found; ambiguous symbol (give the exchange); timed out.

- exchange string, optional when the symbol is unique
- from string, optional YYYY-MM-DD, inclusive
- limit int, optional, default 100, maximum 500
- symbol string, optional, filters to one instrument
- to string, optional YYYY-MM-DD, inclusive

1. Parse the parameters and resolve the optional instrument.
2. Read through store.ListSnapshots.
3. Return the rows grouped by date, each date with its securities total and, per row, quantity, close price, price date, market value and allocation.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.

A date appears only when snapshot rows exist for it, which means it was a session with every price available. Dates are never recalculated here.

## Handler.HandleRebuildSnapshots(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool rebuild_snapshots. Replaces the snapshots of every price date in the range from the current ledger and prices, all or nothing, and logs the rebuild with its reason.

Description shown to the agent: Replaces the stored snapshots for every price date in the range with values recomputed from the current ledger and prices. Use it only after a correction that touches history, such as a backdated transaction or revised prices, with the date from record_transaction's rebuild_from as the start. The rebuild is all or nothing and is logged with the reason. Errors returned: invalid input (missing reason, malformed date, from after to, a date in the future); snapshot incomplete, naming the first date and the instruments lacking a price on it, in which case nothing changed and the range must be narrowed or the prices refreshed; timed out.

- from string, required YYYY-MM-DD, inclusive
- reason string, required, why the rebuild is needed
- to string, required YYYY-MM-DD, inclusive

1. Parse the parameters.
2. Derive a long-timeout context.
3. Rebuild through store.RebuildSnapshots.
4. Return the range, the number of dates rebuilt and the number of rows written.

#### Errors

- **1.** return the failure result.

---

- **3.** if some date could not be valued, return the failure result, whose message names the first date and the instruments lacking a price; nothing was changed.
- **3.1.** otherwise return the failure result.
