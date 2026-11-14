package domain

import "context"

// Store is the central persistence interface. Each method is a self-contained
// unit of work in its own transaction. Implemented by internal/storage/postgres.
type Store interface {
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
	// GetPositions reconstructs positions as of date, or the whole ledger when nil.
	GetPositions(ctx context.Context, date *Date) (PositionsResult, error)
	// GetPortfolio values active positions at real closes as of date, or today
	// in the market timezone when nil.
	GetPortfolio(ctx context.Context, date *Date) (Portfolio, error)
	// UpsertPrices stores the bars for an instrument, replacing the record of
	// any date already stored. All bars are stored or none. Returns
	// ErrInvalidInput or ErrInstrumentNotFound.
	UpsertPrices(ctx context.Context, instrumentID int, bars []PriceBar) error
	// LatestPriceDate returns the latest stored price date, nil when there is none.
	LatestPriceDate(ctx context.Context, instrumentID int) (*Date, error)
	// LatestSessionDate returns the latest price date across all instruments, nil when none.
	LatestSessionDate(ctx context.Context) (*Date, error)
	// WriteSnapshot writes the snapshot rows for one date and logs the outcome
	// in one transaction. It does nothing for a completed date and writes no
	// rows when an active instrument lacks a price dated on date. Returns
	// ErrInvalidInput or ErrSnapshotExists.
	WriteSnapshot(ctx context.Context, date Date) (SnapshotResult, error)
	// RebuildSnapshots replaces the snapshots for every price date in the
	// range, all or nothing, and logs the rebuild. Returns ErrInvalidInput or
	// an IncompleteSnapshotError.
	RebuildSnapshots(ctx context.Context, from, to Date, reason string) (RebuildResult, error)
	// RecordUpdate appends one row to the updates log. Returns ErrInvalidInput.
	RecordUpdate(ctx context.Context, update Update) error
	// ListPrices returns the stored price records of one instrument, newest
	// first, limited. Returns ErrInvalidInput.
	ListPrices(ctx context.Context, filter PriceFilter) ([]PriceBar, error)
	// ListSnapshots returns snapshot rows, newest date first then by symbol and
	// exchange, limited. Returns ErrInvalidInput.
	ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]SnapshotRow, error)
	// ListUpdates returns updates log rows, newest first, limited. Returns ErrInvalidInput.
	ListUpdates(ctx context.Context, filter UpdateFilter) ([]Update, error)
}
