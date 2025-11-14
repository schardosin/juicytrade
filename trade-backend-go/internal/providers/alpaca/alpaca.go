package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"trade-backend-go/internal/config"
	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/utils"
)

// AlpacaProvider implements the Provider interface for Alpaca.
// This is an exact conversion of the Python AlpacaProvider class.
type AlpacaProvider struct {
	*base.BaseProviderImpl
	
	// Credentials - exact same as Python
	APIKey    string
	APISecret string
	BaseURL   string
	DataURL   string
	UsePaper  bool
	
	// HTTP client for API requests
	httpClient *utils.HTTPClient
	
	// Caching - exact same TTL as Python
	quoteCache      *utils.QuoteCache      // 15min TTL
	expirationCache *utils.ExpirationCache // Daily TTL
	symbolCache     *utils.SymbolLookupCache // 6 hours TTL
	
	// Streaming components (to be implemented)
	// optionStream, stockStream, tradingStream would go here
	
	// Connection management
	connecting       bool
	disableOptionStream bool
}

// NewAlpacaProvider creates a new Alpaca provider instance.
// Exact conversion of Python AlpacaProvider.__init__ method.
func NewAlpacaProvider(apiKey, apiSecret, baseURL, dataURL string, usePaper bool) *AlpacaProvider {
	provider := &AlpacaProvider{
		BaseProviderImpl: base.NewBaseProvider("Alpaca"),
		APIKey:          apiKey,
		APISecret:       apiSecret,
		BaseURL:         baseURL,
		DataURL:         dataURL,
		UsePaper:        usePaper,
		httpClient:      utils.NewHTTPClient(),
		quoteCache:      utils.NewQuoteCache(),
		expirationCache: utils.NewExpirationCache(),
		symbolCache:     utils.NewSymbolLookupCache(),
	}
	
	provider.LogInfo("Alpaca provider initialized successfully")
	return provider
}

// NewAlpacaProviderFromConfig creates an Alpaca provider from configuration.
// Matches the Python provider factory logic.
func NewAlpacaProviderFromConfig(cfg *config.Settings) *AlpacaProvider {
	var apiKey, apiSecret, baseURL string
	
	// Determine which credentials to use based on paper/live setting
	// This logic matches the Python provider selection
	if cfg.Provider == "alpaca" {
		// For now, default to paper trading - this would be configurable
		usePaper := true // This should come from config
		
		if usePaper {
			apiKey = cfg.AlpacaAPIKeyPaper
			apiSecret = cfg.AlpacaAPISecretPaper
			baseURL = cfg.AlpacaBaseURLPaper
		} else {
			apiKey = cfg.AlpacaAPIKeyLive
			apiSecret = cfg.AlpacaAPISecretLive
			baseURL = cfg.AlpacaBaseURLLive
		}
		
		return NewAlpacaProvider(apiKey, apiSecret, baseURL, cfg.AlpacaDataURL, usePaper)
	}
	
	return nil
}

// === Market Data Methods - Exact conversions ===

// GetStockQuote gets the latest stock quote for a symbol.
// Exact conversion of Python get_stock_quote method.
func (ap *AlpacaProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	url := fmt.Sprintf("%s/v2/stocks/quotes/latest", ap.DataURL)
	
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}
	
	params := map[string]string{
		"symbols": symbol,
	}
	
	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError(fmt.Sprintf("get_stock_quote for %s", symbol), err)
		return nil, err
	}
	
	var response struct {
		Quotes map[string]struct {
			AskPrice *float64 `json:"ap"`
			BidPrice *float64 `json:"bp"`
		} `json:"quotes"`
	}
	
	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal stock quote response", err)
		return nil, err
	}
	
	if quote, exists := response.Quotes[symbol]; exists {
		return &models.StockQuote{
			Symbol:    symbol,
			Ask:       quote.AskPrice,
			Bid:       quote.BidPrice,
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}
	
	return nil, fmt.Errorf("quote not found for symbol %s", symbol)
}

// GetStockQuotes gets stock quotes for multiple symbols.
// Exact conversion of Python get_stock_quotes method.
func (ap *AlpacaProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	url := fmt.Sprintf("%s/v2/stocks/quotes/latest", ap.DataURL)
	
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}
	
	params := map[string]string{
		"symbols": strings.Join(symbols, ","),
	}
	
	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError(fmt.Sprintf("get_stock_quotes for %v", symbols), err)
		return nil, err
	}
	
	var response struct {
		Quotes map[string]struct {
			AskPrice *float64 `json:"ap"`
			BidPrice *float64 `json:"bp"`
		} `json:"quotes"`
	}
	
	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal stock quotes response", err)
		return nil, err
	}
	
	result := make(map[string]*models.StockQuote)
	for symbol, quote := range response.Quotes {
		result[symbol] = &models.StockQuote{
			Symbol:    symbol,
			Ask:       quote.AskPrice,
			Bid:       quote.BidPrice,
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}
	
	return result, nil
}

