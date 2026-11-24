package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDecimal(t *testing.T) {
	for _, ok := range []string{"0", "10", "-5000", "7.50", " 0.1 ", "-0.25"} {
		got, err := ParseDecimal(ok)
		require.NoError(t, err, ok)
		assert.Equal(t, strings.TrimSpace(ok), string(got))
	}

	for _, bad := range []string{"", "-", "1.", ".5", "1e3", "1,000", "1/2", "abc", "--1", "+1"} {
		_, err := ParseDecimal(bad)
		assert.ErrorIs(t, err, ErrInvalidInput, bad)
	}
}

func TestDecimalSign(t *testing.T) {
	assert.Equal(t, 0, Decimal("0").Sign())
	assert.Equal(t, 0, Decimal("0.000").Sign())
	assert.Equal(t, 0, Decimal("-0").Sign())
	assert.Equal(t, 1, Decimal("0.001").Sign())
	assert.Equal(t, 1, Decimal("10").Sign())
	assert.Equal(t, -1, Decimal("-0.5").Sign())
}

func TestInstrumentNormalizeValidate(t *testing.T) {
	got := Instrument{Symbol: " voo ", Exchange: "nyse arca", Name: " Vanguard S&P 500 ", Currency: "usd", InstrumentType: "etf"}.Normalize()
	assert.Equal(t, Instrument{Symbol: "VOO", Exchange: "NYSE ARCA", Name: "Vanguard S&P 500", Currency: "USD", InstrumentType: "ETF"}, got)
	require.NoError(t, got.Validate())

	for name, mutate := range map[string]func(*Instrument){
		"empty symbol":   func(i *Instrument) { i.Symbol = "" },
		"empty exchange": func(i *Instrument) { i.Exchange = "" },
		"empty name":     func(i *Instrument) { i.Name = "" },
		"non-usd":        func(i *Instrument) { i.Currency = "BRL" },
		"unknown type":   func(i *Instrument) { i.InstrumentType = "BOND" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := got
			mutate(&bad)
			assert.ErrorIs(t, bad.Validate(), ErrInvalidInput)
		})
	}
}

func TestDraftTransactionValidate(t *testing.T) {
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	draft := func(event EventType, quantity, cash string) DraftTransaction {
		return DraftTransaction{
			InstrumentID: 1, EventType: event, QuantityChange: Decimal(quantity), CashAmount: Decimal(cash),
			ExecutedAt: when, EffectiveDate: when.Truncate(24 * time.Hour),
		}
	}

	valid := []DraftTransaction{
		draft(EventBuy, "10", "-5000"),
		draft(EventSell, "-3", "1560"),
		draft(EventDividend, "0", "50"),
		draft(EventTax, "0", "-7.50"),
		draft(EventSplit, "100", ""),
		draft(EventSplit, "-50", "0"),
		draft(EventAdjustment, "0.1", "-50"),
		draft(EventAdjustment, "0.1", ""),
		draft(EventAdjustment, "-1", "5"),
	}
	for _, d := range valid {
		assert.NoError(t, d.Validate(), "%s %s %s", d.EventType, d.QuantityChange, d.CashAmount)
	}

	invalid := map[string]DraftTransaction{
		"unknown event":        draft("TRANSFER", "1", "1"),
		"buy zero quantity":    draft(EventBuy, "0", "-5"),
		"buy negative qty":     draft(EventBuy, "-1", "-5"),
		"buy positive cash":    draft(EventBuy, "1", "5"),
		"buy empty cash":       draft(EventBuy, "1", ""),
		"sell positive qty":    draft(EventSell, "1", "5"),
		"sell negative cash":   draft(EventSell, "-1", "-5"),
		"dividend quantity":    draft(EventDividend, "1", "5"),
		"dividend negative":    draft(EventDividend, "0", "-5"),
		"tax positive":         draft(EventTax, "0", "5"),
		"tax quantity":         draft(EventTax, "1", "-5"),
		"split zero qty":       draft(EventSplit, "0", ""),
		"split with cash":      draft(EventSplit, "2", "5"),
		"adjustment zero qty":  draft(EventAdjustment, "0", "5"),
		"quantity not decimal": draft(EventBuy, "ten", "-5"),
		"cash not decimal":     draft(EventBuy, "1", "five"),
		"empty quantity":       draft(EventBuy, "", "-5"),
	}
	for name, d := range invalid {
		assert.ErrorIs(t, d.Validate(), ErrInvalidInput, name)
	}

	noExecuted := draft(EventBuy, "1", "-1")
	noExecuted.ExecutedAt = time.Time{}
	assert.ErrorIs(t, noExecuted.Validate(), ErrInvalidInput)

	noEffective := draft(EventBuy, "1", "-1")
	noEffective.EffectiveDate = time.Time{}
	assert.ErrorIs(t, noEffective.Validate(), ErrInvalidInput)

	long := strings.Repeat("é", maxNotesLength+1)
	tooLong := draft(EventBuy, "1", "-1")
	tooLong.Notes = &long
	assert.ErrorIs(t, tooLong.Validate(), ErrInvalidInput)

	atLimit := strings.Repeat("é", maxNotesLength)
	ok := draft(EventBuy, "1", "-1")
	ok.Notes = &atLimit
	assert.NoError(t, ok.Validate())
}

