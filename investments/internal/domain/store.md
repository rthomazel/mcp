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

The one central persistence interface. Every consumer depends on it, and tests mock it with mockery. Implemented by internal/storage/postgres.
Each method is a self-contained unit of work in its own transaction; composing several Store calls into one transaction is not supported.

### RegisterInstrument(ctx Context, draft Instrument) (Instrument, error)

Normalizes and validates draft, then inserts it. Returns ErrInvalidInput, or ErrInstrumentExists when the symbol and exchange pair is taken.

### ListInstruments(ctx Context) ([]Instrument, error)

Returns every instrument ordered by symbol, then exchange; an empty registry is an empty slice.

### RecordTransaction(ctx Context, draft DraftTransaction) (RecordResult, error)

Validates draft, serializes with other writers of the same instrument, inserts it, and verifies the whole instrument ledger stays long-only. Returns ErrInvalidInput, ErrInstrumentNotFound, or a NegativeBalanceError, in which case nothing is written. RecordResult.RebuildFrom reports stale snapshots; it never rebuilds.

### ListTransactions(ctx Context, filter TransactionFilter) ([]Transaction, error)

Returns ledger entries matching filter, newest first by ledger order, limited. Returns ErrInvalidInput for a negative limit or an inverted range.

### GetPositions(ctx Context, date *Date) (PositionsResult, error)

Reconstructs positions from the ledger as of date, or the whole ledger when nil. Ledger integrity errors are returned alongside and never omitted.

### GetPortfolio(ctx Context, date *Date) (Portfolio, error)

Values active positions with real closing prices as of date, or today in the market timezone when nil. A missing or stale price makes the portfolio incomplete.

### UpsertPrices(ctx Context, instrumentID int, bars []PriceBar) error

Inserts the bars for an instrument, replacing the stored record for any date that already has one, so a re-fetched revised day corrects the stored figures. All bars are stored or none. An empty slice does nothing. Returns ErrInvalidInput for an invalid bar or a repeated date, and ErrInstrumentNotFound for an unknown instrument.

### LatestPriceDate(ctx Context, instrumentID int) (*Date, error)

Returns the most recent stored price date for the instrument, or nil when it has no prices.

### LatestSessionDate(ctx Context) (*Date, error)

Returns the most recent price date across all instruments, or nil when no prices exist. It is the date the daily job snapshots.

### WriteSnapshot(ctx Context, date Date) (SnapshotResult, error)

Writes the snapshot rows for one date and records the outcome in the updates log, all in one transaction. Does nothing when the date is already completed. Writes no rows and records an incomplete run when an active instrument lacks a price dated exactly on date. Returns ErrInvalidInput for a date after today in the market timezone, and ErrSnapshotExists when unmarked rows exist for the date.

### RebuildSnapshots(ctx Context, from Date, to Date, reason string) (RebuildResult, error)

Replaces the snapshots for every price date in the range from the current ledger and prices, all or nothing, and records the rebuild with its reason and range. Returns ErrInvalidInput for an empty reason, an inverted range or a date after today, and an IncompleteSnapshotError when any date cannot be valued, in which case nothing changes and a failed rebuild is still logged.

### RecordUpdate(ctx Context, update Update) error

Appends one row to the updates log. Returns ErrInvalidInput for an invalid update.

### ListPrices(ctx Context, filter PriceFilter) ([]PriceBar, error)

Returns the stored price records of one instrument matching filter, newest first, limited. Returns ErrInvalidInput for an invalid filter; an unknown instrument has no prices and returns an empty slice.

### ListSnapshots(ctx Context, filter SnapshotFilter) ([]SnapshotRow, error)

Returns snapshot rows matching filter, newest date first then by symbol and exchange, limited. Returns ErrInvalidInput for an invalid filter.

### ListUpdates(ctx Context, filter UpdateFilter) ([]Update, error)

Returns updates log rows matching filter, newest first by start time, limited. Returns ErrInvalidInput for an invalid filter.
