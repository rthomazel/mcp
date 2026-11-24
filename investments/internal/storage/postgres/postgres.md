# Constants

pingAttempts = 5.
pingBackoff = 10 seconds, the database may not be reachable immediately on startup.
postgresUniqueViolation = "23505".
postgresForeignKeyViolation = "23503".
postgresCheckViolation = "23514".

# Types

## Settings

1. StalePriceDays int
2. MarketTimezone string
3. MaxHistoryPoints int
4. Events domain.EventPolicy

The store's tunables, all from the server configuration; the store holds no defaults.

## Store

1. db *sqlx.DB
2. stalePriceDays int
3. marketLocation *Location, the loaded market timezone, used to derive today's date when no date is given
4. maxHistoryPoints int, the most points one value history query returns
5. events domain.EventPolicy
6. clock func() Time, the time source for the event queries and status; the real clock, which tests replace

The generic store, implementing domain.GenericStore, and the core every other store shares: the connection and the settings. Its methods are spread across instrument.go, transaction.go, positions.go, valuation.go, history.go and status.go by concern.

## Cash, Prices, Snapshots, Updates, Events, Monitors

1. Store *Store, embedded

One small type per narrower store, each implementing the domain interface of the same name with a compile-time assertion. They share the Store's connection and settings through the embedded pointer, so a method that needs a shared helper, such as a transaction or the reference date, calls it directly. The embedding means the narrower types also carry the generic store's methods; consumers hold the domain interfaces, which hide them. The methods are in cash.go, price.go, snapshot.go, event.go; the updates log lives in snapshot.go. Monitors is in history.go, allocation.go and volatility.go.

## Stores

1. Store *Store, embedded, so Stores itself implements domain.GenericStore and exposes DB
2. Cash *Cash
3. Prices *Prices
4. Snapshots *Snapshots
5. Updates *Updates
6. Events *Events
7. Monitors *Monitors

Every store over one connection, built by Open.

## Stores.Domain() domain.Stores

Return the stores as the domain bundle the tool handlers take.

# Functions

## Open(dsn string, settings Settings) (*Stores, func() error, error)

1. Parse the DSN with pgx.
2. Open a database/sql handle over the pgx stdlib connector and wrap it with sqlx.
3. Ping, retrying up to pingAttempts with pingBackoff between attempts.
4. Load settings.MarketTimezone.
5. Set the clock to the real one.
6. Build the Store and the narrower stores over it, and return them as Stores with a cleanup function that closes the handle.

#### Errors

- **1.** if parsing fails, return the wrapped error.

---

- **3.** if the last attempt fails, close the handle and return the wrapped error naming the attempt count.

---

- **4.** if the zone cannot be loaded, close the handle and return the wrapped error. Startup fails rather than failing on the first valuation.

Zone data is embedded in the binary by the time/tzdata import in main, so loading never depends on the host or the runtime image having tzdata installed.

## Stores.DB() *sql.DB

Return the underlying database/sql handle, used by the migration runner so it connects exactly as the server does.

## isUniqueViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresUniqueViolation.

## isForeignKeyViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresForeignKeyViolation.

## isCheckViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresCheckViolation.
