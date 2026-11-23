# Constants

variationKey = "variation_pct", windowKey = "window", the settings beside directionKey and intervalKey.
movementDefaultSpan = "1d", the default window and interval.

# Types

## PortfolioMovement

1. store domain.GenericStore, for the market movement
2. events domain.EventStore, where the events are recorded and the last one is read

The monitor for the securities value moving over a window because of prices alone.

## movementSettings

1. variation Decimal, positive
2. direction direction
3. window duration.Duration
4. interval duration.Duration

# Functions

## NewPortfolioMovement(store domain.GenericStore, events domain.EventStore) *PortfolioMovement

Build the monitor.

## PortfolioMovement.Name() string

Return domain.Events.PortfolioMovement without domain.NotificationPrefix, portfolio.movement.

## PortfolioMovement.ValidateParams(params Params) error

Return the error of movementSettingsFrom, or nil.

## PortfolioMovement.Run(ctx Context, params Params) error

1. Read the settings with movementSettingsFrom.
2. Read the movement through store.GetMarketMovement with the window and the variation.
3. if there is none, log at info level and return nil.
4. Find the side with movementSettings.sideReached.
5. if there is none, log at info level and return nil.
6. Read the date of the last portfolio movement event with lastEventDate.
7. if withinInterval holds for it against the movement's end date, log at info level and return nil.
8. Record the event with record.
9. Return nil.

#### Errors

- **1.** if reading fails, return the error.

---

- **2.** if reading fails, return the wrapped error.

---

- **6.** return the error of lastEventDate.

---

- **8.** return the error of record.

The threshold is compared by the store, in SQL and without rounding. The interval is read after the threshold so a quiet day costs no event read.

## PortfolioMovement.record(ctx Context, settings movementSettings, movement *domain.MarketMovement, side direction) error

Record a portfolio movement event through events.Record, with the end date, an at sign and the side as its subject and a PortfolioMovementData built from the movement, the window text as written and the side as its payload. Return the wrapped error when encoding or recording fails.

## movementSettings.sideReached(movement *domain.MarketMovement) direction

Return up when the movement is at least the threshold upward and the direction watches up, otherwise down in the same way, otherwise empty. Both sides cannot hold at once.

## movementSettingsFrom(params Params) (movementSettings, error)

1. Reject any setting other than variation, direction, window and interval with Params.Unknown.
2. Read the variation with Params.Decimal; it is required and must be positive.
3. Read the direction with directionFrom.
4. Read the window and the interval with Params.DurationOr and movementDefaultSpan.

#### Errors

- **2.** a missing, non-text, malformed or non-positive variation is an error naming the setting.

---

- **3.** return the error naming the unknown direction.

---

- **4.** return the duration error naming the setting and the text.
