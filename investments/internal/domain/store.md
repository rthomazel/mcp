# Interfaces

## GenericStore

1. RegisterInstrument(ctx Context, draft Instrument) (Instrument, error)
2. ListInstruments(ctx Context) ([]Instrument, error)
3. RecordTransaction(ctx Context, draft DraftTransaction) (RecordResult, error)
4. ListTransactions(ctx Context, filter TransactionFilter) ([]Transaction, error)
5. GetPositions(ctx Context, date *Date) (PositionsResult, error)
6. GetPortfolio(ctx Context, date *Date) (Portfolio, error)
7. GetStatus(ctx Context) (Status, error)
8. GetValueHistory(ctx Context, filter HistoryFilter) (ValueHistory, error)
9. GetDrawdownState(ctx Context, increment Decimal) (DrawdownState, error)
10. GetHighState(ctx Context) (*HighState, error)
11. GetMarketMovement(ctx Context, window duration.Duration, threshold Decimal) (*MarketMovement, error)
12. GetAllocationDrift(ctx Context, groups []AllocationGroup) (*AllocationDrift, error)
13. GetPriceVolatility(ctx Context, ticker string, window duration.Duration, lookback duration.Duration, threshold Decimal) (*PriceVolatility, error)

The generic store, for everything that has no narrower store: instruments, the ledger and the read-only reports. Groups of related methods have their own stores below. Tests mock each store with mockery. Implemented by internal/storage/postgres.
Each method of every store is a self-contained unit of work in its own transaction; composing several store calls into one transaction is not supported.

### RegisterInstrument(ctx Context, draft Instrument) (Instrument, error)

Normalizes and validates draft, then inserts it. Returns ErrInvalidInput, or ErrInstrumentExists when the symbol and exchange pair is taken.

### ListInstruments(ctx Context) ([]Instrument, error)

Returns every instrument ordered by symbol, then exchange; an empty registry is an empty slice.

### RecordTransaction(ctx Context, draft DraftTransaction) (RecordResult, error)

Validates draft, including an optional fee on buys and sells, serializes with other writers of the same instrument, inserts it, and verifies the whole instrument ledger stays long-only. Returns ErrInvalidInput, ErrInstrumentNotFound, or a NegativeBalanceError, in which case nothing is written. RecordResult.RebuildFrom reports stale snapshots, securities or cash; it never rebuilds.

### ListTransactions(ctx Context, filter TransactionFilter) ([]Transaction, error)

Returns ledger entries matching filter, newest first by ledger order, limited. Returns ErrInvalidInput for a negative limit or an inverted range.

### GetPositions(ctx Context, date *Date) (PositionsResult, error)

Reconstructs positions from the ledger as of date, or the whole ledger when nil. Ledger integrity errors are returned alongside and never omitted.

### GetPortfolio(ctx Context, date *Date) (Portfolio, error)

Values active positions with real closing prices as of date, or today in the market timezone when nil, and reports each holding's average-cost P&L, closed positions, P&L totals, the cash balance and account equity beside the securities value. A missing or stale price makes the portfolio incomplete.

### GetStatus(ctx Context) (Status, error)

Reports freshness and health as of today in the market timezone: per held instrument the latest price date and whether it is missing or stale, the latest price date and latest completed snapshot date, whether the latest price date has no completed snapshot, the failed or incomplete runs no later run has resolved, the events given up on that are not yet expired, and, apart from health, the expired events. Healthy only when no run, price, snapshot or given-up event is a problem; expired events never affect it.

### GetValueHistory(ctx Context, filter HistoryFilter) (ValueHistory, error)

Returns the securities value, and cash and account equity where cash snapshots exist, over the range with one point per window of days, with summary measures: latest value, all-time high, drawdown from it, change from the baseline, range high and low and maximum drawdown. Returns ErrInvalidInput for a negative window, an inverted range or a series over the point limit. Changes in value, not returns.

### GetDrawdownState(ctx Context, increment Decimal) (DrawdownState, error)

Returns the drawdown of the latest securities value from the all-time high, and for each multiple of increment the drawdown is at or below the date its current stay at that level began, over the series GetValueHistory uses. The all-time high is over every stored date, whatever the range. Returns ErrInvalidInput for an increment that is not positive, and an empty state, no date and no levels, when there are no snapshots. Changes in value, not returns.

