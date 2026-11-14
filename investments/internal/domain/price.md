# Types

## PriceBar

1. Date Date
2. Open Decimal
3. High Decimal
4. Low Decimal
5. Close Decimal, the real unadjusted close, the only price used to value holdings
6. AdjustedClose Decimal, kept for future total-return analysis
7. Volume int64, zero for mutual funds

A provider-independent daily price record for one instrument. It carries no instrument identity; Store.UpsertPrices attaches it.

## PriceFilter

1. InstrumentID int
2. From *Date, inclusive
3. To *Date, inclusive
4. Limit int, zero means defaultListLimit, capped at maxListLimit

Selects stored price records of one instrument.

# Interfaces

## PriceProvider

1. Name() string
2. FetchPrices(ctx Context, symbol string, from *Date) (bars []PriceBar, err error)

The provider-independent boundary to market data. Implemented by internal/provider/tiingo; replacing the provider touches nothing else.

### Name() string

A short stable provider name, used by the updates log in a later stage.

### FetchPrices(ctx Context, symbol string, from *Date) (bars []PriceBar, err error)

Returns daily bars for symbol, oldest first with strictly ascending dates, from the date inclusive through the latest completed session, or the full available history when from is nil. The symbol is the instrument's symbol as registered; the provider adapts it to its own format. Every bar has passed PriceBar.Validate. An empty slice means no bars in range and is not an error. Returns ErrSymbolNotFound when the provider does not cover the symbol and ErrRateLimited when it throttles the request.

# Functions

## PriceFilter.Validate() error

1. if InstrumentID is not positive, a limit is negative, or From is after To, return ErrInvalidInput.
2. Return nil.


## PriceBar.Validate() error

1. if date is zero, return ErrInvalidInput.
2. if any price does not parse as a decimal, return ErrInvalidInput naming the field.
3. if close is not positive, return ErrInvalidInput.
4. if volume is negative, return ErrInvalidInput.
5. Return nil.

Mirrors the prices CHECK constraint on close. A bad price is rejected rather than skipped or zeroed, so a gap is never filled silently.
