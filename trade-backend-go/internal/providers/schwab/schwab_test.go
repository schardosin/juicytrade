package schwab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Constructor tests
// =============================================================================

func TestNewSchwabProvider(t *testing.T) {
	p := NewSchwabProvider("key", "secret", "https://cb.example.com", "refresh-tok", "hash123", "https://custom.api.com", "live", "", nil)

	if p.appKey != "key" {
		t.Errorf("expected appKey 'key', got %q", p.appKey)
	}
	if p.appSecret != "secret" {
		t.Errorf("expected appSecret 'secret', got %q", p.appSecret)
	}
	if p.callbackURL != "https://cb.example.com" {
		t.Errorf("expected callbackURL 'https://cb.example.com', got %q", p.callbackURL)
	}
	if p.refreshToken != "refresh-tok" {
		t.Errorf("expected refreshToken 'refresh-tok', got %q", p.refreshToken)
	}
	if p.accountHash != "hash123" {
		t.Errorf("expected accountHash 'hash123', got %q", p.accountHash)
	}
	if p.baseURL != "https://custom.api.com" {
		t.Errorf("expected baseURL 'https://custom.api.com', got %q", p.baseURL)
	}
	if p.accountType != "live" {
		t.Errorf("expected accountType 'live', got %q", p.accountType)
	}
	if p.rateLimiter == nil {
		t.Error("expected rateLimiter to be initialized")
	}
	if p.logger == nil {
		t.Error("expected logger to be initialized")
	}
}

func TestNewSchwabProvider_Defaults(t *testing.T) {
	p := NewSchwabProvider("key", "secret", "https://cb.example.com", "tok", "hash", "", "", "", nil)

	if p.baseURL != "https://api.schwabapi.com" {
		t.Errorf("expected default baseURL 'https://api.schwabapi.com', got %q", p.baseURL)
	}
	if p.accountType != "live" {
		t.Errorf("expected default accountType 'live', got %q", p.accountType)
	}
}

func TestNewSchwabProvider_WithUpdater(t *testing.T) {
	updater := CredentialUpdater(func(instanceID string, updates map[string]interface{}) error {
		return nil
	})

	p := NewSchwabProvider("key", "secret", "https://cb.example.com", "tok", "hash",
		"https://api.schwabapi.com", "live",
		"schwab_live_MyAccount", updater,
	)

	if p.instanceID != "schwab_live_MyAccount" {
		t.Errorf("expected instanceID 'schwab_live_MyAccount', got %q", p.instanceID)
	}
	if p.credentialUpdater == nil {
		t.Error("expected credentialUpdater to be set")
	}
}

func TestNewSchwabProvider_NilUpdater(t *testing.T) {
	p := NewSchwabProvider("key", "secret", "https://cb.example.com", "tok", "hash",
		"https://api.schwabapi.com", "live",
		"", nil,
	)

	if p.instanceID != "" {
		t.Errorf("expected empty instanceID, got %q", p.instanceID)
	}
	if p.credentialUpdater != nil {
		t.Error("expected credentialUpdater to be nil")
	}
	// Verify no panic when constructing with nil updater
}

// =============================================================================
// Test helpers for multi-endpoint mock servers
// =============================================================================

// testServerOptions configures the mock server behavior.
type testServerOptions struct {
	tokenStatus          int
	tokenBody            string
	accountNumbersStatus int
	accountNumbersBody   string
}

// newMultiEndpointServer creates a mock server handling both token and API endpoints.
func newMultiEndpointServer(opts testServerOptions) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(opts.tokenStatus)
			fmt.Fprint(w, opts.tokenBody)

		case strings.HasSuffix(r.URL.Path, "/accounts/accountNumbers"):
			w.WriteHeader(opts.accountNumbersStatus)
			fmt.Fprint(w, opts.accountNumbersBody)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message": "not found: %s"}`, r.URL.Path)
		}
	}))
}

const validTokenBody = `{"access_token": "test-access-token", "expires_in": 1800, "token_type": "Bearer"}`

// =============================================================================
// TestCredentials tests
// =============================================================================

func TestTestCredentials_Success(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus:          http.StatusOK,
		tokenBody:            validTokenBody,
		accountNumbersStatus: http.StatusOK,
		accountNumbersBody:   `[{"accountNumber": "12345678", "hashValue": "test-account-hash"}]`,
	})
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if !success {
		t.Fatalf("expected success=true, got result: %v", result)
	}

	message, _ := result["message"].(string)
	if !strings.Contains(message, "validated successfully") {
		t.Fatalf("expected success message, got: %s", message)
	}
}

