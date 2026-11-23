package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// SnapshotIncomplete watches for a latest price date without a completed snapshot.
type SnapshotIncomplete struct {
	store  domain.GenericStore
	events domain.EventStore
}

// NewSnapshotIncomplete builds the monitor.
func NewSnapshotIncomplete(store domain.GenericStore, events domain.EventStore) *SnapshotIncomplete {
	return &SnapshotIncomplete{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *SnapshotIncomplete) Name() string {
	return strings.TrimPrefix(domain.Events.SnapshotIncomplete, domain.NotificationPrefix)
}

// ValidateParams rejects any setting; the monitor takes none.
func (m *SnapshotIncomplete) ValidateParams(params Params) error {
	return params.Unknown()
}

// Run records one event per unresolved date.
func (m *SnapshotIncomplete) Run(ctx context.Context, _ Params) error {
	status, err := m.store.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("reading status: %w", err)
	}

	if status.LatestPriceDate == nil || !status.SnapshotMissing {
		slog.InfoContext(ctx, "snapshot monitor: nothing incomplete")

		return nil
	}

	date := *status.LatestPriceDate
	subject := date.Format(time.DateOnly)

	data, err := json.Marshal(domain.SnapshotIncompleteData{Date: date})
	if err != nil {
		return fmt.Errorf("encoding event data: %w", err)
	}

	created, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.SnapshotIncomplete, Subject: &subject, Data: data})
	if err != nil {
		return fmt.Errorf("recording event: %w", err)
	}

	if created {
		slog.InfoContext(ctx, "snapshot monitor: recorded incomplete snapshot", "date", subject)
	} else {
		slog.InfoContext(ctx, "snapshot monitor: date already announced", "date", subject)
	}

	return nil
}
