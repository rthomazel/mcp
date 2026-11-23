# Constants

incrementKey = "increment_pct", the one setting.

# Types

## PortfolioDrawdown

1. store domain.GenericStore, for the drawdown state
2. events domain.EventStore, where the events are recorded

The monitor for the securities value falling to a multiple of the increment below its all-time high.

# Functions

## NewPortfolioDrawdown(store domain.GenericStore, events domain.EventStore) *PortfolioDrawdown

Build the monitor.

## PortfolioDrawdown.Name() string

Return domain.Events.PortfolioDrawdown without domain.NotificationPrefix, portfolio.drawdown, the key of its entry in the monitors configuration.

## PortfolioDrawdown.ValidateParams(params Params) error

1. Reject any setting other than incrementKey with Params.Unknown.
2. Read the increment setting with Params.Decimal.
3. Return nil.

#### Errors

- **1.** return an error naming the unknown setting.

---

- **2.** if the setting is missing, does not parse or is not positive by Decimal.Sign, return an error naming it.

## PortfolioDrawdown.Run(ctx Context, params Params) error

1. Parse the increment setting.
2. Read the state through store.GetDrawdownState.
3. if the state has no levels, log at info level and return nil.
4. For each level in order, shallowest first, record a portfolio drawdown event through events.Record, with drawdownSubject as its subject and a PortfolioDrawdownData built from the state and the level as its payload.
5. Log at info level how many levels were recorded and how many were already announced.
6. Return nil.

#### Errors

- **1.** if parsing fails, return the wrapped error.

---

- **2.** if reading fails, return the wrapped error.

---

- **4.** if recording fails, return the wrapped error; the levels not yet recorded are recorded by the next run.

Recording is idempotent per subject, so repeating the run, or a later run in the same stay, records nothing, and the levels of a stay that was cleared and entered again have a new crossing date and so a new subject. The state is computed by the store from stored snapshots, so the monitor holds no state beyond the outbox.

## drawdownSubject(level Decimal, crossedOn Date) string

Return the level, an at sign and the crossing date as YYYY-MM-DD, for example -6@2026-11-10.
