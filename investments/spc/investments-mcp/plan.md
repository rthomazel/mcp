---
id: 2026-11-13-investments-mcp-plan
type: plan
summary: Delivery stages and LGA rollout sequencing for the Investments MCP.
author: Thom
created: 2026-11-13
agents: claude
---

# Investments MCP plan

Stages follow the spec 2026-11-13-investments-mcp. Each stage is reviewed through
its models before code. Technical design lives in the models, not here.

## Stages

1. **Foundations.** Project structure for the new server in the mcp repository,
   its AGENTS.md constitution, database setup and migrations.
2. **Ledger.** Instrument registry and transaction ledger with long-only
   validation and safe concurrent writes.
3. **Positions and valuation.** Position reconstruction, securities valuation and
   allocation.
4. **Market data.** Provider integration behind the provider-independent boundary.
   Starts with a provider spike, which also settles price backfill (open questions 1 and 2).
5. **Ingestion and snapshots.** Daily job, idempotent snapshots, rebuilds and the
   updates log. Depends on stage 4.
6. **MCP surface.** Agent-facing capabilities, including instrument registration
   and rebuilds, and registration in LiteLLM for LibreChat.
7. **Hardening and release.** Tests, operational logging, container image
   publication.

## LGA rollout

1. Add the shared LGA Postgres server with persistent storage and
   secrets, create the investments database and its dedicated role, set up the
   daily per-database backup service, and verify them independently, including a
   test restore.
2. Publish the server image from the mcp repository.
3. Add the server service to the LGA stack on the container network only, with no published ports.
4. Run a manual refresh and one snapshot with a small real portfolio before
   enabling the daily schedule.
5. Register the server in LiteLLM, verify it from LibreChat, and run a supervised
   weekly review.

## Risks

- Provider limits, licensing or VPS reachability could force a provider change
  late; the provider boundary contains the impact.