func TestTestCredentials_PaperAccount(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus:          http.StatusOK,
		tokenBody:            validTokenBody,
		accountNumbersStatus: http.StatusOK,
		accountNumbersBody:   `[{"accountNumber": "12345678", "hashValue": "test-account-hash"}]`,
	})
	defer srv.Close()

	p := NewSchwabProvider(
		"test-app-key",
		"test-app-secret",
		"https://localhost/callback",
		"test-refresh-token",
		"test-account-hash",
		srv.URL,
		"paper", // paper account
		"", nil,
	)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if !success {
		t.Fatalf("expected success=true, got result: %v", result)
	}

	message, _ := result["message"].(string)
	if !strings.Contains(message, "sandbox") {
		t.Fatalf("expected sandbox warning in message, got: %s", message)
	}
	if !strings.Contains(message, "limitations") {
		t.Fatalf("expected limitations warning, got: %s", message)
	}
}

func TestTestCredentials_InvalidCredentials(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus: http.StatusUnauthorized,
		tokenBody:   `{"error": "invalid_grant", "error_description": "Refresh token expired"}`,
	})
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error (result should have success=false), got: %v", err)
	}

	success, _ := result["success"].(bool)
	if success {
		t.Fatalf("expected success=false, got result: %v", result)
	}

	message, _ := result["message"].(string)
	if !strings.Contains(message, "Authentication failed") {
		t.Fatalf("expected authentication failure message, got: %s", message)
	}
}

func TestTestCredentials_AccountHashMismatch(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus:          http.StatusOK,
		tokenBody:            validTokenBody,
		accountNumbersStatus: http.StatusOK,
		accountNumbersBody:   `[{"accountNumber": "12345678", "hashValue": "different-hash"}, {"accountNumber": "87654321", "hashValue": "another-hash"}]`,
	})
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if success {
		t.Fatalf("expected success=false for mismatched hash, got result: %v", result)
	}

	message, _ := result["message"].(string)
	if !strings.Contains(message, "not found") {
		t.Fatalf("expected 'not found' message, got: %s", message)
	}
	if !strings.Contains(message, "2 accounts") {
		t.Fatalf("expected account count in message, got: %s", message)
	}
}

func TestTestCredentials_EmptyResponse(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus:          http.StatusOK,
		tokenBody:            validTokenBody,
		accountNumbersStatus: http.StatusOK,
		accountNumbersBody:   `[]`,
	})
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if success {
		t.Fatalf("expected success=false for empty accounts, got result: %v", result)
	}

	message, _ := result["message"].(string)
	if !strings.Contains(message, "No accounts found") {
		t.Fatalf("expected 'No accounts found' message, got: %s", message)
	}
}

func TestTestCredentials_APIError(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus:          http.StatusOK,
		tokenBody:            validTokenBody,
		accountNumbersStatus: http.StatusInternalServerError,
		accountNumbersBody:   `{"message": "Internal server error"}`,
	})
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if success {
		t.Fatalf("expected success=false for API error, got result: %v", result)
	}
}

func TestTestCredentials_MultipleAccountsMatchesCorrectOne(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus:          http.StatusOK,
		tokenBody:            validTokenBody,
		accountNumbersStatus: http.StatusOK,
		accountNumbersBody:   `[{"accountNumber": "11111111", "hashValue": "hash-aaa"}, {"accountNumber": "22222222", "hashValue": "test-account-hash"}, {"accountNumber": "33333333", "hashValue": "hash-ccc"}]`,
	})
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if !success {
		t.Fatalf("expected success=true when hash is among multiple accounts, got result: %v", result)
	}
}

func TestTestCredentials_AuthExpired(t *testing.T) {
	// No server needed — authExpired short-circuits before any HTTP call
	p := newTestProvider("http://unused")
	p.authExpired = true

	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	success, _ := result["success"].(bool)
	if success {
		t.Fatal("expected success=false when authExpired is true")
	}

	authExpired, _ := result["auth_expired"].(bool)
	if !authExpired {
		t.Fatal("expected auth_expired=true in response")
	}

	message, _ := result["message"].(string)
	if !strings.Contains(message, "Refresh token expired") {
		t.Errorf("expected 'Refresh token expired' in message, got: %s", message)
	}
}

// =============================================================================
// Ping tests
// =============================================================================

func TestPing_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validTokenBody)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	err := p.Ping(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPing_SuccessWithCachedToken(t *testing.T) {
	// Server should not be called if token is still valid
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validTokenBody)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)
	p.accessToken = "cached-token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)

	err := p.Ping(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if called {
		t.Error("expected no HTTP call when token is cached")
	}
}

func TestPing_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_grant"}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	err := p.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error for failed ping, got nil")
	}
}
