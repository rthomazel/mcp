---
id: 2026-11-21-notifications
type: spec
summary: Generic event outbox with a retrying in-process worker, delivering to the operator over Telegram, starting with incomplete daily runs.
author: Thom
created: 2026-11-21
updated: 2026-11-21
agents: claude
---

# Notifications

## Status

Draft. This specifies a generic event outbox, the worker that drains it, the Telegram
channel, and the first event kind, an incomplete daily run. Daily equity and valuation
checks, tied to price fluctuations and portfolio equity, are separate follow-up specs
that add their own event kinds. This spec supersedes the former notifications idea,
and where it conflicts with the notifications non-goal of the base spec, this spec wins.

## Intent

Let the system tell the operator, over Telegram, when something meaningful happens,
without the operator asking the agent. Telegram is easy to integrate and enough for
a personal system.

The outbox itself is generic, not a notification feature. Producers only record that
something happened and know nothing about how it is announced or acted on. What runs
for an event is decided by handlers, so other channels or other kinds of work can be
added later without touching any producer. Notifications are the first use of it.

## Concepts

**Event.** A record that something meaningful happened. It has a type, a free-form
data payload, and bookkeeping about its handling: how many times it has failed, the
earliest time it may be tried again, which handlers already succeeded, when it was
fully handled, and when delivery was given up on, if it was.

**Event type.** A string of words separated by dots, naming what happened, for
example snapshot.incomplete. Types are named in one central place in the domain,
not as scattered literals.

**Outbox.** The table of events. It is both the queue that handling works from and
a durable history of what was recorded.

**Handler.** One side effect attached to one event type, for example one Telegram
message kind. A handler has a stable name, which is stored with the event.

**Registry.** The mapping from each event type to the handlers that run for it.

**Channel.** A destination a notification handler sends through. Telegram is the
first and only channel.

The outbox and the updates log are separate. Neither is derived from the other and
neither depends on the other; an event is recorded beside the updates entry for the
same occurrence, not generated from it.

## Behavior

### Recording events

An event is recorded in the same unit of work as the change that caused it, so an
event exists if and only if the change was committed. Recording an event never
changes the outcome of the change it reports, and no producer waits for handling.
Nothing is recorded on a read.

### Handling

A worker runs inside the server process, like the daily job. It checks the outbox
twice per minute, and once immediately on startup. For each event that is not fully
handled, has not been given up on, and whose backoff has passed, it runs every handler
registered for the event's type that has not yet succeeded for it.

- Retries are per handler: a handler that already succeeded for an event does not run
  again because a sibling handler failed.
- A failed attempt increases the event's failure count and postpones the next attempt
  with an exponential backoff, doubling from a base of thirty seconds.
- When a handler succeeds, its name is recorded on the event.
- An event is fully handled once every handler registered for its type has succeeded.
- An event whose type has no registered handler is an error: it is logged at error
  level and counts as a failure of the event like any other, so it is retried with
  backoff and eventually given up on, and surfaces for attention.
- A failure of one event or handler does not stop the worker from handling the
  others, and store errors are logged without stopping it.
- One pass of the worker can be driven on demand, so behavior is testable without
  waiting on a timer.

Handling is at-least-once. A handler can fail after Telegram accepted a message but
before the success was recorded, so an announcement may rarely be duplicated. A
duplicated message is acceptable; a lost one is not.

Handler names are never reused for different behavior and never removed while events
that reference them may still be in flight, because the stored names are how retries
are tracked.

Events are handled roughly in the order they were recorded; ordering across events or
types is not guaranteed, particularly while one is backing off.

### Giving up

After ten failures an event is given up on. It is no longer retried, and it stays in
the outbox, marked as given up, until the operator removes it. Giving up is never
silent. It does three things:

1. It logs an error naming the event, its type and its failure count, never its payload.
   The log is the dependable record, because the notification below needs the channel
   that may be the very thing failing.
2. It records a delivery-failed event, which a Telegram handler announces to the
   operator, naming the type of the event that failed and when it was recorded.
3. The event shows up in the status view, as described below.

A delivery-failed event that is itself given up on only logs and shows in status. It
never records another delivery-failed event, so a failing channel cannot cause a
cascade.

### Failed deliveries in status

The status view, which already reports unresolved failed and incomplete runs, also
lists every event that was given up on, with its type, when it was recorded and its
failure count. Any such event makes the overall verdict needs attention. It stays
until it is removed.

### Removing events

