# Functions

## Handler.HandleRegisterInstrument(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool register_instrument. Registers a USD-quoted instrument and backfills its full price history.

- currency string, optional, default "USD"; any other value is rejected
- exchange string, required, listing exchange such as NYSE
- instrument_type string, required, one of STOCK, ETF, FUND, OTHER
- name string, required
- symbol string, required

1. Read the parameters; a missing required one is a tool error naming it.
2. Derive a long-timeout context.
3. Register through refresher.RegisterInstrument, which fetches the history first, registers, then stores the bars.
4. Return the instrument.

#### Errors

- **3.** if the instrument was registered but its backfill failed, return a tool error saying so and that refresh_prices retries the backfill.
- **3.1.** otherwise return the failure result.

## Handler.HandleListInstruments(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool list_instruments. No parameters. Returns every instrument ordered by symbol, then exchange.

#### Errors

- if listing fails, return the failure result.
