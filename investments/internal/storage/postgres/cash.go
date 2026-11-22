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
	insertCashTransactionSQL = `
INSERT INTO cash_transactions (kind, amount, executed_at, effective_date, notes)
VALUES ($1, $2::numeric, $3, $4, $5)
RETURNING id, kind, amount::text, executed_at, effective_date, notes`

	openingDateSQL = `SELECT effective_date FROM cash_transactions WHERE kind = 'OPENING_BALANCE'`

	// cashLedgerSQL is the cash ledger: cash transactions and the cash amount of
	// every instrument transaction, ordered by effective date, execution time,
	// source rank and id, from the opening balance date through $1.
	cashLedgerSQL = `
ledger AS (
    SELECT effective_date, executed_at, 0 AS source, id, amount FROM cash_transactions
    UNION ALL
    SELECT effective_date, executed_at, 1 AS source, id, cash_amount AS amount FROM transactions
    WHERE cash_amount <> 0
), bounded AS (
    SELECT l.* FROM ledger l
    WHERE l.effective_date <= $1::date
      AND l.effective_date >= (SELECT effective_date FROM cash_transactions WHERE kind = 'OPENING_BALANCE')
)`

	// Null balance when no opening balance exists on or before $1.
	cashBalanceSQL = `WITH ` + cashLedgerSQL + `
SELECT SUM(amount)::text, GREATEST(SUM(amount), 0)::text FROM bounded`

	cashIntegritySQL = `WITH ` + cashLedgerSQL + `, running AS (
    SELECT effective_date, executed_at, source, id,
           SUM(amount) OVER (ORDER BY effective_date, executed_at, source, id) AS balance
    FROM bounded
)
SELECT effective_date, balance::text FROM running
WHERE balance < 0
ORDER BY effective_date, executed_at, source, id
LIMIT 1`

	insertCashSnapshotSQL = `INSERT INTO cash_snapshots (date, balance_usd) VALUES ($1::date, $2::numeric)`

	listCashTransactionsSQL = `
SELECT id, kind, amount::text, executed_at, effective_date, notes
FROM cash_transactions
WHERE ($1::text IS NULL OR kind = $1)
  AND ($2::date IS NULL OR effective_date >= $2)
  AND ($3::date IS NULL OR effective_date <= $3)
ORDER BY effective_date DESC, executed_at DESC, id DESC
LIMIT $4`
)

// Record validates draft and inserts it. A negative resulting
// balance is not rejected; RebuildFrom tells the caller which cash snapshots are stale.
func (s *Cash) Record(ctx context.Context, draft domain.CashTransaction) (domain.CashRecordResult, error) {
	if err := draft.Validate(); err != nil {
		return domain.CashRecordResult{}, err
	}

	var result domain.CashRecordResult

	err := s.withTx(ctx, func(tx *sqlx.Tx) error {
		inserted, err := insertCashTransaction(ctx, tx, draft)
		if err != nil {
			return err
		}

		result.CashTransaction = inserted

		result.RebuildFrom, err = staleCashFrom(ctx, tx, inserted.EffectiveDate, inserted.Kind == domain.CashOpeningBalance)

		return err
	})
	if err != nil {
		return domain.CashRecordResult{}, err
	}

	slog.InfoContext(ctx, "cash transaction recorded", "id", result.CashTransaction.ID, "kind", result.CashTransaction.Kind)

	return result, nil
}

