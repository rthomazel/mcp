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
}
