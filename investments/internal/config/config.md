# Constants

defaultQueryTimeout = 30 seconds.
defaultStalePriceDays = 5, from the spec: a price five or more days older than the reference date is unusable.

# Types

## Config

1. DSN string
2. QueryTimeout Duration
3. StalePriceDays int

# Functions

## Load() (*Config, error)

1. Read INVESTMENTS_MCP_DSN.
2. Read INVESTMENTS_MCP_QUERY_TIMEOUT as a duration, defaulting to defaultQueryTimeout.
3. Read INVESTMENTS_MCP_STALE_PRICE_DAYS as an integer, defaulting to defaultStalePriceDays.
4. Return the populated Config.

#### Errors

- **1.** if the DSN is empty, return an error naming the variable.

---

- **2.** if the timeout does not parse or is not positive, return the wrapped error.

---

- **3.** if the integer does not parse or is below 1, return the wrapped error.

The DSN points at the dedicated investments database on LGA Postgres, using the role that has no access to other databases.
Later stages add the provider key and the schedule.
Every variable is listed in .env.example, which is configuration and not modelled; the compose service reads the real values from a .env next to it.
