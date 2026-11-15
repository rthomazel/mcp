# Constants

CashOpeningBalance = "OPENING_BALANCE"
CashDeposit = "DEPOSIT"
CashWithdrawal = "WITHDRAWAL"
CashInterest = "INTEREST"
CashFee = "FEE"

# Types

## CashKind

A string, one of the Cash constants.

## CashTransaction

1. ID int64, zero for an entry not yet stored
2. Kind CashKind
3. Amount Decimal, signed: positive adds cash, negative removes it
4. ExecutedAt Time
5. EffectiveDate Date
6. Notes *string

An account-level cash event that belongs to no instrument. Distinct from Transaction, the instrument ledger; the cash ledger is both together. Also the caller's input to Store.RecordCashTransaction, with ID zero; it is normalized and validated before it touches the database.

## CashRecordResult

1. CashTransaction CashTransaction
2. RebuildFrom *Date, set when the entry changes a cash balance on or before the latest existing snapshot date; cash snapshots from that date onward are stale and need an explicit rebuild.

## CashTransactionFilter

1. Kind CashKind, empty for any
2. From *Date, inclusive effective date
3. To *Date, inclusive effective date
4. Limit int, zero means defaultListLimit, capped at maxListLimit

## CashBalance

1. Date Date, the reference date
2. Initialized bool, true when an opening balance exists on or before Date
3. OpeningDate *Date, the opening balance's effective date, nil when none is recorded
4. BalanceUSD *Decimal, nil unless Initialized
5. BuyingPowerUSD *Decimal, the balance floored at zero, nil unless Initialized
6. IntegrityErrors []CashIntegrityError

The balance is the sum of the cash ledger from the opening balance date through Date. Not initialized means no balance, equity or buying power is reported, never a zero.

## CashIntegrityError

1. FirstNegativeDate Date
2. Balance Decimal, the running balance at that entry

Reported when the running cash balance is below zero at any entry. Never rejected at write time, because transactions were recorded before cash existed, and never omitted from results.

## CashSnapshot

1. Date Date
2. BalanceUSD Decimal

A stored daily cash balance, one per snapshot date. Written with the securities snapshot of the same date; see Store.WriteSnapshot.

# Functions

## CashTransaction.Validate() error

1. if kind is not one of the Cash constants, return ErrInvalidInput.
2. if amount is empty or does not parse, return ErrInvalidInput.
3. if executed at or effective date is zero, return ErrInvalidInput.
4. if notes exceed maxNotesLength, return ErrInvalidInput.
5. Check the sign by kind, returning ErrInvalidInput naming the amount:
   1. OPENING_BALANCE: any sign, including zero.
   2. DEPOSIT and INTEREST: positive.
   3. WITHDRAWAL and FEE: negative.
6. Return nil.

Mirrors the cash_transactions CHECK constraint. The opening balance being unique is enforced by the database, not here.

## CashTransactionFilter.Normalize() (CashTransactionFilter, error)

1. Apply the default and the cap to the limit through resolveLimit.
2. Return the filter.

#### Errors

- **1.** if the limit is negative, return ErrInvalidInput.
- **1.1.** if From is after To, return ErrInvalidInput.
- **1.2.** if Kind is set and not one of the Cash constants, return ErrInvalidInput.

## CashKind.valid() bool

Return whether the value is one of the Cash constants.
