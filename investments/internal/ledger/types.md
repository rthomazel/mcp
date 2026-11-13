# Constants

EventBuy = "BUY"
EventSell = "SELL"
EventDividend = "DIVIDEND"
EventTax = "TAX"
EventSplit = "SPLIT"
EventAdjustment = "ADJUSTMENT"

InstrumentStock = "STOCK"
InstrumentETF = "ETF"
InstrumentFund = "FUND"
InstrumentOther = "OTHER"

currencyUSD = "USD", the only supported currency.
maxNotesLength = 1000 characters.

# Vars

## ErrInvalidInput = "invalid input"

Wrapped with the specific reason for any validation failure, so callers match with errors.Is and show the detail.

## ErrInstrumentNotFound = "instrument not found"

## ErrInstrumentExists = "instrument already registered"

Returned when the symbol and exchange pair already exists.

## ErrNegativeBalance = "transaction would make ownership negative"

Matched through NegativeBalanceError.

# Types

## Decimal

A string holding a plain decimal number: optional minus sign, digits, optional fractional digits. No exponent, no separators, no fractions.
Go never does arithmetic on decimals; it only checks sign and passes them to and from Postgres NUMERIC.

## Date

A calendar date, a time.Time at midnight UTC. Rendered as "YYYY-MM-DD" at the API edge.

## EventType

A string, one of the Event constants.

## Instrument

1. ID int
2. Symbol string
3. Exchange string
4. Name string
5. Currency string
6. InstrumentType string

## NewInstrument

1. Symbol string
2. Exchange string
3. Name string
4. Currency string
5. InstrumentType string

## Transaction

1. ID int64
2. InstrumentID int
3. EventType EventType
4. QuantityChange Decimal
5. CashAmount Decimal
6. ExecutedAt Time
7. EffectiveDate Date
8. Notes *string

## NewTransaction

1. InstrumentID int
2. EventType EventType
3. QuantityChange Decimal
4. CashAmount Decimal, empty means zero, only allowed to be empty for events where cash is optional or zero
5. ExecutedAt Time
6. EffectiveDate Date, required
7. Notes *string

Effective date is supplied by the caller. There is no market calendar, so an event executed after the close must be given the next session's date by whoever records it.

## NegativeBalanceError

1. InstrumentID int
2. OnDate Date, the effective date of the first ledger entry at which ownership goes negative
3. Quantity Decimal, the running quantity at that entry

Is(ErrNegativeBalance) is true.

# Functions

## ParseDecimal(s string) (Decimal, error)

1. Trim spaces.
2. Check it matches an optional minus, one or more digits, and optionally a dot with one or more digits.
3. Return it as a Decimal.

#### Errors

- **2.** if it does not match, return ErrInvalidInput naming the value.

## Decimal.Sign() int

Return -1, 0 or 1 by inspecting the text: zero if every digit is zero, negative if it starts with a minus.

## NewInstrument.Normalize() NewInstrument

Trim all fields; upper-case symbol, exchange, currency and instrument type.

## NewInstrument.Validate() error

1. if symbol, exchange or name is empty, return ErrInvalidInput.
2. if currency is not currencyUSD, return ErrInvalidInput.
3. if instrument type is not one of the Instrument constants, return ErrInvalidInput.
4. Return nil.

Call Normalize first; Validate does not normalize.

## NewTransaction.Validate() error

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
