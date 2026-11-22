---
id: 2026-11-21-monitor-snapshot-incomplete
type: spec
summary: Monitor that notifies the operator when the latest price date still has no completed snapshot after the morning price refresh run.
author: Thom
created: 2026-11-21
agents: claude
---

# Monitor: latest price date without a snapshot

## Description

The first monitor, built on the monitor mechanism in the notifications spec. It
tells the operator when the price refresh job has not
produced a completed snapshot for the latest date that has price data.

## Behavior

The monitor runs at 7:00 AM Eastern, an hour after the last price refresh run of the day at
6:00 AM, which leaves time for that run to finish. It judges the latest date that has
price data, using the same definition as the status view: the date is unresolved when
it has no completed snapshot. A date with no active positions that was recorded
complete is resolved.

- If the date is unresolved, the monitor records a snapshot-incomplete event for that
  date, unless one already exists for it.
- If the date is resolved, or no price data exists yet, it records nothing.

Because the monitor judges state at 7:00 AM, the evening run alone never triggers it:
an evening run that is incomplete and is completed by the 6 AM run records nothing.
A date that stays unresolved is announced once, however many times it is judged.
An agent refresh or rebuild that completes the date before 7:00 AM also resolves it.

The notification handler for the event names the date. When the updates log has an
incomplete entry for it, the message names which instruments had which issue, with
symbols, dates and counts only. When it has none, the message says no run was recorded
for the date, which covers a price refresh job that did not run.

The monitor does not depend on there being two price refresh runs: if the job times change,
only the monitor's own time is retuned.

The monitor is named snapshot.incomplete in the monitors configuration and takes no
settings. It runs only when that file has an enabled entry for it, so a deployment with no
monitors file does not announce anything until the entry is added.

## Acceptance criteria

- At 7:00 AM Eastern, when the latest price date has no completed snapshot, one
  snapshot-incomplete event is recorded for it, and a Telegram message names the date
  and the affected instruments, or says no run was recorded.
- The monitor records nothing when the date has a completed snapshot, including a date
  with no active positions, or when no price data exists; and an incomplete evening run
  completed by the 6 AM run produces no event.
- Running the monitor again for the same unresolved date records no second event; after
  the operator removes that event, a still unresolved date is announced again.
- A price refresh job that never ran is caught by the monitor.
- With no enabled entry for snapshot.incomplete in the monitors configuration, the
  monitor records nothing, whatever the state.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, which turns the monitor on.
- Related: 2026-11-15-status, whose definition of an unresolved snapshot date it shares.
- Related: 2026-11-13-investments-mcp-plan, the daily schedule it follows.
