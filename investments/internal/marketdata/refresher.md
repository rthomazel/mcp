# Types

## Refresher

1. store domain.Store
2. provider domain.PriceProvider

Orchestrates the provider and the store. It holds no state of its own.

## RefreshResult

1. Instrument domain.Instrument
2. From *domain.Date, the date the fetch started from, nil for a full history
3. Bars int, the number of bars stored
4. Err error, nil on success

# Functions

## NewRefresher(store domain.Store, provider domain.PriceProvider) *Refresher

Build a Refresher over the store and the provider.

## Refresher.RegisterInstrument(ctx Context, draft domain.Instrument) (domain.Instrument, error)

1. Normalize and validate draft.
2. Fetch the full available history through provider.FetchPrices with a nil from.
3. Register the instrument through store.RegisterInstrument.
4. Store the bars through store.UpsertPrices.
5. Return the instrument.

#### Errors

- **1.** if validation fails, return the domain.ErrInvalidInput error.

---

- **2.** if the provider does not cover the symbol, return domain.ErrInvalidInput wrapping domain.ErrSymbolNotFound and naming the symbol; nothing is registered.
- **2.1.** if the fetch fails otherwise, including rate limiting, return the wrapped error; nothing is registered.

---

- **3.** if registering fails, return the store's error unchanged, for example domain.ErrInstrumentExists.

---

- **4.** if storing fails, return the registered instrument together with an error naming the failed backfill. The instrument stays registered and the next refresh fetches its full history.

The history is fetched before registering so a mistyped symbol never creates an instrument that can never be priced. The provider looks securities up by symbol alone, so no instrument identity is needed for the fetch. A duplicate symbol and exchange costs one wasted provider request, which is acceptable.
Only the first two steps depend on the provider; the stage 6 registration capability calls this method.

## Refresher.RefreshInstrument(ctx Context, instrument domain.Instrument, from *domain.Date) RefreshResult

1. Read the latest stored price date through store.LatestPriceDate.
2. Choose the start: the earlier of from and the latest stored date, or the latest stored date when from is nil, or the full history when the instrument has no prices. Fetch bars through provider.FetchPrices from that date inclusive.
3. Store them through store.UpsertPrices.
4. Return the result with the starting date and the number of bars.

#### Errors

- **1.** if reading fails, return a result whose Err wraps the error and names the symbol.

---

- **2.** if fetching fails, return a result whose Err wraps the error and names the symbol, with zero bars.

---

- **3.** if storing fails, return a result whose Err wraps the error and names the symbol, with zero bars.

The fetch starts at the latest stored date, not the day after, because the provider revises a day's figures after first publishing it; the upsert replaces the stored record. An instrument with no prices therefore gets its full history, which is also how a failed backfill heals.
By default a refresh does not reach back past the latest stored date. A caller passes from to re-fetch older history, which is how older prices are deliberately corrected; the start is never later than the latest stored date, so a gap can never open. Snapshots already written are not recalculated by a correction; the caller rebuilds them.

## Refresher.RefreshAll(ctx Context, from *domain.Date) ([]RefreshResult, error)

1. List the instruments through store.ListInstruments.
2. For each instrument in order, run RefreshInstrument with from and collect its result.
3. Return the results and nil.

#### Errors

- **1.** if listing fails, return nil and the wrapped error.

---

- **2.** if a result's error is domain.ErrRateLimited, stop and add a result with that error for every remaining instrument, so the caller sees which were not refreshed.
- **2.1.** if ctx is cancelled, stop and return the results so far with the context error.

One instrument failing for any other reason does not stop the others; the caller inspects each result. Recording the outcome in the updates log belongs to the snapshot stage.
