---
id: 2026-11-23-idea-weekly-summary
type: idea
summary: Follow-up idea for a weekly portfolio summary delivered as a formatted email, covering cash flows, value, contributors, extremes, winners, losers and P&L.
author: Thom
created: 2026-11-23
agents: claude
---

# Weekly portfolio summary

## Status

Idea only, deferred from the monitors batch. Everything below is to be decided.

## Idea

Once a week, send the operator a summary of the week as an email, formatted with HTML, tables and
perhaps small charts, since it is a report to read and not an alert to act on. A Telegram message is
too small for it, so it needs a new notification channel beside Telegram.

Contents the operator listed:

- cash in and out over the week, total;
- the securities balance and the account equity at the start and end of the week;
- the largest contributors to the change;
- the extremes of the week, the high and low of the value;
- winners and losers among the positions;
- P&L.

## What already exists

- Value history gives the securities value, extremes, change over a range and account equity with
  net contributions.
- Position P&L gives profit and loss per position.
- The market movement measure of the portfolio movement monitor separates price moves from trades;
  contributors, winners and losers should use it so that buying or selling does not show as a gain.

## Gaps to resolve

- An email channel: sender, provider, credentials, and its own notification configuration entry.
- Rendering: the notifications render text; an HTML template and a way to test it.
- Whether it is a monitor that records an event with the week as its subject, as the other
  notifications work, and when it runs, for example Saturday morning for the week through Friday.
- Which weeks to include when the week has holidays or an incomplete snapshot.
- Whether the weekly performance idea, once it exists, belongs inside it.

## Refs

- Related: 2026-11-23-monitor-portfolio-movement, the market movement measure.
- Related: 2026-11-15-value-history, the measures it reads.
- Related: 2026-11-23-idea-portfolio-performance.
