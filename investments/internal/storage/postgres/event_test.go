package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func setClock(store *Store, now *time.Time) {
	store.clock = func() time.Time { return *now }
}

func recordDraft(t *testing.T, store *Store, eventType string, subject *string) {
	t.Helper()

	_, err := store.RecordEvent(context.Background(), domain.DraftEvent{Type: eventType, Subject: subject})
	require.NoError(t, err)
}

func TestRecordEventIsOncePerSubject(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	subject := "2026-01-02"
	draft := domain.DraftEvent{Type: "a.b", Subject: &subject}

	created, err := store.RecordEvent(ctx, draft)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = store.RecordEvent(ctx, draft)
	require.NoError(t, err)
	assert.False(t, created)

	// Events without a subject never collide.
	for range 2 {
		created, err = store.RecordEvent(ctx, domain.DraftEvent{Type: "a.b"})
		require.NoError(t, err)
		assert.True(t, created)
	}

	_, err = store.RecordEvent(ctx, domain.DraftEvent{Type: "Bad"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestListDueEventsRespectsEligibility(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	setClock(store, &now)

	recordDraft(t, store, "a.one", nil)
	recordDraft(t, store, "a.two", nil)

	due, err := store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, due, 2)
	assert.Equal(t, "a.one", due[0].Type)
	assert.Equal(t, now.Add(domain.EventMaxAge), due[0].ExpiredAt.UTC())
	assert.Empty(t, due[0].HandledBy)

	_, err = store.ListDueEvents(ctx, "", 0)
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	limited, err := store.ListDueEvents(ctx, "", 1)
	require.NoError(t, err)
	assert.Len(t, limited, 1)

	// A failure postpones the event until its backoff passes.
	require.NoError(t, store.SaveEventOutcome(ctx, due[0].ID, domain.EventOutcome{Failed: true}))

	due, err = store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "a.two", due[0].Type)

	now = now.Add(domain.EventBackoff(1))
	due, err = store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	assert.Len(t, due, 2)

	// Past ExpiredAt nothing is due.
	now = now.Add(domain.EventMaxAge)
	due, err = store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	assert.Empty(t, due)
}

func TestSaveEventOutcomeMergesHandlersAndMarksHandled(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	recordDraft(t, store, "a.b", nil)

	due, err := store.ListDueEvents(ctx, "", 1)
	require.NoError(t, err)

	id := due[0].ID

	require.NoError(t, store.SaveEventOutcome(ctx, id, domain.EventOutcome{Succeeded: []string{"x"}, Failed: true}))
	require.NoError(t, store.SaveEventOutcome(ctx, id, domain.EventOutcome{Succeeded: []string{"x", "y"}, Handled: true}))

	events, err := store.queryEvents(ctx, store.db, lockEventSQL, id)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, []string{"x", "y"}, events[0].HandledBy)
	assert.Equal(t, 1, events[0].Failures)
	assert.NotNil(t, events[0].ProcessedAt)

	assert.ErrorIs(t, store.SaveEventOutcome(ctx, id+100, domain.EventOutcome{}), domain.ErrEventNotFound)
	assert.ErrorIs(t, store.SaveEventOutcome(ctx, id, domain.EventOutcome{Handled: true, Failed: true}), domain.ErrInvalidInput)
}

func TestGivingUpAnnouncesOnceAndNeverCascades(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	setClock(store, &now)
	recordDraft(t, store, domain.Events.SnapshotIncomplete, nil)

	for range domain.MaxEventFailures {
		due, err := store.ListDueEvents(ctx, "", 10)
		require.NoError(t, err)
		require.NotEmpty(t, due)

		for _, event := range due {
			if event.Type == domain.Events.SnapshotIncomplete {
				require.NoError(t, store.SaveEventOutcome(ctx, event.ID, domain.EventOutcome{Failed: true}))
			}
		}

		now = now.Add(3 * time.Hour)
	}

	status, err := store.GetStatus(ctx)
	require.NoError(t, err)
	require.Len(t, status.GivenUpEvents, 1)
	assert.Equal(t, domain.MaxEventFailures, status.GivenUpEvents[0].Failures)
	assert.False(t, status.Healthy)

	due, err := store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, domain.Events.NotificationFailed, due[0].Type)

	var data domain.NotificationFailedData
	require.NoError(t, json.Unmarshal(due[0].Data, &data))
	assert.Equal(t, domain.Events.SnapshotIncomplete, data.EventType)
	assert.Equal(t, domain.MaxEventFailures, data.Failures)

	// The announcement itself failing gives up without another announcement.
	for range domain.MaxEventFailures {
		require.NoError(t, store.SaveEventOutcome(ctx, due[0].ID, domain.EventOutcome{Failed: true}))
	}

	status, err = store.GetStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, status.GivenUpEvents, 2)
}

func TestRemoveEvent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	setClock(store, &now)

	recordDraft(t, store, "a.pending", nil)
	recordDraft(t, store, "a.handled", nil)
	recordDraft(t, store, "a.failing", nil)

	due, err := store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, due, 3)

	require.NoError(t, store.SaveEventOutcome(ctx, due[1].ID, domain.EventOutcome{Handled: true}))

	for range domain.MaxEventFailures {
		require.NoError(t, store.SaveEventOutcome(ctx, due[2].ID, domain.EventOutcome{Failed: true}))
	}

	assert.ErrorIs(t, store.RemoveEvent(ctx, due[0].ID), domain.ErrEventNotRemovable)
	assert.ErrorIs(t, store.RemoveEvent(ctx, due[1].ID), domain.ErrEventNotRemovable)
	assert.ErrorIs(t, store.RemoveEvent(ctx, 99999), domain.ErrEventNotFound)
	require.NoError(t, store.RemoveEvent(ctx, due[2].ID))
	assert.ErrorIs(t, store.RemoveEvent(ctx, due[2].ID), domain.ErrEventNotFound)

	// A pending event becomes removable once expired, and shows as expired.
	now = now.Add(domain.EventMaxAge)

	status, err := store.GetStatus(ctx)
	require.NoError(t, err)
	// The pending event and the announcement of the removed failing one.
	assert.Len(t, status.ExpiredEvents, 2)
	assert.Empty(t, status.GivenUpEvents)
	require.NoError(t, store.RemoveEvent(ctx, due[0].ID))
}

func TestPruneEventsKeepsUnhandledAndRecentEvents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	setClock(store, &now)

	recordDraft(t, store, "a.old", nil)
	recordDraft(t, store, "a.pending", nil)

	due, err := store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	require.NoError(t, store.SaveEventOutcome(ctx, due[0].ID, domain.EventOutcome{Handled: true}))

	count, err := store.PruneEvents(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)

	now = now.Add(domain.EventRetention + time.Hour)

	count, err = store.PruneEvents(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	status, err := store.GetStatus(ctx)
	require.NoError(t, err)
	assert.Len(t, status.ExpiredEvents, 1)
}

func TestListDueEventsFiltersByTypePrefix(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	recordDraft(t, store, "notification.a", nil)
	recordDraft(t, store, "notificationx.b", nil)
	recordDraft(t, store, "other.c", nil)

	due, err := store.ListDueEvents(ctx, "notification.", 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "notification.a", due[0].Type)

	due, err = store.ListDueEvents(ctx, "", 10)
	require.NoError(t, err)
	assert.Len(t, due, 3)
}
