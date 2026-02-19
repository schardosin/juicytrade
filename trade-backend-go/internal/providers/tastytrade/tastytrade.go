package tastytrade

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/utils"

	"github.com/gorilla/websocket"
)

// weeklyMap maps index symbols to their weekly option root symbols
// This handles cases where weekly options use different root symbols than the underlying
var weeklyMap = map[string]string{
	"SPX": "SPXW",
	"NDX": "NDXP",
	"RUT": "RUTW",
	"VIX": "VIXW",
}

// weeklyRootSymbols is the reverse of weeklyMap — maps weekly root symbols back to their parent.
// Used to detect that e.g. NDXP is a weekly variant of NDX.
var weeklyRootSymbols = func() map[string]string {
	m := make(map[string]string, len(weeklyMap))
	for parent, weekly := range weeklyMap {
		m[weekly] = parent
	}
	return m
}()

// TastyTradeProvider implements the Provider interface for TastyTrade.
// Exact conversion of Python TastyTradeProvider class.
type TastyTradeProvider struct {
	*base.BaseProviderImpl
	accountID      string
	baseURL        string
	clientID       string
	clientSecret   string
	refreshToken   string
	authCode       string
	redirectURI    string
	sessionToken   string
	sessionExpires *time.Time
	quoteToken     string
	quoteExpires   *time.Time
	dxlinkURL      string
	httpClient     *utils.HTTPClient
	streamingState *StreamingState
	// Account Streamer for order events (separate from DXLink for market data)
	accountStreamConn     *websocket.Conn
	accountStreamLock     sync.Mutex
	accountStreamStopChan chan struct{}
	accountStreamDoneChan chan struct{}
	accountStreamURL      string
}

// NewTastyTradeProvider creates a new TastyTrade provider instance.
// Exact conversion of Python TastyTradeProvider.__init__ method.
// accountStreamURL: WebSocket URL for account/order events streaming.
//   - Live: wss://streamer.tastyworks.com
//   - Paper/Sandbox: wss://streamer.cert.tastyworks.com
func NewTastyTradeProvider(accountID, baseURL, clientID, clientSecret, refreshToken, authCode, redirectURI, accountStreamURL string) *TastyTradeProvider {
	// Default to live account stream URL if not provided
	if accountStreamURL == "" {
		accountStreamURL = "wss://streamer.tastyworks.com"
	}

	provider := &TastyTradeProvider{
		BaseProviderImpl: base.NewBaseProvider("TastyTrade"),
		accountID:        accountID,
		baseURL:          baseURL,
		clientID:         clientID,
		clientSecret:     clientSecret,
		refreshToken:     refreshToken,
		authCode:         authCode,
		redirectURI:      redirectURI,
		httpClient:       utils.NewHTTPClient(),
		// Account Streamer for order events - URL varies by environment (live vs paper)
		accountStreamURL:      accountStreamURL,
		accountStreamStopChan: make(chan struct{}),
		accountStreamDoneChan: make(chan struct{}),
	}

	// Initialize streaming state
	provider.initStreamingState()

	return provider
}

// createSession creates or refreshes OAuth2 access token with TastyTrade API.
// Exact conversion of Python _create_session method.
func (p *TastyTradeProvider) createSession(ctx context.Context) error {
	url := fmt.Sprintf("%s/oauth/token", p.baseURL)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
		"User-Agent":   "juicytrade/1.0",
	}

	var payload map[string]interface{}

	// Use refresh_token if available
	if p.clientSecret != "" && p.refreshToken != "" {
		slog.Info("TastyTrade: Creating session using refresh_token")
		payload = map[string]interface{}{
			"grant_type":    "refresh_token",
			"refresh_token": p.refreshToken,
			"client_secret": p.clientSecret,
		}
		if p.clientID != "" {
			payload["client_id"] = p.clientID
		}
	} else if p.clientSecret != "" && p.authCode != "" {
		slog.Info("TastyTrade: Creating session using authorization_code")
		// Exchange authorization_code for tokens
		payload = map[string]interface{}{
			"grant_type":    "authorization_code",
			"code":          p.authCode,
			"client_secret": p.clientSecret,
		}
		if p.clientID != "" {
			payload["client_id"] = p.clientID
		}
		if p.redirectURI != "" {
			payload["redirect_uri"] = p.redirectURI
		}
	} else {
		slog.Error("TastyTrade: OAuth2 credentials incomplete",
			"has_client_secret", p.clientSecret != "",
			"has_refresh_token", p.refreshToken != "",
			"has_auth_code", p.authCode != "")
		return fmt.Errorf("OAuth2 credentials incomplete: provide refresh_token or authorization_code with client_secret")
	}

	response, err := p.httpClient.Post(ctx, url, payload, headers)
	if err != nil {
		slog.Error("TastyTrade: OAuth2 token request failed", "error", err)
		return fmt.Errorf("OAuth2 token request failed: %w", err)
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.Unmarshal(response.Body, &tokenResponse); err != nil {
		slog.Error("TastyTrade: Failed to parse OAuth2 response", "error", err, "body", string(response.Body))
		return fmt.Errorf("failed to parse OAuth2 response: %w", err)
	}

	if tokenResponse.AccessToken == "" {
		slog.Error("TastyTrade: No access token in OAuth2 response", "response", string(response.Body))
		return fmt.Errorf("no access token in OAuth2 response")
	}

	p.sessionToken = fmt.Sprintf("Bearer %s", tokenResponse.AccessToken)
	expiresIn := time.Duration(tokenResponse.ExpiresIn) * time.Second
	if expiresIn == 0 {
		expiresIn = 15 * time.Minute // Default 15 minutes
	}
	expires := time.Now().Add(expiresIn)
	p.sessionExpires = &expires

	// Update refresh token if provided
	if tokenResponse.RefreshToken != "" {
		slog.Info("TastyTrade: OAuth2 refresh token updated")
		p.refreshToken = tokenResponse.RefreshToken
	}

	slog.Info("TastyTrade: OAuth2 access token obtained successfully", "expires_in_seconds", tokenResponse.ExpiresIn)
	return nil
}

// ensureValidSession ensures we have a valid session token.
// Exact conversion of Python _ensure_valid_session method.
func (p *TastyTradeProvider) ensureValidSession(ctx context.Context) error {
	if p.sessionToken == "" || p.sessionExpires == nil {
		slog.Info("TastyTrade: No session token, creating new session")
		return p.createSession(ctx)
	}

	// Check if session is about to expire (refresh 5 minutes early)
	if time.Now().After(p.sessionExpires.Add(-5 * time.Minute)) {
		timeUntilExpiry := time.Until(*p.sessionExpires)
		slog.Info("TastyTrade: Session token expiring soon, refreshing",
			"time_until_expiry", timeUntilExpiry.String())
		return p.createSession(ctx)
	}

	timeUntilExpiry := time.Until(*p.sessionExpires)
	slog.Debug("TastyTrade: Session token valid", "time_until_expiry", timeUntilExpiry.String())
	return nil
}

// makeAuthenticatedRequest makes an authenticated request to TastyTrade API.
// Exact conversion of Python _make_authenticated_request method.
func (p *TastyTradeProvider) makeAuthenticatedRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	if err := p.ensureValidSession(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with TastyTrade: %w", err)
	}

	url := fmt.Sprintf("%s%s", p.baseURL, endpoint)
	headers := map[string]string{
		"Authorization": p.sessionToken,
		"Accept":        "application/json",
		"User-Agent":    "juicytrade/1.0",
	}

	var response *utils.Response
	var err error

	switch strings.ToUpper(method) {
	case "GET":
		response, err = p.httpClient.Get(ctx, url, headers, nil)
	case "POST":
		headers["Content-Type"] = "application/json"
		response, err = p.httpClient.Post(ctx, url, body, headers)
	case "DELETE":
		response, err = p.httpClient.Delete(ctx, url, headers)
	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	// If an error occurred but we have a response (HTTP >= 400), return the body as well
	if err != nil {
		if response != nil {
			slog.Debug("TastyTrade: makeAuthenticatedRequest returning response body with error", "statusCode", response.StatusCode, "bodyLength", len(response.Body), "error", err)
			// Return the response body so callers (like PreviewOrder) can parse validation/error details.
			return response.Body, fmt.Errorf("HTTP %d: %s", response.StatusCode, string(response.Body))
		}
		slog.Debug("TastyTrade: makeAuthenticatedRequest returning nil body with error", "error", err)
		return nil, err
	}

	return response.Body, nil
}

// === Market Data Methods ===

// GetStockQuote gets the latest stock quote for a symbol using streaming (DXLink).
// Subscribes to both Quote (bid/ask) and Trade (last price) events for complete data.
// This is especially important for indices like NDX where bid/ask may be unavailable.
func (p *TastyTradeProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	// Ensure streaming connection is healthy
	if !p.ensureHealthyConnection(ctx) {
		return nil, fmt.Errorf("streaming connection not available for quotes")
	}

	// Prepare one-shot channels to receive quote and trade data
	quoteCh := make(chan map[string]interface{}, 1)
	tradeCh := make(chan map[string]interface{}, 1)

	p.streamingState.requestsLock.Lock()
	p.streamingState.quoteRequests[symbol] = quoteCh
	p.streamingState.tradeRequests[symbol] = tradeCh
	p.streamingState.requestsLock.Unlock()

	// Ensure cleanup of request registration
	defer func() {
		p.streamingState.requestsLock.Lock()
		delete(p.streamingState.quoteRequests, symbol)
		delete(p.streamingState.tradeRequests, symbol)
		p.streamingState.requestsLock.Unlock()
	}()

	// Subscribe to both Quote and Trade for the symbol
	// Trade events provide the last trade price, which is essential for indices like NDX
	success, err := p.SubscribeToSymbols(ctx, []string{symbol}, []string{"Quote", "Trade"})
	if err != nil || !success {
		return nil, fmt.Errorf("failed to subscribe to quote/trade for %s: %v", symbol, err)
	}

	// Always unsubscribe before returning
	defer func() {
		p.UnsubscribeFromSymbols(ctx, []string{symbol}, []string{"Quote", "Trade"})
	}()

	// Wait for quote and trade data with timeout
	// We wait for both but don't require both - either can provide useful data
	timeout := 5 * time.Second
	deadline := time.Now().Add(timeout)

	var quoteData, tradeData map[string]interface{}
	gotQuote, gotTrade := false, false

	// Wait until we have both or timeout
	for !gotQuote || !gotTrade {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		select {
		case q := <-quoteCh:
			quoteData = q
			gotQuote = true
		case t := <-tradeCh:
			tradeData = t
			gotTrade = true
		case <-time.After(remaining):
			// Timeout reached
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Build StockQuote from received data
	var bidPtr, askPtr, lastPtr *float64

	// Extract bid/ask from Quote data
	if quoteData != nil {
		if b, ok := quoteData["bid"].(float64); ok && b > 0 {
			bidVal := b
			bidPtr = &bidVal
		}
		if a, ok := quoteData["ask"].(float64); ok && a > 0 {
			askVal := a
			askPtr = &askVal
		}
	}

	// Extract last price from Trade data
	if tradeData != nil {
		if last, ok := tradeData["last"].(float64); ok && last > 0 {
			lastVal := last
			lastPtr = &lastVal
		}
	}

	slog.Debug("GetStockQuote completed",
		"symbol", symbol,
		"hasBid", bidPtr != nil,
		"hasAsk", askPtr != nil,
		"hasLast", lastPtr != nil)

	return &models.StockQuote{
		Symbol:    symbol,
		Bid:       bidPtr,
		Ask:       askPtr,
		Last:      lastPtr,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// GetStockQuotes gets stock quotes for multiple symbols using batch streaming.
// Optimized version that subscribes to all symbols at once instead of one by one.
// Subscribes to both Quote (bid/ask) and Trade (last price) events for complete data.
func (p *TastyTradeProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	if len(symbols) == 0 {
		return make(map[string]*models.StockQuote), nil
	}

	// Ensure streaming connection is healthy
	if !p.ensureHealthyConnection(ctx) {
		return make(map[string]*models.StockQuote), fmt.Errorf("streaming connection not available for quotes")
	}

	// Create channels for each symbol to receive quotes and trades
	quoteFutures := make(map[string]chan map[string]interface{})
	tradeFutures := make(map[string]chan map[string]interface{})

	p.streamingState.requestsLock.Lock()
	for _, symbol := range symbols {
		quoteFutures[symbol] = make(chan map[string]interface{}, 1)
		tradeFutures[symbol] = make(chan map[string]interface{}, 1)
		p.streamingState.quoteRequests[symbol] = quoteFutures[symbol]
		p.streamingState.tradeRequests[symbol] = tradeFutures[symbol]
	}
	p.streamingState.requestsLock.Unlock()

	// Subscribe to both Quote and Trade data for all symbols at once
	success, err := p.SubscribeToSymbols(ctx, symbols, []string{"Quote", "Trade"})
	if err != nil || !success {
		// Clean up futures
		p.streamingState.requestsLock.Lock()
		for _, symbol := range symbols {
			delete(p.streamingState.quoteRequests, symbol)
			delete(p.streamingState.tradeRequests, symbol)
		}
		p.streamingState.requestsLock.Unlock()
		return make(map[string]*models.StockQuote), fmt.Errorf("failed to subscribe to quotes/trades: %v", err)
	}

	// Track received data per symbol
	type symbolData struct {
		bid      *float64
		ask      *float64
		last     *float64
		gotQuote bool
		gotTrade bool
	}
	dataMap := make(map[string]*symbolData)
	for _, symbol := range symbols {
		dataMap[symbol] = &symbolData{}
	}

	// Wait for results with a SINGLE global timeout (2 seconds)
	timeout := 2 * time.Second
	deadline := time.Now().Add(timeout)

	// Use a single timer for the global timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Count how many symbols have complete data (both quote and trade)
	completeCount := 0
	target := len(symbols)

	// Collect results until timeout or all received
	for completeCount < target {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			slog.Debug("Global timeout reached for quotes batch",
				"complete", completeCount,
				"total", target)
			break
		}

		// Check each symbol's channels without blocking
		foundOne := false
		for symbol := range dataMap {
			data := dataMap[symbol]

			// Check quote channel if not yet received
			if !data.gotQuote {
				select {
				case quoteData := <-quoteFutures[symbol]:
					if quoteData != nil {
						if b, ok := quoteData["bid"].(float64); ok && b > 0 {
							bidVal := b
							data.bid = &bidVal
						}
						if a, ok := quoteData["ask"].(float64); ok && a > 0 {
							askVal := a
							data.ask = &askVal
						}
					}
					data.gotQuote = true
					foundOne = true
					if data.gotTrade {
						completeCount++
					}
				default:
				}
			}

			// Check trade channel if not yet received
			if !data.gotTrade {
				select {
				case tradeData := <-tradeFutures[symbol]:
					if tradeData != nil {
						if last, ok := tradeData["last"].(float64); ok && last > 0 {
							lastVal := last
							data.last = &lastVal
						}
					}
					data.gotTrade = true
					foundOne = true
					if data.gotQuote {
						completeCount++
					}
				default:
				}
			}
		}

		// If no new data, wait a bit before checking again
		if !foundOne {
			select {
			case <-time.After(50 * time.Millisecond):
				// Small delay to avoid busy loop
			case <-timer.C:
				slog.Debug("Global timeout reached for quotes batch",
					"complete", completeCount,
					"total", target)
				goto cleanup
			case <-ctx.Done():
				goto cleanup
			}
		}
	}

cleanup:
	// Build results from collected data
	results := make(map[string]*models.StockQuote)
	for _, symbol := range symbols {
		data := dataMap[symbol]
		results[symbol] = &models.StockQuote{
			Symbol:    symbol,
			Bid:       data.bid,
			Ask:       data.ask,
			Last:      data.last,
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}

	slog.Info("Quotes batch completed",
		"complete", completeCount,
		"total", target,
		"duration", time.Since(deadline.Add(-timeout)).String())

	// Clean up: unsubscribe and remove futures
	p.UnsubscribeFromSymbols(ctx, symbols, []string{"Quote", "Trade"})
	p.streamingState.requestsLock.Lock()
	for _, symbol := range symbols {
		delete(p.streamingState.quoteRequests, symbol)
		delete(p.streamingState.tradeRequests, symbol)
	}
	p.streamingState.requestsLock.Unlock()

	return results, nil
}

// GetExpirationDates gets available expiration dates for options on a symbol with universal enhanced structure.
// Exact conversion of Python get_expiration_dates method.
func (p *TastyTradeProvider) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/option-chains/%s", symbol)
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration dates: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Items []struct {
				ExpirationDate string `json:"expiration-date"`
				RootSymbol     string `json:"root-symbol"`
				ExpirationType string `json:"expiration-type"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse expiration dates response: %w", err)
	}

	// UI to Tasty mapping
	typeMapping := map[string]string{
		"regular":      "monthly",
		"end-of-month": "eom",
		"weekly":       "weekly",
		"quarterly":    "quarterly",
	}

	// Always return enhanced structure for all symbols
	var enhancedDates []map[string]interface{}
	// Group by expiration date and root symbol
	dateSymbolMap := make(map[string]map[string]interface{})

	for _, item := range apiResponse.Data.Items {
		expDate := item.ExpirationDate
		rootSymbol := item.RootSymbol
		if rootSymbol == "" {
			rootSymbol = symbol
		}
		expType := item.ExpirationType
		if expType == "" {
			expType = "Standard"
		}

		if expDate != "" {
			key := fmt.Sprintf("%s-%s", expDate, rootSymbol)
			if _, exists := dateSymbolMap[key]; !exists {
				// Determine type: if the root symbol is a known weekly variant
				// (e.g. NDXP, SPXW), classify as "weekly" regardless of TastyTrade's
				// expiration-type field, which reports "Regular" for these.
				var symbolType string
				if _, isWeekly := weeklyRootSymbols[rootSymbol]; isWeekly {
					symbolType = "weekly"
				} else {
					symbolType = typeMapping[strings.ToLower(expType)]
					if symbolType == "" {
						symbolType = strings.ToLower(expType)
					}
				}

				dateSymbolMap[key] = map[string]interface{}{
					"date":   expDate,
					"symbol": rootSymbol,
					"type":   symbolType,
				}
			}
		}
	}

	// Convert to list and sort by date, then by symbol
	for _, dateInfo := range dateSymbolMap {
		enhancedDates = append(enhancedDates, dateInfo)
	}

	// Sort by date, then by symbol
	for i := 0; i < len(enhancedDates)-1; i++ {
		for j := 0; j < len(enhancedDates)-i-1; j++ {
			date1 := enhancedDates[j]["date"].(string)
			date2 := enhancedDates[j+1]["date"].(string)
			symbol1 := enhancedDates[j]["symbol"].(string)
			symbol2 := enhancedDates[j+1]["symbol"].(string)

			if date1 > date2 || (date1 == date2 && symbol1 > symbol2) {
				enhancedDates[j], enhancedDates[j+1] = enhancedDates[j+1], enhancedDates[j]
			}
		}
	}

	return enhancedDates, nil
}

// GetOptionsChainBasic gets basic options chain data.
// Exact conversion of Python get_options_chain_basic method.
func (p *TastyTradeProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	apiSymbol := symbol
	if underlyingSymbol != nil && *underlyingSymbol != "" {
		apiSymbol = *underlyingSymbol
	}
	endpoint := fmt.Sprintf("/option-chains/%s?root-symbol=%s", apiSymbol, url.QueryEscape(symbol))
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get options chain: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Items []struct {
				Symbol         string `json:"symbol"`
				RootSymbol     string `json:"root-symbol"`
				ExpirationDate string `json:"expiration-date"`
				StrikePrice    string `json:"strike-price"`
				OptionType     string `json:"option-type"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse options chain response: %w", err)
	}

	var contracts []*models.OptionContract
	for _, item := range apiResponse.Data.Items {
		// Filter by expiry if specified
		if expiry != "" && item.ExpirationDate != expiry {
			continue
		}

		// Filter by root symbol to ensure exact chain scoping (parity with Python)
		rootMatches := strings.EqualFold(item.RootSymbol, symbol)
		if !rootMatches && optionType == nil {
			// Only expand to weekly variants (e.g., NDXP for NDX, SPXW for SPX)
			// when no explicit optionType is specified. When the caller requests a
			// specific type (monthly/weekly), they pass the exact root symbol they
			// want and we must not mix in contracts from another root.
			if weeklyRoot, ok := weeklyMap[strings.ToUpper(symbol)]; ok {
				rootMatches = strings.EqualFold(item.RootSymbol, weeklyRoot)
			}
		}
		if item.RootSymbol == "" || !rootMatches {
			continue
		}

		// Convert option type
		contractType := "call"
		if strings.ToLower(item.OptionType) == "p" || strings.ToLower(item.OptionType) == "put" {
			contractType = "put"
		}

		// Convert strike price from string to float64
		strikePrice, err := strconv.ParseFloat(item.StrikePrice, 64)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to parse strike price '%s' for symbol %s: %v", item.StrikePrice, item.Symbol, err))
			continue // Skip this contract if strike price is invalid
		}

		// Clean up the symbol format - TastyTrade returns symbols with extra spaces
		cleanSymbol := p.convertSymbolToStandardFormat(item.Symbol)

		contract := &models.OptionContract{
			Symbol:           cleanSymbol,
			UnderlyingSymbol: item.RootSymbol,
			RootSymbol:       &item.RootSymbol,
			ExpirationDate:   item.ExpirationDate,
			StrikePrice:      strikePrice,
			Type:             contractType,
		}
		contracts = append(contracts, contract)
	}

	if len(contracts) == 0 {
		return contracts, nil
	}

	// Get underlying price if not provided
	if underlyingPrice == nil {
		// Try to get quote for the underlying (respect underlying_symbol override used for API calls)
		quote, err := p.GetStockQuote(ctx, apiSymbol)
		if err == nil && quote != nil && quote.Bid != nil && quote.Ask != nil {
			avgPrice := (*quote.Bid + *quote.Ask) / 2
			underlyingPrice = &avgPrice
		}
	}

	// Smart filtering - focus on ATM strikes by count (same logic as Python)
	if underlyingPrice != nil && len(contracts) > 0 && strikeCount > 0 {
		// Get all unique strikes and sort them
		strikeMap := make(map[float64]bool)
		for _, contract := range contracts {
			strikeMap[contract.StrikePrice] = true
		}

		var strikes []float64
		for strike := range strikeMap {
			strikes = append(strikes, strike)
		}

		// Sort strikes
		for i := 0; i < len(strikes)-1; i++ {
			for j := 0; j < len(strikes)-i-1; j++ {
				if strikes[j] > strikes[j+1] {
					strikes[j], strikes[j+1] = strikes[j+1], strikes[j]
				}
			}
		}

		// Find the ATM strike (closest to underlying price)
		atmStrike := strikes[0]
		minDiff := abs(strikes[0] - *underlyingPrice)
		atmIndex := 0

		for i, strike := range strikes {
			diff := abs(strike - *underlyingPrice)
			if diff < minDiff {
				minDiff = diff
				atmStrike = strike
				atmIndex = i
			}
		}

		// Calculate how many strikes to take on each side
		strikesPerSide := strikeCount / 2

		// Get the range of strikes around ATM
		startIndex := max(0, atmIndex-strikesPerSide)
		endIndex := min(len(strikes), atmIndex+strikesPerSide+1)

		// Select strikes in the range
		selectedStrikes := make(map[float64]bool)
		for i := startIndex; i < endIndex; i++ {
			selectedStrikes[strikes[i]] = true
		}

		// Filter contracts to only include selected strikes
		var filteredContracts []*models.OptionContract
		for _, contract := range contracts {
			if selectedStrikes[contract.StrikePrice] {
				filteredContracts = append(filteredContracts, contract)
			}
		}

		contracts = filteredContracts
		slog.Debug(fmt.Sprintf("TastyTrade: Filtered to %d contracts around ATM strike %.2f (underlying: %.2f)", len(contracts), atmStrike, *underlyingPrice))
	}

	return contracts, nil
}

// GetOptionsGreeksBatch gets Greeks for multiple option symbols using streaming.
// Exact conversion of Python get_streaming_greeks_batch method.
func (p *TastyTradeProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	return p.getStreamingGreeksBatch(ctx, optionSymbols, 2)
}

// getStreamingGreeksBatch subscribes to a list of option symbols and waits for their greeks data.
// This is more efficient than calling getStreamingGreeks for each symbol individually.
// Exact conversion of Python get_streaming_greeks_batch method.
func (p *TastyTradeProvider) getStreamingGreeksBatch(ctx context.Context, symbols []string, timeout int) (map[string]map[string]interface{}, error) {
	if !p.ensureHealthyConnection(ctx) {
		return make(map[string]map[string]interface{}), fmt.Errorf("streaming connection not available")
	}

	// Create futures for each symbol
	futures := make(map[string]chan map[string]interface{})
	for _, symbol := range symbols {
		futures[symbol] = make(chan map[string]interface{}, 1)
		p.streamingState.requestsLock.Lock()
		p.streamingState.greeksRequests[symbol] = futures[symbol]
		p.streamingState.requestsLock.Unlock()
	}

	// Subscribe to Greeks data for all symbols
	success, err := p.SubscribeToSymbols(ctx, symbols, []string{"Greeks"})
	if err != nil || !success {
		// Clean up futures
		p.streamingState.requestsLock.Lock()
		for _, symbol := range symbols {
			delete(p.streamingState.greeksRequests, symbol)
		}
		p.streamingState.requestsLock.Unlock()
		return make(map[string]map[string]interface{}), fmt.Errorf("failed to subscribe to symbols: %v", err)
	}

	// Wait for results with a SINGLE global timeout (not per-symbol)
	// This is critical for performance - we wait for all symbols in parallel
	results := make(map[string]map[string]interface{})
	timeoutDuration := time.Duration(timeout) * time.Second
	deadline := time.Now().Add(timeoutDuration)

	received := 0
	target := len(symbols)

	// Use a single timer for the global timeout
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()

	// Collect results until timeout or all received
	for received < target {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			slog.Warn("Global timeout reached for Greeks batch",
				"received", received,
				"total", target,
				"timeout", timeout)
			break
		}

		// Check each symbol's channel without blocking
		foundOne := false
		for symbol, ch := range futures {
			// Skip already received symbols
			if _, exists := results[symbol]; exists {
				continue
			}

			select {
			case greeks := <-ch:
				if greeks != nil {
					results[symbol] = greeks
				} else {
					results[symbol] = nil
				}
				received++
				foundOne = true
			default:
				// Channel not ready, continue to next
			}
		}

		// If no new data, wait a bit before checking again
		if !foundOne {
			select {
			case <-time.After(50 * time.Millisecond):
				// Small delay to avoid busy loop
			case <-timer.C:
				slog.Warn("Global timeout reached for Greeks batch",
					"received", received,
					"total", target,
					"timeout", timeout)
				goto cleanup
			case <-ctx.Done():
				goto cleanup
			}
		}
	}

cleanup:
	// Mark remaining symbols as nil
	for _, symbol := range symbols {
		if _, exists := results[symbol]; !exists {
			results[symbol] = nil
		}
	}

	slog.Info("Greeks batch completed",
		"received", received,
		"total", target,
		"duration", time.Since(deadline.Add(-timeoutDuration)).String())

	// Clean up: unsubscribe and remove futures
	p.UnsubscribeFromSymbols(ctx, symbols, []string{"Greeks"})
	p.streamingState.requestsLock.Lock()
	for _, symbol := range symbols {
		delete(p.streamingState.greeksRequests, symbol)
	}
	p.streamingState.requestsLock.Unlock()

	return results, nil
}

// ensureHealthyConnection ensures we have a healthy streaming connection, reconnect if needed.
// Exact conversion of Python _ensure_healthy_connection method.
func (p *TastyTradeProvider) ensureHealthyConnection(ctx context.Context) bool {
	// Check if we're already connected and healthy
	if p.streamingState.isConnected && p.streamingState.streamConnection != nil {
		// Test the connection with a ping (simple check)
		// For WebSocket, we can check if the connection is still open
		// In Go, we don't have a direct ping method, so we'll assume it's healthy if connected
		slog.Debug("TastyTrade: Connection health check passed")
		return true
	}

	// Connection is not healthy, attempt to reconnect
	slog.Info("TastyTrade: Establishing new streaming connection...")
	success, err := p.ConnectStreaming(ctx)
	if err != nil {
		slog.Error("TastyTrade: Failed to establish streaming connection", "error", err)
		return false
	}
	return success
}

// GetOptionsChainSmart gets smart options chain data.
// Exact conversion of Python get_options_chain_smart method.
func (p *TastyTradeProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	if includeGreeks {
		// Would use full options chain with Greeks
		return p.GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, atmRange, nil, nil)
	}
	return p.GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, atmRange, nil, nil)
}

