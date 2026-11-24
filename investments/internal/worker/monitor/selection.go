package monitor

import (
	"context"
	"fmt"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// tickersOf resolves a selection to tickers: the listed ones, or the symbols held on the latest snapshot date.
func tickersOf(ctx context.Context, store domain.MonitorStore, selection Selection) ([]string, error) {
	if !selection.Held {
		return selection.Tickers, nil
	}

	tickers, err := store.ListHeldTickers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing held tickers: %w", err)
	}

	return tickers, nil
}

// selectionFrom reads the required instruments setting.
func selectionFrom(params Params) (Selection, error) {
	selection, ok, err := params.Instruments(instrumentsKey)
	if err != nil {
		return selection, err
	}

	if !ok {
		return selection, fmt.Errorf("setting %q is required", instrumentsKey)
	}

	return selection, nil
}