The agent can remove an event that was given up on, to clean up failed deliveries
once the operator has dealt with the cause. Events that are pending, backing off or
fully handled cannot be removed by the agent. Removing is by event identity, which
status exposes for events that gave up. This is the only write the agent has on the
outbox; it cannot create, edit or retry events. The exact tool contract is in the
models.

### Telegram channel

Messages are sent through the Telegram Bot API from the operator's bot, in a direct
chat with the operator. There is a single configured chat. A handler produces the
message text for its event type from the event's data and the channel only sends it.

- A non-success response from Telegram, or a failure to reach it, is a handler
  failure and is retried and counted as described above. Messages are plain text with
  no markup, so content can never be rejected for formatting reasons.
- Until both the bot credential and the chat are configured, a stub channel that
  sends nothing and always succeeds is used, so the system runs without Telegram.
  Events are still recorded and handled.
- The bot credential is a server-side secret. It never appears in tool results, logs
  or event payloads, and configuration allows replacing it without a code change.
- The operator verifies delivery to Telegram themself; the system provides no test
  message.

### Retention

Fully handled events are deleted after one year, by a periodic cleanup run in the same
worker. Events that are not fully handled are never deleted by the cleanup, no matter
how old, and neither are events that were given up on: those leave only when the
operator removes them.

### First event kind: incomplete daily run

The daily job runs in the evening and again at 6 AM Eastern, and an evening run that
is incomplete usually completes the next morning. The 6 AM run records an incomplete
daily run event when, after that run, the date it judged is still not completed, that
is, when it could not write that date's snapshot because a held instrument lacked a
usable price. The evening run records no event, and neither does an agent refresh or
rebuild. If the date is completed by the 6 AM run, or was already completed, nothing
is recorded.

The event is recorded together with the run's outcome in the updates log, and its
Telegram handler announces the date and which instruments had which issue, naming
symbols, dates and counts only. At most one such event is recorded per 6 AM run.

## Non-goals

- Event kinds beyond incomplete daily runs and delivery failures. Equity and
  valuation checks come with their own specs.
- Channels other than Telegram.
- Replies, commands or any inbound Telegram interaction, and a test-message feature.
- Exactly-once delivery.
- Agent tools to create, edit or retry events, or to read the outbox beyond what
  status shows.
- A separate delivery process; the worker runs in the server.
- Mixing the outbox and the updates log.

## Decisions

- The outbox pattern from the walk-in service is followed closely: an events table,
  handlers with stable names, a registry per event type, a polling worker and
  per-handler retries with exponential backoff.
- The outbox is generic; notifications are its first use.
- The worker runs in the server process, polls twice per minute, and gives up after
  ten failures, logging an error.
- An unhandled event type is an error, not a silent skip.
- Giving up logs an error and also records a delivery-failed event, which cannot
  cascade.
- Given-up events appear in status and are removed by an agent tool.
- Processed events are retained for one year; unprocessed and given-up events stay.
- Telegram only, one direct chat, plain text, stub channel when unconfigured.
- The incomplete-run event is first and fires only from the 6 AM run.

## Acceptance criteria

- An event recorded with a change exists after the change commits and does not exist
  when the change is rolled back.
- A pending event with a registered handler is delivered to that handler, recorded as
  succeeded for it, and then marked fully handled.
- With two handlers on one type where one fails, the successful one is not run again
  on retry, the failing one is retried after its backoff, and the event becomes fully
  handled only after both succeed.
- Consecutive failures double the delay from thirty seconds, and an event is not
  retried before its backoff has passed.
- The worker checks about every thirty seconds and once at startup.
- An event with no registered handler is logged as an error and retried like any
  failure.
- After ten failures an event is not retried again, an error is logged, a
  delivery-failed event is recorded, and the event is listed in status with a needs
  attention verdict.
- A given-up delivery-failed event does not record another delivery-failed event.
- The agent can remove a given-up event and no other event; after removal status no
  longer lists it.
- A message reaches the configured Telegram chat when the credential and chat are set,
  and nothing is sent, with no error, when they are not.
- The Telegram credential appears in no log, tool result or event payload.
- The cleanup deletes fully handled events older than a year and no others.
- A restart of the server does not lose pending events.
- A 6 AM run that leaves its date incomplete records one incomplete daily run event
  and a Telegram message naming the date and the affected instruments; an evening run
  that is incomplete records none; a 6 AM run that completes the date records none.

## Open questions

- Which equity and valuation checks to notify on, in the follow-up specs.

## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
- Related: 2026-11-15-status, which lists given-up events.
- Related: 2026-09-17-core-event-bus, the walk-in service's outbox this follows.
- Related: 2026-11-13-investments-mcp-plan, the daily schedule the first event kind relies on.
