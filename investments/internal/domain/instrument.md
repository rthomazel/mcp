# Constants

InstrumentStock = "STOCK"
InstrumentETF = "ETF"
InstrumentFund = "FUND"
InstrumentOther = "OTHER"

currencyUSD = "USD", the only supported currency.

# Types

## Instrument

1. ID int
2. Symbol string
3. Exchange string
4. Name string
5. Currency string
6. InstrumentType string

An Instrument also serves as a draft that is not yet saved: its ID is ignored and assigned on insert. Implementations name such variables draft.

# Functions

## Instrument.Normalize() Instrument

Trim all fields; upper-case symbol, exchange, currency and instrument type.

## Instrument.Validate() error

1. if symbol, exchange or name is empty, return ErrInvalidInput.
2. if currency is not currencyUSD, return ErrInvalidInput.
3. if instrument type is not one of the Instrument constants, return ErrInvalidInput.
4. Return nil.

Call Normalize first; Validate does not normalize. Both ignore the ID.
