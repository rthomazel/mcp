# Constants

accountEquitySQL = select the sum of two NUMERIC parameters, the securities value and the cash balance.
valuationSQL = for each active position as of the reference date (positionsSQL), take the latest prices row with date on or before the reference date (lateral join, newest first, one row); return quantity, close, price date, days between the reference date and the price date, market value as quantity times close, the sum of market values over the rows that have a price, and each row's market value divided by that sum.

# Functions

## Store.GetPortfolio(ctx Context, date *domain.Date) (domain.Portfolio, error)

1. Derive the reference date: date, or today in Store.marketLocation.
2. Run portfolioPnLSQL for the reference date, which carries the valuation and the average-cost P&L.
3. Mark each holding unusable when it has no price row ("missing") or its price is Store.stalePriceDays or more older than the reference date ("stale"); an unusable holding has no market value, unrealized P&L or total return.
4. Select integrity errors via integritySQL for the reference date.
5. Read the cash balance for the reference date via cashBalance.
6. if no holding is unusable, set Complete, SecuritiesValueUSD to the total, each holding's allocation and the unrealized and total return figures of the P&L totals; when cash is also initialized, set AccountEquityUSD through accountEquitySQL.
7. otherwise leave Complete false and SecuritiesValueUSD, AccountEquityUSD, every Allocation and the unrealized and total return figures of the totals nil; holdings that are usable keep their price, value and P&L. Cost, realized, dividends, tax and fees are never withheld.
8. Return the portfolio with its closed positions and the cash balance, whatever its state.

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
The cost and P&L rules are in costbasis.go; closed positions are the instruments with ledger rows and no shares, and an instrument whose ledger goes negative appears only as an integrity error.
valuedHoldings runs valuationSQL alone and takes the query handle, so snapshots run it inside their transaction without the cost walk, and a snapshot treats any holding whose price date differs from the reference date as an issue.
The stale price threshold is Store.stalePriceDays, set from the configuration when the store is opened.
