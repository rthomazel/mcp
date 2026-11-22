# Constants

# Types

## Settings

1. PollInterval Duration, how often to look for due events
2. PruneInterval Duration, how often to prune handled events
3. BatchLimit int, the events taken per pass
4. HandlerTimeout Duration, the deadline for one handler call
5. TypePrefix string, only events whose type starts with it are handled; main sets domain.NotificationPrefix for now

The worker's tunables, from the server configuration, which defaults them to 30 seconds, 24 hours, 100 and 30 seconds.

## Worker

1. store domain.Store
2. source domain.EventHandlerSource
3. settings Settings

The events worker: drains the events table. It holds no state beyond its dependencies and reads no clock; every retry fact lives on the event, and the store decides what is due.

# Functions

## NewWorker(store domain.Store, source domain.EventHandlerSource, settings Settings) *Worker

Build the events worker.

## Worker.Run(ctx Context)

1. Run one pass immediately and prune once.
2. Every PollInterval run another pass, and every PruneInterval prune.
3. Stop when ctx is cancelled, logging at info level that the worker stopped.

A failing pass never stops the loop. The worker runs inside the server process, started from main.

## Worker.RunOnce(ctx Context)

1. Read the handlers from source.
2. List the due events through store.ListDueEvents with TypePrefix and BatchLimit.
3. Handle each one with handleEvent, in the order returned.

#### Errors

- **1.** if reading fails, log at error level; the events of this pass all fail through handleEvent.

---

- **2.** if listing fails, log at error level and return.

The worker takes only events whose type starts with TypePrefix, in the query, so events of other types, which a worker with another prefix and the same retry and backoff pattern may handle, are never read, counted as failures or logged here. The handlers are read before the events are listed, so a broken configuration is logged on every pass, even when no event is due. One pass can be driven directly, so tests need no timer.

## Worker.handleEvent(ctx Context, event domain.Event, handlers domain.EventHandlers, handlersErr error)

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

## Worker.prune(ctx Context)

Delete the expired history through store.PruneEvents and log the number at info level when it is not zero, or the error at error level.
