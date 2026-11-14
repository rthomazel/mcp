# investments-mcp

Personal investment monitoring MCP server: instrument registry, transaction ledger, positions and securities valuation, backed by its own PostgreSQL database. Go.

Specification and plan: [`spc/investments-mcp/`](./spc/investments-mcp/spec.md). Models sit next to the code they describe. No MCP tools are registered yet; stage 6 adds them.

## Develop

```bash
investments/run dev     # reads .env, see .env.example
investments/run mocks   # generate mocks before test and lint
investments/run test    # set INVESTMENTS_MCP_TEST_DSN to include the database tests
```

`INVESTMENTS_MCP_TEST_DSN` is an admin DSN, for example `postgres://user:pass@localhost:5432/postgres`. Each test creates and drops its own database, so the role needs `CREATEDB`.
