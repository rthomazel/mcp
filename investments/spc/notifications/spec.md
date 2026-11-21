---
id: 2026-11-21-notifications
type: spec
summary: Generic event outbox with a retrying in-process worker and scheduled monitors, delivering to the operator over Telegram, starting with a monitor for a missing daily snapshot.
author: Thom
created: 2026-11-21
updated: 2026-11-21
agents: claude
---

# Notifications

## Status

Draft. This specifies a generic event outbox, the worker that drains it, a
generic notification handling over a channel interface with categories, with Telegram as the first channel implementation,
scheduled monitors that record events from observed state, and the first monitor, specified in spc/monitors. Daily equity and
valuation monitors, tied to price fluctuations and portfolio equity, are separate
follow-up specs that add their own monitors and event kinds.

## Intent

Let the system tell the operator when something meaningful happens, without the
operator asking the agent. Telegram is the first way it does so: easy to integrate
and enough for a personal system. Notifying is defined independently of Telegram, so
another way of reaching the operator can replace or join it without touching any
handler or producer.

The outbox itself is generic, not a notification feature. Producers only record that
something happened and know nothing about how it is announced or acted on. What runs
for an event is decided by handlers, so other ways of reaching the operator or other kinds of work can be
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

**Channel interface.** The contract every way of delivering a notification meets: one
operation that sends a plain-text message to a category and reports whether it was
delivered. Notification handlers depend only on it.

**Channel implementation.** The code that delivers through one particular service, such
as Telegram, or the stub that sends nothing. Channel implementations are the only code
that knows about a service. One is configured with its own settings, for example the
Telegram bot credential, and with one or more categories.

**Category.** The named purpose a notification serves, set up within a channel
implementation with its own settings, so one implementation can reach different places
for different purposes. The Telegram channel implementation has two: portfolio, a chat
for what the operator wants to know about the investments, and system, a chat for the
few messages about the system's own health.

**Notification handler.** The generic handler for events that notify. It accepts the
complete notifications configuration as one unit, and for each event it handles it
renders the message text and sends it to every target the configuration routes that
event type to. Event types, producers and monitors know nothing about how an event is
handled.

**Configuration.** One versioned YAML file declaring how notifications work: the channel
implementations in use with their settings, the categories each offers, and for each
notifying event type its message format and its targets, each target being a channel
implementation and one of its categories. Routing lives only here.

**Monitor.** A job that runs on a schedule, judges a condition on the system's
current state, and records an event when the condition holds. A monitor names the
subject it judges, for example a date, so it announces each subject at most once.

The outbox does not replace or duplicate the updates log, which stays the record of
ingestion outcomes. A monitor may read the updates log, as status does, to judge its
condition, but an event is never a copy of an updates entry.

## Behavior

### Recording events

There are two kinds of producer. A change records its event in the same unit of work
as the change itself, so the event exists if and only if the change was committed. A
scheduled monitor records its event from the state it observed, as described below.
Recording an event never changes the outcome of what it reports, and no producer waits
for handling. Nothing is recorded on a read.

### Handling

A worker runs inside the server process, like the daily job. It checks the outbox
twice per minute, and once immediately on startup. For each event that is not fully
handled, has not been given up on, was recorded within the last week, and whose backoff
has passed, it runs every handler
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
   implementation that may be the very thing failing.
2. It records a delivery-failed event, which the configuration routes like any other
   event, intended to the system category, naming the type of the event that failed and
   when it was recorded.
3. The event shows up in the status view, as described below.

A delivery-failed event that is itself given up on only logs and shows in status. It
never records another delivery-failed event, so a failing channel implementation cannot cause a
cascade.

### Age limit

An event is only considered for handling during the week after it was recorded. The
worker's lookup skips anything older, so an event that was given up on, or one that
sat unhandled for a week because the server was down, can never block, slow or crowd
out newer events, and no manual cleanup is needed for delivery to carry on. An event
older than a week that is not fully handled is expired. Skipping an expired event is
not logged, since the lookup runs constantly; status is where it shows.

### Failed deliveries in status

The status view, which already reports unresolved failed and incomplete runs, also
reports events that did not get handled, in two separate areas of its message.

- Events that were given up on within the last week are listed with their type, when
  they were recorded and their failure count. They make the overall verdict needs
  attention until they are removed or become expired.
- Expired events, those not fully handled and older than a week, including events that
  were given up on earlier, are listed in their own area, labeled as expired, with the
  same details. They are informational: they do not affect the verdict, and they stay
  in view until they are removed.

### Removing events

