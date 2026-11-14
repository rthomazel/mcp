package handlers

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const defaultCurrency = "USD"

// HandleAddInstrument registers an instrument and backfills its full price history.
func (h *Handler) HandleAddInstrument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	draft := domain.Instrument{Currency: req.GetString("currency", defaultCurrency)}

	required := []struct {
		name   string
		target *string
	}{
		{"symbol", &draft.Symbol}, {"exchange", &draft.Exchange}, {"name", &draft.Name},
		{"instrument_type", &draft.InstrumentType},
	}

	for _, field := range required {
		value, err := requireString(req, field.name)
		if err != nil {
			return h.failure(ctx, "add_instrument", err), nil
		}

		*field.target = value
	}

	ctx, cancel := h.withTimeout(ctx, true)
	defer cancel()

	instrument, err := h.refresher.RegisterInstrument(ctx, draft)
	if err != nil {
		if instrument.ID != 0 {
			slog.ErrorContext(ctx, "price backfill failed after registering", "symbol", instrument.Symbol, "err", err)

			return mcp.NewToolResultError("instrument " + instrument.Symbol +
				" is registered but its price download failed; refresh_prices retries it: " + err.Error()), nil
		}

		return h.failure(ctx, "add_instrument", err), nil
	}

	return jsonResult(toInstrumentView(instrument))
}

// HandleListInstruments lists every registered instrument.
func (h *Handler) HandleListInstruments(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	instruments, err := h.store.ListInstruments(ctx)
	if err != nil {
		return h.failure(ctx, "list_instruments", err), nil
	}

	views := make([]instrumentView, 0, len(instruments))
	for _, instrument := range instruments {
		views = append(views, toInstrumentView(instrument))
	}

	return jsonResult(views)
}
