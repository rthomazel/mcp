package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// maxDrawdownLevels is the most levels one drawdown state reports.
const maxDrawdownLevels = 100

const (
	historyBoundsSQL = `
SELECT MIN(date), MAX(date) FROM (SELECT date FROM snapshots UNION SELECT date FROM cash_snapshots) dates`

	// dailySeriesSQL is the daily series every history query reads. A cash-only
	// date has securities value zero; equity needs a cash snapshot.
	dailySeriesSQL = `
daily AS (
    SELECT d.date, COALESCE(s.total, 0) AS securities, c.balance_usd AS cash,
           CASE WHEN c.balance_usd IS NULL THEN NULL ELSE COALESCE(s.total, 0) + c.balance_usd END AS equity
    FROM (SELECT date FROM snapshots UNION SELECT date FROM cash_snapshots) d
    LEFT JOIN (SELECT date, SUM(market_value_usd) AS total FROM snapshots GROUP BY date) s ON s.date = d.date
    LEFT JOIN cash_snapshots c ON c.date = d.date
)`

	// The window number is the whole days from the range start over the window; the newest date of each window is kept.
	historyPointsSQL = `WITH ` + dailySeriesSQL + `, windowed AS (
    SELECT *, row_number() OVER (PARTITION BY (date - $1::date) / $3::int ORDER BY date DESC) AS rank
    FROM daily
    WHERE date BETWEEN $1::date AND $2::date
)
SELECT date, securities::text, cash::text, equity::text
FROM windowed
WHERE rank = 1
ORDER BY date
LIMIT $4`

	// seriesSummarySQL takes %[1]s, the value column. $1 is the range start, $2 the range end.
	// The largest fall is the deepest percent below the running peak, earliest trough first.
	seriesSummarySQL = `WITH ` + dailySeriesSQL + `, series AS (
    SELECT date, %[1]s AS value FROM daily WHERE %[1]s IS NOT NULL
), in_range AS (
    SELECT date, value FROM series WHERE date BETWEEN $1::date AND $2::date
), latest AS (
    SELECT date, value FROM in_range ORDER BY date DESC LIMIT 1
), high AS (
    SELECT date, value FROM series WHERE date <= $2::date ORDER BY value DESC, date LIMIT 1
), baseline AS (
    SELECT date, value FROM (
        (SELECT date, value, 0 AS pri FROM series WHERE date <= $1::date ORDER BY date DESC LIMIT 1)
        UNION ALL
        (SELECT date, value, 1 AS pri FROM in_range ORDER BY date LIMIT 1)
    ) candidates ORDER BY pri LIMIT 1
), range_high AS (
    SELECT date, value FROM in_range ORDER BY value DESC, date LIMIT 1
), range_low AS (
    SELECT date, value FROM in_range ORDER BY value, date LIMIT 1
), running AS (
    SELECT date, value, MAX(value) OVER (ORDER BY date) AS peak_value FROM in_range
), falls AS (
    SELECT r.date AS trough_date, r.value AS trough_value, r.peak_value,
           (SELECT MIN(i.date) FROM in_range i WHERE i.value = r.peak_value AND i.date <= r.date) AS peak_date,
           r.value - r.peak_value AS fall,
           COALESCE((r.value - r.peak_value) / NULLIF(r.peak_value, 0) * 100, 0) AS fall_percent
    FROM running r
), worst AS (
    SELECT * FROM falls ORDER BY fall_percent, trough_date LIMIT 1
)
SELECT latest.date, trim_scale(latest.value)::text,
       high.date, trim_scale(high.value)::text,
       trim_scale(latest.value - high.value)::text,
       trim_scale(COALESCE((latest.value - high.value) / NULLIF(high.value, 0) * 100, 0))::text,
       baseline.date, trim_scale(baseline.value)::text,
       trim_scale(latest.value - baseline.value)::text,
       trim_scale((latest.value - baseline.value) / NULLIF(baseline.value, 0) * 100)::text,
       range_high.date, trim_scale(range_high.value)::text,
       range_low.date, trim_scale(range_low.value)::text,
       worst.peak_date, trim_scale(worst.peak_value)::text, worst.trough_date, trim_scale(worst.trough_value)::text,
       trim_scale(worst.fall)::text, trim_scale(worst.fall_percent)::text,
       worst.fall = 0 OR EXISTS (SELECT 1 FROM in_range i WHERE i.date > worst.trough_date AND i.value >= worst.peak_value)
FROM latest, high, baseline, range_high, range_low, worst`

	// drawdownStateSQL takes $1, the increment, and $2, the most levels. A row is shallower than a
	// level when its drawdown is above it, compared by cross multiplication so no division rounds it.
	// The crossing is the first date after the latest shallower one; a zero peak is shallower than every level.
	drawdownStateSQL = `WITH ` + dailySeriesSQL + `, running AS (
    SELECT date, securities AS value, MAX(securities) OVER (ORDER BY date) AS peak FROM daily
), latest AS (
    SELECT * FROM running ORDER BY date DESC LIMIT 1
), high AS (
    SELECT MIN(r.date) AS date FROM running r, latest WHERE r.value = latest.peak
), depth AS (
    SELECT latest.*,
           CASE WHEN latest.peak > 0 THEN floor((latest.peak - latest.value) * 100 / (latest.peak * $1::numeric)) ELSE 0 END AS levels
    FROM latest
)
SELECT depth.date, trim_scale(depth.value)::text,
       trim_scale(COALESCE((depth.value - depth.peak) / NULLIF(depth.peak, 0) * 100, 0))::text,
       high.date, trim_scale(depth.peak)::text,
       lv.level, lv.crossed_on
FROM depth CROSS JOIN high
LEFT JOIN LATERAL (
    SELECT k, trim_scale(-k * $1::numeric)::text AS level,
           (SELECT MIN(r.date) FROM running r WHERE r.date > (
               SELECT MAX(s.date) FROM running s
               WHERE s.peak = 0 OR (s.value - s.peak) * 100 > -k * $1::numeric * s.peak)) AS crossed_on
    FROM generate_series(1, LEAST(depth.levels, $2::numeric)::int) AS k
) lv ON true
ORDER BY lv.k`

	netContributionsSQL = `
SELECT COALESCE(SUM(amount), 0)::text FROM cash_transactions
WHERE kind IN ('DEPOSIT', 'WITHDRAWAL') AND effective_date > $1::date AND effective_date <= $2::date`
)

