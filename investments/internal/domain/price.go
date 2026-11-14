package domain

import (
	"context"
	"fmt"
)

// PriceBar is a provider-independent daily price record for one instrument.
type PriceBar struct {
	Date          Date
	Open          Decimal
	High          Decimal
	Low           Decimal
	Close         Decimal // real unadjusted close, the only price used to value holdings
	AdjustedClose Decimal // kept for future total-return analysis
	Volume        int64   // zero for mutual funds
}

// PriceFilter selects stored price records of one instrument.
type PriceFilter struct {
	InstrumentID int
	From         *Date // inclusive
	To           *Date // inclusive
	Limit        int   // zero means defaultListLimit, capped at maxListLimit
}

// Validate checks the instrument, the limit and the range.
func (f PriceFilter) Validate() error {
	if f.InstrumentID <= 0 {
		return fmt.Errorf("%w: instrument is required", ErrInvalidInput)
	}

	return validateListFilter(f.Limit, f.From, f.To)
}

// ResolvedLimit applies the default and the cap to Limit.
func (f PriceFilter) ResolvedLimit() int { return resolveLimit(f.Limit) }

func validateListFilter(limit int, from, to *Date) error {
	if limit < 0 {
		return fmt.Errorf("%w: limit must not be negative", ErrInvalidInput)
	}

	if from != nil && to != nil && from.After(*to) {
		return fmt.Errorf("%w: from is after to", ErrInvalidInput)
	}

	return nil
}

// PriceProvider is the provider-independent boundary to market data.
type PriceProvider interface {
	// Name is a short stable provider name.
	Name() string
	// FetchPrices returns daily bars for symbol, oldest first with strictly
	// ascending dates, from the date inclusive, or the full available history
	// when from is nil. Every bar has passed PriceBar.Validate. An empty slice
	// is not an error. Returns ErrSymbolNotFound or ErrRateLimited.
	FetchPrices(ctx context.Context, symbol string, from *Date) ([]PriceBar, error)
}

// Validate checks the date, that every price parses, that close is positive
// and that volume is not negative.
func (b PriceBar) Validate() error {
	if b.Date.IsZero() {
		return fmt.Errorf("%w: price date is required", ErrInvalidInput)
	}

	prices := []struct {
		name  string
		value Decimal
	}{
		{"open", b.Open}, {"high", b.High}, {"low", b.Low}, {"close", b.Close}, {"adjusted close", b.AdjustedClose},
	}
	for _, price := range prices {
		if _, err := ParseDecimal(string(price.value)); err != nil {
			return fmt.Errorf("%s on %s: %w", price.name, b.Date.Format("2006-01-02"), err)
		}
	}

	if b.Close.Sign() <= 0 {
		return fmt.Errorf("%w: close on %s must be positive", ErrInvalidInput, b.Date.Format("2006-01-02"))
	}

	if b.Volume < 0 {
		return fmt.Errorf("%w: volume on %s must not be negative", ErrInvalidInput, b.Date.Format("2006-01-02"))
	}

	return nil
}
