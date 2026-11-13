---
id: 2026-11-13-financial-automation-hub
type: idea
summary: Grow the Investments MCP into the single home for personal financial automation.
author: Thom
created: 2026-11-13
agents: claude
---

# Financial automation hub

## Status

Idea only, not part of v1.

## Intent

Make this server the one place for the operator's financial automation. Two further
systems are expected to join the USD portfolio: the Brazilian portfolio and the
AMS trading wallet, each described in its own idea.

## Direction

The Brazilian portfolio stays separate from the USD one rather than mixing data.
What the systems share is design, reused where it fits.

## Constraint

Only consolidate if it does not make the design convoluted. The v1 principle of
keeping the system small stays in force: if folding a system in forces the shared
core to bend around it, keep it separate or give it its own bounded area instead of
distorting the model.

## To be decided

- Whether consolidation means one server with distinct areas, or several servers
  sharing conventions, access and notifications. Databases are already separate
  per system on the shared LGA Postgres server.
- What the systems truly share: the ledger concepts, price and valuation handling,
  the ingestion job, the access path through the gateway, and notifications.
- Whether the name and scope of the project should change if it grows beyond investments.

## Related

- 2026-11-13-brazil-portfolio
- 2026-11-13-ams-wallet
- 2026-11-13-notifications