// GetValueHistory returns the securities value and account equity series over
// a range with summary measures. Changes in value, not returns.
func (s *Store) GetValueHistory(ctx context.Context, filter domain.HistoryFilter) (domain.ValueHistory, error) {
	filter, err := filter.Normalize()
	if err != nil {
		return domain.ValueHistory{}, err
	}

	history := domain.ValueHistory{WindowDays: filter.WindowDays, Points: []domain.ValuePoint{}}

	var earliest, latest sql.NullTime
	if err := s.db.QueryRowxContext(ctx, historyBoundsSQL).Scan(&earliest, &latest); err != nil {
		return domain.ValueHistory{}, fmt.Errorf("reading history bounds: %w", err)
	}

	if !earliest.Valid {
		return history, nil
	}

	history.From, history.To = earliest.Time, latest.Time
	if filter.From != nil {
		history.From = *filter.From
	}

	if filter.To != nil {
		history.To = *filter.To
	}

	if history.Points, err = s.historyPoints(ctx, history); err != nil {
		return domain.ValueHistory{}, err
	}

	if len(history.Points) == 0 {
		return history, nil
	}

	if history.Securities, err = s.seriesSummary(ctx, "securities", history.From, history.To); err != nil {
		return domain.ValueHistory{}, err
	}

	if history.Equity, err = s.seriesSummary(ctx, "equity", history.From, history.To); err != nil {
		return domain.ValueHistory{}, err
	}

	if history.Equity != nil {
		var contributions domain.Decimal

		err := s.db.GetContext(ctx, &contributions, netContributionsSQL, history.Equity.Baseline.Date, history.Equity.Latest.Date)
		if err != nil {
			return domain.ValueHistory{}, fmt.Errorf("summing net contributions: %w", err)
		}

		history.NetContributionsUSD = &contributions
	}

	return history, nil
}

