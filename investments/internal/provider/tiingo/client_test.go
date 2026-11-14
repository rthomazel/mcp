package tiingo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const csvHeader = "date,open,high,low,close,adjClose,volume\n"

func serve(t *testing.T, status int, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient("secret-key", server.URL)
}

func fetch(t *testing.T, status int, body string) ([]domain.PriceBar, error) {
	t.Helper()

	return serve(t, status, body).FetchPrices(context.Background(), "VOO", nil)
}

func TestFetchPricesRequest(t *testing.T) {
	var got *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())

		_, _ = w.Write([]byte(csvHeader))
	}))
	defer server.Close()

	client := NewClient("secret-key", server.URL)

	bars, err := client.FetchPrices(context.Background(), "BRK.B", nil)
	require.NoError(t, err)
	assert.Empty(t, bars)
	assert.NotNil(t, bars)

	assert.Equal(t, "/tiingo/daily/BRK-B/prices", got.URL.Path, "periods become dashes")
	assert.Equal(t, "1900-01-01", got.URL.Query().Get("startDate"), "full history")
	assert.Equal(t, "csv", got.URL.Query().Get("format"))
	assert.Equal(t, priceColumns, got.URL.Query().Get("columns"))
	assert.Equal(t, "Token secret-key", got.Header.Get("Authorization"))
	assert.Equal(t, "application/json", got.Header.Get("Content-Type"))
	assert.NotContains(t, got.URL.String(), "secret-key", "the key never goes in the URL")

	from := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	_, err = client.FetchPrices(context.Background(), "VOO", &from)
	require.NoError(t, err)
	assert.Equal(t, "2026-11-13", got.URL.Query().Get("startDate"))
	assert.Equal(t, "tiingo", client.Name())
}

func TestFetchPricesParsesBars(t *testing.T) {
	body := csvHeader +
		"2026-11-12,693.06,701.42,692.84,696.0,696.0,6184833\n" +
		"2026-11-13,698.66,698.81,687.5,693.38,693.38,5626216\n"

	bars, err := fetch(t, http.StatusOK, body)
	require.NoError(t, err)
	require.Len(t, bars, 2)

	assert.Equal(t, domain.PriceBar{
		Date: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC),
		Open: "693.06", High: "701.42", Low: "692.84", Close: "696.0", AdjustedClose: "696.0", Volume: 6184833,
	}, bars[0])
	assert.Equal(t, domain.Decimal("693.38"), bars[1].Close)
}

func TestFetchPricesColumnOrderFollowsHeader(t *testing.T) {
	body := "volume,close,date,adjClose,open,high,low\n7,5.5,2026-11-13,5.4,5.1,5.9,5.0\n"

	bars, err := fetch(t, http.StatusOK, body)
	require.NoError(t, err)
	require.Len(t, bars, 1)
	assert.Equal(t, domain.Decimal("5.5"), bars[0].Close)
	assert.Equal(t, domain.Decimal("5.4"), bars[0].AdjustedClose)
	assert.EqualValues(t, 7, bars[0].Volume)
}

func TestFetchPricesFailures(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
		is     error
	}{
		"rate limited":           {http.StatusTooManyRequests, "slow down", domain.ErrRateLimited},
		"not found status":       {http.StatusNotFound, `{"detail":"Not found."}`, domain.ErrSymbolNotFound},
		"not found error line":   {http.StatusOK, "Error: Ticker 'VOO' not found\n", domain.ErrSymbolNotFound},
		"empty body":             {http.StatusOK, "", nil},
		"missing column":         {http.StatusOK, "date,open,high,low,close,volume\n", nil},
		"bad date":               {http.StatusOK, csvHeader + "13/11/2026,1,1,1,1,1,1\n", nil},
		"bad price":              {http.StatusOK, csvHeader + "2026-11-13,1,1,1,abc,1,1\n", nil},
		"zero close":             {http.StatusOK, csvHeader + "2026-11-13,1,1,1,0,1,1\n", nil},
		"bad volume":             {http.StatusOK, csvHeader + "2026-11-13,1,1,1,1,1,many\n", nil},
		"dates out of order":     {http.StatusOK, csvHeader + "2026-11-13,1,1,1,1,1,1\n2026-11-12,1,1,1,1,1,1\n", nil},
		"duplicate date":         {http.StatusOK, csvHeader + "2026-11-13,1,1,1,1,1,1\n2026-11-13,1,1,1,1,1,1\n", nil},
		"other error line":       {http.StatusOK, "Error: Something odd\n", nil},
		"server error":           {http.StatusInternalServerError, "boom", nil},
		"unauthorized":           {http.StatusUnauthorized, "bad token", nil},
		"short row":              {http.StatusOK, csvHeader + "2026-11-13,1,1\n", nil},
		"one bad row fails all":  {http.StatusOK, csvHeader + "2026-11-12,1,1,1,1,1,1\n2026-11-13,1,1,1,-1,1,1\n", nil},
		"bad row hidden by good": {http.StatusOK, csvHeader + "2026-11-12,1,1,1,1,1,1\n2026-11-13,x,1,1,1,1,1\n", nil},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bars, err := fetch(t, test.status, test.body)
			require.Error(t, err)
			assert.Nil(t, bars, "never partial")

			if test.is != nil {
				assert.ErrorIs(t, err, test.is)
			}

			assert.NotContains(t, err.Error(), "secret-key")
		})
	}
}

func TestFetchPricesTruncatesErrorBody(t *testing.T) {
	_, err := fetch(t, http.StatusInternalServerError, strings.Repeat("x", 5000))
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 400)
}

func TestFetchPricesTransportError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := NewClient("secret-key", url).FetchPrices(context.Background(), "VOO", nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-key")
	assert.NotContains(t, err.Error(), url, "the URL is dropped from the error")
}

func TestFetchPricesHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(csvHeader))
	}))
	defer server.Close()

	_, err := NewClient("k", server.URL).FetchPrices(ctx, "VOO", nil)
	assert.ErrorIs(t, err, context.Canceled)
}
