package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	eventColumns = `id, type, subject, data, failures, backoff_until, created_at, expired_at, processed_at, given_up_at, handled_by`

	insertEventSQL = `
INSERT INTO events (type, subject, data, created_at, expired_at)
VALUES ($1, $2, $3, $4::timestamptz, $4::timestamptz + make_interval(secs => $5::float8))
ON CONFLICT (type, subject) WHERE subject IS NOT NULL DO NOTHING
RETURNING id`

	dueEventsSQL = `
SELECT ` + eventColumns + ` FROM events
WHERE processed_at IS NULL AND given_up_at IS NULL
  AND expired_at > $1
  AND (backoff_until IS NULL OR backoff_until <= $1)
  AND starts_with(type, $2)
ORDER BY created_at, id
LIMIT $3`

	latestEventSQL = `
SELECT ` + eventColumns + ` FROM events WHERE type = $1 ORDER BY created_at DESC, id DESC LIMIT 1`

	lockEventSQL = `SELECT ` + eventColumns + ` FROM events WHERE id = $1 FOR UPDATE`

	saveEventSQL = `
UPDATE events
SET handled_by = $2, processed_at = $3, failures = $4, backoff_until = $5, given_up_at = $6
WHERE id = $1`

	removeEventSQL = `
DELETE FROM events
WHERE id = $1 AND processed_at IS NULL AND (given_up_at IS NOT NULL OR expired_at <= $2)
RETURNING id`

	eventExistsSQL = `SELECT EXISTS (SELECT 1 FROM events WHERE id = $1)`

	pruneEventsSQL = `DELETE FROM events WHERE processed_at <= $1`

	givenUpEventsSQL = `
SELECT ` + eventColumns + ` FROM events
WHERE processed_at IS NULL AND given_up_at IS NOT NULL AND expired_at > $1
ORDER BY created_at DESC, id DESC
LIMIT $2`

	expiredEventsSQL = `
SELECT ` + eventColumns + ` FROM events
WHERE processed_at IS NULL AND expired_at <= $1
ORDER BY created_at DESC, id DESC
LIMIT $2`
)

// Record validates draft and inserts it into the outbox.
func (s *Events) Record(ctx context.Context, draft domain.DraftEvent) (bool, error) {
	if err := draft.Validate(); err != nil {
		return false, err
	}

	var created bool

	err := s.withTx(ctx, func(tx *sqlx.Tx) error {
		var err error

		created, err = s.insertEvent(ctx, tx, draft, s.clock())

		return err
	})
	if err != nil {
		return false, err
	}

	return created, nil
}

// insertEvent is the one way an event is written. A change that must record
// an event in its own unit of work calls it with its transaction.
func (s *Events) insertEvent(ctx context.Context, tx *sqlx.Tx, draft domain.DraftEvent, now time.Time) (bool, error) {
	data := draft.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}

	var id int64

	err := tx.GetContext(ctx, &id, insertEventSQL, draft.Type, draft.Subject, []byte(data), now, s.events.MaxAge.Seconds())
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("inserting event: %w", err)
	}

	return true, nil
}

// ListDue returns up to limit events due for a handling pass whose type starts with typePrefix.
func (s *Events) ListDue(ctx context.Context, typePrefix string, limit int) ([]domain.Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w: limit must be positive", domain.ErrInvalidInput)
	}

	return s.queryEvents(ctx, s.db, dueEventsSQL, s.clock(), typePrefix, limit)
}

// Latest returns the most recently recorded event of the type, nil when there is none.
func (s *Events) Latest(ctx context.Context, eventType string) (*domain.Event, error) {
	events, err := s.queryEvents(ctx, s.db, latestEventSQL, eventType)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil //nolint:nilnil // no event of the type
	}

	return &events[0], nil
}

