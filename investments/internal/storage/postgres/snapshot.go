package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// snapshotLockKey is the advisory lock key serializing snapshot writers.
const snapshotLockKey = 5202611

const (
	// completedDateSQL: a completed snapshots run for exactly the date, or a
	// completed rebuild whose range contains it.
	completedDateSQL = `
SELECT EXISTS (
    SELECT 1 FROM updates
    WHERE status = 'completed'
      AND ((data_type = 'snapshots' AND date_from = $1::date AND date_to = $1::date)
        OR (data_type = 'snapshot_rebuild' AND date_from <= $1::date AND date_to >= $1::date))
)`
	snapshotRowsExistSQL = `SELECT EXISTS (SELECT 1 FROM snapshots WHERE date = $1::date)`
	insertSnapshotSQL    = `
INSERT INTO snapshots (date, instrument_id, quantity, close_price, price_date)
VALUES ($1::date, $2, $3::numeric, $4::numeric, $5::date)`
	deleteSnapshotRangeSQL = `DELETE FROM snapshots WHERE date BETWEEN $1::date AND $2::date`
	priceDatesSQL          = `SELECT DISTINCT date FROM prices WHERE date BETWEEN $1::date AND $2::date ORDER BY date`
	latestSessionDateSQL   = `SELECT MAX(date) FROM prices`
	// The total is taken over every row of the date before the instrument filter applies.
	listSnapshotsSQL = `
WITH totals AS (
    SELECT date, SUM(market_value_usd) AS total
    FROM snapshots
    WHERE ($2::date IS NULL OR date >= $2) AND ($3::date IS NULL OR date <= $3)
    GROUP BY date
)
SELECT s.date, s.instrument_id, i.symbol, i.exchange, s.quantity::text, s.close_price::text, s.price_date,
       s.market_value_usd::text, t.total::text, (s.market_value_usd / t.total)::text
FROM snapshots s
JOIN instruments i ON i.id = s.instrument_id
JOIN totals t ON t.date = s.date
WHERE ($1::int IS NULL OR s.instrument_id = $1)
  AND ($2::date IS NULL OR s.date >= $2) AND ($3::date IS NULL OR s.date <= $3)
ORDER BY s.date DESC, i.symbol, i.exchange
LIMIT $4`
	listUpdatesSQL = `
SELECT id, provider, data_type, started_at, completed_at, status, records_updated,
       error_message, date_from, date_to, reason
FROM updates
WHERE ($1::text IS NULL OR data_type = $1) AND ($2::text IS NULL OR status = $2)
ORDER BY started_at DESC, id DESC
LIMIT $3`
	insertUpdateSQL = `
INSERT INTO updates (provider, data_type, started_at, completed_at, status, records_updated,
                     error_message, date_from, date_to, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, $9::date, $10)`
)

// LatestSessionDate returns the latest price date across all instruments.
func (s *Store) LatestSessionDate(ctx context.Context) (*domain.Date, error) {
	var latest sql.NullTime

	if err := s.db.GetContext(ctx, &latest, latestSessionDateSQL); err != nil {
		slog.ErrorContext(ctx, "reading latest session date failed", "err", err)

		return nil, fmt.Errorf("reading latest session date: %w", err)
	}

	if !latest.Valid {
		return nil, nil //nolint:nilnil // nil means no prices
	}

	return &latest.Time, nil
}

// RecordUpdate appends one row to the updates log.
func (s *Store) RecordUpdate(ctx context.Context, update domain.Update) error {
	if err := update.Validate(); err != nil {
		return err
	}

	if err := insertUpdate(ctx, s.db, update); err != nil {
		slog.ErrorContext(ctx, "recording update failed", "data_type", update.DataType, "err", err)

		return fmt.Errorf("recording update: %w", err)
	}

	return nil
}

func insertUpdate(ctx context.Context, ext sqlx.ExecerContext, update domain.Update) error {
	_, err := ext.ExecContext(ctx, insertUpdateSQL,
		update.Provider, update.DataType, update.StartedAt, update.CompletedAt, update.Status,
		update.RecordsUpdated, nullIfEmpty(update.ErrorMessage), update.DateFrom, update.DateTo,
		nullIfEmpty(update.Reason))

	return err
}

