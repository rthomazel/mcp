package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	// instrumentDrawdownSQL is the drawdown query over an instrument's adjusted closes: $1 the increment, $2 the
	// most levels and $3 the instrument id. The series is every stored price, so the high is the all-time one,
	// and it is empty, so nothing is judged, when the instrument has no price on the store's latest price date.
	instrumentDrawdownSQL = `WITH daily AS (
    SELECT date, adjusted_close AS securities FROM prices
    WHERE instrument_id = $3 AND EXISTS (SELECT 1 FROM prices WHERE instrument_id = $3 AND date = (SELECT MAX(date) FROM prices))
)` + drawdownStateBody

	// instrumentHighSQL is the high state query over an instrument's adjusted closes: $1 the instrument id. The
	// series is empty when the instrument has no price on the store's latest price date.
	instrumentHighSQL = `WITH daily AS (
    SELECT date, adjusted_close AS securities FROM prices
    WHERE instrument_id = $1 AND EXISTS (SELECT 1 FROM prices WHERE instrument_id = $1 AND date = (SELECT MAX(date) FROM prices))
)` + highStateBody

	// heldTickersSQL lists the symbols of the instruments held on the latest snapshot date.
	heldTickersSQL = `SELECT DISTINCT i.symbol FROM snapshots s JOIN instruments i ON i.id = s.instrument_id
WHERE s.date = (SELECT MAX(date) FROM snapshots) ORDER BY i.symbol`
)

// GetInstrumentDrawdownState returns the drawdown of the ticker's latest adjusted close from its all-time high.
func (s *Monitors) GetInstrumentDrawdownState(ctx context.Context, ticker string, increment domain.Decimal) (domain.DrawdownState, error) {
	if increment.Sign() <= 0 {
		return domain.DrawdownState{}, fmt.Errorf("%w: the increment %q must be positive", domain.ErrInvalidInput, increment)
	}

	id, err := s.resolveTicker(ctx, ticker)
	if err != nil {
		return domain.DrawdownState{}, err
	}

	return s.drawdownState(ctx, instrumentDrawdownSQL, string(increment), maxDrawdownLevels, id)
}

// GetInstrumentHighState returns the ticker's latest adjusted close against its highest earlier one.
func (s *Monitors) GetInstrumentHighState(ctx context.Context, ticker string) (*domain.HighState, error) {
	id, err := s.resolveTicker(ctx, ticker)
	if err != nil {
		return nil, err
	}

	return s.highState(ctx, instrumentHighSQL, id)
}

// ListHeldTickers returns the symbols held on the latest snapshot date, in order.
func (s *Monitors) ListHeldTickers(ctx context.Context) ([]string, error) {
	var tickers []string
	if err := s.db.SelectContext(ctx, &tickers, heldTickersSQL); err != nil {
		return nil, fmt.Errorf("listing held tickers: %w", err)
	}

	return tickers, nil
}

// priceTargetSQL takes $1 the instrument id and $2 the target price. The row is the instrument's price on the
// latest price date of the store, and reached is the target within its low to high range, ends included.
const priceTargetSQL = `SELECT date, trim_scale(low)::text, trim_scale(high)::text, trim_scale(close)::text,
       low <= $2::numeric AND $2::numeric <= high
FROM prices WHERE instrument_id = $1 AND date = (SELECT MAX(date) FROM prices)`

// GetPriceTarget returns the ticker's price range on the latest price date and whether it holds the price.
func (s *Monitors) GetPriceTarget(ctx context.Context, ticker string, price domain.Decimal) (*domain.PriceTarget, error) {
	if price.Sign() <= 0 {
		return nil, fmt.Errorf("%w: the price %q must be positive", domain.ErrInvalidInput, price)
	}

	id, err := s.resolveTicker(ctx, ticker)
	if err != nil {
		return nil, err
	}

	var result domain.PriceTarget

	err = s.db.QueryRowxContext(ctx, priceTargetSQL, id, string(price)).Scan(
		&result.Date, &result.Low, &result.High, &result.Close, &result.Reached)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no price on the latest date
	}

	if err != nil {
		return nil, fmt.Errorf("reading price range of %s: %w", ticker, err)
	}

	return &result, nil
}
