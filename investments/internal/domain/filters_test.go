package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFilterValidation(t *testing.T) {
	early := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	late := early.AddDate(0, 0, 3)

	tests := map[string]struct {
		err error
	}{
		"price ok":                     {PriceFilter{InstrumentID: 1, From: &early, To: &late}.Validate()},
		"price needs an instrument":    {PriceFilter{}.Validate()},
		"price inverted range":         {PriceFilter{InstrumentID: 1, From: &late, To: &early}.Validate()},
		"price negative limit":         {PriceFilter{InstrumentID: 1, Limit: -1}.Validate()},
		"snapshot ok":                  {SnapshotFilter{From: &early, To: &late}.Validate()},
		"snapshot inverted range":      {SnapshotFilter{From: &late, To: &early}.Validate()},
		"snapshot negative limit":      {SnapshotFilter{Limit: -5}.Validate()},
		"update ok":                    {UpdateFilter{DataType: UpdateDataPrices, Status: UpdateStatusFailed}.Validate()},
		"update unknown data type":     {UpdateFilter{DataType: "other"}.Validate()},
		"update unknown status":        {UpdateFilter{Status: "running"}.Validate()},
		"update negative limit":        {UpdateFilter{Limit: -1}.Validate()},
		"update empty filter is valid": {UpdateFilter{}.Validate()},
	}

	valid := map[string]bool{"price ok": true, "snapshot ok": true, "update ok": true, "update empty filter is valid": true}

	for name, test := range tests {
		if valid[name] {
			assert.NoError(t, test.err, name)
		} else {
			assert.ErrorIs(t, test.err, ErrInvalidInput, name)
		}
	}
}

func TestResolvedLimit(t *testing.T) {
	assert.Equal(t, defaultListLimit, PriceFilter{}.ResolvedLimit())
	assert.Equal(t, 7, SnapshotFilter{Limit: 7}.ResolvedLimit())
	assert.Equal(t, maxListLimit, UpdateFilter{Limit: 10_000}.ResolvedLimit())
}

func TestTransactionFilterEventType(t *testing.T) {
	filter, err := TransactionFilter{EventType: EventDividend}.Normalize()
	require.NoError(t, err)
	assert.Equal(t, EventDividend, filter.EventType)
	assert.Equal(t, defaultListLimit, filter.Limit)

	_, err = TransactionFilter{EventType: "GIFT"}.Normalize()
	assert.ErrorIs(t, err, ErrInvalidInput)

	_, err = TransactionFilter{}.Normalize()
	assert.NoError(t, err, "an empty event type means any")
}
