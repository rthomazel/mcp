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
  - name: telegram
    implementation: telegram
    categories:
      - {name: portfolio, settings: {chat_id: "1"}}
      - {name: system, settings: {chat_id: "2"}}
routes:
  - event_type: snapshot.incomplete
    targets:
      - {channel: telegram, category: system}
      - {channel: telegram, category: portfolio}
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "notifications.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoaderReadsAValidFile(t *testing.T) {
	cfg, err := NewLoader(writeConfig(t, validConfig)).Current()

	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Version)
	assert.Len(t, cfg.Channels[0].Categories, 2)
	assert.Len(t, cfg.Routes[0].Targets, 2)
}

func TestLoaderTreatsMissingFileAsUnconfigured(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing.yaml")} {
		cfg, err := NewLoader(path).Current()
		require.NoError(t, err)
		assert.Empty(t, cfg.Routes)
	}
}

func TestLoaderKeepsTheLastValidConfiguration(t *testing.T) {
	path := writeConfig(t, validConfig)
	loader := NewLoader(path)

	_, err := loader.Current()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("version: 2"), 0o600))

	cfg, err := loader.Current()
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Version)

	fresh := NewLoader(path)
	_, err = fresh.Current()
	assert.Error(t, err)
}

func TestConfigValidateRejects(t *testing.T) {
	tests := map[string]string{
		"version":          "version: 2",
		"parse error":      "version: [",
		"implementation":   "version: 1\nchannels: [{name: a, implementation: sms}]",
		"duplicate":        "version: 1\nchannels: [{name: a, implementation: stub}, {name: a, implementation: stub}]",
		"duplicate cat":    "version: 1\nchannels: [{name: a, implementation: stub, categories: [{name: c}, {name: c}]}]",
		"unknown event":    "version: 1\nroutes: [{event_type: other.thing}]",
		"routed twice":     "version: 1\nroutes: [{event_type: snapshot.incomplete}, {event_type: snapshot.incomplete}]",
		"format":           "version: 1\nroutes: [{event_type: snapshot.incomplete, format: html}]",
		"undeclared":       "version: 1\nroutes: [{event_type: snapshot.incomplete, targets: [{channel: a, category: b}]}]",
		"undeclared categ": "version: 1\nchannels: [{name: a, implementation: stub}]\nroutes: [{event_type: snapshot.incomplete, targets: [{channel: a, category: b}]}]",
	}

	for name, content := range tests {
		_, err := NewLoader(writeConfig(t, content)).Current()
		assert.Error(t, err, name)
	}
}

func TestConfigAllowsARouteWithNoTargets(t *testing.T) {
	_, err := NewLoader(writeConfig(t, "version: 1\nroutes: [{event_type: snapshot.incomplete}]")).Current()

	assert.NoError(t, err)
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

	cfg, err := NewLoader(writeConfig(t, validConfig)).Current()
	require.NoError(t, err)

	channel := NewTelegramChannel(cfg.Channels[0], "secret")
	channel.baseURL = server.URL

	require.NoError(t, channel.SendPlainText(context.Background(), "system", "hello"))
	assert.Equal(t, "/botsecret/sendMessage", path)
	assert.Equal(t, map[string]string{"chat_id": "2", "text": "hello"}, body)

	require.NoError(t, channel.SendPlainText(context.Background(), "system", strings.Repeat("é", 5000)))
	assert.Len(t, []rune(body["text"]), telegramMaxText)
	assert.True(t, strings.HasSuffix(body["text"], "…"))
}

func TestTelegramWithoutCredentialsWarnsAndSucceeds(t *testing.T) {
	cfg, err := NewLoader(writeConfig(t, validConfig)).Current()
	require.NoError(t, err)

	noToken := NewTelegramChannel(cfg.Channels[0], "")
	assert.NoError(t, noToken.SendPlainText(context.Background(), "system", "x"))

	noChat := NewTelegramChannel(cfg.Channels[0], "secret")
	assert.NoError(t, noChat.SendPlainText(context.Background(), "unknown", "x"))
}

func TestTelegramFailuresDoNotLeakTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"description":"Too Many Requests"}`))
	}))

	cfg, err := NewLoader(writeConfig(t, validConfig)).Current()
	require.NoError(t, err)

	channel := NewTelegramChannel(cfg.Channels[0], "secret")
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

type fakeStore struct {
	domain.Store

	updates []domain.Update
	err     error
}

func (s *fakeStore) ListUpdates(context.Context, domain.UpdateFilter) ([]domain.Update, error) {
	return s.updates, s.err
}

func TestRenderers(t *testing.T) {
	day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	other := day.AddDate(0, 0, -1)
	data, err := json.Marshal(domain.SnapshotIncompleteData{Date: day})
	require.NoError(t, err)

	event := domain.Event{Type: domain.Events.SnapshotIncomplete, Data: data}
	renderers := NewRenderers(&fakeStore{updates: []domain.Update{
		{DateFrom: &other, DateTo: &other, ErrorMessage: "wrong day"},
		{DateFrom: &day, DateTo: &day, ErrorMessage: "VOO missing price"},
	}})

	text, err := renderers[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), event)
	require.NoError(t, err)
	assert.Contains(t, text, "2026-01-05")
	assert.Contains(t, text, "VOO missing price")
	assert.NotContains(t, text, "wrong day")

	empty := NewRenderers(&fakeStore{})
	text, err = empty[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), event)
	require.NoError(t, err)
	assert.Contains(t, text, "no run was recorded")

	failing := NewRenderers(&fakeStore{err: errors.New("db")})
	_, err = failing[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), event)
	assert.Error(t, err)

	_, err = empty[domain.Events.SnapshotIncomplete][defaultFormat].Render(context.Background(), domain.Event{Data: []byte("[")})
	assert.Error(t, err)

	failed, err := json.Marshal(domain.NotificationFailedData{EventID: 1, EventType: "snapshot.incomplete", RecordedAt: day, Failures: 10})
	require.NoError(t, err)

	text, err = renderers[domain.Events.NotificationFailed][defaultFormat].Render(context.Background(), domain.Event{Data: failed})
	require.NoError(t, err)
	assert.Contains(t, text, "snapshot.incomplete")
	assert.Contains(t, text, "10")
}

func TestSourceExpandsTargetsAndFallsBackToNoTarget(t *testing.T) {
	source := NewSource(NewLoader(writeConfig(t, validConfig)), &fakeStore{}, "")

	handlers, err := source.Handlers(context.Background())
	require.NoError(t, err)

	names := func(eventType string) []string {
		var out []string
		for _, handler := range handlers[eventType] {
			out = append(out, handler.Name())
		}

		return out
	}

	assert.Equal(t, []string{"notification:telegram:system", "notification:telegram:portfolio"}, names(domain.Events.SnapshotIncomplete))
	assert.Equal(t, []string{"notification:none"}, names(domain.Events.NotificationFailed))

	unconfigured := NewSource(NewLoader(""), &fakeStore{}, "")
	handlers, err = unconfigured.Handlers(context.Background())
	require.NoError(t, err)
	assert.Len(t, handlers, len(domain.NotifyingEvents))
	assert.NoError(t, handlers[domain.Events.SnapshotIncomplete][0].Handle(context.Background(), domain.Event{}))
}

func TestSourceFailsOnInvalidConfigurationWithoutAFallback(t *testing.T) {
	source := NewSource(NewLoader(writeConfig(t, "version: 9")), &fakeStore{}, "")

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

func TestTargetHandlerRendersThenSends(t *testing.T) {
	data, err := json.Marshal(domain.SnapshotIncompleteData{Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)})
	require.NoError(t, err)

	channel := &recordingChannel{}
	handler := &targetHandler{
		name: "notification:x:y", channel: channel, category: "y",
		renderer: NewRenderers(&fakeStore{})[domain.Events.SnapshotIncomplete][defaultFormat],
	}

	require.NoError(t, handler.Handle(context.Background(), domain.Event{Type: domain.Events.SnapshotIncomplete, Data: data}))
	assert.Equal(t, "y", channel.category)
	assert.Contains(t, channel.text, "2026-01-05")

	channel.err = errors.New("down")
	assert.Error(t, handler.Handle(context.Background(), domain.Event{Type: domain.Events.SnapshotIncomplete, Data: data}))
	assert.Equal(t, "notification:x:y", handler.Name())
}
