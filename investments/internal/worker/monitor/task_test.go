package monitor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// stubMonitor counts its runs and keeps the params it was given.
type stubMonitor struct {
	runs   int
	params Params
}

func (m *stubMonitor) Name() string                  { return "stub.monitor" }
func (m *stubMonitor) At() domain.ClockTime          { return domain.ClockTime{Hour: 8} }
func (m *stubMonitor) ValidateParams(_ Params) error { return nil }
func (m *stubMonitor) Run(_ context.Context, params Params) error {
	m.runs++
	m.params = params

	return nil
}

func writeMonitorsFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "monitors.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestTaskIsScheduledAtTheMonitorTime(t *testing.T) {
	monitor := &stubMonitor{}
	task := Task(monitor, time.Minute, NewConfigLoader("", []Monitor{monitor}))

	assert.Equal(t, []domain.ClockTime{{Hour: 8}}, task.At)
	assert.Equal(t, time.Minute, task.Timeout)
	assert.Contains(t, task.Name, "stub.monitor")
}

func TestTaskRunsTheMonitorOnlyWhenEnabled(t *testing.T) {
	for name, tc := range map[string]struct {
		file string
		runs int
	}{
		"no file":        {file: "", runs: 0},
		"no entry":       {file: "version: 1\nmonitors: {}", runs: 0},
		"disabled":       {file: "version: 1\nmonitors:\n  stub.monitor:\n    disabled: true", runs: 0},
		"empty entry":    {file: "version: 1\nmonitors:\n  stub.monitor:", runs: 1},
		"enabled":        {file: "version: 1\nmonitors:\n  stub.monitor:\n    disabled: false", runs: 1},
		"enabled, plain": {file: "version: 1\nmonitors:\n  stub.monitor: {}", runs: 1},
	} {
		monitor := &stubMonitor{}
		path := ""

		if tc.file != "" {
			path = writeMonitorsFile(t, tc.file)
		}

		task := Task(monitor, time.Minute, NewConfigLoader(path, []Monitor{monitor}))

		require.NoError(t, task.Run(context.Background()), name)
		assert.Equal(t, tc.runs, monitor.runs, name)
	}
}

func TestTaskPassesTheEntrySettings(t *testing.T) {
	monitor := &stubMonitor{}
	path := writeMonitorsFile(t, "version: 1\nmonitors:\n  stub.monitor:\n    disabled: false\n    increment_pct: \"3\"\n    count: 5")
	task := Task(monitor, time.Minute, NewConfigLoader(path, []Monitor{monitor}))

	require.NoError(t, task.Run(context.Background()))
	assert.Equal(t, Params{"increment_pct": "3", "count": "5"}, monitor.params)
}

func TestTaskReenablingKeepsTheSettings(t *testing.T) {
	monitor := &stubMonitor{}
	path := writeMonitorsFile(t, "version: 1\nmonitors:\n  stub.monitor:\n    disabled: true\n    increment_pct: \"3\"")
	task := Task(monitor, time.Minute, NewConfigLoader(path, []Monitor{monitor}))

	require.NoError(t, task.Run(context.Background()))
	assert.Zero(t, monitor.runs)

	require.NoError(t, os.WriteFile(path, []byte("version: 1\nmonitors:\n  stub.monitor:\n    increment_pct: \"3\""), 0o600))
	require.NoError(t, task.Run(context.Background()))
	assert.Equal(t, 1, monitor.runs)
	assert.Equal(t, Params{"increment_pct": "3"}, monitor.params)
}

func TestTaskFailsOnAnInvalidFileWithoutAnEarlierValidOne(t *testing.T) {
	monitor := &stubMonitor{}
	path := writeMonitorsFile(t, "version: 9")
	task := Task(monitor, time.Minute, NewConfigLoader(path, []Monitor{monitor}))

	assert.Error(t, task.Run(context.Background()))
	assert.Zero(t, monitor.runs)
}

func TestTaskKeepsTheLastValidFileWhenAnEditBreaksIt(t *testing.T) {
	monitor := &stubMonitor{}
	path := writeMonitorsFile(t, "version: 1\nmonitors:\n  stub.monitor: {}")
	task := Task(monitor, time.Minute, NewConfigLoader(path, []Monitor{monitor}))

	require.NoError(t, task.Run(context.Background()))
	require.NoError(t, os.WriteFile(path, []byte("version: 1\nmonitors:\n  stub.monitor: {}\n  nope: {}"), 0o600))
	require.NoError(t, task.Run(context.Background()))
	assert.Equal(t, 2, monitor.runs)
}

func TestConfigRejects(t *testing.T) {
	monitors := []Monitor{newMonitor(&fakeStore{}), NewPortfolioDrawdown(&fakeStore{}, &fakeStore{})}

	for name, content := range map[string]string{
		"wrong version":         "version: 2\nmonitors: {}",
		"missing version":       "monitors: {}",
		"unknown top-level":     "version: 1\nmonitor: {}",
		"unknown monitor":       "version: 1\nmonitors:\n  nope: {}",
		"unknown, disabled":     "version: 1\nmonitors:\n  nope:\n    disabled: true",
		"disabled not boolean":  "version: 1\nmonitors:\n  snapshot.incomplete:\n    disabled: yes please",
		"disabled misspelled":   "version: 1\nmonitors:\n  snapshot.incomplete:\n    disabled: 1",
		"repeated key":          "version: 1\nmonitors:\n  snapshot.incomplete: {}\n  snapshot.incomplete: {}",
		"nested setting":        "version: 1\nmonitors:\n  portfolio.drawdown:\n    increment_pct: [3]",
		"settings it rejects":   "version: 1\nmonitors:\n  snapshot.incomplete:\n    extra: \"1\"",
		"missing increment":     "version: 1\nmonitors:\n  portfolio.drawdown: {}",
		"zero increment":        "version: 1\nmonitors:\n  portfolio.drawdown:\n    increment_pct: \"0\"",
		"negative increment":    "version: 1\nmonitors:\n  portfolio.drawdown:\n    increment_pct: \"-3\"",
		"increment not number":  "version: 1\nmonitors:\n  portfolio.drawdown:\n    increment_pct: three",
		"invalid when disabled": "version: 1\nmonitors:\n  portfolio.drawdown:\n    disabled: true\n    increment_pct: \"0\"",
		"unknown setting":       "version: 1\nmonitors:\n  portfolio.drawdown:\n    increment_pct: \"3\"\n    other: \"1\"",
	} {
		_, err := NewConfigLoader(writeMonitorsFile(t, content), monitors).Current()
		assert.Error(t, err, name)
	}
}

func TestConfigAcceptsAValidFile(t *testing.T) {
	monitors := []Monitor{newMonitor(&fakeStore{}), NewPortfolioDrawdown(&fakeStore{}, &fakeStore{})}
	path := writeMonitorsFile(t, `
version: 1
monitors:
  snapshot.incomplete:
  portfolio.drawdown:
    disabled: true
    increment_pct: "3"
`)

	cfg, err := NewConfigLoader(path, monitors).Current()

	require.NoError(t, err)
	assert.Equal(t, map[string]Entry{
		"snapshot.incomplete": {},
		"portfolio.drawdown":  {Disabled: true, Params: Params{"increment_pct": "3"}},
	}, cfg.Monitors)
}
