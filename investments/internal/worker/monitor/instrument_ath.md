# Types

## InstrumentATH

1. store domain.MonitorStore, for the high state of an instrument and the held tickers
2. events domain.EventStore, where the events are recorded and the last announced high is read

The monitor for instrument adjusted closes reaching new all-time highs. The instrument counterpart of PortfolioATH: an instrument's first price is not a new high, and instruments are spaced by the interval independently.

# Functions

## NewInstrumentATH(store domain.MonitorStore, events domain.EventStore) *InstrumentATH

Build the monitor.

## InstrumentATH.Name() string

Return domain.Events.InstrumentATH without domain.NotificationPrefix, instrument.ath, the key of its entry in the monitors configuration.

## InstrumentATH.ValidateParams(params Params) error

1. Reject any setting other than intervalKey and instrumentsKey with Params.Unknown.
2. Read the interval with Params.DurationOr and athDefaultInterval.
3. Read the instruments with selectionFrom.
4. Return nil.

#### Errors

- **1.** return an error naming the unknown setting.

---

- **2.** return an error naming the interval setting when it is not a valid duration, which includes a bare number.

---

- **3.** return an error naming the instruments setting when it is missing or invalid.

## InstrumentATH.Run(ctx Context, params Params) error

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

1. Read the state through store.GetInstrumentHighState.
2. if the ticker is not registered, log a warning and return nil.
3. if there is no state or the latest date is not a new high, return nil.
4. Read the date of the last instrument ATH event of the ticker with lastEventDate, over the subject prefix of the ticker and an at sign.
5. if that date is within the interval before the latest date, by withinInterval, log at info level and return nil.
6. Record an instrument ATH event through events.Record, with the ticker, an at sign and the date as its subject and an InstrumentATHData as its payload.

The interval is measured from the date of the last announced event, so the monitor holds no state beyond the outbox.