// GetExpirationDates gets available expiration dates for options on a symbol with universal enhanced structure.
// Exact conversion of Python get_expiration_dates method with caching.
func (ap *AlpacaProvider) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	// Check cache first (same as Python)
	if cachedDates, found := ap.expirationCache.GetExpirationDates(symbol); found {
		ap.LogInfo(fmt.Sprintf("Using cached expiration dates for %s (%d dates)", symbol, len(cachedDates)))
		return cachedDates, nil
	}
	
	ap.LogInfo(fmt.Sprintf("Cache miss - fetching expiration dates for %s from API", symbol))
	
	url := fmt.Sprintf("%s/v2/options/contracts", ap.BaseURL)
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}
	
	params := map[string]string{
		"underlying_symbols":    symbol,
		"status":               "active",
		"expiration_date_gte":  time.Now().Format("2006-01-02"),
	}
	
	expirationDates := make(map[string]bool)
	pageToken := ""
	
	for {
		if pageToken != "" {
			params["page_token"] = pageToken
		}
		
		resp, err := ap.httpClient.Get(ctx, url, headers, params)
		if err != nil {
			ap.LogError(fmt.Sprintf("get_expiration_dates API call for %s", symbol), err)
			break
		}
		
		var response struct {
			OptionContracts []struct {
				ExpirationDate string `json:"expiration_date"`
			} `json:"option_contracts"`
			NextPageToken *string `json:"next_page_token"`
		}
		
		if err := json.Unmarshal(resp.Body, &response); err != nil {
			ap.LogError("unmarshal expiration dates response", err)
			break
		}
		
		for _, contract := range response.OptionContracts {
			expirationDates[contract.ExpirationDate] = true
		}
		
		if response.NextPageToken == nil || *response.NextPageToken == "" {
			break
		}
		pageToken = *response.NextPageToken
	}
	
	// Convert to sorted slice (same as Python)
	sortedDates := make([]string, 0, len(expirationDates))
	for date := range expirationDates {
		sortedDates = append(sortedDates, date)
	}
	
	// Sort dates (Go's sort is lexicographic which works for YYYY-MM-DD)
	// This matches Python's sorted() behavior
	for i := 0; i < len(sortedDates)-1; i++ {
		for j := i + 1; j < len(sortedDates); j++ {
			if sortedDates[i] > sortedDates[j] {
				sortedDates[i], sortedDates[j] = sortedDates[j], sortedDates[i]
			}
		}
	}
	
	// Create enhanced dates structure (same as Python)
	enhancedDates := make([]map[string]interface{}, len(sortedDates))
	for i, date := range sortedDates {
		enhancedDates[i] = map[string]interface{}{
			"date":   date,
			"symbol": symbol,
			"type":   "unknown",
		}
	}
	
	ap.LogInfo(fmt.Sprintf("Retrieved %d total expiration dates for %s", len(enhancedDates), symbol))
	
	// Cache the results (same as Python)
	ap.expirationCache.SetExpirationDates(symbol, enhancedDates)
	
	return enhancedDates, nil
}

// === Helper Methods - Exact conversions ===

// isOptionSymbol checks if a symbol is an option symbol.
// Exact conversion of Python _is_option_symbol method.
func (ap *AlpacaProvider) isOptionSymbol(symbol string) bool {
	return ap.IsOptionSymbol(symbol) // Use base implementation
}

// parseOptionSymbol parses option symbol to extract components.
// Exact conversion of Python _parse_option_symbol method.
func (ap *AlpacaProvider) parseOptionSymbol(symbol string) map[string]interface{} {
	// Option symbol format: [UNDERLYING][YYMMDD][C/P][STRIKE:8]
	// The key insight is that the last 15 characters are always:
	// [YYMMDD:6][C/P:1][STRIKE:8] = 15 characters total
	
	if len(symbol) < 15 {
		return nil
	}
	
	// The option part is always the last 15 characters
	optionPart := symbol[len(symbol)-15:]
	underlying := symbol[:len(symbol)-15]
	
	if underlying == "" {
		return nil
	}
	
	// Parse the option part
	datePart := optionPart[:6]  // YYMMDD
	optionType := optionPart[6] // C or P
	strikePart := optionPart[7:] // 8-digit strike
	
	// Validate option type
	if optionType != 'C' && optionType != 'P' {
		return nil
	}
	
	// Validate strike part is 8 digits
	if len(strikePart) != 8 {
		return nil
	}
	
	// Check if strike part is all digits
	for _, char := range strikePart {
		if char < '0' || char > '9' {
			return nil
		}
	}
	
	// Parse expiry date
	year, _ := strconv.Atoi(datePart[:2])
	month, _ := strconv.Atoi(datePart[2:4])
	day, _ := strconv.Atoi(datePart[4:6])
	
	// Validate date components
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil
	}
	
	year += 2000 // Convert YY to YYYY
	expiryDate := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	
	// Parse strike price (divide by 1000 to get actual price)
	strikeInt, _ := strconv.Atoi(strikePart)
	strikePrice := float64(strikeInt) / 1000.0
	
	optionTypeStr := "call"
	if optionType == 'P' {
		optionTypeStr = "put"
	}
	
	return map[string]interface{}{
		"underlying": underlying,
		"type":       optionTypeStr,
		"strike":     strikePrice,
		"expiry":     expiryDate,
	}
}

// === Placeholder methods for interface compliance ===
// These will be implemented in subsequent iterations

func (ap *AlpacaProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	// Get current positions
	positions, err := ap.GetPositions(ctx)
	if err != nil {
		return nil, err
	}
	
	// Convert to enhanced format using base provider logic
	return ap.ConvertPositionsToEnhanced(positions), nil
}

func (ap *AlpacaProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	return false, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	return false, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	return false, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented yet")
}
