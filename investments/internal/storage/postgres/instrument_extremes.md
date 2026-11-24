# Constants

instrumentDrawdownSQL = for $3, an instrument id: a series daily(date, securities) of the instrument's adjusted closes over every stored price, empty unless the instrument has a price on the latest price date of the store, followed by drawdownStateBody with $1 the increment and $2 the most levels.
instrumentHighSQL = for $1, an instrument id: the same series followed by highStateBody.
heldTickersSQL = the distinct symbols of the instruments with a snapshot on the latest snapshot date, ordered.
priceTargetSQL = for $1 an instrument id and $2 a price: the instrument's row on the latest price date of the store, with its date, low, high and close, and whether low is at or below $2 and $2 at or below high, compared as numbers so nothing is rounded. No row without a price on that date.

# Functions

## Monitors.GetInstrumentDrawdownState(ctx Context, ticker string, increment domain.Decimal) (domain.DrawdownState, error)

1. if increment is not positive, return domain.ErrInvalidInput.
2. Resolve the ticker with resolveTicker.
3. Read the state with Store.drawdownState over instrumentDrawdownSQL, the increment, maxDrawdownLevels and the id.
4. Return it.

#### Errors

- **1.** return domain.ErrInvalidInput naming the increment.

---

- **2.** return the error of resolveTicker.

---

- **3.** return the wrapped error.

The series is adjusted closes, so a split or a dividend is never a fall, and it starts at the instrument's first stored price, so the high is the all-time one. The state's value fields hold closes.

## Monitors.GetInstrumentHighState(ctx Context, ticker string) (*domain.HighState, error)

1. Resolve the ticker with resolveTicker.
2. Read the state with Store.highState over instrumentHighSQL and the id.
3. Return it.

#### Errors

- **1.** return the error of resolveTicker.

---

- **2.** return the wrapped error.

## Monitors.ListHeldTickers(ctx Context) ([]string, error)

1. Select the symbols via heldTickersSQL.
2. Return them, empty when there are none.

#### Errors

- **1.** if the query fails, return the wrapped error.

## Monitors.GetPriceTarget(ctx Context, ticker string, price domain.Decimal) (*domain.PriceTarget, error)

1. if price is not positive, return domain.ErrInvalidInput.
2. Resolve the ticker with resolveTicker.
3. Select the row via priceTargetSQL with the id and the price.
4. if there is no row, return nil.
5. Return the range and whether it holds the price.

#### Errors

- **1.** return domain.ErrInvalidInput naming the price.

---

- **2.** return the error of resolveTicker.

---

- **3.** if the query fails, return the wrapped error.

Reads are independent selects on one handle, no transaction.
