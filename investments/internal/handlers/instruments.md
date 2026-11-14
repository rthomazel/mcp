# Functions

## Handler.HandleRegisterInstrument(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool register_instrument. Registers a USD-quoted instrument and backfills its full price history.

Description shown to the agent: Registers a USD-quoted stock, ETF or fund by symbol and exchange, then downloads its full daily price history from the market data provider. Registration is rejected, with nothing created, when the symbol is not covered by the provider. Use it before recording transactions for a new holding. Errors returned: invalid input (missing parameter, unknown instrument type, currency other than USD, or a symbol the provider does not cover); instrument already registered (the symbol and exchange pair exists); provider rate limited (retry later); instrument registered but price backfill failed (call refresh_prices to retry); timed out.

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

Description shown to the agent: Lists every registered instrument with its symbol, exchange, name, type and currency, ordered by symbol. Call it to learn which symbols exist and whether a symbol needs an exchange to be unambiguous. Errors returned: internal error only.

#### Errors

- if listing fails, return the failure result.
