package schwab

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// OAuth Flow State
// =============================================================================

// OAuthFlowState holds all state for one OAuth authorization flow.
// Sensitive fields are excluded from JSON serialization.
type OAuthFlowState struct {
	// Configuration (set at creation)
	AppKey      string `json:"-"`
	AppSecret   string `json:"-"`
	CallbackURL string `json:"-"`
	BaseURL     string `json:"-"`

	// Flow metadata
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // pending → exchanging → completed → finalized | failed

	// Tokens (set after callback)
	RefreshToken string    `json:"-"`
	AccessToken  string    `json:"-"`
	TokenExpiry  time.Time `json:"-"`

	// Accounts (set after account fetch)
	Accounts []SchwabAccountInfo `json:"accounts,omitempty"`

	// Error (set on failure)
	Error string `json:"error,omitempty"`

	// Re-auth context
	ExistingInstanceID string `json:"-"`

	// Protects state transitions
	mu sync.Mutex `json:"-"` //nolint:structcheck
}

// SchwabAccountInfo represents one Schwab brokerage account returned during OAuth.
type SchwabAccountInfo struct {
	AccountNumber string `json:"account_number"`
	HashValue     string `json:"hash_value"`
}

// =============================================================================
// OAuth State Store
// =============================================================================

const (
	oauthStateTTL      = 10 * time.Minute
	oauthCleanupPeriod = 60 * time.Second
)

// SchwabOAuthStore is a thread-safe in-memory store for OAuth flow states.
// Uses sync.Map for lock-free concurrent reads.
type SchwabOAuthStore struct {
	states sync.Map // map[string]*OAuthFlowState
}

// NewSchwabOAuthStore creates a new OAuth state store.
func NewSchwabOAuthStore() *SchwabOAuthStore {
	return &SchwabOAuthStore{}
}

// CreateState generates a cryptographically random state token and stores
// a new pending OAuth flow state. Returns the token or an error.
func (s *SchwabOAuthStore) CreateState(appKey, appSecret, callbackURL, baseURL string) (string, error) {
	token, err := generateStateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}

	state := &OAuthFlowState{
		AppKey:      appKey,
		AppSecret:   appSecret,
		CallbackURL: callbackURL,
		BaseURL:     baseURL,
		CreatedAt:   time.Now(),
		Status:      "pending",
	}

	s.states.Store(token, state)
	return token, nil
}

// GetState returns the OAuth flow state for the given token, or nil if
// the token is not found or the state has expired (>10 minutes old).
func (s *SchwabOAuthStore) GetState(stateToken string) *OAuthFlowState {
	val, ok := s.states.Load(stateToken)
	if !ok {
		return nil
	}

	state := val.(*OAuthFlowState)

	// Check expiry
	if time.Since(state.CreatedAt) > oauthStateTTL {
		s.states.Delete(stateToken)
		return nil
	}

	return state
}

// UpdateState applies the given function to the state identified by stateToken.
// The state's mutex is held during the call to updateFn.
// Returns false if the state is not found or has expired.
func (s *SchwabOAuthStore) UpdateState(stateToken string, updateFn func(*OAuthFlowState)) bool {
	state := s.GetState(stateToken)
	if state == nil {
		return false
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	updateFn(state)
	return true
}

// DeleteState removes the state entry for the given token.
func (s *SchwabOAuthStore) DeleteState(stateToken string) {
	s.states.Delete(stateToken)
}

// StartCleanup launches a background goroutine that removes expired state
// entries every 60 seconds. The goroutine exits when ctx is cancelled.
func (s *SchwabOAuthStore) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(oauthCleanupPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.removeExpired()
			}
		}
	}()
}

// removeExpired deletes all state entries older than oauthStateTTL.
func (s *SchwabOAuthStore) removeExpired() {
	now := time.Now()
	removed := 0

	s.states.Range(func(key, value interface{}) bool {
		state := value.(*OAuthFlowState)
		if now.Sub(state.CreatedAt) > oauthStateTTL {
			s.states.Delete(key)
			removed++
		}
		return true
	})

	if removed > 0 {
		slog.Debug("cleaned up expired OAuth states", "removed", removed)
	}
}

