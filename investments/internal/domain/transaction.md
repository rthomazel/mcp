# Constants

EventBuy = "BUY"
EventSell = "SELL"
EventDividend = "DIVIDEND"
EventTax = "TAX"
EventSplit = "SPLIT"
EventAdjustment = "ADJUSTMENT"

maxNotesLength = 1000 characters.
defaultListLimit = 100.
maxListLimit = 500.

# Types

## EventType

A string, one of the Event constants.

## Transaction

1. ID int64
2. InstrumentID int
3. EventType EventType
4. QuantityChange Decimal
5. CashAmount Decimal
6. ExecutedAt Time
7. EffectiveDate Date
8. Notes *string

## DraftTransaction

1. InstrumentID int
2. EventType EventType
3. QuantityChange Decimal
4. CashAmount Decimal, empty means zero, only allowed to be empty for events where cash is optional or zero
5. ExecutedAt Time
6. EffectiveDate Date, required
7. Notes *string

A draft is caller input that is not yet saved. It carries no server-assigned fields and, unlike a stored Transaction, allows an empty cash amount meaning zero. It is normalized and validated before it touches the database.
Effective date is supplied by the caller. There is no market calendar, so an event executed after the close must be given the next session's date by whoever records it.

## RecordResult

1. Transaction Transaction
2. RebuildFrom *Date, set when the new entry changed quantity on or before the latest existing snapshot date; snapshots from that date onward are now stale and need an explicit rebuild.

## TransactionFilter

1. InstrumentID *int
2. From *Date, inclusive effective date
3. To *Date, inclusive effective date
4. Limit int, zero means defaultListLimit, capped at maxListLimit
5. EventType EventType, empty for any

# Functions

## DraftTransaction.Validate() error

1. if event type is not one of the Event constants, return ErrInvalidInput.
2. if quantity change or non-empty cash amount does not parse, return ErrInvalidInput.
3. if executed at is zero or effective date is zero, return ErrInvalidInput.
4. if notes exceed maxNotesLength, return ErrInvalidInput.
5. Check the signs by event type, returning ErrInvalidInput naming the offending field:
   1. BUY: quantity positive, cash negative.
   2. SELL: quantity negative, cash positive.
   3. DIVIDEND: quantity zero, cash positive.
   4. TAX: quantity zero, cash negative.
   5. SPLIT: quantity non-zero, cash zero.
   6. ADJUSTMENT: quantity non-zero, cash any.
6. Return nil.

An empty cash amount counts as zero, so it fails events that require a non-zero cash amount.
These rules mirror the transactions CHECK constraint in the initial migration.

## TransactionFilter.Normalize() (TransactionFilter, error)

1. Apply the default and the cap to the limit.
2. Return the filter.

#### Errors

- **1.** if the limit is negative, return ErrInvalidInput.
- **1.1.** if From is after To, return ErrInvalidInput.
- **1.2.** if EventType is set and not one of the Event constants, return ErrInvalidInput.