### GetHighState(ctx Context) (*HighState, error)

Returns the latest securities value against the highest earlier value, over the series GetValueHistory uses, nil when there are no snapshots. Changes in value, not returns.

### GetMarketMovement(ctx Context, window duration.Duration, threshold Decimal) (*MarketMovement, error)

Returns the securities value change over window ending at the latest snapshot date that prices explain, with the part trades explain, the largest gainers and losers by symbol, and whether the percent reaches the threshold in either direction. The window start is the window counted back from the latest date by duration arithmetic. Nil when there are no snapshots or none on or before the window start. Returns ErrInvalidInput for a threshold that is not positive. Changes in value, not returns.

### GetAllocationDrift(ctx Context, groups []AllocationGroup) (*AllocationDrift, error)

Returns each group's share of the securities value on the latest date of the series GetValueHistory uses, in the order given, and for a group outside its band the side and the first date of the stay, with every registered member's share. A date without securities has no allocation and is inside every band; being exactly at a bound is inside. Tickers that are not registered are listed and count as zero. Nil when there are no snapshots or the latest date holds no securities. A ticker matching several instruments returns ErrInvalidInput. A share of the value, changed by trades as well as by prices.

### GetPriceVolatility(ctx Context, ticker string, window duration.Duration, lookback duration.Duration, threshold Decimal) (*PriceVolatility, error)

Returns the move of the ticker's adjusted close on the latest price date in the store over window, the volatility of its moves of the same length over lookback, the variation and whether it reaches threshold in either direction. Nil when the ticker has no price on the latest date or none on or before the window start. Returns ErrInstrumentNotFound for an unregistered ticker and ErrInvalidInput for one matching several instruments or a threshold that is not positive.

## CashStore

1. Record(ctx Context, draft CashTransaction) (CashRecordResult, error)
2. List(ctx Context, filter CashTransactionFilter) ([]CashTransaction, error)
3. Get(ctx Context, date *Date) (CashBalance, error)

Cash transactions and the balance.

### Record(ctx Context, draft CashTransaction) (CashRecordResult, error)

Validates draft and inserts it. Returns ErrInvalidInput, including when a second opening balance is recorded. A balance that would go negative is not rejected. CashRecordResult.RebuildFrom reports stale cash snapshots; it never rebuilds.

### List(ctx Context, filter CashTransactionFilter) ([]CashTransaction, error)

Returns cash transactions matching filter, newest first by effective date, limited. Returns ErrInvalidInput for a negative limit, an inverted range or an unknown kind.

### Get(ctx Context, date *Date) (CashBalance, error)

Computes the cash balance as of date, or today in the market timezone when nil, from the cash ledger. Not initialized without an opening balance on or before the date. Negative running balances are returned as integrity errors, never omitted.

## PriceStore

1. Upsert(ctx Context, instrumentID int, bars []PriceBar) error
2. LatestDate(ctx Context, instrumentID int) (*Date, error)
3. LatestSessionDate(ctx Context) (*Date, error)
4. List(ctx Context, filter PriceFilter) ([]PriceBar, error)

Stored daily prices.

### Upsert(ctx Context, instrumentID int, bars []PriceBar) error

Inserts the bars for an instrument, replacing the stored record for any date that already has one, so a re-fetched revised day corrects the stored figures. All bars are stored or none. An empty slice does nothing. Returns ErrInvalidInput for an invalid bar or a repeated date, and ErrInstrumentNotFound for an unknown instrument.

### LatestDate(ctx Context, instrumentID int) (*Date, error)

Returns the most recent stored price date for the instrument, or nil when it has no prices.

### LatestSessionDate(ctx Context) (*Date, error)

Returns the most recent price date across all instruments, or nil when no prices exist. It is the date the price refresh job snapshots.

### List(ctx Context, filter PriceFilter) ([]PriceBar, error)

Returns the stored price records of one instrument matching filter, newest first, limited. Returns ErrInvalidInput for an invalid filter; an unknown instrument has no prices and returns an empty slice.

## SnapshotStore

1. Write(ctx Context, date Date) (SnapshotResult, error)
2. Rebuild(ctx Context, from Date, to Date, reason string) (RebuildResult, error)
3. List(ctx Context, filter SnapshotFilter) ([]SnapshotRow, error)

Daily snapshots of positions and cash.

### Write(ctx Context, date Date) (SnapshotResult, error)

