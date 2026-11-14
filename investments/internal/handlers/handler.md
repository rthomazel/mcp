# Constants

longTimeout = 3 minutes, the deadline for tools that call the provider or rewrite many rows: add_instrument, refresh_prices and rebuild_snapshots.
internalErrorText = "internal error, see the server logs".

# Types

## Handler

1. store domain.Store
2. refresher *marketdata.Refresher
3. queryTimeout Duration, from the configuration, the deadline for every other tool

The MCP tool layer. It holds no state beyond its dependencies and depends on domain.Store, never on the Postgres implementation.

# Functions

## New(store domain.Store, refresher *marketdata.Refresher, queryTimeout Duration) *Handler

Build a Handler.

## Handler.Register(server *MCPServer)

Add the eleven tools to server, each with the description, parameters and hints its model gives, bound to its handler method. Descriptions follow the sibling bench server: plain dense sentences without markdown that lead with exactly what the tool does, state the return shape, say which failures leave nothing written, and point to the related tool for the next step. Parameter descriptions give the format and what absence means, start with Optional when they are, and use schema constraints such as enums and minimums instead of prose. Read tools, including list_updates, carry the read-only hint; rebuild_snapshots carries the destructive hint; refresh_prices and rebuild_snapshots carry the idempotent hint.

## Handler.resolveInstrument(ctx Context, symbol string, exchange string) (domain.Instrument, error)

1. List the instruments through store.ListInstruments.
2. Keep those whose symbol equals the normalized symbol, and whose exchange equals the normalized exchange when one is given.
3. if exactly one remains, return it.

#### Errors

- **1.** return the wrapped error.

---

- **3.** if none remain, return ErrInstrumentNotFound naming the symbol.
- **3.1.** if several remain, return ErrInvalidInput naming the exchanges and asking for one.

The registry is small, so resolving in memory avoids a new store method. Callers refer to instruments by symbol, never by internal id.

## Handler.resolveOptional(ctx Context, symbol string, exchange string) (*int, error)

Return nil when symbol is empty, otherwise the id of resolveInstrument. Used by the list tools, where the instrument is a filter.

## parseDate(name string, value string) (*domain.Date, error)

1. if value is empty, return nil.
2. Parse it as YYYY-MM-DD in UTC; if that fails, return ErrInvalidInput naming the parameter and the expected format.

## parseTimestamp(name string, value string) (Time, error)

Parse RFC 3339; if that fails, return ErrInvalidInput naming the parameter. An empty value is an error.

## jsonResult(value any) (*CallToolResult, error)

Marshal value to indented JSON text and return it as the tool result. A marshalling failure is logged and returned as the internal error text.

## Handler.failure(ctx Context, tool string, err error) *CallToolResult

1. if err is ErrInvalidInput, ErrInstrumentNotFound, ErrInstrumentExists, a negative balance, ErrSnapshotIncomplete, ErrSnapshotExists, ErrSymbolNotFound or ErrRateLimited, return a tool error carrying err's message; these are the caller's to act on and are not logged.
2. if the context deadline was exceeded, log at warn level and return a tool error saying the call timed out.
3. otherwise log at error level with the tool name and the error, and return a tool error with internalErrorText.

Tool errors are results with the error flag set, not Go errors, so the agent sees and can react to them. Messages never carry the provider credential: it travels only in a request header and no error built from a response includes request headers.

## Handler.withTimeout(ctx Context, long bool) (Context, CancelFunc)

Derive ctx with longTimeout when long, queryTimeout otherwise.
