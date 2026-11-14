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
}

type portfolioView struct {
	Date               string          `json:"date"`
	SecuritiesValueUSD *string         `json:"securities_value_usd"`
	Complete           bool            `json:"complete"`
	Holdings           []holdingView   `json:"holdings"`
	IntegrityErrors    []integrityView `json:"integrity_errors"`
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
	From  string `json:"from"`
	To    string `json:"to"`
	Dates int    `json:"dates"`
	Rows  int    `json:"rows"`
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
		Notes: entry.Notes,
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
		})
	}

	return portfolioView{
		Date: formatDate(portfolio.Date), SecuritiesValueUSD: optionalDecimal(portfolio.SecuritiesValueUSD),
		Complete: portfolio.Complete, Holdings: holdings, IntegrityErrors: toIntegrityViews(portfolio.IntegrityErrors),
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
