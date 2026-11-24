package domain

// PriceTarget is an instrument's unadjusted price range on the latest price date
// and whether a target price lies within it.
type PriceTarget struct {
	Date    Date
	Low     Decimal
	High    Decimal
	Close   Decimal
	Reached bool // Low <= price <= High, ends included
}