func TestTransactionFilterNormalize(t *testing.T) {
	got, err := TransactionFilter{}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, defaultListLimit, got.Limit)

	got, err = TransactionFilter{Limit: 10_000}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, maxListLimit, got.Limit)

	got, err = TransactionFilter{Limit: 7}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, 7, got.Limit)

	_, err = TransactionFilter{Limit: -1}.Normalize()
	assert.ErrorIs(t, err, ErrInvalidInput)

	from, to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = TransactionFilter{From: &from, To: &to}.Normalize()
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestNegativeBalanceError(t *testing.T) {
	var err error = &NegativeBalanceError{InstrumentID: 3, OnDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Quantity: "-2"}
	assert.ErrorIs(t, err, ErrNegativeBalance)
	assert.Contains(t, err.Error(), "2026-01-05")
}

func TestPriceBarValidate(t *testing.T) {
	good := PriceBar{
		Date: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
		Open: "698.66", High: "698.81", Low: "687.5", Close: "693.38", AdjustedClose: "693.38", Volume: 5521455,
	}
	require.NoError(t, good.Validate())

	fund := good
	fund.Volume = 0
	require.NoError(t, fund.Validate(), "mutual funds report zero volume")

	for name, mutate := range map[string]func(*PriceBar){
		"zero date":       func(b *PriceBar) { b.Date = time.Time{} },
		"bad open":        func(b *PriceBar) { b.Open = "x" },
		"bad high":        func(b *PriceBar) { b.High = "" },
		"bad low":         func(b *PriceBar) { b.Low = "1e3" },
		"bad close":       func(b *PriceBar) { b.Close = "-" },
		"bad adjusted":    func(b *PriceBar) { b.AdjustedClose = "n/a" },
		"zero close":      func(b *PriceBar) { b.Close = "0.00" },
		"negative close":  func(b *PriceBar) { b.Close = "-5" },
		"negative volume": func(b *PriceBar) { b.Volume = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := good
			mutate(&bad)
			assert.ErrorIs(t, bad.Validate(), ErrInvalidInput)
		})
	}
}

func TestCashTransactionValidate(t *testing.T) {
	at := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	entry := func(kind CashKind, amount string) CashTransaction {
		return CashTransaction{Kind: kind, Amount: Decimal(amount), ExecutedAt: at, EffectiveDate: at}
	}

	tests := map[string]struct {
		entry CashTransaction
		valid bool
	}{
		"opening positive":      {entry(CashOpeningBalance, "10"), true},
		"opening negative":      {entry(CashOpeningBalance, "-10"), true},
		"opening zero":          {entry(CashOpeningBalance, "0"), true},
		"deposit positive":      {entry(CashDeposit, "10"), true},
		"deposit zero":          {entry(CashDeposit, "0"), false},
		"deposit negative":      {entry(CashDeposit, "-1"), false},
		"interest negative":     {entry(CashInterest, "-1"), false},
		"withdrawal negative":   {entry(CashWithdrawal, "-1"), true},
		"withdrawal positive":   {entry(CashWithdrawal, "1"), false},
		"fee positive":          {entry(CashFee, "1"), false},
		"unknown kind":          {entry("BONUS", "1"), false},
		"amount not a decimal":  {entry(CashDeposit, "ten"), false},
		"missing effective":     {CashTransaction{Kind: CashDeposit, Amount: "1", ExecutedAt: at}, false},
		"notes over the limit":  {CashTransaction{Kind: CashDeposit, Amount: "1", ExecutedAt: at, EffectiveDate: at, Notes: ptr(strings.Repeat("x", 1001))}, false},
		"notes at the limit ok": {CashTransaction{Kind: CashDeposit, Amount: "1", ExecutedAt: at, EffectiveDate: at, Notes: ptr(strings.Repeat("x", 1000))}, true},
	}

	for name, test := range tests {
		err := test.entry.Validate()
		if test.valid {
			assert.NoError(t, err, name)
		} else {
			assert.ErrorIs(t, err, ErrInvalidInput, name)
		}
	}
}