// SaveOutcome applies the outcome of one handling pass in one transaction.
func (s *Events) SaveOutcome(ctx context.Context, id int64, outcome domain.EventOutcome) error {
	if err := outcome.Validate(); err != nil {
		return err
	}

	now := s.clock()

	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		events, err := s.queryEvents(ctx, tx, lockEventSQL, id)
		if err != nil {
			return err
		}

		if len(events) == 0 {
			return domain.ErrEventNotFound
		}

		event := events[0]

		handled := slices.Clone(event.HandledBy)

		for _, name := range outcome.Succeeded {
			if !slices.Contains(handled, name) {
				handled = append(handled, name)
			}
		}

		failures, backoffUntil, givenUpAt, processedAt := event.Failures, event.BackoffUntil, event.GivenUpAt, event.ProcessedAt

		if outcome.Failed {
			failures++

			next := now.Add(s.events.Backoff(failures))
			backoffUntil = &next

			if failures >= s.events.MaxFailures {
				givenUpAt = &now

				if err := s.giveUp(ctx, tx, event, failures, now); err != nil {
					return err
				}
			}
		}

		if outcome.Handled {
			processedAt = &now
		}

		encoded, err := json.Marshal(handled)
		if err != nil {
			return fmt.Errorf("encoding handled names: %w", err)
		}

		if _, err := tx.ExecContext(ctx, saveEventSQL, id, encoded, processedAt, failures, backoffUntil, givenUpAt); err != nil {
			return fmt.Errorf("saving event outcome: %w", err)
		}

		return nil
	})
}

// giveUp logs the failure and announces it, except for a failed announcement,
// which only logs so a failing channel cannot cascade.
func (s *Events) giveUp(ctx context.Context, tx *sqlx.Tx, event domain.Event, failures int, now time.Time) error {
	slog.Error("event given up on after repeated failures", "id", event.ID, "type", event.Type, "failures", failures)

	if event.Type == domain.Events.NotificationFailed {
		return nil
	}

	data, err := json.Marshal(domain.NotificationFailedData{
		EventID: event.ID, EventType: event.Type, RecordedAt: event.CreatedAt, Failures: failures,
	})
	if err != nil {
		return fmt.Errorf("encoding notification failed data: %w", err)
	}

	if _, err := s.insertEvent(ctx, tx, domain.DraftEvent{Type: domain.Events.NotificationFailed, Data: data}, now); err != nil {
		return err
	}

	return nil
}

// Remove deletes an event that was given up on or has expired.
func (s *Events) Remove(ctx context.Context, id int64) error {
	var removed int64

	err := s.db.GetContext(ctx, &removed, removeEventSQL, id, s.clock())
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("removing event: %w", err)
	}

	var exists bool
	if err := s.db.GetContext(ctx, &exists, eventExistsSQL, id); err != nil {
		return fmt.Errorf("looking up event: %w", err)
	}

	if !exists {
		return domain.ErrEventNotFound
	}

	return fmt.Errorf("%w: event %d", domain.ErrEventNotRemovable, id)
}

// Prune deletes fully handled events kept for the retention or longer.
func (s *Events) Prune(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx, pruneEventsSQL, s.clock().Add(-s.events.Retention))
	if err != nil {
		return 0, fmt.Errorf("pruning events: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("counting pruned events: %w", err)
	}

	return int(count), nil
}

type eventQuerier interface {
	QueryxContext(ctx context.Context, query string, args ...any) (*sqlx.Rows, error)
}

// queryEvents runs a query selecting eventColumns.
func (s *Store) queryEvents(ctx context.Context, q eventQuerier, query string, args ...any) ([]domain.Event, error) {
	rows, err := q.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	events := []domain.Event{}

	for rows.Next() {
		var (
			event                           domain.Event
			subject                         sql.NullString
			data, handled                   []byte
			backoffUntil, processed, gaveUp sql.NullTime
		)

		if err := rows.Scan(&event.ID, &event.Type, &subject, &data, &event.Failures, &backoffUntil,
			&event.CreatedAt, &event.ExpiredAt, &processed, &gaveUp, &handled); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		if subject.Valid {
			event.Subject = &subject.String
		}

		event.Data = data

		if err := json.Unmarshal(handled, &event.HandledBy); err != nil {
			return nil, fmt.Errorf("decoding handled names: %w", err)
		}

		event.BackoffUntil = nullTimePtr(backoffUntil)
		event.ProcessedAt = nullTimePtr(processed)
		event.GivenUpAt = nullTimePtr(gaveUp)
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading events: %w", err)
	}

	return events, nil
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}

	return &t.Time
}
