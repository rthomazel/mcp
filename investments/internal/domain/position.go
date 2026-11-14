package domain

// Position is an active holding.
type Position struct {
	InstrumentID int
	Symbol       string
	Exchange     string
	Quantity     Decimal
}

// IntegrityError reports an instrument whose ledger goes negative.
type IntegrityError struct {
	InstrumentID      int
	Symbol            string
	Exchange          string
	FirstNegativeDate Date
	Quantity          Decimal // running quantity at that entry
}

// PositionsResult is the reconstruction of positions from the ledger.
type PositionsResult struct {
	Date            *Date // nil means the whole ledger
	Positions       []Position
	IntegrityErrors []IntegrityError
}
