package tastytrade

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/utils"

	"github.com/gorilla/websocket"
)

// TastyTradeProvider implements the Provider interface for TastyTrade.
// Exact conversion of Python TastyTradeProvider class.
type TastyTradeProvider struct {
	*base.BaseProviderImpl
	accountID       string
	baseURL         string
	clientID        string
	clientSecret    string
	refreshToken    string
	authCode        string
	redirectURI     string
	sessionToken    string
	sessionExpires  *time.Time
	quoteToken      string
	quoteExpires    *time.Time
	dxlinkURL       string
	httpClient      *utils.HTTPClient
	streamingState  *StreamingState
}

// NewTastyTradeProvider creates a new TastyTrade provider instance.
// Exact conversion of Python TastyTradeProvider.__init__ method.
func NewTastyTradeProvider(accountID, baseURL, clientID, clientSecret, refreshToken, authCode, redirectURI string) *TastyTradeProvider {
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
		payload = map[string]interface{}{
			"grant_type":    "refresh_token",
			"refresh_token": p.refreshToken,
			"client_secret": p.clientSecret,
		}
		if p.clientID != "" {
			payload["client_id"] = p.clientID
		}
	} else if p.clientSecret != "" && p.authCode != "" {
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
		return fmt.Errorf("OAuth2 credentials incomplete: provide refresh_token or authorization_code with client_secret")
	}

	response, err := p.httpClient.Post(ctx, url, payload, headers)
	if err != nil {
		return fmt.Errorf("OAuth2 token request failed: %w", err)
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.Unmarshal(response.Body, &tokenResponse); err != nil {
		return fmt.Errorf("failed to parse OAuth2 response: %w", err)
	}

	if tokenResponse.AccessToken == "" {
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
		p.refreshToken = tokenResponse.RefreshToken
	}

	slog.Debug("TastyTrade OAuth2 access token obtained successfully")
	return nil
}

// ensureValidSession ensures we have a valid session token.
// Exact conversion of Python _ensure_valid_session method.
func (p *TastyTradeProvider) ensureValidSession(ctx context.Context) error {
	if p.sessionToken == "" || p.sessionExpires == nil {
		return p.createSession(ctx)
	}

	// Check if session is about to expire (refresh 5 minutes early)
	if time.Now().After(p.sessionExpires.Add(-5 * time.Minute)) {
		slog.Debug("Session token expiring soon, refreshing...")
		return p.createSession(ctx)
	}

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

	if err != nil {
		return nil, err
	}

	return response.Body, nil
}

// === Market Data Methods ===

