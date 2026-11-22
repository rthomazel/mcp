package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// Renderer turns an event into plain, channel-neutral text.
type Renderer interface {
	Render(ctx context.Context, event domain.Event) (string, error)
}

// SnapshotIncompleteRenderer describes the incomplete snapshot run for the event's date.
type SnapshotIncompleteRenderer struct {
	store domain.Store
}

// NotificationFailedRenderer announces an event whose delivery was given up on.
type NotificationFailedRenderer struct{}

// NewRenderers returns the renderer of each notifying event type by format.
func NewRenderers(store domain.Store) map[string]map[string]Renderer {
	return map[string]map[string]Renderer{
		domain.Events.SnapshotIncomplete: {defaultFormat: &SnapshotIncompleteRenderer{store: store}},
		domain.Events.NotificationFailed: {defaultFormat: &NotificationFailedRenderer{}},
	}
}

// Render names the date and the run's error message, or that no run was recorded.
func (r *SnapshotIncompleteRenderer) Render(ctx context.Context, event domain.Event) (string, error) {
	var data domain.SnapshotIncompleteData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding snapshot incomplete data: %w", err)
	}

	updates, err := r.store.ListUpdates(ctx, domain.UpdateFilter{
		DataType: domain.UpdateDataSnapshots, Status: domain.UpdateStatusIncomplete,
	})
	if err != nil {
		return "", fmt.Errorf("listing incomplete snapshot runs: %w", err)
	}

	date := data.Date.Format(time.DateOnly)

	for _, update := range updates {
		if update.DateFrom != nil && update.DateTo != nil && update.DateFrom.Equal(data.Date) && update.DateTo.Equal(data.Date) {
			return fmt.Sprintf("Snapshot for %s is incomplete: %s", date, update.ErrorMessage), nil
		}
	}

	return fmt.Sprintf("Snapshot for %s is incomplete: no run was recorded for it.", date), nil
}

// Render names the failed event's type, recorded time and attempts, never its payload.
func (r *NotificationFailedRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.NotificationFailedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding notification failed data: %w", err)
	}

	return fmt.Sprintf("Delivery of a %s notification recorded at %s was given up on after %d failed attempts.",
		data.EventType, data.RecordedAt.Format(time.RFC3339), data.Failures), nil
}
