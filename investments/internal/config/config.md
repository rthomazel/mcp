# Constants

defaultQueryTimeout = 30 seconds.
defaultStalePriceDays = 5, from the spec: a price five or more days older than the reference date is unusable.

# Types

## Config

1. DSN string
2. QueryTimeout Duration
3. StalePriceDays int
4. TiingoAPIKey string, the market data credential, a server-side secret
5. NotificationsConfigFile string, the path of the notifications configuration YAML file, empty meaning no file

# Functions

## Load() (*Config, error)

1. Read INVESTMENTS_MCP_DSN.
2. Read INVESTMENTS_MCP_QUERY_TIMEOUT as a duration, defaulting to defaultQueryTimeout.
3. Read INVESTMENTS_MCP_STALE_PRICE_DAYS as an integer, defaulting to defaultStalePriceDays.
4. Read INVESTMENTS_MCP_TIINGO_API_KEY.
5. Read INVESTMENTS_MCP_NOTIFICATIONS_CONFIG_FILE, optional.
6. Return the populated Config.

#### Errors

- **1.** if the DSN is empty, return an error naming the variable.

---

- **2.** if the timeout does not parse or is not positive, return the wrapped error.

---

- **3.** if the integer does not parse or is below 1, return the wrapped error.

---

- **4.** if the API key is empty, return an error naming the variable.

The DSN points at the dedicated investments database on LGA Postgres, using the role that has no access to other databases.
A missing notifications file, or an unset path, is not an error: notifications are then unconfigured and the system runs without them. The Telegram bot credential is not in Config: the notifications file names the environment variable that holds it, the channel reads it, and it is never logged, returned or put in an event.
The API key is never logged, never put in a URL and never returned to a client. It supports rotation by restarting the server with a new value. A later stage adds the schedule.
Every variable is listed in .env.example, which is configuration and not modelled; the compose service reads the real values from a .env next to it.
