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

Tool refresh_prices. Fetches new prices from the provider now, from each instrument's latest stored date inclusive.

Description shown to the agent: Fetch new daily prices from the provider now, for one instrument or all of them, starting at each instrument's latest stored date inclusive so revised days are corrected. Returns one result per instrument with symbol, exchange, bars stored and an error, empty on success; one instrument failing does not stop the others. When the provider rate limits, the run stops and the remaining instruments are reported as not refreshed; run it again later. The provider allows few requests per hour, so do not repeat it needlessly. Writes no snapshot.

- exchange string, optional when the symbol is unique
- symbol string, optional; every instrument is refreshed when absent

1. Derive a long-timeout context.
2. if symbol is given, resolve the instrument and run refresher.RefreshInstrument for it.
3. otherwise run refresher.RefreshAll.
4. Return one result per instrument: symbol, exchange, the date fetched from, the number of bars stored and the error text, empty on success.

#### Errors

- **2.** if resolving fails, return the failure result.

---

- **3.** if listing fails or the context ended, return the failure result and any results gathered.

A per-instrument error, including rate limiting, is part of a successful tool result, so one failure never hides the other instruments' outcomes. A rate limit stops the run and marks the rest, as in the refresher. The call only refreshes prices: it writes no snapshot and no updates log row.
