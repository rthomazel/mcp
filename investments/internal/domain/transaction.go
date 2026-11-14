package domain

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// EventType is one of the Event constants.
type EventType string

const (
	EventBuy        EventType = "BUY"
	EventSell       EventType = "SELL"
	EventDividend   EventType = "DIVIDEND"
	EventTax        EventType = "TAX"
	EventSplit      EventType = "SPLIT"
	EventAdjustment EventType = "ADJUSTMENT"

	maxNotesLength   = 1000
	defaultListLimit = 100
	maxListLimit     = 500
)

// Transaction is a stored ledger entry.
type Transaction struct {
	ID             int64
	InstrumentID   int
	EventType      EventType
	QuantityChange Decimal
	CashAmount     Decimal
	ExecutedAt     time.Time
	EffectiveDate  Date
	Notes          *string
}

// DraftTransaction is caller input that is not yet saved. An empty cash
// amount means zero. The caller supplies the effective date: there is no
// market calendar, so after-hours events carry the next session's date.
type DraftTransaction struct {
	InstrumentID   int
	EventType      EventType
	QuantityChange Decimal
	CashAmount     Decimal
	ExecutedAt     time.Time
	EffectiveDate  Date
	Notes          *string
}

// RecordResult is the outcome of recording a transaction.
type RecordResult struct {
	Transaction Transaction
	// RebuildFrom is set when the entry changed quantity on or before the
	// latest snapshot date; snapshots from that date onward are stale.
	RebuildFrom *Date
}

// TransactionFilter selects ledger entries to list.
type TransactionFilter struct {
	InstrumentID *int
	From         *Date // inclusive effective date
	To           *Date // inclusive effective date
	Limit        int   // zero means defaultListLimit, capped at maxListLimit
}

// Validate checks the draft's event type, amounts, dates, notes and signs.
func (d DraftTransaction) Validate() error {
	switch d.EventType {
	case EventBuy, EventSell, EventDividend, EventTax, EventSplit, EventAdjustment:
	default:
		return fmt.Errorf("%w: unknown event type %q", ErrInvalidInput, d.EventType)
	}

	quantity, err := ParseDecimal(string(d.QuantityChange))
	if err != nil {
		return fmt.Errorf("quantity_change: %w", err)
	}

	cash := Decimal("0")
	if d.CashAmount != "" {
		if cash, err = ParseDecimal(string(d.CashAmount)); err != nil {
			return fmt.Errorf("cash_amount: %w", err)
		}
	}

	if d.ExecutedAt.IsZero() || d.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: executed_at and effective_date are required", ErrInvalidInput)
	}

	if d.Notes != nil && utf8.RuneCountInString(*d.Notes) > maxNotesLength {
		return fmt.Errorf("%w: notes exceed %d characters", ErrInvalidInput, maxNotesLength)
	}

	return validateSigns(d.EventType, quantity.Sign(), cash.Sign())
}

func validateSigns(event EventType, quantity, cash int) error {
	var quantityOK, cashOK bool

	var quantityRule, cashRule string

	switch event {
	case EventBuy:
		quantityOK, cashOK = quantity > 0, cash < 0
		quantityRule, cashRule = "positive", "negative"
	case EventSell:
		quantityOK, cashOK = quantity < 0, cash > 0
		quantityRule, cashRule = "negative", "positive"
	case EventDividend:
		quantityOK, cashOK = quantity == 0, cash > 0
		quantityRule, cashRule = "zero", "positive"
	case EventTax:
		quantityOK, cashOK = quantity == 0, cash < 0
		quantityRule, cashRule = "zero", "negative"
	case EventSplit:
		quantityOK, cashOK = quantity != 0, cash == 0
		quantityRule, cashRule = "non-zero", "zero"
	default: // EventAdjustment
		quantityOK, cashOK = quantity != 0, true
		quantityRule, cashRule = "non-zero", "any"
	}

	if !quantityOK {
		return fmt.Errorf("%w: %s quantity_change must be %s", ErrInvalidInput, event, quantityRule)
	}

	if !cashOK {
		return fmt.Errorf("%w: %s cash_amount must be %s", ErrInvalidInput, event, cashRule)
	}

	return nil
}

// Normalize applies the default and the cap to the limit and checks the range.
func (f TransactionFilter) Normalize() (TransactionFilter, error) {
	if f.Limit < 0 {
		return f, fmt.Errorf("%w: limit must not be negative", ErrInvalidInput)
	}

	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return f, fmt.Errorf("%w: from is after to", ErrInvalidInput)
	}

	if f.Limit == 0 {
		f.Limit = defaultListLimit
	}

	f.Limit = min(f.Limit, maxListLimit)

	return f, nil
}