// GetNextMarketDate gets the next trading date.
// Exact conversion of Python get_next_market_date method.
func (p *TastyTradeProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	// Simple implementation: next weekday
	today := time.Now()
	nextDate := today.AddDate(0, 0, 1)

	// Skip weekends
	for nextDate.Weekday() == time.Saturday || nextDate.Weekday() == time.Sunday {
		nextDate = nextDate.AddDate(0, 0, 1)
	}

	return nextDate.Format("2006-01-02"), nil
}

// LookupSymbols searches for symbols matching the query.
// Exact conversion of Python lookup_symbols method.
func (p *TastyTradeProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	endpoint := fmt.Sprintf("/symbols/search/%s", query)
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search symbols: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Items []struct {
				Symbol         string `json:"symbol"`
				Description    string `json:"description"`
				ListedMarket   string `json:"listed-market"`
				InstrumentType string `json:"instrument-type"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse symbol search response: %w", err)
	}

	var results []*models.SymbolSearchResult
	for _, item := range apiResponse.Data.Items {
		// Map instrument type
		symbolType := item.InstrumentType
		if symbolType == "Equity" {
			symbolType = "Stock"
		}

		result := &models.SymbolSearchResult{
			Symbol:      item.Symbol,
			Description: item.Description,
			Exchange:    item.ListedMarket,
			Type:        symbolType,
		}
		results = append(results, result)
	}

	return results, nil
}

// GetHistoricalBars gets historical OHLCV bars using DXLink Candle Events.
// Exact conversion of Python get_historical_bars method.
func (p *TastyTradeProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	// 1. Validate inputs and map timeframe
	dxlinkTimeframe := p.mapTimeframeToDXLink(timeframe)
	if dxlinkTimeframe == "" {
		return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	// 2. Calculate fromTime for historical data
	fromTime := p.calculateFromTime(startDate, timeframe)

	// 3. Ensure we have valid quote token
	if err := p.getQuoteToken(ctx); err != nil {
		slog.Error("TastyTrade: Failed to get quote token for historical data")
		return nil, fmt.Errorf("failed to get quote token: %w", err)
	}

	// 4. Create DXLink candle client and collect data
	collectionLimit := limit
	if endDate != nil {
		// Increase collection limit to account for potential filtering
		collectionLimit = max(limit*3, 100)
	}

	// 4. Create DXLink candle client and collect data (exact same as Python)
	candleClient := NewDXLinkCandleClient(p.dxlinkURL, p.quoteToken)
	candles, err := candleClient.GetCandles(ctx, symbol, dxlinkTimeframe, fromTime, collectionLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get DXLink candles: %w", err)
	}

	// 5. Transform to standard format and filter out null data
	var result []map[string]interface{}
	for _, candle := range candles {
		transformed := p.transformDXLinkCandle(candle, timeframe)
		if transformed != nil {
			// Filter out bars with null or NaN OHLC data
			if p.isValidOHLC(transformed) {
				result = append(result, transformed)
			}
		}
	}

	// If no data from DXLink, log the issue and return empty
	if len(result) == 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: No historical data received for %s %s", symbol, timeframe))
		return result, nil
	}

	// 6. Sort by time and apply end_date filter if specified
	p.sortBarsByTime(result)

	// Apply end_date filter if specified
	if endDate != nil && *endDate != "" {
		result = p.filterByEndDate(result, *endDate)
	}

	// Apply limit after filtering
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:] // Get most recent bars
	}

	slog.Debug(fmt.Sprintf("TastyTrade: Retrieved %d historical bars for %s %s", len(result), symbol, timeframe))
	return result, nil
}

// === Account & Portfolio Methods ===

// GetPositions gets all current positions.
// Exact conversion of Python get_positions method.
func (p *TastyTradeProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	if err := p.ensureValidSession(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with TastyTrade: %w", err)
	}

	endpoint := fmt.Sprintf("/accounts/%s/positions", p.accountID)
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Parse response as generic map (like Python) to handle flexible field types
	var apiResponse map[string]interface{}
	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse positions response: %w", err)
	}

	// Extract items from nested structure - same as Python
	data, ok := apiResponse["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing data")
	}

	items, ok := data["items"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing items")
	}

	// Transform TastyTrade positions to our standard model - same as Python
	var positions []*models.Position
	for _, item := range items {
		if positionData, ok := item.(map[string]interface{}); ok {
			position := p.transformTastyTradePosition(positionData)
			if position != nil {
				positions = append(positions, position)
			}
		}
	}

	return positions, nil
}

// GetPositionsEnhanced gets enhanced positions grouped by date_acquired (same order timing).
// Exact conversion of Python get_positions_enhanced method.
func (p *TastyTradeProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	slog.Debug("TastyTrade: Getting enhanced positions grouped by acquisition date...")

	// 1. Get current positions only (no additional API calls needed)
	currentPositions, err := p.GetPositions(ctx)
	if err != nil {
		return models.NewEnhancedPositionsResponse(), nil
	}

	if len(currentPositions) == 0 {
		slog.Info("TastyTrade: No current positions found")
		return models.NewEnhancedPositionsResponse(), nil
	}

	// 2. Use base provider's conversion logic
	return p.BaseProviderImpl.ConvertPositionsToEnhanced(currentPositions), nil
}

// createDateBasedHierarchicalGroups creates hierarchical groups based on date_acquired (same order timing).
// Exact conversion of Python _create_date_based_hierarchical_groups method.
func (p *TastyTradeProvider) createDateBasedHierarchicalGroups(positions []*models.Position) ([]interface{}, error) {
	slog.Debug("TastyTrade: Creating date-based hierarchical symbol groups")

	// Step 1: Group positions by underlying symbol first
	symbolGroups := make(map[string]map[string]interface{})

	for _, position := range positions {
		// Determine underlying symbol
		var underlying string
		var assetClass string

		if p.isOptionSymbol(position.Symbol) {
			parsed := p.parseOptionSymbol(position.Symbol)
			if parsed != nil {
				underlying = parsed["underlying"].(string)
			} else {
				underlying = position.Symbol
			}
			assetClass = "options"
		} else {
			underlying = position.Symbol
			assetClass = "stocks"
		}

		if _, exists := symbolGroups[underlying]; !exists {
			symbolGroups[underlying] = map[string]interface{}{
				"symbol":      underlying,
				"asset_class": assetClass,
				"positions":   []*models.Position{},
			}
		}

		positions := symbolGroups[underlying]["positions"].([]*models.Position)
		positions = append(positions, position)
		symbolGroups[underlying]["positions"] = positions
	}

	// Step 2: Within each underlying, group by date_acquired
	var result []interface{}
	for underlying, symbolGroup := range symbolGroups {
		positions := symbolGroup["positions"].([]*models.Position)
		strategies, err := p.groupByDateAcquired(positions)
		if err != nil {
			continue
		}

		result = append(result, map[string]interface{}{
			"symbol":      underlying,
			"asset_class": symbolGroup["asset_class"],
			"strategies":  strategies,
		})
	}

	slog.Debug(fmt.Sprintf("TastyTrade: Created %d date-based symbol groups", len(result)))
	return result, nil
}

// groupByDateAcquired groups positions by their date_acquired timestamp.
// Exact conversion of Python _group_by_date_acquired method.
func (p *TastyTradeProvider) groupByDateAcquired(positions []*models.Position) ([]interface{}, error) {
	slog.Debug(fmt.Sprintf("TastyTrade: Grouping %d positions by date_acquired", len(positions)))

	// Group by date_acquired
	dateGroups := make(map[string][]*models.Position)

	for _, position := range positions {
		// Use date_acquired as the grouping key
		dateKey := "unknown"
		if position.DateAcquired != nil {
			dateKey = *position.DateAcquired
		}

		// For more precise grouping, we can truncate to minute precision
		// This handles cases where positions from the same order might have slightly different timestamps
		if dateKey != "unknown" {
			// Parse the date and truncate to minute precision for grouping
			if dt, err := time.Parse(time.RFC3339, dateKey); err == nil {
				// Group by minute precision (same order should be within the same minute)
				dateKey = dt.Format("2006-01-02 15:04")
			} else {
				slog.Warn(fmt.Sprintf("TastyTrade: Could not parse date_acquired: %s", dateKey))
				// Keep original value if parsing fails
			}
		}

		if _, exists := dateGroups[dateKey]; !exists {
			dateGroups[dateKey] = []*models.Position{}
		}
		dateGroups[dateKey] = append(dateGroups[dateKey], position)
	}

	slog.Debug(fmt.Sprintf("TastyTrade: Created %d date-based groups", len(dateGroups)))

	// Convert to strategy format
	var strategies []interface{}
	for dateKey, datePositions := range dateGroups {
		strategyName := p.detectStrategyName(datePositions)

		// Calculate DTE if options are present
		dte := p.calculateDTEForPositions(datePositions)

		// Calculate strategy totals (static data only)
		var strategyTotalQty float64
		var strategyCostBasis float64
		for _, pos := range datePositions {
			strategyTotalQty += pos.Qty
			strategyCostBasis += pos.CostBasis
		}

		// Create legs with static broker data
		legs := p.convertPositionsToLegs(datePositions)

		strategy := map[string]interface{}{
			"name":          strategyName,
			"total_qty":     strategyTotalQty,
			"cost_basis":    strategyCostBasis,
			"dte":           dte,
			"legs":          legs,
			"date_acquired": dateKey, // Include the grouping date for reference
		}
		strategies = append(strategies, strategy)
	}

	slog.Debug(fmt.Sprintf("TastyTrade: Created %d strategies from date grouping", len(strategies)))
	return strategies, nil
}

// calculateDTEForPositions calculates days to expiration for a group of positions.
// Exact conversion of Python _calculate_dte_for_positions method.
func (p *TastyTradeProvider) calculateDTEForPositions(positions []*models.Position) *int {
	// Find the earliest expiration date among option positions
	var expiryDates []string
	for _, pos := range positions {
		if p.isOptionSymbol(pos.Symbol) {
			parsed := p.parseOptionSymbol(pos.Symbol)
			if parsed != nil {
				if expiry, ok := parsed["expiry"].(string); ok && expiry != "" {
					expiryDates = append(expiryDates, expiry)
				}
			}
		}
	}

	if len(expiryDates) > 0 {
		// Use the earliest expiration date
		earliestExpiry := expiryDates[0]
		for _, expiry := range expiryDates[1:] {
			if expiry < earliestExpiry {
				earliestExpiry = expiry
			}
		}

		if expiryDate, err := time.Parse("2006-01-02", earliestExpiry); err == nil {
			dte := int(time.Until(expiryDate).Hours() / 24)
			return &dte
		}
	}

	return nil
}

// convertPositionsToLegs converts positions to leg format with static broker data.
// Exact conversion of Python _convert_positions_to_legs method.
func (p *TastyTradeProvider) convertPositionsToLegs(positions []*models.Position) []interface{} {
	var legs []interface{}
	for _, pos := range positions {
		leg := map[string]interface{}{
			"symbol":          pos.Symbol,
			"qty":             pos.Qty,
			"avg_entry_price": pos.AvgEntryPrice,
			"cost_basis":      pos.CostBasis,
			"asset_class":     pos.AssetClass,
			"lastday_price":   nil, // Not available in basic position data
		}
		if pos.DateAcquired != nil {
			leg["date_acquired"] = *pos.DateAcquired
		}
		legs = append(legs, leg)
	}
	return legs
}

// detectStrategyName detects strategy name based on positions using centralized strategy detection.
// Exact conversion of Python _detect_strategy_name method.
func (p *TastyTradeProvider) detectStrategyName(positions []*models.Position) string {
	if len(positions) == 1 {
		if p.isOptionSymbol(positions[0].Symbol) {
			return "Single Option"
		} else {
			return "Stock Position"
		}
	}

	// For multi-leg strategies, use basic strategy detection
	var optionPositions []*models.Position
	for _, pos := range positions {
		if p.isOptionSymbol(pos.Symbol) {
			optionPositions = append(optionPositions, pos)
		}
	}

	if len(optionPositions) >= 2 {
		// Basic strategy detection based on position count
		// In a full implementation, this would use more sophisticated logic
		// to detect spreads, straddles, etc.
		switch len(optionPositions) {
		case 2:
			return "2-Leg Strategy"
		case 4:
			return "4-Leg Strategy"
		case 6:
			return "6-Leg Strategy"
		default:
			return fmt.Sprintf("%d-Leg Strategy", len(optionPositions))
		}
	}

	// Fallback to generic naming
	switch len(positions) {
	case 2:
		return "2-Leg Strategy"
	case 4:
		return "4-Leg Strategy"
	case 6:
		return "6-Leg Strategy"
	default:
		return fmt.Sprintf("%d-Leg Strategy", len(positions))
	}
}

// parseOptionSymbol parses TastyTrade option symbol to extract components.
// Exact conversion of Python _parse_option_symbol method.
func (p *TastyTradeProvider) parseOptionSymbol(symbol string) map[string]interface{} {
	// TastyTrade uses standard OCC format
	// Example: AAPL  220617P00150000
	// Format: ROOT(6) + YYMMDD(6) + C/P(1) + STRIKE(8)

	if len(symbol) < 21 {
		return nil
	}

	// Extract components
	root := strings.TrimSpace(symbol[:6]) // Remove padding spaces
	datePart := symbol[6:12]
	optionType := symbol[12]
	strikePart := symbol[13:21]

	// Parse expiry date
	year, err1 := strconv.Atoi(datePart[:2])
	month, err2 := strconv.Atoi(datePart[2:4])
	day, err3 := strconv.Atoi(datePart[4:6])

	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}

	year += 2000 // Convert YY to YYYY
	expiryDate := fmt.Sprintf("%04d-%02d-%02d", year, month, day)

	// Parse strike price
	strikeRaw, err := strconv.Atoi(strikePart)
	if err != nil {
		return nil
	}
	strikePrice := float64(strikeRaw) / 1000

	// Determine option type
	var contractType string
	if optionType == 'C' {
		contractType = "call"
	} else {
		contractType = "put"
	}

	return map[string]interface{}{
		"underlying": root,
		"type":       contractType,
		"strike":     strikePrice,
		"expiry":     expiryDate,
	}
}

// isOptionSymbol checks if symbol is an option symbol.
func (p *TastyTradeProvider) isOptionSymbol(symbol string) bool {
	// Basic check: option symbols are typically longer than 10 characters
	// and contain 'C' or 'P' and have digits in the last 8 characters
	if len(symbol) <= 10 {
		return false
	}

	// Check for 'C' or 'P' (call/put indicators)
	hasCallPut := strings.Contains(symbol, "C") || strings.Contains(symbol, "P")
	if !hasCallPut {
		return false
	}

	// Check if last 8 characters contain digits (strike price)
	lastEight := symbol[len(symbol)-8:]
	hasDigits := false
	for _, char := range lastEight {
		if char >= '0' && char <= '9' {
			hasDigits = true
			break
		}
	}

	return hasDigits
}

// GetOrders gets orders with optional status filter.
// Exact conversion of Python get_orders method.
func (p *TastyTradeProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	var endpoint string
	if status == "open" {
		// Use live orders endpoint for open orders - same as Python
		endpoint = fmt.Sprintf("/accounts/%s/orders/live", p.accountID)
	} else {
		// Use all orders endpoint for other statuses - same as Python
		endpoint = fmt.Sprintf("/accounts/%s/orders", p.accountID)
	}

	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	// Parse response as generic map (like Python) to handle flexible ID types
	var apiResponse map[string]interface{}
	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}

	// Extract items from nested structure - same as Python
	data, ok := apiResponse["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing data")
	}

	items, ok := data["items"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing items")
	}

	// Transform TastyTrade orders to our standard model - same as Python
	var orders []*models.Order
	for _, item := range items {
		if orderData, ok := item.(map[string]interface{}); ok {
			order := p.transformTastyTradeOrder(orderData)
			if order != nil {
				// Apply additional status filtering for non-live endpoints - same as Python
				if status != "open" && status != "all" {
					orderStatus := strings.ToLower(order.Status)
					if status == "filled" && orderStatus != "filled" {
						continue
					} else if status == "cancelled" && orderStatus != "cancelled" {
						continue
					}
				}
				orders = append(orders, order)
			}
		}
	}

	return orders, nil
}

// GetAccount gets account information.
// Exact conversion of Python get_account method.
func (p *TastyTradeProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	endpoint := fmt.Sprintf("/accounts/%s/balances", p.accountID)
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	var apiResponse struct {
		Data struct {
			AccountNumber          string      `json:"account-number"`
			CashBalance            interface{} `json:"cash-balance"`
			EquityBuyingPower      interface{} `json:"equity-buying-power"`
			DayTradingBuyingPower  interface{} `json:"day-trading-buying-power"`
			MarginEquity           interface{} `json:"margin-equity"`
			RegTMarginRequirement  interface{} `json:"reg-t-margin-requirement"`
			MaintenanceRequirement interface{} `json:"maintenance-requirement"`
			NetLiquidatingValue    interface{} `json:"net-liquidating-value"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	// Convert interface{} values to float64 pointers
	var buyingPower, cash, dayTradingBuyingPower, equity, initialMargin, maintenanceMargin, portfolioValue *float64

	if val := p.interfaceToFloat64(apiResponse.Data.EquityBuyingPower); val != nil {
		buyingPower = val
	}
	if val := p.interfaceToFloat64(apiResponse.Data.CashBalance); val != nil {
		cash = val
	}
	if val := p.interfaceToFloat64(apiResponse.Data.DayTradingBuyingPower); val != nil {
		dayTradingBuyingPower = val
	}
	if val := p.interfaceToFloat64(apiResponse.Data.MarginEquity); val != nil {
		equity = val
	}
	if val := p.interfaceToFloat64(apiResponse.Data.RegTMarginRequirement); val != nil {
		initialMargin = val
	}
	if val := p.interfaceToFloat64(apiResponse.Data.MaintenanceRequirement); val != nil {
		maintenanceMargin = val
	}
	if val := p.interfaceToFloat64(apiResponse.Data.NetLiquidatingValue); val != nil {
		portfolioValue = val
	}

	account := &models.Account{
		AccountID:             p.accountID,
		AccountNumber:         &apiResponse.Data.AccountNumber,
		Status:                "active",
		BuyingPower:           buyingPower,
		Cash:                  cash,
		DayTradingBuyingPower: dayTradingBuyingPower,
		Equity:                equity,
		InitialMargin:         initialMargin,
		MaintenanceMargin:     maintenanceMargin,
		PortfolioValue:        portfolioValue,
	}

	return account, nil
}

// === Order Management Methods ===

// PlaceOrder places a trading order.
// Exact conversion of Python place_order method.
func (p *TastyTradeProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	if err := p.ensureValidSession(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with TastyTrade: %w", err)
	}

	// Transform our standard order format to TastyTrade format
	tastytradeOrder := p.transformToTastyTradeOrder(orderData)

	// Log the order being sent for debugging
	slog.Debug("TastyTrade: Sending order payload", "payload", tastytradeOrder)

	endpoint := fmt.Sprintf("/accounts/%s/orders", p.accountID)
	responseBytes, err := p.makeAuthenticatedRequest(ctx, "POST", endpoint, tastytradeOrder)

	// Handle error responses specially - they contain error details in the body (matching Python logic)
	if err != nil {
		// Check if this is a 422 validation error with response body
		if responseBytes != nil {
			var errorResponse map[string]interface{}
			if parseErr := json.Unmarshal(responseBytes, &errorResponse); parseErr == nil {
				if errorData, ok := errorResponse["error"].(map[string]interface{}); ok {
					var errorMessages []string
					if errorsArray, ok := errorData["errors"].([]interface{}); ok {
						for _, errorItem := range errorsArray {
							if errorMap, ok := errorItem.(map[string]interface{}); ok {
								if message, ok := errorMap["message"].(string); ok && message != "" {
									errorMessages = append(errorMessages, message)
								}
							}
						}
					}
					if len(errorMessages) > 0 {
						errorMsg := strings.Join(errorMessages, "; ")
						return nil, fmt.Errorf("order validation failed: %s", errorMsg)
					}
				}
			}
		}
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// TastyTrade nests the actual order data under data.order
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data field")
	}

	orderResp, ok := data["order"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing order field")
	}

	// Transform TastyTrade order response to our standard model
	return p.transformTastyTradeOrder(orderResp), nil
}

// PlaceMultiLegOrder places a multi-leg trading order.
// Exact conversion of Python place_multi_leg_order method.
func (p *TastyTradeProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	if err := p.ensureValidSession(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with TastyTrade: %w", err)
	}

	// Transform our standard multi-leg order format to TastyTrade format
	tastytradeOrder := p.transformToTastyTradeMultiLegOrder(orderData)

	// Log the order being sent for debugging
	slog.Debug("TastyTrade: Sending multi-leg order", "payload", tastytradeOrder)

	endpoint := fmt.Sprintf("/accounts/%s/orders", p.accountID)
	responseBytes, err := p.makeAuthenticatedRequest(ctx, "POST", endpoint, tastytradeOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to place multi-leg order: %w", err)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// TastyTrade nests the actual order data under data.order
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data field")
	}

	orderResp2, ok := data["order"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing order field")
	}

	// Transform TastyTrade order response to our standard model
	return p.transformTastyTradeOrder(orderResp2), nil
}

// CancelOrder cancels an existing order.
// Exact conversion of Python cancel_order method.
func (p *TastyTradeProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	endpoint := fmt.Sprintf("/accounts/%s/orders/%s", p.accountID, orderID)
	_, err := p.makeAuthenticatedRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("failed to cancel order: %w", err)
	}
	return true, nil
}

