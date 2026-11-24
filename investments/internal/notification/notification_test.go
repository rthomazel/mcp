package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const validConfig = `
version: 1
channels:
  telegram:
    categories:
      portfolio: {chat_id: "1"}
      system: {chat_id: "2"}
  stub: {}
notifications:
  notification.snapshot.incomplete:
    format: plain
    channels:
      telegram: portfolio
      stub:
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "notifications.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoaderReadsAValidFile(t *testing.T) {
	cfg, err := NewConfigLoader(writeConfig(t, validConfig)).Current()

	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Version)
	assert.Equal(t, map[string]string{"chat_id": "2"}, cfg.Channels["telegram"].Categories["system"])
	assert.Equal(t, map[string]map[string]string{"default": {}}, cfg.Channels["stub"].Categories)
	assert.Equal(t, "portfolio", cfg.Notifications["notification.snapshot.incomplete"].Channels["telegram"])
}

func TestChannelWrittenAsInlineSettingsHasOneDefaultCategory(t *testing.T) {
	cfg, err := NewConfigLoader(writeConfig(t, `
version: 1
channels:
  telegram: {chat_id: 123}
  stub:
notifications:
  notification.failed:
    format: plain
    channels:
      telegram:
      stub: default
`)).Current()

	require.NoError(t, err)
	assert.Equal(t, map[string]map[string]string{"default": {"chat_id": "123"}}, cfg.Channels["telegram"].Categories)
	assert.Equal(t, map[string]map[string]string{"default": {}}, cfg.Channels["stub"].Categories)
}

func TestLoaderTreatsMissingFileAsUnconfigured(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing.yaml")} {
		cfg, err := NewConfigLoader(path).Current()
		require.NoError(t, err)
		assert.Empty(t, cfg.Notifications)
	}
}

func TestLoaderKeepsTheLastValidConfiguration(t *testing.T) {
	path := writeConfig(t, validConfig)
	loader := NewConfigLoader(path)

	_, err := loader.Current()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("version: 2"), 0o600))

	cfg, err := loader.Current()
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Version)

	fresh := NewConfigLoader(path)
	_, err = fresh.Current()
	assert.Error(t, err)
}

func TestConfigValidateRejects(t *testing.T) {
	const head = "version: 1\nchannels:\n  telegram:\n    categories: {a: {chat_id: '1'}, b: {chat_id: '2'}}\n  stub: {}\n"

	tests := map[string]string{
		"version":             "version: 2",
		"parse error":         "version: [",
		"unknown channel":     "version: 1\nchannels: {sms: {}}",
		"empty categories":    "version: 1\nchannels: {stub: {categories: {}}}",
		"mixed shape":         "version: 1\nchannels: {stub: {categories: {a: {}}, other: x}}",
		"blank category":      "version: 1\nchannels: {stub: {categories: {' ': {}}}}",
		"telegram no chat":    "version: 1\nchannels: {telegram: {categories: {a: {}}}}",
		"inline telegram":     "version: 1\nchannels: {telegram: {}}",
		"duplicate key":       "version: 1\nchannels: {stub: {}, stub: {}}",
		"unknown event":       head + "notifications: {other.thing: {format: plain, channels: {stub: ''}}}",
		"missing format":      head + "notifications: {notification.snapshot.incomplete: {channels: {stub: ''}}}",
		"other format":        head + "notifications: {notification.snapshot.incomplete: {format: html, channels: {stub: ''}}}",
		"no channels":         head + "notifications: {notification.snapshot.incomplete: {format: plain}}",
		"empty channels":      head + "notifications: {notification.snapshot.incomplete: {format: plain, channels: {}}}",
		"undeclared channel":  head + "notifications: {notification.snapshot.incomplete: {format: plain, channels: {email: ''}}}",
		"undeclared category": head + "notifications: {notification.snapshot.incomplete: {format: plain, channels: {telegram: c}}}",
		"category left out":   head + "notifications: {notification.snapshot.incomplete: {format: plain, channels: {telegram: ''}}}",
	}

	for name, content := range tests {
		_, err := NewConfigLoader(writeConfig(t, content)).Current()
		assert.Error(t, err, name)
	}
}

func TestTelegramSendsPlainTextToTheCategoryChat(t *testing.T) {
	var path string

	var body map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
	}))
	defer server.Close()

	cfg, err := NewConfigLoader(writeConfig(t, validConfig)).Current()
	require.NoError(t, err)

	channel := NewTelegramChannel("telegram", cfg.Channels["telegram"], "secret")
	channel.baseURL = server.URL

	require.NoError(t, channel.SendPlainText(context.Background(), "system", "hello"))
	assert.Equal(t, "/botsecret/sendMessage", path)
	assert.Equal(t, map[string]string{"chat_id": "2", "text": "hello"}, body)

	require.NoError(t, channel.SendPlainText(context.Background(), "system", strings.Repeat("é", 5000)))
	assert.Len(t, []rune(body["text"]), telegramMaxText)
	assert.True(t, strings.HasSuffix(body["text"], "…"))
}

func TestTelegramWithoutCredentialsWarnsAndSucceeds(t *testing.T) {
	cfg, err := NewConfigLoader(writeConfig(t, validConfig)).Current()
	require.NoError(t, err)

	noToken := NewTelegramChannel("telegram", cfg.Channels["telegram"], "")
	assert.NoError(t, noToken.SendPlainText(context.Background(), "system", "x"))

	noChat := NewTelegramChannel("telegram", cfg.Channels["telegram"], "secret")
	assert.NoError(t, noChat.SendPlainText(context.Background(), "unknown", "x"))
}

func TestTelegramFailuresDoNotLeakTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"description":"Too Many Requests"}`))
	}))

	cfg, err := NewConfigLoader(writeConfig(t, validConfig)).Current()
	require.NoError(t, err)

	channel := NewTelegramChannel("telegram", cfg.Channels["telegram"], "secret")
	channel.baseURL = server.URL

	err = channel.SendPlainText(context.Background(), "system", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Contains(t, err.Error(), "Too Many Requests")

	server.Close()

	err = channel.SendPlainText(context.Background(), "system", "x")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
}

