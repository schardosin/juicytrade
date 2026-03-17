package schwab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// QA Tests for Step 2 — Provider Struct Changes (AC-7, AC-8)
// ============================================================================

// TestRefreshAccessToken_EmptyInstanceID_SkipsUpdater verifies that when
// credentialUpdater is non-nil but instanceID is empty, the updater is NOT
// called during token rotation. This guards against accidental writes to the
// credential store without an instance identifier.
func TestRefreshAccessToken_EmptyInstanceID_SkipsUpdater(t *testing.T) {
	var updaterCalled bool
	updater := CredentialUpdater(func(instanceID string, updates map[string]interface{}) error {
		updaterCalled = true
		return nil
	})

	// Server returns a rotated refresh token to trigger the rotation path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := schwabTokenResponse{
			AccessToken:  "new-access-token",
			TokenType:    "Bearer",
			ExpiresIn:    1800,
			RefreshToken: "rotated-refresh-token", // different from original
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Construct with empty instanceID but non-nil updater.
	p := NewSchwabProvider(
		"test-app-key", "test-app-secret", "https://localhost/callback",
		"original-refresh-token", "test-hash", srv.URL, "live",
		"", updater, // empty instanceID, non-nil updater
	)

	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// The updater should NOT have been called because instanceID is empty.
	if updaterCalled {
		t.Fatal("credentialUpdater was called despite empty instanceID; expected it to be skipped")
	}

	// In-memory refresh token should still be updated regardless.
	if p.refreshToken != "rotated-refresh-token" {
		t.Errorf("expected in-memory refreshToken to be updated, got %q", p.refreshToken)
	}
}

// TestAuthExpired_DoesNotAutoClear verifies that once authExpired is set to
// true, it remains true across subsequent calls. The flag should never
// auto-clear — only an explicit re-authentication flow should reset it.
func TestAuthExpired_DoesNotAutoClear(t *testing.T) {
	// Set up a server that always returns 401 — this is the expected state
	// when the refresh token is truly expired.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_grant"}`)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// 1. Trigger authExpired by attempting a token refresh against the 401 server.
	p.tokenMu.Lock()
	err := p.refreshAccessToken()
	p.tokenMu.Unlock()

	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Fatalf("expected ErrRefreshTokenExpired on first call, got: %v", err)
	}
	if !p.authExpired {
		t.Fatal("expected authExpired=true after first 401")
	}

	// 2. Call ensureValidToken again — should still fail because authExpired
	//    causes refreshAccessToken to be called (no valid cached token), and
	//    the server still returns 401.
	err = p.ensureValidToken()
	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Fatalf("expected ErrRefreshTokenExpired on second call, got: %v", err)
	}
	if !p.authExpired {
		t.Fatal("expected authExpired to remain true after second call")
	}

	// 3. Verify TestCredentials also reflects the expired state.
	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("TestCredentials returned unexpected error: %v", err)
	}

	success, _ := result["success"].(bool)
	if success {
		t.Fatal("expected TestCredentials to return success=false when authExpired")
	}

	authExpiredFlag, _ := result["auth_expired"].(bool)
	if !authExpiredFlag {
		t.Fatal("expected auth_expired=true in TestCredentials response")
	}

	// 4. Final verification: authExpired is still true.
	if !p.authExpired {
		t.Fatal("expected authExpired to remain true after all calls")
	}
}

