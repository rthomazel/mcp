// Package proxy executes authenticated HTTP requests on behalf of MCP tool calls.
package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rthomazel/mcp/keys/internal/config"
	"github.com/rthomazel/mcp/keys/internal/secrets"
)

// Response is the structured response returned to the agent.
type Response struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

// Proxy executes authenticated HTTP requests on behalf of tool calls.
type Proxy struct {
	client           *http.Client
	store            *secrets.Store
	maxResponseBytes int64
	maxRequestBytes  int64
}

// blocked is the set of headers always rejected from agent input.
var blocked = map[string]bool{
	"Host":                true,
	"Content-Length":      true,
	"Transfer-Encoding":   true,
	"Connection":          true,
	"Upgrade":             true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
}

// New creates a Proxy with redirects disabled and the given timeout.
func New(timeout time.Duration, maxResponseBytes, maxRequestBytes int64, store *secrets.Store) *Proxy {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Proxy{
		client:           client,
		store:            store,
		maxResponseBytes: maxResponseBytes,
		maxRequestBytes:  maxRequestBytes,
	}
}

// Do executes an authenticated HTTP request for the given tool.
// path, method, body, agentHeaders, responseHeadersFilter come from the agent\'s tool call.
// Returns a Response or a descriptive error. Errors never contain secret values.
func (p *Proxy) Do(ctx context.Context, toolName string, toolCfg config.ToolConfig, reqPath, method, body string, agentHeaders map[string]string, responseHeadersFilter string) (*Response, error) {
	// Check request body size.
	if int64(len(body)) > p.maxRequestBytes {
		return nil, fmt.Errorf("tool %q: request body exceeds limit (%d bytes max)", toolName, p.maxRequestBytes)
	}

	// Validate and join URL.
	finalURL, err := validateAndJoinURL(toolCfg.BaseURL, reqPath)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", toolName, err)
	}

	// Build headers: agent headers first (minus blocked), then injected headers win.
	headers := make(map[string]string)

	for k, v := range agentHeaders {
		if !blocked[http.CanonicalHeaderKey(k)] {
			headers[k] = v
		}
	}

	for headerName, inject := range toolCfg.Inject {
		secretValue := p.store.Get(inject.Secret)
		if inject.Format != "" {
			headers[headerName] = strings.ReplaceAll(inject.Format, "{value}", secretValue)
		} else {
			headers[headerName] = secretValue
		}
	}

	// Execute request.
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, finalURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("tool %q: create request: %w", toolName, err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, fmt.Errorf("tool %q: request failed: %w", toolName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response up to limit+1 to detect oversize.
	limited := io.LimitReader(resp.Body, p.maxResponseBytes+1)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("tool %q: read response: %w", toolName, err)
	}
	if int64(len(respBytes)) > p.maxResponseBytes {
		return nil, fmt.Errorf("tool %q: response body exceeds limit", toolName)
	}

	// Redact response body if it contains secret values.
	respBody := redactBody(string(respBytes), p.store.Values())

	return &Response{
		Status:  resp.StatusCode,
		Headers: responseHeaders(resp, p.store.Values(), responseHeadersFilter),
		Body:    respBody,
	}, nil
}

// validateAndJoinURL checks that reqPath is relative and joins it onto baseURL.
// Normalizes .. segments. Preserves query string.
func validateAndJoinURL(baseURL, reqPath string) (string, error) {
	if strings.Contains(reqPath, "://") {
		return "", fmt.Errorf("path must not contain a scheme")
	}

	parsedPath, err := url.Parse(reqPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if parsedPath.Scheme != "" || parsedPath.Host != "" {
		return "", fmt.Errorf("path must not contain scheme or host")
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	apiPath := parsedPath.Path
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}

	// Prepend any existing base path (e.g. base_url "https://api.example.com/v1").
	basePath := strings.TrimSuffix(parsedBase.Path, "/")
	parsedBase.Path = path.Clean(basePath + apiPath)
	parsedBase.RawQuery = parsedPath.RawQuery
	parsedBase.Fragment = ""

	return parsedBase.String(), nil
}

// redactBody replaces the body with a redaction notice if any secret value is found in it.
func redactBody(body string, secretValues []string) string {
	for _, secret := range secretValues {
		if strings.Contains(body, secret) {
			return "[redacted: response contained secret values]"
		}
	}
	return body
}

// strippedHeaders is the set of hop-by-hop headers that are connection-specific
// and meaningless to the agent outside the immediate TCP connection.
var strippedHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// responseHeaders returns response headers filtered by filter.
// filter "" or "NONE": returns nil (no headers in output).
// filter "ALL": returns all headers except hop-by-hop ones.
// Any other value: treated as a comma-separated list of canonical header names to include.
// Any header value containing a known secret is redacted.
func responseHeaders(resp *http.Response, secretValues []string, filter string) map[string]string {
	normalized := strings.ToUpper(strings.TrimSpace(filter))
	if normalized == "" || normalized == "NONE" {
		return nil
	}

	// Build an allowlist for string filters.
	var allowed map[string]bool
	if normalized != "ALL" {
		allowed = make(map[string]bool)
		for _, name := range strings.Split(filter, ",") {
			allowed[http.CanonicalHeaderKey(strings.TrimSpace(name))] = true
		}
	}

	result := make(map[string]string)

	for k, vals := range resp.Header {
		canonical := http.CanonicalHeaderKey(k)
		if strippedHeaders[canonical] || len(vals) == 0 {
			continue
		}
		if allowed != nil && !allowed[canonical] {
			continue
		}

		val := vals[0]
		for _, secret := range secretValues {
			if strings.Contains(val, secret) {
				val = "[redacted]"
				break
			}
		}

		result[canonical] = val
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
