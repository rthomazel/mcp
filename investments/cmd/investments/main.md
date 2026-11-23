# Vars

## version = "local"

Set at build time through linker flags.

# Functions

## main()

Run run(); on error print the fatal message to stderr and exit with status 1.

## run() error

1. Install a text slog handler on stderr at info level.
2. Load the configuration with config.Load.
3. Open the stores with postgres.Open and the store settings taken from the configuration, defer its cleanup.
4. Apply pending migrations with migrate.
5. Build the Tiingo client, the refresher over the generic and price stores and the price refresh job over the price, snapshot and updates stores, and log at info level that the price refresh job is enabled with its run times and provider.
6. Build the notifications loader over the configured file, the notification handler source over it, the updates store and the Telegram bot token, and the events worker over the events store, the source and the worker settings from the configuration with the notification prefix as its type prefix, and start the worker in a goroutine that stops when the process shuts down.
7. Build one domain.Scheduler in the configured market timezone over the price refresh task and the task of each monitor, snapshot-incomplete, portfolio drawdown, portfolio ATH and portfolio movement, over the generic store and the events store with the monitor timeout, and start it in a goroutine that stops when the process shuts down.
8. Build the MCP server named "investments-mcp" with version.
9. Build the handlers over the stores bundle, the refresher and the query timeout, and register the tools on the server.
10. Serve over stdio.

#### Errors

- **2.** if loading fails, return the wrapped error.

---

- **3.** if opening fails, return the wrapped error.

---

- **4.** if migrating fails, return the wrapped error.

---

- **7.** if building the scheduler fails, return the wrapped error.

The job, the events worker and the monitors live inside the server process, per the specs. A notifications file that is missing or invalid never stops the server from starting; invalid is logged at error level on every worker pass. The refresher is shared by the job and the handlers.
The binary embeds the timezone database through the time/tzdata import, so zone lookups do not depend on the host or the runtime image.
The container wraps the stdio server with mcp-proxy, as the sibling servers do.
Migrations run on every start, before any tool is served, so the schema is current.
The stores are the only dependency handed to later layers, as the domain interfaces; each layer takes only the stores it uses.
