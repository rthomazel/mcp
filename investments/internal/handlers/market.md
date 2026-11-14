# Functions

## Handler.HandleGetPrices(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_prices. Reads stored daily prices of one instrument, newest first.

Description shown to the agent: Read stored daily prices for one instrument, newest first, with open, high, low, close, adjusted close and volume. Valuation uses close, never adjusted close. Reads stored data only; refresh_prices fetches newer prices.

- exchange string, optional when the symbol is unique
- from string, optional YYYY-MM-DD, inclusive
- limit int, optional, default 100, maximum 500
- symbol string, required
- to string, optional YYYY-MM-DD, inclusive

1. Parse the parameters and resolve the instrument.
2. Read through store.ListPrices.
3. Return the bars: open, high, low, close, adjusted close and volume, with the close labelled as the valuation price.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.

## Handler.HandleRefreshPrices(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool refresh_prices. Fetches prices from the provider now, from each instrument's latest stored date inclusive, or from an earlier date to correct history.

Description shown to the agent: Fetch new daily prices from the provider now, for one named instrument or for every held instrument (positive quantity; sold ones are skipped), starting at each instrument's latest stored date inclusive so revised days are corrected, or at an earlier from date to re-fetch older history. Returns one result per instrument with symbol, exchange, bars stored and an error, empty on success; one instrument failing does not stop the others. When the provider rate limits, the run stops and the remaining instruments are reported as not refreshed; run it again later. The provider allows few requests per hour, so do not repeat it needlessly. Pass from only to correct older prices; the default refresh already covers the latest day. Writes no snapshot, and snapshots already stored are not recalculated when older prices change: follow a correction with rebuild_snapshots from that date.

- exchange string, optional when the symbol is unique
- from string, optional YYYY-MM-DD; re-fetch history from this date, replacing the stored prices from it onward, instead of only the latest day
- symbol string, optional; every held instrument is refreshed when absent, and a named instrument is refreshed even when it is not held

1. Parse from and derive a long-timeout context.
2. if symbol is given, resolve the instrument and run refresher.RefreshInstrument for it with from.
3. otherwise run refresher.RefreshAll with from.
4. Return one result per instrument: symbol, exchange, the date fetched from, the number of bars stored and the error text, empty on success.

#### Errors

- **1.** if from is malformed or in the future, return the failure result for ErrInvalidInput.

---

- **2.** if resolving fails, return the failure result.

---

- **3.** if listing fails or the context ended, return the failure result and any results gathered.

A per-instrument error, including rate limiting, is part of a successful tool result, so one failure never hides the other instruments' outcomes. A rate limit stops the run and marks the rest, as in the refresher. The call only refreshes prices: it writes no snapshot and no updates log row.
