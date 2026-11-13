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