func TestCashTransactionFilterNormalize(t *testing.T) {
	early := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	late := early.AddDate(0, 0, 1)

	filter, err := CashTransactionFilter{Limit: 9999}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, maxListLimit, filter.Limit)

	filter, err = CashTransactionFilter{}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, defaultListLimit, filter.Limit)

	for name, bad := range map[string]CashTransactionFilter{
		"negative limit": {Limit: -1}, "inverted range": {From: &late, To: &early}, "unknown kind": {Kind: "BONUS"},
	} {
		_, err := bad.Normalize()
		assert.ErrorIs(t, err, ErrInvalidInput, name)
	}
}

func TestDraftTransactionFeeValidation(t *testing.T) {
	at := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	draft := func(event EventType, quantity, cash, fee string) DraftTransaction {
		return DraftTransaction{
			InstrumentID: 1, EventType: event, QuantityChange: Decimal(quantity), CashAmount: Decimal(cash),
			Fee: Decimal(fee), ExecutedAt: at, EffectiveDate: at,
		}
	}

	assert.NoError(t, draft(EventBuy, "1", "-10", "1").Validate())
	assert.NoError(t, draft(EventSell, "-1", "10", "0").Validate(), "a zero fee is no fee")
	assert.NoError(t, draft(EventDividend, "0", "10", "").Validate())
	assert.ErrorIs(t, draft(EventBuy, "1", "-10", "-1").Validate(), ErrInvalidInput)
	assert.ErrorIs(t, draft(EventBuy, "1", "-10", "abc").Validate(), ErrInvalidInput)
	assert.ErrorIs(t, draft(EventDividend, "0", "10", "1").Validate(), ErrInvalidInput, "fees only on BUY and SELL")
}

func TestStatusSummarize(t *testing.T) {
	latest := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)

	healthy := Status{LatestPriceDate: &latest, Instruments: []InstrumentStatus{{Symbol: "VTI"}}}
	healthy.Summarize()
	assert.True(t, healthy.Healthy)
	assert.Empty(t, healthy.Reasons)

	unhealthy := Status{
		LatestPriceDate: &latest, SnapshotMissing: true,
		Instruments: []InstrumentStatus{
			{Symbol: "VTI", PriceIssue: PriceIssueStale, LatestPriceDate: &latest},
			{Symbol: "BND", PriceIssue: PriceIssueMissing},
		},
		NeedsAttention: []Update{{ID: 1}, {ID: 2}},
	}
	unhealthy.Summarize()

	assert.False(t, unhealthy.Healthy)
	assert.Equal(t, []string{
		"VTI price stale (latest 2026-11-03)",
		"BND price missing",
		"no completed snapshot for 2026-11-03",
		"2 failed or incomplete runs not yet superseded",
	}, unhealthy.Reasons)
}

func TestHistoryFilterNormalize(t *testing.T) {
	early := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	late := early.AddDate(0, 0, 1)

	filter, err := HistoryFilter{}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, 1, filter.WindowDays)

	filter, err = HistoryFilter{WindowDays: 7}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, 7, filter.WindowDays)

	_, err = HistoryFilter{WindowDays: -1}.Normalize()
	assert.ErrorIs(t, err, ErrInvalidInput)

	_, err = HistoryFilter{From: &late, To: &early}.Normalize()
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func ptr(value string) *string { return &value }

func TestDecimalCompare(t *testing.T) {
	for _, tc := range []struct {
		left, right Decimal
		want        int
	}{
		{"1", "2", -1}, {"2", "1", 1}, {"1.0", "1", 0}, {"10", "9.99", 1}, {"-1", "1", -1}, {"0.10", "0.1", 0}, {"100", "100.5", -1},
	} {
		assert.Equal(t, tc.want, tc.left.Compare(tc.right), "%s against %s", tc.left, tc.right)
	}
}
