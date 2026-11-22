package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// EventPolicy is how the outbox retries, expires and retains events. The
// values come from the server configuration.
type EventPolicy struct {
	MaxFailures int           // failed attempts after which an event is given up on
	BackoffBase time.Duration // delay after the first failure; each further failure doubles it
	MaxAge      time.Duration // how long after it was recorded an event is still considered for handling
	Retention   time.Duration // how long after it was fully handled an event is kept
}

// EventTypes names the event types in one place.
type EventTypes struct {
	SnapshotIncomplete string
	NotificationFailed string
}

// Events holds the event type names, dot-separated lowercase words.
var Events = EventTypes{
	SnapshotIncomplete: "notification.snapshot.incomplete",
	NotificationFailed: "notification.failed",
}

// NotificationPrefix starts the type of every event that produces a notification.
// The convention replaces a list: the notifications worker selects by it, and the
// notifications file may configure only types that have it.
const NotificationPrefix = "notification."

// IsNotification reports whether the event type produces a notification.
func IsNotification(eventType string) bool {
	return strings.HasPrefix(eventType, NotificationPrefix)
}

var eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

// Event is one row of the outbox. It is pending until ProcessedAt or GivenUpAt is set.
type Event struct {
	ID           int64
	Type         string
	Subject      *string
	Data         json.RawMessage
	Failures     int
	BackoffUntil *time.Time
	CreatedAt    time.Time
	ExpiredAt    time.Time
	ProcessedAt  *time.Time
	GivenUpAt    *time.Time
	HandledBy    []string
}

// DraftEvent is an event to record.
type DraftEvent struct {
	Type    string
	Subject *string // when set, at most one event of the type exists for the subject
	Data    json.RawMessage
}

// EventOutcome is what one pass did with one event.
type EventOutcome struct {
	Succeeded []string // handlers that succeeded in this pass
	Handled   bool     // every registered handler has now succeeded
	Failed    bool     // a handler failed, or none was registered for the type
}

// SnapshotIncompleteData is the payload of Events.SnapshotIncomplete.
type SnapshotIncompleteData struct {
	Date Date `json:"date"`
}

// NotificationFailedData is the payload of Events.NotificationFailed. It
// never carries the failed event's own payload.
type NotificationFailedData struct {
	EventID    int64     `json:"event_id"`
	EventType  string    `json:"event_type"`
	RecordedAt time.Time `json:"recorded_at"`
	Failures   int       `json:"failures"`
}

// EventHandler is one side effect attached to one event type. Name is
// persisted in Event.HandledBy, so it is stable. Delivery is at least once.
type EventHandler interface {
	Name() string
	Handle(ctx context.Context, event Event) error
}

// ValidEventType reports whether eventType is dot-separated lowercase words.
func ValidEventType(eventType string) bool {
	return eventTypePattern.MatchString(eventType)
}

// Validate mirrors the events table checks.
func (d DraftEvent) Validate() error {
	if !ValidEventType(d.Type) {
		return fmt.Errorf("%w: invalid event type %q", ErrInvalidInput, d.Type)
	}

	if d.Subject != nil && strings.TrimSpace(*d.Subject) == "" {
		return fmt.Errorf("%w: event subject is blank", ErrInvalidInput)
	}

	if len(d.Data) > 0 {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(d.Data, &object); err != nil || object == nil {
			return fmt.Errorf("%w: event data is not a JSON object", ErrInvalidInput)
		}
	}

	return nil
}

// Validate rejects an outcome that is both handled and failed.
func (o EventOutcome) Validate() error {
	if o.Handled && o.Failed {
		return fmt.Errorf("%w: event outcome is both handled and failed", ErrInvalidInput)
	}

	return nil
}

// Backoff is the delay after the given number of failures, doubling from BackoffBase.
func (p EventPolicy) Backoff(failures int) time.Duration {
	if failures < 1 {
		return 0
	}

	return p.BackoffBase << (failures - 1)
}

// Eligible reports whether the event is due for a handling pass at now.
func (e Event) Eligible(now time.Time) bool {
	if e.ProcessedAt != nil || e.GivenUpAt != nil || e.Expired(now) {
		return false
	}

	return e.BackoffUntil == nil || !e.BackoffUntil.After(now)
}

// Expired reports whether the event was not fully handled before ExpiredAt.
func (e Event) Expired(now time.Time) bool {
	return e.ProcessedAt == nil && !e.ExpiredAt.After(now)
}
