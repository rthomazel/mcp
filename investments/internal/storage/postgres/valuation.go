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

// accountEquitySQL adds the securities value and the cash balance in SQL.
const accountEquitySQL = `SELECT ($1::numeric + $2::numeric)::text`

// GetPortfolio values active positions at real closes as of date, or today
// in the market timezone when nil, with average-cost P&L and cash.
func (s *Store) GetPortfolio(ctx context.Context, date *domain.Date) (domain.Portfolio, error) {
	reference := s.referenceDate(date)

	pnl, err := s.portfolioPnL(ctx, reference)
	if err != nil {
		return domain.Portfolio{}, err
	}

	integrity, err := s.integrityErrors(ctx, &reference)
	if err != nil {
		return domain.Portfolio{}, err
	}

	cash, err := cashBalance(ctx, s.db, reference)
	if err != nil {
		return domain.Portfolio{}, err
	}

	portfolio := domain.Portfolio{
		Date: reference, Holdings: pnl.Holdings, IntegrityErrors: integrity,
		Cash: cash, ClosedPositions: pnl.Closed, PnL: pnl.Totals,
	}

	for _, holding := range pnl.Holdings {
		if holding.PriceIssue != "" {
			s.withholdPrices(&portfolio)

			return portfolio, nil
		}
	}

	portfolio.Complete = true

	value := pnl.Securities
	if value == nil {
		zero := domain.Decimal("0")
		value = &zero
	}

	portfolio.SecuritiesValueUSD = value

	if cash.Initialized {
		portfolio.AccountEquityUSD, err = s.accountEquity(ctx, *value, *cash.BalanceUSD)
	}

	return portfolio, err
}

// withholdPrices removes the figures that would mislead over a partial set of prices.
func (s *Store) withholdPrices(portfolio *domain.Portfolio) {
	for i := range portfolio.Holdings {
		portfolio.Holdings[i].Allocation = nil
	}

	portfolio.PnL.UnrealizedPnLUSD, portfolio.PnL.TotalReturnUSD = nil, nil
}

func (s *Store) accountEquity(ctx context.Context, securities, cash domain.Decimal) (*domain.Decimal, error) {
	var equity domain.Decimal
	if err := s.db.GetContext(ctx, &equity, accountEquitySQL, securities, cash); err != nil {
		return nil, fmt.Errorf("computing account equity: %w", err)
	}

	return &equity, nil
}

func (s *Store) referenceDate(date *domain.Date) domain.Date {
	if date != nil {
		return *date
	}

	year, month, day := s.clock().In(s.marketLocation).Date()

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// priceIssue classifies a holding's latest price: missing when there is none,
// stale when it is stalePriceDays or more older than the reference date. The
// price date is returned whenever a price row exists.
func (s *Store) priceIssue(priceDate sql.NullTime, ageDays sql.NullInt64) (string, *domain.Date) {
	if !priceDate.Valid {
		return domain.PriceIssueMissing, nil
	}

	date := priceDate.Time

	if int(ageDays.Int64) >= s.stalePriceDays {
		return domain.PriceIssueStale, &date
	}

	return "", &date
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

		holding.PriceIssue, holding.PriceDate = s.priceIssue(priceDate, ageDays)

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
