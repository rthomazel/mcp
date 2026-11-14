---
id: 2026-11-13-notifications
type: idea
summary: Follow-up idea for Telegram notifications driven by an internal events outbox.
author: Thom
created: 2026-11-13
agents: claude
---

# Notifications

## Status

Idea only, not part of v1. The Investments MCP spec lists notifications as a
non-goal; this captures a possible follow-up. Everything below is to be decided.

## Idea

Send the operator a Telegram message when something meaningful happens in the
system. Telegram is easy to integrate and enough for a personal system.

## Shape

The system keeps an internal events table that code writes to whenever a meaningful
thing happens. It works as an outbox: events are recorded in the same unit of work
as the change that caused them, so an event exists if and only if the change was
committed.

A separate component reads the outbox and delivers events to Telegram, marking each
as delivered. Delivery failures are retried without losing or duplicating events
more than the channel tolerates. Because producers only write events, other
delivery channels could be added later without touching them.

## Proven pattern

A service already in production uses this pattern, so it can be followed closely.

- An events table holds each event's type, a free-form data payload, an error count, a backoff-until time, a processed-at time and the list of handlers that have already succeeded.
- State changes write their event in the same transaction, so an event exists only if the change committed.
- Event types are centralized as named constants in the domain package, not scattered string literals.
- A handler has a stable name and a handle operation. The name is persisted with the event, so renaming a handler breaks events that are still in flight.
- A registry maps each event type to its handlers. A polling worker runs every few seconds, fetches events that are unprocessed and past their backoff, and runs each handler that has not yet succeeded.
- Retries are per handler, not per event. A failed handler doubles its delay from a base of about thirty seconds, with a cap of ten doublings, which is roughly four and a half hours.
- An event is complete once every registered handler has succeeded.
- The worker can be driven for one pass in tests without a timer.

For Telegram this means one handler per notification kind, registered against the events listed below.

## Candidate events

- A daily run completed, or was recorded incomplete, with the reason.
- A price was flagged as unexpectedly old.
- A snapshot rebuild was performed.
- A data-integrity error was found, such as an existing negative balance.
- A transaction was recorded through the agent.

## To be decided

- Which events are meaningful enough to notify, and which only belong in the log.
- Where the delivery component runs: in the server process or separately.
- Delivery guarantees, retry policy and retention of delivered events.
- Where the Telegram credential lives and how it is rotated.
- How this relates to the updates log, which already records ingestion outcomes.
- Whether the outbox is useful beyond notifications.