// GetStockQuote gets the latest stock quote for a symbol.
// Exact conversion of Python get_stock_quote method.
func (p *TastyTradeProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	// TastyTrade doesn't have a direct stock quote endpoint
	// This would typically use streaming, but for now return a basic implementation
	return &models.StockQuote{
		Symbol:    symbol,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// GetStockQuotes gets stock quotes for multiple symbols.
// Exact conversion of Python get_stock_quotes method.
func (p *TastyTradeProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	quotes := make(map[string]*models.StockQuote)
	for _, symbol := range symbols {
		quote, err := p.GetStockQuote(ctx, symbol)
		if err != nil {
			continue
		}
		quotes[symbol] = quote
	}
	return quotes, nil
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
		"regular":        "monthly",
		"end-of-month":   "eom",
		"weekly":         "weekly",
		"quarterly":      "quarterly",
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
				// Determine type based on expiration type from TastyTrade
				symbolType := typeMapping[strings.ToLower(expType)]
				if symbolType == "" {
					symbolType = strings.ToLower(expType)
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
	endpoint := fmt.Sprintf("/option-chains/%s", symbol)
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
		// Try to get quote for the underlying
		quote, err := p.GetStockQuote(ctx, symbol)
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

// GetOptionsGreeksBatch gets Greeks for multiple option symbols.
// Exact conversion of Python get_options_greeks_batch method.
func (p *TastyTradeProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	// TastyTrade doesn't have a dedicated Greeks endpoint
	// This would typically use streaming, but for now return empty
	return make(map[string]map[string]interface{}), nil
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
	endpoint := fmt.Sprintf("/accounts/%s/positions", p.accountID)
	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Items []struct {
				Symbol           string  `json:"symbol"`
				InstrumentType   string  `json:"instrument-type"`
				Quantity         float64 `json:"quantity"`
				AverageOpenPrice float64 `json:"average-open-price"`
				ClosePrice       float64 `json:"close-price"`
				Multiplier       float64 `json:"multiplier"`
				CostEffect       string  `json:"cost-effect"`
				CreatedAt        string  `json:"created-at"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse positions response: %w", err)
	}

	var positions []*models.Position
	for _, item := range apiResponse.Data.Items {
		// Determine asset class
		assetClass := "us_equity"
		if item.InstrumentType == "Equity Option" {
			assetClass = "us_option"
		}

		// Determine side
		side := "long"
		if item.Quantity < 0 {
			side = "short"
		}

		// Calculate values
		multiplier := item.Multiplier
		if multiplier == 0 {
			multiplier = 1
		}

		costBasis := item.AverageOpenPrice * item.Quantity * multiplier
		marketValue := item.ClosePrice * item.Quantity * multiplier
		unrealizedPL := marketValue - costBasis

		// Handle cost effect
		if item.CostEffect == "Credit" {
			costBasis = -costBasis
		}

		position := &models.Position{
			Symbol:        item.Symbol,
			Qty:           item.Quantity,
			Side:          side,
			MarketValue:   marketValue,
			CostBasis:     costBasis,
			UnrealizedPL:  unrealizedPL,
			CurrentPrice:  item.ClosePrice,
			AvgEntryPrice: item.AverageOpenPrice,
			AssetClass:    assetClass,
			DateAcquired:  &item.CreatedAt,
		}
		positions = append(positions, position)
	}

	return positions, nil
}

// GetPositionsEnhanced gets enhanced positions grouped by date_acquired (same order timing).
// Exact conversion of Python get_positions_enhanced method.
func (p *TastyTradeProvider) GetPositionsEnhanced(ctx context.Context) (map[string]interface{}, error) {
	slog.Debug("TastyTrade: Getting enhanced positions grouped by acquisition date...")

	// 1. Get current positions only (no additional API calls needed)
	currentPositions, err := p.GetPositions(ctx)
	if err != nil {
		return map[string]interface{}{
			"enhanced":      true,
			"symbol_groups": []interface{}{},
		}, nil
	}

	if len(currentPositions) == 0 {
		slog.Info("TastyTrade: No current positions found")
		return map[string]interface{}{
			"enhanced":      true,
			"symbol_groups": []interface{}{},
		}, nil
	}

	// 2. Group by date_acquired instead of expensive order chain analysis
	symbolGroups, err := p.createDateBasedHierarchicalGroups(currentPositions)
	if err != nil {
		return map[string]interface{}{
			"enhanced":      true,
			"symbol_groups": []interface{}{},
		}, nil
	}

	slog.Debug(fmt.Sprintf("TastyTrade: Created %d symbol groups using date_acquired grouping", len(symbolGroups)))
	return map[string]interface{}{
		"enhanced":      true,
		"symbol_groups": symbolGroups,
	}, nil
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
			"symbol":         pos.Symbol,
			"qty":            pos.Qty,
			"avg_entry_price": pos.AvgEntryPrice,
			"cost_basis":     pos.CostBasis,
			"asset_class":    pos.AssetClass,
			"lastday_price":  nil, // Not available in basic position data
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
		endpoint = fmt.Sprintf("/accounts/%s/orders/live", p.accountID)
	} else {
		endpoint = fmt.Sprintf("/accounts/%s/orders", p.accountID)
	}

	response, err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Items []struct {
				ID          string `json:"id"`
				Status      string `json:"status"`
				OrderType   string `json:"order-type"`
				TimeInForce string `json:"time-in-force"`
				Price       string `json:"price"`
				PriceEffect string `json:"price-effect"`
				ReceivedAt  string `json:"received-at"`
				UpdatedAt   string `json:"updated-at"`
				Legs        []struct {
					Symbol         string  `json:"symbol"`
					InstrumentType string  `json:"instrument-type"`
					Action         string  `json:"action"`
					Quantity       float64 `json:"quantity"`
				} `json:"legs"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}

	var orders []*models.Order
	for _, item := range apiResponse.Data.Items {
		// Map status
		orderStatus := strings.ToLower(item.Status)
		if item.Status == "Live" || item.Status == "Routed" {
			orderStatus = "open"
		} else if item.Status == "Filled" {
			orderStatus = "filled"
		} else if item.Status == "Cancelled" {
			orderStatus = "cancelled"
		}

		// Get first leg for single-leg orders
		var symbol string
		var quantity float64
		var side string
		var assetClass string

		if len(item.Legs) > 0 {
			leg := item.Legs[0]
			symbol = leg.Symbol
			quantity = leg.Quantity
			
			// Map action to side
			if strings.Contains(strings.ToLower(leg.Action), "buy") {
				side = "buy"
			} else {
				side = "sell"
			}

			// Map instrument type
			if leg.InstrumentType == "Equity Option" {
				assetClass = "us_option"
			} else {
				assetClass = "us_equity"
			}
		}

		// Parse price
		var limitPrice *float64
		if item.Price != "" {
			if price, err := strconv.ParseFloat(item.Price, 64); err == nil {
				// Handle credit/debit
				if item.PriceEffect == "Credit" {
					price = -price
				}
				limitPrice = &price
			}
		}

		order := &models.Order{
			ID:          item.ID,
			Symbol:      symbol,
			AssetClass:  assetClass,
			Side:        side,
			OrderType:   strings.ToLower(item.OrderType),
			Qty:         quantity,
			LimitPrice:  limitPrice,
			Status:      orderStatus,
			TimeInForce: strings.ToLower(item.TimeInForce),
			SubmittedAt: item.ReceivedAt,
		}

		if item.Status == "Filled" {
			order.FilledAt = &item.UpdatedAt
		}

		orders = append(orders, order)
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
			AccountNumber            string      `json:"account-number"`
			CashBalance              interface{} `json:"cash-balance"`
			EquityBuyingPower        interface{} `json:"equity-buying-power"`
			DayTradingBuyingPower    interface{} `json:"day-trading-buying-power"`
			MarginEquity             interface{} `json:"margin-equity"`
			RegTMarginRequirement    interface{} `json:"reg-t-margin-requirement"`
			MaintenanceRequirement   interface{} `json:"maintenance-requirement"`
			NetLiquidatingValue      interface{} `json:"net-liquidating-value"`
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
		AccountID:               p.accountID,
		AccountNumber:           &apiResponse.Data.AccountNumber,
		Status:                  "active",
		BuyingPower:             buyingPower,
		Cash:                    cash,
		DayTradingBuyingPower:   dayTradingBuyingPower,
		Equity:                  equity,
		InitialMargin:           initialMargin,
		MaintenanceMargin:       maintenanceMargin,
		PortfolioValue:          portfolioValue,
	}

	return account, nil
}

// === Order Management Methods ===

// PlaceOrder places a trading order.
// Exact conversion of Python place_order method.
func (p *TastyTradeProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	// Transform order data to TastyTrade format
	tastytrade_order := p.transformToTastyTradeOrder(orderData)

	endpoint := fmt.Sprintf("/accounts/%s/orders", p.accountID)
	response, err := p.makeAuthenticatedRequest(ctx, "POST", endpoint, tastytrade_order)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	var apiResponse struct {
		Data struct {
			Order struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"order"`
		} `json:"data"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	// Return basic order info
	return &models.Order{
		ID:     apiResponse.Data.Order.ID,
		Status: strings.ToLower(apiResponse.Data.Order.Status),
	}, nil
}

// PlaceMultiLegOrder places a multi-leg trading order.
// Exact conversion of Python place_multi_leg_order method.
func (p *TastyTradeProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return p.PlaceOrder(ctx, orderData) // Same implementation for now
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
	streamConnection   *websocket.Conn
	connectionReady    chan struct{}
	isConnected        bool
	subscribedSymbols  map[string]bool
	streamingQueue     chan *models.MarketData
	streamingCache     base.StreamingCache
	shutdownEvent      chan struct{}
	streamingTask      *StreamingTask
	connectionID       string
	recvLock           sync.Mutex
	greeksRequests     map[string]chan map[string]interface{}
	quoteRequests      map[string]chan map[string]interface{}
	requestsLock       sync.Mutex
	messageChan        chan map[string]interface{} // Add: buffered channel for messages
	errorChan          chan error                   // Add: for read errors
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
		messageChan:       make(chan map[string]interface{}, 100), // Buffered to avoid blocking reader
		errorChan:         make(chan error, 1),
	}
}

// ConnectStreaming connects to DXLink streaming service with health monitoring.
// Exact conversion of Python connect_streaming method.
func (p *TastyTradeProvider) ConnectStreaming(ctx context.Context) (bool, error) {
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
		HandshakeTimeout: 15 * time.Second,
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
	close(p.streamingState.shutdownEvent)

	// Cancel streaming task
	if p.streamingState.streamingTask != nil {
		p.streamingState.streamingTask.cancel()
		<-p.streamingState.streamingTask.done
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

// SubscribeToSymbols subscribes to real-time data for symbols via DXLink.
// Exact conversion of Python subscribe_to_symbols method.
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
	var subscriptions []map[string]interface{}

	for _, symbol := range symbols {
		// Convert to streaming symbol format for DXLink
		var streamingSymbol string
		if p.isOptionSymbol(symbol) {
			streamingSymbol = p.convertToStreamerSymbol(symbol)
			slog.Info(fmt.Sprintf("TastyTrade: Converted option symbol %s -> %s for streaming", symbol, streamingSymbol))
		} else {
			streamingSymbol = symbol // Stock symbols use as-is
			slog.Debug(fmt.Sprintf("TastyTrade: Using stock symbol as-is: %s", streamingSymbol))
		}

		// Add Quote subscription only if requested
		if containsString(dataTypes, "Quote") {
			subscriptions = append(subscriptions, map[string]interface{}{
				"type":   "Quote",
				"symbol": streamingSymbol,
			})
			slog.Info(fmt.Sprintf("TastyTrade: Added Quote subscription for %s (original: %s)", streamingSymbol, symbol))
		}

	// Add Greeks subscription for option symbols only if requested
	if containsString(dataTypes, "Greeks") && p.isOptionSymbol(symbol) {
		subscriptions = append(subscriptions, map[string]interface{}{
			"type":   "Greeks",
			"symbol": streamingSymbol,
		})
		slog.Info(fmt.Sprintf("TastyTrade: Added Greeks subscription for %s (original: %s)", streamingSymbol, symbol))
	} else if containsString(dataTypes, "Greeks") && !p.isOptionSymbol(symbol) {
		slog.Info(fmt.Sprintf("TastyTrade: Skipping Greeks subscription for non-option symbol %s", symbol))
	}
	}

	// Send subscription message
	subscriptionMsg := map[string]interface{}{
		"type":    "FEED_SUBSCRIPTION",
		"channel": 1,
		"reset":   false,
		"add":     subscriptions,
	}

	slog.Info(fmt.Sprintf("TastyTrade: Sending subscription message with %d subscriptions for data_types: %v", len(subscriptions), dataTypes))
	
	if err := p.streamingState.streamConnection.WriteJSON(subscriptionMsg); err != nil {
		slog.Error("Failed to send subscription message", "error", err)
		return false, err
	}

	// Update subscribed symbols
	for _, symbol := range symbols {
		p.streamingState.subscribedSymbols[symbol] = true
	}

	slog.Info(fmt.Sprintf("TastyTrade: Subscribed to %d symbols with %d subscriptions", len(symbols), len(subscriptions)))
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

	if err := p.streamingState.streamConnection.WriteJSON(unsubscriptionMsg); err != nil {
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

// dxlinkStreamingSetup executes DXLink setup sequence for streaming connection.
// Exact conversion of Python _dxlink_streaming_setup method.
func (p *TastyTradeProvider) dxlinkStreamingSetup(ctx context.Context) error {
	conn := p.streamingState.streamConnection
	
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
	p.streamingState.recvLock.Lock()
	var setupResponse map[string]interface{}
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
	
	if err := conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send AUTH message: %w", err)
	}
	
	// Wait for AUTH_STATE: AUTHORIZED
	p.streamingState.recvLock.Lock()
	var authSuccess map[string]interface{}
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
	
	if err := conn.WriteJSON(channelMsg); err != nil {
		return fmt.Errorf("failed to send CHANNEL_REQUEST: %w", err)
	}
	
	// Wait for CHANNEL_OPENED
	p.streamingState.recvLock.Lock()
	var channelResponse map[string]interface{}
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
			"Quote":  []string{"eventType", "eventSymbol", "bidPrice", "askPrice", "bidSize", "askSize"},
			"Greeks": []string{"eventType", "eventSymbol", "delta", "gamma", "theta", "vega", "volatility"},
		},
	}
	
	if err := conn.WriteJSON(feedSetupMsg); err != nil {
		return fmt.Errorf("failed to send FEED_SETUP: %w", err)
	}
	
	// Wait for FEED_CONFIG
	p.streamingState.recvLock.Lock()
	var feedResponse map[string]interface{}
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
func (p *TastyTradeProvider) readerGoroutine() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Reader panic recovered", "panic", r)
		}
		p.streamingState.isConnected = false
		if p.streamingState.streamConnection != nil {
			p.streamingState.streamConnection.Close()
			p.streamingState.streamConnection = nil
		}
		close(p.streamingState.messageChan) // Signal processing loop
	}()

	for {
		select {
		case <-p.streamingState.shutdownEvent:
			slog.Debug("Reader shutdown requested")
			return
		default:
			// No timeout here - blocking read
			p.streamingState.recvLock.Lock()
			var message map[string]interface{}
			err := p.streamingState.streamConnection.ReadJSON(&message)
			p.streamingState.recvLock.Unlock()
			if err != nil {
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

// processStreamingData processes incoming streaming data with automatic recovery using channels.
func (p *TastyTradeProvider) processStreamingData(ctx context.Context) {
	slog.Info("TastyTrade: Starting streaming data processor with channel-based architecture")
	
	defer func() {
		if r := recover(); r != nil {
			slog.Error("TastyTrade: Streaming processor panic recovered", "panic", r)
			// Mark connection as failed and clean up
			p.streamingState.isConnected = false
			if p.streamingState.streamConnection != nil {
				p.streamingState.streamConnection.Close()
				p.streamingState.streamConnection = nil
			}
		}
		slog.Info("TastyTrade: Streaming data processor stopped")
	}()
	
	// Store current subscriptions for recovery
	currentSubscriptions := make(map[string]bool)
	recoveryAttempt := 0
	maxRecoveryAttempts := 5
	
	// Start periodic keepalive task
	keepaliveCtx, keepaliveCancel := context.WithCancel(ctx)
	defer keepaliveCancel()
	go p.periodicKeepalive(keepaliveCtx)
	
	for {
		select {
		case <-ctx.Done():
			slog.Info("TastyTrade: Streaming processor cancelled")
			return
		case <-p.streamingState.shutdownEvent:
			slog.Info("TastyTrade: Streaming processor shutdown requested")
			return
		default:
			// Main streaming loop: use channels while connected
			if p.streamingState.isConnected && p.streamingState.streamConnection != nil {
				// Start reader goroutine
				go p.readerGoroutine()
				
				// Process messages from channels
				for {
					select {
					case <-ctx.Done():
						return
					case <-p.streamingState.shutdownEvent:
						return
					case message, ok := <-p.streamingState.messageChan:
						if !ok {
							// Channel closed - connection lost
							slog.Info("TastyTrade: Message channel closed, connection lost")
							p.handleConnectionLoss("Message channel closed", currentSubscriptions)
							goto recovery
						}
						
						slog.Debug(fmt.Sprintf("TastyTrade: Received message type: %s", message["type"]))
						
						// Process different message types
						switch message["type"] {
						case "FEED_DATA":
							feedData, ok := message["data"].([]interface{})
							if ok {
								slog.Debug(fmt.Sprintf("TastyTrade: Processing FEED_DATA with %d items", len(feedData)))
								p.processFeedEvents(feedData)
							} else {
								slog.Warn("TastyTrade: FEED_DATA message has no data field or wrong type")
							}
						case "KEEPALIVE":
							slog.Debug("TastyTrade: Received keepalive from server")
						default:
							slog.Debug(fmt.Sprintf("TastyTrade: Received message: %s", message["type"]))
						}
						
						// Reset recovery attempt counter on successful data processing
						recoveryAttempt = 0
						
					case err := <-p.streamingState.errorChan:
						// Connection error from reader
						slog.Error("TastyTrade: Connection error from reader", "error", err)
						p.handleConnectionLoss(fmt.Sprintf("Reader error: %v", err), currentSubscriptions)
						goto recovery
						
					case <-time.After(30 * time.Second):
						// Timeout - check if connection is still alive
						if !p.streamingState.isConnected {
							slog.Info("TastyTrade: Connection marked as disconnected during timeout")
							p.handleConnectionLoss("Connection timeout", currentSubscriptions)
							goto recovery
						}
					}
				}
			}
			
		recovery:
			// Attempt recovery
			if recoveryAttempt < maxRecoveryAttempts {
				recoveryAttempt++
				slog.Info(fmt.Sprintf("TastyTrade: Attempting recovery #%d/%d", recoveryAttempt, maxRecoveryAttempts))
				
				// Attempt to recover the connection
				if p.attemptConnectionRecovery(ctx, currentSubscriptions, recoveryAttempt) {
					slog.Info("TastyTrade: Recovery successful, resuming streaming")
					continue
				} else {
					slog.Error(fmt.Sprintf("TastyTrade: Recovery attempt #%d failed", recoveryAttempt))
					
					// Wait before next attempt with exponential backoff
					delay := time.Duration(min(5*(1<<(recoveryAttempt-1)), 60)) * time.Second
					slog.Info(fmt.Sprintf("TastyTrade: Waiting %v before next recovery attempt", delay))
					
					select {
					case <-time.After(delay):
						continue
					case <-ctx.Done():
						return
					case <-p.streamingState.shutdownEvent:
						return
					}
				}
			} else {
				// Max recovery attempts reached
				slog.Error("TastyTrade: Max recovery attempts reached, stopping streaming processor")
				return
			}
		}
	}
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
				
				if err := p.streamingState.streamConnection.WriteJSON(keepaliveMsg); err != nil {
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


// processFeedEvents processes FEED_DATA events and sends to streaming cache or queue.
// Exact conversion of Python _process_feed_events method.
func (p *TastyTradeProvider) processFeedEvents(feedData []interface{}) {
	if p.streamingState.streamingQueue == nil && p.streamingState.streamingCache == nil {
		slog.Warn("TastyTrade: No streaming queue or cache available for feed events")
		return
	}
	
	// Process Quote events
	quoteData := p.processQuoteFeedData(feedData)
	if len(quoteData) > 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: Processing %d quote updates", len(quoteData)))
		for symbol, quote := range quoteData {
			standardSymbol := p.convertSymbolToStandardFormat(symbol)
			
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
	
	// Process Greeks events
	greeksData := p.processGreeksFeedData(feedData)
	if len(greeksData) > 0 {
		slog.Debug(fmt.Sprintf("TastyTrade: Processing %d Greeks updates", len(greeksData)))
		for symbol, greeks := range greeksData {
			standardSymbol := p.convertSymbolToStandardFormat(symbol)
			
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
						"delta":               itemArray[i+2],
						"gamma":               itemArray[i+3],
						"theta":               itemArray[i+4],
						"vega":                itemArray[i+5],
						"implied_volatility":  nil, // Default to nil
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

// handleConnectionLoss handles connection loss by updating state and preparing for recovery.
// Exact conversion of Python _handle_connection_loss method.
func (p *TastyTradeProvider) handleConnectionLoss(reason string, currentSubscriptions map[string]bool) {
	slog.Warn(fmt.Sprintf("TastyTrade: Connection lost - %s", reason))
	
	// Update connection state
	p.streamingState.isConnected = false
	p.streamingState.connectionReady = make(chan struct{})
	
	// Store current subscriptions for recovery
	if p.streamingState.subscribedSymbols != nil {
		for symbol := range p.streamingState.subscribedSymbols {
			currentSubscriptions[symbol] = true
		}
		slog.Info(fmt.Sprintf("TastyTrade: Stored %d subscriptions for recovery", len(currentSubscriptions)))
	}
	
	// Clean up connection
	if p.streamingState.streamConnection != nil {
		p.streamingState.streamConnection.Close()
		p.streamingState.streamConnection = nil
	}
}

// attemptConnectionRecovery attempts to recover the streaming connection and restore subscriptions.
// Exact conversion of Python _attempt_connection_recovery method.
func (p *TastyTradeProvider) attemptConnectionRecovery(ctx context.Context, currentSubscriptions map[string]bool, attemptNumber int) bool {
	slog.Info(fmt.Sprintf("TastyTrade: Starting connection recovery attempt #%d", attemptNumber))
	
	// Attempt to reconnect
	success, err := p.ConnectStreaming(ctx)
	if err != nil || !success {
		slog.Error("TastyTrade: Connection recovery failed", "error", err)
		return false
	}
	
	slog.Info("TastyTrade: Connection recovery successful")
	
	// Restore subscriptions if we had any
	if len(currentSubscriptions) > 0 {
		slog.Info(fmt.Sprintf("TastyTrade: Restoring %d subscriptions", len(currentSubscriptions)))
		
		// Convert map back to slice for subscription
		var symbolsList []string
		for symbol := range currentSubscriptions {
			symbolsList = append(symbolsList, symbol)
		}
		
		// Resubscribe to all symbols (both quotes and Greeks)
		success, err := p.SubscribeToSymbols(ctx, symbolsList, []string{"Quote", "Greeks"})
		if err != nil || !success {
			slog.Warn("TastyTrade: Failed to restore some subscriptions", "error", err)
			// Don't fail recovery just because subscription restoration failed
		} else {
			slog.Info(fmt.Sprintf("TastyTrade: Successfully restored %d subscriptions", len(symbolsList)))
		}
	}
	
	return true
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
			date := datePart  // YYMMDD
			optionType := symbol[i+6] // C or P
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

	tastytradeOrder := p.transformToTastyTradeOrder(orderData)
	
	slog.Debug("TastyTrade: Sending dry-run payload", "payload", tastytradeOrder)
	
	endpoint := fmt.Sprintf("/accounts/%s/orders/dry-run", p.accountID)
	responseBytes, err := p.makeAuthenticatedRequest(ctx, "POST", endpoint, tastytradeOrder)
	if err != nil {
		// Handle 422 responses specially - they contain error details in the body
		if strings.Contains(err.Error(), "422") {
			return map[string]interface{}{
				"status":             "error",
				"validation_errors":  []string{"Order validation failed"},
				"commission":         0,
				"cost":              0,
				"fees":              0,
				"order_cost":        0,
				"margin_change":     0,
				"buying_power_effect": 0,
				"day_trades":        0,
				"estimated_total":   0,
			}, nil
		}
		return nil, fmt.Errorf("failed to preview order: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse preview response: %w", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		data = make(map[string]interface{})
	}

	errorData, hasError := response["error"].(map[string]interface{})
	if hasError {
		errorMessages := []string{}
		if errorsArray, ok := errorData["errors"].([]interface{}); ok {
			for _, errorItem := range errorsArray {
				if errorMap, ok := errorItem.(map[string]interface{}); ok {
					if message, ok := errorMap["message"].(string); ok && message != "" {
						errorMessages = append(errorMessages, message)
					}
				}
			}
		}
		
		return map[string]interface{}{
			"status":             "error",
			"validation_errors":  errorMessages,
			"commission":         0,
			"cost":              0,
			"fees":              0,
			"order_cost":        0,
			"margin_change":     0,
			"buying_power_effect": 0,
			"day_trades":        0,
			"estimated_total":   0,
		}, nil
	}

	// Check for warnings
	if warnings, ok := data["warnings"].([]interface{}); ok && len(warnings) > 0 {
		errorWarnings := []interface{}{}
		for _, warning := range warnings {
			if warningMap, ok := warning.(map[string]interface{}); ok {
				if message, ok := warningMap["message"].(string); ok {
					if message != "Your order will begin working during next valid session." {
						errorWarnings = append(errorWarnings, warning)
					}
				}
			}
		}
		
		if len(errorWarnings) > 0 {
			errorMessages := []string{}
			for _, warning := range errorWarnings {
				if warningMap, ok := warning.(map[string]interface{}); ok {
					if message, ok := warningMap["message"].(string); ok {
						errorMessages = append(errorMessages, message)
					}
				}
			}
			
			return map[string]interface{}{
				"status":             "error",
				"validation_errors":  errorMessages,
				"commission":         0,
				"cost":              0,
				"fees":              0,
				"order_cost":        0,
				"margin_change":     0,
				"buying_power_effect": 0,
				"day_trades":        0,
				"estimated_total":   0,
			}, nil
		}
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
		"status":             "ok",
		"commission":         commission,
		"cost":              orderPrice,
		"fees":              fees,
		"order_cost":        orderPrice,
		"margin_change":     marginChange,
		"buying_power_effect": buyingPowerImpact,
		"day_trades":        0, // Not provided in dry-run
		"validation_errors":  []string{},
		"estimated_total":   estimatedTotal,
	}, nil
}

// === Helper Methods ===

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
				date := datePart  // YYMMDD
				optionType := symbolNoDot[i+6] // C or P
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

// transformToTastyTradeOrder transforms standard order to TastyTrade format.
func (p *TastyTradeProvider) transformToTastyTradeOrder(orderData map[string]interface{}) map[string]interface{} {
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

	symbol := getString(orderData, "symbol")
	side := getString(orderData, "side")
	orderType := getString(orderData, "order_type")
	tif := getString(orderData, "time_in_force")
	quantity := getFloat(orderData, "quantity")

	// Determine instrument type
	instrumentType := "Equity"
	if p.isOptionSymbol(symbol) {
		instrumentType = "Equity Option"
	}

	// Build TastyTrade order
	tastytrade_order := map[string]interface{}{
		"order-type":     orderTypeMap[orderType],
		"time-in-force":  tifMap[tif],
		"legs": []map[string]interface{}{
			{
				"instrument-type": instrumentType,
				"action":          actionMap[side],
				"quantity":        int(quantity),
				"symbol":          symbol,
			},
		},
	}

	// Add price for limit orders
	if orderType == "limit" || orderType == "stop_limit" {
		if price := getFloat(orderData, "price"); price != 0 {
			tastytrade_order["price"] = price
			if side == "sell" {
				tastytrade_order["price-effect"] = "Credit"
			} else {
				tastytrade_order["price-effect"] = "Debit"
			}
		}
	}

	return tastytrade_order
}

// getString gets a string value from map with default empty string.
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
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
	dxlinkURL   string
	quoteToken  string
	channelID   int
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
						"eventType":   itemArray[i],     // "Candle"
						"eventSymbol": itemArray[i+1],   // "SPY{=d}"
						"time":        timestamp,        // timestamp in ms
						"open":        itemArray[i+3],   // open price
						"high":        itemArray[i+4],   // high price
						"low":         itemArray[i+5],   // low price
						"close":       itemArray[i+6],   // close price
						"volume":      itemArray[i+7],   // volume
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