Writes the snapshot rows for one date, and the date's cash snapshot when cash is initialized, and records the outcome in the updates log, all in one transaction. Does nothing when the date is already completed. Writes no rows and records an incomplete run when an active instrument lacks a price dated exactly on date. Returns ErrInvalidInput for a date after today in the market timezone, and ErrSnapshotExists when unmarked rows exist for the date.

### Rebuild(ctx Context, from Date, to Date, reason string) (RebuildResult, error)

Replaces the securities and cash snapshots for every price date in the range from the current ledger and prices, all or nothing, and records the rebuild with its reason and range. Returns ErrInvalidInput for an empty reason, an inverted range or a date after today, and an IncompleteSnapshotError when any date cannot be valued, in which case nothing changes and a failed rebuild is still logged.

### List(ctx Context, filter SnapshotFilter) ([]SnapshotRow, error)

Returns snapshot rows matching filter, newest date first then by symbol and exchange, limited. Returns ErrInvalidInput for an invalid filter.

## UpdateStore

1. Record(ctx Context, update Update) error
2. List(ctx Context, filter UpdateFilter) ([]Update, error)

The updates log.

### Record(ctx Context, update Update) error

Appends one row to the updates log. Returns ErrInvalidInput for an invalid update.

### List(ctx Context, filter UpdateFilter) ([]Update, error)

Returns updates log rows matching filter, newest first by start time, limited. Returns ErrInvalidInput for an invalid filter.

## EventStore

1. Record(ctx Context, draft DraftEvent) (created bool, err error)
2. ListDue(ctx Context, typePrefix string, limit int) ([]Event, error)
3. SaveOutcome(ctx Context, id int64, outcome EventOutcome) error
4. Remove(ctx Context, id int64) error
5. Prune(ctx Context) (int, error)
6. Latest(ctx Context, eventType string, subjectPrefix string) (*Event, error)

The events outbox. Consumed by the events worker, the monitors and the remove_event tool.

### Record(ctx Context, draft DraftEvent) (created bool, err error)

Validates draft and inserts it into the outbox. When the draft has a subject and an event of the same type and subject already exists, does nothing and reports created false, so a monitor announces each subject once; a removed event no longer counts. Returns ErrInvalidInput for an invalid draft.

### ListDue(ctx Context, typePrefix string, limit int) ([]Event, error)

Returns up to limit events whose type starts with typePrefix, empty for any, that Event.Eligible accepts at the store's current time, oldest first by recorded time then id: not fully handled, not given up on, not yet expired, and past their backoff. Expired and given-up events are never returned. Events of other types are not read, so one kind of worker never sees or fails another kind's events. Returns ErrInvalidInput for a non-positive limit.

### SaveOutcome(ctx Context, id int64, outcome EventOutcome) error

Applies the outcome of one handling pass in one transaction: adds the succeeded handler names to the event, sets it fully handled when the outcome says so, and for a failure counts it and postpones the next attempt by the policy's backoff for the new count. When the count reaches the policy's maximum failures the event is given up on instead, an error is logged naming its id, type and failure count and never its payload, and a notification-failed event is recorded in the same transaction, except for an event that is itself notification-failed, which only logs. Returns ErrInvalidInput for an invalid outcome and ErrEventNotFound when the event is gone.

### Remove(ctx Context, id int64) error

Deletes an event that was given up on or has expired. Returns ErrEventNotFound for an unknown id, and ErrEventNotRemovable for an event that is pending, backing off or fully handled; nothing is deleted then.

### Latest(ctx Context, eventType string, subjectPrefix string) (*Event, error)

Returns the most recently recorded event of the type whose subject starts with subjectPrefix, empty for any, handled or not, newest by recorded time then id; nil when there is none, which includes events deleted by pruning. Monitors read the date in its payload to space their announcements.

### Prune(ctx Context) (int, error)

Deletes fully handled events whose handling finished the policy's retention or longer ago, and returns how many. Events not fully handled are never deleted, however old.

# Types

## Stores

1. Generic GenericStore
2. Cash CashStore
3. Prices PriceStore
4. Snapshots SnapshotStore
5. Updates UpdateStore
6. Events EventStore

Not an interface: the bundle of every store, for the consumers that need several, such as the tool handlers. A consumer that needs one takes that one store, so its tests fake only what it uses.
