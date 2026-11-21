# Constants

insertEventSQL = insert an events row from type, subject and data, ON CONFLICT (type, subject) WHERE subject IS NOT NULL DO NOTHING, returning the id; no row comes back when the conflict was hit.
dueEventsSQL = select the full events rows that are not processed, not given up on, recorded after now minus EventMaxAge, and whose backoff_until is null or not after now, ordered by created_at then id, limited. It mirrors domain.Event.Eligible.
lockEventSQL = select the full row of one event by id FOR UPDATE.
saveEventSQL = update one event: set handled_by to its names plus the new ones without duplicates, processed_at when handled, failures, backoff_until and given_up_at as the caller computed.
removeEventSQL = delete one event by id when processed_at is null and either given_up_at is set or created_at is EventMaxAge or longer before now, returning the id.
pruneEventsSQL = delete the events whose processed_at is EventRetention or longer before now.
givenUpEventsSQL = select the full rows with processed_at null, given_up_at set and created_at after now minus EventMaxAge, newest first by created_at then id, limited.
expiredEventsSQL = select the full rows with processed_at null and created_at EventMaxAge or longer before now, newest first by created_at then id, limited.

# Functions

## Store.RecordEvent(ctx Context, draft domain.DraftEvent) (bool, error)

1. Validate draft.
2. Run insertEvent in its own transaction.
3. Return whether a row was inserted.

#### Errors

- **1.** if validation fails, return the domain.ErrInvalidInput error.

---

- **2.** if the insert fails, return the wrapped error.

## Store.insertEvent(ctx Context, tx *sqlx.Tx, draft domain.DraftEvent) (bool, error)

1. Insert through insertEventSQL, with an empty Data stored as an empty JSON object.
2. Return true when an id came back, false when the conflict clause skipped it.

The one way an event is written. A change that must record an event in its own unit of work calls it with its transaction, so the event exists if and only if the change committed; RecordEvent and the give-up path in SaveEventOutcome are the callers today.

## Store.ListDueEvents(ctx Context, limit int) ([]domain.Event, error)

1. Return domain.ErrInvalidInput when limit is not positive.
2. Select through dueEventsSQL with the store's clock as now.
3. Convert each row, with a null backoff, processed or given-up time as nil.

#### Errors

- **2.** if the query fails, return the wrapped error.

An expired or given-up event is never selected, so it cannot block, slow or crowd out newer events, and nothing is logged for skipping it.

## Store.SaveEventOutcome(ctx Context, id int64, outcome domain.EventOutcome) error

1. Validate the outcome.
2. Run inside withTx:
   1. Lock the event through lockEventSQL.
   2. Merge outcome.Succeeded into the handled names.
   3. if outcome.Failed, add one failure and set backoff_until to now plus domain.EventBackoff of the new count.
   4. if the new count reached domain.MaxEventFailures, set given_up_at to now and log an error naming the id, type and failure count.
   5. if given up and the type is not domain.Events.DeliveryFailed, record a delivery-failed event through insertEvent carrying the id, type, recorded time and failure count.
   6. if outcome.Handled, set processed_at to now.
   7. Save through saveEventSQL.
3. Return nil.

#### Errors

- **1.** if invalid, return the domain.ErrInvalidInput error.

---

- **2.1.** if locking fails, the event may be gone.
- **2.1.1.** if no row exists, return domain.ErrEventNotFound.
- **2.1.2.** otherwise return the wrapped error.

---

- **2.5.** if recording fails, return the wrapped error; the whole transaction rolls back and the next pass tries again.

---

- **2.7.** if saving fails, return the wrapped error.

A delivery-failed event that is itself given up on only logs and is shown by status, so a failing channel cannot cascade. The log line is the dependable record, since the announcement needs the channel that may be failing. The payload is never logged. Reading the clock once per call keeps the timestamps of one outcome equal.

## Store.RemoveEvent(ctx Context, id int64) error

1. Delete through removeEventSQL with the store's clock as now.
2. if a row was deleted, return nil.
3. Look the event up by id.
4. if it does not exist, return domain.ErrEventNotFound.
5. Otherwise return domain.ErrEventNotRemovable naming the id.

#### Errors

- **1.** if the delete fails, return the wrapped error.

---

- **3.** if the lookup fails, return the wrapped error.

The conditions are in the delete itself, so removal and the eligibility check cannot disagree under a concurrent worker pass.

## Store.PruneEvents(ctx Context) (int, error)

Delete through pruneEventsSQL with the store's clock as now and return the number of rows removed, or the wrapped error.

The events table is created by a migration, reviewed as SQL and not modelled here. It has one row per event with the fields of domain.Event; a check that the type is dot-separated lowercase words and that failures are not negative; a partial index over pending events, those neither processed nor given up on, ordered by creation time, which backs ListDueEvents; and a partial unique index over type and subject where the subject is set, which makes RecordEvent atomic and gives monitors their one event per subject.
