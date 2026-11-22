package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

type fakeStore struct {
	domain.Store

	due      []domain.Event
	prefix   string
	listErr  error
	outcomes map[int64]domain.EventOutcome
	pruned   int
}

func (s *fakeStore) ListDueEvents(_ context.Context, prefix string, _ int) ([]domain.Event, error) {
	s.prefix = prefix

	return s.due, s.listErr
}

func (s *fakeStore) SaveEventOutcome(_ context.Context, id int64, outcome domain.EventOutcome) error {
	if s.outcomes == nil {
		s.outcomes = map[int64]domain.EventOutcome{}
	}

	s.outcomes[id] = outcome

	return nil
}

func (s *fakeStore) PruneEvents(context.Context) (int, error) {
	s.pruned++

	return 0, nil
}

type fakeHandler struct {
	name  string
	err   error
	calls int
}

func (h *fakeHandler) Name() string { return h.name }

func (h *fakeHandler) Handle(context.Context, domain.Event) error {
	h.calls++

	return h.err
}

type fakeSource struct {
	handlers map[string][]domain.EventHandler
	err      error
}

func (s fakeSource) Handlers(context.Context) (Handlers, error) {
	return func(eventType string) []domain.EventHandler { return s.handlers[eventType] }, s.err
}

func TestRunOnceHandlesEveryHandler(t *testing.T) {
	a, b := &fakeHandler{name: "a"}, &fakeHandler{name: "b"}
	store := &fakeStore{due: []domain.Event{{ID: 1, Type: "x"}}}

	NewNotifications(store, fakeSource{handlers: map[string][]domain.EventHandler{"x": {a, b}}}).RunOnce(context.Background())

	assert.Equal(t, domain.EventOutcome{Succeeded: []string{"a", "b"}, Handled: true}, store.outcomes[1])
	assert.Equal(t, "notification.", store.prefix, "only notification events are taken")
}

func TestRunOnceRetriesOnlyTheFailedHandler(t *testing.T) {
	done, failing := &fakeHandler{name: "a"}, &fakeHandler{name: "b", err: errors.New("down")}
	store := &fakeStore{due: []domain.Event{{ID: 1, Type: "x", HandledBy: []string{"a"}}}}

	NewNotifications(store, fakeSource{handlers: map[string][]domain.EventHandler{"x": {done, failing}}}).RunOnce(context.Background())

	assert.Zero(t, done.calls)
	assert.Equal(t, 1, failing.calls)
	assert.Equal(t, domain.EventOutcome{Failed: true}, store.outcomes[1])
}

func TestRunOnceFailsOtherHandlersIndependently(t *testing.T) {
	failing, ok := &fakeHandler{name: "a", err: errors.New("down")}, &fakeHandler{name: "b"}
	store := &fakeStore{due: []domain.Event{{ID: 1, Type: "x"}}}

	NewNotifications(store, fakeSource{handlers: map[string][]domain.EventHandler{"x": {failing, ok}}}).RunOnce(context.Background())

	assert.Equal(t, domain.EventOutcome{Succeeded: []string{"b"}, Failed: true}, store.outcomes[1])
}

func TestRunOnceFailsAnEventWithoutHandlers(t *testing.T) {
	store := &fakeStore{due: []domain.Event{{ID: 1, Type: "unknown"}}}

	NewNotifications(store, fakeSource{handlers: map[string][]domain.EventHandler{}}).RunOnce(context.Background())

	assert.Equal(t, domain.EventOutcome{Failed: true}, store.outcomes[1])
}

func TestRunOnceFailsEventsWhenHandlersAreUnavailable(t *testing.T) {
	store := &fakeStore{due: []domain.Event{{ID: 1, Type: "x"}}}

	NewNotifications(store, fakeSource{err: errors.New("bad config")}).RunOnce(context.Background())

	assert.Equal(t, domain.EventOutcome{Failed: true}, store.outcomes[1])
}

func TestRunOnceStopsWhenListingFails(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}

	NewNotifications(store, fakeSource{}).RunOnce(context.Background())

	assert.Empty(t, store.outcomes)
}

func TestRunStopsOnCancelAfterOnePassAndPrune(t *testing.T) {
	store := &fakeStore{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	NewNotifications(store, fakeSource{}).Run(ctx)

	require.Equal(t, 1, store.pruned)
}
