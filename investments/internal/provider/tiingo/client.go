package tiingo

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	defaultBaseURL = "https://api.tiingo.com"
	providerName   = "tiingo"
	// The endpoint returns only the latest row when no start date is given.
	earliestDate      = "1900-01-01"
	requestTimeout    = 60 * time.Second
	maxErrorBodyBytes = 200
	priceColumns      = "date,open,high,low,close,adjClose,volume"
)

var _ domain.PriceProvider = (*Client)(nil)

// Client fetches prices from Tiingo. The API key travels only in the
// Authorization header and never appears in a URL, log or error.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient builds a Client; an empty baseURL means the public API.
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{httpClient: &http.Client{Timeout: requestTimeout}, baseURL: baseURL, apiKey: apiKey}
}

// Name returns the provider name.
func (c *Client) Name() string {
	return providerName
}

// FetchPrices returns daily bars for symbol from the date inclusive, or the
// full available history when from is nil.
func (c *Client) FetchPrices(ctx context.Context, symbol string, from *domain.Date) ([]domain.PriceBar, error) {
	start := earliestDate
	if from != nil {
		start = from.Format(time.DateOnly)
	}

	query := url.Values{"startDate": {start}, "format": {"csv"}, "columns": {priceColumns}}
	endpoint := fmt.Sprintf("%s/tiingo/daily/%s/prices?%s",
		c.baseURL, url.PathEscape(strings.ReplaceAll(symbol, ".", "-")), query.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err // drop the URL, keep the cause
		}

		return nil, fmt.Errorf("requesting prices for %s: %w", symbol, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading prices for %s: %w", symbol, err)
	}

	if err := checkResponse(resp.StatusCode, body, symbol); err != nil {
		return nil, err
	}

	bars, err := parseBars(body)
	if err != nil {
		return nil, fmt.Errorf("parsing prices for %s: %w", symbol, err)
	}

	return bars, nil
}

// checkResponse maps failures. For CSV the provider reports an unknown ticker
// as HTTP 200 with an error line, not as 404.
func checkResponse(status int, body []byte, symbol string) error {
	switch {
	case status == http.StatusTooManyRequests:
		return domain.ErrRateLimited
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s", domain.ErrSymbolNotFound, symbol)
	case status != http.StatusOK:
		return fmt.Errorf("provider returned status %d: %s", status, truncate(body))
	case bytes.HasPrefix(body, []byte("Error:")):
		if strings.Contains(strings.ToLower(string(body)), "not found") {
			return fmt.Errorf("%w: %s", domain.ErrSymbolNotFound, symbol)
		}

		return fmt.Errorf("provider error: %s", truncate(body))
	}

	return nil
}

func truncate(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > maxErrorBodyBytes {
		text = text[:maxErrorBodyBytes]
	}

	return text
}

// parseBars reads the CSV body. One bad row fails the whole fetch, so a gap
// or a zeroed price is never stored silently.
func parseBars(body []byte) ([]domain.PriceBar, error) {
	reader := csv.NewReader(bytes.NewReader(body))

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	columns, err := columnPositions(header)
	if err != nil {
		return nil, err
	}

	bars := []domain.PriceBar{}

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return bars, nil
		}

		if err != nil {
			return nil, fmt.Errorf("reading record: %w", err)
		}

		line, _ := reader.FieldPos(0)

		bar, err := parseBar(record, columns)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}

		if len(bars) > 0 && !bar.Date.After(bars[len(bars)-1].Date) {
			return nil, fmt.Errorf("line %d: date %s is not after the previous date", line, bar.Date.Format(time.DateOnly))
		}

		bars = append(bars, bar)
	}
}

func columnPositions(header []string) (map[string]int, error) {
	columns := make(map[string]int, len(header))
	for position, name := range header {
		columns[strings.TrimSpace(name)] = position
	}

	for _, name := range strings.Split(priceColumns, ",") {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("missing column %q", name)
		}
	}

	return columns, nil
}

func parseBar(record []string, columns map[string]int) (domain.PriceBar, error) {
	field := func(name string) string {
		if position := columns[name]; position < len(record) {
			return record[position]
		}

		return ""
	}

	date, err := time.Parse(time.DateOnly, field("date"))
	if err != nil {
		return domain.PriceBar{}, fmt.Errorf("parsing date: %w", err)
	}

	volume, err := strconv.ParseInt(strings.TrimSpace(field("volume")), 10, 64)
	if err != nil {
		return domain.PriceBar{}, fmt.Errorf("parsing volume: %w", err)
	}

	bar := domain.PriceBar{
		Date: date, Open: domain.Decimal(field("open")), High: domain.Decimal(field("high")),
		Low: domain.Decimal(field("low")), Close: domain.Decimal(field("close")),
		AdjustedClose: domain.Decimal(field("adjClose")), Volume: volume,
	}

	if err := bar.Validate(); err != nil {
		return domain.PriceBar{}, err
	}

	return bar, nil
}