type fakeUpdates struct {
	domain.UpdateStore

	updates []domain.Update
	err     error
}

func (s *fakeUpdates) List(context.Context, domain.UpdateFilter) ([]domain.Update, error) {
	return s.updates, s.err
}

func TestRenderers(t *testing.T) {
	day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	other := day.AddDate(0, 0, -1)
	data, err := json.Marshal(domain.SnapshotIncompleteData{Date: day})
	require.NoError(t, err)

	event := domain.Event{Type: domain.Events.SnapshotIncomplete, Data: data}
	renderers := NewRenderers(&fakeUpdates{updates: []domain.Update{
		{DateFrom: &other, DateTo: &other, ErrorMessage: "wrong day"},
		{DateFrom: &day, DateTo: &day, ErrorMessage: "VOO missing price"},
	}})

	text, err := renderers[domain.Events.SnapshotIncomplete][defaultFormat].RenderPlainText(context.Background(), event)
	require.NoError(t, err)
	assert.Contains(t, text, "2026-01-05")
	assert.Contains(t, text, "VOO missing price")
	assert.NotContains(t, text, "wrong day")

	empty := NewRenderers(&fakeUpdates{})
	text, err = empty[domain.Events.SnapshotIncomplete][defaultFormat].RenderPlainText(context.Background(), event)
	require.NoError(t, err)
	assert.Contains(t, text, "no run was recorded")

	failing := NewRenderers(&fakeUpdates{err: errors.New("db")})
	_, err = failing[domain.Events.SnapshotIncomplete][defaultFormat].RenderPlainText(context.Background(), event)
	assert.Error(t, err)

	_, err = empty[domain.Events.SnapshotIncomplete][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)

	failed, err := json.Marshal(domain.NotificationFailedData{EventID: 1, EventType: "notification.snapshot.incomplete", RecordedAt: day, Failures: 10})
	require.NoError(t, err)

	text, err = renderers[domain.Events.NotificationFailed][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: failed})
	require.NoError(t, err)
	assert.Contains(t, text, "notification.snapshot.incomplete")
	assert.Contains(t, text, "10")
}

func TestSourceExpandsChannelsAndFallsBackToNoChannel(t *testing.T) {
	source := NewSource(NewConfigLoader(writeConfig(t, validConfig)), &fakeUpdates{}, "")

	handlers, err := source.Handlers(context.Background())
	require.NoError(t, err)

	names := func(eventType string) []string {
		var out []string
		for _, handler := range handlers(eventType) {
			out = append(out, handler.Name())
		}

		return out
	}

	assert.Equal(t, []string{"notification.stub.default", "notification.telegram.portfolio"}, names(domain.Events.SnapshotIncomplete))
	assert.Equal(t, []string{"notification.none"}, names(domain.Events.NotificationFailed))

	unconfigured := NewSource(NewConfigLoader(""), &fakeUpdates{}, "")
	handlers, err = unconfigured.Handlers(context.Background())
	require.NoError(t, err)
	assert.NoError(t, handlers(domain.Events.SnapshotIncomplete)[0].Handle(context.Background(), domain.Event{}))
	assert.Nil(t, handlers("price.moved"), "an event that is not a notification has no handler here")
}

