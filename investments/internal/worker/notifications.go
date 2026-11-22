package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	pollInterval   = 30 * time.Second
	pruneInterval  = 24 * time.Hour
	batchLimit     = 100
	handlerTimeout = 30 * time.Second
)

// HandlerSource provides the handlers registered for each event type, read
// fresh on every pass so configuration changes apply without a restart.
type HandlerSource interface {
	Handlers(ctx context.Context) (Handlers, error)
}

// Handlers returns the handlers registered for an event type; none means no
// handler is registered.
type Handlers func(eventType string) []domain.EventHandler

// Notifications drains the outbox. Every retry fact lives on the event.
type Notifications struct {
	store  domain.Store
	source HandlerSource
}

// NewNotifications builds the notifications worker.
func NewNotifications(store domain.Store, source HandlerSource) *Notifications {
	return &Notifications{store: store, source: source}
}

// Run blocks until ctx is cancelled. A failing pass never stops the loop.
func (w *Notifications) Run(ctx context.Context) {
	w.RunOnce(ctx)
	w.prune(ctx)

	poll := time.NewTicker(pollInterval)
	defer poll.Stop()

	prune := time.NewTicker(pruneInterval)
	defer prune.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "outbox worker stopped")

			return
		case <-poll.C:
			w.RunOnce(ctx)
		case <-prune.C:
			w.prune(ctx)
		}
	}
}

// RunOnce runs one pass over the due events.
func (w *Notifications) RunOnce(ctx context.Context) {
	handlers, handlersErr := w.source.Handlers(ctx)
	if handlersErr != nil {
		slog.ErrorContext(ctx, "reading event handlers", "err", handlersErr)
	}

	events, err := w.store.ListDueEvents(ctx, domain.NotificationPrefix, batchLimit)
	if err != nil {
		slog.ErrorContext(ctx, "listing due events", "err", err)

		return
	}

	for _, event := range events {
		w.handleEvent(ctx, event, handlers, handlersErr)
	}
}

func (w *Notifications) handleEvent(ctx context.Context, event domain.Event, handlers Handlers, handlersErr error) {
	if handlersErr != nil {
		w.save(ctx, event, domain.EventOutcome{Failed: true})

		return
	}

	registered := handlers(event.Type)
	if len(registered) == 0 {
		slog.WarnContext(ctx, "no handler registered for event type, leaving the event untouched", "id", event.ID, "type", event.Type)

		return
	}

	outcome := domain.EventOutcome{}

	for _, handler := range registered {
		name := handler.Name()
		if slices.Contains(event.HandledBy, name) {
			continue
		}

		if err := w.run(ctx, handler, event); err != nil {
			slog.ErrorContext(ctx, "event handler failed", "id", event.ID, "type", event.Type, "handler", name, "err", err)

			outcome.Failed = true

			continue
		}

		outcome.Succeeded = append(outcome.Succeeded, name)
	}

	outcome.Handled = !outcome.Failed

	w.save(ctx, event, outcome)
}

func (w *Notifications) run(ctx context.Context, handler domain.EventHandler, event domain.Event) error {
	ctx, cancel := context.WithTimeout(ctx, handlerTimeout)
	defer cancel()

	return handler.Handle(ctx, event)
}

func (w *Notifications) save(ctx context.Context, event domain.Event, outcome domain.EventOutcome) {
	if err := w.store.SaveEventOutcome(ctx, event.ID, outcome); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			slog.WarnContext(ctx, "event removed while handling", "id", event.ID)

			return
		}

		slog.ErrorContext(ctx, "saving event outcome", "id", event.ID, "err", fmt.Sprint(err))
	}
}

func (w *Notifications) prune(ctx context.Context) {
	count, err := w.store.PruneEvents(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "pruning events", "err", err)

		return
	}

	if count > 0 {
		slog.InfoContext(ctx, "pruned handled events", "count", count)
	}
}
