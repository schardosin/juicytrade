package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// CreateState tests
// =============================================================================

func TestOAuthStore_CreateState_GeneratesUniqueTokens(t *testing.T) {
	store := NewSchwabOAuthStore()

	token1, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	token2, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	if token1 == token2 {
		t.Errorf("expected unique tokens, both are %q", token1)
	}
}

func TestOAuthStore_CreateState_StoresCorrectData(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, err := store.CreateState("my-key", "my-secret", "https://cb.example.com", "https://api.schwab.com")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	state := store.GetState(token)
	if state == nil {
		t.Fatal("expected state to be found")
	}

	if state.AppKey != "my-key" {
		t.Errorf("expected AppKey 'my-key', got %q", state.AppKey)
	}
	if state.AppSecret != "my-secret" {
		t.Errorf("expected AppSecret 'my-secret', got %q", state.AppSecret)
	}
	if state.CallbackURL != "https://cb.example.com" {
		t.Errorf("expected CallbackURL 'https://cb.example.com', got %q", state.CallbackURL)
	}
	if state.BaseURL != "https://api.schwab.com" {
		t.Errorf("expected BaseURL 'https://api.schwab.com', got %q", state.BaseURL)
	}
	if state.Status != "pending" {
		t.Errorf("expected Status 'pending', got %q", state.Status)
	}
	if state.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

// =============================================================================
// GetState tests
// =============================================================================

func TestOAuthStore_GetState_NotFound(t *testing.T) {
	store := NewSchwabOAuthStore()

	state := store.GetState("nonexistent-token-abc123")
	if state != nil {
		t.Errorf("expected nil for unknown token, got %+v", state)
	}
}

func TestOAuthStore_GetState_Expired(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	// Manually set CreatedAt to 11 minutes ago to simulate expiry
	state := store.GetState(token)
	if state == nil {
		t.Fatal("expected state to exist before expiry manipulation")
	}
	state.mu.Lock()
	state.CreatedAt = time.Now().Add(-11 * time.Minute)
	state.mu.Unlock()

	// Now GetState should return nil (expired)
	expired := store.GetState(token)
	if expired != nil {
		t.Error("expected nil for expired state")
	}
}

// =============================================================================
// UpdateState tests
// =============================================================================

func TestOAuthStore_UpdateState_TransitionsStatus(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, _ := store.CreateState("key", "secret", "https://cb", "https://api")

	ok := store.UpdateState(token, func(s *OAuthFlowState) {
		s.Status = "exchanging"
	})
	if !ok {
		t.Fatal("expected UpdateState to return true")
	}

	state := store.GetState(token)
	if state.Status != "exchanging" {
		t.Errorf("expected Status 'exchanging', got %q", state.Status)
	}
}

func TestOAuthStore_UpdateState_NotFound(t *testing.T) {
	store := NewSchwabOAuthStore()

	ok := store.UpdateState("nonexistent", func(s *OAuthFlowState) {
		s.Status = "should-not-happen"
	})
	if ok {
		t.Error("expected UpdateState to return false for unknown token")
	}
}

// =============================================================================
// DeleteState tests
// =============================================================================

func TestOAuthStore_DeleteState(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, _ := store.CreateState("key", "secret", "https://cb", "https://api")

	// Verify it exists
	if store.GetState(token) == nil {
		t.Fatal("expected state to exist before delete")
	}

	store.DeleteState(token)

	// Verify it's gone
	if store.GetState(token) != nil {
		t.Error("expected state to be nil after delete")
	}
}

// =============================================================================
// Cleanup tests
// =============================================================================

func TestOAuthStore_StartCleanup_RemovesExpired(t *testing.T) {
	store := NewSchwabOAuthStore()

	// Create a state and backdate it beyond TTL
	token, _ := store.CreateState("key", "secret", "https://cb", "https://api")
	state := store.GetState(token)
	state.mu.Lock()
	state.CreatedAt = time.Now().Add(-15 * time.Minute)
	state.mu.Unlock()

	// Create a fresh state that should survive cleanup
	freshToken, _ := store.CreateState("key2", "secret2", "https://cb2", "https://api2")

	// Run cleanup directly (no need to wait for the goroutine ticker)
	store.removeExpired()

	// Expired state should be gone (check raw map since GetState also checks expiry)
	if _, loaded := store.states.Load(token); loaded {
		t.Error("expected expired state to be removed by cleanup")
	}

	// Fresh state should still be there
	if store.GetState(freshToken) == nil {
		t.Error("expected fresh state to survive cleanup")
	}
}

func TestOAuthStore_StartCleanup_ContextCancel(t *testing.T) {
	store := NewSchwabOAuthStore()

	ctx, cancel := context.WithCancel(context.Background())
	store.StartCleanup(ctx)

	// Cancel immediately — the goroutine should exit without error or panic
	cancel()

	// Give the goroutine a moment to exit
	time.Sleep(50 * time.Millisecond)
}

// =============================================================================
// Token generation tests
// =============================================================================

func TestOAuthStore_StateToken_Length(t *testing.T) {
	// 32 bytes → base64url (no padding) = ceil(32*4/3) = 43 characters
	token, err := generateStateToken()
	if err != nil {
		t.Fatalf("generateStateToken returned error: %v", err)
	}

	if len(token) != 43 {
		t.Errorf("expected token length 43, got %d (token=%q)", len(token), token)
	}
}

// =============================================================================
// Concurrency tests
// =============================================================================

func TestOAuthStore_ConcurrentAccess(t *testing.T) {
	store := NewSchwabOAuthStore()

	const goroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Collect tokens from create goroutines for use by read/update goroutines
	tokens := make(chan string, goroutines*opsPerGoroutine)

	// Half the goroutines create states
	for i := 0; i < goroutines/2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				token, err := store.CreateState("key", "secret", "https://cb", "https://api")
				if err != nil {
					t.Errorf("CreateState error: %v", err)
					return
				}
				tokens <- token
			}
		}()
	}

	// Other half read, update, and delete states
	for i := 0; i < goroutines/2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				select {
				case token := <-tokens:
					// Read
					store.GetState(token)

					// Update
					store.UpdateState(token, func(s *OAuthFlowState) {
						s.Status = "exchanging"
					})

					// Read again
					store.GetState(token)

					// Delete half the time
					if j%2 == 0 {
						store.DeleteState(token)
					}
				default:
					// No tokens available yet, create and immediately read
					tk, _ := store.CreateState("k", "s", "c", "b")
					store.GetState(tk)
				}
			}
		}()
	}

	// Wait with timeout to detect deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success — no races, no deadlocks
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for concurrent operations — possible deadlock")
	}
}

