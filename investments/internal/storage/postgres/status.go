package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	heldFreshnessSQL = `
SELECT i.id, i.symbol, i.exchange, p.date, ($1::date - p.date) AS age_days
FROM (
    SELECT instrument_id FROM transactions
    WHERE effective_date <= $1::date
    GROUP BY instrument_id
    HAVING SUM(quantity_change) > 0
) held
JOIN instruments i ON i.id = held.instrument_id
LEFT JOIN LATERAL (
    SELECT MAX(date) AS date FROM prices WHERE instrument_id = i.id AND date <= $1::date
) p ON true
ORDER BY i.symbol, i.exchange`

	// A price date is covered by a completed snapshots run for it or a completed rebuild containing it.
	latestCompletedSnapshotSQL = `
SELECT MAX(p.date)
FROM (SELECT DISTINCT date FROM prices) p
WHERE EXISTS (
    SELECT 1 FROM updates
    WHERE status = 'completed'
      AND ((data_type = 'snapshots' AND date_from = p.date AND date_to = p.date)
        OR (data_type = 'snapshot_rebuild' AND date_from <= p.date AND date_to >= p.date))
)`

	// Unresolved failed or incomplete runs: a snapshot run is superseded by a
	// completed run for its date, a prices run by a later completed prices
	// run, a rebuild by a later completed rebuild covering its range.
	attentionSQL = `
SELECT u.id, u.provider, u.data_type, u.started_at, u.completed_at, u.status, u.records_updated,
       u.error_message, u.date_from, u.date_to, u.reason
FROM updates u
WHERE u.status IN ('failed', 'incomplete')
  AND NOT CASE u.data_type
      WHEN 'snapshots' THEN EXISTS (
          SELECT 1 FROM updates n
          WHERE n.status = 'completed'
            AND ((n.data_type = 'snapshots' AND n.date_from = u.date_from AND n.date_to = u.date_to)
              OR (n.data_type = 'snapshot_rebuild' AND n.date_from <= u.date_from AND n.date_to >= u.date_to)))
      WHEN 'prices' THEN EXISTS (
          SELECT 1 FROM updates n
          WHERE n.status = 'completed' AND n.data_type = 'prices' AND n.started_at > u.started_at)
      WHEN 'snapshot_rebuild' THEN EXISTS (
          SELECT 1 FROM updates n
          WHERE n.status = 'completed' AND n.data_type = 'snapshot_rebuild' AND n.started_at > u.started_at
            AND n.date_from <= u.date_from AND n.date_to >= u.date_to)
      ELSE false
  END
ORDER BY u.started_at DESC, u.id DESC
LIMIT $1`
)

// GetStatus aggregates price freshness of held instruments, snapshot coverage
// and unresolved failed runs into a health view.
func (s *Store) GetStatus(ctx context.Context) (domain.Status, error) {
	today := s.referenceDate(nil)
	status := domain.Status{Date: today}

	instruments, err := s.heldInstrumentStatus(ctx, today)
	if err != nil {
		return domain.Status{}, err
	}

	status.Instruments = instruments

	if status.LatestPriceDate, err = s.LatestSessionDate(ctx); err != nil {
		return domain.Status{}, err
	}

	var latestSnapshot sql.NullTime
	if err := s.db.GetContext(ctx, &latestSnapshot, latestCompletedSnapshotSQL); err != nil {
		return domain.Status{}, fmt.Errorf("reading latest completed snapshot: %w", err)
	}

	if latestSnapshot.Valid {
		status.LatestSnapshotDate = &latestSnapshot.Time
	}

	if status.LatestPriceDate != nil {
		var done bool
		if err := s.db.GetContext(ctx, &done, completedDateSQL, *status.LatestPriceDate); err != nil {
			return domain.Status{}, fmt.Errorf("checking latest snapshot: %w", err)
		}

		status.SnapshotMissing = !done
	}

	if status.NeedsAttention, err = s.queryUpdates(ctx, attentionSQL, domain.UpdateFilter{}.ResolvedLimit()); err != nil {
		return domain.Status{}, err
	}

	now := s.clock()

	if status.GivenUpEvents, err = s.queryEvents(ctx, s.db, givenUpEventsSQL, now, domain.UpdateFilter{}.ResolvedLimit()); err != nil {
		return domain.Status{}, err
	}

	if status.ExpiredEvents, err = s.queryEvents(ctx, s.db, expiredEventsSQL, now, domain.UpdateFilter{}.ResolvedLimit()); err != nil {
		return domain.Status{}, err
	}

	status.Summarize()

	return status, nil
}

func (s *Store) heldInstrumentStatus(ctx context.Context, today domain.Date) ([]domain.InstrumentStatus, error) {
	rows, err := s.db.QueryxContext(ctx, heldFreshnessSQL, today)
	if err != nil {
		return nil, fmt.Errorf("reading held instruments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	instruments := []domain.InstrumentStatus{}

	for rows.Next() {
		var (
			instrument domain.InstrumentStatus
			priceDate  sql.NullTime
			ageDays    sql.NullInt64
		)

		if err := rows.Scan(&instrument.InstrumentID, &instrument.Symbol, &instrument.Exchange, &priceDate, &ageDays); err != nil {
			return nil, fmt.Errorf("scanning held instrument: %w", err)
		}

		instrument.PriceIssue, instrument.LatestPriceDate = s.priceIssue(priceDate, ageDays)
		instruments = append(instruments, instrument)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading held instruments: %w", err)
	}

	return instruments, nil
}
