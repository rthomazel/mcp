package domain

// Price issue values for Holding.PriceIssue.
const (
	PriceIssueMissing = "missing"
	PriceIssueStale   = "stale"
)

// Holding is one active position valued at its closing price.
type Holding struct {
	InstrumentID   int
	Symbol         string
	Exchange       string
	Quantity       Decimal
	ClosePrice     *Decimal // nil when no usable price
	PriceDate      *Date
	MarketValueUSD *Decimal // nil when no usable price
	Allocation     *Decimal // nil when the portfolio is incomplete
	PriceIssue     string   // empty when usable
}

// Portfolio is the securities valuation at a reference date. It is incomplete
// when any active holding has no usable price; the total and allocations are
// then withheld.
type Portfolio struct {
	Date               Date
	SecuritiesValueUSD *Decimal
	Complete           bool
	Holdings           []Holding
	IntegrityErrors    []IntegrityError
}
