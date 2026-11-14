package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
)

const (
	// longTimeout bounds tools that call the provider or rewrite many rows.
	longTimeout       = 3 * time.Minute
	internalErrorText = "internal error, see the server logs"

	dateFormat      = "2006-01-02"
	dateDescription = "YYYY-MM-DD."
	exchangeText    = "Optional. Listing exchange, such as NYSE; needed only when the symbol is registered on more than one exchange."
	limitText       = "Optional. Maximum rows to return, default 100, maximum 500."
)

// Handler is the MCP tool layer. It holds no state beyond its dependencies.
type Handler struct {
	store        domain.Store
	refresher    *marketdata.Refresher
	queryTimeout time.Duration
}

// New builds a Handler.
func New(store domain.Store, refresher *marketdata.Refresher, queryTimeout time.Duration) *Handler {
	return &Handler{store: store, refresher: refresher, queryTimeout: queryTimeout}
}

// Register adds the eleven tools to the server.
func (h *Handler) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("add_instrument",
		mcp.WithDescription("Register a USD-quoted stock, ETF or fund by symbol and exchange, then download its full daily price history from the provider. Returns the instrument. Fails without registering when the symbol is not covered by the provider, the currency is not USD, or the symbol and exchange pair already exists. If the instrument registers but the price download fails, it stays registered and the error says so; refresh_prices retries the download. Call this before add_transaction for a new holding."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Ticker symbol, such as VTI.")),
		mcp.WithString("exchange", mcp.Required(), mcp.Description("Listing exchange, such as NYSE.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Display name, such as Vanguard Total Stock Market ETF.")),
		mcp.WithString("instrument_type", mcp.Required(), mcp.Enum("STOCK", "ETF", "FUND", "OTHER"), mcp.Description("Kind of instrument.")),
		mcp.WithString("currency", mcp.Description("Optional. Quote currency, default USD; any other value is rejected.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), h.HandleAddInstrument)

	s.AddTool(mcp.NewTool("list_instruments",
		mcp.WithDescription("List every registered instrument with symbol, exchange, name, type and currency, ordered by symbol. Call this to learn which symbols exist and which need an exchange to be unambiguous."),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleListInstruments)

	s.AddTool(mcp.NewTool("add_transaction",
		mcp.WithDescription("Append one ledger entry for a registered instrument. Signs are enforced per event type: BUY has positive quantity_change and negative cash_amount, SELL negative quantity and positive cash, DIVIDEND zero quantity and positive cash, TAX zero quantity and negative cash, SPLIT a nonzero quantity and no cash, ADJUSTMENT a nonzero quantity with optional cash. Holdings are long-only: an entry that would make ownership negative at any point in the timeline, backdated ones included, fails and writes nothing, and the error names the instrument, date and running quantity. Entries cannot be edited or deleted. Returns the stored entry. A buy or other entry that adds quantity to an instrument that was not held also fetches that instrument's prices from its latest stored date through today, so a new or re-opened position is valued right away; the result reports how many bars were stored, or the error text when that fetch failed, in which case the entry is still recorded and refresh_prices retries. When the entry changes history that snapshots already cover, the result includes rebuild_from, the date to pass to rebuild_snapshots; recording never rebuilds on its own."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Ticker symbol of a registered instrument.")),
		mcp.WithString("exchange", mcp.Description(exchangeText)),
		mcp.WithString("event_type", mcp.Required(), mcp.Enum("BUY", "SELL", "DIVIDEND", "TAX", "SPLIT", "ADJUSTMENT"), mcp.Description("Kind of event.")),
		mcp.WithString("quantity_change", mcp.Required(), mcp.Description("Decimal share quantity change, signed per the event type, 0 for DIVIDEND and TAX.")),
		mcp.WithString("cash_amount", mcp.Description("Optional. Decimal USD cash impact, signed per the event type; 0 when absent.")),
		mcp.WithString("executed_at", mcp.Required(), mcp.Description("When the event happened, RFC 3339.")),
		mcp.WithString("effective_date", mcp.Required(), mcp.Description("Date the event economically applies to, "+dateDescription+" After-hours events carry the next session's date.")),
		mcp.WithString("notes", mcp.Description("Optional. Free text, up to 1000 characters.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), h.HandleAddTransaction)

	s.AddTool(mcp.NewTool("list_transactions",
		mcp.WithDescription("List ledger entries newest first by ledger order, optionally for one instrument, one event type such as DIVIDEND, and an inclusive effective-date range. Returns each entry with its symbol and exchange."),
		mcp.WithString("symbol", mcp.Description("Optional. Restrict to one instrument.")),
		mcp.WithString("exchange", mcp.Description(exchangeText)),
		mcp.WithString("event_type", mcp.Enum("BUY", "SELL", "DIVIDEND", "TAX", "SPLIT", "ADJUSTMENT"), mcp.Description("Optional. Restrict to one event type.")),
		mcp.WithString("from", mcp.Description("Optional. Inclusive effective date, "+dateDescription)),
		mcp.WithString("to", mcp.Description("Optional. Inclusive effective date, "+dateDescription)),
		mcp.WithInteger("limit", mcp.Min(1), mcp.Max(500), mcp.Description(limitText)),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleListTransactions)

	s.AddTool(mcp.NewTool("get_positions",
		mcp.WithDescription("Reconstruct active positions, instruments with a positive quantity, from the ledger as of an effective date, or the whole ledger when no date is given. Returns the positions, each with its name and type, plus any integrity errors, instruments whose running quantity goes negative; report those to the user, they are never omitted. Quantities only, no prices or values: use get_portfolio for what the holdings are worth."),
		mcp.WithString("date", mcp.Description("Optional. Positions as of this effective date, "+dateDescription+" The whole ledger when absent.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetPositions)

	s.AddTool(mcp.NewTool("get_portfolio",
		mcp.WithDescription("Value active positions at real closing prices for a date, default today in the New York market timezone. Securities only, cash excluded. Returns per-holding name, type, quantity, close, price date and market value, plus the total and each allocation. A holding with no price, or a price five or more days older than the date, makes the portfolio incomplete: the total and allocations are withheld and the affected holdings are flagged missing or stale. Fix that with refresh_prices. Use get_positions when only quantities are needed, or to see holdings on a past date regardless of prices."),
		mcp.WithString("date", mcp.Description("Optional. Valuation date, "+dateDescription+" Today in New York when absent.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetPortfolio)

	s.AddTool(mcp.NewTool("get_prices",
		mcp.WithDescription("Read stored daily prices for one instrument, newest first, with open, high, low, close, adjusted close and volume. Valuation uses close, never adjusted close. Reads stored data only; refresh_prices fetches newer prices."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Ticker symbol of a registered instrument.")),
		mcp.WithString("exchange", mcp.Description(exchangeText)),
		mcp.WithString("from", mcp.Description("Optional. Inclusive start date, "+dateDescription)),
		mcp.WithString("to", mcp.Description("Optional. Inclusive end date, "+dateDescription)),
		mcp.WithInteger("limit", mcp.Min(1), mcp.Max(500), mcp.Description(limitText)),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetPrices)

	s.AddTool(mcp.NewTool("refresh_prices",
		mcp.WithDescription("Fetch new daily prices from the provider now, for one named instrument or for every held instrument (positive quantity; sold ones are skipped), starting at each instrument's latest stored date inclusive so revised days are corrected, or at an earlier from date to re-fetch older history. Returns one result per instrument with symbol, exchange, bars stored and an error, empty on success; one instrument failing does not stop the others. When the provider rate limits, the run stops and the remaining instruments are reported as not refreshed; run it again later. The provider allows few requests per hour, so do not repeat it needlessly. Pass from only to correct older prices; the default refresh already covers the latest day. Writes no snapshot, and snapshots already stored are not recalculated when older prices change: follow a correction with rebuild_snapshots from that date."),
		mcp.WithString("symbol", mcp.Description("Optional. Refresh only this instrument, even if it is not held; every held instrument when absent.")),
		mcp.WithString("exchange", mcp.Description(exchangeText)),
		mcp.WithString("from", mcp.Description("Optional. Re-fetch history from this date, "+dateDescription+" Replaces the stored prices from it onward.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	), h.HandleRefreshPrices)

	s.AddTool(mcp.NewTool("get_snapshots",
		mcp.WithDescription("Read stored daily valuation snapshots, newest date first, grouped by date with each date's securities total and each holding's quantity, close, price date, market value and allocation. A date has snapshots only if every active holding had a price on it, and stored snapshots never change when holdings or prices change later. Use this for history and trends; get_portfolio gives the current value."),
		mcp.WithString("symbol", mcp.Description("Optional. Restrict to one instrument; each date's total still covers the whole portfolio.")),
		mcp.WithString("exchange", mcp.Description(exchangeText)),
		mcp.WithString("from", mcp.Description("Optional. Inclusive start date, "+dateDescription)),
		mcp.WithString("to", mcp.Description("Optional. Inclusive end date, "+dateDescription)),
		mcp.WithInteger("limit", mcp.Min(1), mcp.Max(500), mcp.Description(limitText)),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetSnapshots)

	s.AddTool(mcp.NewTool("rebuild_snapshots",
		mcp.WithDescription("Replace the stored snapshots for every price date in the range with values recomputed from the current ledger and prices. Use only after a correction that touches history, such as a backdated transaction or revised prices, starting at the rebuild_from date add_transaction returned. All or nothing: if any date in the range has an active holding without a price on it, the error names the first such date and the instruments, nothing changes, and the range must be narrowed or prices refreshed. Also fills price dates in the range that have no snapshot yet, such as a missed day. Returns the range, dates rebuilt and rows written. The rebuild is logged with the reason."),
		mcp.WithString("from", mcp.Required(), mcp.Description("Inclusive start date, "+dateDescription)),
		mcp.WithString("to", mcp.Required(), mcp.Description("Inclusive end date, "+dateDescription)),
		mcp.WithString("reason", mcp.Required(), mcp.Description("Why the rebuild is needed; recorded in the updates log.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	), h.HandleRebuildSnapshots)

	s.AddTool(mcp.NewTool("list_updates",
		mcp.WithDescription("Read the log of price refreshes, daily snapshot runs and snapshot rebuilds, newest first. Each entry has its type, status, start and end time, records written, the date or date range it covers, the provider for refreshes, the reason for rebuilds and the error message when it did not complete. A status of incomplete means a run finished but could not value every holding or refresh every instrument; failed means a rebuild was refused or a refresh could not read the held instruments. Use it to check freshness and why a day has no snapshot, then refresh_prices or rebuild_snapshots to fix it. A completed snapshots entry marks its date as done."),
		mcp.WithString("data_type", mcp.Enum(domain.UpdateDataPrices, domain.UpdateDataSnapshots, domain.UpdateDataSnapshotRebuild), mcp.Description("Optional. Restrict to one kind of run.")),
		mcp.WithString("status", mcp.Enum(domain.UpdateStatusCompleted, domain.UpdateStatusIncomplete, domain.UpdateStatusFailed), mcp.Description("Optional. Restrict to one outcome.")),
		mcp.WithInteger("limit", mcp.Min(1), mcp.Max(500), mcp.Description(limitText)),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleListUpdates)
}

// withTimeout derives ctx with the long timeout when long, the query timeout otherwise.
func (h *Handler) withTimeout(ctx context.Context, long bool) (context.Context, context.CancelFunc) {
	if long {
		return context.WithTimeout(ctx, longTimeout)
	}

	return context.WithTimeout(ctx, h.queryTimeout)
}

// failure turns an error into a tool error. Caller mistakes carry their
// message and are not logged; anything else is logged and hidden. Messages
// never carry the provider credential, which only travels in a request header.
func (h *Handler) failure(ctx context.Context, tool string, err error) *mcp.CallToolResult {
	var negative *domain.NegativeBalanceError

	switch {
	case errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrInstrumentNotFound),
		errors.Is(err, domain.ErrInstrumentExists), errors.As(err, &negative),
		errors.Is(err, domain.ErrSnapshotIncomplete), errors.Is(err, domain.ErrSnapshotExists),
		errors.Is(err, domain.ErrSymbolNotFound), errors.Is(err, domain.ErrRateLimited):
		return mcp.NewToolResultError(err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		slog.WarnContext(ctx, "tool call timed out", "tool", tool)

		return mcp.NewToolResultError("the call timed out; nothing further was attempted")
	default:
		slog.ErrorContext(ctx, "tool call failed", "tool", tool, "err", err)

		return mcp.NewToolResultError(internalErrorText)
	}
}

// jsonResult renders value as indented JSON text.
func jsonResult(value any) (*mcp.CallToolResult, error) {
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		slog.Error("marshalling tool result failed", "err", err)

		return mcp.NewToolResultError(internalErrorText), nil
	}

	return mcp.NewToolResultText(string(text)), nil
}

// findInstrument matches the symbol, and the exchange when given, among instruments.
func findInstrument(instruments []domain.Instrument, symbol, exchange string) (domain.Instrument, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	exchange = strings.ToUpper(strings.TrimSpace(exchange))

	var matches []domain.Instrument

	for _, instrument := range instruments {
		if instrument.Symbol == symbol && (exchange == "" || instrument.Exchange == exchange) {
			matches = append(matches, instrument)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return domain.Instrument{}, fmt.Errorf("%w: %s", domain.ErrInstrumentNotFound, symbol)
	default:
		exchanges := make([]string, 0, len(matches))
		for _, match := range matches {
			exchanges = append(exchanges, match.Exchange)
		}

		return domain.Instrument{}, fmt.Errorf("%w: %s is registered on %s; give the exchange",
			domain.ErrInvalidInput, symbol, strings.Join(exchanges, ", "))
	}
}

// resolveInstrument lists the instruments and finds the one named.
func (h *Handler) resolveInstrument(ctx context.Context, symbol, exchange string) (domain.Instrument, error) {
	instruments, err := h.store.ListInstruments(ctx)
	if err != nil {
		return domain.Instrument{}, fmt.Errorf("listing instruments: %w", err)
	}

	return findInstrument(instruments, symbol, exchange)
}

// optionalInstrumentID resolves symbol to an id among instruments, nil when symbol is empty.
func optionalInstrumentID(instruments []domain.Instrument, symbol, exchange string) (*int, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, nil //nolint:nilnil // nil means no filter
	}

	instrument, err := findInstrument(instruments, symbol, exchange)
	if err != nil {
		return nil, err
	}

	return &instrument.ID, nil
}

func indexInstruments(instruments []domain.Instrument) map[int]domain.Instrument {
	index := make(map[int]domain.Instrument, len(instruments))
	for _, instrument := range instruments {
		index[instrument.ID] = instrument
	}

	return index
}

func requireString(req mcp.CallToolRequest, name string) (string, error) {
	value := strings.TrimSpace(req.GetString(name, ""))
	if value == "" {
		return "", fmt.Errorf("%w: missing required parameter: %s", domain.ErrInvalidInput, name)
	}

	return value, nil
}

// parseDate parses an optional YYYY-MM-DD value; empty is nil.
func parseDate(name, value string) (*domain.Date, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil //nolint:nilnil // nil means absent
	}

	parsed, err := time.Parse(dateFormat, value)
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be YYYY-MM-DD, got %q", domain.ErrInvalidInput, name, value)
	}

	return &parsed, nil
}

func requireDate(req mcp.CallToolRequest, name string) (domain.Date, error) {
	value, err := requireString(req, name)
	if err != nil {
		return domain.Date{}, err
	}

	date, err := parseDate(name, value)
	if err != nil {
		return domain.Date{}, err
	}

	return *date, nil
}

func optionalDate(req mcp.CallToolRequest, name string) (*domain.Date, error) {
	return parseDate(name, req.GetString(name, ""))
}

// parseTimestamp parses a required RFC 3339 value.
func parseTimestamp(name, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be RFC 3339, got %q", domain.ErrInvalidInput, name, value)
	}

	return parsed, nil
}
