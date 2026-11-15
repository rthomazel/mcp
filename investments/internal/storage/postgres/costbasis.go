package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// portfolioPnLSQL walks every instrument's ledger up to $1 with the average
// cost rules (see costbasis.md), then joins the result to valuationSQL. One
// row per instrument that has ledger entries: open ones carry the valuation,
// closed ones have a null quantity. The total columns repeat on every row.
const portfolioPnLSQL = `
WITH RECURSIVE ledger AS (
    SELECT instrument_id, event_type, quantity_change AS q, cash_amount AS c, fee,
           row_number() OVER (PARTITION BY instrument_id ORDER BY effective_date, executed_at, id) AS rn
    FROM transactions
    WHERE effective_date <= $1::date
), walk AS (
    SELECT instrument_id, 0::bigint AS rn, 0::numeric AS qty, 0::numeric AS cost,
           0::numeric AS realized, false AS neg
    FROM (SELECT DISTINCT instrument_id FROM ledger) first_rows
    UNION ALL
    SELECT l.instrument_id, l.rn, w.qty + l.q,
           CASE
               WHEN l.event_type = 'BUY' THEN w.cost - l.c
               WHEN l.event_type = 'SELL' OR (l.event_type = 'ADJUSTMENT' AND l.q < 0) THEN w.cost - rel.released
               WHEN l.event_type = 'ADJUSTMENT' THEN w.cost + CASE WHEN l.c < 0 THEN -l.c ELSE 0 END
               ELSE w.cost
           END,
           w.realized + CASE WHEN rel.released IS NULL THEN 0 ELSE l.c - rel.released END,
           w.neg OR w.qty + l.q < 0
    FROM walk w
    JOIN ledger l ON l.instrument_id = w.instrument_id AND l.rn = w.rn + 1
    CROSS JOIN LATERAL (
        SELECT CASE
                   WHEN NOT (l.event_type = 'SELL' OR (l.event_type = 'ADJUSTMENT' AND l.q < 0)) THEN NULL
                   WHEN w.qty + l.q = 0 THEN w.cost
                   ELSE w.cost * -l.q / NULLIF(w.qty, 0)
               END AS released
    ) rel
), final AS (
    SELECT DISTINCT ON (instrument_id) instrument_id, qty, cost, realized, neg
    FROM walk
    ORDER BY instrument_id, rn DESC
), sums AS (
    SELECT instrument_id,
           COALESCE(SUM(c) FILTER (WHERE event_type = 'DIVIDEND'), 0) AS dividends,
           COALESCE(-SUM(c) FILTER (WHERE event_type = 'TAX'), 0) AS tax,
           SUM(fee) AS fees
    FROM ledger
    GROUP BY instrument_id
), v AS (` + valuationSQL + `
), pnl AS (
    SELECT i.id, i.symbol, i.exchange, f.qty, f.cost, f.realized, s.dividends, s.tax, s.fees,
           v.quantity, v.close, v.price_date, v.age_days, v.market_value, v.allocation,
           v.market_value::numeric - f.cost AS unrealized
    FROM final f
    JOIN sums s USING (instrument_id)
    JOIN instruments i ON i.id = f.instrument_id
    LEFT JOIN v ON v.id = f.instrument_id
    WHERE NOT f.neg
)
SELECT id, symbol, exchange, quantity, close, price_date, age_days, market_value, allocation,
       trim_scale(cost)::text, trim_scale(cost / NULLIF(qty, 0))::text AS average_cost,
       trim_scale(unrealized)::text, trim_scale(unrealized / NULLIF(cost, 0) * 100)::text AS unrealized_percent,
       trim_scale(realized)::text, trim_scale(dividends)::text, trim_scale(tax)::text, trim_scale(fees)::text,
       trim_scale(unrealized + realized + dividends - tax)::text AS total_return,
       trim_scale(SUM(cost) OVER ())::text, trim_scale(SUM(unrealized) OVER ())::text, trim_scale(SUM(realized) OVER ())::text,
       trim_scale(SUM(dividends) OVER ())::text, trim_scale(SUM(tax) OVER ())::text, trim_scale(SUM(fees) OVER ())::text,
       trim_scale(SUM(unrealized) OVER () + SUM(realized) OVER () + SUM(dividends) OVER () - SUM(tax) OVER ())::text,
       trim_scale(realized + dividends - tax)::text AS closed_return,
       trim_scale(SUM(market_value::numeric) OVER ())::text AS securities_total
FROM pnl
ORDER BY symbol, exchange`