// === Streaming Methods ===

// Streaming infrastructure fields (add to struct)
type StreamingState struct {
	streamConnection  *websocket.Conn
	connectionReady   chan struct{}
	isConnected       bool
	subscribedSymbols map[string]bool
	streamingQueue    chan *models.MarketData
	streamingCache    base.StreamingCache
	shutdownEvent     chan struct{}
	streamingTask     *StreamingTask
	connectionID      string
	recvLock          sync.Mutex
	writeLock         sync.Mutex // Add: mutex for WebSocket writes
	connectionMutex   sync.Mutex // Add: mutex to prevent concurrent ConnectStreaming calls
	greeksRequests    map[string]chan map[string]interface{}
	quoteRequests     map[string]chan map[string]interface{}
	tradeRequests     map[string]chan map[string]interface{} // For receiving Trade events (last price)
	requestsLock      sync.Mutex
	messageChan       chan map[string]interface{} // Add: buffered channel for messages
	errorChan         chan error                  // Add: for read errors

	// Order event streaming
	orderEventCallback func(*models.OrderEvent)
	orderEventChan     chan *models.OrderEvent
}

type StreamingTask struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Initialize streaming state in constructor
func (p *TastyTradeProvider) initStreamingState() {
	p.streamingState = &StreamingState{
		connectionReady:   make(chan struct{}),
		subscribedSymbols: make(map[string]bool),
		shutdownEvent:     make(chan struct{}),
		connectionID:      fmt.Sprintf("tastytrade_%s", p.accountID),
		greeksRequests:    make(map[string]chan map[string]interface{}),
		quoteRequests:     make(map[string]chan map[string]interface{}),
		tradeRequests:     make(map[string]chan map[string]interface{}),
		messageChan:       make(chan map[string]interface{}, 100),
		errorChan:         make(chan error, 1),
		orderEventChan:    make(chan *models.OrderEvent, 50),
	}
}

// ConnectStreaming connects to DXLink streaming service with health monitoring.
// Exact conversion of Python connect_streaming method.
func (p *TastyTradeProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	// CRITICAL: Lock to prevent concurrent connection attempts
	p.streamingState.connectionMutex.Lock()
	defer p.streamingState.connectionMutex.Unlock()

	// If already connected, return success
	if p.streamingState.isConnected {
		slog.Debug("TastyTrade: Already connected to streaming")
		return true, nil
	}

	slog.Info("TastyTrade: Establishing new streaming connection...")

	// Initialize streaming state if needed
	if p.streamingState == nil {
		p.initStreamingState()
	}

	slog.Info("TastyTrade: Establishing streaming connection...")

	// Clean up any existing connection first
	if p.streamingState.streamingTask != nil {
		p.streamingState.streamingTask.cancel()
		<-p.streamingState.streamingTask.done
		p.streamingState.streamingTask = nil
	}

	// Recreate channels to ensure fresh state for new connection
	// This is safe because we waited for the previous task to finish
	p.streamingState.messageChan = make(chan map[string]interface{}, 100)
	p.streamingState.errorChan = make(chan error, 1)
	p.streamingState.shutdownEvent = make(chan struct{})

	if p.streamingState.streamConnection != nil {
		p.streamingState.streamConnection.Close()
		p.streamingState.streamConnection = nil
	}

	// Reset connection state
	p.streamingState.isConnected = false
	p.streamingState.connectionReady = make(chan struct{})

	// Get fresh quote token for DXLink authentication
	if err := p.getQuoteToken(ctx); err != nil {
		slog.Error("Failed to get quote token for streaming", "error", err)
		return false, err
	}

	// Create DXLink streaming connection with retry logic and proper WebSocket configuration
	maxRetries := 3
	var conn *websocket.Conn
	var err error

	// Configure WebSocket dialer with proper settings (matching Python websockets.connect parameters)
	dialer := &websocket.Dialer{
		HandshakeTimeout: 5 * time.Second, // Reduced from 15s for faster failure detection
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		slog.Info(fmt.Sprintf("TastyTrade: Connection attempt %d/%d", attempt+1, maxRetries))

		conn, _, err = dialer.DialContext(ctx, p.dxlinkURL, nil)
		if err == nil {
			// Configure the connection with proper timeouts (matching Python ping_interval=30, ping_timeout=10)
			conn.SetPingHandler(func(appData string) error {
				slog.Debug("TastyTrade: Received ping from server")
				// Extend read deadline on Ping to keep connection alive
				if err := conn.SetReadDeadline(time.Now().Add(70 * time.Second)); err != nil {
					slog.Warn("Failed to extend read deadline on ping", "error", err)
				}
				return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
			})

			conn.SetPongHandler(func(appData string) error {
				slog.Debug("TastyTrade: Received pong from server")
				return nil
			})

			// Set close handler to log connection closures
			conn.SetCloseHandler(func(code int, text string) error {
				slog.Info(fmt.Sprintf("TastyTrade: WebSocket closed by server: code=%d, text=%s", code, text))
				return nil
			})

			break
		}

		slog.Warn(fmt.Sprintf("TastyTrade: Connection attempt %d failed: %v", attempt+1, err))
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * time.Second) // Exponential backoff
		}
	}

	if err != nil {
		return false, fmt.Errorf("failed to connect to DXLink after %d attempts: %w", maxRetries, err)
	}

	p.streamingState.streamConnection = conn

	// Execute DXLink setup sequence
	if err := p.dxlinkStreamingSetup(ctx); err != nil {
		slog.Error("DXLink streaming setup failed", "error", err)
		conn.Close()
		return false, err
	}

	p.streamingState.isConnected = true
	close(p.streamingState.connectionReady)

	// Start the streaming data processing task
	p.startStreamingTask(ctx)

	slog.Info("TastyTrade streaming connected successfully")
	return true, nil
}

// DisconnectStreaming disconnects from DXLink streaming service.
// Exact conversion of Python disconnect_streaming method.
func (p *TastyTradeProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	if p.streamingState == nil {
		return true, nil
	}

	// Signal shutdown
	// Signal shutdown safely
	select {
	case <-p.streamingState.shutdownEvent:
		// Already closed
	default:
		close(p.streamingState.shutdownEvent)
	}

	// Cancel streaming task (with timeout to avoid blocking forever)
	if p.streamingState.streamingTask != nil {
		p.streamingState.streamingTask.cancel()

		// Wait for task to finish with timeout (non-blocking)
		done := make(chan struct{})
		go func() {
			if p.streamingState.streamingTask != nil {
				<-p.streamingState.streamingTask.done
			}
			close(done)
		}()

		// Wait up to 2 seconds for graceful shutdown
		select {
		case <-done:
			slog.Info("TastyTrade: Streaming task exited gracefully")
		case <-time.After(2 * time.Second):
			slog.Warn("TastyTrade: Streaming task did not exit in time, continuing...")
		}
	}

	// Close connection
	if p.streamingState.streamConnection != nil {
		p.streamingState.streamConnection.Close()
		p.streamingState.streamConnection = nil
	}

	p.streamingState.isConnected = false
	p.streamingState.subscribedSymbols = make(map[string]bool)

	slog.Info("TastyTrade streaming disconnected")
	return true, nil
}

// EnsureHealthyConnection ensures we have a healthy streaming connection, reconnects if needed.
// Exact conversion of Python _ensure_healthy_connection method.
func (p *TastyTradeProvider) EnsureHealthyConnection(ctx context.Context) error {
	// Check if we're already connected and healthy
	if p.streamingState != nil && p.streamingState.isConnected && p.streamingState.streamConnection != nil {
		// Test the connection with a ping (use caller's context for timeout)
		if err := p.Ping(ctx); err == nil {
			slog.Debug("TastyTrade: Connection health check passed")
			return nil
		}
		slog.Warn("TastyTrade: Connection health check failed, reconnecting...")
		p.streamingState.isConnected = false
	}

	// Connection is not healthy, attempt to reconnect
	// CRITICAL: Use Background context for connection, not caller's context!
	// The connection must survive beyond the HTTP request that triggered this check.
	slog.Info("TastyTrade: Ensuring healthy connection, reconnecting...")
	success, err := p.ConnectStreaming(context.Background())
	if err != nil || !success {
		return fmt.Errorf("failed to establish healthy connection: %w", err)
	}
	return nil
}

// writeJSONWithTimeout writes a JSON message with a timeout and locking.
func (p *TastyTradeProvider) writeJSONWithTimeout(v interface{}) error {
	p.streamingState.writeLock.Lock()
	defer p.streamingState.writeLock.Unlock()

	if p.streamingState.streamConnection == nil {
		return fmt.Errorf("connection is nil")
	}

	// Set write deadline for this operation
	if err := p.streamingState.streamConnection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	if err := p.streamingState.streamConnection.WriteJSON(v); err != nil {
		return err
	}

	// Reset deadline (optional, but good practice if we want to leave it clean)
	// p.streamingState.streamConnection.SetWriteDeadline(time.Time{})
	return nil
}

// SubscribeToSymbols subscribes to real-time data for symbols via DXLink.
// Batches subscriptions into chunks of 50 to avoid server-side limits.
func (p *TastyTradeProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	if p.streamingState == nil || !p.streamingState.isConnected {
		return false, fmt.Errorf("streaming not connected")
	}

	slog.Info(fmt.Sprintf("TastyTrade: subscribe_to_symbols called with %d symbols, data_types: %v", len(symbols), dataTypes))

	// Wait for connection to be ready
	select {
	case <-p.streamingState.connectionReady:
	case <-ctx.Done():
		return false, ctx.Err()
	}

	// Default to both Quote and Greeks if no data_types specified
	if dataTypes == nil {
		dataTypes = []string{"Quote", "Greeks"}
	}

	// Prepare subscriptions using correct streaming symbols
	var allSubscriptions []map[string]interface{}

	for _, symbol := range symbols {
		// Convert to streaming symbol format for DXLink
		var streamingSymbol string
		if p.isOptionSymbol(symbol) {
			streamingSymbol = p.convertToStreamerSymbol(symbol)
			slog.Debug(fmt.Sprintf("TastyTrade: Converted option symbol %s -> %s", symbol, streamingSymbol))
		} else {
			streamingSymbol = symbol // Stock symbols use as-is
			slog.Debug(fmt.Sprintf("TastyTrade: Using stock symbol as-is: %s", streamingSymbol))
		}

		// Add Quote subscription only if requested
		if containsString(dataTypes, "Quote") {
			allSubscriptions = append(allSubscriptions, map[string]interface{}{
				"type":   "Quote",
				"symbol": streamingSymbol,
			})
		}

		// Add Greeks subscription for option symbols only if requested
		if containsString(dataTypes, "Greeks") && p.isOptionSymbol(symbol) {
			allSubscriptions = append(allSubscriptions, map[string]interface{}{
				"type":   "Greeks",
				"symbol": streamingSymbol,
			})
		}

		// Add Trade (Volume) subscription - automatically included with Greeks for options
		// Trade events contain dayVolume which we need for the options chain
		// Also supports explicit "Volume" or "Trade" data type requests
		if (containsString(dataTypes, "Greeks") || containsString(dataTypes, "Volume") || containsString(dataTypes, "Trade")) && p.isOptionSymbol(symbol) {
			allSubscriptions = append(allSubscriptions, map[string]interface{}{
				"type":   "Trade",
				"symbol": streamingSymbol,
			})
		}
	}

	// Batch subscriptions into chunks of 50 to avoid DXLink limits
	batchSize := 50
	totalBatches := (len(allSubscriptions) + batchSize - 1) / batchSize

	slog.Info(fmt.Sprintf("TastyTrade: Sending %d subscriptions in %d batch(es) for data_types: %v", len(allSubscriptions), totalBatches, dataTypes))

	for i := 0; i < len(allSubscriptions); i += batchSize {
		end := i + batchSize
		if end > len(allSubscriptions) {
			end = len(allSubscriptions)
		}
		batch := allSubscriptions[i:end]

		// Send subscription message for this batch
		subscriptionMsg := map[string]interface{}{
			"type":    "FEED_SUBSCRIPTION",
			"channel": 1,
			"reset":   false,
			"add":     batch,
		}

		slog.Debug(fmt.Sprintf("TastyTrade: Sending batch %d/%d with %d subscriptions", (i/batchSize)+1, totalBatches, len(batch)))

		// Use helper to write with timeout
		if err := p.writeJSONWithTimeout(subscriptionMsg); err != nil {
			slog.Error("Failed to send subscription batch", "error", err, "batch", (i/batchSize)+1, "batch_size", len(batch))
			return false, fmt.Errorf("failed to send subscription batch: %w", err)
		}

		// Small delay between batches to avoid overwhelming the server
		if i+batchSize < len(allSubscriptions) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Update subscribed symbols
	for _, symbol := range symbols {
		p.streamingState.subscribedSymbols[symbol] = true
	}

	slog.Info(fmt.Sprintf("TastyTrade: Successfully subscribed to %d symbols with %d subscriptions", len(symbols), len(allSubscriptions)))
	return true, nil
}

// UnsubscribeFromSymbols unsubscribes from real-time data for symbols via DXLink.
// Exact conversion of Python unsubscribe_from_symbols method.
func (p *TastyTradeProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	if p.streamingState == nil || !p.streamingState.isConnected {
		return true, nil // Already disconnected
	}

	slog.Info(fmt.Sprintf("TastyTrade: unsubscribe_from_symbols called with %d symbols and data_types: %v", len(symbols), dataTypes))

	// If no data_types are specified, default to both for backward compatibility
	if dataTypes == nil {
		dataTypes = []string{"Quote", "Greeks"}
	}

	// Convert symbols to TastyTrade format and prepare unsubscriptions
	var subscriptions []map[string]interface{}

	for _, symbol := range symbols {
		// Convert to streaming symbol format for DXLink
		var streamingSymbol string
		if p.isOptionSymbol(symbol) {
			streamingSymbol = p.convertToStreamerSymbol(symbol)
			slog.Debug(fmt.Sprintf("TastyTrade: Using streamer symbol for unsubscription: %s", streamingSymbol))
		} else {
			streamingSymbol = symbol // Stock symbols use as-is
		}

		for _, dataType := range dataTypes {
			if dataType == "Quote" {
				subscriptions = append(subscriptions, map[string]interface{}{
					"type":   "Quote",
					"symbol": streamingSymbol,
				})
			}

			if dataType == "Greeks" && p.isOptionSymbol(symbol) {
				subscriptions = append(subscriptions, map[string]interface{}{
					"type":   "Greeks",
					"symbol": streamingSymbol,
				})
				// Also unsubscribe from Trade (volume) which was auto-subscribed with Greeks
				subscriptions = append(subscriptions, map[string]interface{}{
					"type":   "Trade",
					"symbol": streamingSymbol,
				})
			}
		}
	}

	if len(subscriptions) == 0 {
		slog.Info("TastyTrade: No subscriptions to remove.")
		return true, nil
	}

	// Send unsubscription message
	unsubscriptionMsg := map[string]interface{}{
		"type":    "FEED_SUBSCRIPTION",
		"channel": 1,
		"reset":   false,
		"remove":  subscriptions,
	}

	// Use helper to write with timeout
	if err := p.writeJSONWithTimeout(unsubscriptionMsg); err != nil {
		slog.Error("Failed to send unsubscription message", "error", err)
		return false, err
	}

	// Update subscribed symbols
	for _, symbol := range symbols {
		delete(p.streamingState.subscribedSymbols, symbol)
	}

	slog.Info(fmt.Sprintf("TastyTrade: Unsubscribed from %d symbols", len(symbols)))
	return true, nil
}

// GetSubscribedSymbols gets currently subscribed symbols.
func (p *TastyTradeProvider) GetSubscribedSymbols() map[string]bool {
	if p.streamingState == nil {
		return make(map[string]bool)
	}
	return p.streamingState.subscribedSymbols
}

// IsStreamingConnected checks if streaming is connected.
func (p *TastyTradeProvider) IsStreamingConnected() bool {
	if p.streamingState == nil {
		return false
	}
	return p.streamingState.isConnected && p.streamingState.streamConnection != nil
}

// SetStreamingQueue sets the streaming queue.
func (p *TastyTradeProvider) SetStreamingQueue(queue chan *models.MarketData) {
	if p.streamingState == nil {
		p.initStreamingState()
	}
	p.streamingState.streamingQueue = queue
}

// SetStreamingCache sets the streaming cache.
func (p *TastyTradeProvider) SetStreamingCache(cache base.StreamingCache) {
	if p.streamingState == nil {
		p.initStreamingState()
	}
	p.streamingState.streamingCache = cache
}

// periodicKeepalive sends periodic DXLink keepalive messages to maintain connection (like Python implementation)
func (p *TastyTradeProvider) periodicKeepalive(ctx context.Context) {
	slog.Info("TastyTrade: Starting periodic keepalive task")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("TastyTrade: Periodic keepalive task cancelled")
			return
		case <-ticker.C:
			// Only send keepalive if we're connected
			if p.streamingState.isConnected && p.streamingState.streamConnection != nil {
				keepaliveMsg := map[string]interface{}{
					"type":    "KEEPALIVE",
					"channel": 0,
				}

				// Use helper to write with timeout
				if err := p.writeJSONWithTimeout(keepaliveMsg); err != nil {
					slog.Warn("TastyTrade: Periodic keepalive failed", "error", err)
					// Don't trigger recovery here - let the main loop handle it
					return
				}
				slog.Debug("TastyTrade: Periodic DXLink keepalive sent")
			} else {
				// Not connected, stop keepalive task
				slog.Debug("TastyTrade: Not connected, stopping keepalive task")
				return
			}
		}
	}
}

