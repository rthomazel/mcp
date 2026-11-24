package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

// priceVolatilitySQL takes $1 the instrument id, $2 and $3 the window as months and days, $4 and $5 the look-back
// the same way, and $6 the threshold. A move is the adjusted close change from the last price date on or before
// the date counted back by the window. The judged move is the latest price date's; the baseline is the moves
// strictly after the look-back start and before the latest date. The threshold is compared as the move against
// the threshold times the volatility, so no division rounds it.
const priceVolatilitySQL = `WITH latest AS (
    SELECT MAX(date) AS date FROM prices
), px AS (
    SELECT date, adjusted_close AS adj FROM prices WHERE instrument_id = $1
), moves AS (
    SELECT t.date, b.date AS start_date, (t.adj - b.adj) / NULLIF(b.adj, 0) * 100 AS move
    FROM px t
    JOIN LATERAL (
        SELECT date, adj FROM px
        WHERE date <= (t.date - make_interval(months => $2::int, days => $3::int))::date
        ORDER BY date DESC LIMIT 1
    ) b ON true
), judged AS (
    SELECT m.* FROM moves m JOIN latest ON m.date = latest.date
), baseline AS (
    SELECT COUNT(m.move) AS moves, STDDEV_SAMP(m.move) AS volatility
    FROM moves m, latest
    WHERE m.date < latest.date AND m.date > (latest.date - make_interval(months => $4::int, days => $5::int))::date
)
SELECT judged.date, judged.start_date, trim_scale(judged.move)::text, baseline.moves,
       trim_scale(NULLIF(baseline.volatility, 0))::text,
       trim_scale(judged.move / NULLIF(baseline.volatility, 0))::text,
       COALESCE(baseline.volatility > 0 AND judged.move >= $6::numeric * baseline.volatility, false),
       COALESCE(baseline.volatility > 0 AND judged.move <= -$6::numeric * baseline.volatility, false)
FROM judged, baseline`

// GetPriceVolatility returns the ticker's move over window on the latest price date against its volatility.
func (s *Monitors) GetPriceVolatility(ctx context.Context, ticker string, window, lookback duration.Duration,
	threshold domain.Decimal,
) (*domain.PriceVolatility, error) {
	if threshold.Sign() <= 0 {
		return nil, fmt.Errorf("%w: the threshold %q must be positive", domain.ErrInvalidInput, threshold)
	}

	ids, err := s.resolveTickers(ctx, []string{ticker})
	if err != nil {
		return nil, err
	}

	id, ok := ids[ticker]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrInstrumentNotFound, ticker)
	}

	var (
		result     domain.PriceVolatility
		volatility sql.NullString
		variation  sql.NullString
	)

	err = s.db.QueryRowxContext(ctx, priceVolatilitySQL, id, window.Months, window.Days, lookback.Months, lookback.Days,
		string(threshold)).Scan(&result.Date, &result.StartDate, &result.MovePercent, &result.Moves,
		&volatility, &variation, &result.AtLeastUp, &result.AtLeastDown)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no price on the latest date, or none before the window start
	}

	if err != nil {
		return nil, fmt.Errorf("reading price volatility of %s: %w", ticker, err)
	}

	result.VolatilityPercent, result.Variation = decimalPtr(volatility), decimalPtr(variation)

	return &result, nil
}
