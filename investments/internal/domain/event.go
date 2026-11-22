package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// MaxEventFailures is the failed attempts after which an event is given up on.
	MaxEventFailures = 10
	// EventBackoffBase is the delay after the first failure; each further failure doubles it.
	EventBackoffBase = 30 * time.Second
	// EventMaxAge is how long after it was recorded an event is still considered for handling.
	EventMaxAge = 7 * 24 * time.Hour
	// EventRetention is how long after it was fully handled an event is kept.
	EventRetention = 365 * 24 * time.Hour
)

// EventTypes names the event types in one place.
type EventTypes struct {
	SnapshotIncomplete string
	NotificationFailed string
}

// Events holds the event type names, dot-separated lowercase words.
var Events = EventTypes{
	SnapshotIncomplete: "snapshot.incomplete",
	NotificationFailed: "notification.failed",
}

// NotifyingEvents are the types that notify, in a stable order.
var NotifyingEvents = []string{Events.SnapshotIncomplete, Events.NotificationFailed}

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

// EventBackoff is the delay after the given number of failures, doubling from EventBackoffBase.
func EventBackoff(failures int) time.Duration {
	if failures < 1 {
		return 0
	}

	return EventBackoffBase << (failures - 1)
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