func nullIfEmpty(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

// WriteSnapshot writes the snapshot rows for one date and logs the outcome in
// one transaction, so the idempotency marker never disagrees with the rows.
func (s *Store) WriteSnapshot(ctx context.Context, date domain.Date) (domain.SnapshotResult, error) {
	if err := s.checkNotFuture(date); err != nil {
		return domain.SnapshotResult{}, err
	}

	result := domain.SnapshotResult{Date: date}
	started := time.Now()

	fail := func(step string, err error) error {
		slog.ErrorContext(ctx, "writing snapshot failed", "step", step, "date", date.Format(time.DateOnly), "err", err)

		return fmt.Errorf("%s: %w", step, err)
	}

	err := s.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockSnapshots(ctx, tx); err != nil {
			return fail("locking snapshots", err)
		}

		var done bool
		if err := tx.GetContext(ctx, &done, completedDateSQL, date); err != nil {
			return fail("checking completed date", err)
		}

		if done {
			result.Skipped, result.Complete = true, true

			return nil
		}

		holdings, _, err := s.valuedHoldings(ctx, tx, date)
		if err != nil {
			return fail("valuing holdings", err)
		}

		result.Issues = snapshotIssues(holdings, date)
		update := domain.Update{
			DataType: domain.UpdateDataSnapshots, StartedAt: started,
			DateFrom: &date, DateTo: &date,
		}

		if len(result.Issues) > 0 {
			slog.WarnContext(ctx, "snapshot incomplete, nothing written",
				"date", date.Format(time.DateOnly), "issues", domain.DescribeIssues(result.Issues))

			update.Status = domain.UpdateStatusIncomplete
			update.ErrorMessage = domain.DescribeIssues(result.Issues)
		} else {
			var exists bool
			if err := tx.GetContext(ctx, &exists, snapshotRowsExistSQL, date); err != nil {
				return fail("checking existing rows", err)
			}

			if exists {
				slog.ErrorContext(ctx, "snapshot rows exist for an uncompleted date", "date", date.Format(time.DateOnly))

				return fmt.Errorf("%w: %s", domain.ErrSnapshotExists, date.Format(time.DateOnly))
			}

			if err := insertSnapshotRows(ctx, tx, date, holdings); err != nil {
				return fail("inserting rows", err)
			}

			result.Complete, result.Rows = true, len(holdings)
			update.Status, update.RecordsUpdated = domain.UpdateStatusCompleted, result.Rows
		}

		update.CompletedAt = time.Now()
		if err := insertUpdate(ctx, tx, update); err != nil {
			return fail("recording update", err)
		}

		return nil
	})
	if err != nil {
		return domain.SnapshotResult{}, err
	}

	return result, nil
}

// RebuildSnapshots replaces the snapshots for every price date in the range,
// all or nothing.
func (s *Store) RebuildSnapshots(ctx context.Context, from, to domain.Date, reason string) (domain.RebuildResult, error) {
	reason = strings.TrimSpace(reason)

	switch {
	case reason == "":
		return domain.RebuildResult{}, fmt.Errorf("%w: a rebuild needs a reason", domain.ErrInvalidInput)
	case from.After(to):
		return domain.RebuildResult{}, fmt.Errorf("%w: rebuild range is inverted", domain.ErrInvalidInput)
	}

	if err := s.checkNotFuture(to); err != nil {
		return domain.RebuildResult{}, err
	}

	result := domain.RebuildResult{From: from, To: to}
	started := time.Now()

	fail := func(step string, err error) error {
		slog.ErrorContext(ctx, "rebuilding snapshots failed", "step", step,
			"from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly), "err", err)

		return fmt.Errorf("%s: %w", step, err)
	}

	err := s.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockSnapshots(ctx, tx); err != nil {
			return fail("locking snapshots", err)
		}

		if _, err := tx.ExecContext(ctx, deleteSnapshotRangeSQL, from, to); err != nil {
			return fail("deleting range", err)
		}

		var dates []time.Time
		if err := tx.SelectContext(ctx, &dates, priceDatesSQL, from, to); err != nil {
			return fail("selecting price dates", err)
		}

		for _, day := range dates {
			holdings, _, err := s.valuedHoldings(ctx, tx, day)
			if err != nil {
				return fail("valuing holdings", err)
			}

			if issues := snapshotIssues(holdings, day); len(issues) > 0 {
				return &domain.IncompleteSnapshotError{Date: day, Issues: issues}
			}

			if err := insertSnapshotRows(ctx, tx, day, holdings); err != nil {
				return fail("inserting rows", err)
			}

			result.Rows += len(holdings)
		}

		result.Dates = len(dates)

		err := insertUpdate(ctx, tx, domain.Update{
			DataType: domain.UpdateDataSnapshotRebuild, StartedAt: started, CompletedAt: time.Now(),
			Status: domain.UpdateStatusCompleted, RecordsUpdated: result.Rows,
			DateFrom: &from, DateTo: &to, Reason: reason,
		})
		if err != nil {
			return fail("recording update", err)
		}

		return nil
	})
	if err != nil {
		var incomplete *domain.IncompleteSnapshotError
		if errors.As(err, &incomplete) {
			slog.ErrorContext(ctx, "snapshot rebuild incomplete, nothing changed",
				"date", incomplete.Date.Format(time.DateOnly), "issues", domain.DescribeIssues(incomplete.Issues))

			s.recordFailedRebuild(ctx, started, from, to, reason, err)
		}

		return domain.RebuildResult{}, err
	}

	slog.InfoContext(ctx, "snapshots rebuilt", "from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly),
		"reason", reason, "dates", result.Dates, "rows", result.Rows)

	return result, nil
}

