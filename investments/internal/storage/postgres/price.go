package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	// $2 is a JSON array of bars with every decimal as a string, which keeps
	// the exact text of NUMERIC values.
	upsertPricesSQL = `
INSERT INTO prices (instrument_id, date, open, high, low, close, adjusted_close, volume)
SELECT $1, bar.date, bar.open, bar.high, bar.low, bar.close, bar.adjusted_close, bar.volume
FROM jsonb_to_recordset($2::jsonb) AS bar(
    date date, open numeric, high numeric, low numeric, close numeric, adjusted_close numeric, volume bigint
)
ON CONFLICT (instrument_id, date) DO UPDATE SET
    open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close,
    adjusted_close = EXCLUDED.adjusted_close,
    volume = EXCLUDED.volume`

	latestPriceDateSQL = `SELECT max(date) FROM prices WHERE instrument_id = $1`

	listPricesSQL = `
SELECT date, open::text, high::text, low::text, close::text, adjusted_close::text, volume
FROM prices
WHERE instrument_id = $1
  AND ($2::date IS NULL OR date >= $2)
  AND ($3::date IS NULL OR date <= $3)
ORDER BY date DESC
LIMIT $4`
)

type priceBarJSON struct {
	Date          string `json:"date"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Close         string `json:"close"`
	AdjustedClose string `json:"adjusted_close"`
	Volume        int64  `json:"volume"`
}

// UpsertPrices stores the bars in one statement, replacing existing dates.
func (s *Store) UpsertPrices(ctx context.Context, instrumentID int, bars []domain.PriceBar) error {
	if len(bars) == 0 {
		return nil
	}

	payload, err := marshalBars(bars)
	if err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, upsertPricesSQL, instrumentID, payload); err != nil {
		switch {
		case isForeignKeyViolation(err):
			return fmt.Errorf("%w: id %d", domain.ErrInstrumentNotFound, instrumentID)
		case isCheckViolation(err):
			return fmt.Errorf("%w: rejected by database check: %w", domain.ErrInvalidInput, err)
		}

		return fmt.Errorf("upserting prices: %w", err)
	}

	slog.InfoContext(ctx, "prices stored", "instrument_id", instrumentID, "bars", len(bars))

	return nil
}

// marshalBars validates the bars, rejects a repeated date and encodes them
// for upsertPricesSQL. ON CONFLICT DO UPDATE cannot touch a row twice in one
// statement, hence the repeated date check.
func marshalBars(bars []domain.PriceBar) ([]byte, error) {
	seen := make(map[time.Time]bool, len(bars))
	encoded := make([]priceBarJSON, 0, len(bars))

	for _, bar := range bars {
		if err := bar.Validate(); err != nil {
			return nil, err
		}

		if seen[bar.Date] {
			return nil, fmt.Errorf("%w: price date %s repeats", domain.ErrInvalidInput, bar.Date.Format(time.DateOnly))
		}

		seen[bar.Date] = true

		encoded = append(encoded, priceBarJSON{
			Date: bar.Date.Format(time.DateOnly), Open: string(bar.Open), High: string(bar.High),
			Low: string(bar.Low), Close: string(bar.Close), AdjustedClose: string(bar.AdjustedClose), Volume: bar.Volume,
		})
	}

	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encoding prices: %w", err)
	}

	return payload, nil
}

// LatestPriceDate returns the latest stored price date, nil when there is none.
func (s *Store) LatestPriceDate(ctx context.Context, instrumentID int) (*domain.Date, error) {
	var latest sql.NullTime
	if err := s.db.QueryRowxContext(ctx, latestPriceDateSQL, instrumentID).Scan(&latest); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("reading latest price date: %w", err)
	}

	if !latest.Valid {
		return nil, nil
	}

	date := latest.Time

	return &date, nil
}

// ListPrices returns the stored price records of one instrument, newest first.
func (s *Store) ListPrices(ctx context.Context, filter domain.PriceFilter) ([]domain.PriceBar, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	bars := []domain.PriceBar{}

	rows, err := s.db.QueryxContext(ctx, listPricesSQL, filter.InstrumentID, filter.From, filter.To, filter.ResolvedLimit())
	if err != nil {
		slog.ErrorContext(ctx, "listing prices failed", "instrument_id", filter.InstrumentID, "err", err)

		return nil, fmt.Errorf("listing prices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var bar domain.PriceBar
		if err := rows.Scan(&bar.Date, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.AdjustedClose, &bar.Volume); err != nil {
			slog.ErrorContext(ctx, "scanning price failed", "err", err)

			return nil, fmt.Errorf("scanning price: %w", err)
		}

		bars = append(bars, bar)
	}

	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "listing prices failed", "instrument_id", filter.InstrumentID, "err", err)

		return nil, fmt.Errorf("listing prices: %w", err)
	}

	return bars, nil
}
