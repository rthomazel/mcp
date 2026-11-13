# Constants

insertInstrumentSQL = insert an instruments row returning the full row.
listInstrumentsSQL = select all instruments ordered by symbol, then exchange.

# Types

## Store

1. pool *pgxpool.Pool

Owns the instrument registry and the transaction ledger. Shared by instrument.go and transaction.go.

# Functions

## NewStore(pool *pgxpool.Pool) *Store

Build a Store around the pool.

## Store.RegisterInstrument(ctx Context, in NewInstrument) (Instrument, error)

1. Normalize and validate in.
2. Insert via insertInstrumentSQL and scan the result.
3. Log the registration at info level with the instrument id, symbol and exchange.
4. Return the instrument.

#### Errors

- **1.** if validation fails, return the ErrInvalidInput error.

---

- **2.** if the insert violates the symbol and exchange uniqueness, return ErrInstrumentExists naming the pair.
- **2.1.** if the currency check fires despite validation, return ErrInvalidInput.
- **2.2.** otherwise return the wrapped error.

## Store.ListInstruments(ctx Context) ([]Instrument, error)

Select via listInstrumentsSQL and return all rows; an empty registry returns an empty slice, not nil.
