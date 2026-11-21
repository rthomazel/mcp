---
id: 2026-11-15-status
type: spec
summary: Aggregated health view of price freshness, snapshots and unresolved failures, beside the raw updates log.
author: Thom
created: 2026-11-15
updated: 2026-11-21
agents: claude
---

# Status

## Intent

Answer whether the data is healthy without reading the raw updates log: did the refresh run, why is a
snapshot missing, which holdings have stale prices.

## Behavior

A status view answers whether the data is healthy, beyond the raw updates log. It reports,
for each held instrument, its latest stored price date and whether it is missing or unexpectedly
old; the latest completed snapshot date and whether the latest price date has no completed
snapshot; and the failed or incomplete runs that still need attention. A run needs
attention until a later completed run of the same kind supersedes it: for a snapshot run, a completed run for the same date; for a rebuild, a later completed rebuild whose range covers it. The view gives one overall verdict, healthy or needs attention, with reasons.
It also reports outbox events that were not handled, as specified in the notifications feature: events given up on within the last week need attention, and older unhandled events are listed separately as expired, without affecting the verdict. The raw updates log stays available.

## Acceptance criteria

- Status flags a held instrument with a missing or five-day-old price and a latest price date with no completed snapshot, lists unresolved failed or incomplete runs, and clears an item once a later run supersedes it.

## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
- Related: 2026-11-21-notifications, whose given-up and expired events status lists.
