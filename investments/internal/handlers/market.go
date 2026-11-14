package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
)

// refreshResultView is the refresh_prices result. Error is set when the run
// itself ended early, with the results gathered so far.
type refreshResultView struct {
	Results []refreshView `json:"results"`
	Error   string        `json:"error,omitempty"`
}

// HandleGetPrices reads stored daily prices of one instrument.
func (h *Handler) HandleGetPrices(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_prices"

	symbol, err := requireString(req, "symbol")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	from, err := optionalDate(req, "from")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	to, err := optionalDate(req, "to")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	instrument, err := h.resolveInstrument(ctx, symbol, req.GetString("exchange", ""))
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	bars, err := h.store.ListPrices(ctx, domain.PriceFilter{
		InstrumentID: instrument.ID, From: from, To: to, Limit: req.GetInt("limit", 0),
	})
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(toPriceBarViews(bars))
}

// HandleRefreshPrices fetches prices from the provider now.
func (h *Handler) HandleRefreshPrices(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "refresh_prices"

	from, err := optionalDate(req, "from")
	if err == nil && from != nil && from.After(time.Now().UTC()) {
		err = fmt.Errorf("%w: from must not be in the future", domain.ErrInvalidInput)
	}

	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, true)
	defer cancel()

	var results []marketdata.RefreshResult

	if symbol := req.GetString("symbol", ""); symbol != "" {
		instrument, err := h.resolveInstrument(ctx, symbol, req.GetString("exchange", ""))
		if err != nil {
			return h.failure(ctx, tool, err), nil
		}

		results = []marketdata.RefreshResult{h.refresher.RefreshInstrument(ctx, instrument, from)}
	} else if results, err = h.refresher.RefreshAll(ctx, from); err != nil {
		return h.refreshEndedEarly(ctx, tool, results, err)
	}

	view := refreshResultView{Results: make([]refreshView, 0, len(results))}
	for _, result := range results {
		view.Results = append(view.Results, toRefreshView(result))
	}

	return jsonResult(view)
}

// refreshEndedEarly reports the failure together with any results gathered.
func (h *Handler) refreshEndedEarly(ctx context.Context, tool string, results []marketdata.RefreshResult, err error) (*mcp.CallToolResult, error) {
	failure := h.failure(ctx, tool, err)
	if len(results) == 0 {
		return failure, nil
	}

	view := refreshResultView{Results: make([]refreshView, 0, len(results)), Error: failureText(failure)}
	for _, result := range results {
		view.Results = append(view.Results, toRefreshView(result))
	}

	rendered, marshalErr := jsonResult(view)
	if marshalErr == nil {
		rendered.IsError = true
	}

	return rendered, marshalErr
}

func failureText(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			return text.Text
		}
	}

	return internalErrorText
}
