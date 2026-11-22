package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// Settings are the worker's tunables, taken from the server configuration.
type Settings struct {
	PollInterval   time.Duration // how often to look for due events
	PruneInterval  time.Duration // how often to prune handled events
	BatchLimit     int           // events taken per pass
	TypePrefix     string        // only events whose type starts with it are handled
	HandlerTimeout time.Duration // deadline for one handler call
}

// Worker drains the events table, handling the events whose type starts with
// the configured prefix. Every retry fact lives on the event.
type Worker struct {
	store    domain.Store
	source   domain.EventHandlerSource
	settings Settings
}

// NewWorker builds the events worker.
func NewWorker(store domain.Store, source domain.EventHandlerSource, settings Settings) *Worker {
	return &Worker{store: store, source: source, settings: settings}
}

// Run blocks until ctx is cancelled. A failing pass never stops the loop.
func (w *Worker) Run(ctx context.Context) {
	w.RunOnce(ctx)
	w.prune(ctx)

	poll := time.NewTicker(w.settings.PollInterval)
	defer poll.Stop()

	prune := time.NewTicker(w.settings.PruneInterval)
	defer prune.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "events worker stopped")

			return
		case <-poll.C:
			w.RunOnce(ctx)
		case <-prune.C:
			w.prune(ctx)
		}
	}
}

// RunOnce runs one pass over the due events.
func (w *Worker) RunOnce(ctx context.Context) {
	handlers, handlersErr := w.source.Handlers(ctx)
	if handlersErr != nil {
		slog.ErrorContext(ctx, "reading event handlers", "err", handlersErr)
	}

	events, err := w.store.ListDueEvents(ctx, w.settings.TypePrefix, w.settings.BatchLimit)
	if err != nil {
		slog.ErrorContext(ctx, "listing due events", "err", err)

		return
	}

	for _, event := range events {
		w.handleEvent(ctx, event, handlers, handlersErr)
	}
}

func (w *Worker) handleEvent(ctx context.Context, event domain.Event, handlers domain.EventHandlers, handlersErr error) {
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

func (w *Worker) run(ctx context.Context, handler domain.EventHandler, event domain.Event) error {
	ctx, cancel := context.WithTimeout(ctx, w.settings.HandlerTimeout)
	defer cancel()

	return handler.Handle(ctx, event)
}

func (w *Worker) save(ctx context.Context, event domain.Event, outcome domain.EventOutcome) {
	if err := w.store.SaveEventOutcome(ctx, event.ID, outcome); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			slog.WarnContext(ctx, "event removed while handling", "id", event.ID)

			return
		}

		slog.ErrorContext(ctx, "saving event outcome", "id", event.ID, "err", fmt.Sprint(err))
	}
}

func (w *Worker) prune(ctx context.Context) {
	count, err := w.store.PruneEvents(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "pruning events", "err", err)

		return
	}

	if count > 0 {
		slog.InfoContext(ctx, "pruned handled events", "count", count)
	}
}
