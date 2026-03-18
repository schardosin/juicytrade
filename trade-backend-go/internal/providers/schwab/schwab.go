package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"

	"github.com/gorilla/websocket"
)

// CredentialUpdater is a callback function that the provider uses to persist
// credential changes back to the credential store. Injected at construction time.
// Pass nil in tests or when persistence is not needed.
type CredentialUpdater func(instanceID string, updates map[string]interface{}) error

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

	// --- Instance identity and credential persistence ---
	instanceID        string            // Provider instance ID (e.g., "schwab_live_MyAccount")
	credentialUpdater CredentialUpdater // Callback to persist credential changes (may be nil)

	// --- Auth health status for re-authentication signaling ---
	authExpired bool // Set to true when refresh token is confirmed expired

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
func NewSchwabProvider(appKey, appSecret, callbackURL, refreshToken, accountHash, baseURL, accountType string,
	instanceID string,
	credentialUpdater CredentialUpdater,
) *SchwabProvider {
	// Apply defaults
	if baseURL == "" {
		baseURL = "https://api.schwabapi.com"
	}
	if accountType == "" {
		accountType = "live"
	}

	provider := &SchwabProvider{
		BaseProviderImpl:  base.NewBaseProvider("Schwab"),
		appKey:            appKey,
		appSecret:         appSecret,
		callbackURL:       callbackURL,
		refreshToken:      refreshToken,
		accountHash:       accountHash,
		baseURL:           baseURL,
		accountType:       accountType,
		instanceID:        instanceID,
		credentialUpdater: credentialUpdater,
		logger:            slog.Default().With("provider", "schwab"),
		rateLimiter:       newRateLimiter(120, 2.0),
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

// PreviewOrder previews a trading order by calling the Schwab previewOrder API.
//
// Endpoint: POST /trader/v1/accounts/{accountHash}/previewOrder
// The request body is the same format as placeOrder. The response includes
// commission/fee breakdowns, order cost, buying power effect, and validation results.
func (s *SchwabProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	// 1. Build the order request — try multi-leg if "legs" are present
	var req *schwabOrderRequest
	var buildErr error

	if legs, ok := orderData["legs"]; ok {
		if legsArr, ok := legs.([]interface{}); ok && len(legsArr) > 0 {
			req, buildErr = buildSchwabMultiLegOrderRequest(orderData)
		} else {
			req, buildErr = buildSchwabOrderRequest(orderData)
		}
	} else {
		req, buildErr = buildSchwabOrderRequest(orderData)
	}

	if buildErr != nil {
		return errorPreviewResult(fmt.Sprintf("Failed to build order: %s", buildErr.Error())), nil
	}

	// 2. Marshal to JSON
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return errorPreviewResult(fmt.Sprintf("Failed to marshal order: %s", err.Error())), nil
	}

	// 3. Call the Schwab preview endpoint
	reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "/previewOrder")
	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodPost, reqURL, jsonBody)

	// 4. Handle transport/auth errors — return structured error, not a Go error
	if err != nil {
		return errorPreviewResult(err.Error()), nil
	}

	// 5. Parse the successful response
	return parsePreviewResponse(body)
}