// dxlinkStreamingSetup executes DXLink setup sequence for streaming connection.
// Exact conversion of Python _dxlink_streaming_setup method.
func (p *TastyTradeProvider) dxlinkStreamingSetup(ctx context.Context) error {
	conn := p.streamingState.streamConnection
	timeout := 5 * time.Second

	// Helper to set deadlines
	setDeadlines := func() {
		conn.SetWriteDeadline(time.Now().Add(timeout))
		conn.SetReadDeadline(time.Now().Add(timeout))
	}

	// 1. SETUP
	setupMsg := map[string]interface{}{
		"type":                   "SETUP",
		"channel":                0,
		"version":                "0.1-DXF-JS/0.3.0",
		"keepaliveTimeout":       60,
		"acceptKeepaliveTimeout": 60,
	}

	setDeadlines()
	if err := conn.WriteJSON(setupMsg); err != nil {
		return fmt.Errorf("failed to send SETUP message: %w", err)
	}

	// Wait for SETUP response
	p.streamingState.recvLock.Lock()
	var setupResponse map[string]interface{}
	setDeadlines()
	err := conn.ReadJSON(&setupResponse)
	p.streamingState.recvLock.Unlock()
	if err != nil {
		return fmt.Errorf("failed to read SETUP response: %w", err)
	}

	if setupResponse["type"] != "SETUP" {
		return fmt.Errorf("unexpected SETUP response: %v", setupResponse)
	}

	// 2. Wait for AUTH_STATE: UNAUTHORIZED
	p.streamingState.recvLock.Lock()
	var authState map[string]interface{}
	setDeadlines()
	err = conn.ReadJSON(&authState)
	p.streamingState.recvLock.Unlock()
	if err != nil {
		return fmt.Errorf("failed to read AUTH_STATE: %w", err)
	}

	if authState["type"] != "AUTH_STATE" || authState["state"] != "UNAUTHORIZED" {
		return fmt.Errorf("unexpected AUTH_STATE response: %v", authState)
	}

	// 3. AUTHORIZE
	authMsg := map[string]interface{}{
		"type":    "AUTH",
		"channel": 0,
		"token":   p.quoteToken,
	}

	setDeadlines()
	if err := conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send AUTH message: %w", err)
	}

	// Wait for AUTH_STATE: AUTHORIZED
	p.streamingState.recvLock.Lock()
	var authSuccess map[string]interface{}
	setDeadlines()
	err = conn.ReadJSON(&authSuccess)
	p.streamingState.recvLock.Unlock()
	if err != nil {
		return fmt.Errorf("failed to read AUTH success: %w", err)
	}

	if authSuccess["type"] != "AUTH_STATE" || authSuccess["state"] != "AUTHORIZED" {
		return fmt.Errorf("authorization failed: %v", authSuccess)
	}

	// 4. CHANNEL_REQUEST
	channelMsg := map[string]interface{}{
		"type":    "CHANNEL_REQUEST",
		"channel": 1,
		"service": "FEED",
		"parameters": map[string]interface{}{
			"contract": "AUTO",
		},
	}

	setDeadlines()
	if err := conn.WriteJSON(channelMsg); err != nil {
		return fmt.Errorf("failed to send CHANNEL_REQUEST: %w", err)
	}

	// Wait for CHANNEL_OPENED
	p.streamingState.recvLock.Lock()
	var channelResponse map[string]interface{}
	setDeadlines()
	err = conn.ReadJSON(&channelResponse)
	p.streamingState.recvLock.Unlock()
	if err != nil {
		return fmt.Errorf("failed to read CHANNEL_OPENED: %w", err)
	}

	if channelResponse["type"] != "CHANNEL_OPENED" {
		return fmt.Errorf("channel open failed: %v", channelResponse)
	}

	// 5. FEED_SETUP for Quote and Greeks events
	feedSetupMsg := map[string]interface{}{
		"type":                    "FEED_SETUP",
		"channel":                 1,
		"acceptAggregationPeriod": 0.1,
		"acceptDataFormat":        "COMPACT",
		"acceptEventFields": map[string]interface{}{
			"Quote":   []string{"eventType", "eventSymbol", "bidPrice", "askPrice", "bidSize", "askSize"},
			"Greeks":  []string{"eventType", "eventSymbol", "delta", "gamma", "theta", "vega", "volatility"},
			"Summary": []string{"eventType", "eventSymbol", "openInterest", "dayOpenPrice", "dayHighPrice", "dayLowPrice", "prevDayClosePrice"},
			"Trade":   []string{"eventType", "eventSymbol", "price", "dayVolume", "size"},
		},
	}

	setDeadlines()
	if err := conn.WriteJSON(feedSetupMsg); err != nil {
		return fmt.Errorf("failed to send FEED_SETUP: %w", err)
	}

	// Wait for FEED_CONFIG
	p.streamingState.recvLock.Lock()
	var feedResponse map[string]interface{}
	setDeadlines()
	err = conn.ReadJSON(&feedResponse)
	p.streamingState.recvLock.Unlock()
	if err != nil {
		return fmt.Errorf("failed to read FEED_CONFIG: %w", err)
	}

	if feedResponse["type"] != "FEED_CONFIG" {
		return fmt.Errorf("feed setup failed: %v", feedResponse)
	}

	slog.Info("DXLink streaming setup completed successfully")

	// Send initial KEEPALIVE to prevent immediate idle timeout
	keepaliveMsg := map[string]interface{}{
		"type":    "KEEPALIVE",
		"channel": 0,
	}
	setDeadlines()
	if err := conn.WriteJSON(keepaliveMsg); err != nil {
		slog.Warn("Failed to send initial keepalive", "error", err)
	} else {
		slog.Debug("Sent initial KEEPALIVE after setup")
	}

	return nil
}

// startStreamingTask starts the streaming data processing task.
// Exact conversion of Python _start_streaming_task method.
func (p *TastyTradeProvider) startStreamingTask(ctx context.Context) {
	taskCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	p.streamingState.streamingTask = &StreamingTask{
		cancel: cancel,
		done:   done,
	}

	go func() {
		defer close(done)
		p.processStreamingData(taskCtx)
	}()

	slog.Info("TastyTrade: Started streaming data processing task")
}

// readerGoroutine performs blocking reads from WebSocket and sends messages/errors to channels
func (p *TastyTradeProvider) readerGoroutine(done chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Reader panic recovered", "panic", r)
		}
		p.streamingState.isConnected = false
		if p.streamingState.streamConnection != nil {
			p.streamingState.streamConnection.Close()
			p.streamingState.streamConnection = nil
		}

		// Safely close message channel if not already closed
		select {
		case <-done:
			// Already done/closed
		default:
			// Close message channel to signal processing loop
			// Use recover to handle potential double close if race condition persists
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Warn("Ignored panic closing messageChan", "panic", r)
					}
				}()
				close(p.streamingState.messageChan)
			}()
		}

		close(done)
	}()

	for {
		select {
		case <-p.streamingState.shutdownEvent:
			slog.Debug("Reader shutdown requested")
			return
		default:
			// Set read deadline to detect dead connections (70s > 60s keepalive)
			p.streamingState.recvLock.Lock()
			if p.streamingState.streamConnection != nil {
				p.streamingState.streamConnection.SetReadDeadline(time.Now().Add(70 * time.Second))
			}
			var message map[string]interface{}
			err := p.streamingState.streamConnection.ReadJSON(&message)
			p.streamingState.recvLock.Unlock()
			if err != nil {
				// Check for normal closure or timeout
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					slog.Info("TastyTrade: WebSocket closed normally", "error", err)
				} else if strings.Contains(err.Error(), "i/o timeout") {
					slog.Error("TastyTrade: WebSocket read timeout - connection dead")
				} else {
					slog.Error("TastyTrade: WebSocket read error", "error", err)
				}

				select {
				case p.streamingState.errorChan <- err:
					slog.Debug("Sent error to processing loop")
				default:
				}
				return // Exit reader on any error
			}
			select {
			case p.streamingState.messageChan <- message:
			default:
				slog.Warn("Message channel full, dropping message")
			}
		}
	}
}

// processStreamingData processes incoming streaming data - simplified without internal recovery.
// Recovery is handled externally by the health manager.
func (p *TastyTradeProvider) processStreamingData(ctx context.Context) {
	slog.Info("TastyTrade: Starting streaming data processor")

	var readerDone chan struct{}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("TastyTrade: Streaming processor panic recovered", "panic", r)
		}
		// Mark connection as failed on exit
		p.streamingState.isConnected = false
		if p.streamingState.streamConnection != nil {
			p.streamingState.streamConnection.Close()
			p.streamingState.streamConnection = nil
		}

		// Wait for reader goroutine to finish
		// This ensures we don't have a race condition where the old reader
		// tries to close the channel of a new connection
		slog.Info("TastyTrade: Waiting for reader goroutine to exit...")
		select {
		case <-readerDone:
			slog.Info("TastyTrade: Reader goroutine exited")
		case <-time.After(5 * time.Second):
			slog.Warn("TastyTrade: Timed out waiting for reader goroutine")
		}

		slog.Info("TastyTrade: Streaming data processor stopped")
	}()

	// Start periodic keepalive task
	keepaliveCtx, keepaliveCancel := context.WithCancel(ctx)
	defer keepaliveCancel()
	go p.periodicKeepalive(keepaliveCtx)

	// Start reader goroutine
	readerDone = make(chan struct{})
	go p.readerGoroutine(readerDone)

	// Simple message processing loop - no recovery logic
	for {
		select {
		case <-ctx.Done():
			slog.Info("TastyTrade: Streaming processor cancelled")
			return

		case <-p.streamingState.shutdownEvent:
			slog.Info("TastyTrade: Streaming processor shutdown requested")
			return

		case message, ok := <-p.streamingState.messageChan:
			if !ok {
				// Channel closed - connection lost, exit and let health manager handle recovery
				slog.Warn("TastyTrade: Message channel closed, connection lost")
				return
			}

			// Process different message types
			switch message["type"] {
			case "FEED_DATA":
				// Record that we received data (for health monitoring)
				p.BaseProviderImpl.UpdateLastDataTime()
				if feedData, ok := message["data"].([]interface{}); ok {
					p.processFeedEvents(feedData)
				}
			case "KEEPALIVE":
				slog.Debug("TastyTrade: Received keepalive from server")
				p.BaseProviderImpl.UpdateLastDataTime()
			default:
				slog.Debug(fmt.Sprintf("TastyTrade: Received message type: %s", message["type"]))
				// Log full message for debugging
				msgJSON, _ := json.Marshal(message)
				slog.Debug("TastyTrade: Full message", "message", string(msgJSON))
			}

		case err := <-p.streamingState.errorChan:
			// Connection error - exit and let health manager handle recovery
			slog.Error("TastyTrade: Connection error", "error", err)
			return

		case <-time.After(120 * time.Second):
			// Stale connection check - if no messages for 2 minutes, exit
			slog.Warn("TastyTrade: No messages received for 2 minutes, connection may be stale")
			return
		}
	}
}

// processFeedEvents processes FEED_DATA events and sends to streaming cache or queue.
// Exact conversion of Python _process_feed_events method.
func (p *TastyTradeProvider) processFeedEvents(feedData []interface{}) {
	// Debug: Log the event types present in the feed data (temporarily at INFO level for debugging)
	eventTypes := make(map[string]int)
	for _, item := range feedData {
		if itemArray, ok := item.([]interface{}); ok {
			for _, elem := range itemArray {
				if str, ok := elem.(string); ok {
					if str == "Quote" || str == "Greeks" || str == "Summary" || str == "Trade" {
						eventTypes[str]++
					}
				}
			}
		}
	}
	if len(eventTypes) > 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: Feed event types received: %v", eventTypes))
	}

	if p.streamingState.streamingQueue == nil && p.streamingState.streamingCache == nil {
		slog.Warn("TastyTrade: No streaming queue or cache available for feed events")
		return
	}

	// Process Quote events
	quoteData := p.processQuoteFeedData(feedData)

	// Process Trade events (for volume data AND last price)
	// Process Trade BEFORE sending quotes so we can merge last price into quote data
	tradeData := p.processTradeFeedData(feedData)

	// Handle Trade requests (for GetStockQuote last price)
	if len(tradeData) > 0 {
		for symbol, trade := range tradeData {
			standardSymbol := p.convertSymbolToStandardFormat(symbol)

			// Dispatch to tradeRequests channels (for GetStockQuote)
			p.streamingState.requestsLock.Lock()
			if ch, exists := p.streamingState.tradeRequests[standardSymbol]; exists {
				select {
				case ch <- trade:
				default:
				}
			}
			p.streamingState.requestsLock.Unlock()
		}
	}

	// Send Quote events to cache (with merged last price from Trade events)
	if len(quoteData) > 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: Processing %d quote updates", len(quoteData)))
		for symbol, quote := range quoteData {
			standardSymbol := p.convertSymbolToStandardFormat(symbol)

			// CRITICAL: Merge last price from Trade data into Quote data
			// This is essential for indices like NDX where bid/ask may be NaN but last price is valid
			if trade, exists := tradeData[symbol]; exists {
				if last, ok := trade["last"]; ok {
					quote["last"] = last
					slog.Debug(fmt.Sprintf("TastyTrade: Merged last=%v into quote for %s", last, symbol))
				}
			}

			// Handle quote requests
			p.streamingState.requestsLock.Lock()
			if ch, exists := p.streamingState.quoteRequests[standardSymbol]; exists {
				select {
				case ch <- quote:
				default:
				}
			}
			p.streamingState.requestsLock.Unlock()

			// Send to cache or queue
			marketData := &models.MarketData{
				Symbol:    standardSymbol,
				Data:      quote,
				DataType:  "quote",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			p.sendToCacheOrQueue(marketData)
			slog.Debug(fmt.Sprintf("TastyTrade: Sent quote update for %s", standardSymbol))
		}
	}

	// Also send Trade-only updates for symbols that didn't have Quote data
	// This ensures symbols with only Trade events still get their last price to the cache
	if len(tradeData) > 0 {
		for symbol, trade := range tradeData {
			// Skip if we already sent this symbol with quote data
			if _, hasQuote := quoteData[symbol]; hasQuote {
				continue
			}

			standardSymbol := p.convertSymbolToStandardFormat(symbol)

			// Send trade data as quote type with just the last price
			// This allows the frontend to fall back to last price when bid/ask unavailable
			quoteFromTrade := map[string]interface{}{
				"last": trade["last"],
			}

			marketData := &models.MarketData{
				Symbol:    standardSymbol,
				Data:      quoteFromTrade,
				DataType:  "quote",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			p.sendToCacheOrQueue(marketData)
			slog.Debug(fmt.Sprintf("TastyTrade: Sent trade-only quote update for %s (last=%v)", standardSymbol, trade["last"]))
		}
	}

	// Process Greeks events
	greeksData := p.processGreeksFeedData(feedData)
	if len(greeksData) > 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: Processing %d Greeks updates", len(greeksData)))
		for symbol, greeks := range greeksData {
			standardSymbol := p.convertSymbolToStandardFormat(symbol)

			// Merge volume from Trade data if available for this symbol
			if trade, exists := tradeData[symbol]; exists {
				if volume, ok := trade["volume"]; ok {
					greeks["volume"] = volume
					slog.Debug(fmt.Sprintf("TastyTrade: Merged volume=%v into Greeks for %s", volume, symbol))
				}
			}

			// Handle Greeks requests
			p.streamingState.requestsLock.Lock()
			if ch, exists := p.streamingState.greeksRequests[standardSymbol]; exists {
				select {
				case ch <- greeks:
				default:
				}
			}
			p.streamingState.requestsLock.Unlock()

			// Send to cache or queue
			marketData := &models.MarketData{
				Symbol:    standardSymbol,
				Data:      greeks,
				DataType:  "greeks",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			p.sendToCacheOrQueue(marketData)
			slog.Debug(fmt.Sprintf("TastyTrade: Sent Greeks update for %s", standardSymbol))
		}
	}

	// Process Trade events that don't have matching Greeks (standalone volume updates)
	// This handles cases where Trade updates arrive without Greeks
	if len(tradeData) > 0 {
		for symbol, trade := range tradeData {
			// Skip if we already processed this symbol with Greeks
			if _, hasGreeks := greeksData[symbol]; hasGreeks {
				continue
			}

			standardSymbol := p.convertSymbolToStandardFormat(symbol)

			// Send volume-only update as greeks type (frontend expects volume in greeks)
			volumeData := map[string]interface{}{
				"volume": trade["volume"],
			}

			marketData := &models.MarketData{
				Symbol:    standardSymbol,
				Data:      volumeData,
				DataType:  "greeks",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			p.sendToCacheOrQueue(marketData)
			slog.Debug(fmt.Sprintf("TastyTrade: Sent volume-only update for %s", standardSymbol))
		}
	}
}

// processQuoteFeedData processes FEED_DATA messages and extracts Quote events.
// Exact conversion of Python _process_quote_feed_data method.
func (p *TastyTradeProvider) processQuoteFeedData(feedData []interface{}) map[string]map[string]interface{} {
	quoteData := make(map[string]map[string]interface{})

	// DXLink COMPACT format: flat array with Quote data
	// Format: ['Quote', 'SYMBOL', bidPrice, askPrice, bidSize, askSize, 'Quote', ...]
	for _, item := range feedData {
		if itemArray, ok := item.([]interface{}); ok {
			// Parse the flat array - each Quote event has 6 consecutive elements
			i := 0
			for i+5 < len(itemArray) {
				if itemArray[i] == "Quote" {
					symbol := itemArray[i+1].(string)
					quote := map[string]interface{}{
						"bid":      itemArray[i+2],
						"ask":      itemArray[i+3],
						"bid_size": itemArray[i+4],
						"ask_size": itemArray[i+5],
					}
					quoteData[symbol] = quote
					i += 6 // Move to next Quote event (6 fields per event)
				} else {
					i++ // Skip non-Quote data
				}
			}
		}
	}

	return quoteData
}

// processGreeksFeedData processes FEED_DATA messages and extracts Greeks events.
// Exact conversion of Python _process_greeks_feed_data method.
func (p *TastyTradeProvider) processGreeksFeedData(feedData []interface{}) map[string]map[string]interface{} {
	greeksData := make(map[string]map[string]interface{})

	// DXLink COMPACT format: flat array with Greeks data
	// Format: ['Greeks', 'SYMBOL', delta, gamma, theta, vega, volatility, 'Greeks', ...]
	for _, item := range feedData {
		if itemArray, ok := item.([]interface{}); ok {
			// Parse the flat array - each Greeks event has 7 consecutive elements
			i := 0
			for i+5 < len(itemArray) {
				if itemArray[i] == "Greeks" {
					symbol := itemArray[i+1].(string)
					greeks := map[string]interface{}{
						"delta":              itemArray[i+2],
						"gamma":              itemArray[i+3],
						"theta":              itemArray[i+4],
						"vega":               itemArray[i+5],
						"implied_volatility": nil, // Default to nil
					}
					// Only set implied_volatility if it's available
					if i+6 < len(itemArray) {
						greeks["implied_volatility"] = itemArray[i+6]
					}
					greeksData[symbol] = greeks
					i += 7 // Move to next Greeks event (7 fields per event)
				} else {
					i++ // Skip non-Greeks data
				}
			}
		}
	}

	return greeksData
}

// processTradeFeedData processes FEED_DATA messages and extracts Trade events (volume and last price).
// Trade events contain dayVolume which we merge into Greeks data, and price which is the last trade price.
// We request fields: ["eventType", "eventSymbol", "price", "dayVolume", "size"]
// So each Trade event has 5 elements in COMPACT format.
func (p *TastyTradeProvider) processTradeFeedData(feedData []interface{}) map[string]map[string]interface{} {
	tradeData := make(map[string]map[string]interface{})

	// Number of fields we requested for Trade events
	const tradeFieldCount = 5 // eventType, eventSymbol, price, dayVolume, size

	for _, item := range feedData {
		if itemArray, ok := item.([]interface{}); ok {
			// Parse the flat array - each Trade event has tradeFieldCount consecutive elements
			i := 0
			for i < len(itemArray) {
				if itemArray[i] == "Trade" {
					// Make sure we have enough elements
					if i+tradeFieldCount > len(itemArray) {
						slog.Warn(fmt.Sprintf("TastyTrade: Incomplete Trade event at index %d, only %d elements remaining", i, len(itemArray)-i))
						break
					}
					symbol := itemArray[i+1].(string)
					// Fields: [0]=eventType, [1]=eventSymbol, [2]=price, [3]=dayVolume, [4]=size
					lastPrice := itemArray[i+2] // price field is the last trade price
					dayVolume := itemArray[i+3]

					slog.Debug(fmt.Sprintf("TastyTrade: Trade for %s: last=%v, dayVolume=%v", symbol, lastPrice, dayVolume))

					// Store volume and last price data
					trade := map[string]interface{}{
						"volume": dayVolume,
						"last":   lastPrice, // Add last trade price for GetStockQuote fallback
					}
					tradeData[symbol] = trade
					i += tradeFieldCount // Move to next Trade event
				} else {
					i++ // Skip non-Trade data
				}
			}
		}
	}

	if len(tradeData) > 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: Extracted %d Trade events with volume/last price", len(tradeData)))
	}

	return tradeData
}

