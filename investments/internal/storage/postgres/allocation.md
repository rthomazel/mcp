# Constants

resolveTickersSQL = select the symbol and id of every instrument whose symbol is in a list, ordered by symbol then id.
allocationMembersCTE = two relations from parameters: the bands, one row per group with its name and optional bounds, an empty text meaning none, and the members, the pairs of a group name and an instrument id.
allocationDriftSQL = over dailySeriesSQL and the bands, judge each group on every date: its value is the sum of its members' snapshot market values on the date, its total the securities value of the date; it is above when total is positive, a maximum exists and value times 100 is greater than the maximum times total, below when total is positive, a minimum exists and value times 100 is less than the minimum times total, otherwise inside, by cross multiplication so no division rounds it; for the latest date return each group's name, allocation percent, side, whether securities are held, and for a group with a side its stay start, the day after the most recent date whose side differs from it, else the first date of the series.
allocationMemberSharesSQL = over the same parameters, each registered member's share of the securities value on the latest date, zero when it has no snapshot row, ordered by group and symbol.

# Types

## allocationArgs

1. names []string, the groups in input order
2. mins []string, the minimums as text, empty for none
3. maxs []string, the maximums as text, empty for none
4. memberGroups []string, the group of each registered member
5. memberIDs []int32, the instrument id of each registered member

The parameters of the allocation queries, built once and shared by both.

# Functions

## Monitors.GetAllocationDrift(ctx Context, groups []domain.AllocationGroup) (*domain.AllocationDrift, error)

1. Build the arguments with allocationArgs, resolving every ticker.
2. Select the groups' state on the latest date through allocationDriftSQL.
3. if there are no rows, or the latest date holds no securities, return nil.
4. Add each group's member shares through addMemberShares.
5. Return the groups in the order of the input, with the unregistered tickers.

#### Errors

- **1.** if resolving fails, return the wrapped error; a ticker on several instruments is domain.ErrInvalidInput.

---

- **2.** if the query fails, return the wrapped error.

---

- **4.** if the query fails, return the wrapped error.

The series is dailySeriesSQL's, so a cash-only date has securities zero and is inside every band, ending a stay, as in the value history.

## Store.allocationArgs(ctx Context, groups []domain.AllocationGroup) (allocationArgs, []string, error)

1. Resolve all tickers of all groups with resolveTickers.
2. For each group, in order, add its name and bounds and one member row for each ticker that resolved.
3. Collect the tickers that did not resolve, each once, in the order met.
4. Return the arguments and the unregistered tickers.

#### Errors

- **1.** return the error of resolveTickers.

## Store.resolveTickers(ctx Context, tickers []string) (map[string]int, error)

Select through resolveTickersSQL and map each symbol to its instrument id. A symbol on more than one instrument, which can happen on different exchanges, returns domain.ErrInvalidInput naming the ticker, so the operator resolves the ambiguity. Shared with GetPriceVolatility.

## Store.addMemberShares(ctx Context, args allocationArgs, byName map[string]*domain.GroupAllocation) error

Select through allocationMemberSharesSQL and append each row to its group's Members. A row for a group that is not in byName is an error.

## boundText(bound *domain.Decimal) string

Return the bound as text, empty for nil.