func (s *Store) historyPoints(ctx context.Context, history domain.ValueHistory) ([]domain.ValuePoint, error) {
	rows, err := s.db.QueryxContext(ctx, historyPointsSQL, history.From, history.To, history.WindowDays, s.maxHistoryPoints+1)
	if err != nil {
		return nil, fmt.Errorf("reading value history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	points := []domain.ValuePoint{}

	for rows.Next() {
		var (
			point        domain.ValuePoint
			cash, equity sql.NullString
		)

		if err := rows.Scan(&point.Date, &point.SecuritiesValueUSD, &cash, &equity); err != nil {
			return nil, fmt.Errorf("scanning value point: %w", err)
		}

		point.CashUSD, point.AccountEquityUSD = decimalPtr(cash), decimalPtr(equity)
		points = append(points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading value history: %w", err)
	}

	if len(points) > s.maxHistoryPoints {
		return nil, fmt.Errorf("%w: the series has more than %d points, use a larger window_days or a narrower range",
			domain.ErrInvalidInput, s.maxHistoryPoints)
	}

	return points, nil
}

// seriesSummary computes the measures over a daily series column, nil when the range has no value.
func (s *Store) seriesSummary(ctx context.Context, column string, from, to domain.Date) (*domain.SeriesSummary, error) {
	var (
		summary       domain.SeriesSummary
		changePercent sql.NullString
		latestDate    sql.NullTime
	)

	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(seriesSummarySQL, column), from, to).Scan(
		&latestDate, &summary.Latest.ValueUSD,
		&summary.AllTimeHigh.Date, &summary.AllTimeHigh.ValueUSD, &summary.FromHighUSD, &summary.FromHighPercent,
		&summary.Baseline.Date, &summary.Baseline.ValueUSD, &summary.ChangeUSD, &changePercent,
		&summary.RangeHigh.Date, &summary.RangeHigh.ValueUSD, &summary.RangeLow.Date, &summary.RangeLow.ValueUSD,
		&summary.MaxDrawdown.PeakDate, &summary.MaxDrawdown.PeakValueUSD,
		&summary.MaxDrawdown.TroughDate, &summary.MaxDrawdown.TroughValueUSD,
		&summary.MaxDrawdown.DrawdownUSD, &summary.MaxDrawdown.DrawdownPercent, &summary.MaxDrawdown.Recovered)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no value in the range
	}

	if err != nil {
		return nil, fmt.Errorf("summarizing %s series: %w", column, err)
	}

	summary.Latest.Date, summary.ChangePercent = latestDate.Time, decimalPtr(changePercent)

	return &summary, nil
}

// GetDrawdownState returns the drawdown of the latest securities value from
// the all-time high and the levels it is at or below. Changes in value, not returns.
func (s *Store) GetDrawdownState(ctx context.Context, increment domain.Decimal) (domain.DrawdownState, error) {
	if increment.Sign() <= 0 {
		return domain.DrawdownState{}, fmt.Errorf("%w: the increment %q must be positive", domain.ErrInvalidInput, increment)
	}

	rows, err := s.db.QueryxContext(ctx, drawdownStateSQL, string(increment), maxDrawdownLevels)
	if err != nil {
		return domain.DrawdownState{}, fmt.Errorf("reading drawdown state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	state := domain.DrawdownState{Levels: []domain.DrawdownLevel{}}

	for rows.Next() {
		var (
			level     sql.NullString
			crossedOn sql.NullTime
		)

		err := rows.Scan(&state.Date, &state.ValueUSD, &state.DrawdownPercent, &state.HighDate, &state.HighValueUSD, &level, &crossedOn)
		if err != nil {
			return domain.DrawdownState{}, fmt.Errorf("scanning drawdown state: %w", err)
		}

		if level.Valid {
			state.Levels = append(state.Levels, domain.DrawdownLevel{Level: domain.Decimal(level.String), CrossedOn: crossedOn.Time})
		}
	}

	if err := rows.Err(); err != nil {
		return domain.DrawdownState{}, fmt.Errorf("reading drawdown state: %w", err)
	}

	return state, nil
}