// sendToCacheOrQueue sends market data to cache if available, otherwise to queue.
// Exact conversion of Python _send_to_cache_or_queue method.
func (p *TastyTradeProvider) sendToCacheOrQueue(marketData *models.MarketData) {
	if p.streamingState.streamingCache != nil {
		// Send to cache (async)
		go func() {
			if err := p.streamingState.streamingCache.Update(marketData); err != nil {
				slog.Error("Failed to update streaming cache", "error", err)
			}
		}()
	} else if p.streamingState.streamingQueue != nil {
		// Send to queue (non-blocking)
		select {
		case p.streamingState.streamingQueue <- marketData:
		default:
			slog.Warn("Streaming queue full, dropping market data")
		}
	}
}

// convertToStreamerSymbol converts standard OCC symbol to TastyTrade DXLink streamer format.
// Exact conversion of Python _convert_to_streamer_symbol method.
func (p *TastyTradeProvider) convertToStreamerSymbol(symbol string) string {
	if !p.isOptionSymbol(symbol) {
		return symbol
	}

	// Parse OCC format: ROOT + YYMMDD + C/P + STRIKE(8 digits)
	// Need to find the 6-digit date part in the correct position
	if len(symbol) < 15 {
		slog.Warn(fmt.Sprintf("TastyTrade: Symbol too short for option parsing: %s", symbol))
		return symbol
	}

	// Find where the date starts (6 consecutive digits)
	for i := 0; i <= len(symbol)-15; i++ { // Need at least 15 chars after position i
		datePart := symbol[i : i+6]
		if p.isAllDigits(datePart) {
			// Found the date part
			root := symbol[:i]
			date := datePart                 // YYMMDD
			optionType := symbol[i+6]        // C or P
			strikePart := symbol[i+7 : i+15] // 8 digits

			// Convert strike to actual dollar amount (OCC format is dollars * 1000)
			strikeRaw, err := strconv.Atoi(strikePart)
			if err != nil {
				slog.Error(fmt.Sprintf("TastyTrade: Invalid strike price in symbol: %s", symbol))
				return symbol
			}

			// Divide by 1000 to get actual strike price
			strikeDollars := float64(strikeRaw) / 1000

			// Convert to clean string (remove .0 if it's a whole number)
			var strikeClean string
			if strikeDollars == float64(int(strikeDollars)) {
				strikeClean = strconv.Itoa(int(strikeDollars))
			} else {
				strikeClean = fmt.Sprintf("%.3f", strikeDollars)
				// Remove trailing zeros
				strikeClean = strings.TrimRight(strikeClean, "0")
				strikeClean = strings.TrimRight(strikeClean, ".")
			}

			// Build streamer symbol: .ROOT + DATE + TYPE + STRIKE
			streamerSymbol := fmt.Sprintf(".%s%s%c%s", root, date, optionType, strikeClean)

			slog.Debug(fmt.Sprintf("TastyTrade: Converted %s -> %s", symbol, streamerSymbol))
			return streamerSymbol
		}
	}

	slog.Warn(fmt.Sprintf("TastyTrade: Could not parse option symbol: %s", symbol))
	return symbol
}

// isAllDigits checks if a string contains only digits.
func (p *TastyTradeProvider) isAllDigits(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// containsString checks if a slice contains a string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TestCredentials tests provider credentials.
// Exact conversion of Python test_credentials method.
func (p *TastyTradeProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	slog.Info("Testing TastyTrade credentials...")
	account, err := p.GetAccount(ctx)
	if err != nil {
		slog.Error("TastyTrade credentials validation failed", "error", err)
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Authentication failed: %v", err),
		}, nil
	}

	if account != nil && account.AccountID != "" {
		slog.Info("TastyTrade credentials are valid")
		return map[string]interface{}{
			"success": true,
			"message": "Successfully connected to TastyTrade.",
		}, nil
	}

	slog.Error("TastyTrade credentials validation failed: No account info returned")
	return map[string]interface{}{
		"success": false,
		"message": "Invalid credentials or no account info found.",
	}, nil
}

// Ping sends a heartbeat message to the provider to verify connection health.
func (p *TastyTradeProvider) Ping(ctx context.Context) error {
	if !p.streamingState.isConnected || p.streamingState.streamConnection == nil {
		return fmt.Errorf("streaming not connected")
	}

	// Send KEEPALIVE message
	keepaliveMsg := map[string]interface{}{
		"type":    "KEEPALIVE",
		"channel": 0,
	}

	p.streamingState.writeLock.Lock()
	defer p.streamingState.writeLock.Unlock()

	if err := p.streamingState.streamConnection.WriteJSON(keepaliveMsg); err != nil {
		return fmt.Errorf("failed to send keepalive: %w", err)
	}

	// Note: We don't wait for response here as KEEPALIVE response is handled asynchronously
	// in the reader goroutine. If the write succeeds, we assume the connection is at least
	// capable of sending data.
	return nil
}

// HealthCheck performs a health check on the TastyTrade provider.
// Exact conversion of Python health_check method.
func (p *TastyTradeProvider) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	sessionValid := false
	if err := p.ensureValidSession(ctx); err == nil {
		sessionValid = true
	}

	result := map[string]interface{}{
		"provider":           p.GetName(),
		"connected":          p.IsStreamingConnected(),
		"session_valid":      sessionValid,
		"subscribed_symbols": len(p.GetSubscribedSymbols()),
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	if p.sessionExpires != nil {
		result["session_expires_at"] = p.sessionExpires.Format(time.RFC3339)
	}

	return result, nil
}

// PreviewOrder previews a multi-leg trading order using the dry-run endpoint.
// Exact conversion of Python preview_order method.
func (p *TastyTradeProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	if err := p.ensureValidSession(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with TastyTrade: %w", err)
	}

	// Use the multi-leg transformation like Python does
	tastytradeOrder := p.transformToTastyTradeMultiLegOrder(orderData)

	endpoint := fmt.Sprintf("/accounts/%s/orders/dry-run", p.accountID)
	responseBytes, err := p.makeAuthenticatedRequest(ctx, "POST", endpoint, tastytradeOrder) // Handle error responses specially - they contain error details in the body (matching Python logic)
	if err != nil {
		errStr := err.Error()

		// Extract JSON from error message if responseBytes is nil
		var jsonData []byte
		if responseBytes != nil {
			jsonData = responseBytes
		} else {
			// Extract JSON from error string: "HTTP 422: {json...}"
			if colonIndex := strings.Index(errStr, ": "); colonIndex != -1 {
				jsonStr := errStr[colonIndex+2:]
				jsonData = []byte(jsonStr)
			}
		}

		// Try to parse the JSON data regardless of error type to extract any error messages
		if len(jsonData) == 0 {
			return map[string]interface{}{
				"status":              "error",
				"validation_errors":   []string{"Error occurred but no response data available"},
				"commission":          0,
				"cost":                0,
				"fees":                0,
				"order_cost":          0,
				"margin_change":       0,
				"buying_power_effect": 0,
				"day_trades":          0,
				"estimated_total":     0,
			}, nil
		}

		var response map[string]interface{}
		if parseErr := json.Unmarshal(jsonData, &response); parseErr == nil {

			// Try multiple ways to extract error messages
			var errorMessages []string

			// Method 1: response.error.errors[].message (TastyTrade format)
			if errData, hasError := response["error"].(map[string]interface{}); hasError {
				if errorsArray, ok := errData["errors"].([]interface{}); ok {
					for _, errorItem := range errorsArray {
						if errorMap, ok := errorItem.(map[string]interface{}); ok {
							// Try 'reason' first (more specific), then 'message' as fallback
							var errorMsg string
							if reason, ok := errorMap["reason"].(string); ok && reason != "" {
								errorMsg = reason
							} else if message, ok := errorMap["message"].(string); ok && message != "" {
								errorMsg = message
							}
							if errorMsg != "" {
								errorMessages = append(errorMessages, errorMsg)
							}
						}
					}
				}
				// Also try direct message in error object
				if len(errorMessages) == 0 {
					if message, ok := errData["message"].(string); ok && message != "" {
						errorMessages = append(errorMessages, message)
					}
				}
			}

			// Method 2: response.data.errors[] (alternative format)
			if len(errorMessages) == 0 {
				if dataObj, ok := response["data"].(map[string]interface{}); ok {
					if dataErrors, ok := dataObj["errors"].([]interface{}); ok {
						slog.Info("TastyTrade: Found data.errors array", "dataErrors", dataErrors)
						for _, errorItem := range dataErrors {
							switch e := errorItem.(type) {
							case string:
								if e != "" {
									errorMessages = append(errorMessages, e)

								}
							case map[string]interface{}:
								// Try 'reason' first (more specific), then 'message' as fallback
								var errorMsg string
								if reason, ok := e["reason"].(string); ok && reason != "" {
									errorMsg = reason

								} else if message, ok := e["message"].(string); ok && message != "" {
									errorMsg = message

								}
								if errorMsg != "" {
									errorMessages = append(errorMessages, errorMsg)
								}
							}
						}
					}
				}
			}

			// Method 3: Top-level message
			if len(errorMessages) == 0 {
				if message, ok := response["message"].(string); ok && message != "" {
					errorMessages = append(errorMessages, message)
				}
			}

			if len(errorMessages) > 0 {
				return map[string]interface{}{
					"status":              "error",
					"validation_errors":   errorMessages,
					"commission":          0,
					"cost":                0,
					"fees":                0,
					"order_cost":          0,
					"margin_change":       0,
					"buying_power_effect": 0,
					"day_trades":          0,
					"estimated_total":     0,
				}, nil
			}
		} else {
			// Return the raw data as error if we can't parse it
			return map[string]interface{}{
				"status":              "error",
				"validation_errors":   []string{fmt.Sprintf("API Error: %s", string(jsonData))},
				"commission":          0,
				"cost":                0,
				"fees":                0,
				"order_cost":          0,
				"margin_change":       0,
				"buying_power_effect": 0,
				"day_trades":          0,
				"estimated_total":     0,
			}, nil
		}
	} // If we have a response body, attempt to parse it and decide based on its contents.
	if responseBytes != nil {
		slog.Debug("TastyTrade: Parsing response body", "body", string(responseBytes))
		var response map[string]interface{}
		if parseErr := json.Unmarshal(responseBytes, &response); parseErr == nil {
			slog.Debug("TastyTrade: Successfully parsed response JSON", "response", response)

			// 1) Top-level "error" object -> gather messages and return as validation errors (like Tradier)
			if errData, hasError := response["error"].(map[string]interface{}); hasError {
				slog.Debug("TastyTrade: Found top-level error object", "error", errData)
				errorMessages := []string{}
				if errorsArray, ok := errData["errors"].([]interface{}); ok {
					slog.Debug("TastyTrade: Found errors array", "errors", errorsArray)
					for _, errorItem := range errorsArray {
						if errorMap, ok := errorItem.(map[string]interface{}); ok {
							if message, ok := errorMap["message"].(string); ok && message != "" {
								errorMessages = append(errorMessages, message)
								slog.Debug("TastyTrade: Extracted error message", "message", message)
							}
						}
					}
				}
				if len(errorMessages) == 0 {
					if message, ok := errData["message"].(string); ok && message != "" {
						errorMessages = append(errorMessages, message)
						slog.Debug("TastyTrade: Extracted error.message", "message", message)
					}
				}

				if len(errorMessages) > 0 {
					slog.Debug("TastyTrade: Returning error messages", "messages", errorMessages)
					return map[string]interface{}{
						"status":              "error",
						"validation_errors":   errorMessages,
						"commission":          0,
						"cost":                0,
						"fees":                0,
						"order_cost":          0,
						"margin_change":       0,
						"buying_power_effect": 0,
						"day_trades":          0,
						"estimated_total":     0,
					}, nil
				}
			}

			// 2) Data object present: check for warnings/errors and for successful order preview
			if dataObj, ok := response["data"].(map[string]interface{}); ok {
				slog.Debug("TastyTrade: Found data object", "data", dataObj)

				// 2a) If "warnings" exist, filter out informational warnings like Python does
				if warnings, ok := dataObj["warnings"].([]interface{}); ok && len(warnings) > 0 {
					slog.Debug("TastyTrade: Found warnings", "warnings", warnings)
					errorMessages := []string{}
					for _, warning := range warnings {
						if warningMap, ok := warning.(map[string]interface{}); ok {
							if message, ok := warningMap["message"].(string); ok && message != "" {
								// Filter out informational warnings (matching Python logic)
								if message != "Your order will begin working during next valid session." {
									errorMessages = append(errorMessages, message)
									slog.Debug("TastyTrade: Extracted warning message", "message", message)
								} else {
									slog.Debug("TastyTrade: Filtered out informational warning", "message", message)
								}
							}
						}
					}
					if len(errorMessages) > 0 {
						slog.Debug("TastyTrade: Returning error warnings (filtered)", "messages", errorMessages)
						return map[string]interface{}{
							"status":              "error",
							"validation_errors":   errorMessages,
							"commission":          0,
							"cost":                0,
							"fees":                0,
							"order_cost":          0,
							"margin_change":       0,
							"buying_power_effect": 0,
							"day_trades":          0,
							"estimated_total":     0,
						}, nil
					}
				}

				// 2b) If data.order.status indicates success, treat as success even if HTTP returned non-2xx.
				if orderDataObj, ok := dataObj["order"].(map[string]interface{}); ok {
					if statusStr, ok := orderDataObj["status"].(string); ok && strings.ToLower(statusStr) == "ok" {
						slog.Debug("TastyTrade: Found successful order status, processing as success")
						// Process successful preview and return result (ignore HTTP error if body contains usable success)
						return p.processSuccessfulPreviewResponse(response)
					}
				}

				// 2c) Some APIs may return validation details under data.errors - surface them too
				if dataErrors, ok := dataObj["errors"].([]interface{}); ok && len(dataErrors) > 0 {
					slog.Debug("TastyTrade: Found data.errors", "errors", dataErrors)
					errorMessages := []string{}
					for _, e := range dataErrors {
						switch ev := e.(type) {
						case string:
							if ev != "" {
								errorMessages = append(errorMessages, ev)
								slog.Debug("TastyTrade: Extracted data.errors string", "message", ev)
							}
						case map[string]interface{}:
							// Try 'reason' first (more specific), then 'message' as fallback
							var errorMsg string
							if reason, ok := ev["reason"].(string); ok && reason != "" {
								errorMsg = reason
								slog.Debug("TastyTrade: Extracted data.errors.reason", "reason", reason)
							} else if msg, ok := ev["message"].(string); ok && msg != "" {
								errorMsg = msg
								slog.Debug("TastyTrade: Extracted data.errors.message", "message", msg)
							}
							if errorMsg != "" {
								errorMessages = append(errorMessages, errorMsg)
							}
						}
					}
					if len(errorMessages) > 0 {
						slog.Debug("TastyTrade: Returning data.errors messages", "messages", errorMessages)
						return map[string]interface{}{
							"status":              "error",
							"validation_errors":   errorMessages,
							"commission":          0,
							"cost":                0,
							"fees":                0,
							"order_cost":          0,
							"margin_change":       0,
							"buying_power_effect": 0,
							"day_trades":          0,
							"estimated_total":     0,
						}, nil
					}
				}
			}

			// Check for top-level message as fallback
			if message, ok := response["message"].(string); ok && message != "" {
				slog.Debug("TastyTrade: Found top-level message", "message", message)
				return map[string]interface{}{
					"status":              "error",
					"validation_errors":   []string{message},
					"commission":          0,
					"cost":                0,
					"fees":                0,
					"order_cost":          0,
					"margin_change":       0,
					"buying_power_effect": 0,
					"day_trades":          0,
					"estimated_total":     0,
				}, nil
			}

			// 3) If parse succeeded and we didn't return above but there was no explicit success,
			//    still attempt to treat this as a successful preview if the response shape includes enough info.
			//    For safety, call processSuccessfulPreviewResponse if it can extract sensible values.
			if res, procErr := p.processSuccessfulPreviewResponse(response); procErr == nil {
				// If processSuccessfulPreviewResponse returned a status "ok", forward it
				if status, ok := res["status"].(string); ok && strings.ToLower(status) == "ok" {
					slog.Debug("TastyTrade: processSuccessfulPreviewResponse returned ok status")
					return res, nil
				}
			}

			slog.Debug("TastyTrade: No specific error patterns found, response parsed but no actionable data")
		} else {
			slog.Error("TastyTrade: Failed to parse response JSON", "error", parseErr, "body", string(responseBytes))
			// Parsing failed: surface parse error as validation error if we have an HTTP error (prefer not to return Go error so UI gets structured response)
			if err != nil {
				msg := fmt.Sprintf("Failed to parse preview response: %s", parseErr.Error())
				return map[string]interface{}{
					"status":              "error",
					"validation_errors":   []string{msg},
					"commission":          0,
					"cost":                0,
					"fees":                0,
					"order_cost":          0,
					"margin_change":       0,
					"buying_power_effect": 0,
					"day_trades":          0,
					"estimated_total":     0,
				}, nil
			}
		}
	}

	// If we reach here and there was an underlying transport/HTTP error with no parseable body,
	// prefer extracting human-facing messages from the response body (if any) instead of returning raw err.Error().
	if err != nil {
		// Try to parse the response body if available and extract messages by precedence.
		if responseBytes != nil {
			var parsed map[string]interface{}
			if perr := json.Unmarshal(responseBytes, &parsed); perr == nil {
				msgs := []string{}

				// 1) response["error"].errors[].message, then response["error"].message
				if errObj, ok := parsed["error"].(map[string]interface{}); ok {
					if errsArr, ok := errObj["errors"].([]interface{}); ok {
						for _, ei := range errsArr {
							if em, ok := ei.(map[string]interface{}); ok {
								if m, ok := em["message"].(string); ok && m != "" {
									msgs = append(msgs, m)
								}
							}
						}
					}
					if len(msgs) == 0 {
						if m, ok := errObj["message"].(string); ok && m != "" {
							msgs = append(msgs, m)
						}
					}
				}

				// 2) response["data"].warnings[].message (filter out informational warnings)
				if len(msgs) == 0 {
					if dataObj, ok := parsed["data"].(map[string]interface{}); ok {
						if warnings, ok := dataObj["warnings"].([]interface{}); ok && len(warnings) > 0 {
							for _, w := range warnings {
								if wm, ok := w.(map[string]interface{}); ok {
									if m, ok := wm["message"].(string); ok && m != "" {
										// Filter out informational warnings (matching Python logic)
										if m != "Your order will begin working during next valid session." {
											msgs = append(msgs, m)
										}
									}
								}
							}
						}
					}
				}

				// 3) response["data"].errors (strings or objects with "message")
				if len(msgs) == 0 {
					if dataObj, ok := parsed["data"].(map[string]interface{}); ok {
						if dataErrors, ok := dataObj["errors"].([]interface{}); ok && len(dataErrors) > 0 {
							for _, de := range dataErrors {
								switch dv := de.(type) {
								case string:
									if dv != "" {
										msgs = append(msgs, dv)
									}
								case map[string]interface{}:
									// Try 'reason' first (more specific), then 'message' as fallback
									if reason, ok := dv["reason"].(string); ok && reason != "" {
										msgs = append(msgs, reason)
									} else if m, ok := dv["message"].(string); ok && m != "" {
										msgs = append(msgs, m)
									}
								}
							}
						}
					}
				}

				// 4) fallback response["message"]
				if len(msgs) == 0 {
					if m, ok := parsed["message"].(string); ok && m != "" {
						msgs = append(msgs, m)
					}
				}

				// If we extracted any human-facing messages, return them (numeric preview fields zeroed).
				if len(msgs) > 0 {
					return map[string]interface{}{
						"status":              "error",
						"validation_errors":   msgs,
						"commission":          0,
						"cost":                0,
						"fees":                0,
						"order_cost":          0,
						"margin_change":       0,
						"buying_power_effect": 0,
						"day_trades":          0,
						"estimated_total":     0,
					}, nil
				}
			}
		}

		// Generic fallback message (do NOT include raw err.Error() which may contain HTTP status + JSON).
		slog.Debug("TastyTrade: Reached generic fallback for error extraction", "error", err, "responseAvailable", responseBytes != nil)
		return map[string]interface{}{
			"status":              "error",
			"validation_errors":   []string{"Preview failed: unable to extract error details"},
			"commission":          0,
			"cost":                0,
			"fees":                0,
			"order_cost":          0,
			"margin_change":       0,
			"buying_power_effect": 0,
			"day_trades":          0,
			"estimated_total":     0,
		}, nil
	}

	// Final fallback: unparseable/no-response
	return map[string]interface{}{
		"status":              "error",
		"validation_errors":   []string{"Unable to parse preview response"},
		"commission":          0,
		"cost":                0,
		"fees":                0,
		"order_cost":          0,
		"margin_change":       0,
		"buying_power_effect": 0,
		"day_trades":          0,
		"estimated_total":     0,
	}, nil
}

