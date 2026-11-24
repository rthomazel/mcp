package domain

// PriceVolatility is an instrument's price move over a window on the latest price
// date, against the volatility of its moves of the same length over a look-back.
type PriceVolatility struct {
	Date              Date
	StartDate         Date     // last price date on or before the window start
	MovePercent       Decimal  // adjusted close change over the window
	Moves             int      // moves in the look-back the volatility is taken from
	VolatilityPercent *Decimal // sample standard deviation of those moves; nil when there are fewer than two or it is zero
	Variation         *Decimal // MovePercent over VolatilityPercent, in standard deviations, signed; nil without volatility
	AtLeastUp         bool     // Variation is at or above the threshold
	AtLeastDown       bool     // Variation is at or below minus the threshold
}
