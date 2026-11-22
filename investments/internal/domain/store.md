# Interfaces

## Store

1. RegisterInstrument(ctx Context, draft Instrument) (Instrument, error)
2. ListInstruments(ctx Context) ([]Instrument, error)
3. RecordTransaction(ctx Context, draft DraftTransaction) (RecordResult, error)
4. ListTransactions(ctx Context, filter TransactionFilter) ([]Transaction, error)
5. GetPositions(ctx Context, date *Date) (PositionsResult, error)
6. GetPortfolio(ctx Context, date *Date) (Portfolio, error)
7. UpsertPrices(ctx Context, instrumentID int, bars []PriceBar) error
8. LatestPriceDate(ctx Context, instrumentID int) (*Date, error)
9. LatestSessionDate(ctx Context) (*Date, error)
10. WriteSnapshot(ctx Context, date Date) (SnapshotResult, error)
11. RebuildSnapshots(ctx Context, from Date, to Date, reason string) (RebuildResult, error)
12. RecordUpdate(ctx Context, update Update) error
13. ListPrices(ctx Context, filter PriceFilter) ([]PriceBar, error)
14. ListSnapshots(ctx Context, filter SnapshotFilter) ([]SnapshotRow, error)
15. ListUpdates(ctx Context, filter UpdateFilter) ([]Update, error)
16. RecordCashTransaction(ctx Context, draft CashTransaction) (CashRecordResult, error)
17. ListCashTransactions(ctx Context, filter CashTransactionFilter) ([]CashTransaction, error)
18. GetCash(ctx Context, date *Date) (CashBalance, error)
19. GetStatus(ctx Context) (Status, error)
20. GetValueHistory(ctx Context, filter HistoryFilter) (ValueHistory, error)
21. RecordEvent(ctx Context, draft DraftEvent) (created bool, err error)
22. ListDueEvents(ctx Context, limit int) ([]Event, error)
23. SaveEventOutcome(ctx Context, id int64, outcome EventOutcome) error
24. RemoveEvent(ctx Context, id int64) error
25. PruneEvents(ctx Context) (int, error)

The one central persistence interface. Every consumer depends on it, and tests mock it with mockery. Implemented by internal/storage/postgres.
Each method is a self-contained unit of work in its own transaction; composing several Store calls into one transaction is not supported.

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

### UpsertPrices(ctx Context, instrumentID int, bars []PriceBar) error

Inserts the bars for an instrument, replacing the stored record for any date that already has one, so a re-fetched revised day corrects the stored figures. All bars are stored or none. An empty slice does nothing. Returns ErrInvalidInput for an invalid bar or a repeated date, and ErrInstrumentNotFound for an unknown instrument.

### LatestPriceDate(ctx Context, instrumentID int) (*Date, error)

Returns the most recent stored price date for the instrument, or nil when it has no prices.

### LatestSessionDate(ctx Context) (*Date, error)

Returns the most recent price date across all instruments, or nil when no prices exist. It is the date the daily job snapshots.

### WriteSnapshot(ctx Context, date Date) (SnapshotResult, error)

Writes the snapshot rows for one date, and the date's cash snapshot when cash is initialized, and records the outcome in the updates log, all in one transaction. Does nothing when the date is already completed. Writes no rows and records an incomplete run when an active instrument lacks a price dated exactly on date. Returns ErrInvalidInput for a date after today in the market timezone, and ErrSnapshotExists when unmarked rows exist for the date.

### RebuildSnapshots(ctx Context, from Date, to Date, reason string) (RebuildResult, error)

Replaces the securities and cash snapshots for every price date in the range from the current ledger and prices, all or nothing, and records the rebuild with its reason and range. Returns ErrInvalidInput for an empty reason, an inverted range or a date after today, and an IncompleteSnapshotError when any date cannot be valued, in which case nothing changes and a failed rebuild is still logged.

### RecordUpdate(ctx Context, update Update) error

