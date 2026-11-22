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
	"github.com/rthomazel/mcp/investments/internal/worker/marketdata"
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
	store        domain.Store // the generic store; the others are in stores
	stores       domain.Stores
	refresher    *marketdata.PriceRefresher
	queryTimeout time.Duration
}

// New builds a Handler.
func New(stores domain.Stores, refresher *marketdata.PriceRefresher, queryTimeout time.Duration) *Handler {
	return &Handler{store: stores.Store, stores: stores, refresher: refresher, queryTimeout: queryTimeout}
}

// Register adds the tools to the server.
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
		mcp.WithDescription("Append one ledger entry for a registered instrument. Signs are enforced per event type, and only BUY and SELL may carry a fee: BUY has positive quantity_change and negative cash_amount, SELL negative quantity and positive cash, DIVIDEND zero quantity and positive cash, TAX zero quantity and negative cash, SPLIT a nonzero quantity and no cash, ADJUSTMENT a nonzero quantity with optional cash. Holdings are long-only: an entry that would make ownership negative at any point in the timeline, backdated ones included, fails and writes nothing, and the error names the instrument, date and running quantity. Entries cannot be edited or deleted. Returns the stored entry. A buy or other entry that adds quantity to an instrument that was not held also fetches that instrument's prices from its latest stored date through today, so a new or re-opened position is valued right away; the result reports how many bars were stored, or the error text when that fetch failed, in which case the entry is still recorded and refresh_prices retries. When the entry changes history that snapshots already cover, the result includes rebuild_from, the date to pass to rebuild_snapshots; recording never rebuilds on its own."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Ticker symbol of a registered instrument.")),
		mcp.WithString("exchange", mcp.Description(exchangeText)),
		mcp.WithString("event_type", mcp.Required(), mcp.Enum("BUY", "SELL", "DIVIDEND", "TAX", "SPLIT", "ADJUSTMENT"), mcp.Description("Kind of event.")),
		mcp.WithString("quantity_change", mcp.Required(), mcp.Description("Decimal share quantity change, signed per the event type, 0 for DIVIDEND and TAX.")),
		mcp.WithString("cash_amount", mcp.Description("Optional. Decimal USD cash impact, signed per the event type; 0 when absent.")),
		mcp.WithString("executed_at", mcp.Required(), mcp.Description("When the event happened, RFC 3339.")),
		mcp.WithString("effective_date", mcp.Required(), mcp.Description("Date the event economically applies to, "+dateDescription+" After-hours events carry the next session's date.")),
		mcp.WithString("fee", mcp.Description("Optional. Non-negative decimal USD trade fee, BUY and SELL only; the part of cash_amount that was a fee, never in addition to it. Zero when absent.")),
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
		mcp.WithDescription("Value active positions at real closing prices for a date, default today in the New York market timezone. The securities value excludes cash. Returns per-holding name, type, quantity, close, price date and market value, plus the total and each allocation, which are over securities only. Each holding also carries total cost, average cost, unrealized P&L in dollars and as a percent of cost, and cumulative realized P&L, dividends received, tax withheld and trade fees paid, each separate, plus a total return summing unrealized and realized P&L plus dividends minus tax. Fees are already inside cost and sale proceeds, so they are shown for information and never subtracted again. Fully sold positions are listed separately with their realized P&L and dividends. P&L totals add up every position, open and closed. Total return is a position total, not a time-weighted or money-weighted return. Beside them it returns the cash balance and buying power, and account equity, securities value plus cash; cash is null when no opening balance exists yet, and equity is withheld when the portfolio is incomplete or cash is not initialized. A holding with no price, or a price five or more days older than the date, makes the portfolio incomplete: the total, allocations, market value and unrealized P&L figures that need a price are withheld and the affected holdings are flagged missing or stale. Fix that with refresh_prices. Use get_positions when only quantities are needed, or to see holdings on a past date regardless of prices."),
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
		mcp.WithDescription("Replace the stored snapshots for every price date in the range with values recomputed from the current ledger and prices. Use only after a correction that touches history, such as a backdated transaction or revised prices, starting at the rebuild_from date add_transaction or add_cash_transaction returned. Cash snapshots are replaced with the securities snapshots. All or nothing: if any date in the range has an active holding without a price on it, the error names the first such date and the instruments, nothing changes, and the range must be narrowed or prices refreshed. Also fills price dates in the range that have no snapshot yet, such as a missed day. Returns the range, dates rebuilt, rows written and dates with a cash snapshot. The rebuild is logged with the reason."),
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
	s.AddTool(mcp.NewTool("add_cash_transaction",
		mcp.WithDescription("Append one cash entry that belongs to no instrument: the opening balance, a deposit, a withdrawal, interest or an account fee. Signs are enforced per kind: OPENING_BALANCE any sign, DEPOSIT and INTEREST positive, WITHDRAWAL and FEE negative. Only one opening balance can exist, and cash is reported only from its effective date onward, so record it first. Trades, dividends and taxes are not entered here; their cash impact is counted from add_transaction. Entries cannot be edited or deleted. A balance that goes negative is recorded and reported as an integrity error, never rejected. Returns the stored entry. When the entry changes a balance that cash snapshots already cover, the result includes rebuild_from, the date to pass to rebuild_snapshots; recording never rebuilds on its own."),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("OPENING_BALANCE", "DEPOSIT", "WITHDRAWAL", "INTEREST", "FEE"), mcp.Description("Kind of cash entry.")),
		mcp.WithString("amount", mcp.Required(), mcp.Description("Decimal USD amount, signed per the kind.")),
		mcp.WithString("executed_at", mcp.Required(), mcp.Description("When the event happened, RFC 3339.")),
		mcp.WithString("effective_date", mcp.Required(), mcp.Description("Date the event economically applies to, "+dateDescription)),
		mcp.WithString("notes", mcp.Description("Optional. Free text, up to 1000 characters.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), h.HandleAddCashTransaction)

	s.AddTool(mcp.NewTool("list_cash_transactions",
		mcp.WithDescription("List cash entries, not trades, dividends or taxes, newest first by effective date, optionally for one kind and an inclusive effective-date range. Use list_transactions for the instrument ledger."),
		mcp.WithString("kind", mcp.Enum("OPENING_BALANCE", "DEPOSIT", "WITHDRAWAL", "INTEREST", "FEE"), mcp.Description("Optional. Restrict to one kind.")),
		mcp.WithString("from", mcp.Description("Optional. Inclusive effective date, "+dateDescription)),
		mcp.WithString("to", mcp.Description("Optional. Inclusive effective date, "+dateDescription)),
		mcp.WithInteger("limit", mcp.Min(1), mcp.Max(500), mcp.Description(limitText)),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleListCashTransactions)

	s.AddTool(mcp.NewTool("get_cash",
		mcp.WithDescription("Report the cash balance and buying power as of a date, default today in the New York market timezone. Cash is the opening balance plus cash entries and the cash impact of trades, dividends and taxes from the opening balance date onward; buying power is the balance floored at zero, there is no margin. Without an opening balance on or before the date, cash is reported as not initialized, with no balance or buying power, and the opening date when one exists later: record one with add_cash_transaction first. Integrity errors list any date the running balance went negative; report those to the user, they are never omitted. Securities value and account equity are in get_portfolio."),
		mcp.WithString("date", mcp.Description("Optional. Balance as of this date, "+dateDescription+" Today in New York when absent.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetCash)

	s.AddTool(mcp.NewTool("get_status",
		mcp.WithDescription("Check whether the portfolio data is healthy, as of today in the New York market timezone. Returns one verdict, healthy or needs attention, with a reason for each problem, and the facts behind it: for every held instrument its latest stored price date and whether that price is missing or stale, the latest price date overall, the latest price date covered by a completed snapshot, whether the latest price date still has no completed snapshot, and the failed or incomplete runs that no later run has resolved, the notification events given up on after repeated failures (each with the id remove_event takes), and, apart from the verdict, the expired events that were never handled. A snapshot missing for a price date just fetched is expected until the next price refresh run or a rebuild_snapshots. Sold instruments are not checked. Use refresh_prices for missing or stale prices and rebuild_snapshots for missing snapshots; use list_updates for the raw run log."),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetStatus)

	s.AddTool(mcp.NewTool("remove_event",
		mcp.WithDescription("Remove an event from the notification outbox once its failed delivery has been dealt with, by the id get_status lists for an event given up on or expired. Only those two can be removed: an event that is pending, backing off or already handled is refused and nothing is deleted. Removing deletes the record permanently and does not resend it, and a problem that is still unresolved can be announced again by its monitor. Returns the removed id. This is the only change the agent can make to the outbox."),
		mcp.WithNumber("id", mcp.Required(), mcp.Min(1), mcp.Description("The event id from get_status.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
	), h.HandleRemoveEvent)

	s.AddTool(mcp.NewTool("get_value_history",
		mcp.WithDescription("Report portfolio value over a date range with one point per window of window_days days, from the stored daily snapshots; 1 is daily, 7 about weekly, 30 about monthly, and windows are counted from the range start. Each point is the value on the last snapshot date within its window, so weekends and holidays never appear and dates without a snapshot are absent, never interpolated; points carry cash and account equity when a cash snapshot exists for the date. The summary, for securities value and again for account equity when cash snapshots exist, gives the latest value, the all-time high and its date measured from the earliest snapshot through the range end, the drawdown of the latest value from that high, the change from the baseline, which is the last snapshot on or before the range start and is named in the result, the high and low within the range, and the maximum drawdown within the range with its peak and trough dates and whether it has recovered. Net contributions, deposits minus withdrawals over the same span, are given with the equity summary. These are changes in value, not investment return: a deposit or a purchase raises securities value without being a gain, so read the equity figures against the net contributions. To look at a window such as the past six months since the all-time high, call once to get the high's date, then again from that date. A series over 1000 points is rejected; use a larger window_days or a narrower range."),
		mcp.WithString("from", mcp.Description("Optional. Inclusive start date, "+dateDescription+" The earliest snapshot when absent.")),
		mcp.WithString("to", mcp.Description("Optional. Inclusive end date, "+dateDescription+" The latest snapshot when absent.")),
		mcp.WithInteger("window_days", mcp.Min(1), mcp.Description("Optional. Days each point covers, default 1.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.HandleGetValueHistory)
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
		errors.Is(err, domain.ErrSymbolNotFound), errors.Is(err, domain.ErrRateLimited),
		errors.Is(err, domain.ErrEventNotFound), errors.Is(err, domain.ErrEventNotRemovable):
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
