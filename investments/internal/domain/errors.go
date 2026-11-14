package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidInput is wrapped with the specific reason for any validation failure.
	ErrInvalidInput = errors.New("invalid input")
	// ErrInstrumentNotFound is returned for an unknown instrument reference.
	ErrInstrumentNotFound = errors.New("instrument not found")
	// ErrInstrumentExists is returned when the symbol and exchange pair already exists.
	ErrInstrumentExists = errors.New("instrument already registered")
	// ErrSymbolNotFound is returned by a PriceProvider that does not cover a symbol.
	ErrSymbolNotFound = errors.New("symbol not found")
	// ErrRateLimited is returned by a PriceProvider that throttles a request.
	ErrRateLimited = errors.New("provider rate limited")
	// ErrNegativeBalance is matched through NegativeBalanceError.
	ErrNegativeBalance = errors.New("transaction would make ownership negative")
)

// NegativeBalanceError reports the first ledger entry at which ownership goes negative.
type NegativeBalanceError struct {
	InstrumentID int
	OnDate       Date
	Quantity     Decimal
}

func (e *NegativeBalanceError) Error() string {
	return fmt.Sprintf("%s: instrument %d would hold %s on %s",
		ErrNegativeBalance, e.InstrumentID, e.Quantity, e.OnDate.Format(time.DateOnly))
}

// Is reports a match with ErrNegativeBalance.
func (e *NegativeBalanceError) Is(target error) bool {
	return target == ErrNegativeBalance
}
