package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidEventType(t *testing.T) {
	for _, ok := range []string{"snapshot.incomplete", "a", "price_move.big", "a1.b2"} {
		assert.True(t, ValidEventType(ok), ok)
	}

	for _, bad := range []string{"", "Snapshot", "a..b", ".a", "a.", "1a", "a-b", "a b"} {
		assert.False(t, ValidEventType(bad), bad)
	}
}

func TestDraftEventValidate(t *testing.T) {
	blank := " "
	subject := "2026-01-02"

	tests := map[string]struct {
		draft DraftEvent
		ok    bool
	}{
		"minimal":        {DraftEvent{Type: "a.b"}, true},
		"with subject":   {DraftEvent{Type: "a.b", Subject: &subject, Data: json.RawMessage(`{"x":1}`)}, true},
		"bad type":       {DraftEvent{Type: "A"}, false},
		"blank subject":  {DraftEvent{Type: "a", Subject: &blank}, false},
		"array data":     {DraftEvent{Type: "a", Data: json.RawMessage(`[]`)}, false},
		"null data":      {DraftEvent{Type: "a", Data: json.RawMessage(`null`)}, false},
		"malformed data": {DraftEvent{Type: "a", Data: json.RawMessage(`{`)}, false},
	}

	for name, tc := range tests {
		err := tc.draft.Validate()
		if tc.ok {
			assert.NoError(t, err, name)
		} else {
			assert.ErrorIs(t, err, ErrInvalidInput, name)
		}
	}
}

func TestIsNotification(t *testing.T) {
	assert.True(t, IsNotification(Events.SnapshotIncomplete))
	assert.True(t, IsNotification(Events.NotificationFailed))
	assert.False(t, IsNotification("price.moved"))
	assert.False(t, IsNotification("notifications.x"))
}

func TestEventOutcomeValidate(t *testing.T) {
	assert.NoError(t, EventOutcome{Handled: true}.Validate())
	assert.ErrorIs(t, EventOutcome{Handled: true, Failed: true}.Validate(), ErrInvalidInput)
}

func TestEventBackoffDoubles(t *testing.T) {
	policy := EventPolicy{BackoffBase: 30 * time.Second}

	assert.Equal(t, time.Duration(0), policy.Backoff(0))
	assert.Equal(t, 30*time.Second, policy.Backoff(1))
	assert.Equal(t, time.Minute, policy.Backoff(2))
	assert.Equal(t, 30*time.Second<<8, policy.Backoff(9))
}

func TestEventEligibleAndExpired(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	base := Event{ExpiredAt: future}

	assert.True(t, base.Eligible(now))

	processed := base
	processed.ProcessedAt = &past
	assert.False(t, processed.Eligible(now))
	assert.False(t, processed.Expired(now))

	givenUp := base
	givenUp.GivenUpAt = &past
	assert.False(t, givenUp.Eligible(now))
	assert.False(t, givenUp.Expired(now))

	backoff := base
	backoff.BackoffUntil = &future
	assert.False(t, backoff.Eligible(now))
	backoff.BackoffUntil = &past
	assert.True(t, backoff.Eligible(now))

	expired := Event{ExpiredAt: now}
	assert.True(t, expired.Expired(now))
	assert.False(t, expired.Eligible(now))
}

func TestSummarizeReportsGivenUpEventsButNotExpired(t *testing.T) {
	status := Status{ExpiredEvents: []Event{{ID: 1}}}
	status.Summarize()
	assert.True(t, status.Healthy)

	status.GivenUpEvents = []Event{{ID: 2}}
	status.Summarize()
	assert.False(t, status.Healthy)
	assert.Len(t, status.Reasons, 1)
}