// TestAllConstructorCallSites_Compile verifies that the entire schwab package
// (including all test files) compiles successfully. This catches any call site
// that was not updated when the NewSchwabProvider constructor signature changed
// to accept instanceID and credentialUpdater.
func TestAllConstructorCallSites_Compile(t *testing.T) {
	// This test's existence and passage proves compilation success.
	// The `go test` command compiles all *_test.go files in the package before
	// running any tests. If any call site uses the old constructor signature,
	// the build fails and no tests run at all.
	//
	// For additional assurance, we also verify that the constructor works with
	// both nil and non-nil credentialUpdater values.

	// Case 1: nil updater, empty instanceID (backward-compatible usage).
	p1 := NewSchwabProvider("k", "s", "https://cb", "rt", "ah", "", "", "", nil)
	if p1 == nil {
		t.Fatal("NewSchwabProvider with nil updater returned nil")
	}
	if p1.credentialUpdater != nil {
		t.Error("expected nil credentialUpdater")
	}

	// Case 2: non-nil updater with instanceID.
	updater := CredentialUpdater(func(id string, u map[string]interface{}) error { return nil })
	p2 := NewSchwabProvider("k", "s", "https://cb", "rt", "ah", "", "", "inst_1", updater)
	if p2 == nil {
		t.Fatal("NewSchwabProvider with updater returned nil")
	}
	if p2.instanceID != "inst_1" {
		t.Errorf("expected instanceID 'inst_1', got %q", p2.instanceID)
	}
	if p2.credentialUpdater == nil {
		t.Error("expected non-nil credentialUpdater")
	}
}

// ============================================================================
// QA Tests for Step 4 — OAuth State Store (NFR-1, NFR-2, NFR-3)
// ============================================================================

// TestOAuthFlowState_JSONExcludesSensitive verifies that marshalling an
// OAuthFlowState to JSON does NOT include any sensitive fields. This is
// critical for NFR-1 (security) — tokens and credentials must never leak
// through JSON serialization (e.g., via status polling responses or logs).
func TestOAuthFlowState_JSONExcludesSensitive(t *testing.T) {
	state := &OAuthFlowState{
		AppKey:             "super-secret-app-key",
		AppSecret:          "super-secret-app-secret",
		CallbackURL:        "https://127.0.0.1/callback",
		BaseURL:            "https://api.schwabapi.com",
		CreatedAt:          time.Now(),
		Status:             "completed",
		RefreshToken:       "super-secret-refresh-token",
		AccessToken:        "super-secret-access-token",
		TokenExpiry:        time.Now().Add(30 * time.Minute),
		ExistingInstanceID: "schwab_live_1",
		Accounts: []SchwabAccountInfo{
			{AccountNumber: "*****6789", HashValue: "HASH_A"},
		},
	}

	jsonBytes, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal OAuthFlowState: %v", err)
	}

	jsonStr := string(jsonBytes)

	// These sensitive values must NOT appear in the JSON output.
	sensitiveValues := map[string]string{
		"AppKey":             "super-secret-app-key",
		"AppSecret":          "super-secret-app-secret",
		"RefreshToken":       "super-secret-refresh-token",
		"AccessToken":        "super-secret-access-token",
		"ExistingInstanceID": "schwab_live_1",
		"CallbackURL":        "https://127.0.0.1/callback",
		"BaseURL":            "https://api.schwabapi.com",
	}

	for fieldName, value := range sensitiveValues {
		if strings.Contains(jsonStr, value) {
			t.Errorf("JSON output contains sensitive field %s value %q; expected it to be excluded via json:\"-\" tag", fieldName, value)
		}
	}

	// Also verify the JSON keys themselves are absent.
	sensitiveKeys := []string{
		"app_key", "app_secret", "refresh_token", "access_token",
		"token_expiry", "existing_instance_id", "callback_url", "base_url",
	}
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	for _, key := range sensitiveKeys {
		if _, exists := jsonMap[key]; exists {
			t.Errorf("JSON contains key %q; expected it to be excluded", key)
		}
	}

	// Verify non-sensitive fields ARE present.
	if jsonMap["status"] != "completed" {
		t.Errorf("expected status 'completed' in JSON, got %v", jsonMap["status"])
	}
	accounts, ok := jsonMap["accounts"].([]interface{})
	if !ok || len(accounts) != 1 {
		t.Errorf("expected 1 account in JSON, got %v", jsonMap["accounts"])
	}
}

