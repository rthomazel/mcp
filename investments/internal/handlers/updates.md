# Functions

## Handler.HandleListUpdates(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool list_updates. Reads the updates log.

Description shown to the agent: Read the log of price refreshes, daily snapshot runs and snapshot rebuilds, newest first. Each entry has its type, status, start and end time, records written, the date or date range it covers, the provider for refreshes, the reason for rebuilds and the error message when it did not complete. A status of incomplete means a run finished but could not value every holding or refresh every instrument; failed means a rebuild was refused or a refresh could not list instruments. Use it to check freshness and why a day has no snapshot, then refresh_prices or rebuild_snapshots to fix it. A completed snapshots entry marks its date as done.

- data_type string, optional, one of prices, snapshots, snapshot_rebuild
- limit int, optional, default 100, maximum 500
- status string, optional, one of completed, incomplete, failed

1. Parse the parameters.
2. List through store.ListUpdates.
3. Return the entries.

#### Errors

- **1.** return the failure result for ErrInvalidInput.

---

- **2.** return the failure result.
