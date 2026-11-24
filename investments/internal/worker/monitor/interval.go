package monitor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

// lastEventDate is the date field of the payload of the latest event of the
// type whose subject starts with the prefix, nil when there is none.
func lastEventDate(ctx context.Context, events domain.EventStore, eventType, subjectPrefix string) (*domain.Date, error) {
	event, err := events.Latest(ctx, eventType, subjectPrefix)
	if err != nil {
		return nil, fmt.Errorf("reading latest %s event: %w", eventType, err)
	}

	if event == nil {
		return nil, nil //nolint:nilnil // no earlier event
	}

	var data struct {
		Date domain.Date `json:"date"`
	}

	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, fmt.Errorf("decoding latest %s event: %w", eventType, err)
	}

	return &data.Date, nil
}

// withinInterval reports whether the last event is less than the interval
// before the latest date.
func withinInterval(last *domain.Date, interval duration.Duration, latest domain.Date) bool {
	return last != nil && last.After(interval.Before(latest))
}
