package tastytrade

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/utils"
)

// TestGetQuoteToken_CacheHit verifies that a valid cached token is returned without API call.
func TestGetQuoteToken_CacheHit(t *testing.T) {
	// Track API calls
	var apiCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		t.Errorf("unexpected API call to %s - should have used cached token", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"token":      "fresh-token",
				"dxlink-url": "wss://fresh.example.com",
			},
		})
	}))
	defer server.Close()

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		baseURL:          server.URL,
		httpClient:       utils.NewHTTPClient(),
		// Pre-set a valid cached token (expires in 23 hours - well within safety margin)
		quoteToken: "cached-token-abc",
		dxlinkURL:  "wss://cached.example.com",
	}
	expires := time.Now().Add(23 * time.Hour)
	provider.quoteExpires = &expires

	// Also set valid session to avoid session refresh
	sessionExpires := time.Now().Add(1 * time.Hour)
	provider.sessionToken = "Bearer test-session"
	provider.sessionExpires = &sessionExpires

	ctx := context.Background()
	err := provider.getQuoteToken(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Token should remain the cached value
	if provider.quoteToken != "cached-token-abc" {
		t.Errorf("expected cached token 'cached-token-abc', got '%s'", provider.quoteToken)
	}

	// No API calls should have been made
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Errorf("expected 0 API calls, got %d", atomic.LoadInt32(&apiCalls))
	}
}

