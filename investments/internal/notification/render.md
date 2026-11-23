# Interfaces

## Renderer

1. Render(ctx Context, event domain.Event) (string, error)

Turns an event into plain, channel-neutral text, with no markup, so no handler is written for one service's formatting. A renderer may read current system state. Renderers are registered per event type and format; defaultFormat, plain, is the only format today.

# Types

## SnapshotIncompleteRenderer

1. updates domain.UpdateStore

## PortfolioDrawdownRenderer

No fields.

## PortfolioATHRenderer

No fields.

## PortfolioMovementRenderer

No fields.

## NotificationFailedRenderer

No fields.

# Functions

## NewRenderers(updates domain.UpdateStore) map[string]map[string]Renderer

Return the renderer for domain.Events.SnapshotIncomplete, domain.Events.PortfolioDrawdown, domain.Events.PortfolioATH, domain.Events.PortfolioMovement and domain.Events.NotificationFailed under defaultFormat.

## SnapshotIncompleteRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as SnapshotIncompleteData.
2. List the incomplete snapshot runs through store.Updates.List.
3. Pick the newest whose date range is the event's date.
4. if one exists, return a message naming the date and the run's error message, which names the instruments and their issues.
5. Otherwise return a message naming the date and saying that no run was recorded for it.

#### Errors

- **1.** if decoding fails, return the wrapped error.

---

- **2.** if listing fails, return the wrapped error.

Only symbols, dates and counts reach the message. The no-run wording covers a price refresh job that never ran.

## PortfolioDrawdownRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as PortfolioDrawdownData.
2. Round the drawdown to one decimal place, halves away from zero.
3. Return a message naming the level crossed, the rounded drawdown without its sign, the date of the high with its securities value, the current securities value and the crossing date, and saying that this value change is affected by sales and withdrawals.

#### Errors

- **1.** if decoding fails, return the wrapped error.

---

- **2.** if the drawdown is not a decimal number, return an error naming it.

The message is built from the payload alone. Only the message is rounded; the payload keeps the exact figure, which stays equal to the value history's.

## NotificationFailedRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as NotificationFailedData.
2. Return a message naming the failed event's type, when it was recorded and the number of failed attempts, and saying that its delivery was given up on.

#### Errors

- **1.** if decoding fails, return the wrapped error.

It never repeats the failed event's payload.

## PortfolioATHRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as PortfolioATHData.
2. Round the increase percent with roundedPercent.
3. Return a message naming the date and the new value, the dollar and percent increase over the previous high with its value and date, the date of the last announced high or none, and saying that this value change is affected by sales and withdrawals.

#### Errors

- **1.** if decoding fails, return the wrapped error.

---

- **2.** if the percent is not a decimal number, return an error naming it.

## PortfolioMovementRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as PortfolioMovementData.
2. Round the market percent with roundedPercent.
3. Build a message naming the direction as rose or fell, the rounded percent without its sign and the market dollars with a sign, the window as written, its start and end dates, the securities values at both ends and the split of the change into market and trades.
4. if there are gainers, add a line listing them by symbol with signed dollars.
5. if there are losers, add a line listing them the same way.
6. Add that this value change is affected by sales and withdrawals, and return the message.

#### Errors

- **1.** if decoding fails, return the wrapped error.

---

- **2.** if the percent is not a decimal number, return an error naming it.

## roundedDollars(amount Decimal) (string, error)

Round to cents, halves away from zero, with math/big, and never return a negative zero: an amount that rounds to zero is 0.00.

## dollars(amounts ...Decimal) ([]string, error)

Round each amount with roundedDollars, in order, naming the failure.

## roundedPercent(percent Decimal) (string, error)

Round to one decimal place, halves away from zero, with math/big so no float is involved, and keep the sign. Shared by the renderers that show a percent.

## signedDollars(amount Decimal) (string, error)

Round the amount to cents, then write it with its sign before the dollar sign: +$34.00, -$50.00. Zero is written +$0.00.

## contributorList(contributors []domain.Contributor) (string, error)

Join each symbol and its signed dollars with commas.

Every percent is rounded to one decimal and every dollar amount to cents in the renderer, never earlier: events keep the exact figures from the database, and a message never shows more digits than a reader can use. A renderer for a new event type uses roundedPercent and roundedDollars for each number it prints.