func TestSourceFailsOnInvalidConfigurationWithoutAFallback(t *testing.T) {
	source := NewSource(NewConfigLoader(writeConfig(t, "version: 9")), &fakeUpdates{}, "")

	_, err := source.Handlers(context.Background())
	assert.Error(t, err)
}

type recordingChannel struct {
	category, text string
	err            error
}

func (c *recordingChannel) SendPlainText(_ context.Context, category, text string) error {
	c.category, c.text = category, text

	return c.err
}

func TestChannelHandlerRendersThenSends(t *testing.T) {
	data, err := json.Marshal(domain.SnapshotIncompleteData{Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)})
	require.NoError(t, err)

	channel := &recordingChannel{}
	handler := &channelHandler{
		name: "notification.x.y", channel: channel, category: "y",
		renderer: NewRenderers(&fakeUpdates{})[domain.Events.SnapshotIncomplete][defaultFormat],
	}

	require.NoError(t, handler.Handle(context.Background(), domain.Event{Type: domain.Events.SnapshotIncomplete, Data: data}))
	assert.Equal(t, "y", channel.category)
	assert.Contains(t, channel.text, "2026-01-05")

	channel.err = errors.New("down")
	assert.Error(t, handler.Handle(context.Background(), domain.Event{Type: domain.Events.SnapshotIncomplete, Data: data}))
	assert.Equal(t, "notification.x.y", handler.Name())
}

func TestPortfolioDrawdownRenderer(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	data, err := json.Marshal(domain.PortfolioDrawdownData{
		Level: "-6", CrossedOn: day.AddDate(0, 0, -2), Date: day, DrawdownPercent: "-7.2",
		ValueUSD: "92800", HighDate: day.AddDate(0, 0, -19), HighValueUSD: "100000",
	})
	require.NoError(t, err)

	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PortfolioDrawdown][defaultFormat]

	text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "down 7.2%")
	assert.Contains(t, text, "-6% level on 2026-11-18")
	assert.Contains(t, text, "$92800.")
	assert.NotContains(t, text, "$92800.00")
	assert.Contains(t, text, "2026-11-01: $100000.")
	assert.Contains(t, text, "This value change is affected by sales and withdrawals.")

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)
}

func TestPortfolioATHRenderer(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	last := day.AddDate(0, 0, -16)
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PortfolioATH][defaultFormat]

	data, err := json.Marshal(domain.PortfolioATHData{
		Date: day, ValueUSD: "110000", PreviousHighDate: day.AddDate(0, 0, -3), PreviousHighValueUSD: "108000",
		IncreaseUSD: "2000", IncreasePercent: "1.851851", LastAnnouncedDate: &last,
	})
	require.NoError(t, err)

	text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "new all-time high on 2026-11-20: $110000.00")
	assert.Contains(t, text, "$2000.00 (1.9%) above the previous high of $108000.00 on 2026-11-17")
	assert.Contains(t, text, "Last announced high: 2026-11-04.")
	assert.Contains(t, text, "This value change is affected by sales and withdrawals.")

	data, err = json.Marshal(domain.PortfolioATHData{
		Date: day, ValueUSD: "2", PreviousHighValueUSD: "1", IncreaseUSD: "1", IncreasePercent: "1",
	})
	require.NoError(t, err)

	text, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "Last announced high: none.")

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)
}

func TestPortfolioMovementRenderer(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PortfolioMovement][defaultFormat]

	data, err := json.Marshal(domain.PortfolioMovementData{
		Date: day, Direction: "down", Window: "3d", StartDate: day.AddDate(0, 0, -3), StartValueUSD: "100000",
		EndValueUSD: "96000", MarketUSD: "-3456", MarketPercent: "-3.456", TradesUSD: "-544",
		Gainers: []domain.Contributor{{Symbol: "GLD", MovementUSD: "120"}},
		Losers:  []domain.Contributor{{Symbol: "QQQ", MovementUSD: "-2000"}, {Symbol: "VOO", MovementUSD: "-1576"}},
	})
	require.NoError(t, err)

	text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "fell 3.5% (-$3456.00) over 3d, from 2026-11-17 to 2026-11-20")
	assert.Contains(t, text, "from $100000.00 to $96000.00: market -$3456.00, trades -$544.00")
	assert.Contains(t, text, "Largest gains: GLD +$120.00.")
	assert.Contains(t, text, "Largest losses: QQQ -$2000.00, VOO -$1576.00.")
	assert.Contains(t, text, "This value change is affected by sales and withdrawals.")

	data, err = json.Marshal(domain.PortfolioMovementData{
		Date: day, Direction: "up", Window: "1d", StartValueUSD: "100", EndValueUSD: "110", MarketUSD: "10", MarketPercent: "0.5", TradesUSD: "0",
	})
	require.NoError(t, err)

	text, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "rose 0.5% (+$10.00)")
	assert.NotContains(t, text, "Largest")

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)
}

