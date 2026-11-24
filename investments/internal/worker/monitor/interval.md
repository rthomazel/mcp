# Functions

## lastEventDate(ctx Context, events domain.EventStore, eventType string, subjectPrefix string) (*domain.Date, error)

1. Read the latest event of the type and subject prefix through events.Latest.
2. if there is none, return nil.
3. Decode the date field of its payload and return it.

#### Errors

- **1.** if reading fails, return the wrapped error.

---

- **3.** if decoding fails, return the wrapped error.

Every monitor payload that is spaced by an interval carries date, the judged date, so the interval is measured from the date the last announcement was about and not from when its event was created, and needs no state of its own.

## withinInterval(last *Date, interval duration.Duration, latest Date) bool

Return true when last is set and is after interval.Before(latest), so an event exactly one interval before the latest date no longer suppresses it.
