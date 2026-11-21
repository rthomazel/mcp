---
id: 2026-11-21-notifications
type: spec
summary: Outbox of internal events delivered to the operator over Telegram by retrying per-event-type handlers, with no event kinds defined yet.
author: Thom
created: 2026-11-21
agents: claude
---

# Notifications

## Status

Draft. This specifies the delivery infrastructure only: the outbox, the worker that
drains it, and the Telegram channel. No event kind notifies yet. The first kinds will
be tied to price fluctuations of instruments and to portfolio equity, and each will be
specified as its own feature when it is introduced, extending this spec. This spec
supersedes the notifications idea, and where it conflicts with the notifications
non-goal of the base spec, this spec wins.

## Intent

Let the system tell the operator, over Telegram, when something meaningful happens,
without the operator asking the agent. Telegram is easy to integrate and enough for
a personal system.

Producers of events must not know about Telegram. They only record that something
happened. How, whether and to whom it is announced is decided by handlers, so other
channels can be added later without touching any producer.

## Concepts

**Event.** A record that something meaningful happened. It has a type, a free-form
data payload, and bookkeeping about its delivery: how many times handling failed,
the earliest time it may be tried again, which handlers already succeeded, and when
it was fully handled.

**Outbox.** The table of events. It is both the queue that delivery works from and
a durable history of what was announced.

**Handler.** One side effect attached to one event type, for example one Telegram
message kind. A handler has a stable name, which is stored with the event.

**Registry.** The mapping from each event type to the handlers that run for it.
Event types are named in one central place in the domain, not as scattered string
literals.

**Channel.** A destination a handler delivers through. Telegram is the first and
only channel.

## Behavior

### Recording events

An event is recorded in the same unit of work as the change that caused it, so an
event exists if and only if the change was committed. Recording an event never
changes the outcome of the change it reports, and no producer waits for delivery.

Events are only recorded for changes that happen in this system. Nothing is recorded
on a read.

### Delivery

A worker runs inside the server process, like the daily job, and checks the outbox
every few seconds, starting immediately on startup. For each event that is not fully
handled and whose backoff has passed, it runs every handler registered for the event's
type that has not yet succeeded for it.

- Retries are per handler, never per event: a handler that already succeeded for an
  event does not run again because a sibling handler failed.
- A failing handler increases the event's failure count and postpones the next
  attempt with a doubling delay starting at about thirty seconds, with the delay
  stopping its growth after nine doublings, roughly four and a half hours. Delivery
  is retried indefinitely; an event is never abandoned automatically.
- When a handler succeeds its name is recorded on the event.
- An event is fully handled once every handler registered for its type has
  succeeded. An event whose type has no registered handler is fully handled
  immediately, so it is retained as history without being retried.
- A failure of one event or one handler does not stop the worker from handling the
  others, and the worker survives store errors, which are logged.
- One pass of the worker can be driven on demand, so behavior is testable without
  waiting on a timer.

Delivery is at-least-once. A handler can fail after Telegram accepted a message but
before the success was recorded, so an announcement may rarely be duplicated. A
duplicated message is acceptable; a lost one is not.

Handler names are never reused for different behavior and never removed while events
that reference them may still be in flight, because the stored names are how retries
are tracked.

### Ordering

Events are handled roughly in the order they were recorded, but ordering across
events or event types is not guaranteed, particularly while one is backing off.

### Telegram channel

Telegram messages are sent through the Telegram Bot API as the operator's bot to a
single configured chat, which is the operator's. A handler produces the message
text for its event type from the event's data; the channel only sends it.

- A non-success response from Telegram, or a failure to reach it, is a handler
  failure and is retried per the rules above. The message text is plain, with no
  markup, so message content can never be rejected for formatting reasons.
- Until both the bot credential and the chat are configured, a stub channel that
  sends nothing and always succeeds is used, so the system runs without Telegram.
  Events are still recorded and are marked handled.
- The bot credential is a server-side secret. It never appears in tool results, logs
  or event payloads, and configuration allows replacing it without a code change.

### Retention

Fully handled events are deleted after one year, by a periodic cleanup run in the
same worker. Events that are not fully handled are never deleted by the cleanup, no
matter how old, so an undelivered notification cannot silently disappear.

### Suppressing noisy repeats

The daily job runs in the evening and again in the morning, and an evening run that
is incomplete usually completes the next morning. When a run-incomplete event kind is
introduced, it announces only a date that is still incomplete after the morning
run, not the evening run. This rule is recorded here as a constraint for that
future event kind; it adds no event now.

## Non-goals

- Defining any event kind that notifies. Those come with their own feature specs.
- Channels other than Telegram.
- Replies, commands or any inbound Telegram interaction.
- Exactly-once delivery.
- Exposing events or the outbox through the agent tools.
- A separate delivery process; the worker runs in the server.

## Decisions

- The outbox pattern from the walk-in service is followed closely: an events table,
  handlers with stable names, a registry per event type, a polling worker and
  per-handler retries with doubling backoff.
- The worker runs in the server process, consistent with the daily job.
- No event kinds ship with this feature.
- Processed events are retained for one year, unprocessed events indefinitely.
- Telegram only, one chat, plain-text messages, stub channel when unconfigured.

## Acceptance criteria

- An event recorded with a change exists after the change commits and does not exist
  when the change is rolled back.
- A pending event with a registered handler is delivered to that handler, recorded as
  succeeded for it, and then marked fully handled.
- With two handlers on one type where one fails, the successful one is not run again
  on retry, the failing one is retried after its backoff, and the event becomes fully
  handled only after both succeed.
- Consecutive failures double the delay from about thirty seconds, growing no further
  after nine doublings.
- An event is not retried before its backoff has passed.
- An event whose type has no handler is marked fully handled without any attempt.
- A Telegram failure is retried and the failure does not affect the change that
  produced the event or any other event.
- A message reaches the configured Telegram chat when the credential and chat are
  set, and nothing is sent, with no error, when they are not.
- The Telegram credential appears in no log, tool result or event payload.
- The cleanup deletes fully handled events older than one year and no others.
- A restart of the server does not lose pending events.

## Open questions

- Whether failing or long-stuck events should surface in the status view, since a
  delivery that keeps failing is otherwise silent. Tentatively yes once the first
  event kind exists.
- How the pipeline is verified end to end in a deployed environment while no event
  kind exists, for example through a one-off operator-triggered test message.
- Whether the outbox is useful beyond notifications, for example as the trigger for
  work other than delivery. Not needed now.
- The relationship between event kinds and the updates log, which already records
  ingestion outcomes, decided per event kind when it is introduced.

## Refs

- Replaces: 2026-11-13-notifications, the idea this was developed from.
- Related: 2026-11-13-investments-mcp, the base spec this extends.
- Related: 2026-09-17-core-event-bus, the walk-in service's outbox this follows.