func TestRenderersRoundDollarsToCents(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	renderers := NewRenderers(&fakeUpdates{})

	ath, err := json.Marshal(domain.PortfolioATHData{
		Date: day, ValueUSD: "1296.29535", PreviousHighValueUSD: "1295.004", IncreaseUSD: "1.29135", IncreasePercent: "0.0997",
	})
	require.NoError(t, err)

	text, err := renderers[domain.Events.PortfolioATH][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: ath})
	require.NoError(t, err)
	assert.Contains(t, text, "$1296.30.")
	assert.Contains(t, text, "$1.29 (0.1%) above the previous high of $1295.00")

	movement, err := json.Marshal(domain.PortfolioMovementData{
		Date: day, Direction: "up", Window: "1d", StartValueUSD: "1000.004", EndValueUSD: "1034.5", MarketUSD: "0.004",
		MarketPercent: "0.0004", TradesUSD: "-0.006", Gainers: []domain.Contributor{{Symbol: "VOO", MovementUSD: "34.555"}},
	})
	require.NoError(t, err)

	text, err = renderers[domain.Events.PortfolioMovement][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: movement})
	require.NoError(t, err)
	assert.Contains(t, text, "rose 0.0% (+$0.00)", "a sub-cent amount is not negative or signed zero")
	assert.Contains(t, text, "from $1000.00 to $1034.50: market +$0.00, trades -$0.01")
	assert.Contains(t, text, "VOO +$34.56.", "halves round away from zero")

	_, err = renderers[domain.Events.PortfolioMovement][defaultFormat].RenderPlainText(context.Background(),
		domain.Event{Data: []byte(`{"market_percent":"1","market_usd":"abc"}`)})
	assert.Error(t, err)
}

func TestPortfolioDrawdownRendererRounding(t *testing.T) {
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PortfolioDrawdown][defaultFormat]

	for percent, want := range map[string]string{
		"-7.2": "down 7.2%", "-7.24": "down 7.2%", "-7.25": "down 7.3%", "-7.2999999": "down 7.3%",
		"-7": "down 7.0%", "-12.3456789": "down 12.3%", "-99.96": "down 100.0%",
	} {
		data, err := json.Marshal(domain.PortfolioDrawdownData{
			Level: "-3", DrawdownPercent: domain.Decimal(percent), ValueUSD: "90", HighValueUSD: "100",
		})
		require.NoError(t, err)

		text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
		require.NoError(t, err)
		assert.Contains(t, text, want, percent)
	}

	data, err := json.Marshal(domain.PortfolioDrawdownData{Level: "-3", DrawdownPercent: "abc"})
	require.NoError(t, err)

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	assert.Error(t, err)
}

func TestAllocationDriftRenderer(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.AllocationDrift][defaultFormat]
	maximum, minimum := domain.Decimal("15"), domain.Decimal("5")

	data, err := json.Marshal(domain.AllocationDriftData{
		Group: "us-core", Side: "above", Date: day, StayStart: day.AddDate(0, 0, -3), AllocationPercent: "17.456789",
		MaxPercent: &maximum, Members: []domain.MemberAllocation{{Symbol: "VOO", AllocationPercent: "10.04"}, {Symbol: "VTI", AllocationPercent: "7.416789"}},
	})
	require.NoError(t, err)

	text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "Allocation of us-core is above its maximum: 17.5% of the securities value against 15.0%")
	assert.Contains(t, text, "since 2026-11-17 (as of 2026-11-20)")
	assert.Contains(t, text, "Instruments: VOO 10.0%, VTI 7.4%.")
	assert.Contains(t, text, "trades as well as with prices")

	data, err = json.Marshal(domain.AllocationDriftData{Group: "gold", Side: "below", Date: day, StayStart: day, AllocationPercent: "4.96", MinPercent: &minimum})
	require.NoError(t, err)

	text, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "is below its minimum: 4.96% of the securities value against 5.00%",
		"a rounding that would show the bound reached keeps two decimals")

	data, err = json.Marshal(domain.AllocationDriftData{Group: "gold", Side: "below", AllocationPercent: "4"})
	require.NoError(t, err)

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	assert.Error(t, err, "a side without its bound")

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)
}

