# Constants

snapshotIncompleteAt = 07:00 in the market timezone, an hour after the last price refresh run of the day.

# Types

## SnapshotIncomplete

1. store domain.GenericStore, for the status
2. events domain.EventStore, where the event is recorded

The monitor for a latest price date without a completed snapshot.

# Functions

## NewSnapshotIncomplete(store domain.GenericStore, events domain.EventStore) *SnapshotIncomplete

Build the monitor.

## SnapshotIncomplete.Name() string

Return domain.Events.SnapshotIncomplete.

## SnapshotIncomplete.At() ClockTime

Return snapshotIncompleteAt.

## SnapshotIncomplete.Run(ctx Context) error

1. Read the status through store.GetStatus.
2. if no price date exists, or the latest price date has a completed snapshot, log at info level and return nil.
3. Record a snapshot-incomplete event through store.Events.Record, with the date as its subject, as YYYY-MM-DD, and as its payload.
4. Log at info level whether it was recorded or the date was already announced.
5. Return nil.

#### Errors

- **1.** if reading fails, return the wrapped error.

---

- **3.** if recording fails, return the wrapped error.

It uses the same definition of an unresolved date as the status view, whether the evening run, the 6 AM run, an agent refresh or rebuild, or no run at all left it. One event per date: repeating the run, or a later run seeing the same date, records nothing, until the operator removes the event. The payload is the date only; the message wording is the renderer's.
