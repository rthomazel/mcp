# Constants

priceVolatilitySQL = for one instrument id, with the window and the look-back as months and days and a threshold: take the latest price date in the whole store; for each price date of the instrument find the last price date on or before the date minus the window, using make_interval so months are subtracted first with the day clamped to the end of the month, then days, the same rule as duration.Before, and the move as the adjusted close change as a percent; judge the latest date's move; the baseline is the count and sample standard deviation of the moves of the dates after the latest date minus the look-back and before the latest date; return the date, the start date, the move, the baseline count, the volatility (null when zero), the variation (null without volatility) and whether the move is at or above the threshold times the volatility, or at or below minus it, by multiplication so no division rounds it, both false without volatility. No row when the instrument has no price on the latest date or none before the window start.

# Functions

## Monitors.GetPriceVolatility(ctx Context, ticker string, window duration.Duration, lookback duration.Duration, threshold domain.Decimal) (*domain.PriceVolatility, error)

1. if threshold is not positive, return domain.ErrInvalidInput.
2. Resolve the ticker with resolveTicker.
4. Select through priceVolatilitySQL with the instrument id, the window and look-back months and days, and the threshold.
5. if there is no row, return nil.
6. Return the result, with the volatility and variation only when they exist.

#### Errors

- **1.** return domain.ErrInvalidInput naming the threshold.

---

- **2.** return the error of resolveTicker, domain.ErrInstrumentNotFound for an unregistered ticker and domain.ErrInvalidInput for one on several instruments.

---

- **4.** if the query fails, return the wrapped error.

Window arithmetic is in SQL, one query per instrument, so every date of the look-back finds its own base price; a test pins that make_interval and duration.Before agree for the supported spans, including month ends and leap days. Reads are independent selects on one handle, no transaction.