// processSuccessfulPreviewResponse processes a successful preview response from TastyTrade
func (p *TastyTradeProvider) processSuccessfulPreviewResponse(response map[string]interface{}) (map[string]interface{}, error) {
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		data = make(map[string]interface{})
	}

	feeCalculation, _ := data["fee-calculation"].(map[string]interface{})
	buyingPowerEffect, _ := data["buying-power-effect"].(map[string]interface{})
	orderData2, _ := data["order"].(map[string]interface{})

	priceEffect := getString(orderData2, "price-effect")
	orderPrice := getFloat(orderData2, "price")
	totalFees := getFloat(feeCalculation, "total-fees")
	commission := getFloat(feeCalculation, "commission")
	fees := totalFees - commission
	marginChange := getFloat(buyingPowerEffect, "change-in-margin-requirement")
	buyingPowerImpact := getFloat(buyingPowerEffect, "impact")

	// Calculate estimated_total based on price-effect
	var estimatedTotal float64
	if strings.ToLower(priceEffect) == "credit" {
		estimatedTotal = (orderPrice * 100) - totalFees
	} else {
		estimatedTotal = (orderPrice * 100) + totalFees
	}

	return map[string]interface{}{
		"status":              "ok",
		"commission":          commission,
		"cost":                orderPrice,
		"fees":                fees,
		"order_cost":          orderPrice,
		"margin_change":       marginChange,
		"buying_power_effect": buyingPowerImpact,
		"day_trades":          0, // Not provided in dry-run
		"validation_errors":   []string{},
		"estimated_total":     estimatedTotal,
	}, nil
}

// === Helper Methods ===

// transformTastyTradePosition transforms TastyTrade position to our standard model.
// Exact conversion of Python _transform_tastytrade_position method.
func (p *TastyTradeProvider) transformTastyTradePosition(rawPosition map[string]interface{}) *models.Position {
	symbol := getString(rawPosition, "symbol")

	// Convert TastyTrade symbol to standard OCC format for UI consistency - same as Python
	standardSymbol := p.convertSymbolToStandardFormat(symbol)

	// Map TastyTrade instrument-type to our standard asset_class - same as Python
	instrumentType := getString(rawPosition, "instrument-type")
	var assetClass string
	if instrumentType == "Equity" {
		assetClass = "us_equity"
	} else if instrumentType == "Equity Option" {
		assetClass = "us_option"
	} else {
		assetClass = "unknown"
	}

	// Determine side based on quantity - same as Python
	qty := getFloat(rawPosition, "quantity")
	var side string
	if qty >= 0 {
		side = "long"
	} else {
		side = "short"
	}

	// Get TastyTrade-specific fields - same as Python
	avgOpenPrice := getFloat(rawPosition, "average-open-price")
	closePrice := getFloat(rawPosition, "close-price")
	quantity := qty
	multiplier := getFloat(rawPosition, "multiplier")
	if multiplier == 0 {
		multiplier = 1 // Default multiplier
	}

	// Calculate cost basis (total amount paid/received for the position) - same as Python
	costBasis := avgOpenPrice * quantity * multiplier

	// Calculate current market value - same as Python
	marketValue := closePrice * quantity * multiplier

	// Calculate unrealized P/L - same as Python
	unrealizedPL := marketValue - costBasis

	// Handle cost-effect (Credit means we received money, so cost_basis should be negative) - same as Python
	costEffect := getString(rawPosition, "cost-effect")
	if costEffect == "Credit" {
		costBasis = -abs(costBasis) // Make cost basis negative for credits
	}

	// Handle date-acquired field - same as Python
	dateAcquired := getString(rawPosition, "created-at")
	var dateAcquiredPtr *string
	if dateAcquired != "" {
		dateAcquiredPtr = &dateAcquired
	}

	return &models.Position{
		Symbol:        standardSymbol, // UI gets standard OCC format
		Qty:           quantity,
		Side:          side,
		MarketValue:   marketValue,
		CostBasis:     costBasis,
		UnrealizedPL:  unrealizedPL,
		CurrentPrice:  closePrice,
		AvgEntryPrice: avgOpenPrice, // Use average-open-price directly
		AssetClass:    assetClass,
		DateAcquired:  dateAcquiredPtr,
	}
}

// convertSymbolToStandardFormat converts TastyTrade symbol to standard OCC format.
// Handles both:
// - TastyTrade API symbols with spaces: "SPY   251114C00595000" -> "SPY251114C00595000"
// - TastyTrade streamer symbols: ".TSLA251114P315" -> "TSLA251114P00315000"
// Exact conversion of Python convert_symbol_to_standard_format method.
func (p *TastyTradeProvider) convertSymbolToStandardFormat(symbol string) string {
	// Handle TastyTrade API symbols with extra spaces
	if !strings.HasPrefix(symbol, ".") && strings.Contains(symbol, "  ") { // Two or more spaces
		// Remove extra spaces from TastyTrade API symbols
		// Normalize all whitespace to single spaces, then remove all spaces
		cleanedSymbol := strings.Join(strings.Fields(symbol), "")
		slog.Debug(fmt.Sprintf("TastyTrade: Cleaned API symbol %s -> %s", symbol, cleanedSymbol))
		return cleanedSymbol
	}

	// Handle streamer symbols (start with dot)
	if strings.HasPrefix(symbol, ".") {
		// Remove the leading dot
		symbolNoDot := symbol[1:]

		// Parse the streamer symbol format: ROOT + YYMMDD + C/P + STRIKE
		if len(symbolNoDot) < 10 {
			slog.Warn(fmt.Sprintf("TastyTrade: Streamer symbol too short: %s", symbol))
			return symbol
		}

		// Find where the date starts (6 consecutive digits)
		for i := 0; i <= len(symbolNoDot)-9; i++ { // Need at least 10 chars after position i
			datePart := symbolNoDot[i : i+6]
			if p.isAllDigits(datePart) {
				// Found the date part
				root := symbolNoDot[:i]
				date := datePart                // YYMMDD
				optionType := symbolNoDot[i+6]  // C or P
				strikePart := symbolNoDot[i+7:] // Strike price

				// Convert strike back to 8-digit OCC format (multiply by 1000)
				strikeDollars, err := strconv.ParseFloat(strikePart, 64)
				if err != nil {
					slog.Error(fmt.Sprintf("TastyTrade: Invalid strike price in streamer symbol: %s", symbol))
					return symbol
				}

				strikeRaw := int(strikeDollars * 1000)
				strikeFormatted := fmt.Sprintf("%08d", strikeRaw) // 8 digits with leading zeros

				// Build standard OCC symbol: ROOT + DATE + TYPE + STRIKE
				standardSymbol := fmt.Sprintf("%s%s%c%s", root, date, optionType, strikeFormatted)

				slog.Debug(fmt.Sprintf("TastyTrade: Converted streamer %s -> standard %s", symbol, standardSymbol))
				return standardSymbol
			}
		}

		slog.Warn(fmt.Sprintf("TastyTrade: Could not parse streamer symbol: %s", symbol))
		return symbol
	}

	// If no special handling needed, return as-is
	return symbol
}

// Convert OCC -> TastyTrade format (ROOT padded to 6 chars with spaces + rest)
func (p *TastyTradeProvider) convertToTastytradeFormat(symbol string) string {
	// Only option symbols
	if !p.isOptionSymbol(symbol) {
		return symbol
	}

	// Ensure symbol long enough to parse
	if len(symbol) >= 15 {
		// Find the date part by locating 6 consecutive digits (YYMMDD)
		for i := 1; i <= len(symbol)-14; i++ {
			potentialDate := symbol[i : i+6]
			if p.isAllDigits(potentialDate) {
				// basic year sanity: YY >= 20
				yearPrefix := potentialDate[:2]
				if y, err := strconv.Atoi(yearPrefix); err == nil && y >= 20 {
					root := symbol[:i]
					dateAndRest := symbol[i:]
					// Left-justify root to 6 chars, pad with spaces
					paddedRoot := fmt.Sprintf("%-6s", root)
					tasty := paddedRoot + dateAndRest
					slog.Debug(fmt.Sprintf("TastyTrade: Converted OCC -> TastyTrade: %s -> %s", symbol, tasty))
					return tasty
				}
			}
		}
	}

	// Fallback - return original
	return symbol
}

// transformToTastyTradeOrder transforms standard order to TastyTrade format.
func (p *TastyTradeProvider) transformToTastyTradeOrder(orderData map[string]interface{}) map[string]interface{} {
	// Map order types exactly as Python
	orderTypeMap := map[string]string{
		"market":     "Market",
		"limit":      "Limit",
		"stop":       "Stop",
		"stop_limit": "Stop Limit",
	}

	// Map actions exactly as Python
	actionMap := map[string]string{
		"buy":           "Buy to Open",
		"sell":          "Sell to Close",
		"buy_to_open":   "Buy to Open",
		"sell_to_open":  "Sell to Open",
		"buy_to_close":  "Buy to Close",
		"sell_to_close": "Sell to Close",
	}

	// Map time in force exactly as Python
	tifMap := map[string]string{
		"day": "Day",
		"gtc": "GTC",
		"ioc": "IOC",
		"fok": "FOK",
	}

	symbol := getString(orderData, "symbol")
	side := getString(orderData, "side")
	orderType := getString(orderData, "order_type")
	tif := getString(orderData, "time_in_force")

	// Handle both 'quantity' and 'qty' field names - exactly as Python
	quantity := getFloat(orderData, "quantity")
	if quantity == 0 {
		quantity = getFloat(orderData, "qty")
	}

	// Determine instrument type - exactly as Python
	instrumentType := "Equity"
	if p.isOptionSymbol(symbol) {
		instrumentType = "Equity Option"
	}

	// For TastyTrade, ALL orders (stocks and options) use legs array format
	// This matches the Python implementation exactly
	tastytrade_order := map[string]interface{}{
		"order-type":    orderTypeMap[orderType],
		"time-in-force": tifMap[tif],
		"legs": []map[string]interface{}{
			{
				"instrument-type": instrumentType,
				"action":          actionMap[side],
				"quantity":        int(quantity), // Convert to int as Python does
				"symbol":          symbol,        // Use symbol as-is for stocks
			},
		},
	}

	// Add price for limit orders - handle multiple price field names exactly as Python
	if orderType == "limit" || orderType == "stop_limit" {
		var price float64

		// Check multiple possible price fields exactly as Python
		if val := getFloat(orderData, "price"); val != 0 {
			price = val
		} else if val := getFloat(orderData, "limit_price"); val != 0 {
			price = val
		} else if val := getFloat(orderData, "net_price"); val != 0 {
			price = val
		} else if val := getFloat(orderData, "premium"); val != 0 {
			price = val
		}

		if price != 0 {
			// TastyTrade requires positive prices - exactly as Python
			if price < 0 {
				tastytrade_order["price"] = -price // Make positive
				tastytrade_order["price-effect"] = "Credit"
			} else {
				tastytrade_order["price"] = price
				// Determine price-effect based on side - exactly as Python logic
				if side == "sell" || side == "sell_to_open" || side == "sell_to_close" {
					tastytrade_order["price-effect"] = "Credit"
				} else {
					tastytrade_order["price-effect"] = "Debit"
				}
			}
		}
	}

	// Add stop price for stop orders - exactly as Python
	if stopPrice := getFloat(orderData, "stop_price"); stopPrice != 0 {
		tastytrade_order["stop-trigger"] = stopPrice
	}

	return tastytrade_order
}

// transformToTastyTradeMultiLegOrder transforms standard multi-leg order format to TastyTrade format.
// Exact conversion of Python _transform_to_tastytrade_multi_leg_order method.
func (p *TastyTradeProvider) transformToTastyTradeMultiLegOrder(orderData map[string]interface{}) map[string]interface{} {
	// Map order types
	orderTypeMap := map[string]string{
		"market":     "Market",
		"limit":      "Limit",
		"stop":       "Stop",
		"stop_limit": "Stop Limit",
	}

	// Map actions
	actionMap := map[string]string{
		"buy":           "Buy to Open",
		"sell":          "Sell to Close",
		"buy_to_open":   "Buy to Open",
		"sell_to_open":  "Sell to Open",
		"buy_to_close":  "Buy to Close",
		"sell_to_close": "Sell to Close",
	}

	// Map time in force
	tifMap := map[string]string{
		"day": "Day",
		"gtc": "GTC",
		"ioc": "IOC",
		"fok": "FOK",
	}

	// Build legs with proper symbol conversion
	var legs []map[string]interface{}
	orderLegs, _ := orderData["legs"].([]interface{})

	// If no legs provided, this is a single-leg order - create leg from order data
	if len(orderLegs) == 0 {
		symbol := getString(orderData, "symbol")
		side := getString(orderData, "side")
		// Frontend sends "qty" but we need to check both "qty" and "quantity"
		quantity := getFloat(orderData, "qty")
		if quantity == 0 {
			quantity = getFloat(orderData, "quantity")
		}
		if quantity == 0 {
			quantity = 1 // Default to 1 if no quantity specified
		}

		// Determine instrument type
		instrumentType := "Equity"
		if p.isOptionSymbol(symbol) {
			instrumentType = "Equity Option"
		}

		// Determine the correct action for sell orders
		action := actionMap[side]
		if side == "sell" {
			// Check if this is a short sell
			if isShortSell, ok := orderData["is_short_sell"].(bool); ok && isShortSell {
				action = "Sell to Open" // Short selling
			} else {
				action = "Sell to Close" // Closing existing position
			}
		}

		// Convert symbol to TastyTrade provider format for API if option
		tastytradeSymbol := symbol
		if p.isOptionSymbol(symbol) {
			tastytradeSymbol = p.convertToTastytradeFormat(symbol)
		}

		legs = append(legs, map[string]interface{}{
			"instrument-type": instrumentType,
			"action":          action,
			"quantity":        int(quantity),
			"symbol":          tastytradeSymbol,
		})
	} else {
		// Process multi-leg order
		for _, leg := range orderLegs {
			legMap, ok := leg.(map[string]interface{})
			if !ok {
				continue
			}

			symbol := getString(legMap, "symbol")
			side := getString(legMap, "side")
			// Frontend sends "qty" but we need to check both "qty" and "quantity"
			quantity := getFloat(legMap, "qty")
			if quantity == 0 {
				quantity = getFloat(legMap, "quantity")
			}
			if quantity == 0 {
				quantity = 1 // Default to 1 if no quantity specified
			}

			// Determine instrument type
			instrumentType := "Equity"
			if p.isOptionSymbol(symbol) {
				instrumentType = "Equity Option"
			}

			// Convert symbol to TastyTrade provider format for API if option
			tastytradeSymbol := symbol
			if p.isOptionSymbol(symbol) {
				tastytradeSymbol = p.convertToTastytradeFormat(symbol)
			}

			legs = append(legs, map[string]interface{}{
				"instrument-type": instrumentType,
				"action":          actionMap[side],
				"quantity":        int(quantity),
				"symbol":          tastytradeSymbol,
			})
		}
	}

	// Build TastyTrade multi-leg order
	orderType := getString(orderData, "order_type")
	if orderType == "" {
		orderType = "limit"
	}

	tif := getString(orderData, "time_in_force")
	if tif == "" {
		tif = "day"
	}

	tastytradeOrder := map[string]interface{}{
		"order-type":    orderTypeMap[orderType],
		"time-in-force": tifMap[tif],
		"legs":          legs,
	}

	// Add price and price-effect for limit orders (required for TastyTrade)
	if orderType == "limit" || orderType == "stop_limit" {
		// Check multiple possible price fields
		price := getFloat(orderData, "price")
		if price == 0 {
			price = getFloat(orderData, "limit_price")
		}
		if price == 0 {
			price = getFloat(orderData, "net_price")
		}
		if price == 0 {
			price = getFloat(orderData, "premium")
		}

		if price != 0 { // Allow 0 price but not missing price
			// TastyTrade requires positive prices and correct price-effect
			tastytradeOrder["price"] = abs(price) // Always make price positive

			// Determine price-effect based on order side and price sign
			side := getString(orderData, "side")
			if price < 0 {
				// Negative price explicitly indicates credit
				tastytradeOrder["price-effect"] = "Credit"
			} else if side == "sell" || side == "sell_to_open" || side == "sell_to_close" {
				// Selling typically receives credit (you get money)
				tastytradeOrder["price-effect"] = "Credit"
			} else {
				// For multi-leg orders or buy orders, determine from price sign
				// If price is positive and we don't have a clear sell side, default to Debit
				tastytradeOrder["price-effect"] = "Debit"
			}
		}
	}

	return tastytradeOrder
}

// transformTastyTradeOrder transforms TastyTrade order response to our standard model.
// Exact conversion of Python _transform_tastytrade_order method.
func (p *TastyTradeProvider) transformTastyTradeOrder(rawOrder map[string]interface{}) *models.Order {
	// Map TastyTrade status to our standard status
	statusMap := map[string]string{
		"Live":      "open",
		"Routed":    "open", // TastyTrade uses "Routed" for submitted orders
		"Filled":    "filled",
		"Cancelled": "cancelled",
		"Rejected":  "rejected",
		"Expired":   "expired",
	}

	// Map TastyTrade order type to our standard format
	orderTypeMap := map[string]string{
		"Market":     "market",
		"Limit":      "limit",
		"Stop":       "stop",
		"Stop Limit": "stop_limit",
	}

	// Extract legs information
	legs, _ := rawOrder["legs"].([]interface{})

	var symbol string
	var quantity float64
	var side string
	var assetClass string

	// For single-leg orders, use the first leg
	if len(legs) > 0 {
		if firstLeg, ok := legs[0].(map[string]interface{}); ok {
			symbol = getString(firstLeg, "symbol")
			// Convert TastyTrade symbol to standard OCC format for UI consistency
			symbol = p.convertSymbolToStandardFormat(symbol)
			quantity = getFloat(firstLeg, "quantity")
			side = p.mapTastyTradeActionToSide(getString(firstLeg, "action"))

			// Determine asset class from instrument type
			instrumentType := getString(firstLeg, "instrument-type")
			if instrumentType == "Equity Option" {
				assetClass = "us_option"
			} else if instrumentType == "Equity" {
				assetClass = "us_equity"
			} else {
				assetClass = "unknown"
			}
		}
	} else {
		symbol = ""
		quantity = 0
		side = "buy"
		assetClass = "unknown"
	}

	// Calculate total filled quantity from all legs
	totalFilledQty := 0.0
	for _, leg := range legs {
		if legMap, ok := leg.(map[string]interface{}); ok {
			legQuantity := getFloat(legMap, "quantity")
			remainingQuantity := getFloat(legMap, "remaining-quantity")
			filledQty := legQuantity - remainingQuantity
			totalFilledQty += filledQty
		}
	}

	// Parse submitted timestamp
	submittedAt := getString(rawOrder, "received-at")
	if submittedAt == "" {
		submittedAt = time.Now().Format(time.RFC3339)
	}

	// Handle limit_price with credit/debit conversion (same as Tradier)
	var limitPrice *float64
	if priceVal := getFloat(rawOrder, "price"); priceVal != 0 {
		limitPrice = &priceVal
	}
	priceEffect := strings.ToLower(getString(rawOrder, "price-effect"))

	// Convert credit orders to negative limit_price for UI consistency
	if priceEffect == "credit" && limitPrice != nil {
		negative := -*limitPrice
		limitPrice = &negative
	}

	// Handle avg_fill_price with same credit/debit logic
	var avgFillPrice *float64
	if filledPriceVal := getFloat(rawOrder, "filled-price"); filledPriceVal != 0 {
		if priceEffect == "credit" {
			negative := -filledPriceVal
			avgFillPrice = &negative
		} else {
			avgFillPrice = &filledPriceVal
		}
	}

	// Handle stop price
	var stopPrice *float64
	if stopVal := getFloat(rawOrder, "stop-trigger"); stopVal != 0 {
		stopPrice = &stopVal
	}

	// Handle filled_at timestamp
	var filledAt *string
	if getString(rawOrder, "status") == "Filled" {
		if updatedAt := getString(rawOrder, "updated-at"); updatedAt != "" {
			filledAt = &updatedAt
		}
	}

	// Handle legs for multi-leg orders
	var orderLegs []models.OrderLeg
	if len(legs) > 1 {
		orderLegs = p.transformTastyTradeLegs(legs)
	}

	// Get status with fallback to "unknown" - same as Python
	rawStatus := getString(rawOrder, "status")
	status := statusMap[rawStatus]
	if status == "" {
		status = "unknown"
	}

	// Get order type with fallback to "limit" - same as Python
	rawOrderType := getString(rawOrder, "order-type")
	orderType := orderTypeMap[rawOrderType]
	if orderType == "" {
		orderType = "limit" // Default fallback
	}

	return &models.Order{
		ID:           getString(rawOrder, "id"),
		Symbol:       symbol,
		AssetClass:   assetClass,
		Side:         side,
		OrderType:    orderType,
		Qty:          quantity,
		FilledQty:    totalFilledQty,
		LimitPrice:   limitPrice,
		StopPrice:    stopPrice,
		AvgFillPrice: avgFillPrice,
		Status:       status,
		TimeInForce:  strings.ToLower(getString(rawOrder, "time-in-force")),
		SubmittedAt:  submittedAt,
		FilledAt:     filledAt,
		Legs:         orderLegs,
	}
}

