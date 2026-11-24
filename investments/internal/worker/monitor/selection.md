# Functions

## tickersOf(ctx Context, store domain.MonitorStore, selection Selection) ([]string, error)

1. if the selection is a list, return its tickers.
2. List the held tickers through store.ListHeldTickers and return them.

#### Errors

- **2.** if listing fails, return the wrapped error.

The held selection is read at each run, so an instrument bought later is watched from its next run and one sold is not.

## selectionFrom(params Params) (Selection, error)

1. Read the instruments setting with Params.Instruments.
2. if it is missing, return an error naming it.
3. Return it.

#### Errors

- **1.** return the error of Params.Instruments, for a word other than held, an empty list, a repeated ticker or a ticker that is not text.

---

- **2.** return an error naming the setting.
