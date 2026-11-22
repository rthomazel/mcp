# Constants

MaxEventFailures = 10, the failed attempts after which an event is given up on.
EventBackoffBase = 30 seconds, the delay after the first failure; each further failure doubles it.
EventMaxAge = 7 days, how long after it was recorded an event is still considered for handling; it sets the event's ExpiredAt.
EventRetention = 1 year, how long after it was fully handled an event is kept.

# Vars

## Events

1. Events.SnapshotIncomplete = "snapshot.incomplete"
2. Events.NotificationFailed = "notification.failed"

Event types are dot-separated words, named here and nowhere else.

## NotifyingEvents = Events.SnapshotIncomplete, Events.NotificationFailed

The types that notify, in a stable order. Every one of them has a notification handler; the notifications configuration routes them.

# Types

## Event

1. ID int64
2. Type string
3. Subject *string, nil unless a monitor recorded the event for a subject such as a date
4. Data RawMessage, a JSON object
5. Failures int
6. BackoffUntil *Time, the earliest time of the next attempt, nil before the first failure
7. CreatedAt Time
8. ExpiredAt Time, the time from which the event is expired, CreatedAt plus EventMaxAge, fixed when it is recorded
9. ProcessedAt *Time, set when every registered handler has succeeded
10. GivenUpAt *Time, set when Failures reached MaxEventFailures
11. HandledBy []string, the names of the handlers that already succeeded

One row of the outbox. An event is pending until ProcessedAt or GivenUpAt is set. It is expired when it is not fully handled and ExpiredAt is not after the reference time, whether or not it was given up on. Events are identified by ID, which status exposes because they have no natural key.

## DraftEvent

1. Type string
2. Subject *string, when set at most one event of the type exists for the subject
3. Data RawMessage, a JSON object, empty meaning an empty object

An event to record.

## EventOutcome

1. Succeeded []string, the names of the handlers that succeeded in this pass
2. Handled bool, every registered handler has now succeeded
3. Failed bool, a handler failed, or no handler was registered for the type

What one pass did with one event. Handled and Failed are never both true.

## SnapshotIncompleteData

1. Date Date, the latest price date that has no completed snapshot

Payload of Events.SnapshotIncomplete. The subject of the event is the date, formatted YYYY-MM-DD.

## NotificationFailedData

1. EventID int64
2. EventType string
3. RecordedAt Time, when the failed event was recorded
4. Failures int

Payload of Events.NotificationFailed. It never carries the failed event's own payload.

# Interfaces

## EventHandler

1. Name() string
2. Handle(ctx Context, event Event) error

One side effect attached to one event type. Name is persisted in Event.HandledBy, so it is stable: never reused for different behavior, and not removed while events that name it may be in flight. A handler may run more than once for an event, since delivery is at least once.

# Functions

## ValidEventType(eventType string) bool

Report whether eventType is one or more words of lowercase letters, digits and underscores, starting with a letter, separated by single dots.

## DraftEvent.Validate() error

1. if Type is not valid under ValidEventType, return ErrInvalidInput.
2. if Subject is set and blank, return ErrInvalidInput.
3. if Data is not empty and is not a JSON object, return ErrInvalidInput.
4. Return nil.

Mirrors the events table checks.

## EventOutcome.Validate() error

Return ErrInvalidInput when Handled and Failed are both true.

## EventBackoff(failures int) Duration

Return EventBackoffBase multiplied by two to the power failures minus one: 30 seconds after the first failure, a minute after the second, and so on, a little over an hour after the eighth and a little over two hours after the ninth. Zero for a count below one.

Never used for the last failure, which gives up instead of retrying.

## Event.Eligible(now Time) bool

1. if ProcessedAt or GivenUpAt is set, return false.
2. if Expired(now), return false.
3. if BackoffUntil is set and is after now, return false.
4. Return true.

The reference for the SQL that selects due events, and the way tests pin it.

## Event.Expired(now Time) bool

Report whether ProcessedAt is nil and ExpiredAt is not after now.
