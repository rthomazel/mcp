package domain

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// CashKind is one of the Cash constants.
type CashKind string

// Cash kinds. Signs: the opening balance takes any, deposit and interest are
// positive, withdrawal and fee are negative.
const (
	CashOpeningBalance CashKind = "OPENING_BALANCE"
	CashDeposit        CashKind = "DEPOSIT"
	CashWithdrawal     CashKind = "WITHDRAWAL"
	CashInterest       CashKind = "INTEREST"
	CashFee            CashKind = "FEE"
)

// CashTransaction is an account-level cash event that belongs to no
// instrument. It is also the input to RecordCashTransaction, with a zero ID.
type CashTransaction struct {
	ID            int64
	Kind          CashKind
	Amount        Decimal // signed: positive adds cash, negative removes it
	ExecutedAt    time.Time
	EffectiveDate Date
	Notes         *string
}

// CashRecordResult is the outcome of recording a cash transaction.
type CashRecordResult struct {
	CashTransaction CashTransaction
	// RebuildFrom is set when the entry moves a cash balance on or before the
	// latest snapshot date; cash snapshots from that date onward are stale.
	RebuildFrom *Date
}

// CashTransactionFilter selects cash transactions to list.
type CashTransactionFilter struct {
	Kind  CashKind // empty for any
	From  *Date    // inclusive effective date
	To    *Date    // inclusive effective date
	Limit int      // zero means defaultListLimit, capped at maxListLimit
}

// CashBalance is the cash position at a reference date. Not initialized means
// no balance, equity or buying power, never a zero.
type CashBalance struct {
	Date            Date
	Initialized     bool
	OpeningDate     *Date
	BalanceUSD      *Decimal
	BuyingPowerUSD  *Decimal // the balance floored at zero
	IntegrityErrors []CashIntegrityError
}

// CashIntegrityError reports the first entry at which the running cash
// balance is below zero. Reported, never rejected.
type CashIntegrityError struct {
	FirstNegativeDate Date
	Balance           Decimal
}

// CashSnapshot is a stored daily cash balance.
type CashSnapshot struct {
	Date       Date
	BalanceUSD Decimal
}

// Validate checks the kind, amount, dates, notes and the sign by kind.
func (c CashTransaction) Validate() error {
	if !c.Kind.valid() {
		return fmt.Errorf("%w: unknown cash kind %q", ErrInvalidInput, c.Kind)
	}

	amount, err := ParseDecimal(string(c.Amount))
	if err != nil {
		return fmt.Errorf("amount: %w", err)
	}

	if c.ExecutedAt.IsZero() || c.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: executed_at and effective_date are required", ErrInvalidInput)
	}

	if c.Notes != nil && utf8.RuneCountInString(*c.Notes) > maxNotesLength {
		return fmt.Errorf("%w: notes exceed %d characters", ErrInvalidInput, maxNotesLength)
	}

	switch c.Kind {
	case CashDeposit, CashInterest:
		if amount.Sign() <= 0 {
			return fmt.Errorf("%w: %s amount must be positive, got %s", ErrInvalidInput, c.Kind, amount)
		}
	case CashWithdrawal, CashFee:
		if amount.Sign() >= 0 {
			return fmt.Errorf("%w: %s amount must be negative, got %s", ErrInvalidInput, c.Kind, amount)
		}
	}

	return nil
}

// Normalize applies the default and the cap to the limit and checks the range and kind.
func (f CashTransactionFilter) Normalize() (CashTransactionFilter, error) {
	if f.Limit < 0 {
		return f, fmt.Errorf("%w: limit must not be negative", ErrInvalidInput)
	}

	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return f, fmt.Errorf("%w: from is after to", ErrInvalidInput)
	}

	if f.Kind != "" && !f.Kind.valid() {
		return f, fmt.Errorf("%w: unknown cash kind %q", ErrInvalidInput, f.Kind)
	}

	f.Limit = resolveLimit(f.Limit)

	return f, nil
}

func (k CashKind) valid() bool {
	switch k {
	case CashOpeningBalance, CashDeposit, CashWithdrawal, CashInterest, CashFee:
		return true
	}

	return false
}
