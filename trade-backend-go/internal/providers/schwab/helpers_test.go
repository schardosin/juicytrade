package schwab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// URL builder tests
// =============================================================================

func TestBuildMarketDataURL(t *testing.T) {
	p := newTestProvider("https://api.schwabapi.com")

	tests := []struct {
		path     string
		expected string
	}{
		{"/quotes", "https://api.schwabapi.com/marketdata/v1/quotes"},
		{"/chains", "https://api.schwabapi.com/marketdata/v1/chains"},
		{"/expirationchain", "https://api.schwabapi.com/marketdata/v1/expirationchain"},
		{"/pricehistory", "https://api.schwabapi.com/marketdata/v1/pricehistory"},
		{"/instruments", "https://api.schwabapi.com/marketdata/v1/instruments"},
	}

	for _, tt := range tests {
		got := p.buildMarketDataURL(tt.path)
		if got != tt.expected {
			t.Errorf("buildMarketDataURL(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestBuildTraderURL(t *testing.T) {
	p := newTestProvider("https://api.schwabapi.com")

	tests := []struct {
		path     string
		expected string
	}{
		{"/accounts", "https://api.schwabapi.com/trader/v1/accounts"},
		{"/accounts/HASH123/orders", "https://api.schwabapi.com/trader/v1/accounts/HASH123/orders"},
		{"/userPreference", "https://api.schwabapi.com/trader/v1/userPreference"},
	}

	for _, tt := range tests {
		got := p.buildTraderURL(tt.path)
		if got != tt.expected {
			t.Errorf("buildTraderURL(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

// =============================================================================
// parseErrorResponse tests
// =============================================================================

func TestParseErrorResponse_OAuthError(t *testing.T) {
	body := []byte(`{"error": "invalid_grant", "error_description": "Refresh token expired"}`)
	err := parseErrorResponse(body, 401)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: invalid_grant: Refresh token expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_OAuthErrorNoDescription(t *testing.T) {
	body := []byte(`{"error": "server_error"}`)
	err := parseErrorResponse(body, 500)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: server_error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_APIErrors(t *testing.T) {
	body := []byte(`{"errors": [{"id": "abc123", "status": 400, "title": "Bad Request", "detail": "Invalid symbol", "message": "Symbol INVALID not found"}]}`)
	err := parseErrorResponse(body, 400)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should pick up "message" first (preferred key)
	if !strings.Contains(err.Error(), "schwab: Symbol INVALID not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_APIErrorsDetailFallback(t *testing.T) {
	body := []byte(`{"errors": [{"id": "abc123", "status": 404, "title": "Not Found", "detail": "Resource not found"}]}`)
	err := parseErrorResponse(body, 404)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// No "message" key, should fall back to "detail"
	if !strings.Contains(err.Error(), "schwab: Resource not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_MessageError(t *testing.T) {
	body := []byte(`{"message": "Not Found"}`)
	err := parseErrorResponse(body, 404)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: Not Found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_PlainText(t *testing.T) {
	body := []byte(`Service Unavailable`)
	err := parseErrorResponse(body, 503)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: HTTP 503: Service Unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_EmptyBody(t *testing.T) {
	err := parseErrorResponse([]byte{}, 500)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: HTTP 500 (empty response)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorResponse_LongBody(t *testing.T) {
	// Body longer than 200 chars should be truncated
	body := []byte(strings.Repeat("x", 300))
	err := parseErrorResponse(body, 500)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "...") {
		t.Fatalf("expected truncation indicator '...', got: %v", errStr)
	}
}

// =============================================================================
// doAuthenticatedRequest tests
// =============================================================================

// newTestProviderWithToken creates a provider with a pre-set valid token
// pointing at the given test server URL.
func newTestProviderWithToken(baseURL string) *SchwabProvider {
	p := newTestProvider(baseURL)
	p.accessToken = "valid-test-token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)
	return p
}

func TestDoAuthenticatedRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-test-token" {
			t.Errorf("expected Bearer auth, got: %s", auth)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got: %s", r.Header.Get("Accept"))
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"symbol": "AAPL", "price": 150.25}`)
	}))
	defer srv.Close()

	p := newTestProviderWithToken(srv.URL)

	body, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, srv.URL+"/marketdata/v1/quotes", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", status)
	}
	if !strings.Contains(string(body), "AAPL") {
		t.Fatalf("expected body to contain AAPL, got: %s", string(body))
	}
}

func TestDoAuthenticatedRequest_PostContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got: %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json for POST, got: %s", ct)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	p := newTestProviderWithToken(srv.URL)

	body, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodPost, srv.URL+"/trader/v1/orders", []byte(`{"order":"data"}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected status 201, got: %d", status)
	}
	_ = body
}

func TestDoAuthenticatedRequest_401Retry(t *testing.T) {
	var callCount int32

	// Token endpoint for refresh
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "refreshed-token", "expires_in": 1800, "token_type": "Bearer"}`)
	}))
	defer tokenSrv.Close()

	// API endpoint: returns 401 on first call, 200 on second
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error": "unauthorized"}`)
			return
		}
		// Second call should have refreshed token
		auth := r.Header.Get("Authorization")
		if auth != "Bearer refreshed-token" {
			t.Errorf("expected refreshed token on retry, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result": "success"}`)
	}))
	defer apiSrv.Close()

	p := newTestProviderWithToken(tokenSrv.URL)

	body, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, apiSrv.URL+"/data", nil)
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200 after retry, got: %d", status)
	}
	if !strings.Contains(string(body), "success") {
		t.Fatalf("expected success body, got: %s", string(body))
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 API calls (original + retry), got: %d", atomic.LoadInt32(&callCount))
	}
}

func TestDoAuthenticatedRequest_401RetryAlsoFails(t *testing.T) {
	// Token endpoint for refresh
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "refreshed-token", "expires_in": 1800, "token_type": "Bearer"}`)
	}))
	defer tokenSrv.Close()

	// API always returns 401
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_token", "error_description": "Token is invalid"}`)
	}))
	defer apiSrv.Close()

	p := newTestProviderWithToken(tokenSrv.URL)

	_, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, apiSrv.URL+"/data", nil)
	if err == nil {
		t.Fatal("expected error when retry also fails, got nil")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got: %d", status)
	}
	if !strings.Contains(err.Error(), "schwab: invalid_token") {
		t.Fatalf("expected parsed error, got: %v", err)
	}
}