func insertCashTransaction(ctx context.Context, tx *sqlx.Tx, draft domain.CashTransaction) (domain.CashTransaction, error) {
	var inserted domain.CashTransaction

	err := tx.QueryRowxContext(ctx, insertCashTransactionSQL,
		string(draft.Kind), string(draft.Amount), draft.ExecutedAt, draft.EffectiveDate, draft.Notes,
	).Scan(&inserted.ID, &inserted.Kind, &inserted.Amount, &inserted.ExecutedAt, &inserted.EffectiveDate, &inserted.Notes)

	switch {
	case err == nil:
		return inserted, nil
	case isUniqueViolation(err):
		return domain.CashTransaction{}, fmt.Errorf("%w: an opening balance is already recorded", domain.ErrInvalidInput)
	case isCheckViolation(err):
		return domain.CashTransaction{}, fmt.Errorf("%w: rejected by database check: %w", domain.ErrInvalidInput, err)
	}

	return domain.CashTransaction{}, fmt.Errorf("inserting cash transaction: %w", err)
}

// staleCashFrom returns the date from which stored cash snapshots are stale
// after an entry effective on effective, nil when none are.
func staleCashFrom(ctx context.Context, tx *sqlx.Tx, effective domain.Date, isOpening bool) (*domain.Date, error) {
	var latest sql.NullTime
	if err := tx.QueryRowxContext(ctx, latestSnapshotDateSQL).Scan(&latest); err != nil {
		return nil, fmt.Errorf("reading latest snapshot date: %w", err)
	}

	if !latest.Valid || effective.After(latest.Time) {
		return nil, nil //nolint:nilnil // nil means nothing is stale
	}

	if !isOpening {
		var opening sql.NullTime

		err := tx.QueryRowxContext(ctx, openingDateSQL).Scan(&opening)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && effective.Before(opening.Time)) {
			return nil, nil //nolint:nilnil // entries before the opening balance never count
		}

		if err != nil {
			return nil, fmt.Errorf("reading opening date: %w", err)
		}
	}

	return &effective, nil
}

// List returns cash transactions matching filter, newest first.
func (s *Cash) List(ctx context.Context, filter domain.CashTransactionFilter) ([]domain.CashTransaction, error) {
	filter, err := filter.Normalize()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryxContext(ctx, listCashTransactionsSQL, nullIfEmpty(string(filter.Kind)), filter.From, filter.To, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("listing cash transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := []domain.CashTransaction{}

	for rows.Next() {
		var entry domain.CashTransaction
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.Amount, &entry.ExecutedAt, &entry.EffectiveDate, &entry.Notes); err != nil {
			return nil, fmt.Errorf("scanning cash transaction: %w", err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing cash transactions: %w", err)
	}

	return entries, nil
}

// Get returns the cash balance as of date, or today in the market timezone when nil.
func (s *Cash) Get(ctx context.Context, date *domain.Date) (domain.CashBalance, error) {
	return cashBalance(ctx, s.db, s.referenceDate(date))
}

// cashBalance computes the balance, buying power and first negative balance as of date.
func cashBalance(ctx context.Context, q sqlx.QueryerContext, date domain.Date) (domain.CashBalance, error) {
	balance := domain.CashBalance{Date: date}

	var opening sql.NullTime

	err := q.QueryRowxContext(ctx, openingDateSQL).Scan(&opening)
	if errors.Is(err, sql.ErrNoRows) {
		return balance, nil
	}

	if err != nil {
		return balance, fmt.Errorf("reading opening date: %w", err)
	}

	openingDate := opening.Time
	balance.OpeningDate = &openingDate

	if openingDate.After(date) {
		return balance, nil
	}

	var total, buyingPower domain.Decimal
	if err := q.QueryRowxContext(ctx, cashBalanceSQL, date).Scan(&total, &buyingPower); err != nil {
		return balance, fmt.Errorf("summing cash: %w", err)
	}

	balance.Initialized, balance.BalanceUSD, balance.BuyingPowerUSD = true, &total, &buyingPower

	var negative domain.CashIntegrityError

	err = q.QueryRowxContext(ctx, cashIntegritySQL, date).Scan(&negative.FirstNegativeDate, &negative.Balance)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return balance, fmt.Errorf("checking cash integrity: %w", err)
	}

	if err == nil {
		balance.IntegrityErrors = []domain.CashIntegrityError{negative}
	}

	return balance, nil
}
