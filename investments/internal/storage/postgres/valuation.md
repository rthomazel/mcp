# Constants

accountEquitySQL = select the sum of two NUMERIC parameters, the securities value and the cash balance.
valuationSQL = for each active position as of the reference date (positionsSQL), take the latest prices row with date on or before the reference date (lateral join, newest first, one row); return quantity, close, price date, days between the reference date and the price date, market value as quantity times close, the sum of market values over the rows that have a price, and each row's market value divided by that sum.

# Functions

## Store.GetPortfolio(ctx Context, date *domain.Date) (domain.Portfolio, error)

1. Derive the reference date: date, or today in Store.marketLocation.
2. Run valuationSQL for the reference date.
3. Mark each holding unusable when it has no price row ("missing") or its price is Store.stalePriceDays or more older than the reference date ("stale").
4. Select integrity errors via integritySQL for the reference date.
5. Read the cash balance for the reference date via cashBalance.
6. if no holding is unusable, set Complete, SecuritiesValueUSD to the total, and each holding's allocation; when cash is also initialized, set AccountEquityUSD through accountEquitySQL.
7. otherwise leave Complete false and SecuritiesValueUSD, AccountEquityUSD and every Allocation nil; holdings that are usable keep their price and value.
8. Return the portfolio with the cash balance, whatever its state.

#### Errors

- **2.** if the query fails, return the wrapped error.

---

- **4.** if the query fails, return the wrapped error.

---

- **5.** if reading cash fails, return the wrapped error.
- **6.1.** if the equity query fails, return the wrapped error.

An empty portfolio is complete with a securities value of zero and no holdings.
A stale or missing price is never replaced with an older or zero price, matching the snapshot policy. A partial total is withheld because a share of value computed over only some holdings would mislead.
Valuation always uses close, never adjusted close.
Cash is excluded from the securities value: only quantity times price counts. Account equity is a separate figure beside it, added in SQL, and is withheld whenever the securities value is or cash is not initialized.
The value, total and allocation arithmetic is done in SQL; Go only decides what to expose. Allocations are exact to NUMERIC division precision and sum to one within that rounding.
valuedHoldings takes the query handle, so snapshots run it inside their transaction, and a snapshot treats any holding whose price date differs from the reference date as an issue.
The stale price threshold is Store.stalePriceDays, set from the configuration when the store is opened.