// =============================================================================
// Token Generation
// =============================================================================

// generateStateToken generates a 32-byte cryptographically random token
// encoded as base64url (no padding). The result is 43 characters long.
func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// =============================================================================
// Token Exchange
// =============================================================================

// tokenExchangeResult holds the parsed result of an authorization code exchange.
type tokenExchangeResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// exchangeCodeForTokens exchanges an OAuth authorization code for access and
// refresh tokens using Schwab's token endpoint.
func exchangeCodeForTokens(baseURL, appKey, appSecret, callbackURL, code string) (*tokenExchangeResult, error) {
	tokenURL := baseURL + "/v1/oauth/token"

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", callbackURL)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(appKey, appSecret)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("invalid authorization code or redirect URI (400): %s", truncateBody(body, 200))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid client credentials (401): %s", truncateBody(body, 200))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, truncateBody(body, 200))
	}

	var tokenResp schwabTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}

	return &tokenExchangeResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}

// =============================================================================
// Account Number Fetch
// =============================================================================

// schwabAccountNumberEntry matches the JSON structure returned by the
// Schwab /accounts/accountNumbers endpoint.
type schwabAccountNumberEntry struct {
	AccountNumber string `json:"accountNumber"`
	HashValue     string `json:"hashValue"`
}

// fetchAccountNumbers retrieves the list of brokerage accounts associated with
// the given access token. Account numbers are masked for display safety.
func fetchAccountNumbers(baseURL, accessToken string) ([]SchwabAccountInfo, error) {
	accountsURL := baseURL + "/trader/v1/accounts/accountNumbers"

	req, err := http.NewRequest(http.MethodGet, accountsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("accounts request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read accounts response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("accounts request failed with status %d: %s", resp.StatusCode, truncateBody(body, 200))
	}

	var entries []schwabAccountNumberEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse accounts response: %w", err)
	}

	if len(entries) == 0 {
		return []SchwabAccountInfo{}, nil
	}

	accounts := make([]SchwabAccountInfo, len(entries))
	for i, entry := range entries {
		accounts[i] = SchwabAccountInfo{
			AccountNumber: maskAccountNumber(entry.AccountNumber),
			HashValue:     entry.HashValue,
		}
	}

	return accounts, nil
}

// =============================================================================
// Helpers
// =============================================================================

// maskAccountNumber returns a masked version of an account number,
// showing only the last 4 digits (e.g., "*****6789").
func maskAccountNumber(number string) string {
	if number == "" {
		return ""
	}
	if len(number) <= 4 {
		return number
	}
	masked := strings.Repeat("*", len(number)-4) + number[len(number)-4:]
	return masked
}

// =============================================================================
// Callback HTML
// =============================================================================