// mapTastyTradeActionToSide maps TastyTrade action to our standard side.
// Exact conversion of Python _map_tastytrade_action_to_side method.
func (p *TastyTradeProvider) mapTastyTradeActionToSide(action string) string {
	actionLower := strings.ToLower(action)
	if strings.Contains(actionLower, "buy") {
		return "buy"
	} else if strings.Contains(actionLower, "sell") {
		return "sell"
	} else {
		return "buy" // Default fallback
	}
}

// transformTastyTradeLegs transforms TastyTrade legs to our standard format.
// Exact conversion of Python _transform_tastytrade_legs method.
func (p *TastyTradeProvider) transformTastyTradeLegs(legs []interface{}) []models.OrderLeg {
	var transformedLegs []models.OrderLeg
	for _, leg := range legs {
		if legMap, ok := leg.(map[string]interface{}); ok {
			transformedLegs = append(transformedLegs, models.OrderLeg{
				Symbol: p.convertSymbolToStandardFormat(getString(legMap, "symbol")),
				Side:   p.mapTastyTradeActionToSide(getString(legMap, "action")),
				Qty:    getFloat(legMap, "quantity"),
			})
		}
	}
	return transformedLegs
}

// getString gets a string value from map with default empty string.
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
		// Handle numbers as strings (for fields like ID that can be either)
		if num, ok := val.(float64); ok {
			return fmt.Sprintf("%.0f", num)
		}
		if num, ok := val.(int); ok {
			return fmt.Sprintf("%d", num)
		}
	}
	return ""
}

// getFloat gets a float64 value from map with default 0.
func getFloat(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

// interfaceToFloat64 converts interface{} to *float64, handling strings and numbers.
func (p *TastyTradeProvider) interfaceToFloat64(val interface{}) *float64 {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return &f
		}
	}

	return nil
}

// === Historical Data Helper Methods ===