// =============================================================================
// exchangeCodeForTokens tests
// =============================================================================

func TestExchangeCodeForTokens_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"expires_in":    1800,
			"token_type":    "Bearer",
		})
	}))
	defer srv.Close()

	result, err := exchangeCodeForTokens(srv.URL, "key", "secret", "https://cb", "auth-code-123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.AccessToken != "test-access-token" {
		t.Errorf("expected AccessToken 'test-access-token', got %q", result.AccessToken)
	}
	if result.RefreshToken != "test-refresh-token" {
		t.Errorf("expected RefreshToken 'test-refresh-token', got %q", result.RefreshToken)
	}
	if result.ExpiresIn != 1800 {
		t.Errorf("expected ExpiresIn 1800, got %d", result.ExpiresIn)
	}
}

func TestExchangeCodeForTokens_InvalidCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant"}`)
	}))
	defer srv.Close()

	_, err := exchangeCodeForTokens(srv.URL, "key", "secret", "https://cb", "bad-code")
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "invalid authorization code") {
		t.Errorf("expected 'invalid authorization code' error, got: %v", err)
	}
}

func TestExchangeCodeForTokens_InvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid_client"}`)
	}))
	defer srv.Close()

	_, err := exchangeCodeForTokens(srv.URL, "bad-key", "bad-secret", "https://cb", "code")
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
	if !strings.Contains(err.Error(), "invalid client credentials") {
		t.Errorf("expected 'invalid client credentials' error, got: %v", err)
	}
}

func TestExchangeCodeForTokens_BasicAuthHeader(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"expires_in":   1800,
		})
	}))
	defer srv.Close()

	_, err := exchangeCodeForTokens(srv.URL, "my-app-key", "my-app-secret", "https://cb", "code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(receivedAuth, "Basic ") {
		t.Fatalf("expected Basic auth header, got %q", receivedAuth)
	}
}

