package domain

import "context"

// GenericStore is the generic persistence interface: instruments, the ledger and the
// read-only reports. Related methods live in the narrower stores below. Each
// method of every store is a self-contained unit of work in its own
// transaction. All are implemented by internal/storage/postgres.
type GenericStore interface {
	// RegisterInstrument normalizes, validates and inserts draft. Returns
	// ErrInvalidInput, or ErrInstrumentExists when the symbol and exchange are taken.
	RegisterInstrument(ctx context.Context, draft Instrument) (Instrument, error)
	// ListInstruments returns every instrument ordered by symbol, then exchange.
	ListInstruments(ctx context.Context) ([]Instrument, error)
	// RecordTransaction validates draft, serializes with other writers of the
	// same instrument, inserts it and verifies the ledger stays long-only.
	// Returns ErrInvalidInput, ErrInstrumentNotFound or a NegativeBalanceError.
	RecordTransaction(ctx context.Context, draft DraftTransaction) (RecordResult, error)
	// ListTransactions returns ledger entries matching filter, newest first by ledger order.
	ListTransactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error)
	// GetStatus aggregates price freshness of held instruments, snapshot
	// coverage and unresolved failed runs into a health view.
	GetStatus(ctx context.Context) (Status, error)
	// GetValueHistory returns the securities value and account equity series
	// with summary measures. Returns ErrInvalidInput, including for a series
	// over the configured maximum number of points.
	GetValueHistory(ctx context.Context, filter HistoryFilter) (ValueHistory, error)
	// GetPositions reconstructs positions as of date, or the whole ledger when nil.
	GetPositions(ctx context.Context, date *Date) (PositionsResult, error)
	// GetPortfolio values active positions at real closes as of date, or today
	// in the market timezone when nil, with average-cost P&L and cash.
	GetPortfolio(ctx context.Context, date *Date) (Portfolio, error)
}

// CashStore holds cash transactions and the balance.
type CashStore interface {
	// Record validates draft and inserts it. Returns ErrInvalidInput,
	// including for a second opening balance. A balance that goes negative is
	// not rejected. RebuildFrom reports stale cash snapshots.
	Record(ctx context.Context, draft CashTransaction) (CashRecordResult, error)
	// List returns cash transactions matching filter, newest first.
	List(ctx context.Context, filter CashTransactionFilter) ([]CashTransaction, error)
	// Get returns the cash balance as of date, or today in the market
	// timezone when nil; not initialized without an opening balance.
	Get(ctx context.Context, date *Date) (CashBalance, error)
}

// PriceStore holds the stored daily prices.
type PriceStore interface {
	// Upsert stores the bars for an instrument, replacing the record of any
	// date already stored. All bars are stored or none. Returns
	// ErrInvalidInput or ErrInstrumentNotFound.
	Upsert(ctx context.Context, instrumentID int, bars []PriceBar) error
	// LatestDate returns the latest stored price date, nil when there is none.
	LatestDate(ctx context.Context, instrumentID int) (*Date, error)
	// LatestSessionDate returns the latest price date across all instruments, nil when none.
	LatestSessionDate(ctx context.Context) (*Date, error)
	// List returns the stored price records of one instrument, newest first,
	// limited. Returns ErrInvalidInput.
	List(ctx context.Context, filter PriceFilter) ([]PriceBar, error)
}

// SnapshotStore holds the daily snapshots.
type SnapshotStore interface {
	// Write writes the snapshot rows for one date and logs the outcome in one
	// transaction. It does nothing for a completed date and writes no rows
	// when an active instrument lacks a price dated on date. Returns
	// ErrInvalidInput or ErrSnapshotExists.
	Write(ctx context.Context, date Date) (SnapshotResult, error)
	// Rebuild replaces the snapshots for every price date in the range, all or
	// nothing, and logs the rebuild. Returns ErrInvalidInput or an
	// IncompleteSnapshotError.
	Rebuild(ctx context.Context, from, to Date, reason string) (RebuildResult, error)
	// List returns snapshot rows, newest date first then by symbol and
	// exchange, limited. Returns ErrInvalidInput.
	List(ctx context.Context, filter SnapshotFilter) ([]SnapshotRow, error)
}

// UpdateStore holds the updates log.
type UpdateStore interface {
	// Record appends one row to the updates log. Returns ErrInvalidInput.
	Record(ctx context.Context, update Update) error
	// List returns updates log rows, newest first, limited. Returns ErrInvalidInput.
	List(ctx context.Context, filter UpdateFilter) ([]Update, error)
}

// EventStore holds the events outbox.
type EventStore interface {
	// Record validates draft and inserts it into the outbox. With a subject,
	// an existing event of the same type and subject makes it a no-op
	// reporting created false. Returns ErrInvalidInput.
	Record(ctx context.Context, draft DraftEvent) (created bool, err error)
	// ListDue returns up to limit events Event.Eligible accepts now whose type
	// starts with typePrefix (empty for any), oldest first. Returns
	// ErrInvalidInput for a non-positive limit.
	ListDue(ctx context.Context, typePrefix string, limit int) ([]Event, error)
	// SaveOutcome applies one handling pass in one transaction: merges
	// succeeded handler names, marks handled, or counts a failure with backoff
	// and gives up at the configured maximum failures (error log,
	// notification.failed event). Returns ErrInvalidInput or ErrEventNotFound.
	SaveOutcome(ctx context.Context, id int64, outcome EventOutcome) error
	// Remove deletes an event given up on or expired. Returns
	// ErrEventNotFound or ErrEventNotRemovable.
	Remove(ctx context.Context, id int64) error
	// Prune deletes fully handled events older than the configured retention
	// and returns how many.
	Prune(ctx context.Context) (int, error)
}

// Stores bundles every store for the consumers that need several, such as the
// tool handlers. A consumer that needs one takes that one.
type Stores struct {
	Generic   GenericStore
	Cash      CashStore
	Prices    PriceStore
	Snapshots SnapshotStore
	Updates   UpdateStore
	Events    EventStore
}
