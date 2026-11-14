# Functions

## Handler.HandleAddInstrument(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool add_instrument. Registers a USD-quoted instrument and backfills its full price history, through refresher.RegisterInstrument.

Description shown to the agent: Register a USD-quoted stock, ETF or fund by symbol and exchange, then download its full daily price history from the provider. Returns the instrument. Fails without registering when the symbol is not covered by the provider, the currency is not USD, or the symbol and exchange pair already exists. If the instrument registers but the price download fails, it stays registered and the error says so; refresh_prices retries the download. Call this before add_transaction for a new holding.

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

Description shown to the agent: List every registered instrument with symbol, exchange, name, type and currency, ordered by symbol. Call this to learn which symbols exist and which need an exchange to be unambiguous.

#### Errors

- if listing fails, return the failure result.
