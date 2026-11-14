# Constants

valuationSQL = for each active position as of the reference date (positionsSQL), take the latest prices row with date on or before the reference date (lateral join, newest first, one row); return quantity, close, price date, days between the reference date and the price date, market value as quantity times close, the sum of market values over the rows that have a price, and each row's market value divided by that sum.

# Functions

## Store.GetPortfolio(ctx Context, date *domain.Date) (domain.Portfolio, error)

1. Derive the reference date: date, or today in Store.marketLocation.
2. Run valuationSQL for the reference date.
3. Mark each holding unusable when it has no price row ("missing") or its price is Store.stalePriceDays or more older than the reference date ("stale").
4. Select integrity errors via integritySQL for the reference date.
5. if no holding is unusable, set Complete, SecuritiesValueUSD to the total, and each holding's allocation.
6. otherwise leave Complete false and SecuritiesValueUSD and every Allocation nil; holdings that are usable keep their price and value.
7. Return the portfolio.

#### Errors

- **2.** if the query fails, return the wrapped error.

---

- **4.** if the query fails, return the wrapped error.

An empty portfolio is complete with a securities value of zero and no holdings.
A stale or missing price is never replaced with an older or zero price, matching the snapshot policy. A partial total is withheld because a share of value computed over only some holdings would mislead.
Valuation always uses close, never adjusted close.
Cash is excluded: only quantity times price counts.
The value, total and allocation arithmetic is done in SQL; Go only decides what to expose. Allocations are exact to NUMERIC division precision and sum to one within that rounding.
Stage 5 reuses valuationSQL with the snapshot date as the reference date and requires the price date to equal it.
The stale price threshold is Store.stalePriceDays, set from the configuration when the store is opened.