// parsePreviewResponse parses the Schwab previewOrder API response into the
// standardized preview result map.
//
// Response schema (from Schwab official docs):
//
//	{
//	  "orderStrategy": {
//	    "orderBalance": {
//	      "orderValue": 15000.00,
//	      "projectedBuyingPower": 5000.00,
//	      "projectedCommission": 0.65
//	    }
//	  },
//	  "orderValidationResult": {
//	    "rejects": [{"activityMessage": "...", "originalSeverity": "REJECT"}],
//	    "warns": [{"activityMessage": "...", "originalSeverity": "WARN"}],
//	    "alerts": [{"activityMessage": "...", "originalSeverity": "ALERT"}]
//	  },
//	  "commissionAndFee": {
//	    "commission": {"commissionLegs": [{"commissionValues": [{"value": 0.65}]}]},
//	    "fee": {"feeLegs": [{"feeValues": [{"value": 0.02}]}]}
//	  }
//	}
func parsePreviewResponse(body []byte) (map[string]interface{}, error) {
	if len(body) == 0 {
		return okPreviewResult(0, 0, 0, 0), nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return errorPreviewResult(fmt.Sprintf("Failed to parse preview response: %s", err.Error())), nil
	}

	// Check for validation rejects first — if present, the order is invalid
	if validationResult, ok := raw["orderValidationResult"].(map[string]interface{}); ok {
		if rejects, ok := validationResult["rejects"].([]interface{}); ok && len(rejects) > 0 {
			var messages []string
			for _, r := range rejects {
				if rMap, ok := r.(map[string]interface{}); ok {
					// Try "activityMessage" first (actual Schwab API), fall back to "message"
					if msg, ok := rMap["activityMessage"].(string); ok && msg != "" {
						messages = append(messages, msg)
					} else if msg, ok := rMap["message"].(string); ok && msg != "" {
						messages = append(messages, msg)
					}
				}
			}
			if len(messages) > 0 {
				result := errorPreviewResult(strings.Join(messages, "; "))
				result["validation_errors"] = messages
				return result, nil
			}
		}
	}

	// Extract commission from commissionAndFee.commission.commissionLegs[].commissionValues[].value
	commission := sumNestedFeeValues(raw, "commission", "commissionLegs", "commissionValues")

	// Extract fees from commissionAndFee.fee.feeLegs[].feeValues[].value
	fees := sumNestedFeeValues(raw, "fee", "feeLegs", "feeValues")

	// Extract order cost and buying power from orderStrategy.orderBalance
	orderCost := 0.0
	buyingPowerEffect := 0.0
	if orderStrategy, ok := raw["orderStrategy"].(map[string]interface{}); ok {
		if orderBalance, ok := orderStrategy["orderBalance"].(map[string]interface{}); ok {
			if v, ok := orderBalance["orderValue"].(float64); ok {
				orderCost = v
			}
			if v, ok := orderBalance["projectedBuyingPower"].(float64); ok {
				buyingPowerEffect = v
			}
		}
		// Also try orderValue at the orderStrategy level as fallback
		if orderCost == 0 {
			if v, ok := orderStrategy["orderValue"].(float64); ok {
				orderCost = v
			}
		}
	}

	estimatedTotal := orderCost + commission + fees

	result := okPreviewResult(commission, fees, orderCost, estimatedTotal)
	result["buying_power_effect"] = buyingPowerEffect
	return result, nil
}

// sumNestedFeeValues extracts and sums fee/commission values from the deeply nested
// Schwab commissionAndFee structure.
//
// Path: commissionAndFee.{feeType}.{legsKey}[].{valuesKey}[].value
// Example: commissionAndFee.commission.commissionLegs[].commissionValues[].value
func sumNestedFeeValues(raw map[string]interface{}, feeType, legsKey, valuesKey string) float64 {
	total := 0.0

	commAndFee, ok := raw["commissionAndFee"].(map[string]interface{})
	if !ok {
		return total
	}
	feeObj, ok := commAndFee[feeType].(map[string]interface{})
	if !ok {
		return total
	}
	legs, ok := feeObj[legsKey].([]interface{})
	if !ok {
		return total
	}
	for _, leg := range legs {
		legMap, ok := leg.(map[string]interface{})
		if !ok {
			continue
		}
		vals, ok := legMap[valuesKey].([]interface{})
		if !ok {
			continue
		}
		for _, val := range vals {
			valMap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}
			if v, ok := valMap["value"].(float64); ok {
				total += v
			}
		}
	}

	return total
}

// errorPreviewResult returns a standardized error preview result map.
// This is used when the preview fails at any stage (build, marshal, API call, parse).
func errorPreviewResult(message string) map[string]interface{} {
	return map[string]interface{}{
		"status":              "error",
		"validation_errors":   []string{message},
		"commission":          0.0,
		"cost":                0.0,
		"fees":                0.0,
		"order_cost":          0.0,
		"margin_change":       0.0,
		"buying_power_effect": 0.0,
		"day_trades":          0,
		"estimated_total":     0.0,
	}
}

// okPreviewResult returns a standardized success preview result map.
func okPreviewResult(commission, fees, orderCost, estimatedTotal float64) map[string]interface{} {
	return map[string]interface{}{
		"status":              "ok",
		"commission":          commission,
		"cost":                orderCost,
		"fees":                fees,
		"order_cost":          orderCost,
		"margin_change":       0.0,
		"buying_power_effect": 0.0,
		"day_trades":          0,
		"estimated_total":     estimatedTotal,
		"validation_errors":   []string{},
	}
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
	// Step 0: If auth is already known to be expired, short-circuit
	if s.authExpired {
		return map[string]interface{}{
			"success":      false,
			"message":      "Refresh token expired. Please reconnect to Schwab.",
			"auth_expired": true,
		}, nil
	}

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