// pnlRow is one row of portfolioPnLSQL.
type pnlRow struct {
	ID                                                int
	Symbol, Exchange                                  string
	Quantity, Close                                   sql.NullString
	PriceDate                                         sql.NullTime
	AgeDays                                           sql.NullInt64
	MarketValue, Allocation                           sql.NullString
	Cost                                              domain.Decimal
	AverageCost, Unrealized, UnrealizedPercent        sql.NullString
	Realized, Dividends, Tax, Fees                    domain.Decimal
	TotalReturn                                       sql.NullString
	TotalCost                                         domain.Decimal
	TotalUnrealized                                   sql.NullString
	TotalRealized, TotalDividends, TotalTax, TotalFee domain.Decimal
	TotalReturnAll                                    sql.NullString
	ClosedReturn                                      domain.Decimal
	SecuritiesTotal                                   sql.NullString
}

// pnlResult is the parsed portfolioPnLSQL output. Securities is the sum of
// market values, nil when there are no open positions.
type pnlResult struct {
	Holdings   []domain.Holding
	Closed     []domain.ClosedPosition
	Totals     domain.PnLTotals
	Securities *domain.Decimal
}

// portfolioPnL values the open positions with their average-cost P&L, lists
// the closed positions and sums the totals. Price issues are not judged here.
func (s *Store) portfolioPnL(ctx context.Context, reference domain.Date) (pnlResult, error) {
	rows, err := s.db.QueryxContext(ctx, portfolioPnLSQL, reference)
	if err != nil {
		return pnlResult{}, fmt.Errorf("computing portfolio P&L: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := pnlResult{
		Holdings: []domain.Holding{}, Closed: []domain.ClosedPosition{},
		Totals: domain.PnLTotals{CostUSD: "0", RealizedPnLUSD: "0", DividendsUSD: "0", TaxUSD: "0", FeesUSD: "0"},
	}

	for rows.Next() {
		var row pnlRow

		err := rows.Scan(&row.ID, &row.Symbol, &row.Exchange, &row.Quantity, &row.Close, &row.PriceDate, &row.AgeDays,
			&row.MarketValue, &row.Allocation, &row.Cost, &row.AverageCost, &row.Unrealized, &row.UnrealizedPercent,
			&row.Realized, &row.Dividends, &row.Tax, &row.Fees, &row.TotalReturn, &row.TotalCost, &row.TotalUnrealized,
			&row.TotalRealized, &row.TotalDividends, &row.TotalTax, &row.TotalFee, &row.TotalReturnAll, &row.ClosedReturn, &row.SecuritiesTotal)
		if err != nil {
			return pnlResult{}, fmt.Errorf("scanning P&L row: %w", err)
		}

		result.Securities = decimalPtr(row.SecuritiesTotal)
		result.Totals = domain.PnLTotals{
			CostUSD: row.TotalCost, RealizedPnLUSD: row.TotalRealized, DividendsUSD: row.TotalDividends,
			TaxUSD: row.TotalTax, FeesUSD: row.TotalFee,
			UnrealizedPnLUSD: decimalPtr(row.TotalUnrealized), TotalReturnUSD: decimalPtr(row.TotalReturnAll),
		}

		if row.Quantity.Valid {
			result.Holdings = append(result.Holdings, s.holdingFromPnL(row))

			continue
		}

		result.Closed = append(result.Closed, domain.ClosedPosition{
			InstrumentID: row.ID, Symbol: row.Symbol, Exchange: row.Exchange, RealizedPnLUSD: row.Realized,
			DividendsUSD: row.Dividends, TaxUSD: row.Tax, FeesUSD: row.Fees, TotalReturnUSD: row.ClosedReturn,
		})
	}

	if err := rows.Err(); err != nil {
		return pnlResult{}, fmt.Errorf("computing portfolio P&L: %w", err)
	}

	return result, nil
}

func (s *Store) holdingFromPnL(row pnlRow) domain.Holding {
	holding := domain.Holding{
		InstrumentID: row.ID, Symbol: row.Symbol, Exchange: row.Exchange, Quantity: domain.Decimal(row.Quantity.String),
		CostUSD: row.Cost, AverageCostUSD: domain.Decimal(row.AverageCost.String),
		RealizedPnLUSD: row.Realized, DividendsUSD: row.Dividends, TaxUSD: row.Tax, FeesUSD: row.Fees,
		Allocation: decimalPtr(row.Allocation),
	}

	holding.PriceIssue, holding.PriceDate = s.priceIssue(row.PriceDate, row.AgeDays)
	if holding.PriceIssue != "" {
		return holding
	}

	holding.ClosePrice, holding.MarketValueUSD = decimalPtr(row.Close), decimalPtr(row.MarketValue)
	holding.UnrealizedPnLUSD = decimalPtr(row.Unrealized)
	holding.UnrealizedPnLPercent = decimalPtr(row.UnrealizedPercent)
	holding.TotalReturnUSD = decimalPtr(row.TotalReturn)

	return holding
}

func decimalPtr(value sql.NullString) *domain.Decimal {
	if !value.Valid {
		return nil
	}

	decimal := domain.Decimal(value.String)

	return &decimal
}
