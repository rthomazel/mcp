# Functions

## Handler.HandleGetSnapshots(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_snapshots. Reads stored daily valuation snapshot rows, newest date first.

Description shown to the agent: Read stored daily valuation snapshots, newest date first, grouped by date with each date's securities total and each holding's quantity, close, price date, market value and allocation. A date has snapshots only if every active holding had a price on it, and stored snapshots never change when holdings or prices change later. Use this for history and trends; get_portfolio gives the current value.

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

Description shown to the agent: Replace the stored snapshots for every price date in the range with values recomputed from the current ledger and prices. Use only after a correction that touches history, such as a backdated transaction or revised prices, starting at the rebuild_from date add_transaction returned. All or nothing: if any date in the range has an active holding without a price on it, the error names the first such date and the instruments, nothing changes, and the range must be narrowed or prices refreshed. Also fills price dates in the range that have no snapshot yet, such as a missed day. Returns the range, dates rebuilt and rows written. The rebuild is logged with the reason.

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
