package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newYork(t *testing.T) *time.Location {
	t.Helper()

	location, err := time.LoadLocation(marketTimezone)
	require.NoError(t, err)

	return location
}

func TestNextRun(t *testing.T) {
	ny := newYork(t)

	tests := map[string]struct {
		now  time.Time
		want time.Time
	}{
		"before morning":  {time.Date(2026, 11, 14, 1, 0, 0, 0, ny), time.Date(2026, 11, 14, 6, 0, 0, 0, ny)},
		"between runs":    {time.Date(2026, 11, 14, 12, 0, 0, 0, ny), time.Date(2026, 11, 14, 20, 30, 0, 0, ny)},
		"after evening":   {time.Date(2026, 11, 14, 21, 0, 0, 0, ny), time.Date(2026, 11, 15, 6, 0, 0, 0, ny)},
		"exactly on time": {time.Date(2026, 11, 14, 6, 0, 0, 0, ny), time.Date(2026, 11, 14, 20, 30, 0, 0, ny)},
		// Spring forward on 2026-03-08: 20:30 EDT is 00:30Z, not 01:30Z.
		"across spring forward": {time.Date(2026, 3, 7, 21, 0, 0, 0, ny), time.Date(2026, 3, 8, 6, 0, 0, 0, ny)},
		// Fall back on 2026-11-01.
		"across fall back": {time.Date(2026, 10, 31, 21, 0, 0, 0, ny), time.Date(2026, 11, 1, 6, 0, 0, 0, ny)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := nextRun(test.now, ny)

			assert.True(t, test.want.Equal(got), "want %s got %s", test.want, got)
			assert.Equal(t, test.want.Hour(), got.In(ny).Hour())
		})
	}
}

// The host zone, such as Brasilia time, must not change the run times.
func TestNextRunIgnoresHostZone(t *testing.T) {
	ny := newYork(t)
	instant := time.Date(2026, 11, 14, 12, 0, 0, 0, ny)
	want := time.Date(2026, 11, 14, 20, 30, 0, 0, ny)

	brasilia, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	previous := time.Local
	time.Local = brasilia

	t.Cleanup(func() { time.Local = previous })

	assert.True(t, want.Equal(nextRun(instant.In(brasilia), ny)))
	assert.True(t, want.Equal(nextRun(instant, ny)))
}

func TestRunTimesDescription(t *testing.T) {
	assert.Equal(t, "06:00, 20:30 America/New_York", RunTimesDescription())
}
