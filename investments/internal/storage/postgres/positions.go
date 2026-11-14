package postgres

import (
	"context"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	// $1 is the optional date; null means the whole ledger.
	positionsSQL = `
SELECT i.id, i.symbol, i.exchange, SUM(t.quantity_change)::text
FROM transactions t
JOIN instruments i ON i.id = t.instrument_id
WHERE $1::date IS NULL OR t.effective_date <= $1::date
GROUP BY i.id, i.symbol, i.exchange
HAVING SUM(t.quantity_change) > 0
ORDER BY i.symbol, i.exchange`

	integritySQL = `
SELECT id, symbol, exchange, effective_date, running
FROM (
    SELECT DISTINCT ON (ledger.instrument_id)
           i.id, i.symbol, i.exchange, ledger.effective_date, ledger.running::text AS running
    FROM (
        SELECT instrument_id, id, executed_at, effective_date,
               SUM(quantity_change) OVER (
                   PARTITION BY instrument_id ORDER BY effective_date, executed_at, id
               ) AS running
        FROM transactions
        WHERE $1::date IS NULL OR effective_date <= $1::date
    ) ledger
    JOIN instruments i ON i.id = ledger.instrument_id
    WHERE ledger.running < 0
    ORDER BY ledger.instrument_id, ledger.effective_date, ledger.executed_at, ledger.id
) first_negative
ORDER BY symbol, exchange`
)

// GetPositions reconstructs positions from the ledger as of date, or the
// whole ledger when nil.
func (s *Store) GetPositions(ctx context.Context, date *domain.Date) (domain.PositionsResult, error) {
	result := domain.PositionsResult{Date: date, Positions: []domain.Position{}}

	rows, err := s.db.QueryxContext(ctx, positionsSQL, date)
	if err != nil {
		return domain.PositionsResult{}, fmt.Errorf("querying positions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var position domain.Position
		if err := rows.Scan(&position.InstrumentID, &position.Symbol, &position.Exchange, &position.Quantity); err != nil {
			return domain.PositionsResult{}, fmt.Errorf("scanning position: %w", err)
		}

		result.Positions = append(result.Positions, position)
	}

	if err := rows.Err(); err != nil {
		return domain.PositionsResult{}, fmt.Errorf("querying positions: %w", err)
	}

	result.IntegrityErrors, err = s.integrityErrors(ctx, date)
	if err != nil {
		return domain.PositionsResult{}, err
	}

	return result, nil
}

// integrityErrors lists instruments whose ledger goes negative as of date.
func (s *Store) integrityErrors(ctx context.Context, date *domain.Date) ([]domain.IntegrityError, error) {
	rows, err := s.db.QueryxContext(ctx, integritySQL, date)
	if err != nil {
		return nil, fmt.Errorf("querying ledger integrity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	integrity := []domain.IntegrityError{}

	for rows.Next() {
		var item domain.IntegrityError
		if err := rows.Scan(&item.InstrumentID, &item.Symbol, &item.Exchange, &item.FirstNegativeDate, &item.Quantity); err != nil {
			return nil, fmt.Errorf("scanning integrity error: %w", err)
		}

		integrity = append(integrity, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("querying ledger integrity: %w", err)
	}

	return integrity, nil
}