The agent can remove an event that was given up on or has expired, to clean up failed deliveries
once the operator has dealt with the cause. Events that are pending, backing off or
fully handled cannot be removed by the agent. Removing is by event identity, which
status exposes for events that gave up. This is the only write the agent has on the
outbox; it cannot create, edit or retry events. The exact tool contract is in the
models.

### Notification handling

Notifying is generic. Each event type that notifies has one notification handler: the
same handler, configured with a message format for the type and a list of targets. The
message format turns the event's data into plain, channel-neutral text, with no
markup, and may look at current system state to do so, for example the updates log.
The handler knows channel implementations and categories only by name and through
the channel interface, never a particular service, so no handler is written for one service's
formatting, and content cannot be rejected for formatting reasons.

- A handler sends to every target it is configured with. Targets are independent: each
  is delivered and retried on its own, so a target that already delivered is not sent
  the message again because another failed, and one failing target does not stop
  delivery to the others.
- A failure on any target is a failure of the handler for that event, counted toward
  the ten per event as described above.
- Each target is tracked on the event under a stable name derived from the handler, the
  channel implementation and the category. Like handler names, these names are never reused for a
  different purpose and never removed while events that reference them may still be in
  flight.
- An event type that notifies but has no target configured logs a warning and is
  treated as delivered, since there is nothing to send to.
- Targets never fall back to one another: a message reaches only the targets its
  handler names.
- Each target uses one channel implementation. Adding an implementation or a category
  needs no change to any handler or producer, only the implementation, if new, and
  configuration.

The stub channel implementation sends nothing and always succeeds, logging a warning each time it
handles a message, so an unconfigured system is visible in the logs. It is used until a
real channel implementation is configured, and a category with nothing configured behind
it behaves the same way, so the system runs without Telegram and events are still
recorded and handled.

Categories keep different kinds of message apart. Investment notifications go to the
portfolio category and messages about the system itself go to the system
category, so the operator can read, mute or route them separately. The two are in
the same place only when the operator configures them to be.

The system category is for the occasional message about the system, not a place for
logs or alerts. Logs and system failures are observed through infrastructure-level
observability, which this does not replace.

### Configuration file

The notifications configuration is a single YAML file, because it is too structured for
flat settings. It carries a version number that identifies its format, so the format can
evolve; a file with a version the server does not know is rejected.

- Changes to the file take effect without restarting the server, within about one worker
  pass. How the file is loaded and cached is a matter for the models.
- Secrets are not in the file. The file says where each secret comes from, for example
  an environment variable, so the file can be versioned and reviewed without exposing
  the Telegram credential, which can still be rotated without a code change.
- If no file exists, notifications are unconfigured: an event type has no target, which
  logs a warning and counts as delivered, as described above, so the system runs
  without Telegram.
- If a file exists but is invalid, events are never treated as delivered because of it.
  When the server starts with an invalid file, it still starts, but the notification
  handler fails with an error logged until the file is valid, so events retry and
  surface through the rules above. When an edit makes a running file invalid, the last
  valid configuration keeps being used. In both cases an error is logged on every
  worker pass for as long as the file is invalid, not once, so it cannot go unnoticed.
- The configuration source is replaceable. A later source, such as a database behind a
  dashboard, supplies the same complete configuration to the handler and changes no
  event type, producer or monitor.

### Telegram channel implementation

The Telegram channel implementation sends through the Telegram Bot API from the operator's bot. Each
Telegram category is configured with its own chat, which can be a direct chat with
the operator, a group or a broadcast channel; two categories can use different chats, and can be set to
the same one if the operator wants.

- A non-success response from Telegram, or a failure to reach it, is reported to
  the interface as a failure to deliver.
- The Telegram channel implementation is active when the bot credential is configured, and a
  category when its chat is; otherwise the stub behavior above applies.
- The bot credential is a server-side secret. It never appears in tool results, logs
  or event payloads, and configuration allows replacing it without a code change.
- The operator verifies delivery to Telegram themself; the system provides no test
  message.

### Retention

Fully handled events are deleted after one year, by a periodic cleanup run in the same
worker. Events that are not fully handled are never deleted by the cleanup, no matter
how old, and neither are events that were given up on or expired: those leave only when the
operator removes them.

### Scheduled monitors

A scheduler runs inside the server process, like the daily job, and runs each monitor
at its own time in the market timezone, so daylight saving changes keep the same local
times. Monitors are independent of the daily job and of each other: a monitor does not know
how or when the job ran, only what state it left. A monitor does not run at startup; if
the server was down at its time, the next scheduled time judges the state then.

- A monitor records an event only for a subject that has no event of that monitor in the
  outbox yet, so repeating the monitor, or a later monitor seeing the same unresolved
  subject, never announces it twice. An event the operator removed no longer counts,
  so a subject that is still unresolved can be announced again.
