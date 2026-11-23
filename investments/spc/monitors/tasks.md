---
id: 2026-11-23-monitors-tasks
type: tasks
summary: Delivery order for the monitors added on 2026-11-23, one task per coherent change.
author: Thom
created: 2026-11-23
agents: claude
---

# Tasks

Each task is reviewed through its models before code. Specs are in this directory; technical
design belongs in the models.

## [ ] 1. Durations

Add the duration parser and its date arithmetic as a small package, with the accepted and rejected
cases of the duration spec as tests. See the duration spec.

## [ ] 2. Structured settings

Amend the monitors configuration model and loader for list and mapping settings, duration settings
and the instrument selection (list of tickers, or held), and make an invalid monitors file disable every
monitor and log an error when there is no earlier valid content. See the monitors configuration spec.

## [ ] 3. Portfolio extremes

Refactor the portfolio drawdown monitor into the portfolio extremes design, with the drawdown
and the new high sharing the value series, and run it at 7:00 instead of 7:05. Add the portfolio
all-time high (ATH) monitor with the interval lookup over earlier events. No compatibility with the old drawdown code is
needed. See the portfolio extremes spec.

## [ ] 4. Portfolio movement

Add the market movement calculation over a window in SQL, the window and interval settings, and the monitor. See the portfolio movement spec.

## [ ] 5. Allocation drift

Add group allocation over the snapshot series, the stay start, and the monitor. See the
allocation drift spec.

## [ ] 6. Price volatility

Add the window move and volatility calculation over adjusted closes, the window and interval settings, and the monitor. See the
price volatility spec.

## [ ] 7. Price target

Add the monitor over the latest daily range. See the price target spec.

## [ ] 8. Instrument drawdown and instrument all-time high

Add the per-instrument series, levels, crossings and interval, sharing code with the portfolio
monitors where the models allow. Depends on tasks 2 and 3, whose settings and extremes engine it reuses. See the instrument extremes spec.

## [ ] 9. Notification renderers and routing

Add a message renderer per new event type, and sample routing entries in the notifications
configuration documentation. Can land with each monitor instead of separately; decide when
modelling task 3.
