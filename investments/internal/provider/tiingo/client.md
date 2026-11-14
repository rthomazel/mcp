# Constants

defaultBaseURL = "https://api.tiingo.com"
providerName = "tiingo"
earliestDate = "1900-01-01", the start date sent for a full history, since the endpoint returns only the latest row when no start date is given.
requestTimeout = 60 seconds, a full history is about 1 MB.
maxErrorBodyBytes = 200, the most of a response body that is included in an error.
priceColumns = "date,open,high,low,close,adjClose,volume".

# Types

## Client

1. httpClient *http.Client
2. baseURL string
3. token string

Implements domain.PriceProvider, with a compile-time assertion. The token travels only in the Authorization header and never appears in a URL, log or error.

# Functions

## NewClient(token string, baseURL string) *Client

Build a Client with an HTTP client using requestTimeout. An empty baseURL means defaultBaseURL; tests point it at a local server.

## Client.Name() string

Return providerName.

## Client.FetchPrices(ctx Context, symbol string, from *domain.Date) ([]domain.PriceBar, error)

1. Convert the symbol to Tiingo's format by replacing periods with dashes.
2. Build GET {baseURL}/tiingo/daily/{symbol}/prices with the symbol path-escaped, startDate set to from or earliestDate, format csv and columns priceColumns.
3. Send it with the headers Authorization: Token {token} and Content-Type: application/json.
4. Read the response body.
5. Check the response for failure.
6. Parse the body with parseBars.
7. Return the bars.

#### Errors

- **3.** if the request fails, return the wrapped error.

---

- **4.** if reading fails, return the wrapped error.

---

- **5.** if the status is 429, return domain.ErrRateLimited.
- **5.1.** if the status is 404, return domain.ErrSymbolNotFound naming the symbol.
- **5.2.** if the status is otherwise not 200, return an error with the status and the body truncated to maxErrorBodyBytes.
- **5.3.** if the status is 200 and the body starts with "Error:", return domain.ErrSymbolNotFound naming the symbol when the message says not found, otherwise an error with the message.

---

- **6.** if parsing fails, return the wrapped error.

CSV is requested because it carries exact decimal text, so no float64 is involved, and it is far smaller than JSON.
For CSV the provider reports an unknown ticker as HTTP 200 with an error line in the body, not as 404; step 5.3 handles it and 5.1 covers the other endpoints.
Exceeding the rate limit is not described in the provider documentation. Treating 429 as the signal is an assumption to confirm the first time it happens.
The provider has no exchange parameter, so the instrument's exchange is not used. Mutual funds return a price a day later than exchange-traded securities; the client does not special-case them.

## parseBars(body []byte) ([]domain.PriceBar, error)

1. Read the header row and map column names to positions, requiring every name in priceColumns.
2. For each record, parse the date as YYYY-MM-DD, the prices with domain.ParseDecimal and the volume as an integer, then validate the bar with PriceBar.Validate.
3. Check the dates are strictly ascending.
4. Return the bars; a header-only body returns an empty slice.

#### Errors

- **1.** if the body is empty or a required column is missing, return an error naming it.

---

- **2.** if any record fails, return an error naming its line; no bars are returned.

---

- **3.** if a date is not after the previous one, return an error naming it.

The whole fetch fails on one bad row, never partially, so a gap or a zeroed price is never stored silently. Volume is read as an integer; adjusted volume and the other adjusted prices are not requested.
