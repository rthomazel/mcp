# Vars

## version = "local"

Set at build time through linker flags.

# Functions

## main()

Run run(); on error print the fatal message to stderr and exit with status 1.

## run() error

1. Install a text slog handler on stderr at info level.
2. Load the configuration with config.Load.
3. Open the store with postgres.Open, defer its cleanup.
4. Apply pending migrations with migrate.
5. Build the Tiingo client, the refresher and the daily job, and log at info level that the daily job is enabled with its run times and provider, and start the scheduler in a goroutine that stops when the process shuts down.
6. Build the MCP server named "investments-mcp" with version.
7. Serve over stdio.

#### Errors

- **2.** if loading fails, return the wrapped error.

---

- **3.** if opening fails, return the wrapped error.

---

- **4.** if migrating fails, return the wrapped error.

The job and its scheduler live inside the server process, per the spec. Tools are registered in a later stage; until then the server starts with none.
The binary embeds the timezone database through the time/tzdata import, so zone lookups do not depend on the host or the runtime image.
The container wraps the stdio server with mcp-proxy, as the sibling servers do.
Migrations run on every start, before any tool is served, so the schema is current.
The store is the only dependency handed to later layers, as the domain.Store interface.