- A monitor that fails is logged at error level and does not stop other monitors or the
  next scheduled time.
- Monitors hold no state of their own beyond the outbox.

Monitors exist because an event should follow observed state, not which run happened
to execute. A monitor also catches what a producer cannot, such as a daily job that never
ran.

Each monitor has its own spec under spc/monitors, one file per monitor. The first is
the latest price date without a snapshot, in its own spec; it records the
snapshot-incomplete event; where it is announced is decided by the notifications
configuration, not by the monitor.

## Non-goals

- Event kinds and monitors beyond the snapshot monitor and delivery failures. Equity and
  valuation monitors come with their own specs.
- Channel implementations other than Telegram and the stub.
- Replacing logging or infrastructure-level observability of system failures.
- Categories beyond portfolio and system, which the operator may add by configuration.
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
- Failures are counted per event, not per handler or target: per-handler counting needs
  a more elaborate column or table, and an event with several targets shares one count
  of ten. Revisit if a type gets several handlers or targets that diverge in practice.
- Giving up logs an error and also records a delivery-failed event, which cannot
  cascade.
- Given-up events need attention in status for a week and are then listed as expired,
  informationally; both are removed by an agent tool.
- Processed events are retained for one year; unprocessed, given-up and expired events stay.
- Events are only considered for handling for a week after they were recorded, so a
  stuck event never blocks newer ones.
- Notifying is generic: one notification handler per notifying event type, configured
  with a message format and a list of targets, each a channel implementation and one of
  its categories. Handlers depend only on the channel interface. Telegram is the first
  channel implementation, with plain text, and the stub behavior applies when an
  implementation or category is unconfigured.
- The configuration is a versioned YAML file with secrets kept out of it, read live and
  replaceable by another source later, such as a database.
- Event types, producers and monitors are unaware of how events are handled: the
  notification handler takes the complete configuration of channel implementations,
  categories and routes.
- Categories separate investment notifications from system messages. The intended
  configuration sends delivery failures to the system category, which never falls back
  to the portfolio one.
- Events can follow observed state: a scheduled monitor, not the daily job, records the
  snapshot-incomplete event, so it does not depend on there being two
  daily runs and also covers a run that never happened.
- A monitor announces each subject at most once.

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
- An event older than one week that is not fully handled is not picked up again and
  is listed in status as expired, in an area that does not affect the verdict; it does
  not block or delay newer events.
- The agent can remove a given-up or expired event and no other event; after removal status no
  longer lists it.
- A notification handler uses only the channel interface; replacing a channel
  implementation changes no handler and no producer.
- An event configured for two targets is delivered to both; if one fails, the one that
  delivered is not sent the message again on retry and the failing one is retried.
- A channel implementation can have more than one category, set in the notifications configuration,
  and a message reaches only the categories its targets name.
- A message reaches the chat configured for its category when the credential and
  that chat are set; when they are not, nothing is sent, a warning is logged and the
  handler succeeds. An event type with no target configured logs a warning and is
  treated as delivered.
- With the intended configuration, a delivery-failed event reaches the system category
  and a snapshot-incomplete event the portfolio category, and a category never falls
  back to the other. No event type, producer or monitor refers to a channel
  implementation or category.
- A change to the configuration file takes effect without a restart; an invalid file never
  causes events to be treated as delivered, and a running configuration is kept when an
  edit makes the file invalid, and an error is logged on every worker pass while it is
  invalid.
- The notification handler accepts the complete configuration, so changing routes,
  categories or channel implementations changes no event type, producer or monitor.
- The Telegram credential appears in no log, tool result or event payload.
- The cleanup deletes fully handled events older than a year and no others.
- A restart of the server does not lose pending events.
- Repeating a monitor for a subject it already announced records no second event; after
  the operator removes that event, a still unresolved subject is announced again.
- A failing monitor is logged and does not stop other monitors or its next run, and the
  monitor does not run at startup.

## Open questions

- Which equity and valuation monitors to notify on, in the follow-up specs.
- How notifications can carry formatting. Messages are plain text today, so any channel
  implementation can deliver them. Once a channel implementation such as email supports
  richer content like HTML, a notification might offer a richer form while Telegram
  keeps plain text. Whether the message format produces one neutral text that each
  channel implementation renders, or several forms chosen per channel implementation, is
  for future discussion.

## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
- Related: 2026-11-15-status, which lists given-up and expired events.
- Related: 2026-09-17-core-event-bus, the walk-in service's outbox this follows.
- Related: 2026-11-21-monitor-snapshot-incomplete, the first monitor.
- Related: 2026-11-13-investments-mcp-plan, the daily schedule the first monitor follows.