func TestPriceVolatilityRenderer(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PriceVolatility][defaultFormat]

	data, err := json.Marshal(domain.PriceVolatilityData{
		Instrument: "QQQ", Date: day, Direction: "down", Window: "5d", Lookback: "3mo", StartDate: day.AddDate(0, 0, -5),
		MovePercent: "-4.3456789", Variation: "-2.449", Threshold: "2", VolatilityPercent: "1.7742", Moves: 61,
	})
	require.NoError(t, err)

	text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "QQQ fell 4.3% over 5d, from 2026-11-15 to 2026-11-20: 2.4 standard deviations against a threshold of 2.0.")
	assert.Contains(t, text, "Its usual move over 5d is 1.8%, the standard deviation of 61 moves in the last 3mo.")
	assert.Contains(t, text, "adjusted closes")

	data, err = json.Marshal(domain.PriceVolatilityData{Instrument: "QQQ", Direction: "up", MovePercent: "x"})
	require.NoError(t, err)

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	assert.Error(t, err)

	_, err = renderer.RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)
}

func TestPortfolioDrawdownRendererWholeDollars(t *testing.T) {
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PortfolioDrawdown][defaultFormat]

	data, err := json.Marshal(domain.PortfolioDrawdownData{
		Level: "-3", DrawdownPercent: "-3.5", ValueUSD: "96499.5", HighValueUSD: "100000.49",
	})
	require.NoError(t, err)

	text, err := renderer.RenderPlainText(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "$96500.")
	assert.Contains(t, text, "$100000.")
	assert.NotContains(t, text, "96499")
	assert.NotContains(t, text, "100000.49")
}

func TestInstrumentRenderers(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	last := day.AddDate(0, 0, -20)
	renderers := NewRenderers(&fakeUpdates{})

	target, err := json.Marshal(domain.PriceTargetData{
		Instrument: "QQQ", Price: "232", Date: day, Low: "230.004", High: "236.995", Close: "234.5",
	})
	require.NoError(t, err)

	text, err := renderers[domain.Events.PriceTarget][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: target})
	require.NoError(t, err)
	assert.Contains(t, text, "QQQ reached its target price of $232.00 on 2026-11-20")
	assert.Contains(t, text, "Low $230.00, high $237.00, close $234.50")

	drawdown, err := json.Marshal(domain.InstrumentDrawdownData{
		Instrument: "QQQ", Level: "-5", CrossedOn: day.AddDate(0, 0, -2), Date: day, DrawdownPercent: "-7.2499",
		Close: "400.125", HighDate: day.AddDate(0, 0, -30), HighClose: "431.239",
	})
	require.NoError(t, err)

	text, err = renderers[domain.Events.InstrumentDrawdown][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: drawdown})
	require.NoError(t, err)
	assert.Contains(t, text, "QQQ is down 7.2% from its high, crossing the -5% level on 2026-11-18")
	assert.Contains(t, text, "$400.13")
	assert.Contains(t, text, "2026-10-21: $431.24")

	high, err := json.Marshal(domain.InstrumentATHData{
		Instrument: "QQQ", Date: day, Close: "450.999", PreviousHighDate: day.AddDate(0, 0, -3), PreviousHigh: "447.001",
		IncreasePercent: "0.8948", LastAnnouncedDate: &last,
	})
	require.NoError(t, err)

	text, err = renderers[domain.Events.InstrumentATH][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: high})
	require.NoError(t, err)
	assert.Contains(t, text, "QQQ reached a new all-time high on 2026-11-20: $451.00")
	assert.Contains(t, text, "0.9% above the previous high of $447.00 on 2026-11-17")
	assert.Contains(t, text, "Last announced high: 2026-10-31")

	for eventType := range map[string]bool{domain.Events.PriceTarget: true, domain.Events.InstrumentDrawdown: true, domain.Events.InstrumentATH: true} {
		_, err = renderers[eventType][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: []byte("[")})
		assert.Error(t, err, eventType)
	}

	bad, err := json.Marshal(domain.InstrumentDrawdownData{Instrument: "QQQ", DrawdownPercent: "abc"})
	require.NoError(t, err)

	_, err = renderers[domain.Events.InstrumentDrawdown][defaultFormat].RenderPlainText(context.Background(), domain.Event{Data: bad})
	assert.Error(t, err)
}
