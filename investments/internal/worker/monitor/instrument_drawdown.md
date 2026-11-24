# Types

## InstrumentDrawdown

1. store domain.MonitorStore, for the drawdown state of an instrument and the held tickers
2. events domain.EventStore, where the events are recorded

The monitor for instrument adjusted closes falling to a multiple of the increment below their all-time highs. The instrument counterpart of PortfolioDrawdown, with the same levels, crossings and increment.

# Functions

## NewInstrumentDrawdown(store domain.MonitorStore, events domain.EventStore) *InstrumentDrawdown

Build the monitor.

## InstrumentDrawdown.Name() string

Return domain.Events.InstrumentDrawdown without domain.NotificationPrefix, instrument.drawdown, the key of its entry in the monitors configuration.

## InstrumentDrawdown.ValidateParams(params Params) error

1. Reject any setting other than incrementKey and instrumentsKey with Params.Unknown.
2. Read the increment with incrementFrom.
3. Read the instruments with selectionFrom.
4. Return nil.

#### Errors

- **1.** return an error naming the unknown setting.

---

- **2.** return an error naming the increment setting when it is missing, does not parse or is not positive.

---

- **3.** return an error naming the instruments setting when it is missing or invalid.

## InstrumentDrawdown.Run(ctx Context, params Params) error

1. Read the settings as ValidateParams does.
2. Resolve the selection to tickers with tickersOf.
3. For each ticker judge it, collecting failures.
4. Return the failures joined, nil when there are none.

#### Errors

- **1.** return the wrapped error.

---

- **2.** return the wrapped error.

---

- **3.** a failing instrument does not stop the others: its error names the ticker.

### judge

1. Read the state through store.GetInstrumentDrawdownState.
2. if the ticker is not registered, log a warning and return nil.
3. if the state has no levels, which includes an instrument with no price on the latest price date, log at info level and return nil.
4. For each level in order, shallowest first, record an instrument drawdown event through events.Record, with the ticker, an at sign and drawdownSubject as its subject and an InstrumentDrawdownData built from the state and the level as its payload.

Recording is idempotent per subject, so repeating the run, or a later run in the same stay, records nothing; a level of a stay that was cleared and entered again has a new crossing date and so a new subject. The monitor holds no state beyond the outbox.
