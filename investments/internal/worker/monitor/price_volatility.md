# Constants

instrumentsKey = "instruments", instrumentKey = "instrument", stdDevKey = "variation_std_dev", lookbackKey = "lookback", with windowKey, intervalKey and directionKey shared with the other monitors.
defaultLookback = "3mo", the look-back unless the window is longer.
volatilityDefaultInterval = "1d".
minVolatilityMoves = 20, the fewest baseline moves a variation is judged from.

# Types

## PriceVolatility

1. store domain.GenericStore, for the price moves
2. events domain.EventStore, where the events are recorded and the last one of an instrument is read

The monitor for an instrument's price move over a window against its usual moves.

## volatilityEntry

1. ticker string, upper case
2. threshold Decimal, positive standard deviations
3. direction direction

## volatilitySettings

1. window duration.Duration
2. interval duration.Duration
3. lookback duration.Duration, the one that applies
4. entries []volatilityEntry

# Functions

## NewPriceVolatility(store domain.GenericStore, events domain.EventStore) *PriceVolatility

Build the monitor.

## PriceVolatility.Name() string

Return domain.Events.PriceVolatility without domain.NotificationPrefix, price.volatility.

## PriceVolatility.ValidateParams(params Params) error

Return the error of volatilitySettingsFrom, or nil.

## PriceVolatility.Run(ctx Context, params Params) error

1. Read the settings with volatilitySettingsFrom.
2. Judge each entry in order with judge.
3. Return the failures joined, nil when there are none.

#### Errors

- **1.** return the error.

---

- **3.** return each failed entry's error wrapped with its ticker; one failing instrument never stops the others.

## PriceVolatility.judge(ctx Context, settings volatilitySettings, entry volatilityEntry) error

1. Read the move through store.GetPriceVolatility with the window, the look-back and the threshold.
2. if the ticker is not registered, log at warning level and return nil.
3. if there is no price to judge, log at info level and return nil.
4. if the result has fewer than minVolatilityMoves moves, or no volatility, log at info level and return nil, since a variation cannot be judged.
5. Find the side with volatilityEntry.sideReached; if none, return nil.
6. Read the date of the last event of this instrument with lastEventDate, with the ticker and an at sign as the subject prefix.
7. if withinInterval holds against the result's date, log at info level and return nil.
8. Record a price volatility event through events.Record, with the ticker, the date and the side joined by at signs as its subject and a PriceVolatilityData built from the result as its payload.
9. Return nil.

#### Errors

- **1.** if reading fails for another reason, return the wrapped error.

---

- **6.** return the error of lastEventDate.

---

- **8.** if encoding or recording fails, return the wrapped error.

A day is judged once because the date is in the subject. Each instrument has its own interval, since the prefix selects its events only.

## volatilityEntry.sideReached(result *domain.PriceVolatility) direction

Return up when the move is at least the threshold upward and the entry watches up, otherwise down in the same way, otherwise empty.

## volatilitySettingsFrom(params Params) (volatilitySettings, error)

1. Reject any setting other than window, interval, lookback and instruments with Params.Unknown.
2. Read the window and the interval with Params.DurationOr, one day each by default.
3. Read the look-back with lookbackFrom.
4. Read the instruments with volatilityEntriesFrom.

#### Errors

- **2.** return the duration error naming the setting and the text.

## lookbackFrom(params Params, window duration.Duration) (duration.Duration, error)

1. Read lookback with Params.Duration.
2. if set, reject one shorter than the window by Duration.Compare, and return it.
3. Otherwise return defaultLookback, or the window when the window is longer.

#### Errors

- **2.** return an error naming both.

## volatilityEntriesFrom(params Params) ([]volatilityEntry, error)

Read instruments with Params.Mappings, required and non-empty, build each with volatilityEntryFrom, naming its position in any error, and reject a ticker listed twice.

## volatilityEntryFrom(item Params) (volatilityEntry, error)

1. Reject any key other than instrument, variation_std_dev and direction.
2. Read the ticker with Params.Text; it is required and not empty, and is upper-cased.
3. Read the threshold with Params.Decimal; it is required and greater than zero.
4. Read the direction with directionFrom.
5. Return the entry.