// TestGetQuoteToken_EmptyToken verifies that an empty token triggers a fresh fetch.
func TestGetQuoteToken_EmptyToken(t *testing.T) {
	var apiCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		if r.URL.Path == "/api-quote-tokens" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"token":      "new-token-xyz",
					"dxlink-url": "wss://new.example.com",
				},
			})
		} else if r.URL.Path == "/oauth/token" {
			// Session refresh
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "session-token",
				"expires_in":   900,
			})
		} else {
			t.Errorf("unexpected API call to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		baseURL:          server.URL,
		httpClient:       utils.NewHTTPClient(),
		quoteToken:       "", // Empty - should trigger fetch
		clientSecret:     "test-secret",
		refreshToken:     "test-refresh",
	}

	// Set valid session to avoid needing OAuth
	sessionExpires := time.Now().Add(1 * time.Hour)
	provider.sessionToken = "Bearer test-session"
	provider.sessionExpires = &sessionExpires

	ctx := context.Background()
	err := provider.getQuoteToken(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Token should be the freshly fetched value
	if provider.quoteToken != "new-token-xyz" {
		t.Errorf("expected 'new-token-xyz', got '%s'", provider.quoteToken)
	}
	if provider.dxlinkURL != "wss://new.example.com" {
		t.Errorf("expected 'wss://new.example.com', got '%s'", provider.dxlinkURL)
	}

	// Exactly 1 API call to /api-quote-tokens
	if atomic.LoadInt32(&apiCalls) != 1 {
		t.Errorf("expected 1 API call, got %d", atomic.LoadInt32(&apiCalls))
	}
}

// TestGetQuoteToken_ExpiredToken verifies that an expired token triggers a fresh fetch.
func TestGetQuoteToken_ExpiredToken(t *testing.T) {
	var apiCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		if r.URL.Path == "/api-quote-tokens" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"token":      "refreshed-token",
					"dxlink-url": "wss://refreshed.example.com",
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		baseURL:          server.URL,
		httpClient:       utils.NewHTTPClient(),
		quoteToken:       "old-expired-token",
		dxlinkURL:        "wss://old.example.com",
	}
	// Token expired 1 hour ago
	expired := time.Now().Add(-1 * time.Hour)
	provider.quoteExpires = &expired

	// Set valid session
	sessionExpires := time.Now().Add(1 * time.Hour)
	provider.sessionToken = "Bearer test-session"
	provider.sessionExpires = &sessionExpires

	ctx := context.Background()
	err := provider.getQuoteToken(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Token should be refreshed
	if provider.quoteToken != "refreshed-token" {
		t.Errorf("expected 'refreshed-token', got '%s'", provider.quoteToken)
	}

	if atomic.LoadInt32(&apiCalls) != 1 {
		t.Errorf("expected 1 API call, got %d", atomic.LoadInt32(&apiCalls))
	}
}

// TestGetQuoteToken_SafetyMargin verifies that a token within the 5-minute safety margin
// triggers a refresh even though it hasn't technically expired yet.
func TestGetQuoteToken_SafetyMargin(t *testing.T) {
	var apiCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		if r.URL.Path == "/api-quote-tokens" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"token":      "margin-refreshed-token",
					"dxlink-url": "wss://margin.example.com",
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		baseURL:          server.URL,
		httpClient:       utils.NewHTTPClient(),
		quoteToken:       "about-to-expire-token",
		dxlinkURL:        "wss://expiring.example.com",
	}
	// Token expires in 3 minutes - within the 5-minute safety margin
	almostExpired := time.Now().Add(3 * time.Minute)
	provider.quoteExpires = &almostExpired

	// Set valid session
	sessionExpires := time.Now().Add(1 * time.Hour)
	provider.sessionToken = "Bearer test-session"
	provider.sessionExpires = &sessionExpires

	ctx := context.Background()
	err := provider.getQuoteToken(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Token should be refreshed because it's within the safety margin
	if provider.quoteToken != "margin-refreshed-token" {
		t.Errorf("expected 'margin-refreshed-token', got '%s'", provider.quoteToken)
	}

	if atomic.LoadInt32(&apiCalls) != 1 {
		t.Errorf("expected 1 API call (safety margin refresh), got %d", atomic.LoadInt32(&apiCalls))
	}
}

// TestGetQuoteToken_SafetyMargin_JustOutside verifies that a token expiring in 6 minutes
// (just outside the 5-minute safety margin) is still considered valid and cached.
func TestGetQuoteToken_SafetyMargin_JustOutside(t *testing.T) {
	var apiCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		t.Errorf("unexpected API call - token should be cached (expires in 6 min)")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		baseURL:          server.URL,
		httpClient:       utils.NewHTTPClient(),
		quoteToken:       "still-valid-token",
		dxlinkURL:        "wss://valid.example.com",
	}
	// Token expires in 6 minutes - just outside the 5-minute safety margin
	stillValid := time.Now().Add(6 * time.Minute)
	provider.quoteExpires = &stillValid

	// Set valid session
	sessionExpires := time.Now().Add(1 * time.Hour)
	provider.sessionToken = "Bearer test-session"
	provider.sessionExpires = &sessionExpires

	ctx := context.Background()
	err := provider.getQuoteToken(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Token should remain cached
	if provider.quoteToken != "still-valid-token" {
		t.Errorf("expected 'still-valid-token', got '%s'", provider.quoteToken)
	}

	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Errorf("expected 0 API calls, got %d", atomic.LoadInt32(&apiCalls))
	}
}

// TestGetQuoteToken_ConcurrentAccess verifies thread safety when multiple goroutines
// call getQuoteToken simultaneously with an empty cache.
func TestGetQuoteToken_ConcurrentAccess(t *testing.T) {
	var apiCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		// Simulate some latency
		time.Sleep(10 * time.Millisecond)
		if r.URL.Path == "/api-quote-tokens" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"token":      "concurrent-token",
					"dxlink-url": "wss://concurrent.example.com",
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		baseURL:          server.URL,
		httpClient:       utils.NewHTTPClient(),
		quoteToken:       "", // Empty - all goroutines will try to fetch
	}

	// Set valid session
	sessionExpires := time.Now().Add(1 * time.Hour)
	provider.sessionToken = "Bearer test-session"
	provider.sessionExpires = &sessionExpires

	ctx := context.Background()
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := provider.getQuoteToken(ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	// Due to the double-check locking pattern, only 1 API call should be made
	// (the first goroutine fetches, others see the cached value after acquiring the write lock)
	calls := atomic.LoadInt32(&apiCalls)
	if calls != 1 {
		t.Errorf("expected exactly 1 API call with double-check locking, got %d", calls)
	}

	// All goroutines should see a non-empty token
	if provider.quoteToken == "" {
		t.Error("expected non-empty token after concurrent access")
	}
}