// recordFailedRebuild logs the failed rebuild after its transaction rolled
// back. A failure to log is itself logged and not returned.
func (s *Store) recordFailedRebuild(ctx context.Context, started time.Time, from, to domain.Date, reason string, cause error) {
	err := insertUpdate(ctx, s.db, domain.Update{
		DataType: domain.UpdateDataSnapshotRebuild, StartedAt: started, CompletedAt: time.Now(),
		Status: domain.UpdateStatusFailed, ErrorMessage: cause.Error(),
		DateFrom: &from, DateTo: &to, Reason: reason,
	})
	if err != nil {
		slog.ErrorContext(ctx, "recording failed rebuild failed", "err", err)
	}
}

func (s *Store) checkNotFuture(date domain.Date) error {
	if date.After(s.referenceDate(nil)) {
		return fmt.Errorf("%w: %s is in the future", domain.ErrInvalidInput, date.Format(time.DateOnly))
	}

	return nil
}

func lockSnapshots(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", snapshotLockKey)

	return err
}

// snapshotIssues lists holdings whose price is not dated exactly on date.
func snapshotIssues(holdings []domain.Holding, date domain.Date) []domain.SnapshotIssue {
	var issues []domain.SnapshotIssue

	for _, holding := range holdings {
		switch {
		case holding.PriceDate == nil:
			issues = append(issues, snapshotIssue(holding, domain.PriceIssueMissing))
		case !holding.PriceDate.Equal(date) || holding.ClosePrice == nil:
			issues = append(issues, snapshotIssue(holding, domain.PriceIssueStale))
		}
	}

	return issues
}

func snapshotIssue(holding domain.Holding, issue string) domain.SnapshotIssue {
	return domain.SnapshotIssue{
		InstrumentID: holding.InstrumentID, Symbol: holding.Symbol, Exchange: holding.Exchange,
		PriceDate: holding.PriceDate, Issue: issue,
	}
}

func insertSnapshotRows(ctx context.Context, tx *sqlx.Tx, date domain.Date, holdings []domain.Holding) error {
	for _, holding := range holdings {
		if _, err := tx.ExecContext(ctx, insertSnapshotSQL,
			date, holding.InstrumentID, holding.Quantity, *holding.ClosePrice, *holding.PriceDate); err != nil {
			return err
		}
	}

	return nil
}

// ListSnapshots returns snapshot rows, newest date first, each with its date's total.
func (s *Store) ListSnapshots(ctx context.Context, filter domain.SnapshotFilter) ([]domain.SnapshotRow, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	snapshots := []domain.SnapshotRow{}

	rows, err := s.db.QueryxContext(ctx, listSnapshotsSQL, filter.InstrumentID, filter.From, filter.To, filter.ResolvedLimit())
	if err != nil {
		slog.ErrorContext(ctx, "listing snapshots failed", "err", err)

		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var row domain.SnapshotRow

		err := rows.Scan(&row.Date, &row.InstrumentID, &row.Symbol, &row.Exchange, &row.Quantity, &row.ClosePrice,
			&row.PriceDate, &row.MarketValueUSD, &row.DateTotalUSD, &row.Allocation)
		if err != nil {
			slog.ErrorContext(ctx, "scanning snapshot failed", "err", err)

			return nil, fmt.Errorf("scanning snapshot: %w", err)
		}

		snapshots = append(snapshots, row)
	}

	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "listing snapshots failed", "err", err)

		return nil, fmt.Errorf("listing snapshots: %w", err)
	}

	return snapshots, nil
}

// ListUpdates returns updates log rows, newest first.
func (s *Store) ListUpdates(ctx context.Context, filter domain.UpdateFilter) ([]domain.Update, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	updates := []domain.Update{}

	rows, err := s.db.QueryxContext(ctx, listUpdatesSQL,
		nullIfEmpty(filter.DataType), nullIfEmpty(filter.Status), filter.ResolvedLimit())
	if err != nil {
		slog.ErrorContext(ctx, "listing updates failed", "err", err)

		return nil, fmt.Errorf("listing updates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			update          domain.Update
			completed       sql.NullTime
			message, reason sql.NullString
		)

		err := rows.Scan(&update.ID, &update.Provider, &update.DataType, &update.StartedAt, &completed, &update.Status,
			&update.RecordsUpdated, &message, &update.DateFrom, &update.DateTo, &reason)
		if err != nil {
			slog.ErrorContext(ctx, "scanning update failed", "err", err)

			return nil, fmt.Errorf("scanning update: %w", err)
		}

		update.CompletedAt, update.ErrorMessage, update.Reason = completed.Time, message.String, reason.String
		updates = append(updates, update)
	}

	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "listing updates failed", "err", err)

		return nil, fmt.Errorf("listing updates: %w", err)
	}

	return updates, nil
}
