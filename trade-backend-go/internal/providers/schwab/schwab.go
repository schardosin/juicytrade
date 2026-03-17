package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"

	"github.com/gorilla/websocket"
)

// Compile-time interface check: SchwabProvider must implement base.Provider.
var _ base.Provider = (*SchwabProvider)(nil)

// SchwabProvider implements the base.Provider interface for Charles Schwab
// (formerly TD Ameritrade) brokerage accounts.
type SchwabProvider struct {
	*base.BaseProviderImpl

	// --- Configuration (immutable after construction) ---
	appKey       string // Schwab Developer Portal App Key (client_id)
	appSecret    string // Schwab Developer Portal Secret (client_secret)
	callbackURL  string // OAuth redirect URI
	refreshToken string // OAuth2 refresh token (long-lived, ~7 days)
	accountHash  string // Schwab account hash (not raw account number)
	baseURL      string // API base URL (production or sandbox)
	accountType  string // "live" or "paper"

	// --- OAuth Token State (protected by tokenMu) ---
	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time

	// --- Rate Limiter ---
	rateLimiter *rateLimiter

	// --- Market Data Streaming State (protected by streamMu) ---
	streamMu         sync.RWMutex
	streamConn       *websocket.Conn
	streamCustomerID string // SchwabClientCustomerId from user preferences
	streamCorrelID   string // SchwabClientCorrelId from user preferences
	streamSocketURL  string // WebSocket URL from user preferences
	streamRequestID  int    // Auto-incrementing request ID for stream messages
	streamStopChan   chan struct{}
	streamDoneChan   chan struct{}

	// --- Account Event Streaming State (protected by acctStreamMu) ---
	acctStreamMu       sync.RWMutex
	acctStreamActive   bool
	orderEventCallback func(*models.OrderEvent)

	// --- Logger ---
	logger *slog.Logger
}

// NewSchwabProvider creates a new Schwab provider instance.
// Follows the same constructor pattern as TastyTrade/Tradier providers.
func NewSchwabProvider(appKey, appSecret, callbackURL, refreshToken, accountHash, baseURL, accountType string) *SchwabProvider {
	// Apply defaults
	if baseURL == "" {
		baseURL = "https://api.schwabapi.com"
	}
	if accountType == "" {
		accountType = "live"
	}

	provider := &SchwabProvider{
		BaseProviderImpl: base.NewBaseProvider("Schwab"),
		appKey:           appKey,
		appSecret:        appSecret,
		callbackURL:      callbackURL,
		refreshToken:     refreshToken,
		accountHash:      accountHash,
		baseURL:          baseURL,
		accountType:      accountType,
		logger:           slog.Default().With("provider", "schwab"),
		rateLimiter:      newRateLimiter(120, 2.0),
	}

	return provider
}

// =============================================================================
// Account & Portfolio Methods (implemented in account.go)
// Orders: GetOrders, CancelOrder implemented in orders.go
// =============================================================================

// =============================================================================
// Order Management Methods (PlaceOrder, PlaceMultiLegOrder in orders.go; CancelOrder in orders.go)
// =============================================================================

// PreviewOrder previews a trading order to get cost estimates and validation.
func (s *SchwabProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("schwab: order preview not supported by Schwab API")
}

// =============================================================================
// Streaming Methods (ConnectStreaming, DisconnectStreaming in streaming.go)
// =============================================================================

// SubscribeToSymbols subscribes to real-time data for symbols.
// Implemented in streaming.go.

// UnsubscribeFromSymbols unsubscribes from real-time data for symbols.
// Implemented in streaming.go.

// =============================================================================
// Account Event Streaming Methods (implemented in account_stream.go)
// =============================================================================

// =============================================================================
// Utility Methods
// =============================================================================

// TestCredentials validates credentials by refreshing the token and verifying
// the account hash against the Schwab accountNumbers endpoint.
func (s *SchwabProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	// Step 1: Attempt token refresh
	if err := s.ensureValidToken(); err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Authentication failed: %s", err.Error()),
		}, nil
	}

	// Step 2: Fetch account numbers to verify account hash
	url := s.buildTraderURL("/accounts/accountNumbers")
	body, statusCode, err := s.doAuthenticatedRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to fetch account numbers: %s", err.Error()),
		}, nil
	}

	if statusCode != http.StatusOK {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Schwab API returned status %d when fetching account numbers", statusCode),
		}, nil
	}

	// Step 3: Parse response — array of {accountNumber, hashValue}
	var accounts []struct {
		AccountNumber string `json:"accountNumber"`
		HashValue     string `json:"hashValue"`
	}
	if err := json.Unmarshal(body, &accounts); err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to parse account numbers response: %s", err.Error()),
		}, nil
	}

	if len(accounts) == 0 {
		return map[string]interface{}{
			"success": false,
			"message": "No accounts found for these credentials",
		}, nil
	}

	// Step 4: Verify account hash matches
	hashFound := false
	for _, acct := range accounts {
		if acct.HashValue == s.accountHash {
			hashFound = true
			break
		}
	}

	if !hashFound {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Account hash %q not found among %d accounts returned by Schwab", truncateToken(s.accountHash), len(accounts)),
		}, nil
	}

	// Success — add sandbox warning for paper accounts
	message := "Schwab credentials validated successfully"
	if s.accountType == "paper" {
		message = "Schwab sandbox credentials validated. Note: The Schwab sandbox has known limitations — some order types may be rejected and behavior may be inconsistent with production."
	}

	s.logger.Info("credentials validated", "account_count", len(accounts))

	return map[string]interface{}{
		"success": true,
		"message": message,
	}, nil
}

// HealthCheck performs a health check on the provider.
func (s *SchwabProvider) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	return s.BaseProviderImpl.HealthCheck(ctx)
}

// Ping verifies connection health by ensuring the token is valid.
func (s *SchwabProvider) Ping(ctx context.Context) error {
	return s.ensureValidToken()
}
