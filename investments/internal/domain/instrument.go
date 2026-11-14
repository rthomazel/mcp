package domain

import (
	"fmt"
	"strings"
)

const (
	InstrumentStock = "STOCK"
	InstrumentETF   = "ETF"
	InstrumentFund  = "FUND"
	InstrumentOther = "OTHER"

	currencyUSD = "USD"
)

// Instrument is a registered tradable. It also serves as an unsaved draft,
// whose ID is ignored and assigned on insert.
type Instrument struct {
	ID             int
	Symbol         string
	Exchange       string
	Name           string
	Currency       string
	InstrumentType string
}

// Normalize trims all fields and upper-cases symbol, exchange, currency and type.
func (i Instrument) Normalize() Instrument {
	i.Symbol = strings.ToUpper(strings.TrimSpace(i.Symbol))
	i.Exchange = strings.ToUpper(strings.TrimSpace(i.Exchange))
	i.Name = strings.TrimSpace(i.Name)
	i.Currency = strings.ToUpper(strings.TrimSpace(i.Currency))
	i.InstrumentType = strings.ToUpper(strings.TrimSpace(i.InstrumentType))

	return i
}

// Validate checks a normalized instrument; it does not normalize.
func (i Instrument) Validate() error {
	if i.Symbol == "" || i.Exchange == "" || i.Name == "" {
		return fmt.Errorf("%w: symbol, exchange and name are required", ErrInvalidInput)
	}

	if i.Currency != currencyUSD {
		return fmt.Errorf("%w: currency %q is not supported, only %s", ErrInvalidInput, i.Currency, currencyUSD)
	}

	switch i.InstrumentType {
	case InstrumentStock, InstrumentETF, InstrumentFund, InstrumentOther:
		return nil
	}

	return fmt.Errorf("%w: unknown instrument type %q", ErrInvalidInput, i.InstrumentType)
}
