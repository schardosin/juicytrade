package schwab

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestProvider creates a SchwabProvider pointing at the given base URL for testing.
func newTestProvider(baseURL string) *SchwabProvider {
	return NewSchwabProvider(
		"test-app-key",
		"test-app-secret",
		"https://localhost/callback",
		"test-refresh-token",
		"test-account-hash",
		baseURL,
		"live",
		"", nil,
	)
}

// validTokenResponseJSON returns a valid Schwab token response JSON string.
func validTokenResponseJSON() string {
	return `{
		"access_token": "new-access-token-12345",
		"token_type": "Bearer",
		"expires_in": 1800,
		"refresh_token": "",
		"scope": "api",
		"id_token": ""
	}`
}

// =============================================================================
// ensureValidToken tests
// =============================================================================

func TestEnsureValidToken_CachedToken(t *testing.T) {
	// Track whether the server is called
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validTokenResponseJSON())
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// Pre-set a valid token with future expiry (10 minutes from now)
	p.accessToken = "existing-valid-token"
	p.tokenExpiry = time.Now().Add(10 * time.Minute)

	err := p.ensureValidToken()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if called {
		t.Fatal("expected no HTTP call when token is still valid, but server was called")
	}
	if p.accessToken != "existing-valid-token" {
		t.Fatalf("expected token to remain unchanged, got: %s", p.accessToken)
	}
}

func TestEnsureValidToken_ExpiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validTokenResponseJSON())
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// Set an expired token
	p.accessToken = "expired-token"
	p.tokenExpiry = time.Now().Add(-1 * time.Hour)

	err := p.ensureValidToken()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.accessToken != "new-access-token-12345" {
		t.Fatalf("expected token to be refreshed, got: %s", p.accessToken)
	}
}

func TestEnsureValidToken_NearExpiryToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validTokenResponseJSON())
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// Set a token expiring in 3 minutes (within the 5-minute buffer)
	p.accessToken = "near-expiry-token"
	p.tokenExpiry = time.Now().Add(3 * time.Minute)

	err := p.ensureValidToken()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.accessToken != "new-access-token-12345" {
		t.Fatalf("expected token to be refreshed due to near expiry, got: %s", p.accessToken)
	}
}

func TestEnsureValidToken_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validTokenResponseJSON())
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// No token set (default empty state)
	err := p.ensureValidToken()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.accessToken != "new-access-token-12345" {
		t.Fatalf("expected token to be set after refresh, got: %s", p.accessToken)
	}
}

// =============================================================================
// refreshAccessToken tests
// =============================================================================

func TestRefreshAccessToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/oauth/token") {
			t.Errorf("expected path ending in /v1/oauth/token, got %s", r.URL.Path)
		}

		// Verify Content-Type
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("expected Content-Type application/x-www-form-urlencoded, got %s", ct)
		}

		// Verify Basic auth header
		auth := r.Header.Get("Authorization")
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("test-app-key:test-app-secret"))
		if auth != expectedAuth {
			t.Errorf("expected Authorization %s, got %s", expectedAuth, auth)
		}

		// Verify form body
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "grant_type=refresh_token") {
			t.Errorf("expected grant_type=refresh_token in body, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "refresh_token=test-refresh-token") {
			t.Errorf("expected refresh_token=test-refresh-token in body, got: %s", bodyStr)
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"access_token": "fresh-access-token",
			"token_type": "Bearer",
			"expires_in": 1800,
			"refresh_token": "",
			"scope": "api",
			"id_token": "some-id-token"
		}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)
	beforeRefresh := time.Now()

	// Call with tokenMu held (as required by the contract)
	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.accessToken != "fresh-access-token" {
		t.Fatalf("expected access token 'fresh-access-token', got: %s", p.accessToken)
	}

	// Token expiry should be ~30 minutes from now
	expectedExpiry := beforeRefresh.Add(1800 * time.Second)
	if p.tokenExpiry.Before(expectedExpiry.Add(-5 * time.Second)) {
		t.Fatalf("token expiry too early: %v (expected around %v)", p.tokenExpiry, expectedExpiry)
	}
	if p.tokenExpiry.After(expectedExpiry.Add(5 * time.Second)) {
		t.Fatalf("token expiry too late: %v (expected around %v)", p.tokenExpiry, expectedExpiry)
	}
}

func TestRefreshAccessToken_ExpiredRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_grant", "error_description": "Refresh token expired"}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Fatalf("expected ErrRefreshTokenExpired, got: %v", err)
	}
	if !p.authExpired {
		t.Fatal("expected authExpired to be true after 401 response")
	}
}

