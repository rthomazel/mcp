# Constants

upsertPricesSQL = insert prices rows for one instrument from a JSON array expanded in SQL with jsonb_to_recordset, ON CONFLICT (instrument_id, date) DO UPDATE every price column.
latestPriceDateSQL = select the maximum date from prices for one instrument, null if none.

# Functions

## Store.UpsertPrices(ctx Context, instrumentID int, bars []domain.PriceBar) error

1. if bars is empty, return nil.
2. Validate every bar with PriceBar.Validate and check that no date repeats.
3. Marshal the bars to a JSON array with every decimal as a string.
4. Execute upsertPricesSQL with the instrument id and the JSON, as one statement.
5. Log the write at info level with the instrument id and the number of bars.
6. Return nil.

#### Errors

- **2.** if a bar is invalid, return the domain.ErrInvalidInput error naming its date.
- **2.1.** if a date repeats, return domain.ErrInvalidInput naming it.

---

- **3.** if marshalling fails, return the wrapped error.

---

- **4.** if the instrument does not exist (foreign key violation), return domain.ErrInstrumentNotFound.
- **4.1.** if a check constraint fails, return domain.ErrInvalidInput.
- **4.2.** otherwise return the wrapped error.

One statement makes the write atomic without an explicit transaction. Passing decimals as JSON strings keeps their exact text, and a repeated date is rejected up front because ON CONFLICT DO UPDATE cannot touch the same row twice in one statement.
A full history is a few thousand rows, small enough for one statement.

## Store.LatestPriceDate(ctx Context, instrumentID int) (*domain.Date, error)

1. Select via latestPriceDateSQL.
2. Return the date, or nil when there is none.

#### Errors

- **1.** if the query fails, return the wrapped error.

An unknown instrument has no prices and returns nil, not an error.
