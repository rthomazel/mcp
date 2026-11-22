# Constants

pollInterval = 30 seconds, twice per minute.
pruneInterval = 24 hours.
batchLimit = 100, the events taken per pass.
handlerTimeout = 30 seconds, the deadline for one handler call.

# Types

## Worker

1. store domain.Store
2. source HandlerSource
3. clock func() Time

Drains the outbox. It holds no state beyond its dependencies; every retry fact lives on the event.

# Interfaces

## HandlerSource

1. Handlers(ctx Context) (map[string][]domain.EventHandler, error)

The handlers registered for each event type, read fresh on every pass so configuration changes apply without a restart. Defined here, implemented by the notification package. A type with no entry has no registered handler. An error means the handlers are unavailable.

# Functions

## NewWorker(store domain.Store, source HandlerSource) *Worker

Build a Worker using the real clock.

## Worker.Run(ctx Context)

1. Run one pass immediately and prune once.
2. Every pollInterval run another pass, and every pruneInterval prune.
3. Stop when ctx is cancelled, logging at info level that the worker stopped.

A failing pass never stops the loop. The worker runs inside the server process, started from main.

## Worker.RunOnce(ctx Context)

1. Read the handlers from source.
2. List the due events through store.ListDueEvents with batchLimit.
3. Handle each one with handleEvent, in the order returned.

#### Errors

- **1.** if reading fails, log at error level; the events of this pass all fail through handleEvent.

---

- **2.** if listing fails, log at error level and return.

The handlers are read before the events are listed, so a broken configuration is logged on every pass, even when no event is due. One pass can be driven directly, so tests need no timer.

## Worker.handleEvent(ctx Context, event domain.Event, handlers map[string][]domain.EventHandler, handlersErr error)

1. if handlersErr is set, save a failed outcome and return.
2. Look up the handlers registered for event.Type.
3. if there are none, log at error level naming the id and type, save a failed outcome and return.
4. Run each handler whose name is not in event.HandledBy, with handlerTimeout, collecting the names that succeeded and noting any failure.
5. Save the outcome through store.SaveEventOutcome: the succeeded names, Failed when any handler failed, Handled when none did.

#### Errors

- **4.** if a handler fails, log at error level naming the id, type, handler and the error, never the payload; the other handlers still run.

---

- **5.** if saving fails, log at error level and move on; the event is picked up again on the next pass.

Retries are per handler: a handler already in HandledBy is skipped, so a failing sibling never reruns it. An event whose handlers have all succeeded before, for example because a route was removed, becomes fully handled here. A failed outcome counts toward the ten failures per event, however many handlers failed. An event with no registered handler is an error, not a skip.

## Worker.prune(ctx Context)

Delete the expired history through store.PruneEvents and log the number at info level when it is not zero, or the error at error level.
