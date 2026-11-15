package handlers

import (
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
)

// View types render domain values for the agent: snake_case keys, exact
// decimal strings, YYYY-MM-DD dates, null for absent values and no internal ids.

type instrumentView struct {
	Symbol         string `json:"symbol"`
	Exchange       string `json:"exchange"`
	Name           string `json:"name"`
	InstrumentType string `json:"instrument_type"`
	Currency       string `json:"currency"`
}

type transactionView struct {
	Symbol         string  `json:"symbol"`
	Exchange       string  `json:"exchange"`
	EventType      string  `json:"event_type"`
	QuantityChange string  `json:"quantity_change"`
	CashAmount     string  `json:"cash_amount"`
	ExecutedAt     string  `json:"executed_at"`
	EffectiveDate  string  `json:"effective_date"`
	Notes          *string `json:"notes"`
	Fee            string  `json:"fee"`
}

type positionView struct {
	Symbol         string `json:"symbol"`
	Exchange       string `json:"exchange"`
	Name           string `json:"name"`
	InstrumentType string `json:"instrument_type"`
	Quantity       string `json:"quantity"`
}

type integrityView struct {
	Symbol            string `json:"symbol"`
	Exchange          string `json:"exchange"`
	FirstNegativeDate string `json:"first_negative_date"`
	Quantity          string `json:"quantity"`
}

type holdingView struct {
	Symbol         string  `json:"symbol"`
	Exchange       string  `json:"exchange"`
	Name           string  `json:"name"`
	InstrumentType string  `json:"instrument_type"`
	Quantity       string  `json:"quantity"`
	ClosePrice     *string `json:"close_price"`
	PriceDate      *string `json:"price_date"`
	MarketValueUSD *string `json:"market_value_usd"`
	Allocation     *string `json:"allocation"`
	PriceIssue     *string `json:"price_issue"`

	CostUSD              string  `json:"cost_usd"`
	AverageCostUSD       string  `json:"average_cost_usd"`
	UnrealizedPnLUSD     *string `json:"unrealized_pnl_usd"`
	UnrealizedPnLPercent *string `json:"unrealized_pnl_percent"`
	RealizedPnLUSD       string  `json:"realized_pnl_usd"`
	DividendsUSD         string  `json:"dividends_usd"`
	TaxUSD               string  `json:"tax_usd"`
	FeesUSD              string  `json:"fees_usd"`
	TotalReturnUSD       *string `json:"total_return_usd"`
}

type portfolioView struct {
	Date               string               `json:"date"`
	SecuritiesValueUSD *string              `json:"securities_value_usd"`
	Complete           bool                 `json:"complete"`
	Holdings           []holdingView        `json:"holdings"`
	IntegrityErrors    []integrityView      `json:"integrity_errors"`
	Cash               cashBalanceView      `json:"cash"`
	AccountEquityUSD   *string              `json:"account_equity_usd"`
	ClosedPositions    []closedPositionView `json:"closed_positions"`
	PnL                pnlTotalsView        `json:"pnl"`
}

