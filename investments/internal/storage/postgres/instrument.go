package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	insertInstrumentSQL = `
INSERT INTO instruments (symbol, exchange, name, currency, instrument_type)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, symbol, exchange, name, currency, instrument_type`

	listInstrumentsSQL = `
SELECT id, symbol, exchange, name, currency, instrument_type
FROM instruments
ORDER BY symbol, exchange`
)

// RegisterInstrument normalizes, validates and inserts draft.
func (s *Store) RegisterInstrument(ctx context.Context, draft domain.Instrument) (domain.Instrument, error) {
	draft = draft.Normalize()
	if err := draft.Validate(); err != nil {
		return domain.Instrument{}, err
	}

	var instrument domain.Instrument

	err := s.db.QueryRowxContext(ctx, insertInstrumentSQL,
		draft.Symbol, draft.Exchange, draft.Name, draft.Currency, draft.InstrumentType,
	).Scan(&instrument.ID, &instrument.Symbol, &instrument.Exchange, &instrument.Name,
		&instrument.Currency, &instrument.InstrumentType)
	if err != nil {
		switch {
		case isUniqueViolation(err):
			return domain.Instrument{}, fmt.Errorf("%w: %s on %s", domain.ErrInstrumentExists, draft.Symbol, draft.Exchange)
		case isCheckViolation(err):
			return domain.Instrument{}, fmt.Errorf("%w: rejected by database check: %w", domain.ErrInvalidInput, err)
		}

		return domain.Instrument{}, fmt.Errorf("inserting instrument: %w", err)
	}

	slog.InfoContext(ctx, "instrument registered", "id", instrument.ID, "symbol", instrument.Symbol, "exchange", instrument.Exchange)

	return instrument, nil
}

// ListInstruments returns every instrument ordered by symbol, then exchange.
func (s *Store) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	rows, err := s.db.QueryxContext(ctx, listInstrumentsSQL)
	if err != nil {
		return nil, fmt.Errorf("listing instruments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	instruments := []domain.Instrument{}

	for rows.Next() {
		var instrument domain.Instrument
		if err := rows.Scan(&instrument.ID, &instrument.Symbol, &instrument.Exchange, &instrument.Name,
			&instrument.Currency, &instrument.InstrumentType); err != nil {
			return nil, fmt.Errorf("scanning instrument: %w", err)
		}

		instruments = append(instruments, instrument)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing instruments: %w", err)
	}

	return instruments, nil
}
