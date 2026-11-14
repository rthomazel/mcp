# Vars

## ErrInvalidInput = "invalid input"

Wrapped with the specific reason for any validation failure, so callers match with errors.Is and show the detail.

## ErrInstrumentNotFound = "instrument not found"

## ErrInstrumentExists = "instrument already registered"

Returned when the symbol and exchange pair already exists.

## ErrSymbolNotFound = "symbol not found"

Returned by a PriceProvider that does not cover a symbol.

## ErrRateLimited = "provider rate limited"

Returned by a PriceProvider that throttles a request. Callers stop and retry later.

## ErrNegativeBalance = "transaction would make ownership negative"

Matched through NegativeBalanceError.

# Types

## NegativeBalanceError

1. InstrumentID int
2. OnDate Date, the effective date of the first ledger entry at which ownership goes negative
3. Quantity Decimal, the running quantity at that entry

Is(ErrNegativeBalance) is true.
