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

	text, err := renderers[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), event)
	require.NoError(t, err)
	assert.Contains(t, text, "2026-01-05")
	assert.Contains(t, text, "VOO missing price")
	assert.NotContains(t, text, "wrong day")

	empty := NewRenderers(&fakeUpdates{})
	text, err = empty[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), event)
	require.NoError(t, err)
	assert.Contains(t, text, "no run was recorded")

	failing := NewRenderers(&fakeUpdates{err: errors.New("db")})
	_, err = failing[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), event)
	assert.Error(t, err)

	_, err = empty[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)

	failed, err := json.Marshal(domain.NotificationFailedData{EventID: 1, EventType: "notification.snapshot.incomplete", RecordedAt: day, Failures: 10})
	require.NoError(t, err)

	text, err = renderers[domain.Events.NotificationFailed][defaultFormat].Render(context.Background(), domain.Event{Data: failed})
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

	text, err := renderer.Render(context.Background(), domain.Event{Data: data})
	require.NoError(t, err)
	assert.Contains(t, text, "down 7.2%")
	assert.Contains(t, text, "-6% level on 2026-11-18")
	assert.Contains(t, text, "$92800")
	assert.Contains(t, text, "2026-11-01: $100000")
	assert.Contains(t, text, "This value change is affected by sales and withdrawals.")

	_, err = renderer.Render(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)
}

func TestPortfolioDrawdownRendererRounding(t *testing.T) {
	renderer := NewRenderers(&fakeUpdates{})[domain.Events.PortfolioDrawdown][defaultFormat]

	for percent, want := range map[string]string{
		"-7.2": "down 7.2%", "-7.24": "down 7.2%", "-7.25": "down 7.3%", "-7.2999999": "down 7.3%",
		"-7": "down 7.0%", "-12.3456789": "down 12.3%", "-99.96": "down 100.0%",
	} {
		data, err := json.Marshal(domain.PortfolioDrawdownData{Level: "-3", DrawdownPercent: domain.Decimal(percent)})
		require.NoError(t, err)

		text, err := renderer.Render(context.Background(), domain.Event{Data: data})
		require.NoError(t, err)
		assert.Contains(t, text, want, percent)
	}

	data, err := json.Marshal(domain.PortfolioDrawdownData{Level: "-3", DrawdownPercent: "abc"})
	require.NoError(t, err)

	_, err = renderer.Render(context.Background(), domain.Event{Data: data})
	assert.Error(t, err)
}
