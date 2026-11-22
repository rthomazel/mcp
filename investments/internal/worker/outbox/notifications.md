# Constants

# Types

## Settings

1. PollInterval Duration, how often to look for due events
2. PruneInterval Duration, how often to prune handled events
3. BatchLimit int, the events taken per pass
4. HandlerTimeout Duration, the deadline for one handler call

The worker's tunables, from the server configuration, which defaults them to 30 seconds, 24 hours, 100 and 30 seconds.

## Notifications

1. store domain.Store
2. source HandlerSource
3. settings Settings

The notifications worker: drains the outbox. It holds no state beyond its dependencies and reads no clock; every retry fact lives on the event, and the store decides what is due.

# Interfaces

## HandlerSource

1. Handlers(ctx Context) (Handlers, error)

The handlers registered for each event type, read fresh on every pass so configuration changes apply without a restart. Defined here, implemented by the notification package under worker. An error means the handlers are unavailable.

## Handlers = func(eventType string) []domain.EventHandler

The handlers registered for an event type. None means no handler is registered. A function and not a map, since the set of types is the open set of types with the notification prefix.

# Functions

## NewNotifications(store domain.Store, source HandlerSource, settings Settings) *Notifications

Build the notifications worker.

## Notifications.Run(ctx Context)

1. Run one pass immediately and prune once.
2. Every PollInterval run another pass, and every PruneInterval prune.
3. Stop when ctx is cancelled, logging at info level that the worker stopped.

A failing pass never stops the loop. The worker runs inside the server process, started from main.

## Notifications.RunOnce(ctx Context)

1. Read the handlers from source.
2. List the due events through store.ListDueEvents with domain.NotificationPrefix and BatchLimit.
3. Handle each one with handleEvent, in the order returned.

#### Errors

- **1.** if reading fails, log at error level; the events of this pass all fail through handleEvent.

---

- **2.** if listing fails, log at error level and return.

The worker takes only events whose type starts with domain.NotificationPrefix, in the query, so events of other types, which other workers with the same retry and backoff pattern may handle, are never read, counted as failures or logged here. The handlers are read before the events are listed, so a broken configuration is logged on every pass, even when no event is due. One pass can be driven directly, so tests need no timer.

## Notifications.handleEvent(ctx Context, event domain.Event, handlers Handlers, handlersErr error)

1. if handlersErr is set, save a failed outcome and return.
2. Look up the handlers registered for event.Type with handlers.
3. if there are none, log at warn level naming the id and type and return, saving nothing.
4. Run each handler whose name is not in event.HandledBy, with HandlerTimeout, collecting the names that succeeded and noting any failure.
5. Save the outcome through store.SaveEventOutcome: the succeeded names, Failed when any handler failed, Handled when none did.

#### Errors

- **4.** if a handler fails, log at error level naming the id, type, handler and the error, never the payload; the other handlers still run.

---

- **5.** if saving fails, log at error level and move on; the event is picked up again on the next pass.

Retries are per handler: a handler already in HandledBy is skipped, so a failing sibling never reruns it. An event whose handlers have all succeeded before, for example because a route was removed, becomes fully handled here. A failed outcome counts toward the ten failures per event, however many handlers failed. An event with no registered handler, for example a misspelled type, is a warning and a skip: nothing is saved, so no failure is counted and it is never given up on, and the warning repeats on each pass until the event expires and status lists it as expired.

## Notifications.prune(ctx Context)

Delete the expired history through store.PruneEvents and log the number at info level when it is not zero, or the error at error level.
