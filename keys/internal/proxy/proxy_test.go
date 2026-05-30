package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rthomazel/mcp/keys/internal/config"
	"github.com/rthomazel/mcp/keys/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAndJoinURL(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		path      string
		want      string
		wantError bool
	}{
		{"absolute path", "https://api.github.com", "/repos/owner/repo", "https://api.github.com/repos/owner/repo", false},
		{"no leading slash", "https://api.github.com", "repos/owner/repo", "https://api.github.com/repos/owner/repo", false},
		{"dot-dot normalized", "https://api.github.com", "/a/../b", "https://api.github.com/b", false},
		{"traversal cannot escape root", "https://api.github.com", "/a/../../etc/passwd", "https://api.github.com/etc/passwd", false},
		{"query string preserved", "https://api.github.com", "/search?q=foo&page=2", "https://api.github.com/search?q=foo&page=2", false},
		{"empty path becomes root", "https://api.github.com", "", "https://api.github.com/", false},
		{"absolute url rejected", "https://api.github.com", "https://evil.com/path", "", true},
		{"scheme-relative rejected", "https://api.github.com", "//evil.com/path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndJoinURL(tt.base, tt.path)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScrubBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		secrets []string
		want    string
	}{
		{"no secret in body", `{"id":1}`, []string{"tok123"}, `{"id":1}`},
		{"secret found verbatim", `{"token":"tok123"}`, []string{"tok123"}, "[redacted: response contained secret values]"},
		{"partial match not redacted", `{"t":"tok12"}`, []string{"tok123"}, `{"t":"tok12"}`},
		{"empty body", "", []string{"tok123"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, scrubBody(tt.body, tt.secrets))
		})
	}
}

func TestAllowedHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		want    map[string]string
	}{
		{"content-type included", http.Header{"Content-Type": {"application/json"}}, map[string]string{"Content-Type": "application/json"}},
		{"x-ratelimit-remaining included", http.Header{"X-Ratelimit-Remaining": {"59"}}, map[string]string{"X-Ratelimit-Remaining": "59"}},
		{"x-ratelimit varied case", http.Header{"X-Ratelimit-Limit": {"5000"}}, map[string]string{"X-Ratelimit-Limit": "5000"}},
		{"authorization dropped", http.Header{"Authorization": {"Bearer tok"}}, map[string]string{}},
		{"set-cookie dropped", http.Header{"Set-Cookie": {"session=abc"}}, map[string]string{}},
		{"no headers", http.Header{}, map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 200, Header: tt.headers}
			assert.Equal(t, tt.want, allowedHeaders(resp))
		})
	}
}

func newProxy(t *testing.T, store *secrets.Store) *Proxy {
	t.Helper()
	return New(5*time.Second, 1024*1024, 1024*1024, store)
}

func TestDo(t *testing.T) {
	t.Run("injected header arrives at server", func(t *testing.T) {
		receivedAuth := ""
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{"ok":true}`)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{"api_key": "mysecret"})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{
			BaseURL: srv.URL,
			Inject:  map[string]config.InjectConfig{"Authorization": {Secret: "api_key", Format: "Bearer {value}"}},
		}

		resp, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", nil)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Status)
		assert.Equal(t, "Bearer mysecret", receivedAuth)
	})

	t.Run("agent authorization overwritten by inject", func(t *testing.T) {
		receivedAuth := ""
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{}`)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{"api_key": "injected"})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{
			BaseURL: srv.URL,
			Inject:  map[string]config.InjectConfig{"Authorization": {Secret: "api_key", Format: "Bearer {value}"}},
		}

		resp, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", map[string]string{"Authorization": "agent-value"})
		require.NoError(t, err)
		require.Equal(t, 200, resp.Status)
		assert.Equal(t, "Bearer injected", receivedAuth)
	})

	t.Run("blocked header Connection stripped", func(t *testing.T) {
		received := ""
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = r.Header.Get("Connection")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{}`)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		_, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", map[string]string{"Connection": "keep-alive"})
		require.NoError(t, err)
		assert.Equal(t, "", received)
	})

	t.Run("redirect not followed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://example.com", http.StatusMovedPermanently)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		resp, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.Status)
	})

	t.Run("request body over limit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{})
		p := New(5*time.Second, 1024*1024, 100, store) // 100B request limit
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		_, err := p.Do(context.Background(), "tool", tcfg, "/", "POST", strings.Repeat("x", 101), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request body exceeds limit")
	})

	t.Run("response body over limit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// write 1025 KB — just over the 1MB limit
			for range 1025 {
				io.WriteString(w, strings.Repeat("x", 1024))
			}
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		_, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "response body exceeds limit")
	})

	t.Run("response body scrubbed when secret present", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{"token":"supersecret"}`)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{"tok": "supersecret"})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		resp, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", nil)
		require.NoError(t, err)
		assert.Equal(t, "[redacted: response contained secret values]", resp.Body)
	})

	t.Run("server returns 404 no error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"error":"not found"}`)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		resp, err := p.Do(context.Background(), "tool", tcfg, "/", "GET", "", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.Status)
	})

	t.Run("context cancelled returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		store := secrets.NewStoreForTest(map[string]string{})
		p := newProxy(t, store)
		tcfg := config.ToolConfig{BaseURL: srv.URL, Inject: map[string]config.InjectConfig{}}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := p.Do(ctx, "tool", tcfg, "/", "GET", "", nil)
		require.Error(t, err)
	})
}