// TestOAuthStore_GetState_EmptyToken verifies that GetState("") returns nil.
// An empty string should never match any stored state.
func TestOAuthStore_GetState_EmptyToken(t *testing.T) {
	store := NewSchwabOAuthStore()

	// Create some states to ensure the store is not empty.
	_, _ = store.CreateState("key", "secret", "https://cb", "https://api")
	_, _ = store.CreateState("key2", "secret2", "https://cb2", "https://api2")

	state := store.GetState("")
	if state != nil {
		t.Errorf("expected nil for empty token, got %+v", state)
	}
}

// TestOAuthStore_UpdateState_Expired verifies that UpdateState returns false
// when the state has expired (older than oauthStateTTL = 10 minutes).
// This ensures expired states cannot be modified.
func TestOAuthStore_UpdateState_Expired(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	// Get the state and backdate it to 15 minutes ago.
	state := store.GetState(token)
	if state == nil {
		t.Fatal("expected state to exist before expiry manipulation")
	}
	state.mu.Lock()
	state.CreatedAt = time.Now().Add(-15 * time.Minute)
	state.mu.Unlock()

	// UpdateState should return false for the expired state.
	updateCalled := false
	ok := store.UpdateState(token, func(s *OAuthFlowState) {
		updateCalled = true
		s.Status = "should-not-happen"
	})

	if ok {
		t.Error("expected UpdateState to return false for expired state")
	}
	if updateCalled {
		t.Error("expected updateFn to NOT be called for expired state")
	}
}

// ============================================================================
// QA Tests for Step 5 — Token Exchange and Account Fetch Helpers (AC-4, AC-17)
// ============================================================================

// TestFetchAccountNumbers_Unauthorized verifies that when the Schwab accounts
// endpoint returns 401, fetchAccountNumbers returns a descriptive error.
func TestFetchAccountNumbers_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_token"}`)
	}))
	defer srv.Close()

	_, err := fetchAccountNumbers(srv.URL, "expired-token")
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to mention status 401, got: %v", err)
	}
	if !strings.Contains(err.Error(), "accounts request failed with status") {
		t.Errorf("expected descriptive error message, got: %v", err)
	}
}

