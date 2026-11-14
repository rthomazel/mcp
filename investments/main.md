# Vars

## version = "local"

Set at build time through linker flags.

# Functions

## main()

Run run(); on error print the fatal message to stderr and exit with status 1.

## run() error

1. Install a text slog handler on stderr at info level.
2. Load the configuration with internal.LoadConfig.
3. Apply pending migrations with storage.Migrate.
4. Open the connection pool with storage.Connect, defer closing it.
5. Construct ledger.NewPostgresStore from the pool and portfolio.NewPostgresStore from the pool and the stale price threshold.
6. Build the MCP server named "investments-mcp" with version.
7. Serve over stdio.

#### Errors

- **2.** if loading fails, return the wrapped error.

---

- **3.** if migrating fails, return the wrapped error.

---

- **4.** if connecting fails, return the wrapped error.

---

- **5.** if constructing the portfolio store fails, return the wrapped error.

Tools are registered in a later stage; until then the server starts with none.
The binary embeds the timezone database through the time/tzdata import, so zone lookups do not depend on the host or the runtime image.
The container wraps the stdio server with mcp-proxy, as the sibling servers do.
Migrations run on every start, before the pool opens, so the schema is current before any tool is served.
