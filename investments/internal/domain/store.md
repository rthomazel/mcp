# Interfaces

## Store

1. RegisterInstrument(ctx Context, draft Instrument) (Instrument, error)
2. ListInstruments(ctx Context) ([]Instrument, error)
3. RecordTransaction(ctx Context, draft DraftTransaction) (RecordResult, error)
4. ListTransactions(ctx Context, filter TransactionFilter) ([]Transaction, error)
5. GetPositions(ctx Context, date *Date) (PositionsResult, error)
6. GetPortfolio(ctx Context, date *Date) (Portfolio, error)

The one central persistence interface. Every consumer depends on it, and tests mock it with mockery. Implemented by internal/storage/postgres.
Each method is a self-contained unit of work in its own transaction; composing several Store calls into one transaction is not supported.
Later stages add price, snapshot, rebuild and update-log methods here.

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