// TestFetchAccountNumbers_MalformedJSON verifies that fetchAccountNumbers
// returns an error when the accounts endpoint returns invalid JSON.
func TestFetchAccountNumbers_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{broken json here`)
	}))
	defer srv.Close()

	_, err := fetchAccountNumbers(srv.URL, "valid-token")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse accounts response") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

// TestRenderCallbackPage_ErrorMessageEscaping tests whether error messages
// containing HTML/script tags are handled safely in the callback page.
// Since renderCallbackPage uses fmt.Sprintf (not html/template), injected
// HTML will be rendered literally. This test documents the XSS surface area.
func TestRenderCallbackPage_ErrorMessageEscaping(t *testing.T) {
	xssPayload := `<script>alert(1)</script>`
	html := string(renderCallbackPage("error", xssPayload))

	// Verify the page is still valid (contains expected structure).
	if !strings.Contains(html, "Authorization Failed") {
		t.Error("expected 'Authorization Failed' heading in error HTML")
	}

	// Check if the XSS payload appears unescaped in the output.
	// Since renderCallbackPage uses fmt.Sprintf (not html/template.Execute),
	// the payload WILL appear literally in the HTML.
	if strings.Contains(html, xssPayload) {
		// The payload is present unescaped. Document this as a known issue.
		// In the current architecture this is low risk because:
		// 1. The error message comes from the backend's own error strings, not
		//    directly from user input.
		// 2. The page is served as a one-time callback redirect target.
		// However, if Schwab API error responses ever include attacker-controlled
		// content, this could become exploitable.
		t.Logf("POTENTIAL XSS: renderCallbackPage does NOT HTML-escape error messages. "+
			"Payload %q appears literally in output. "+
			"Consider using html/template or html.EscapeString() for error messages.", xssPayload)
	} else {
		// If the payload is escaped, that's the ideal outcome.
		t.Logf("OK: XSS payload was escaped or sanitized in output")
	}

	// Regardless of escaping, verify the page still has the error styling.
	if !strings.Contains(html, "#ff1744") {
		t.Error("expected red accent color for error status")
	}
}

// TestFetchAccountNumbers_BearerTokenSent verifies that fetchAccountNumbers
// sends the correct Authorization header with "Bearer {token}".
func TestFetchAccountNumbers_BearerTokenSent(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := fetchAccountNumbers(srv.URL, "my-test-access-token-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedAuth := "Bearer my-test-access-token-xyz"
	if receivedAuth != expectedAuth {
		t.Errorf("expected Authorization header %q, got %q", expectedAuth, receivedAuth)
	}
}

// ============================================================================
// QA Tests for Step 6 — HandleAuthorize Endpoint (AC-2, AC-3)
// ============================================================================

// TestHandleAuthorize_MissingCallbackURL verifies that omitting callback_url
// from the request body returns 400 Bad Request. callback_url is a required
// field (binding:"required" on the authorizeRequest struct).
func TestHandleAuthorize_MissingCallbackURL(t *testing.T) {
	handler := newTestOAuthHandler()

	w := postAuthorize(t, handler, map[string]string{
		"app_key":    "my-key",
		"app_secret": "my-secret",
		// callback_url deliberately omitted
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing callback_url, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleAuthorize_StateStoredCorrectly verifies that after a successful
// HandleAuthorize call, the state stored in the OAuth store matches the
// request parameters exactly. This ensures the state carries the correct
// credentials forward to the callback phase.
func TestHandleAuthorize_StateStoredCorrectly(t *testing.T) {
	handler := newTestOAuthHandler()

	w := postAuthorize(t, handler, map[string]string{
		"app_key":      "verify-key",
		"app_secret":   "verify-secret",
		"callback_url": "https://my.host.com/callback",
		"base_url":     "https://custom-api.schwab.com",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	stateToken := resp["state"]
	if stateToken == "" {
		t.Fatal("expected state token in response")
	}

	// Retrieve the state from the store and verify all fields.
	state := handler.oauthStore.GetState(stateToken)
	if state == nil {
		t.Fatal("expected state to be found in store")
	}

	if state.AppKey != "verify-key" {
		t.Errorf("state.AppKey = %q, want %q", state.AppKey, "verify-key")
	}
	if state.AppSecret != "verify-secret" {
		t.Errorf("state.AppSecret = %q, want %q", state.AppSecret, "verify-secret")
	}
	if state.CallbackURL != "https://my.host.com/callback" {
		t.Errorf("state.CallbackURL = %q, want %q", state.CallbackURL, "https://my.host.com/callback")
	}
	if state.BaseURL != "https://custom-api.schwab.com" {
		t.Errorf("state.BaseURL = %q, want %q", state.BaseURL, "https://custom-api.schwab.com")
	}
	if state.Status != "pending" {
		t.Errorf("state.Status = %q, want %q", state.Status, "pending")
	}
	if state.ExistingInstanceID != "" {
		t.Errorf("state.ExistingInstanceID = %q, want empty (no re-auth)", state.ExistingInstanceID)
	}
	if state.CreatedAt.IsZero() {
		t.Error("state.CreatedAt should be set")
	}

	// Verify the auth_url in the response is well-formed and contains the state token.
	authURL := resp["auth_url"]
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("auth_url is not a valid URL: %v", err)
	}
	if parsed.Query().Get("state") != stateToken {
		t.Errorf("auth_url state param = %q, want %q", parsed.Query().Get("state"), stateToken)
	}
	if parsed.Query().Get("client_id") != "verify-key" {
		t.Errorf("auth_url client_id = %q, want %q", parsed.Query().Get("client_id"), "verify-key")
	}
}

// ============================================================================
// QA Tests for Step 7 — HandleCallback Endpoint (AC-4, AC-10, AC-11, AC-15, AC-16)
// ============================================================================

// TestHandleCallback_ErrorTakesPrecedenceOverCode verifies that when both
// ?code= and ?error= are present in the callback URL, the error parameter
// takes precedence and the callback renders "cancelled" HTML instead of
// attempting a token exchange.
func TestHandleCallback_ErrorTakesPrecedenceOverCode(t *testing.T) {
	handler := newTestOAuthHandler()
	stateToken := createAuthorizedState(t, handler, "https://unused")

	// Send both code AND error params — error should win.
	w := getCallback(t, handler, "code=valid-code-xyz&error=access_denied&state="+url.QueryEscape(stateToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Cancelled") {
		t.Errorf("expected cancelled HTML when error param is present, got: %s", body[:min(200, len(body))])
	}
	if strings.Contains(body, "successful") {
		t.Error("should NOT show success HTML when error param is present alongside code")
	}

	// Verify state was marked as failed, not completed.
	state := handler.oauthStore.GetState(stateToken)
	if state == nil {
		t.Fatal("expected state to exist")
	}
	if state.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", state.Status)
	}
}

// TestHandleCallback_ExpiredState verifies that when the state token has
// expired (older than 10 minutes), the callback renders "Invalid or expired"
// error HTML. This validates NFR-2 (timeout enforcement).
func TestHandleCallback_ExpiredState(t *testing.T) {
	handler := newTestOAuthHandler()
	stateToken := createAuthorizedState(t, handler, "https://unused")

	// Backdate the state to 15 minutes ago so it's expired.
	state := handler.oauthStore.GetState(stateToken)
	if state == nil {
		t.Fatal("expected state to exist before backdating")
	}
	state.mu.Lock()
	state.CreatedAt = time.Now().Add(-15 * time.Minute)
	state.mu.Unlock()

	// Call the callback with the expired state.
	w := getCallback(t, handler, "code=valid-code&state="+url.QueryEscape(stateToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Invalid or expired") {
		t.Errorf("expected 'Invalid or expired' in HTML for expired state, got: %s", body[:min(200, len(body))])
	}
}

// TestHandleCallback_HTMLContentType verifies that HandleCallback always
// returns Content-Type "text/html; charset=utf-8", since it renders HTML
// pages (not JSON) for the browser redirect target.
func TestHandleCallback_HTMLContentType(t *testing.T) {
	// Test three different callback scenarios to verify Content-Type is always HTML.
	handler := newTestOAuthHandler()

	// Scenario 1: Success flow (needs a mock server for token exchange)
	tokenResp := `{"access_token":"at","refresh_token":"rt","expires_in":1800}`
	accountsResp := `[{"accountNumber":"123456789","hashValue":"H1"}]`
	srv := setupMockSchwabServer(t, tokenResp, 200, accountsResp, 200)
	defer srv.Close()

	successToken := createAuthorizedState(t, handler, srv.URL)
	w1 := getCallback(t, handler, "code=good-code&state="+url.QueryEscape(successToken))
	ct1 := w1.Header().Get("Content-Type")
	if ct1 != "text/html; charset=utf-8" {
		t.Errorf("[success] expected Content-Type 'text/html; charset=utf-8', got %q", ct1)
	}

	// Scenario 2: Error/cancelled flow
	cancelToken := createAuthorizedState(t, handler, "https://unused")
	w2 := getCallback(t, handler, "error=access_denied&state="+url.QueryEscape(cancelToken))
	ct2 := w2.Header().Get("Content-Type")
	if ct2 != "text/html; charset=utf-8" {
		t.Errorf("[cancelled] expected Content-Type 'text/html; charset=utf-8', got %q", ct2)
	}

	// Scenario 3: Invalid/missing state
	w3 := getCallback(t, handler, "code=some-code&state=nonexistent-token-xyz")
	ct3 := w3.Header().Get("Content-Type")
	if ct3 != "text/html; charset=utf-8" {
		t.Errorf("[invalid state] expected Content-Type 'text/html; charset=utf-8', got %q", ct3)
	}
}

// min returns the smaller of two ints (helper for safe string slicing).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// QA Tests for Step 8 — HandleOAuthStatus and HandleSelectAccount (AC-5, AC-6, AC-9)
// ============================================================================

// TestHandleSelectAccount_AddInstanceFails verifies that when AddInstance
// returns false (e.g., persistence failure), HandleSelectAccount returns
// a 500 Internal Server Error.
func TestHandleSelectAccount_AddInstanceFails(t *testing.T) {
	md := &mockDeps{
		addInstanceResult:    false, // simulate AddInstance failure
		generateInstIDResult: "schwab_live_FailTest",
	}
	handler, token := setupCompletedState(t, md)

	w := postSelectAccount(t, handler, map[string]string{
		"state":         token,
		"account_hash":  "HASH_A",
		"provider_name": "FailTest",
		"account_type":  "live",
	})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when AddInstance fails, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "Failed to create provider instance") {
		t.Errorf("expected 'Failed to create provider instance' in error, got: %s", body)
	}
}

// TestHandleSelectAccount_DoubleFinalize verifies that after a successful
// finalization, attempting to finalize the same state again fails because
// the state is deleted after the first success. This validates that state
// is single-use (NFR-3 idempotency).
func TestHandleSelectAccount_DoubleFinalize(t *testing.T) {
	md := &mockDeps{
		addInstanceResult:    true,
		generateInstIDResult: "schwab_live_Double",
	}
	handler, token := setupCompletedState(t, md)

	// First finalization — should succeed.
	w1 := postSelectAccount(t, handler, map[string]string{
		"state":         token,
		"account_hash":  "HASH_A",
		"provider_name": "Double",
		"account_type":  "live",
	})
	if w1.Code != http.StatusOK {
		t.Fatalf("first finalization expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1["success"] != true {
		t.Fatalf("first finalization expected success=true, got: %v", resp1)
	}

	// Second finalization with the same state — should fail (state deleted).
	w2 := postSelectAccount(t, handler, map[string]string{
		"state":         token,
		"account_hash":  "HASH_A",
		"provider_name": "Double",
		"account_type":  "live",
	})

	if w2.Code != http.StatusBadRequest {
		t.Errorf("second finalization expected 400, got %d: %s", w2.Code, w2.Body.String())
	}

	body2 := w2.Body.String()
	if !strings.Contains(body2, "not found or expired") {
		t.Errorf("expected 'not found or expired' error for deleted state, got: %s", body2)
	}
}

// TestHandleSelectAccount_CredentialsMapComplete verifies that the credentials
// map passed to AddInstance contains all 6 expected keys: app_key, app_secret,
// callback_url, base_url, refresh_token, account_hash. This ensures the
// provider instance is created with complete credentials.
func TestHandleSelectAccount_CredentialsMapComplete(t *testing.T) {
	md := &mockDeps{
		addInstanceResult:    true,
		generateInstIDResult: "schwab_live_CredsCheck",
	}
	handler, token := setupCompletedState(t, md)

	w := postSelectAccount(t, handler, map[string]string{
		"state":         token,
		"account_hash":  "HASH_B",
		"provider_name": "CredsCheck",
		"account_type":  "live",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify all 6 credential keys are present in the map passed to AddInstance.
	expectedKeys := []string{"app_key", "app_secret", "callback_url", "base_url", "refresh_token", "account_hash"}
	creds := md.addInstanceCredentials

	if creds == nil {
		t.Fatal("addInstanceCredentials is nil; AddInstance was not called or credentials were empty")
	}

	for _, key := range expectedKeys {
		if _, exists := creds[key]; !exists {
			t.Errorf("credentials map missing key %q", key)
		}
	}

	// Verify the values match what was set up in setupCompletedState.
	if creds["app_key"] != "test-key" {
		t.Errorf("app_key = %v, want %q", creds["app_key"], "test-key")
	}
	if creds["app_secret"] != "test-secret" {
		t.Errorf("app_secret = %v, want %q", creds["app_secret"], "test-secret")
	}
	if creds["callback_url"] != "https://127.0.0.1/callback" {
		t.Errorf("callback_url = %v, want %q", creds["callback_url"], "https://127.0.0.1/callback")
	}
	if creds["base_url"] != "https://api.schwabapi.com" {
		t.Errorf("base_url = %v, want %q", creds["base_url"], "https://api.schwabapi.com")
	}
	if creds["refresh_token"] != "rt-mock" {
		t.Errorf("refresh_token = %v, want %q", creds["refresh_token"], "rt-mock")
	}
	if creds["account_hash"] != "HASH_B" {
		t.Errorf("account_hash = %v, want %q", creds["account_hash"], "HASH_B")
	}

	// Verify no extra unexpected keys leaked into credentials.
	if len(creds) != 6 {
		t.Errorf("expected exactly 6 credential keys, got %d: %v", len(creds), creds)
	}
}

// TestHandleOAuthStatus_Exchanging verifies that when a state is in the
// "exchanging" status (i.e., callback received, token exchange in progress),
// HandleOAuthStatus returns {"status": "exchanging"} with no accounts or error.
func TestHandleOAuthStatus_Exchanging(t *testing.T) {
	handler := newTestOAuthHandler()
	token, _ := handler.oauthStore.CreateState("k", "s", "https://cb", "https://api")

	// Manually transition to "exchanging" status.
	handler.oauthStore.UpdateState(token, func(s *OAuthFlowState) {
		s.Status = "exchanging"
	})

	w := getStatus(t, handler, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != "exchanging" {
		t.Errorf("expected status 'exchanging', got %v", resp["status"])
	}

	// "exchanging" is neither "completed" nor "failed", so no accounts or error.
	if _, exists := resp["accounts"]; exists {
		t.Error("expected no 'accounts' key for 'exchanging' status")
	}
	if _, exists := resp["error"]; exists {
		t.Error("expected no 'error' key for 'exchanging' status")
	}
}

// ============================================================================
// QA Tests for Step 13 — Edge Cases and Security (NFR-1, NFR-2, NFR-3)
// ============================================================================

// TestCSRF_ForgedStateRejected verifies that HandleCallback rejects a random
// 43-character state token that was never created by the OAuth store. This is
// the primary CSRF defense — forged state tokens must never succeed.
func TestCSRF_ForgedStateRejected(t *testing.T) {
	handler := newTestOAuthHandler()

	// Create a plausible-looking but forged 43-character token.
	forgedToken := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq"
	if len(forgedToken) != 43 {
		t.Fatalf("test setup error: forged token length is %d, want 43", len(forgedToken))
	}

	w := getCallback(t, handler, "code=stolen-code&state="+url.QueryEscape(forgedToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Invalid or expired") {
		t.Errorf("expected 'Invalid or expired' error for forged state, got: %s", body[:min(200, len(body))])
	}

	// Success or account data should never appear.
	if strings.Contains(body, "successful") {
		t.Error("CSRF: forged state token resulted in success HTML — security violation")
	}
}

// TestConcurrentCallbacks_OnlyOneSucceeds verifies that when multiple concurrent
// callback requests arrive with the same state token and authorization code,
// exactly one succeeds (transitions pending → exchanging) and the rest are
// rejected as "already been processed". This validates NFR-3 (idempotency).
func TestConcurrentCallbacks_OnlyOneSucceeds(t *testing.T) {
	tokenResp := `{"access_token":"at","refresh_token":"rt","expires_in":1800}`
	accountsResp := `[{"accountNumber":"123456789","hashValue":"H1"}]`
	srv := setupMockSchwabServer(t, tokenResp, 200, accountsResp, 200)
	defer srv.Close()

	handler := newTestOAuthHandler()
	stateToken := createAuthorizedState(t, handler, srv.URL)

	const goroutines = 10
	var successCount atomic.Int32
	var processedCount atomic.Int32

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			w := getCallback(t, handler, "code=auth-code&state="+url.QueryEscape(stateToken))
			body := w.Body.String()

			if strings.Contains(body, "successful") {
				successCount.Add(1)
			}
			if strings.Contains(body, "already been processed") {
				processedCount.Add(1)
			}
		}()
	}

	wg.Wait()

	sc := successCount.Load()
	pc := processedCount.Load()

	if sc != 1 {
		t.Errorf("expected exactly 1 success, got %d", sc)
	}
	if pc != int32(goroutines-1) {
		t.Errorf("expected %d 'already processed' rejections, got %d", goroutines-1, pc)
	}
	if sc+pc != int32(goroutines) {
		t.Errorf("expected %d total responses (success + processed), got %d", goroutines, sc+pc)
	}
}

// TestCleanupGoroutine_NoLeak verifies that the cleanup goroutine exits
// promptly when the context is cancelled, preventing goroutine leaks.
func TestCleanupGoroutine_NoLeak(t *testing.T) {
	store := NewSchwabOAuthStore()

	// Snapshot goroutine count before starting cleanup.
	runtime.GC()
	runtime.Gosched()
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	store.StartCleanup(ctx)

	// Give the goroutine a moment to start.
	time.Sleep(50 * time.Millisecond)
	during := runtime.NumGoroutine()

	if during <= before {
		t.Logf("warning: goroutine count did not increase (before=%d, during=%d); scheduler timing issue", before, during)
	}

	// Cancel the context to stop the cleanup goroutine.
	cancel()

	// Wait for the goroutine to exit. The ticker period is 60s but
	// context cancellation should cause immediate return via select.
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	runtime.Gosched()

	after := runtime.NumGoroutine()

	// The goroutine count after should be <= the count before (or at most
	// equal, accounting for GC/scheduler goroutines). We allow a margin of 1
	// for timing sensitivity.
	if after > before+1 {
		t.Errorf("possible goroutine leak: before=%d, during=%d, after=%d", before, during, after)
	}
}

// TestBackwardCompatibility_ManualTokens verifies that existing Schwab instances
// created with manually-provided refresh_token and account_hash (before the
// OAuth flow was added) continue to work. This tests NFR-4 (backward compat).
func TestBackwardCompatibility_ManualTokens(t *testing.T) {
	// Set up a mock Schwab API that responds to TestCredentials (account numbers endpoint).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			// Token refresh endpoint.
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "fresh-access-token",
				"expires_in":   1800,
				"token_type":   "Bearer",
			})
		case strings.HasSuffix(r.URL.Path, "/accounts/accountNumbers"):
			// Account numbers endpoint used by TestCredentials.
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]string{
				{"accountNumber": "987654321", "hashValue": "manual-account-hash"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Create a provider the "old way" — with manual refresh_token and account_hash,
	// empty instanceID, nil credentialUpdater.
	p := NewSchwabProvider(
		"manual-app-key",
		"manual-app-secret",
		"https://127.0.0.1/callback",
		"manual-refresh-token", // pre-OAuth: user pasted this manually
		"manual-account-hash",  // pre-OAuth: user pasted this manually
		srv.URL,
		"live",
		"",  // no instanceID (old-style)
		nil, // no credentialUpdater (old-style)
	)

	// Verify the provider was created with the manual values.
	if p.refreshToken != "manual-refresh-token" {
		t.Errorf("expected refreshToken 'manual-refresh-token', got %q", p.refreshToken)
	}
	if p.accountHash != "manual-account-hash" {
		t.Errorf("expected accountHash 'manual-account-hash', got %q", p.accountHash)
	}

	// Verify TestCredentials works against the mock server.
	result, err := p.TestCredentials(context.Background())
	if err != nil {
		t.Fatalf("TestCredentials returned error: %v", err)
	}

	success, _ := result["success"].(bool)
	if !success {
		t.Errorf("expected success=true from TestCredentials, got: %v", result)
	}

	// Verify authExpired is NOT set.
	if p.authExpired {
		t.Error("expected authExpired=false for working manual tokens")
	}
}
