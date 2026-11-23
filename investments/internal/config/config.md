# Types

## Config

1. DSN string
2. QueryTimeout Duration
3. StalePriceDays int
4. TiingoAPIKey string, the market data credential, a server-side secret
5. NotificationsConfigFile string, the path of the notifications configuration YAML file, empty meaning no file
6. MonitorsConfigFile string, the path of the monitors configuration YAML file, empty meaning no file, so no monitor runs
7. TelegramBotToken string, the Telegram bot credential, a server-side secret, empty meaning Telegram is unconfigured
8. MarketTimezone string, the IANA zone of the market day, shared by the price refresh job, the monitors and today's date
9. PriceRefreshRunTimes []domain.ClockTime, the times of day the price refresh job runs, ascending, in the market timezone
10. MonitorsRunTime domain.ClockTime, the time of day every monitor runs, in the market timezone
11. MaxHistoryPoints int, the most points one value history query returns; a longer series is rejected with a hint to use a larger window or a narrower range, never silently truncated
12. EventMaxFailures int, the failed attempts after which an event is given up on
13. EventBackoffBase Duration, the delay after the first failure; each further failure doubles it
14. EventMaxAge Duration, how long after it was recorded an event is still considered for handling
15. EventRetention Duration, how long after it was fully handled an event is kept
16. WorkerPollInterval Duration, how often the events worker looks for due events
17. WorkerPruneInterval Duration, how often it prunes handled events
18. WorkerBatchLimit int, the events taken per pass
19. WorkerHandlerTimeout Duration, the deadline for one handler call
20. MonitorTimeout Duration, the deadline for one monitor run

Every tunable has a default, so only the DSN and the API key are required. The defaults live here and nowhere else; the packages that use a value take it as a parameter and hold no default.

# Functions

## Defaults() Config

Return the configuration with every default and no required value:

- QueryTimeout 30 seconds.
- StalePriceDays 5, from the spec: a price five or more days older than the reference date is unusable.
- MarketTimezone America/New_York.
- PriceRefreshRunTimes 06:00 and 20:30, MonitorsRunTime 07:00: the monitors run an hour after the morning refresh.
- MaxHistoryPoints 1000.
- EventMaxFailures 10, EventBackoffBase 30 seconds, EventMaxAge 7 days, EventRetention 1 year.
- WorkerPollInterval 30 seconds, twice per minute, WorkerPruneInterval 24 hours, WorkerBatchLimit 100, WorkerHandlerTimeout 30 seconds.
- MonitorTimeout 2 minutes.

Tests use it to get the same values the server runs with.

## Load() (*Config, error)

1. Start from Defaults.
2. Read INVESTMENTS_MCP_DSN and INVESTMENTS_MCP_TIINGO_API_KEY.
3. Read INVESTMENTS_MCP_NOTIFICATIONS_CONFIG_FILE, INVESTMENTS_MCP_MONITORS_CONFIG_FILE and INVESTMENTS_MCP_TELEGRAM_BOT_TOKEN, optional.
4. Read INVESTMENTS_MCP_MARKET_TIMEZONE, optional.
5. Read INVESTMENTS_MCP_PRICE_REFRESH_RUN_TIMES, a comma separated list of HH:MM times, and INVESTMENTS_MCP_MONITORS_RUN_TIME, one HH:MM time, both optional, with domain.ParseClockTimes and ParseClockTime.
6. Read each duration variable, optional, overriding its default: INVESTMENTS_MCP_QUERY_TIMEOUT, _EVENT_BACKOFF_BASE, _EVENT_MAX_AGE, _EVENT_RETENTION, _WORKER_POLL_INTERVAL, _WORKER_PRUNE_INTERVAL, _WORKER_HANDLER_TIMEOUT and _MONITOR_TIMEOUT.
7. Read each integer variable, optional, overriding its default: INVESTMENTS_MCP_STALE_PRICE_DAYS, _MAX_HISTORY_POINTS, _EVENT_MAX_FAILURES and _WORKER_BATCH_LIMIT.
8. Return the populated Config.

#### Errors

- **1.** if the DSN is empty, return an error naming the variable.

---

- **2.** if the API key is empty, return an error naming the variable.

---

- **5.** if a run time does not parse, or the monitors variable has more than one time, return the wrapped error naming the variable.

---

- **6.** if a duration does not parse or is not positive, return the wrapped error naming the variable.

---

- **7.** if an integer does not parse or is below 1, return the wrapped error naming the variable.

The DSN points at the dedicated investments database on LGA Postgres, using the role that has no access to other databases.
A missing notifications file, or an unset path, is not an error: notifications are then unconfigured and the system runs without them. The Telegram bot credential is read here from the environment and handed to the notification source; it is never in the notifications file, and never logged, returned or put in an event. The run times are server configuration and not part of the monitors file, so a monitor time never changes with an edit and needs a restart. Nothing checks that the monitors run after the morning price refresh; the defaults keep them an hour apart and moving one past the other is the operator's choice. The market timezone is not validated here: whoever loads it, the store and the schedulers, returns the error for an unknown zone, so the server fails at startup.
The API key is never logged, never put in a URL and never returned to a client. It supports rotation by restarting the server with a new value. A later stage adds the schedule.
Every variable is listed in .env.example, which is configuration and not modelled; the compose service reads the real values from a .env next to it.