// renderCallbackPage returns a minimal self-contained HTML page displayed
// in the browser after the Schwab OAuth redirect. No external dependencies.
func renderCallbackPage(status, errorMessage string) []byte {
	var icon, heading, message, accentColor string

	switch status {
	case "success":
		icon = "&#10004;" // checkmark
		heading = "Authorization Successful"
		message = "Authorization successful! You may close this tab."
		accentColor = "#00c853"
	case "error":
		icon = "&#10008;" // X mark
		heading = "Authorization Failed"
		message = errorMessage
		accentColor = "#ff1744"
	case "cancelled":
		icon = "&#9888;" // warning triangle
		heading = "Authorization Cancelled"
		message = "The authorization was cancelled."
		accentColor = "#ffc107"
	default:
		icon = "&#9888;"
		heading = "Unknown Status"
		message = status
		accentColor = "#ffc107"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JuicyTrade - Schwab Authorization</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #1a1a2e;
    color: #ffffff;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
  }
  .card {
    text-align: center;
    background: #16213e;
    border-radius: 12px;
    padding: 48px 40px;
    max-width: 480px;
    box-shadow: 0 4px 24px rgba(0,0,0,0.3);
  }
  .icon {
    font-size: 64px;
    color: %s;
    margin-bottom: 16px;
  }
  h1 {
    font-size: 22px;
    margin-bottom: 12px;
    font-weight: 600;
  }
  p {
    font-size: 15px;
    color: #b0b0b0;
    line-height: 1.5;
  }
</style>
</head>
<body>
<div class="card">
  <div class="icon">%s</div>
  <h1>%s</h1>
  <p>%s</p>
</div>
</body>
</html>`, accentColor, icon, heading, message)

	return []byte(html)
}

// =============================================================================
// OAuth Handler
// =============================================================================

// OAuthHandlerDeps holds closure functions that let the handler interact with
// the providers package (CredentialStore, ProviderManager) without creating
// a circular import.
type OAuthHandlerDeps struct {
	AddInstance      func(instanceID, providerType, accountType, displayName string, credentials map[string]interface{}) bool
	UpdateCredFields func(instanceID string, updates map[string]interface{}) error
	GenerateInstID   func(providerType, accountType, displayName string) string
	ReinitInstance   func(instanceID string) error
	GetInstance      func(instanceID string) map[string]interface{}
}

// SchwabOAuthHandler manages the full OAuth authorization flow via HTTP endpoints.
type SchwabOAuthHandler struct {
	oauthStore *SchwabOAuthStore
	deps       OAuthHandlerDeps
}

// NewSchwabOAuthHandler creates a new handler with its own internal OAuthStore.
// Call StartCleanup(ctx) separately to launch the TTL cleanup goroutine.
func NewSchwabOAuthHandler(deps OAuthHandlerDeps) *SchwabOAuthHandler {
	return &SchwabOAuthHandler{
		oauthStore: NewSchwabOAuthStore(),
		deps:       deps,
	}
}

// StartCleanup launches the background goroutine that removes expired OAuth states.
func (h *SchwabOAuthHandler) StartCleanup(ctx context.Context) {
	h.oauthStore.StartCleanup(ctx)
}

// authorizeRequest is the expected JSON body for HandleAuthorize.
type authorizeRequest struct {
	AppKey      string `json:"app_key" binding:"required"`
	AppSecret   string `json:"app_secret" binding:"required"`
	CallbackURL string `json:"callback_url" binding:"required"`
	BaseURL     string `json:"base_url"`
	InstanceID  string `json:"instance_id"`
}

// HandleAuthorize initiates a Schwab OAuth flow. It creates a state token,
// builds the Schwab authorization URL, and returns it to the caller.
//
// POST /api/schwab/oauth/authorize
//
//	Request:  { "app_key": "...", "app_secret": "...", "callback_url": "...", "base_url": "...", "instance_id": "..." }
//	Response: { "auth_url": "...", "state": "..." }
func (h *SchwabOAuthHandler) HandleAuthorize(c *gin.Context) {
	var req authorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate callback_url starts with https://
	if !strings.HasPrefix(req.CallbackURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "callback_url must start with https://"})
		return
	}

	// Default base_url
	if req.BaseURL == "" {
		req.BaseURL = "https://api.schwabapi.com"
	}

	// Create state
	stateToken, err := h.oauthStore.CreateState(req.AppKey, req.AppSecret, req.CallbackURL, req.BaseURL)
	if err != nil {
		slog.Error("failed to create OAuth state", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create authorization state"})
		return
	}

	// If re-auth, store the existing instance ID
	if req.InstanceID != "" {
		h.oauthStore.UpdateState(stateToken, func(s *OAuthFlowState) {
			s.ExistingInstanceID = req.InstanceID
		})
	}

	// Build Schwab authorization URL
	authURL := fmt.Sprintf("%s/v1/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		req.BaseURL,
		url.QueryEscape(req.AppKey),
		url.QueryEscape(req.CallbackURL),
		url.QueryEscape(stateToken),
	)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    stateToken,
	})
}
