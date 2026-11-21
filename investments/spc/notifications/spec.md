---
id: 2026-11-21-notifications
type: spec
summary: Generic event outbox with a retrying in-process worker and scheduled checks, delivering to the operator over Telegram, starting with a check for a missing daily snapshot.
author: Thom
created: 2026-11-21
updated: 2026-11-21
agents: claude
---

# Notifications

## Status

Draft. This specifies a generic event outbox, the worker that drains it, a
channel-independent notification interface with Telegram as its first adapter,
scheduled checks that record events from observed state, and the first check, a daily run that left the latest price date without a snapshot. Daily equity and
valuation checks, tied to price fluctuations and portfolio equity, are separate
follow-up specs that add their own checks and event kinds.

## Intent

Let the system tell the operator when something meaningful happens, without the
operator asking the agent. Telegram is the first way it does so: easy to integrate
and enough for a personal system. Notifying is defined independently of Telegram, so
another way of reaching the operator can replace or join it without touching any
handler or producer.

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

**Handler.** One side effect attached to one event type, for example one kind of
notification message. A handler has a stable name, which is stored with the event.

**Registry.** The mapping from each event type to the handlers that run for it.

**Channel.** The notification interface: one operation that sends a plain-text
message to the operator and reports whether it was delivered. Notification handlers
depend only on this interface, never on any particular service. Telegram is its first
adapter, and a stub that sends nothing is another. Which adapter is active is decided
by configuration.

**Check.** A routine that runs on a schedule, judges a condition on the system's
current state, and records an event when the condition holds. A check names the
subject it judges, for example a date, so it announces each subject at most once.

The outbox does not replace or duplicate the updates log, which stays the record of
ingestion outcomes. A check may read the updates log, as status does, to judge its
condition, but an event is never a copy of an updates entry.

## Behavior

### Recording events

There are two kinds of producer. A change records its event in the same unit of work
as the change itself, so the event exists if and only if the change was committed. A
scheduled check records its event from the state it observed, as described below.
Recording an event never changes the outcome of what it reports, and no producer waits
for handling. Nothing is recorded on a read.

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
2. It records a delivery-failed event, which a notification handler announces to the
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

### Channel interface

A notification handler produces the message text for its event type from the event's
data and hands it to the channel, which only sends it. Message text is plain and
channel-neutral, with no markup, so no handler is written for one service's formatting
and content cannot be rejected for formatting reasons. The channel reports failure
when the message could not be delivered, and that is a handler failure, retried and
counted as described above.

One adapter is active at a time. Adding another adapter requires no change to any
handler or producer, only the adapter and its configuration. Sending to several
adapters at once is not specified.

The stub adapter sends nothing and always succeeds. It is active until the active
service is configured, so the system runs without Telegram, and events are still
recorded and handled.

### Telegram adapter

The Telegram adapter sends through the Telegram Bot API from the operator's bot, in a
direct chat with the operator. There is a single configured chat.

- A non-success response from Telegram, or a failure to reach it, is reported to
  the interface as a failure to deliver.
- The adapter is active only when both the bot credential and the chat are configured;
  otherwise the stub is.
- The bot credential is a server-side secret. It never appears in tool results, logs
  or event payloads, and configuration allows replacing it without a code change.
- The operator verifies delivery to Telegram themself; the system provides no test
  message.

### Retention

Fully handled events are deleted after one year, by a periodic cleanup run in the same
worker. Events that are not fully handled are never deleted by the cleanup, no matter
how old, and neither are events that were given up on: those leave only when the
operator removes them.

### Scheduled checks

A scheduler runs inside the server process, like the daily job, and runs each check
at its own time in the market timezone, so daylight saving changes keep the same local
times. Checks are independent of the daily job and of each other: a check does not know
how or when the job ran, only what state it left. A check does not run at startup; if
the server was down at its time, the next scheduled time judges the state then.

- A check records an event only for a subject that has no event of that check in the
  outbox yet, so repeating the check, or a later check seeing the same unresolved
  subject, never announces it twice. An event the operator removed no longer counts,
  so a subject that is still unresolved can be announced again.
- A check that fails is logged at error level and does not stop other checks or the
  next scheduled time.
