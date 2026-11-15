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

	CostUSD              Decimal  // average-cost basis of the shares held
	AverageCostUSD       Decimal  // cost per share
	UnrealizedPnLUSD     *Decimal // nil when no usable price
	UnrealizedPnLPercent *Decimal // percent of cost, nil when unrealized is nil or cost is zero
	RealizedPnLUSD       Decimal  // cumulative up to the date
	DividendsUSD         Decimal
	TaxUSD               Decimal  // positive amount withheld
	FeesUSD              Decimal  // for information: already inside cost and proceeds
	TotalReturnUSD       *Decimal // unrealized + realized + dividends - tax; nil when unrealized is nil
}

// ClosedPosition is a fully sold instrument and what it earned. Never needs a price.
type ClosedPosition struct {
	InstrumentID   int
	Symbol         string
	Exchange       string
	RealizedPnLUSD Decimal
	DividendsUSD   Decimal
	TaxUSD         Decimal
	FeesUSD        Decimal
	TotalReturnUSD Decimal // realized + dividends - tax
}

// PnLTotals sums over every open and closed position. Figures that need a
// price are nil when the portfolio is incomplete.
type PnLTotals struct {
	CostUSD          Decimal
	UnrealizedPnLUSD *Decimal
	RealizedPnLUSD   Decimal
	DividendsUSD     Decimal
	TaxUSD           Decimal
	FeesUSD          Decimal
	TotalReturnUSD   *Decimal
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
	Cash               CashBalance
	AccountEquityUSD   *Decimal // securities value plus cash; nil when incomplete or cash is not initialized
	ClosedPositions    []ClosedPosition
	PnL                PnLTotals
}