func TestExchangeCodeForTokens_FormBody(t *testing.T) {
	var receivedBody string
	var receivedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"expires_in":   1800,
		})
	}))
	defer srv.Close()

	_, err := exchangeCodeForTokens(srv.URL, "key", "secret", "https://my-cb.com/callback", "my-auth-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedContentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected Content-Type 'application/x-www-form-urlencoded', got %q", receivedContentType)
	}
	if !strings.Contains(receivedBody, "grant_type=authorization_code") {
		t.Errorf("expected grant_type=authorization_code in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "code=my-auth-code") {
		t.Errorf("expected code=my-auth-code in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "redirect_uri=") {
		t.Errorf("expected redirect_uri in body, got: %s", receivedBody)
	}
}

// =============================================================================
// fetchAccountNumbers tests
// =============================================================================

func TestFetchAccountNumbers_SingleAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]string{
			{"accountNumber": "123456789", "hashValue": "ABC123HASH"},
		})
	}))
	defer srv.Close()

	accounts, err := fetchAccountNumbers(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	// Account number should be masked
	if accounts[0].AccountNumber != "*****6789" {
		t.Errorf("expected masked account '*****6789', got %q", accounts[0].AccountNumber)
	}
	if accounts[0].HashValue != "ABC123HASH" {
		t.Errorf("expected hash 'ABC123HASH', got %q", accounts[0].HashValue)
	}
}

func TestFetchAccountNumbers_MultipleAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]string{
			{"accountNumber": "111111111", "hashValue": "HASH1"},
			{"accountNumber": "222222222", "hashValue": "HASH2"},
			{"accountNumber": "333333333", "hashValue": "HASH3"},
		})
	}))
	defer srv.Close()

	accounts, err := fetchAccountNumbers(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(accounts) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(accounts))
	}
	// Verify all are masked
	for i, acct := range accounts {
		if !strings.HasPrefix(acct.AccountNumber, "*") {
			t.Errorf("account %d not masked: %q", i, acct.AccountNumber)
		}
	}
}

func TestFetchAccountNumbers_EmptyAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	accounts, err := fetchAccountNumbers(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if accounts == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}
}

// =============================================================================
// maskAccountNumber tests
// =============================================================================

func TestMaskAccountNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"1", "1"},
		{"12", "12"},
		{"1234", "1234"},
		{"12345", "*2345"},
		{"123456789", "*****6789"},
		{"9876543210", "******3210"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got := maskAccountNumber(tt.input)
			if got != tt.expected {
				t.Errorf("maskAccountNumber(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// renderCallbackPage tests
// =============================================================================

func TestRenderCallbackPage_Success(t *testing.T) {
	html := string(renderCallbackPage("success", ""))

	if !strings.Contains(html, "JuicyTrade - Schwab Authorization") {
		t.Error("expected page title in HTML")
	}
	if !strings.Contains(html, "successful") {
		t.Error("expected 'successful' in HTML for success status")
	}
	if !strings.Contains(html, "close this tab") {
		t.Error("expected 'close this tab' in HTML for success status")
	}
	if !strings.Contains(html, "#00c853") {
		t.Error("expected green accent color for success")
	}
}

func TestRenderCallbackPage_Error(t *testing.T) {
	html := string(renderCallbackPage("error", "Something went wrong with OAuth"))

	if !strings.Contains(html, "Something went wrong with OAuth") {
		t.Error("expected error message in HTML")
	}
	if !strings.Contains(html, "Failed") {
		t.Error("expected 'Failed' heading in HTML for error status")
	}
	if !strings.Contains(html, "#ff1744") {
		t.Error("expected red accent color for error")
	}
}

func TestRenderCallbackPage_Cancelled(t *testing.T) {
	html := string(renderCallbackPage("cancelled", ""))

	if !strings.Contains(html, "cancelled") || !strings.Contains(html, "Cancelled") {
		t.Error("expected 'cancelled' in HTML for cancelled status")
	}
	if !strings.Contains(html, "#ffc107") {
		t.Error("expected yellow accent color for cancelled")
	}
}