- Checks hold no state of their own beyond the outbox.

Checks exist because an event should follow observed state, not which run happened
to execute. A check also catches what a producer cannot, such as a daily job that never
ran.

### First check: latest price date without a snapshot

The first check runs at 7:00 AM Eastern, an hour after the last daily run of the day
at 6:00 AM, which leaves time for that run to finish. It judges the latest date that
has price data, using the same definition as the status view: the date is unresolved
when it has no completed snapshot. A date with no active positions that was recorded
complete is resolved.

- If the date is unresolved, the check records a snapshot-incomplete event for that
  date, unless one already exists for it.
- If the date is resolved, or no price data exists yet, it records nothing.

Because the check judges state at 7:00 AM, the evening run alone never triggers it:
an evening run that is incomplete and is completed by the 6 AM run records nothing.
A date that stays unresolved is announced once, however many times it is checked.
An agent refresh or rebuild that completes the date before 7:00 AM also resolves it.

The notification handler for the event names the date. When the updates log has an
incomplete entry for it, the message names which instruments had which issue, with
symbols, dates and counts only. When it has none, the message says no run was recorded
for the date, which covers a daily job that did not run.

## Non-goals

- Event kinds and checks beyond the snapshot check and delivery failures. Equity and
  valuation checks come with their own specs.
- Adapters other than Telegram and the stub, and sending to several at once.
- Replies, commands or any inbound Telegram interaction, and a test-message feature.
- Exactly-once delivery.
- Agent tools to create, edit or retry events, or to read the outbox beyond what
  status shows.
- A separate delivery process; the worker runs in the server.
- Using the outbox to store or replace the updates log.
- Announcing that an unresolved date was later resolved.

## Decisions

- The outbox pattern from the walk-in service is followed closely: an events table,
  handlers with stable names, a registry per event type, a polling worker and
  per-handler retries with exponential backoff.
- The outbox is generic; notifications are its first use.
- The worker runs in the server process, polls twice per minute, and gives up after
  ten failures, logging an error.
- An unhandled event type is an error, not a silent skip.
- Failures are counted per event, not per handler: with one handler per type they are
  the same, and per-handler counting needs a more elaborate column or table. Revisit
  when a type gets a second handler.
- Giving up logs an error and also records a delivery-failed event, which cannot
  cascade.
- Given-up events appear in status and are removed by an agent tool.
- Processed events are retained for one year; unprocessed and given-up events stay.
- Notifying goes through a channel interface that handlers depend on; Telegram is the
  first adapter, with one direct chat and plain text, and the stub is active when it is
  unconfigured. One adapter is active at a time.
- Events follow observed state: a scheduled check at 7:00 AM Eastern, not the daily
  job, records the snapshot-incomplete event, so it does not depend on there being two
  daily runs and also covers a run that never happened.
- A check announces each subject at most once.

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
- A notification handler uses only the channel interface; replacing the active adapter
  changes no handler and no producer.
- A message reaches the configured Telegram chat when the credential and chat are set,
  and nothing is sent, with no error, when they are not.
- The Telegram credential appears in no log, tool result or event payload.
- The cleanup deletes fully handled events older than a year and no others.
- A restart of the server does not lose pending events.
- At 7:00 AM Eastern, when the latest price date has no completed snapshot, one
  snapshot-incomplete event is recorded for it, and a Telegram message names the date
  and the affected instruments, or says no run was recorded.
- The check records nothing when the date has a completed snapshot, including a date
  with no active positions, or when no price data exists; and an incomplete evening run
  completed by the 6 AM run produces no event.
- Running the check again for the same unresolved date records no second event; after
  the operator removes that event, a still unresolved date is announced again.
- A daily job that never ran is caught by the check.
- A failing check is logged and does not stop other checks or its next run, and the
  check does not run at startup.

## Open questions

- Which equity and valuation checks to notify on, in the follow-up specs.

## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
- Related: 2026-11-15-status, which lists given-up events.
- Related: 2026-09-17-core-event-bus, the walk-in service's outbox this follows.
- Related: 2026-11-13-investments-mcp-plan, the daily schedule the first check follows.