// getQuoteToken gets quote token for DXLink streaming.
// Exact conversion of Python _get_quote_token method.
func (p *TastyTradeProvider) getQuoteToken(ctx context.Context) error {
	if err := p.ensureValidSession(ctx); err != nil {
		return err
	}

	endpoint := "/api-quote-tokens"
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		// If unauthorized, clear session so next attempt refreshes it
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized") {
			slog.Warn("TastyTrade: Quote token request unauthorized, clearing session to force refresh")
			p.sessionToken = ""
			p.sessionExpires = nil
		}
		return fmt.Errorf("failed to get quote token: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Token     string `json:"token"`
			DXLinkURL string `json:"dxlink-url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return fmt.Errorf("failed to parse quote token response: %w", err)
	}

	if apiResponse.Data.Token == "" || apiResponse.Data.DXLinkURL == "" {
		return fmt.Errorf("failed to get quote token from TastyTrade")
	}

	p.quoteToken = apiResponse.Data.Token
	p.dxlinkURL = apiResponse.Data.DXLinkURL

	// Quote tokens are valid for 24 hours
	expires := time.Now().Add(24 * time.Hour)
	p.quoteExpires = &expires

	slog.Debug("TastyTrade quote token obtained successfully")
	return nil
}

// mapTimeframeToDXLink converts our timeframe to DXLink candle format.
// Exact conversion of Python _map_timeframe_to_dxlink method.
func (p *TastyTradeProvider) mapTimeframeToDXLink(timeframe string) string {
	timeframeMap := map[string]string{
		"1m":  "1m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1h",
		"4h":  "4h",
		"D":   "d",  // Daily should be 'd', not '1d'
		"W":   "w",  // Weekly should be 'w', not '1w'
		"M":   "mo", // Monthly should be 'mo', not '1M'
	}
	return timeframeMap[timeframe]
}

// calculateFromTime calculates Unix timestamp for DXLink fromTime parameter.
// Exact conversion of Python _calculate_from_time method.
func (p *TastyTradeProvider) calculateFromTime(startDate *string, timeframe string) int64 {
	if startDate != nil && *startDate != "" {
		// Parse provided start date and convert to UTC midnight for DXLink
		startDt, err := time.Parse("2006-01-02", *startDate)
		if err != nil {
			slog.Warn(fmt.Sprintf("Could not parse start_date %s: %v", *startDate, err))
			// Fallback to default calculation
		} else {
			// For DXLink, we need to send UTC midnight timestamp
			startDtUTC := startDt.UTC()

			// For daily timeframes, use the exact date without buffer
			// For intraday timeframes, apply a small buffer
			if timeframe == "D" {
				fromTimeMs := startDtUTC.UnixMilli()
				return fromTimeMs
			} else {
				// Intraday: Apply small buffer to avoid edge cases
				bufferHours := 1 // 1 hour buffer for intraday
				startDtUTCBuffered := startDtUTC.Add(-time.Duration(bufferHours) * time.Hour)
				fromTimeMs := startDtUTCBuffered.UnixMilli()
				return fromTimeMs
			}
		}
	}

	// Calculate default lookback based on timeframe
	now := time.Now()
	var startDt time.Time

	switch timeframe {
	case "1m", "5m", "15m", "30m":
		// Intraday: last 5 days
		startDt = now.AddDate(0, 0, -5)
	case "1h", "4h":
		// Hourly: last 30 days
		startDt = now.AddDate(0, 0, -30)
	case "D":
		// Daily: last 365 days
		startDt = now.AddDate(0, 0, -365)
	case "W":
		// Weekly: last 2 years
		startDt = now.AddDate(0, 0, -730)
	case "M":
		// Monthly: last 5 years
		startDt = now.AddDate(0, 0, -1825)
	default:
		// Default: last 30 days
		startDt = now.AddDate(0, 0, -30)
	}

	// Convert to UTC for consistency
	startDtUTC := startDt.UTC()

	// Apply buffer for default lookbacks
	var bufferHours int
	if timeframe == "D" {
		bufferHours = 24 // 1 day buffer for daily
	} else {
		bufferHours = 1 // 1 hour for others
	}

	startDtUTCBuffered := startDtUTC.Add(-time.Duration(bufferHours) * time.Hour)
	fromTimeMs := startDtUTCBuffered.UnixMilli()
	return fromTimeMs
}

// transformDXLinkCandle transforms DXLink candle event to standard OHLCV format.
// Exact conversion of Python _transform_dxlink_candle method.
func (p *TastyTradeProvider) transformDXLinkCandle(candleData map[string]interface{}, timeframe string) map[string]interface{} {
	// Extract timestamp and convert to appropriate format
	timestampRaw, ok := candleData["time"]
	if !ok {
		slog.Warn("Missing timestamp in candle data")
		return nil
	}

	timestamp, ok := timestampRaw.(int64)
	if !ok {
		slog.Warn("Invalid timestamp format in candle data")
		return nil
	}

	// DXLink time is in milliseconds UTC, convert to datetime in UTC
	dtUTC := time.UnixMilli(timestamp).UTC()

	var timeStr string
	if timeframe == "1m" || timeframe == "5m" || timeframe == "15m" || timeframe == "30m" || timeframe == "1h" || timeframe == "4h" {
		// Intraday - convert to Eastern Time and include time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			// Fallback to UTC if timezone loading fails
			timeStr = dtUTC.Format("2006-01-02 15:04")
		} else {
			dtET := dtUTC.In(loc)
			timeStr = dtET.Format("2006-01-02 15:04")
		}
	} else {
		// Daily+ - DXLink sends midnight UTC timestamps, use UTC date directly
		timeStr = dtUTC.Format("2006-01-02")
	}

	// Extract OHLCV data with proper NaN handling for volume
	volumeRaw := candleData["volume"]
	var volume int64
	if volumeRaw == nil || volumeRaw == "NaN" {
		volume = 0
	} else {
		switch v := volumeRaw.(type) {
		case float64:
			volume = int64(v)
		case int64:
			volume = v
		case int:
			volume = int64(v)
		default:
			volume = 0
		}
	}

	// Extract OHLC data and handle null values properly
	openVal := candleData["open"]
	highVal := candleData["high"]
	lowVal := candleData["low"]
	closeVal := candleData["close"]

	// Convert to float64 if not null, otherwise keep as nil
	var openPrice, highPrice, lowPrice, closePrice *float64
	if openVal != nil {
		if f, ok := openVal.(float64); ok {
			openPrice = &f
		}
	}
	if highVal != nil {
		if f, ok := highVal.(float64); ok {
			highPrice = &f
		}
	}
	if lowVal != nil {
		if f, ok := lowVal.(float64); ok {
			lowPrice = &f
		}
	}
	if closeVal != nil {
		if f, ok := closeVal.(float64); ok {
			closePrice = &f
		}
	}

	result := map[string]interface{}{
		"time":   timeStr,
		"volume": volume,
	}

	// Only add OHLC if they're not nil
	if openPrice != nil {
		result["open"] = *openPrice
	}
	if highPrice != nil {
		result["high"] = *highPrice
	}
	if lowPrice != nil {
		result["low"] = *lowPrice
	}
	if closePrice != nil {
		result["close"] = *closePrice
	}

	return result
}

// isValidOHLC checks if OHLC data is valid (not null or NaN).
func (p *TastyTradeProvider) isValidOHLC(bar map[string]interface{}) bool {
	// Check if all OHLC values are present and valid
	openVal, hasOpen := bar["open"]
	highVal, hasHigh := bar["high"]
	lowVal, hasLow := bar["low"]
	closeVal, hasClose := bar["close"]

	if !hasOpen || !hasHigh || !hasLow || !hasClose {
		return false
	}

	// Check if values are valid floats (not NaN)
	if openVal == nil || highVal == nil || lowVal == nil || closeVal == nil {
		return false
	}

	// Additional check for float64 type and NaN
	if open, ok := openVal.(float64); ok {
		if open != open { // NaN check
			return false
		}
	}
	if high, ok := highVal.(float64); ok {
		if high != high { // NaN check
			return false
		}
	}
	if low, ok := lowVal.(float64); ok {
		if low != low { // NaN check
			return false
		}
	}
	if close, ok := closeVal.(float64); ok {
		if close != close { // NaN check
			return false
		}
	}

	return true
}

// sortBarsByTime sorts bars by their time field.
func (p *TastyTradeProvider) sortBarsByTime(bars []map[string]interface{}) {
	// Simple bubble sort by time string (works for ISO format)
	for i := 0; i < len(bars)-1; i++ {
		for j := 0; j < len(bars)-i-1; j++ {
			time1, ok1 := bars[j]["time"].(string)
			time2, ok2 := bars[j+1]["time"].(string)
			if ok1 && ok2 && time1 > time2 {
				bars[j], bars[j+1] = bars[j+1], bars[j]
			}
		}
	}
}

// filterByEndDate filters bars to only include those on or before end_date.
func (p *TastyTradeProvider) filterByEndDate(bars []map[string]interface{}, endDate string) []map[string]interface{} {
	// Parse end_date
	endDt, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		slog.Error(fmt.Sprintf("Error parsing end_date %s: %v", endDate, err))
		return bars
	}
	endDateStr := endDt.Format("2006-01-02")

	var filteredResult []map[string]interface{}
	for _, bar := range bars {
		if timeStr, ok := bar["time"].(string); ok {
			// Extract date part (YYYY-MM-DD)
			var barDate string
			if len(timeStr) >= 10 {
				barDate = timeStr[:10]
			} else {
				barDate = timeStr
			}

			if barDate <= endDateStr {
				filteredResult = append(filteredResult, bar)
			}
		}
	}

	return filteredResult
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// NewDXLinkCandleClient creates a new DXLink candle client.
func NewDXLinkCandleClient(dxlinkURL, quoteToken string) *DXLinkCandleClient {
	return &DXLinkCandleClient{
		dxlinkURL:  dxlinkURL,
		quoteToken: quoteToken,
		channelID:  1, // Use channel 1 for candle data
	}
}

// DXLinkCandleClient is a dedicated client for DXLink Candle Events to fetch historical data.
type DXLinkCandleClient struct {
	dxlinkURL  string
	quoteToken string
	channelID  int
}

// GetCandles gets historical candles via DXLink streaming.
func (c *DXLinkCandleClient) GetCandles(ctx context.Context, symbol, timeframe string, fromTime int64, limit int) ([]map[string]interface{}, error) {
	slog.Debug(fmt.Sprintf("DXLink: Getting candles for %s %s from %d", symbol, timeframe, fromTime))

	// Connect to DXLink WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.dxlinkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DXLink WebSocket: %w", err)
	}
	defer conn.Close()

	// Execute DXLink setup sequence
	if err := c.dxlinkSetupSequence(ctx, conn); err != nil {
		slog.Error("DXLink setup sequence failed", "error", err)
		return nil, err
	}

	// Subscribe and collect candles
	candles, err := c.subscribeAndCollectCandles(ctx, conn, symbol, timeframe, fromTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to collect candles: %w", err)
	}

	slog.Debug(fmt.Sprintf("DXLink: Collected %d candles for %s", len(candles), symbol))
	return candles, nil
}

// dxlinkSetupSequence executes the full DXLink setup sequence.
// Exact conversion of Python _dxlink_setup_sequence method.
func (c *DXLinkCandleClient) dxlinkSetupSequence(ctx context.Context, conn *websocket.Conn) error {
	// 1. SETUP
	setupMsg := map[string]interface{}{
		"type":                   "SETUP",
		"channel":                0,
		"version":                "0.1-DXF-JS/0.3.0",
		"keepaliveTimeout":       60,
		"acceptKeepaliveTimeout": 60,
	}

	if err := conn.WriteJSON(setupMsg); err != nil {
		return fmt.Errorf("failed to send SETUP message: %w", err)
	}

	// Wait for SETUP response
	var setupResponse map[string]interface{}
	if err := conn.ReadJSON(&setupResponse); err != nil {
		return fmt.Errorf("failed to read SETUP response: %w", err)
	}

	if setupResponse["type"] != "SETUP" {
		return fmt.Errorf("unexpected SETUP response: %v", setupResponse)
	}

	// 2. Wait for AUTH_STATE: UNAUTHORIZED
	var authState map[string]interface{}
	if err := conn.ReadJSON(&authState); err != nil {
		return fmt.Errorf("failed to read AUTH_STATE: %w", err)
	}

	if authState["type"] != "AUTH_STATE" || authState["state"] != "UNAUTHORIZED" {
		return fmt.Errorf("unexpected AUTH_STATE response: %v", authState)
	}

	// 3. AUTHORIZE
	authMsg := map[string]interface{}{
		"type":    "AUTH",
		"channel": 0,
		"token":   c.quoteToken,
	}

	if err := conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send AUTH message: %w", err)
	}

	// Wait for AUTH_STATE: AUTHORIZED
	var authSuccess map[string]interface{}
	if err := conn.ReadJSON(&authSuccess); err != nil {
		return fmt.Errorf("failed to read AUTH success: %w", err)
	}

	if authSuccess["type"] != "AUTH_STATE" || authSuccess["state"] != "AUTHORIZED" {
		return fmt.Errorf("authorization failed: %v", authSuccess)
	}

	// 4. CHANNEL_REQUEST
	channelMsg := map[string]interface{}{
		"type":    "CHANNEL_REQUEST",
		"channel": c.channelID,
		"service": "FEED",
		"parameters": map[string]interface{}{
			"contract": "AUTO",
		},
	}

	if err := conn.WriteJSON(channelMsg); err != nil {
		return fmt.Errorf("failed to send CHANNEL_REQUEST: %w", err)
	}

	// Wait for CHANNEL_OPENED
	var channelResponse map[string]interface{}
	if err := conn.ReadJSON(&channelResponse); err != nil {
		return fmt.Errorf("failed to read CHANNEL_OPENED: %w", err)
	}

	if channelResponse["type"] != "CHANNEL_OPENED" {
		return fmt.Errorf("channel open failed: %v", channelResponse)
	}

	// 5. FEED_SETUP for Candle events
	feedSetupMsg := map[string]interface{}{
		"type":                    "FEED_SETUP",
		"channel":                 c.channelID,
		"acceptAggregationPeriod": 0.1,
		"acceptDataFormat":        "COMPACT",
		"acceptEventFields": map[string]interface{}{
			"Candle": []string{"eventType", "eventSymbol", "time", "open", "high", "low", "close", "volume"},
		},
	}

	if err := conn.WriteJSON(feedSetupMsg); err != nil {
		return fmt.Errorf("failed to send FEED_SETUP: %w", err)
	}

	// Wait for FEED_CONFIG
	var feedResponse map[string]interface{}
	if err := conn.ReadJSON(&feedResponse); err != nil {
		return fmt.Errorf("failed to read FEED_CONFIG: %w", err)
	}

	if feedResponse["type"] != "FEED_CONFIG" {
		return fmt.Errorf("feed setup failed: %v", feedResponse)
	}

	slog.Info("DXLink setup sequence completed successfully")
	return nil
}

// subscribeAndCollectCandles subscribes to candle events and collects historical data.
// Exact conversion of Python _subscribe_and_collect_candles method.
func (c *DXLinkCandleClient) subscribeAndCollectCandles(ctx context.Context, conn *websocket.Conn, symbol, timeframe string, fromTime int64, limit int) ([]map[string]interface{}, error) {
	// Create candle symbol format: SYMBOL{=PERIODtype}
	candleSymbol := fmt.Sprintf("%s{=%s}", symbol, timeframe)

	// FEED_SUBSCRIPTION with fromTime for historical data and trading session parameters
	subscriptionMsg := map[string]interface{}{
		"type":    "FEED_SUBSCRIPTION",
		"channel": c.channelID,
		"reset":   true,
		"add": []map[string]interface{}{
			{
				"type":     "Candle",
				"symbol":   candleSymbol,
				"fromTime": fromTime,
				"tho":      "true", // Trading hours only - exclude extended session
				"a":        "s",    // Align candles to trading session (not midnight)
			},
		},
	}

	if err := conn.WriteJSON(subscriptionMsg); err != nil {
		return nil, fmt.Errorf("failed to send subscription: %w", err)
	}

	slog.Debug(fmt.Sprintf("DXLink: Subscribed to %s from timestamp %d", candleSymbol, fromTime))

	// Collect candles with optimized timeout strategy and deduplication
	candlesDict := make(map[int64]map[string]interface{}) // Use timestamp as key for deduplication
	consecutiveEmptyMessages := 0
	startTime := time.Now()
	initialBurstComplete := false
	noNewDataIterations := 0
	maxNoDataIterations := 2 // Very aggressive - stop after 2 iterations without new data

	for len(candlesDict) < limit && noNewDataIterations < maxNoDataIterations {
		// Short timeout - we want to detect completion quickly
		timeout := 1 * time.Second
		conn.SetReadDeadline(time.Now().Add(timeout))

		var message map[string]interface{}
		err := conn.ReadJSON(&message)

		if err != nil {
			// Check if it's a timeout
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				noNewDataIterations++
				consecutiveEmptyMessages++

				// Aggressive completion detection
				if len(candlesDict) > 0 && noNewDataIterations >= 1 {
					slog.Debug(fmt.Sprintf("DXLink: No new data for %d iterations with %d unique candles, assuming complete", noNewDataIterations, len(candlesDict)))
					break
				}
				continue
			}
			return nil, fmt.Errorf("failed to read message: %w", err)
		}

		if message["type"] == "FEED_DATA" {
			feedData, ok := message["data"].([]interface{})
			if !ok {
				slog.Info("DXLink: FEED_DATA message has no data field or wrong type")
				continue
			}

			// Process candle events
			newCandles := c.processFeedData(feedData)
			slog.Info(fmt.Sprintf("DXLink: processFeedData returned %d candles", len(newCandles)))
			if len(newCandles) > 0 {
				// Deduplicate by timestamp - keep the most recent version of each candle
				candlesBefore := len(candlesDict)
				for _, candle := range newCandles {
					if timestamp, ok := candle["time"].(int64); ok {
						// Always keep the latest version (DXLink sends updates for current candle)
						candlesDict[timestamp] = candle
					}
				}

				// Check if we actually got new unique candles
				candlesAfter := len(candlesDict)
				if candlesAfter > candlesBefore {
					// We got new unique data, reset counters
					noNewDataIterations = 0
					consecutiveEmptyMessages = 0

					// Detect initial burst completion (when we get < 10 candles at once)
					if !initialBurstComplete && len(newCandles) < 10 {
						initialBurstComplete = true
						slog.Debug(fmt.Sprintf("DXLink: Initial burst complete, got %d unique candles", candlesAfter))
					}

					// Log progress
					elapsed := time.Since(startTime)
					if candlesAfter%50 == 0 || elapsed > 2*time.Second {
						slog.Debug(fmt.Sprintf("DXLink: Received %d candles, total unique: %d (elapsed: %.1fs)", len(newCandles), candlesAfter, elapsed.Seconds()))
						startTime = time.Now() // Reset timer
					}
				} else {
					// Got candles but they were duplicates - increment counter
					noNewDataIterations++
					slog.Debug(fmt.Sprintf("DXLink: Got %d duplicate candles, no new unique data (iteration %d)", len(newCandles), noNewDataIterations))
				}
			} else {
				// No candles in this message
				noNewDataIterations++
				consecutiveEmptyMessages++
			}
		} else {
			// Handle other message types (keepalive, etc.)
			slog.Debug(fmt.Sprintf("DXLink: Received %s message", message["type"]))
			noNewDataIterations++
			consecutiveEmptyMessages++
		}
	}

	// Convert dict back to list and sort by timestamp
	var candles []map[string]interface{}
	for _, candle := range candlesDict {
		candles = append(candles, candle)
	}

	// Sort by timestamp
	for i := 0; i < len(candles)-1; i++ {
		for j := 0; j < len(candles)-i-1; j++ {
			time1, ok1 := candles[j]["time"].(int64)
			time2, ok2 := candles[j+1]["time"].(int64)
			if ok1 && ok2 && time1 > time2 {
				candles[j], candles[j+1] = candles[j+1], candles[j]
			}
		}
	}

	// Apply limit
	if limit > 0 && len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}

	totalElapsed := time.Since(startTime)
	slog.Info(fmt.Sprintf("DXLink: Final candle count: %d (total time: %.1fs)", len(candles), totalElapsed.Seconds()))
	return candles, nil
}

// processFeedData processes FEED_DATA messages and extracts candle events.
// Exact conversion of Python _process_feed_data method.
func (c *DXLinkCandleClient) processFeedData(feedData []interface{}) []map[string]interface{} {
	var candles []map[string]interface{}

	// DXLink COMPACT format: flat array with candle data
	// Format: ['Candle', 'SYMBOL{=timeframe}', timestamp, open, high, low, close, volume, 'Candle', ...]
	for _, item := range feedData {
		if itemArray, ok := item.([]interface{}); ok {
			slog.Debug(fmt.Sprintf("DXLink: Processing array with %d elements", len(itemArray)))

			// Parse the flat array - each candle is 8 consecutive elements
			i := 0
			for i+7 < len(itemArray) {
				if itemArray[i] == "Candle" {
					// Convert timestamp to int64 if it's float64
					var timestamp int64
					if ts, ok := itemArray[i+2].(float64); ok {
						timestamp = int64(ts)
					}

					candle := map[string]interface{}{
						"eventType":   itemArray[i],   // "Candle"
						"eventSymbol": itemArray[i+1], // "SPY{=d}"
						"time":        timestamp,      // timestamp in ms
						"open":        itemArray[i+3], // open price
						"high":        itemArray[i+4], // high price
						"low":         itemArray[i+5], // low price
						"close":       itemArray[i+6], // close price
						"volume":      itemArray[i+7], // volume
					}
					candles = append(candles, candle)
					slog.Debug(fmt.Sprintf("DXLink: Parsed candle: %s at %d, OHLC: %.2f/%.2f/%.2f/%.2f",
						itemArray[i+1], timestamp,
						itemArray[i+3], itemArray[i+4], itemArray[i+5], itemArray[i+6]))
					i += 8 // Move to next candle (8 fields per candle)
				} else {
					i++ // Skip non-candle data
				}
			}
		}
	}

	slog.Info(fmt.Sprintf("DXLink: Processed %d candles from feed data", len(candles)))
	return candles
}

// ======================== Account Streamer for Order Events ========================

// StartAccountStream starts the TastyTrade account events stream via Account Streamer WebSocket
// This is separate from DXLink which is for market data only
func (p *TastyTradeProvider) StartAccountStream(ctx context.Context) error {
	slog.Info("TastyTrade: Starting account event stream via Account Streamer...")

	p.accountStreamLock.Lock()
	defer p.accountStreamLock.Unlock()

	// Check if already connected
	if p.accountStreamConn != nil {
		slog.Info("TastyTrade: Account stream already connected")
		return nil
	}

	// Reset stop channel if it was closed
	select {
	case <-p.accountStreamStopChan:
		p.accountStreamStopChan = make(chan struct{})
	default:
	}

	// Ensure we have a valid session for the auth token
	if err := p.ensureValidSession(ctx); err != nil {
		return fmt.Errorf("failed to get session for account stream: %w", err)
	}

	// Extract Bearer token from session token - KEEP the "Bearer " prefix for Account Streamer!
	// The documentation says: "When using Access Tokens make sure you have 'Bearer ' preceding your token value"
	authToken := p.sessionToken // Keep full "Bearer {token}" format

	// Connect to Account Streamer WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.accountStreamURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Account Streamer: %w", err)
	}

	p.accountStreamConn = conn

	// Send connect message to authenticate and subscribe to account updates
	connectMsg := map[string]interface{}{
		"action":     "connect",
		"value":      []string{p.accountID},
		"auth-token": authToken,
		"request-id": 1,
	}

	slog.Info("TastyTrade: Sending Account Streamer connect message",
		"account", p.accountID,
		"url", p.accountStreamURL)

	if err := conn.WriteJSON(connectMsg); err != nil {
		conn.Close()
		p.accountStreamConn = nil
		return fmt.Errorf("failed to send connect message: %w", err)
	}

	// Wait for response
	var response map[string]interface{}
	if err := conn.ReadJSON(&response); err != nil {
		conn.Close()
		p.accountStreamConn = nil
		return fmt.Errorf("failed to read connect response: %w", err)
	}

	// Log response without sensitive data
	status, _ := response["status"].(string)
	sessionID, _ := response["web-socket-session-id"].(string)
	slog.Info("TastyTrade: Account Streamer response", "status", status, "session_id", sessionID)

	// Check if connection was successful
	if status != "ok" {
		errMsg, _ := response["message"].(string)
		conn.Close()
		p.accountStreamConn = nil
		slog.Error("TastyTrade: Account Streamer connection failed", "status", status, "message", errMsg)
		return fmt.Errorf("Account Streamer connection failed: %s - %s", status, errMsg)
	}

	// Start the account stream processor with heartbeat
	go p.processAccountStream(ctx, authToken)

	slog.Info("TastyTrade account event stream started via Account Streamer")
	return nil
}

// processAccountStream handles incoming order events from the Account Streamer with auto-reconnection
func (p *TastyTradeProvider) processAccountStream(ctx context.Context, authToken string) {
	// Create done channel for goroutine completion
	done := make(chan struct{})
	p.accountStreamDoneChan = done

	reconnectAttempts := 0
	maxReconnectAttempts := 10

	defer func() {
		// Recover from any panics (e.g., "repeated read on failed websocket connection")
		if r := recover(); r != nil {
			slog.Error("TastyTrade: Account Streamer recovered from panic", "panic", r)
		}
		p.accountStreamLock.Lock()
		if p.accountStreamConn != nil {
			p.accountStreamConn.Close()
			p.accountStreamConn = nil
		}
		close(done)
		p.accountStreamLock.Unlock()
		slog.Info("TastyTrade: Account Streamer goroutine exited")
	}()

	slog.Info("TastyTrade: Account Streamer processor started")

	// Heartbeat ticker - send heartbeat every 30 seconds (within 2s-1m range)
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("TastyTrade: Account Streamer context cancelled")
			return
		case <-p.accountStreamStopChan:
			slog.Info("TastyTrade: Account Streamer stop requested")
			return
		case <-heartbeatTicker.C:
			// Send heartbeat
			p.accountStreamLock.Lock()
			if p.accountStreamConn != nil {
				heartbeatMsg := map[string]interface{}{
					"action":     "heartbeat",
					"auth-token": authToken,
					"request-id": time.Now().UnixNano(),
				}
				if err := p.accountStreamConn.WriteJSON(heartbeatMsg); err != nil {
					slog.Error("TastyTrade: Failed to send heartbeat, will attempt reconnect", "error", err)
					p.accountStreamConn.Close()
					p.accountStreamConn = nil
					p.accountStreamLock.Unlock()

					// Attempt reconnection
					if p.attemptAccountStreamReconnect(ctx, authToken, &reconnectAttempts, maxReconnectAttempts) {
						continue
					}
					return
				}
				slog.Debug("TastyTrade: Sent heartbeat")
			}
			p.accountStreamLock.Unlock()
		default:
			p.accountStreamLock.Lock()
			conn := p.accountStreamConn
			p.accountStreamLock.Unlock()

			if conn == nil {
				// Connection is nil, attempt reconnect
				if p.attemptAccountStreamReconnect(ctx, authToken, &reconnectAttempts, maxReconnectAttempts) {
					continue
				}
				slog.Info("TastyTrade: Account Streamer giving up after max reconnect attempts")
				return
			}

			// Reset reconnect attempts on successful connection usage
			reconnectAttempts = 0

			// Set read deadline
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Read message with panic recovery for failed websocket connections
			var message map[string]interface{}
			var readErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("TastyTrade: Recovered from ReadJSON panic", "panic", r)
						readErr = fmt.Errorf("websocket read panic: %v", r)
					}
				}()
				readErr = conn.ReadJSON(&message)
			}()

			if readErr != nil {
				if netErr, ok := readErr.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					continue
				}
				slog.Error("TastyTrade: Account Streamer read error, will attempt reconnect", "error", readErr)

				p.accountStreamLock.Lock()
				if p.accountStreamConn != nil {
					p.accountStreamConn.Close()
					p.accountStreamConn = nil
				}
				p.accountStreamLock.Unlock()

				// Attempt reconnection
				if p.attemptAccountStreamReconnect(ctx, authToken, &reconnectAttempts, maxReconnectAttempts) {
					continue
				}
				return
			}

			// Process the message
			p.handleAccountStreamMessage(message)
		}
	}
}

// attemptAccountStreamReconnect attempts to reconnect the Account Streamer with exponential backoff
func (p *TastyTradeProvider) attemptAccountStreamReconnect(ctx context.Context, authToken string, attempts *int, maxAttempts int) bool {
	if *attempts >= maxAttempts {
		slog.Error("TastyTrade: Max Account Streamer reconnect attempts reached", "attempts", *attempts)
		return false
	}

	*attempts++

	// Calculate backoff delay: 1s, 2s, 4s, 8s, 16s, 30s (capped)
	backoffSeconds := 1 << uint(*attempts-1)
	if backoffSeconds > 30 {
		backoffSeconds = 30
	}
	delay := time.Duration(backoffSeconds) * time.Second

	slog.Info("TastyTrade: Attempting Account Streamer reconnection", "attempt", *attempts, "maxAttempts", maxAttempts, "delay", delay)

	select {
	case <-ctx.Done():
		return false
	case <-p.accountStreamStopChan:
		return false
	case <-time.After(delay):
	}

	// Close existing connection if any
	p.accountStreamLock.Lock()
	if p.accountStreamConn != nil {
		p.accountStreamConn.Close()
		p.accountStreamConn = nil
	}
	p.accountStreamLock.Unlock()

	// Connect to Account Streamer WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.accountStreamURL, nil)
	if err != nil {
		slog.Error("TastyTrade: Failed to reconnect to Account Streamer", "error", err, "attempt", *attempts)
		return p.attemptAccountStreamReconnect(ctx, authToken, attempts, maxAttempts)
	}

	// Send connect message
	connectMsg := map[string]interface{}{
		"action":     "connect",
		"value":      []string{p.accountID},
		"auth-token": authToken,
		"request-id": time.Now().UnixNano(),
	}

	if err := conn.WriteJSON(connectMsg); err != nil {
		slog.Error("TastyTrade: Failed to send connect message during reconnect", "error", err)
		conn.Close()
		return p.attemptAccountStreamReconnect(ctx, authToken, attempts, maxAttempts)
	}

	// Wait for response with panic recovery
	var response map[string]interface{}
	var readErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("TastyTrade: Recovered from ReadJSON panic during reconnect", "panic", r)
				readErr = fmt.Errorf("websocket read panic: %v", r)
			}
		}()
		readErr = conn.ReadJSON(&response)
	}()

	if readErr != nil {
		slog.Error("TastyTrade: Failed to read connect response during reconnect", "error", readErr)
		conn.Close()
		return p.attemptAccountStreamReconnect(ctx, authToken, attempts, maxAttempts)
	}

	// Check if connection was successful
	status, _ := response["status"].(string)
	if status != "ok" {
		errMsg, _ := response["message"].(string)
		conn.Close()
		slog.Error("TastyTrade: Account Streamer reconnection failed", "status", status, "message", errMsg)
		return p.attemptAccountStreamReconnect(ctx, authToken, attempts, maxAttempts)
	}

	// Update connection
	p.accountStreamLock.Lock()
	p.accountStreamConn = conn
	p.accountStreamLock.Unlock()

	slog.Info("TastyTrade: Successfully reconnected to Account Streamer", "attempt", *attempts)
	return true
}

// handleAccountStreamMessage processes incoming Account Streamer messages
func (p *TastyTradeProvider) handleAccountStreamMessage(message map[string]interface{}) {
	msgType, _ := message["type"].(string)
	slog.Debug("TastyTrade: Account Streamer message received", "type", msgType)

	switch msgType {
	case "Order":
		// Handle order events
		orderEvent := p.parseAccountStreamOrderEvent(message)
		if orderEvent != nil {
			slog.Info("TastyTrade: Parsed order event from Account Streamer", "id", orderEvent.GetIDAsString(), "status", orderEvent.Status, "symbol", orderEvent.Symbol)

			// Normalize the event based on status transitions
			normalizedEvent, shouldEmit := models.GetGlobalNormalizer().NormalizeEvent(orderEvent)

			if shouldEmit && normalizedEvent != "" {
				orderEvent.NormalizedEvent = normalizedEvent

				if p.streamingState != nil && p.streamingState.orderEventCallback != nil {
					slog.Info("TastyTrade: Emitting normalized event",
						"orderID", orderEvent.ID,
						"normalizedEvent", normalizedEvent,
						"status", orderEvent.Status)
					p.streamingState.orderEventCallback(orderEvent)
				}
			}
		}

	case "error", "Error":
		// Handle errors
		errMsg, _ := message["error"].(string)
		slog.Error("TastyTrade: Account Streamer error", "message", errMsg, "full", message)

	case "status":
		// Handle status messages (like connect OK)
		status, _ := message["status"].(string)
		slog.Debug("TastyTrade: Account Streamer status", "status", status)

	default:
		slog.Debug("TastyTrade: Unhandled Account Streamer message type", "type", msgType, "message", message)
	}
}

// parseAccountStreamOrderEvent parses an order event from the Account Streamer
func (p *TastyTradeProvider) parseAccountStreamOrderEvent(message map[string]interface{}) *models.OrderEvent {
	// Extract data field
	data, ok := message["data"].(map[string]interface{})
	if !ok {
		slog.Warn("TastyTrade: Order event missing data field", "message", message)
		return nil
	}

	// Map TastyTrade status to our standard status (normalized to match Tradier format)
	// Standard statuses: pending, open, filled, canceled, rejected, partially_filled, expired
	// TastyTrade sends various statuses that we normalize to our standard format
	statusMap := map[string]string{
		// Initial order states → pending (matches Tradier's "pending")
		"Received":     "pending",
		"received":     "pending",
		"routed":       "pending",
		"Routed":       "pending", // Capitalized variant
		" Routed":      "pending", // With leading space
		"queued":       "pending",
		"Live":         "pending",
		"live":         "pending",
		"working":      "pending",
		"confirm":      "pending",
		"confirmation": "pending",
		"submitted":    "pending",
		"Submit":       "pending",
		"placed":       "pending",
		"New":          "pending",
		"new":          "pending",
		"INIT":         "pending",
		"INITIAL":      "pending",
		"APPROVED":     "pending",
		"APPROVAL":     "pending",

		// Open states → open (order is live in market, no toast needed)
		"Open":   "open",
		"open":   "open",
		"ROUTED": "open", // Capitalized variant (only for when order is fully open)

		// Filled states → filled
		"Filled":   "filled",
		"filled":   "filled",
		"FILL":     "filled",
		"complete": "filled",
		"COMPLETE": "filled",

		// Partially filled → partially_filled
		"Partially Filled": "partially_filled",
		"partially_filled": "partially_filled",
		"PARTIAL":          "partially_filled",
		"partial":          "partially_filled",

		// Canceled states → canceled
		"Canceled":         "canceled",
		"canceled":         "canceled",
		"Cancelled":        "canceled", // UK spelling
		"CANCEL":           "canceled",
		"cancel":           "canceled",
		"Cancel Received":  "canceled",
		"Cancel Requested": "canceled",
		"cancel_received":  "canceled",
		"cancel_requested": "canceled",
		"CANCEL_RECEIVED":  "canceled",
		"CANCEL_REQUESTED": "canceled",

		// Rejected states → rejected
		"Rejected": "rejected",
		"rejected": "rejected",
		"REJECT":   "rejected",
		"REJECTED": "rejected",

		// Expired states → expired
		"Expired": "expired",
		"expired": "expired",
		"EXPIRED": "expired",
		"expire":  "expired",
	}

	// Get order ID
	var id interface{}
	if idVal, ok := data["id"].(string); ok {
		id = idVal
	} else if idVal, ok := data["id"].(float64); ok {
		id = fmt.Sprintf("%.0f", idVal)
	} else if idVal, ok := data["id"].(int); ok {
		id = fmt.Sprintf("%d", idVal)
	}

	// Get status
	rawStatus, _ := data["status"].(string)
	status := statusMap[rawStatus]
	if status == "" {
		slog.Warn("TastyTrade: Unknown order status", "rawStatus", rawStatus, "orderID", id)
		status = "unknown"
	} else {
		slog.Debug("TastyTrade: Mapped status", "rawStatus", rawStatus, "normalizedStatus", status, "orderID", id)
	}

	// Get symbol - TastyTrade uses "underlying-symbol" for orders
	symbol, _ := data["underlying-symbol"].(string)

	// Get action/side - extract from legs
	legs, _ := data["legs"].([]interface{})
	var side string
	if len(legs) > 0 {
		if firstLeg, ok := legs[0].(map[string]interface{}); ok {
			action, _ := firstLeg["action"].(string)
			side = p.mapTastyTradeActionToSide(action)
			// If symbol not set, try to get from leg
			if symbol == "" {
				symbol, _ = firstLeg["symbol"].(string)
			}
		}
	}

	// Convert symbol to standard OCC format if it's an option
	symbol = p.convertSymbolToStandardFormat(symbol)

	// Get quantities - TastyTrade uses "size" for order size
	quantity := getFloat(data, "size")

	// Get executed quantity from legs fills
	var executedQty float64
	for _, leg := range legs {
		if legMap, ok := leg.(map[string]interface{}); ok {
			fills, _ := legMap["fills"].([]interface{})
			for _, fill := range fills {
				if fillMap, ok := fill.(map[string]interface{}); ok {
					executedQty += getFloat(fillMap, "quantity")
				}
			}
		}
	}

	// Get price - check fills for avg fill price
	var avgFillPrice float64
	if executedQty > 0 {
		for _, leg := range legs {
			if legMap, ok := leg.(map[string]interface{}); ok {
				fills, _ := legMap["fills"].([]interface{})
				for _, fill := range fills {
					if fillMap, ok := fill.(map[string]interface{}); ok {
						if fillPrice := getFloat(fillMap, "fill-price"); fillPrice > 0 {
							avgFillPrice = fillPrice
						}
					}
				}
			}
		}
	}

	// Get order type
	orderType := getString(data, "order-type")
	orderType = p.mapTastyTradeOrderTypeToStandard(orderType)

	// Get timestamps
	submittedAt := getString(data, "received-at")
	if submittedAt == "" {
		submittedAt = time.Now().Format(time.RFC3339)
	}

	return &models.OrderEvent{
		ID:               id,
		Event:            "order_update",
		Status:           status,
		Type:             orderType,
		Symbol:           symbol,
		Side:             side,
		Quantity:         quantity,
		ExecutedQuantity: executedQty,
		AvgFillPrice:     avgFillPrice,
		TransactionDate:  submittedAt,
		Account:          p.accountID,
	}
}

// mapTastyTradeOrderTypeToStandard maps TastyTrade order type to our standard format
func (p *TastyTradeProvider) mapTastyTradeOrderTypeToStandard(tastyOrderType string) string {
	typeMap := map[string]string{
		"Market":     "market",
		"Limit":      "limit",
		"Stop":       "stop",
		"Stop Limit": "stop_limit",
	}
	return typeMap[tastyOrderType]
}

// StopAccountStream stops the TastyTrade account events stream
func (p *TastyTradeProvider) StopAccountStream() {
	slog.Info("TastyTrade: Stopping account event stream...")

	p.accountStreamLock.Lock()
	defer p.accountStreamLock.Unlock()

	// Signal stop
	select {
	case <-p.accountStreamStopChan:
		// Already closed
	default:
		close(p.accountStreamStopChan)
	}

	// Close connection
	if p.accountStreamConn != nil {
		p.accountStreamConn.Close()
		p.accountStreamConn = nil
	}

	// Wait for goroutine to finish (with timeout)
	select {
	case <-p.accountStreamDoneChan:
		slog.Info("TastyTrade: Account Streamer goroutine finished")
	case <-time.After(5 * time.Second):
		slog.Warn("TastyTrade: Account Streamer goroutine did not finish in time")
	}

	slog.Info("TastyTrade account event stream stopped")
}

// SetOrderEventCallback sets the callback for receiving order events
func (p *TastyTradeProvider) SetOrderEventCallback(cb func(*models.OrderEvent)) {
	slog.Info("TastyTrade: SetOrderEventCallback called")
	if p.streamingState == nil {
		slog.Info("TastyTrade: streamingState is nil, initializing...")
		p.initStreamingState()
	}
	p.streamingState.orderEventCallback = cb
	slog.Info("TastyTrade: Order event callback set successfully")
}

// IsAccountStreamConnected checks if account stream is connected
func (p *TastyTradeProvider) IsAccountStreamConnected() bool {
	p.accountStreamLock.Lock()
	defer p.accountStreamLock.Unlock()
	return p.accountStreamConn != nil
}
