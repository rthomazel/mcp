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

## [ ] 1. Structured settings

Amend the monitors configuration model and loader for list and mapping settings and the
instrument selection (list of tickers, or held). See the monitors configuration spec.

## [ ] 2. Portfolio extremes

Refactor the portfolio drawdown monitor into the portfolio extremes design, with the drawdown
and the new high sharing the value series, and run it at 7:00 instead of 7:05. Add the portfolio
all-time high (ATH) monitor with the interval lookup over earlier events. No compatibility with the old drawdown
code is needed. See the portfolio extremes spec.

## [ ] 3. Portfolio movement

Add the market movement calculation over a window in SQL, the window and interval settings, and the monitor. See the portfolio movement spec.

## [ ] 4. Allocation drift

Add group allocation over the snapshot series, the stay start, and the monitor. See the
allocation drift spec.

## [ ] 5. Price volatility

Add the window move and volatility calculation over adjusted closes, the window and interval settings, and the monitor. See the
price volatility spec.

## [ ] 6. Price target

Add the monitor over the latest daily range. See the price target spec.

## [ ] 7. Instrument drawdown and instrument all-time high

Add the per-instrument series, levels, crossings and interval, sharing code with the portfolio
monitors where the models allow. Depends on tasks 1 and 2, whose extremes engine it reuses. See the instrument extremes spec.

## [ ] 8. Notification renderers and routing

Add a message renderer per new event type, and sample routing entries in the notifications
configuration documentation. Can land with each monitor instead of separately; decide when
modelling task 2.
