package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func newDrift(store *fakeStore) *AllocationDrift { return NewAllocationDrift(store, store) }

func yamlParams(t *testing.T, content string) Params {
	t.Helper()

	var params Params

	require.NoError(t, yaml.Unmarshal([]byte(content), &params))

	return params
}

const driftSettings = `
groups:
  - name: us-core
    instruments: [voo, VTI]
    max_pct: "15"
  - name: gold
    instruments: [GLD]
    min_pct: "5"
    max_pct: "10"
`

func driftOn(side string) *domain.AllocationDrift {
	stay := day(11, 17)

	return &domain.AllocationDrift{
		Date: day(11, 20), Unregistered: []string{"NOPE"},
		Groups: []domain.GroupAllocation{
			{Name: "us-core", AllocationPercent: "17.5", Side: side, StayStart: &stay,
				Members: []domain.MemberAllocation{{Symbol: "VOO", AllocationPercent: "10"}}},
			{Name: "gold", AllocationPercent: "7"},
		},
	}
}

func TestAllocationDriftIsNamedAfterItsEvent(t *testing.T) {
	assert.Equal(t, "allocation.drift", newDrift(&fakeStore{}).Name())
}

func TestAllocationDriftRecordsGroupsOutsideTheirBand(t *testing.T) {
	store := &fakeStore{created: true, drift: driftOn("above")}

	require.NoError(t, newDrift(store).Run(context.Background(), yamlParams(t, driftSettings)))

	require.Len(t, store.groups, 2)
	assert.Equal(t, []string{"VOO", "VTI"}, store.groups[0].Tickers)
	assert.Equal(t, domain.Decimal("15"), *store.groups[0].MaxPercent)
	assert.Nil(t, store.groups[0].MinPercent)

	require.Len(t, store.recorded, 1, "gold is inside its band")
	assert.Equal(t, domain.Events.AllocationDrift, store.recorded[0].Type)
	assert.Equal(t, "us-core@above@2026-11-17", *store.recorded[0].Subject)

	var data domain.AllocationDriftData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, "us-core", data.Group)
	assert.Equal(t, day(11, 17), data.StayStart)
	assert.Equal(t, day(11, 20), data.Date)
	assert.Equal(t, domain.Decimal("17.5"), data.AllocationPercent)
	assert.Equal(t, domain.Decimal("15"), *data.MaxPercent)
	assert.Nil(t, data.MinPercent)
	assert.Equal(t, "VOO", data.Members[0].Symbol)
}

func TestAllocationDriftRecordsNothingInsideTheBandOrWithoutSecurities(t *testing.T) {
	for name, drift := range map[string]*domain.AllocationDrift{"no securities": nil, "inside": driftOn("")} {
		store := &fakeStore{created: true, drift: drift}

		require.NoError(t, newDrift(store).Run(context.Background(), yamlParams(t, driftSettings)), name)
		assert.Empty(t, store.recorded, name)
	}
}

func TestAllocationDriftReturnsStoreErrors(t *testing.T) {
	store := &fakeStore{driftErr: errors.New("ambiguous")}

	assert.Error(t, newDrift(store).Run(context.Background(), yamlParams(t, driftSettings)))
}

func TestAllocationDriftValidatesItsSettings(t *testing.T) {
	monitor := newDrift(&fakeStore{})

	assert.NoError(t, monitor.ValidateParams(yamlParams(t, driftSettings)))
	assert.NoError(t, monitor.ValidateParams(yamlParams(t, "groups:\n  - {name: a, instruments: [X], min_pct: \"10\", max_pct: \"10\"}")))
	assert.NoError(t, monitor.ValidateParams(yamlParams(t, "groups:\n  - {name: a, instruments: [X], min_pct: \"0\"}")))

	for name, content := range map[string]string{
		"missing groups":      "{}",
		"empty groups":        "groups: []",
		"unknown setting":     "groups:\n  - {name: a, instruments: [X], max_pct: \"5\"}\nother: 1",
		"unknown key":         "groups:\n  - {name: a, instruments: [X], max_pct: \"5\", size_pct: \"5\"}",
		"no name":             "groups:\n  - {instruments: [X], max_pct: \"5\"}",
		"blank name":          "groups:\n  - {name: \" \", instruments: [X], max_pct: \"5\"}",
		"repeated name":       "groups:\n  - {name: a, instruments: [X], max_pct: \"5\"}\n  - {name: a, instruments: [Y], max_pct: \"5\"}",
		"no instruments":      "groups:\n  - {name: a, max_pct: \"5\"}",
		"held":                "groups:\n  - {name: a, instruments: held, max_pct: \"5\"}",
		"empty instruments":   "groups:\n  - {name: a, instruments: [], max_pct: \"5\"}",
		"repeat in group":     "groups:\n  - {name: a, instruments: [X, x], max_pct: \"5\"}",
		"ticker in two":       "groups:\n  - {name: a, instruments: [X], max_pct: \"5\"}\n  - {name: b, instruments: [X], max_pct: \"5\"}",
		"no bounds":           "groups:\n  - {name: a, instruments: [X]}",
		"min above max":       "groups:\n  - {name: a, instruments: [X], min_pct: \"6\", max_pct: \"5\"}",
		"bound above 100":     "groups:\n  - {name: a, instruments: [X], max_pct: \"100.5\"}",
		"negative bound":      "groups:\n  - {name: a, instruments: [X], min_pct: \"-1\"}",
		"unquoted bound":      "groups:\n  - {name: a, instruments: [X], max_pct: 5}",
		"not a number":        "groups:\n  - {name: a, instruments: [X], max_pct: lots}",
		"group not a mapping": "groups: [a]",
	} {
		assert.Error(t, monitor.ValidateParams(yamlParams(t, content)), name)
	}
}
