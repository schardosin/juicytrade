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
// Market Data Methods (stubs — implemented methods are in market_data.go)
// =============================================================================

// GetNextMarketDate gets the next trading date.
func (s *SchwabProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	return "", fmt.Errorf("schwab: GetNextMarketDate not yet implemented")
}

// LookupSymbols searches for symbols matching the query.
func (s *SchwabProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	return nil, fmt.Errorf("schwab: LookupSymbols not yet implemented")
}

// GetHistoricalBars gets historical OHLCV bars for charting.
func (s *SchwabProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	return nil, fmt.Errorf("schwab: GetHistoricalBars not yet implemented")
}

// =============================================================================
// Account & Portfolio Methods
// =============================================================================

// GetPositions gets all current positions.
func (s *SchwabProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	return nil, fmt.Errorf("schwab: GetPositions not yet implemented")
}

// GetPositionsEnhanced gets enhanced positions with hierarchical grouping.
func (s *SchwabProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	return nil, fmt.Errorf("schwab: GetPositionsEnhanced not yet implemented")
}

// GetOrders gets orders with optional status filter.
func (s *SchwabProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	return nil, fmt.Errorf("schwab: GetOrders not yet implemented")
}

// GetAccount gets account information including balance and buying power.
func (s *SchwabProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	return nil, fmt.Errorf("schwab: GetAccount not yet implemented")
}

// =============================================================================
// Order Management Methods
// =============================================================================

// PlaceOrder places a trading order.
func (s *SchwabProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, fmt.Errorf("schwab: PlaceOrder not yet implemented")
}

// PlaceMultiLegOrder places a multi-leg trading order.
func (s *SchwabProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, fmt.Errorf("schwab: PlaceMultiLegOrder not yet implemented")
}

// PreviewOrder previews a trading order to get cost estimates and validation.
func (s *SchwabProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("schwab: order preview not supported by Schwab API")
}

// CancelOrder cancels an existing order.
func (s *SchwabProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	return false, fmt.Errorf("schwab: CancelOrder not yet implemented")
}

// =============================================================================
// Streaming Methods
// =============================================================================

// ConnectStreaming connects to the Schwab streaming service via WebSocket.
func (s *SchwabProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("schwab: ConnectStreaming not yet implemented")
}

// DisconnectStreaming disconnects from the Schwab streaming service.
func (s *SchwabProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("schwab: DisconnectStreaming not yet implemented")
}

// SubscribeToSymbols subscribes to real-time data for symbols.
func (s *SchwabProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	return false, fmt.Errorf("schwab: SubscribeToSymbols not yet implemented")
}

// UnsubscribeFromSymbols unsubscribes from real-time data for symbols.
func (s *SchwabProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	return false, fmt.Errorf("schwab: UnsubscribeFromSymbols not yet implemented")
}

// =============================================================================
// Account Event Streaming Methods
// =============================================================================

// StartAccountStream starts the account events stream for real-time order updates.
// Schwab uses the same WebSocket for account activity and market data.
func (s *SchwabProvider) StartAccountStream(ctx context.Context) error {
	return fmt.Errorf("schwab: StartAccountStream not yet implemented")
}

// StopAccountStream stops the account events stream.
func (s *SchwabProvider) StopAccountStream() {
	s.logger.Info("schwab: StopAccountStream not yet implemented")
}

// SetOrderEventCallback sets the callback for receiving order events.
func (s *SchwabProvider) SetOrderEventCallback(callback func(*models.OrderEvent)) {
	s.acctStreamMu.Lock()
	defer s.acctStreamMu.Unlock()
	s.orderEventCallback = callback
}

// IsAccountStreamConnected checks if account stream is connected.
func (s *SchwabProvider) IsAccountStreamConnected() bool {
	s.acctStreamMu.RLock()
	defer s.acctStreamMu.RUnlock()
	return s.acctStreamActive && s.IsConnected
}

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