func TestRefreshAccessToken_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{not valid json`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: failed to parse token response") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestRefreshAccessToken_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "", "expires_in": 1800}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err == nil {
		t.Fatal("expected error for missing access_token, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: token response missing access_token") {
		t.Fatalf("expected missing access_token error, got: %v", err)
	}
}

func TestRefreshAccessToken_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `Internal Server Error`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "schwab: token request returned status 500") {
		t.Fatalf("expected status 500 error, got: %v", err)
	}
}

func TestRefreshAccessToken_TokenRotation(t *testing.T) {
	// This test verifies that token rotation is detected (a new refresh_token
	// is returned that differs from the original). The warning is logged but
	// the function still succeeds.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := schwabTokenResponse{
			AccessToken:  "rotated-access-token",
			TokenType:    "Bearer",
			ExpiresIn:    1800,
			RefreshToken: "brand-new-refresh-token", // Different from original
			Scope:        "api",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err != nil {
		t.Fatalf("expected no error despite token rotation, got: %v", err)
	}
	if p.accessToken != "rotated-access-token" {
		t.Fatalf("expected access token 'rotated-access-token', got: %s", p.accessToken)
	}
	// The refresh token should now be updated in-memory (new behavior)
	if p.refreshToken != "brand-new-refresh-token" {
		t.Fatalf("expected refresh token to be updated in-memory, got: %s", p.refreshToken)
	}
}

func TestRefreshAccessToken_RotatesAndPersists(t *testing.T) {
	// Verify that when the server returns a new refresh_token AND a credentialUpdater
	// is configured, the updater is called with the correct arguments.
	var updaterCalled bool
	var updaterInstanceID string
	var updaterUpdates map[string]interface{}

	updater := CredentialUpdater(func(instanceID string, updates map[string]interface{}) error {
		updaterCalled = true
		updaterInstanceID = instanceID
		updaterUpdates = updates
		return nil
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := schwabTokenResponse{
			AccessToken:  "new-access",
			TokenType:    "Bearer",
			ExpiresIn:    1800,
			RefreshToken: "rotated-refresh-token",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewSchwabProvider(
		"test-app-key", "test-app-secret", "https://localhost/callback",
		"original-refresh-token", "test-hash", srv.URL, "live",
		"schwab_live_1", updater,
	)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !updaterCalled {
		t.Fatal("expected credentialUpdater to be called")
	}
	if updaterInstanceID != "schwab_live_1" {
		t.Errorf("expected instanceID 'schwab_live_1', got %q", updaterInstanceID)
	}
	if updaterUpdates["refresh_token"] != "rotated-refresh-token" {
		t.Errorf("expected refresh_token 'rotated-refresh-token', got %v", updaterUpdates["refresh_token"])
	}
	// In-memory should also be updated
	if p.refreshToken != "rotated-refresh-token" {
		t.Errorf("expected in-memory refreshToken updated, got %q", p.refreshToken)
	}
}

func TestRefreshAccessToken_RotatesPersistFailure(t *testing.T) {
	// Verify that when the credentialUpdater returns an error, the in-memory
	// refresh token is still updated and no panic occurs.
	updater := CredentialUpdater(func(instanceID string, updates map[string]interface{}) error {
		return fmt.Errorf("disk full")
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := schwabTokenResponse{
			AccessToken:  "new-access",
			TokenType:    "Bearer",
			ExpiresIn:    1800,
			RefreshToken: "rotated-token-not-persisted",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewSchwabProvider(
		"key", "secret", "https://cb", "original-token", "hash", srv.URL, "live",
		"schwab_live_1", updater,
	)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err != nil {
		t.Fatalf("expected no error (persist failure is logged, not returned), got: %v", err)
	}
	// In-memory should still be updated even if persistence failed
	if p.refreshToken != "rotated-token-not-persisted" {
		t.Errorf("expected in-memory refreshToken updated despite persist failure, got %q", p.refreshToken)
	}
}

func TestRefreshAccessToken_SetsAuthExpiredOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_grant"}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	if p.authExpired {
		t.Fatal("expected authExpired=false initially")
	}

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Fatalf("expected ErrRefreshTokenExpired, got: %v", err)
	}
	if !p.authExpired {
		t.Fatal("expected authExpired=true after 401 response")
	}
}

func TestRefreshAccessToken_ExpiresInZero(t *testing.T) {
	// If expires_in is 0 or missing, we should still accept the token
	// but the expiry will be effectively "now" which means the next
	// ensureValidToken call will trigger another refresh.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "zero-expiry-token", "expires_in": 0}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.accessToken != "zero-expiry-token" {
		t.Fatalf("expected access token 'zero-expiry-token', got: %s", p.accessToken)
	}
}

// =============================================================================
// truncateToken tests
// =============================================================================

func TestTruncateToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678..."},
		{"a-very-long-refresh-token-value", "a-very-l..."},
	}

	for _, tt := range tests {
		got := truncateToken(tt.input)
		if got != tt.expected {
			t.Errorf("truncateToken(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