func TestDoAuthenticatedRequest_429RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `Rate limit exceeded`)
	}))
	defer srv.Close()

	p := newTestProviderWithToken(srv.URL)

	_, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, srv.URL+"/data", nil)
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got: %d", status)
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected rate limit error, got: %v", err)
	}
}

func TestDoAuthenticatedRequest_500Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "Internal server error occurred"}`)
	}))
	defer srv.Close()

	p := newTestProviderWithToken(srv.URL)

	_, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, srv.URL+"/data", nil)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got: %d", status)
	}
	if !strings.Contains(err.Error(), "schwab: Internal server error occurred") {
		t.Fatalf("expected parsed error message, got: %v", err)
	}
}

func TestDoAuthenticatedRequest_NoToken(t *testing.T) {
	// Token endpoint returns success
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/oauth/token") {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"access_token": "new-token", "expires_in": 1800, "token_type": "Bearer"}`)
			return
		}
		// API endpoint
		auth := r.Header.Get("Authorization")
		if auth != "Bearer new-token" {
			t.Errorf("expected new token, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok": true}`)
	}))
	defer tokenSrv.Close()

	p := newTestProvider(tokenSrv.URL)
	// No token set — doAuthenticatedRequest should trigger ensureValidToken → refresh

	body, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, tokenSrv.URL+"/data", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", status)
	}
	if !strings.Contains(string(body), "ok") {
		t.Fatalf("expected ok body, got: %s", string(body))
	}
}

func TestDoAuthenticatedRequest_AuthFailure(t *testing.T) {
	// Token endpoint returns 401 — refresh token expired
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_grant"}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)
	// No valid token, and refresh will fail

	_, _, err := p.doAuthenticatedRequest(context.Background(), http.MethodGet, srv.URL+"/data", nil)
	if err == nil {
		t.Fatal("expected error when token refresh fails, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("expected authentication failed error, got: %v", err)
	}
}

func TestDoAuthenticatedRequest_RetryPreservesBody(t *testing.T) {
	var callCount int32
	var firstBody, secondBody []byte

	// Token endpoint for refresh
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "refreshed-token", "expires_in": 1800, "token_type": "Bearer"}`)
	}))
	defer tokenSrv.Close()

	// API endpoint: returns 401 on first call, 200 on second.
	// Records the request body for both calls.
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			firstBody = bodyBytes
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error": "unauthorized"}`)
			return
		}
		secondBody = bodyBytes
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result": "success"}`)
	}))
	defer apiSrv.Close()

	p := newTestProviderWithToken(tokenSrv.URL)

	requestBody := []byte(`{"orderType":"LIMIT","session":"NORMAL","price":150.50,"duration":"DAY","orderStrategyType":"SINGLE","orderLegCollection":[{"instruction":"BUY","quantity":100,"instrument":{"symbol":"AAPL","assetType":"EQUITY"}}]}`)

	respBody, status, err := p.doAuthenticatedRequest(context.Background(), http.MethodPost, apiSrv.URL+"/trader/v1/accounts/HASH/previewOrder", requestBody)
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200 after retry, got: %d", status)
	}
	if !strings.Contains(string(respBody), "success") {
		t.Fatalf("expected success body, got: %s", string(respBody))
	}

	// Verify both requests were made
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 API calls (original + retry), got: %d", atomic.LoadInt32(&callCount))
	}

	// Verify both requests had the same non-empty body
	if len(firstBody) == 0 {
		t.Fatal("first request body was empty")
	}
	if len(secondBody) == 0 {
		t.Fatal("second request body was empty — body was not preserved on 401 retry")
	}
	if string(firstBody) != string(secondBody) {
		t.Fatalf("request bodies differ:\n  first:  %s\n  second: %s", string(firstBody), string(secondBody))
	}
	if string(firstBody) != string(requestBody) {
		t.Fatalf("request body doesn't match original:\n  expected: %s\n  got:      %s", string(requestBody), string(firstBody))
	}
}

// =============================================================================
// truncateBody tests
// =============================================================================

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		maxLen   int
		expected string
	}{
		{"short", []byte("hello"), 200, "hello"},
		{"exact", []byte("12345"), 5, "12345"},
		{"long", []byte("123456789"), 5, "12345..."},
		{"empty", []byte{}, 200, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody(tt.body, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateBody(%q, %d) = %q, want %q", tt.body, tt.maxLen, got, tt.expected)
			}
		})
	}
}
