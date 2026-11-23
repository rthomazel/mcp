package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func decodeParams(t *testing.T, content string) Params {
	t.Helper()

	var params Params

	require.NoError(t, yaml.Unmarshal([]byte(content), &params))

	return params
}

func TestParamsUnknownNamesTheFirstInNameOrder(t *testing.T) {
	err := decodeParams(t, "b: 1\na: 2\nok: 3").Unknown("ok")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"a"`)
	assert.NoError(t, Params{"ok": 1}.Unknown("ok"))
}

func TestParamsDecimal(t *testing.T) {
	params := decodeParams(t, "text: \"2.5\"\nnumber: 2.5\nbad: abc")

	got, ok, err := params.Decimal("text")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.EqualValues(t, "2.5", got)

	_, ok, err = params.Decimal("missing")
	require.NoError(t, err)
	assert.False(t, ok)

	_, _, err = params.Decimal("number")
	assert.Error(t, err, "an unquoted number is not text")

	_, _, err = params.Decimal("bad")
	assert.Error(t, err)
}

func TestParamsDuration(t *testing.T) {
	params := decodeParams(t, "window: 5 days\nbare: 15\nbad: 15x\nfloat: 1.5")

	got, ok, err := params.Duration("window")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 5, got.Days)

	_, ok, err = params.Duration("missing")
	require.NoError(t, err)
	assert.False(t, ok)

	for _, key := range []string{"bare", "bad", "float"} {
		_, _, err = params.Duration(key)
		assert.Error(t, err, key)
	}

	_, _, err = params.Duration("bare")
	assert.Contains(t, err.Error(), "unit is missing")
}

func TestParamsMappings(t *testing.T) {
	params := decodeParams(t, "groups:\n  - name: a\n    min_pct: \"5\"\n  - name: b\nscalar: 1\nempty: []\nmixed: [a]")

	got, ok, err := params.Mappings("groups")
	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0]["name"])

	nested, _, err := got[0].Text("name")
	require.NoError(t, err)
	assert.Equal(t, "a", nested)

	for _, key := range []string{"scalar", "empty", "mixed"} {
		_, _, err = params.Mappings(key)
		assert.Error(t, err, key)
	}

	_, ok, err = params.Mappings("missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestParamsInstruments(t *testing.T) {
	params := decodeParams(t, "list: [qqq, \" gld \"]\nheld: HELD\nsingle: qqq\nempty: []\ndup: [a, A]\nnumber: [1]\nblank: [\"\"]")

	got, ok, err := params.Instruments("list")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, Selection{Tickers: []string{"QQQ", "GLD"}}, got)

	got, _, err = params.Instruments("held")
	require.NoError(t, err)
	assert.Equal(t, Selection{Held: true}, got)

	for _, key := range []string{"single", "empty", "dup", "number", "blank"} {
		_, _, err = params.Instruments(key)
		assert.Error(t, err, key)
	}

	_, ok, err = params.Instruments("missing")
	require.NoError(t, err)
	assert.False(t, ok)
}