type priceBarView struct {
	Date          string `json:"date"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Close         string `json:"close"`
	AdjustedClose string `json:"adjusted_close"`
	Volume        int64  `json:"volume"`
}

type snapshotRowView struct {
	Symbol         string `json:"symbol"`
	Exchange       string `json:"exchange"`
	Quantity       string `json:"quantity"`
	ClosePrice     string `json:"close_price"`
	PriceDate      string `json:"price_date"`
	MarketValueUSD string `json:"market_value_usd"`
	Allocation     string `json:"allocation"`
}

type snapshotDayView struct {
	Date               string            `json:"date"`
	SecuritiesValueUSD string            `json:"securities_value_usd"`
	Holdings           []snapshotRowView `json:"holdings"`
}

type refreshView struct {
	Symbol   string  `json:"symbol"`
	Exchange string  `json:"exchange"`
	From     *string `json:"from"`
	Bars     int     `json:"bars"`
	Error    string  `json:"error"`
}

type rebuildView struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Dates     int    `json:"dates"`
	Rows      int    `json:"rows"`
	CashDates int    `json:"cash_dates"`
}

type updateView struct {
	ID             int64   `json:"id"`
	DataType       string  `json:"data_type"`
	Status         string  `json:"status"`
	Provider       *string `json:"provider"`
	StartedAt      string  `json:"started_at"`
	CompletedAt    *string `json:"completed_at"`
	RecordsUpdated int     `json:"records_updated"`
	DateFrom       *string `json:"date_from"`
	DateTo         *string `json:"date_to"`
	Reason         *string `json:"reason"`
	ErrorMessage   *string `json:"error_message"`
}

func formatDate(date domain.Date) string { return date.Format(dateFormat) }

func formatOptionalDate(date *domain.Date) *string {
	if date == nil {
		return nil
	}

	text := formatDate(*date)

	return &text
}

func optionalText(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}

func optionalDecimal(value *domain.Decimal) *string {
	if value == nil {
		return nil
	}

	text := string(*value)

	return &text
}

func toInstrumentView(instrument domain.Instrument) instrumentView {
	return instrumentView{
		Symbol: instrument.Symbol, Exchange: instrument.Exchange, Name: instrument.Name,
		InstrumentType: instrument.InstrumentType, Currency: instrument.Currency,
	}
}

func toTransactionView(entry domain.Transaction, instrument domain.Instrument) transactionView {
	return transactionView{
		Symbol: instrument.Symbol, Exchange: instrument.Exchange, EventType: string(entry.EventType),
		QuantityChange: string(entry.QuantityChange), CashAmount: string(entry.CashAmount),
		ExecutedAt: entry.ExecutedAt.UTC().Format(time.RFC3339), EffectiveDate: formatDate(entry.EffectiveDate),
		Notes: entry.Notes, Fee: string(entry.Fee),
	}
}

func toPositionViews(positions []domain.Position, instruments map[int]domain.Instrument) []positionView {
	views := make([]positionView, 0, len(positions))
	for _, position := range positions {
		instrument := instruments[position.InstrumentID]
		views = append(views, positionView{
			Symbol: position.Symbol, Exchange: position.Exchange, Name: instrument.Name,
			InstrumentType: instrument.InstrumentType, Quantity: string(position.Quantity),
		})
	}

	return views
}

func toIntegrityViews(errs []domain.IntegrityError) []integrityView {
	views := make([]integrityView, 0, len(errs))
	for _, item := range errs {
		views = append(views, integrityView{
			Symbol: item.Symbol, Exchange: item.Exchange,
			FirstNegativeDate: formatDate(item.FirstNegativeDate), Quantity: string(item.Quantity),
		})
	}

	return views
}

func toPortfolioView(portfolio domain.Portfolio, instruments map[int]domain.Instrument) portfolioView {
	holdings := make([]holdingView, 0, len(portfolio.Holdings))

	for _, holding := range portfolio.Holdings {
		instrument := instruments[holding.InstrumentID]
		holdings = append(holdings, holdingView{
			Symbol: holding.Symbol, Exchange: holding.Exchange, Name: instrument.Name,
			InstrumentType: instrument.InstrumentType, Quantity: string(holding.Quantity),
			ClosePrice: optionalDecimal(holding.ClosePrice), PriceDate: formatOptionalDate(holding.PriceDate),
			MarketValueUSD: optionalDecimal(holding.MarketValueUSD), Allocation: optionalDecimal(holding.Allocation),
			PriceIssue: optionalText(holding.PriceIssue),
			CostUSD:    string(holding.CostUSD), AverageCostUSD: string(holding.AverageCostUSD),
			UnrealizedPnLUSD: optionalDecimal(holding.UnrealizedPnLUSD), UnrealizedPnLPercent: optionalDecimal(holding.UnrealizedPnLPercent),
			RealizedPnLUSD: string(holding.RealizedPnLUSD), DividendsUSD: string(holding.DividendsUSD),
			TaxUSD: string(holding.TaxUSD), FeesUSD: string(holding.FeesUSD), TotalReturnUSD: optionalDecimal(holding.TotalReturnUSD),
		})
	}

	return portfolioView{
		Date: formatDate(portfolio.Date), SecuritiesValueUSD: optionalDecimal(portfolio.SecuritiesValueUSD),
		Complete: portfolio.Complete, Holdings: holdings, IntegrityErrors: toIntegrityViews(portfolio.IntegrityErrors),
		Cash: toCashBalanceView(portfolio.Cash), AccountEquityUSD: optionalDecimal(portfolio.AccountEquityUSD),
		ClosedPositions: toClosedPositionViews(portfolio.ClosedPositions, instruments), PnL: toPnLTotalsView(portfolio.PnL),
	}
}

func toPriceBarViews(bars []domain.PriceBar) []priceBarView {
	views := make([]priceBarView, 0, len(bars))
	for _, bar := range bars {
		views = append(views, priceBarView{
			Date: formatDate(bar.Date), Open: string(bar.Open), High: string(bar.High), Low: string(bar.Low),
			Close: string(bar.Close), AdjustedClose: string(bar.AdjustedClose), Volume: bar.Volume,
		})
	}

	return views
}

// toSnapshotDays groups rows, which arrive ordered by date, into one entry per date.
func toSnapshotDays(rows []domain.SnapshotRow) []snapshotDayView {
	days := []snapshotDayView{}

	for _, row := range rows {
		if len(days) == 0 || days[len(days)-1].Date != formatDate(row.Date) {
			days = append(days, snapshotDayView{
				Date: formatDate(row.Date), SecuritiesValueUSD: string(row.DateTotalUSD), Holdings: []snapshotRowView{},
			})
		}

		day := &days[len(days)-1]
		day.Holdings = append(day.Holdings, snapshotRowView{
			Symbol: row.Symbol, Exchange: row.Exchange, Quantity: string(row.Quantity),
			ClosePrice: string(row.ClosePrice), PriceDate: formatDate(row.PriceDate),
			MarketValueUSD: string(row.MarketValueUSD), Allocation: string(row.Allocation),
		})
	}

	return days
}

func toRefreshView(result marketdata.RefreshResult) refreshView {
	view := refreshView{
		Symbol: result.Instrument.Symbol, Exchange: result.Instrument.Exchange,
		From: formatOptionalDate(result.From), Bars: result.Bars,
	}
	if result.Err != nil {
		view.Error = result.Err.Error()
	}

	return view
}

func toUpdateViews(updates []domain.Update) []updateView {
	views := make([]updateView, 0, len(updates))

	for _, update := range updates {
		view := updateView{
			ID: update.ID, DataType: update.DataType, Status: update.Status, Provider: update.Provider,
			StartedAt: update.StartedAt.UTC().Format(time.RFC3339), RecordsUpdated: update.RecordsUpdated,
			DateFrom: formatOptionalDate(update.DateFrom), DateTo: formatOptionalDate(update.DateTo),
			Reason: optionalText(update.Reason), ErrorMessage: optionalText(update.ErrorMessage),
		}

		if !update.CompletedAt.IsZero() {
			completed := update.CompletedAt.UTC().Format(time.RFC3339)
			view.CompletedAt = &completed
		}

		views = append(views, view)
	}

	return views
}

type cashTransactionView struct {
	Kind          string  `json:"kind"`
	Amount        string  `json:"amount"`
	ExecutedAt    string  `json:"executed_at"`
	EffectiveDate string  `json:"effective_date"`
	Notes         *string `json:"notes"`
}

type cashIntegrityView struct {
	FirstNegativeDate string `json:"first_negative_date"`
	Balance           string `json:"balance"`
}

type cashBalanceView struct {
	Date            string              `json:"date"`
	Initialized     bool                `json:"initialized"`
	OpeningDate     *string             `json:"opening_date"`
	BalanceUSD      *string             `json:"balance_usd"`
	BuyingPowerUSD  *string             `json:"buying_power_usd"`
	IntegrityErrors []cashIntegrityView `json:"integrity_errors"`
}

type closedPositionView struct {
	Symbol         string `json:"symbol"`
	Exchange       string `json:"exchange"`
	Name           string `json:"name"`
	RealizedPnLUSD string `json:"realized_pnl_usd"`
	DividendsUSD   string `json:"dividends_usd"`
	TaxUSD         string `json:"tax_usd"`
	FeesUSD        string `json:"fees_usd"`
	TotalReturnUSD string `json:"total_return_usd"`
}

type pnlTotalsView struct {
	CostUSD          string  `json:"cost_usd"`
	UnrealizedPnLUSD *string `json:"unrealized_pnl_usd"`
	RealizedPnLUSD   string  `json:"realized_pnl_usd"`
	DividendsUSD     string  `json:"dividends_usd"`
	TaxUSD           string  `json:"tax_usd"`
	FeesUSD          string  `json:"fees_usd"`
	TotalReturnUSD   *string `json:"total_return_usd"`
}

type instrumentStatusView struct {
	Symbol          string  `json:"symbol"`
	Exchange        string  `json:"exchange"`
	LatestPriceDate *string `json:"latest_price_date"`
	PriceIssue      *string `json:"price_issue"`
}

type statusView struct {
	Date               string                 `json:"date"`
	Healthy            bool                   `json:"healthy"`
	Reasons            []string               `json:"reasons"`
	Instruments        []instrumentStatusView `json:"instruments"`
	LatestPriceDate    *string                `json:"latest_price_date"`
	LatestSnapshotDate *string                `json:"latest_snapshot_date"`
	SnapshotMissing    bool                   `json:"snapshot_missing"`
	NeedsAttention     []updateView           `json:"needs_attention"`
}

type valuePointView struct {
	Date               string  `json:"date"`
	SecuritiesValueUSD string  `json:"securities_value_usd"`
	CashUSD            *string `json:"cash_usd"`
	AccountEquityUSD   *string `json:"account_equity_usd"`
}

type extremeView struct {
	Date     string `json:"date"`
	ValueUSD string `json:"value_usd"`
}

type drawdownView struct {
	PeakDate        string `json:"peak_date"`
	PeakValueUSD    string `json:"peak_value_usd"`
	TroughDate      string `json:"trough_date"`
	TroughValueUSD  string `json:"trough_value_usd"`
	DrawdownUSD     string `json:"drawdown_usd"`
	DrawdownPercent string `json:"drawdown_percent"`
	Recovered       bool   `json:"recovered"`
}

type seriesSummaryView struct {
	Latest          extremeView  `json:"latest"`
	AllTimeHigh     extremeView  `json:"all_time_high"`
	FromHighUSD     string       `json:"from_high_usd"`
	FromHighPercent string       `json:"from_high_percent"`
	Baseline        extremeView  `json:"baseline"`
	ChangeUSD       string       `json:"change_usd"`
	ChangePercent   *string      `json:"change_percent"`
	RangeHigh       extremeView  `json:"range_high"`
	RangeLow        extremeView  `json:"range_low"`
	MaxDrawdown     drawdownView `json:"max_drawdown"`
}

type valueHistoryView struct {
	From                *string            `json:"from"`
	To                  *string            `json:"to"`
	WindowDays          int                `json:"window_days"`
	Points              []valuePointView   `json:"points"`
	Securities          *seriesSummaryView `json:"securities"`
	Equity              *seriesSummaryView `json:"equity"`
	NetContributionsUSD *string            `json:"net_contributions_usd"`
	Note                string             `json:"note"`
}

const historyNote = "values are changes in portfolio value, not investment returns: a deposit or a purchase raises value without being a gain"

func toCashTransactionView(entry domain.CashTransaction) cashTransactionView {
	return cashTransactionView{
		Kind: string(entry.Kind), Amount: string(entry.Amount),
		ExecutedAt: entry.ExecutedAt.UTC().Format(time.RFC3339), EffectiveDate: formatDate(entry.EffectiveDate),
		Notes: entry.Notes,
	}
}

func toCashBalanceView(balance domain.CashBalance) cashBalanceView {
	integrity := make([]cashIntegrityView, 0, len(balance.IntegrityErrors))
	for _, item := range balance.IntegrityErrors {
		integrity = append(integrity, cashIntegrityView{FirstNegativeDate: formatDate(item.FirstNegativeDate), Balance: string(item.Balance)})
	}

	return cashBalanceView{
		Date: formatDate(balance.Date), Initialized: balance.Initialized, OpeningDate: formatOptionalDate(balance.OpeningDate),
		BalanceUSD: optionalDecimal(balance.BalanceUSD), BuyingPowerUSD: optionalDecimal(balance.BuyingPowerUSD),
		IntegrityErrors: integrity,
	}
}

func toClosedPositionViews(positions []domain.ClosedPosition, instruments map[int]domain.Instrument) []closedPositionView {
	views := make([]closedPositionView, 0, len(positions))
	for _, position := range positions {
		views = append(views, closedPositionView{
			Symbol: position.Symbol, Exchange: position.Exchange, Name: instruments[position.InstrumentID].Name,
			RealizedPnLUSD: string(position.RealizedPnLUSD), DividendsUSD: string(position.DividendsUSD),
			TaxUSD: string(position.TaxUSD), FeesUSD: string(position.FeesUSD), TotalReturnUSD: string(position.TotalReturnUSD),
		})
	}

	return views
}

func toPnLTotalsView(totals domain.PnLTotals) pnlTotalsView {
	return pnlTotalsView{
		CostUSD: string(totals.CostUSD), UnrealizedPnLUSD: optionalDecimal(totals.UnrealizedPnLUSD),
		RealizedPnLUSD: string(totals.RealizedPnLUSD), DividendsUSD: string(totals.DividendsUSD),
		TaxUSD: string(totals.TaxUSD), FeesUSD: string(totals.FeesUSD), TotalReturnUSD: optionalDecimal(totals.TotalReturnUSD),
	}
}

func toStatusView(status domain.Status) statusView {
	instruments := make([]instrumentStatusView, 0, len(status.Instruments))
	for _, instrument := range status.Instruments {
		instruments = append(instruments, instrumentStatusView{
			Symbol: instrument.Symbol, Exchange: instrument.Exchange,
			LatestPriceDate: formatOptionalDate(instrument.LatestPriceDate), PriceIssue: optionalText(instrument.PriceIssue),
		})
	}

	return statusView{
		Date: formatDate(status.Date), Healthy: status.Healthy, Reasons: status.Reasons, Instruments: instruments,
		LatestPriceDate: formatOptionalDate(status.LatestPriceDate), LatestSnapshotDate: formatOptionalDate(status.LatestSnapshotDate),
		SnapshotMissing: status.SnapshotMissing, NeedsAttention: toUpdateViews(status.NeedsAttention),
	}
}

func toValueHistoryView(history domain.ValueHistory) valueHistoryView {
	points := make([]valuePointView, 0, len(history.Points))
	for _, point := range history.Points {
		points = append(points, valuePointView{
			Date: formatDate(point.Date), SecuritiesValueUSD: string(point.SecuritiesValueUSD),
			CashUSD: optionalDecimal(point.CashUSD), AccountEquityUSD: optionalDecimal(point.AccountEquityUSD),
		})
	}

	view := valueHistoryView{
		WindowDays: history.WindowDays, Points: points, Securities: toSeriesSummaryView(history.Securities),
		Equity: toSeriesSummaryView(history.Equity), NetContributionsUSD: optionalDecimal(history.NetContributionsUSD),
		Note: historyNote,
	}

	if !history.From.IsZero() {
		view.From, view.To = formatOptionalDate(&history.From), formatOptionalDate(&history.To)
	}

	return view
}

func toSeriesSummaryView(summary *domain.SeriesSummary) *seriesSummaryView {
	if summary == nil {
		return nil
	}

	drawdown := summary.MaxDrawdown

	return &seriesSummaryView{
		Latest: toExtremeView(summary.Latest), AllTimeHigh: toExtremeView(summary.AllTimeHigh),
		FromHighUSD: string(summary.FromHighUSD), FromHighPercent: string(summary.FromHighPercent),
		Baseline: toExtremeView(summary.Baseline), ChangeUSD: string(summary.ChangeUSD),
		ChangePercent: optionalDecimal(summary.ChangePercent),
		RangeHigh:     toExtremeView(summary.RangeHigh), RangeLow: toExtremeView(summary.RangeLow),
		MaxDrawdown: drawdownView{
			PeakDate: formatDate(drawdown.PeakDate), PeakValueUSD: string(drawdown.PeakValueUSD),
			TroughDate: formatDate(drawdown.TroughDate), TroughValueUSD: string(drawdown.TroughValueUSD),
			DrawdownUSD: string(drawdown.DrawdownUSD), DrawdownPercent: string(drawdown.DrawdownPercent),
			Recovered: drawdown.Recovered,
		},
	}
}

func toExtremeView(extreme domain.Extreme) extremeView {
	return extremeView{Date: formatDate(extreme.Date), ValueUSD: string(extreme.ValueUSD)}
}
