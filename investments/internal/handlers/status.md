# Functions

## Handler.HandleGetStatus(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_status. Summarizes whether the data is healthy.

Description shown to the agent: Check whether the portfolio data is healthy, as of today in the New York market timezone. Returns one verdict, healthy or needs attention, with a reason for each problem, and the facts behind it: for every held instrument its latest stored price date and whether that price is missing or stale, the latest price date overall, the latest price date covered by a completed snapshot, whether the latest price date still has no completed snapshot, and the failed or incomplete runs that no later run has resolved. It also lists events whose notification delivery was given up on within the last week, which need attention, with their id, type, when they were recorded and how many attempts failed, and apart from them, in an expired list that never changes the verdict, notification events older than a week that were never delivered. Remove either kind with remove_event once dealt with. A snapshot missing for a price date just fetched is expected until the next daily run or a rebuild_snapshots. Sold instruments are not checked. Use refresh_prices for missing or stale prices and rebuild_snapshots for missing snapshots; use list_updates for the raw run log.

1. Read through store.GetStatus.
2. Return the status, the instruments by symbol and exchange, the unresolved runs as updates, the events given up on and the expired events.

#### Errors

- **1.** return the failure result.
