# Constants

insertInstrumentSQL = insert an instruments row returning the full row.
listInstrumentsSQL = select all instruments ordered by symbol, then exchange.

# Functions

## PostgresStore.RegisterInstrument(ctx Context, draft Instrument) (Instrument, error)

1. Normalize and validate draft.
2. Insert via insertInstrumentSQL and scan the result.
3. Log the registration at info level with the instrument id, symbol and exchange.
4. Return the instrument.

#### Errors

- **1.** if validation fails, return the ErrInvalidInput error.

---

- **2.** if the insert violates the symbol and exchange uniqueness, return ErrInstrumentExists naming the pair.
- **2.1.** if the currency check fires despite validation, return ErrInvalidInput.
- **2.2.** otherwise return the wrapped error.

## PostgresStore.ListInstruments(ctx Context) ([]Instrument, error)

Select via listInstrumentsSQL and return all rows; an empty registry returns an empty slice, not nil.
