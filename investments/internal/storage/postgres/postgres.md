# Constants

pingAttempts = 5.
pingBackoff = 10 seconds, the database may not be reachable immediately on startup.
marketTimezone = "America/New_York", used to derive today's date when no date is given.
postgresUniqueViolation = "23505".
postgresCheckViolation = "23514".

# Types

## Store

1. db *sqlx.DB
2. stalePriceDays int
3. marketLocation *Location, the loaded marketTimezone

The one flat struct implementing domain.Store, with a compile-time assertion. Its methods are spread across instrument.go, transaction.go, positions.go and valuation.go by concern.

# Functions

## Open(dsn string, stalePriceDays int) (*Store, func() error, error)

1. Parse the DSN with pgx.
2. Open a database/sql handle over the pgx stdlib connector and wrap it with sqlx.
3. Ping, retrying up to pingAttempts with pingBackoff between attempts.
4. Load marketTimezone.
5. Return the Store and a cleanup function that closes the handle.

#### Errors

- **1.** if parsing fails, return the wrapped error.

---

- **3.** if the last attempt fails, close the handle and return the wrapped error naming the attempt count.

---

- **4.** if the zone cannot be loaded, close the handle and return the wrapped error. Startup fails rather than failing on the first valuation.

Zone data is embedded in the binary by the time/tzdata import in main, so loading never depends on the host or the runtime image having tzdata installed.

## Store.DB() *sql.DB

Return the underlying database/sql handle, used by the migration runner so it connects exactly as the server does.

## isUniqueViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresUniqueViolation.

## isCheckViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresCheckViolation.
