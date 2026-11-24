# Constants

targetsKey = "targets", the one setting.
priceKey = "price", a field of a target; the instrument field is instrumentKey.

# Types

## PriceTarget

1. store domain.MonitorStore, for the price range
2. events domain.EventStore, where the events are recorded

The monitor for instrument daily price ranges reaching target prices.

## priceTargetEntry

1. ticker string, upper case
2. price Decimal, normalized with Decimal.Normalized

Comparable, so a repeated pair is found with a map.

# Functions

## NewPriceTarget(store domain.MonitorStore, events domain.EventStore) *PriceTarget

Build the monitor.

## PriceTarget.Name() string

Return domain.Events.PriceTarget without domain.NotificationPrefix, price.target, the key of its entry in the monitors configuration.

## PriceTarget.ValidateParams(params Params) error

1. Parse the settings with priceTargetsFrom.
2. Return nil.

#### Errors

- **1.** return the error.

## PriceTarget.Run(ctx Context, params Params) error

1. Parse the settings.
2. For each target judge it, collecting failures.
3. Return the failures joined, nil when there are none.

#### Errors

- **1.** return the wrapped error.

---

- **2.** a failing target does not stop the others: its error names the ticker and price.

### judge

1. Read the range through store.GetPriceTarget.
2. if the ticker is not registered, log a warning and return nil.
3. if there is no price on the latest date or the target is not reached, return nil.
4. Record a price target event through events.Record, with the ticker, an at sign and the normalized price as its subject and a PriceTargetData as its payload.

The subject carries no date, so a target is announced once however long the price stays in range; the outbox deletes handled events after a year, which announces it again if the price is in range then. Weekends and holidays judge the same date again and record nothing new.

## priceTargetsFrom(params Params) ([]priceTargetEntry, error)

1. Reject any setting other than targetsKey with Params.Unknown.
2. Read the list with Params.Mappings.
3. For each item read its entry with priceTargetFrom, rejecting a pair of instrument and price already listed.
4. Return the entries in order.

#### Errors

- **1.** return an error naming the unknown setting.

---

- **2.** return an error naming the setting when it is missing, empty or not a list of mappings.

---

- **3.** return an error naming the item.

## priceTargetFrom(item Params) (priceTargetEntry, error)

1. Reject any key other than instrumentKey and priceKey.
2. Read the instrument as text, trimmed and upper case, which may not be empty.
3. Read the price with Params.Decimal, which is required and greater than zero by Decimal.Sign, and normalize it.
4. Return the entry.

#### Errors

- **1.** return an error naming the unknown key.

---

- **2.** return an error naming the setting.

---

- **3.** return an error naming the setting when it is missing, not quoted text, not a decimal or not positive.