Appends one row to the updates log. Returns ErrInvalidInput for an invalid update.

### ListPrices(ctx Context, filter PriceFilter) ([]PriceBar, error)

Returns the stored price records of one instrument matching filter, newest first, limited. Returns ErrInvalidInput for an invalid filter; an unknown instrument has no prices and returns an empty slice.

### ListSnapshots(ctx Context, filter SnapshotFilter) ([]SnapshotRow, error)

Returns snapshot rows matching filter, newest date first then by symbol and exchange, limited. Returns ErrInvalidInput for an invalid filter.

### ListUpdates(ctx Context, filter UpdateFilter) ([]Update, error)

Returns updates log rows matching filter, newest first by start time, limited. Returns ErrInvalidInput for an invalid filter.

### RecordCashTransaction(ctx Context, draft CashTransaction) (CashRecordResult, error)

Validates draft and inserts it. Returns ErrInvalidInput, including when a second opening balance is recorded. A balance that would go negative is not rejected. CashRecordResult.RebuildFrom reports stale cash snapshots; it never rebuilds.

### ListCashTransactions(ctx Context, filter CashTransactionFilter) ([]CashTransaction, error)

Returns cash transactions matching filter, newest first by effective date, limited. Returns ErrInvalidInput for a negative limit, an inverted range or an unknown kind.

### GetCash(ctx Context, date *Date) (CashBalance, error)

Computes the cash balance as of date, or today in the market timezone when nil, from the cash ledger. Not initialized without an opening balance on or before the date. Negative running balances are returned as integrity errors, never omitted.

### GetStatus(ctx Context) (Status, error)

Reports freshness and health as of today in the market timezone: per held instrument the latest price date and whether it is missing or stale, the latest price date and latest completed snapshot date, whether the latest price date has no completed snapshot, the failed or incomplete runs no later run has resolved, the events given up on that are not yet expired, and, apart from health, the expired events. Healthy only when no run, price, snapshot or given-up event is a problem; expired events never affect it.

### GetValueHistory(ctx Context, filter HistoryFilter) (ValueHistory, error)

Returns the securities value, and cash and account equity where cash snapshots exist, over the range with one point per window of days, with summary measures: latest value, all-time high, drawdown from it, change from the baseline, range high and low and maximum drawdown. Returns ErrInvalidInput for a negative window, an inverted range or a series over the point limit. Changes in value, not returns.

### RecordEvent(ctx Context, draft DraftEvent) (created bool, err error)

Validates draft and inserts it into the outbox. When the draft has a subject and an event of the same type and subject already exists, does nothing and reports created false, so a monitor announces each subject once; a removed event no longer counts. Returns ErrInvalidInput for an invalid draft.

### ListDueEvents(ctx Context, limit int) ([]Event, error)

Returns up to limit events that Event.Eligible accepts at the store's current time, oldest first by recorded time then id: not fully handled, not given up on, not yet expired, and past their backoff. Expired and given-up events are never returned. Returns ErrInvalidInput for a non-positive limit.

### SaveEventOutcome(ctx Context, id int64, outcome EventOutcome) error

Applies the outcome of one handling pass in one transaction: adds the succeeded handler names to the event, sets it fully handled when the outcome says so, and for a failure counts it and postpones the next attempt by EventBackoff. When the count reaches MaxEventFailures the event is given up on instead, an error is logged naming its id, type and failure count and never its payload, and a notification-failed event is recorded in the same transaction, except for an event that is itself notification-failed, which only logs. Returns ErrInvalidInput for an invalid outcome and ErrEventNotFound when the event is gone.

### RemoveEvent(ctx Context, id int64) error

Deletes an event that was given up on or has expired. Returns ErrEventNotFound for an unknown id, and ErrEventNotRemovable for an event that is pending, backing off or fully handled; nothing is deleted then.

### PruneEvents(ctx Context) (int, error)

Deletes fully handled events whose handling finished EventRetention or longer ago, and returns how many. Events not fully handled are never deleted, however old.
