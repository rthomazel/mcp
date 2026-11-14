# Interfaces

## Store

1. GetPositions(ctx Context, date *Date) (PositionsResult, error)
2. GetPortfolio(ctx Context, date *Date) (Portfolio, error)

The surface consumers depend on, so the tool handlers can be tested against a mockery mock. Implemented by PostgresStore.

# Types

## PostgresStore

1. pool *pgxpool.Pool
2. stalePriceDays int
3. marketLocation *Location, the loaded marketTimezone

One flat struct implementing Store; its methods are spread across positions.go and valuation.go by concern.

# Functions

## NewPostgresStore(pool *pgxpool.Pool, stalePriceDays int) (*PostgresStore, error)

1. Load marketTimezone.
2. Build a PostgresStore around the pool, the threshold and the location.
3. Return it.

#### Errors

- **1.** if the zone cannot be loaded, return the wrapped error. Startup fails rather than failing on the first valuation.

Zone data is embedded in the binary by the time/tzdata import in main, so loading never depends on the host or the runtime image having tzdata installed.
