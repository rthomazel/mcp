package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	lockInstrumentSQL = `SELECT id FROM instruments WHERE id = $1 FOR UPDATE`

	// Empty cash is stored as zero.
	insertTransactionSQL = `
INSERT INTO transactions (instrument_id, event_type, quantity_change, cash_amount, executed_at, effective_date, notes)
VALUES ($1, $2, $3::numeric, COALESCE(NULLIF($4, '')::numeric, 0), $5, $6, $7)
RETURNING id, instrument_id, event_type, quantity_change::text, cash_amount::text, executed_at, effective_date, notes`

	firstNegativeSQL = `
SELECT effective_date, running::text
FROM (
    SELECT id, executed_at, effective_date,
           SUM(quantity_change) OVER (ORDER BY effective_date, executed_at, id) AS running
    FROM transactions
    WHERE instrument_id = $1
) ledger
WHERE running < 0
ORDER BY effective_date, executed_at, id
LIMIT 1`

	latestSnapshotDateSQL = `SELECT max(date) FROM snapshots`

	listTransactionsSQL = `
SELECT id, instrument_id, event_type, quantity_change::text, cash_amount::text, executed_at, effective_date, notes
FROM transactions
WHERE ($1::int IS NULL OR instrument_id = $1)
  AND ($2::date IS NULL OR effective_date >= $2)
  AND ($3::date IS NULL OR effective_date <= $3)
  AND ($5::text IS NULL OR event_type = $5)
ORDER BY effective_date DESC, executed_at DESC, id DESC
LIMIT $4`
)

// RecordTransaction validates draft, locks the instrument, inserts the entry
// and verifies the whole instrument ledger stays long-only.
func (s *Store) RecordTransaction(ctx context.Context, draft domain.DraftTransaction) (domain.RecordResult, error) {
	if err := draft.Validate(); err != nil {
		return domain.RecordResult{}, err
	}

	var result domain.RecordResult

	err := s.withTx(ctx, func(tx *sqlx.Tx) error {
		var lockedID int
		if err := tx.QueryRowxContext(ctx, lockInstrumentSQL, draft.InstrumentID).Scan(&lockedID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: id %d", domain.ErrInstrumentNotFound, draft.InstrumentID)
			}

			return fmt.Errorf("locking instrument: %w", err)
		}

		inserted, err := insertTransaction(ctx, tx, draft)
		if err != nil {
			return err
		}

		if err := checkLongOnly(ctx, tx, draft.InstrumentID); err != nil {
			return err
		}

		var latestSnapshot sql.NullTime
		if err := tx.QueryRowxContext(ctx, latestSnapshotDateSQL).Scan(&latestSnapshot); err != nil {
			return fmt.Errorf("reading latest snapshot date: %w", err)
		}

		result.Transaction = inserted

		changesQuantity := inserted.QuantityChange.Sign() != 0
		if changesQuantity && latestSnapshot.Valid && !inserted.EffectiveDate.After(latestSnapshot.Time) {
			rebuildFrom := inserted.EffectiveDate
			result.RebuildFrom = &rebuildFrom
		}

		return nil
	})
	if err != nil {
		return domain.RecordResult{}, err
	}

	slog.InfoContext(ctx, "transaction recorded",
		"id", result.Transaction.ID, "instrument_id", result.Transaction.InstrumentID, "event_type", result.Transaction.EventType)

	return result, nil
}

func insertTransaction(ctx context.Context, tx *sqlx.Tx, draft domain.DraftTransaction) (domain.Transaction, error) {
	var inserted domain.Transaction

	err := tx.QueryRowxContext(ctx, insertTransactionSQL,
		draft.InstrumentID, string(draft.EventType), string(draft.QuantityChange), string(draft.CashAmount),
		draft.ExecutedAt, draft.EffectiveDate, draft.Notes,
	).Scan(&inserted.ID, &inserted.InstrumentID, &inserted.EventType, &inserted.QuantityChange,
		&inserted.CashAmount, &inserted.ExecutedAt, &inserted.EffectiveDate, &inserted.Notes)
	if err != nil {
		if isCheckViolation(err) {
			return domain.Transaction{}, fmt.Errorf("%w: rejected by database check: %w", domain.ErrInvalidInput, err)
		}

		return domain.Transaction{}, fmt.Errorf("inserting transaction: %w", err)
	}

	return inserted, nil
}

// checkLongOnly returns a NegativeBalanceError when any prefix of the
// instrument's ledger is negative.
func checkLongOnly(ctx context.Context, tx *sqlx.Tx, instrumentID int) error {
	var (
		onDate   domain.Date
		quantity domain.Decimal
	)

	err := tx.QueryRowxContext(ctx, firstNegativeSQL, instrumentID).Scan(&onDate, &quantity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("checking ledger balance: %w", err)
	}

	return &domain.NegativeBalanceError{InstrumentID: instrumentID, OnDate: onDate, Quantity: quantity}
}

// ListTransactions returns ledger entries matching filter, newest first.
func (s *Store) ListTransactions(ctx context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	filter, err := filter.Normalize()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryxContext(ctx, listTransactionsSQL, filter.InstrumentID, filter.From, filter.To, filter.Limit,
		nullIfEmpty(string(filter.EventType)))
	if err != nil {
		return nil, fmt.Errorf("listing transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	transactions := []domain.Transaction{}

	for rows.Next() {
		var entry domain.Transaction
		if err := rows.Scan(&entry.ID, &entry.InstrumentID, &entry.EventType, &entry.QuantityChange,
			&entry.CashAmount, &entry.ExecutedAt, &entry.EffectiveDate, &entry.Notes); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}

		transactions = append(transactions, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing transactions: %w", err)
	}

	return transactions, nil
}
