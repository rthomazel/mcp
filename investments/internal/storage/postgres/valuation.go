package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// valuationSQL values active positions as of $1 with the latest price on or
// before it. The total and allocation arithmetic stays in SQL.
const valuationSQL = `
WITH active AS (
    SELECT i.id, i.symbol, i.exchange, SUM(t.quantity_change) AS quantity
    FROM transactions t
    JOIN instruments i ON i.id = t.instrument_id
    WHERE t.effective_date <= $1::date
    GROUP BY i.id, i.symbol, i.exchange
    HAVING SUM(t.quantity_change) > 0
), valued AS (
    SELECT a.id, a.symbol, a.exchange, a.quantity,
           p.close, p.date AS price_date,
           a.quantity * p.close AS market_value
    FROM active a
    LEFT JOIN LATERAL (
        SELECT close, date FROM prices
        WHERE instrument_id = a.id AND date <= $1::date
        ORDER BY date DESC
        LIMIT 1
    ) p ON true
)
SELECT id, symbol, exchange, quantity::text, close::text, price_date,
       ($1::date - price_date) AS age_days,
       market_value::text,
       SUM(market_value) OVER ()::text AS total,
       (market_value / SUM(market_value) OVER ())::text AS allocation
FROM valued
ORDER BY symbol, exchange`

// GetPortfolio values active positions at real closes as of date, or today
// in the market timezone when nil.
func (s *Store) GetPortfolio(ctx context.Context, date *domain.Date) (domain.Portfolio, error) {
	reference := s.referenceDate(date)

	holdings, total, err := s.valuedHoldings(ctx, s.db, reference)
	if err != nil {
		return domain.Portfolio{}, err
	}

	integrity, err := s.integrityErrors(ctx, &reference)
	if err != nil {
		return domain.Portfolio{}, err
	}

	portfolio := domain.Portfolio{Date: reference, Holdings: holdings, IntegrityErrors: integrity}

	complete := true

	for _, holding := range holdings {
		if holding.PriceIssue != "" {
			complete = false
		}
	}

	if !complete {
		for i := range holdings {
			holdings[i].Allocation = nil
		}

		return portfolio, nil
	}

	portfolio.Complete = true

	if len(holdings) == 0 {
		zero := domain.Decimal("0")
		portfolio.SecuritiesValueUSD = &zero

		return portfolio, nil
	}

	portfolio.SecuritiesValueUSD = &total

	return portfolio, nil
}

func (s *Store) referenceDate(date *domain.Date) domain.Date {
	if date != nil {
		return *date
	}

	year, month, day := time.Now().In(s.marketLocation).Date()

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// valuedHoldings runs valuationSQL on q, the store or a transaction, and marks missing and stale prices. A
// holding with an issue keeps its price date but no price or value. The
// allocation is set where SQL computed one; GetPortfolio withholds it when
// the portfolio is incomplete.
func (s *Store) valuedHoldings(ctx context.Context, q sqlx.QueryerContext, reference domain.Date) ([]domain.Holding, domain.Decimal, error) {
	rows, err := q.QueryxContext(ctx, valuationSQL, reference)
	if err != nil {
		return nil, "", fmt.Errorf("valuing portfolio: %w", err)
	}
	defer func() { _ = rows.Close() }()

	holdings := []domain.Holding{}

	var total domain.Decimal

	for rows.Next() {
		var (
			holding                             domain.Holding
			closePrice, marketValue, totalValue sql.NullString
			allocation                          sql.NullString
			priceDate                           sql.NullTime
			ageDays                             sql.NullInt64
		)

		if err := rows.Scan(&holding.InstrumentID, &holding.Symbol, &holding.Exchange, &holding.Quantity,
			&closePrice, &priceDate, &ageDays, &marketValue, &totalValue, &allocation); err != nil {
			return nil, "", fmt.Errorf("scanning holding: %w", err)
		}

		switch {
		case !priceDate.Valid:
			holding.PriceIssue = domain.PriceIssueMissing
		case int(ageDays.Int64) >= s.stalePriceDays:
			holding.PriceIssue = domain.PriceIssueStale
		}

		if priceDate.Valid {
			date := priceDate.Time
			holding.PriceDate = &date
		}

		if holding.PriceIssue == "" {
			price, value := domain.Decimal(closePrice.String), domain.Decimal(marketValue.String)
			holding.ClosePrice, holding.MarketValueUSD = &price, &value
		}

		if allocation.Valid {
			share := domain.Decimal(allocation.String)
			holding.Allocation = &share
		}

		total = domain.Decimal(totalValue.String)

		holdings = append(holdings, holding)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("valuing portfolio: %w", err)
	}

	return holdings, total, nil
}
