package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"trade-backend-go/internal/config"
	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/utils"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/gorilla/websocket"
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
	quoteCache      *utils.QuoteCache        // 15min TTL
	expirationCache *utils.ExpirationCache   // Daily TTL
	symbolCache     *utils.SymbolLookupCache // 6 hours TTL

	// Streaming components using Alpaca SDK
	stockStream   *stream.StocksClient
	optionStream  *stream.OptionClient
	tradingClient *alpaca.Client

	// Connection management
	connecting          bool
	disableOptionStream bool
	streamMutex         sync.Mutex
	subscribedSymbols   map[string]bool
	streamingQueue      chan *models.MarketData

	// Account event streaming
	orderEventCallback func(*models.OrderEvent)
	ctxCancel          context.CancelFunc
}

// NewAlpacaProvider creates a new Alpaca provider instance.
// Exact conversion of Python AlpacaProvider.__init__ method.
func NewAlpacaProvider(apiKey, apiSecret, baseURL, dataURL string, usePaper bool) *AlpacaProvider {
	provider := &AlpacaProvider{
		BaseProviderImpl:  base.NewBaseProvider("Alpaca"),
		APIKey:            apiKey,
		APISecret:         apiSecret,
		BaseURL:           baseURL,
		DataURL:           dataURL,
		UsePaper:          usePaper,
		httpClient:        utils.NewHTTPClient(),
		quoteCache:        utils.NewQuoteCache(),
		expirationCache:   utils.NewExpirationCache(),
		symbolCache:       utils.NewSymbolLookupCache(),
		subscribedSymbols: make(map[string]bool),
		streamingQueue:    make(chan *models.MarketData, 1000),
	}

	// Initialize trading client for account operations
	provider.tradingClient = alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
		BaseURL:   baseURL,
	})

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

	// Use BaseURL (trading API) for options contracts endpoint
	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/options/contracts", baseURL)
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	params := map[string]string{
		"underlying_symbols":  symbol,
		"status":              "active",
		"expiration_date_gte": time.Now().Format("2006-01-02"),
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

// === Data Transformation Methods ===

// transformPosition transforms Alpaca position to our standard model.
// Exact conversion of Python _transform_position method.
func (ap *AlpacaProvider) transformPosition(rawPosition interface{}) *models.Position {
	// Handle struct type from API response
	var symbol, qty, marketValue, costBasis, unrealizedPL, unrealizedPLPC string
	var currentPrice, avgEntryPrice, assetClass string
	var lastdayPrice *string

	switch pos := rawPosition.(type) {
	case struct {
		Symbol         string
		Qty            string
		MarketValue    string
		CostBasis      string
		UnrealizedPL   string
		UnrealizedPLPC string
		CurrentPrice   string
		AvgEntryPrice  string
		AssetClass     string
		LastdayPrice   *string
	}:
		symbol = pos.Symbol
		qty = pos.Qty
		marketValue = pos.MarketValue
		costBasis = pos.CostBasis
		unrealizedPL = pos.UnrealizedPL
		unrealizedPLPC = pos.UnrealizedPLPC
		currentPrice = pos.CurrentPrice
		avgEntryPrice = pos.AvgEntryPrice
		assetClass = pos.AssetClass
		lastdayPrice = pos.LastdayPrice
	default:
		return nil
	}

	// Parse numeric values
	qtyFloat, _ := strconv.ParseFloat(qty, 64)
	marketValueFloat, _ := strconv.ParseFloat(marketValue, 64)
	costBasisFloat, _ := strconv.ParseFloat(costBasis, 64)
	unrealizedPLFloat, _ := strconv.ParseFloat(unrealizedPL, 64)
	currentPriceFloat, _ := strconv.ParseFloat(currentPrice, 64)
	avgEntryPriceFloat, _ := strconv.ParseFloat(avgEntryPrice, 64)

	// Parse unrealized P/L percentage
	var unrealizedPLPCFloat *float64
	if unrealizedPLPC != "" {
		if val, err := strconv.ParseFloat(unrealizedPLPC, 64); err == nil {
			unrealizedPLPCFloat = &val
		}
	}

	// Parse lastday price (if 0, set to nil as it indicates same-day trade)
	var lastdayPriceFloat *float64
	if lastdayPrice != nil && *lastdayPrice != "" {
		if val, err := strconv.ParseFloat(*lastdayPrice, 64); err == nil && val != 0 {
			lastdayPriceFloat = &val
		}
	}

	// Determine side
	side := "long"
	if qtyFloat < 0 {
		side = "short"
	}

	position := &models.Position{
		Symbol:         symbol,
		Qty:            qtyFloat,
		Side:           side,
		MarketValue:    marketValueFloat,
		CostBasis:      costBasisFloat,
		UnrealizedPL:   unrealizedPLFloat,
		UnrealizedPLPC: unrealizedPLPCFloat,
		CurrentPrice:   currentPriceFloat,
		AvgEntryPrice:  avgEntryPriceFloat,
		AssetClass:     assetClass,
		LastdayPrice:   lastdayPriceFloat,
		DateAcquired:   nil, // Alpaca does not provide this field
	}

	// Parse option-specific information if it's an option
	if assetClass == "us_option" {
		optionInfo := ap.parseOptionSymbol(symbol)
		if optionInfo != nil {
			if underlying, ok := optionInfo["underlying"].(string); ok {
				position.UnderlyingSymbol = &underlying
			}
			if optType, ok := optionInfo["type"].(string); ok {
				position.OptionType = &optType
			}
			if strike, ok := optionInfo["strike"].(float64); ok {
				position.StrikePrice = &strike
			}
			if expiry, ok := optionInfo["expiry"].(string); ok {
				position.ExpiryDate = &expiry
			}
		}
	}

	return position
}

// transformAccount transforms Alpaca account to our standard model.
// Exact conversion of Python _transform_account method.
func (ap *AlpacaProvider) transformAccount(rawAccount interface{}) *models.Account {
	// Handle struct type from API response
	var id, accountNumber, status, currency string
	var buyingPower, cash, portfolioValue, equity string
	var daytradingBuyingPower, regtBuyingPower, optionsBuyingPower string
	var patternDayTrader, tradingBlocked, transfersBlocked, accountBlocked bool
	var createdAt, multiplier string
	var longMarketValue, shortMarketValue, initialMargin, maintenanceMargin string
	var daytradeCount, optionsApprovedLevel, optionsTradingLevel int

	switch acc := rawAccount.(type) {
	case struct {
		ID                    string
		AccountNumber         string
		Status                string
		Currency              string
		BuyingPower           string
		Cash                  string
		PortfolioValue        string
		Equity                string
		DaytradingBuyingPower string
		RegtBuyingPower       string
		OptionsBuyingPower    string
		PatternDayTrader      bool
		TradingBlocked        bool
		TransfersBlocked      bool
		AccountBlocked        bool
		CreatedAt             string
		Multiplier            string
		LongMarketValue       string
		ShortMarketValue      string
		InitialMargin         string
		MaintenanceMargin     string
		DaytradeCount         int
		OptionsApprovedLevel  int
		OptionsTradingLevel   int
	}:
		id = acc.ID
		accountNumber = acc.AccountNumber
		status = acc.Status
		currency = acc.Currency
		buyingPower = acc.BuyingPower
		cash = acc.Cash
		portfolioValue = acc.PortfolioValue
		equity = acc.Equity
		daytradingBuyingPower = acc.DaytradingBuyingPower
		regtBuyingPower = acc.RegtBuyingPower
		optionsBuyingPower = acc.OptionsBuyingPower
		patternDayTrader = acc.PatternDayTrader
		tradingBlocked = acc.TradingBlocked
		transfersBlocked = acc.TransfersBlocked
		accountBlocked = acc.AccountBlocked
		createdAt = acc.CreatedAt
		multiplier = acc.Multiplier
		longMarketValue = acc.LongMarketValue
		shortMarketValue = acc.ShortMarketValue
		initialMargin = acc.InitialMargin
		maintenanceMargin = acc.MaintenanceMargin
		daytradeCount = acc.DaytradeCount
		optionsApprovedLevel = acc.OptionsApprovedLevel
		optionsTradingLevel = acc.OptionsTradingLevel
	default:
		return nil
	}

	// Parse numeric values
	var buyingPowerFloat, cashFloat, portfolioValueFloat, equityFloat *float64
	var dayTradingBuyingPowerFloat, regtBuyingPowerFloat, optionsBuyingPowerFloat *float64
	var longMarketValueFloat, shortMarketValueFloat, initialMarginFloat, maintenanceMarginFloat *float64

	if buyingPower != "" {
		if val, err := strconv.ParseFloat(buyingPower, 64); err == nil {
			buyingPowerFloat = &val
		}
	}
	if cash != "" {
		if val, err := strconv.ParseFloat(cash, 64); err == nil {
			cashFloat = &val
		}
	}
	if portfolioValue != "" {
		if val, err := strconv.ParseFloat(portfolioValue, 64); err == nil {
			portfolioValueFloat = &val
		}
	}
	if equity != "" {
		if val, err := strconv.ParseFloat(equity, 64); err == nil {
			equityFloat = &val
		}
	}
	if daytradingBuyingPower != "" {
		if val, err := strconv.ParseFloat(daytradingBuyingPower, 64); err == nil {
			dayTradingBuyingPowerFloat = &val
		}
	}
	if regtBuyingPower != "" {
		if val, err := strconv.ParseFloat(regtBuyingPower, 64); err == nil {
			regtBuyingPowerFloat = &val
		}
	}
	if optionsBuyingPower != "" {
		if val, err := strconv.ParseFloat(optionsBuyingPower, 64); err == nil {
			optionsBuyingPowerFloat = &val
		}
	}
	if longMarketValue != "" {
		if val, err := strconv.ParseFloat(longMarketValue, 64); err == nil {
			longMarketValueFloat = &val
		}
	}
	if shortMarketValue != "" {
		if val, err := strconv.ParseFloat(shortMarketValue, 64); err == nil {
			shortMarketValueFloat = &val
		}
	}
	if initialMargin != "" {
		if val, err := strconv.ParseFloat(initialMargin, 64); err == nil {
			initialMarginFloat = &val
		}
	}
	if maintenanceMargin != "" {
		if val, err := strconv.ParseFloat(maintenanceMargin, 64); err == nil {
			maintenanceMarginFloat = &val
		}
	}

	// Default currency if empty
	if currency == "" {
		currency = "USD"
	}

	return &models.Account{
		AccountID:             id,
		AccountNumber:         &accountNumber,
		Status:                status,
		Currency:              currency,
		BuyingPower:           buyingPowerFloat,
		Cash:                  cashFloat,
		PortfolioValue:        portfolioValueFloat,
		Equity:                equityFloat,
		DayTradingBuyingPower: dayTradingBuyingPowerFloat,
		RegtBuyingPower:       regtBuyingPowerFloat,
		OptionsBuyingPower:    optionsBuyingPowerFloat,
		PatternDayTrader:      &patternDayTrader,
		TradingBlocked:        &tradingBlocked,
		TransfersBlocked:      &transfersBlocked,
		AccountBlocked:        &accountBlocked,
		CreatedAt:             &createdAt,
		Multiplier:            &multiplier,
		LongMarketValue:       longMarketValueFloat,
		ShortMarketValue:      shortMarketValueFloat,
		InitialMargin:         initialMarginFloat,
		MaintenanceMargin:     maintenanceMarginFloat,
		DaytradeCount:         &daytradeCount,
		OptionsApprovedLevel:  &optionsApprovedLevel,
		OptionsTradingLevel:   &optionsTradingLevel,
	}
}

// OptionContractResponse represents the API response structure for an option contract
type OptionContractResponse struct {
	Symbol           string  `json:"symbol"`
	UnderlyingSymbol string  `json:"underlying_symbol"`
	ExpirationDate   string  `json:"expiration_date"`
	StrikePrice      string  `json:"strike_price"`
	Type             string  `json:"type"`
	Bid              *string `json:"bid"`
	Ask              *string `json:"ask"`
	ClosePrice       *string `json:"close_price"`
	Volume           *string `json:"volume"`
	OpenInterest     *string `json:"open_interest"`
}

// transformOptionContract transforms Alpaca option contract to our standard model.
// Exact conversion of Python _transform_option_contract method.
func (ap *AlpacaProvider) transformOptionContract(contract OptionContractResponse) *models.OptionContract {
	symbol := contract.Symbol
	underlyingSymbol := contract.UnderlyingSymbol
	expirationDate := contract.ExpirationDate
	strikeStr := contract.StrikePrice
	optionType := contract.Type
	bid := contract.Bid
	ask := contract.Ask
	closePrice := contract.ClosePrice
	volumeStr := contract.Volume
	openInterestStr := contract.OpenInterest

	// Parse strike price
	strikePrice, err := strconv.ParseFloat(strikeStr, 64)
	if err != nil {
		ap.LogError(fmt.Sprintf("parse strike price for %s", symbol), err)
		return nil
	}

	// Parse bid/ask/close prices
	var bidPrice, askPrice, closePriceFloat *float64
	if bid != nil && *bid != "" {
		if price, err := strconv.ParseFloat(*bid, 64); err == nil {
			bidPrice = &price
		}
	}
	if ask != nil && *ask != "" {
		if price, err := strconv.ParseFloat(*ask, 64); err == nil {
			askPrice = &price
		}
	}
	if closePrice != nil && *closePrice != "" {
		if price, err := strconv.ParseFloat(*closePrice, 64); err == nil {
			closePriceFloat = &price
		}
	}

	// Parse volume and open interest from strings
	var volume, openInterest *int
	if volumeStr != nil && *volumeStr != "" {
		if val, err := strconv.Atoi(*volumeStr); err == nil {
			volume = &val
		}
	}
	if openInterestStr != nil && *openInterestStr != "" {
		if val, err := strconv.Atoi(*openInterestStr); err == nil {
			openInterest = &val
		}
	}

	return &models.OptionContract{
		Symbol:           symbol,
		UnderlyingSymbol: underlyingSymbol,
		ExpirationDate:   expirationDate,
		StrikePrice:      strikePrice,
		Type:             strings.ToLower(optionType),
		Bid:              bidPrice,
		Ask:              askPrice,
		ClosePrice:       closePriceFloat,
		Volume:           volume,
		OpenInterest:     openInterest,
	}
}

// IsMarketOpen checks if the market is currently open.
// Exact conversion of Python is_market_open method.
func (ap *AlpacaProvider) IsMarketOpen(ctx context.Context) (bool, error) {
	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/calendar", baseURL)
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	today := time.Now().Format("20060102") // YYYYMMDD format
	params := map[string]string{
		"start": today,
		"end":   today,
	}

	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError("is_market_open API call", err)
		return false, nil // Assume closed on error for safety
	}

	var response []struct {
		Date  string `json:"date"`
		Open  string `json:"open"`
		Close string `json:"close"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal market calendar response", err)
		return false, nil
	}

	if len(response) == 0 {
		return false, nil // Not a trading day
	}

	marketDay := response[0]
	todayFormatted := time.Now().Format("2006-01-02")

	if marketDay.Date != todayFormatted {
		return false, nil
	}

	// Parse market hours (EST)
	if marketDay.Open == "" || marketDay.Close == "" {
		return false, nil
	}

	// Get current time in EST
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		return false, err
	}
	nowEST := time.Now().In(est)
	currentTime := nowEST.Format("15:04")

	// Check if current time is within market hours
	isOpen := marketDay.Open <= currentTime && currentTime <= marketDay.Close

	ap.LogInfo(fmt.Sprintf("Market status check: Current EST time %s, Market hours %s-%s, Open: %v", currentTime, marketDay.Open, marketDay.Close, isOpen))
	return isOpen, nil
}

// GetLatestOptionQuotes gets latest option quotes for multiple symbols with caching.
// Exact conversion of Python get_latest_option_quotes method.
func (ap *AlpacaProvider) GetLatestOptionQuotes(ctx context.Context, symbols []string) (map[string]map[string]interface{}, error) {
	if len(symbols) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	// Check if market is open
	marketOpen, err := ap.IsMarketOpen(ctx)
	if err != nil {
		marketOpen = true // Assume open on error
	}

	if !marketOpen {
		// Market is closed - check cache first
		cachedResults := make(map[string]map[string]interface{})
		var symbolsToFetch []string

		for _, symbol := range symbols {
			if cachedQuote, found := ap.quoteCache.GetQuote(symbol); found {
				cachedResults[symbol] = cachedQuote
			} else {
				symbolsToFetch = append(symbolsToFetch, symbol)
			}
		}

		if len(cachedResults) > 0 {
			ap.LogInfo(fmt.Sprintf("Market closed - using cached quotes for %d symbols", len(cachedResults)))
		}

		// Fetch fresh data for symbols not in cache
		if len(symbolsToFetch) > 0 {
			freshResults := ap.fetchFreshQuotes(ctx, symbolsToFetch)
			// Cache the fresh results
			for symbol, quote := range freshResults {
				ap.quoteCache.SetQuote(symbol, quote)
				cachedResults[symbol] = quote
			}
		}

		return cachedResults, nil
	}

	// Market is open - always fetch fresh data
	ap.LogInfo(fmt.Sprintf("Market open - fetching fresh quotes for %d symbols", len(symbols)))
	return ap.fetchFreshQuotes(ctx, symbols), nil
}

// fetchFreshQuotes fetches fresh quotes from API (no caching).
// Exact conversion of Python _fetch_fresh_quotes method.
func (ap *AlpacaProvider) fetchFreshQuotes(ctx context.Context, symbols []string) map[string]map[string]interface{} {
	if len(symbols) == 0 {
		return make(map[string]map[string]interface{})
	}

	// Alpaca API has a limit of 100 symbols per request
	batchSize := 100
	allResults := make(map[string]map[string]interface{})

	for i := 0; i < len(symbols); i += batchSize {
		end := min(i+batchSize, len(symbols))
		batchSymbols := symbols[i:end]
		batchResult := ap.getLatestOptionQuotesBatch(ctx, batchSymbols)

		for symbol, quote := range batchResult {
			allResults[symbol] = quote
		}
	}

	ap.LogInfo(fmt.Sprintf("Fetched fresh quotes for %d option symbols", len(allResults)))
	return allResults
}

// getLatestOptionQuotesBatch gets latest option quotes for a batch of symbols (max 100).
// Exact conversion of Python _get_latest_option_quotes_batch method.
func (ap *AlpacaProvider) getLatestOptionQuotesBatch(ctx context.Context, symbols []string) map[string]map[string]interface{} {
	if len(symbols) == 0 {
		return make(map[string]map[string]interface{})
	}

	url := fmt.Sprintf("%s/v1beta1/options/quotes/latest", ap.DataURL)
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
		ap.LogError("get_latest_option_quotes_batch API call", err)
		return make(map[string]map[string]interface{})
	}

	var response struct {
		Quotes map[string]struct {
			BidPrice  *float64 `json:"bp"`
			AskPrice  *float64 `json:"ap"`
			BidSize   *int     `json:"bs"`
			AskSize   *int     `json:"as"`
			Timestamp string   `json:"t"`
		} `json:"quotes"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal option quotes batch response", err)
		return make(map[string]map[string]interface{})
	}

	result := make(map[string]map[string]interface{})
	for symbol, quoteData := range response.Quotes {
		result[symbol] = map[string]interface{}{
			"bid":       quoteData.BidPrice,
			"ask":       quoteData.AskPrice,
			"bid_size":  quoteData.BidSize,
			"ask_size":  quoteData.AskSize,
			"timestamp": quoteData.Timestamp,
		}
	}

	return result
}

// getQuantityFromOrderData extracts quantity from order data, checking both "qty" and "quantity" fields.
// This matches Tradier's implementation to handle frontend sending either field name.
func (ap *AlpacaProvider) getQuantityFromOrderData(data map[string]interface{}) int {
	// DEBUG: Log what we're receiving
	ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Full orderData: %+v", data))
	ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - qty field value: %+v (type: %T)", data["qty"], data["qty"]))
	ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - quantity field value: %+v (type: %T)", data["quantity"], data["quantity"]))

	// Try "qty" first (Python uses this)
	if val, ok := data["qty"].(float64); ok {
		ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Extracted qty as float64: %f -> %d", val, int(val)))
		return int(val)
	}
	if val, ok := data["qty"].(int); ok {
		ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Extracted qty as int: %d", val))
		return val
	}

	// Try "quantity" as fallback (frontend sends this)
	if val, ok := data["quantity"].(float64); ok {
		ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Extracted quantity as float64: %f -> %d", val, int(val)))
		return int(val)
	}
	if val, ok := data["quantity"].(int); ok {
		ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Extracted quantity as int: %d", val))
		return val
	}

	// Try to parse from string
	if val, ok := data["qty"].(string); ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Parsed qty from string: %s -> %d", val, parsed))
			return parsed
		}
	}
	if val, ok := data["quantity"].(string); ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			ap.LogInfo(fmt.Sprintf("getQuantityFromOrderData - Parsed quantity from string: %s -> %d", val, parsed))
			return parsed
		}
	}

	// Default to 1 for multi-leg orders
	ap.LogInfo("getQuantityFromOrderData - No qty found, defaulting to 1")
	return 1
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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
	datePart := optionPart[:6]   // YYMMDD
	optionType := optionPart[6]  // C or P
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

// === Market Data Methods - Options Chain ===

// GetOptionsChainBasic gets basic options chain (no Greeks) for fast loading, ATM-focused.
// Exact conversion of Python get_options_chain_basic method.
func (ap *AlpacaProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	// Use underlying_symbol if provided, otherwise use symbol
	apiSymbol := symbol
	if underlyingSymbol != nil && *underlyingSymbol != "" {
		apiSymbol = *underlyingSymbol
	}

	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/options/contracts", baseURL)
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	params := map[string]string{
		"underlying_symbols": apiSymbol,
		"expiration_date":    expiry,
		"root_symbol":        symbol,
		"limit":              "1000",
	}

	ap.LogInfo(fmt.Sprintf("GetOptionsChainBasic - URL: %s, Params: %v", url, params))

	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError(fmt.Sprintf("get_options_chain_basic API call for %s %s", symbol, expiry), err)
		return nil, err
	}

	ap.LogInfo(fmt.Sprintf("GetOptionsChainBasic - Response status: %d, Body length: %d bytes", resp.StatusCode, len(resp.Body)))

	var response struct {
		OptionContracts []OptionContractResponse `json:"option_contracts"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal options chain response", err)
		ap.LogInfo(fmt.Sprintf("Failed to unmarshal response body: %s", string(resp.Body[:min(500, len(resp.Body))])))
		return nil, err
	}

	ap.LogInfo(fmt.Sprintf("GetOptionsChainBasic - Received %d option contracts from API", len(response.OptionContracts)))

	// First pass: transform contracts and identify missing quotes
	var allContracts []*models.OptionContract
	var symbolsNeedingQuotes []string

	ap.LogInfo(fmt.Sprintf("Starting transformation of %d contracts", len(response.OptionContracts)))

	for i, contract := range response.OptionContracts {
		optionContract := ap.transformOptionContract(contract)
		if optionContract != nil {
			allContracts = append(allContracts, optionContract)

			// Check if bid/ask are missing
			if (optionContract.Bid == nil || optionContract.Ask == nil) && optionContract.Symbol != "" {
				symbolsNeedingQuotes = append(symbolsNeedingQuotes, optionContract.Symbol)
			}
		} else {
			if i < 3 {
				ap.LogInfo(fmt.Sprintf("Failed to transform contract %d: %+v", i, contract))
			}
		}
		if i == 0 {
			ap.LogInfo(fmt.Sprintf("First contract transformed: %+v", optionContract))
		}
	}

	// Second pass: fetch latest quotes for contracts with missing bid/ask (only when market is closed)
	if len(symbolsNeedingQuotes) > 0 {
		marketOpen, err := ap.IsMarketOpen(ctx)
		if err == nil && !marketOpen {
			// Market is closed - fetch latest quotes
			ap.LogInfo(fmt.Sprintf("Market closed - fetching latest quotes for %d option contracts", len(symbolsNeedingQuotes)))
			latestQuotes, err := ap.GetLatestOptionQuotes(ctx, symbolsNeedingQuotes)
			if err == nil {
				updatedCount := 0
				for _, contract := range allContracts {
					if quoteData, exists := latestQuotes[contract.Symbol]; exists {
						if contract.Bid == nil {
							if bid, ok := quoteData["bid"].(*float64); ok {
								contract.Bid = bid
								updatedCount++
							}
						}
						if contract.Ask == nil {
							if ask, ok := quoteData["ask"].(*float64); ok {
								contract.Ask = ask
							}
						}
					}
				}
				ap.LogInfo(fmt.Sprintf("Updated %d contracts with latest quotes", updatedCount))
			}
		} else if err == nil {
			ap.LogInfo(fmt.Sprintf("Market open - skipping quote fetch for %d contracts (relying on live stream)", len(symbolsNeedingQuotes)))
		}
	}

	if len(allContracts) == 0 {
		return allContracts, nil
	}

	// If no underlying price provided, try to get current stock quote
	if underlyingPrice == nil {
		stockQuote, err := ap.GetStockQuote(ctx, symbol)
		if err == nil && stockQuote != nil && stockQuote.Bid != nil && stockQuote.Ask != nil {
			avgPrice := (*stockQuote.Bid + *stockQuote.Ask) / 2
			underlyingPrice = &avgPrice
		} else {
			// Fallback: use middle strike as approximation
			var strikes []float64
			for _, c := range allContracts {
				strikes = append(strikes, c.StrikePrice)
			}
			if len(strikes) > 0 {
				// Sort strikes
				for i := 0; i < len(strikes)-1; i++ {
					for j := i + 1; j < len(strikes); j++ {
						if strikes[i] > strikes[j] {
							strikes[i], strikes[j] = strikes[j], strikes[i]
						}
					}
				}
				middlePrice := strikes[len(strikes)/2]
				underlyingPrice = &middlePrice
			} else {
				// Return all contracts if we can't determine ATM
				return allContracts, nil
			}
		}
	}

	// Get all unique strikes and sort them
	strikeMap := make(map[float64]bool)
	for _, c := range allContracts {
		strikeMap[c.StrikePrice] = true
	}

	var strikes []float64
	for strike := range strikeMap {
		strikes = append(strikes, strike)
	}

	// Sort strikes
	for i := 0; i < len(strikes)-1; i++ {
		for j := i + 1; j < len(strikes); j++ {
			if strikes[i] > strikes[j] {
				strikes[i], strikes[j] = strikes[j], strikes[i]
			}
		}
	}

	// Find the ATM strike (closest to underlying price)
	minDiff := abs(*underlyingPrice - strikes[0])
	atmIndex := 0

	for i, strike := range strikes {
		diff := abs(*underlyingPrice - strike)
		if diff < minDiff {
			minDiff = diff
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
	var result []*models.OptionContract
	for _, contract := range allContracts {
		if selectedStrikes[contract.StrikePrice] {
			result = append(result, contract)
		}
	}

	ap.LogInfo(fmt.Sprintf("Basic options chain for %s %s: %d contracts around ATM $%.2f", symbol, expiry, len(result), *underlyingPrice))
	return result, nil
}

// GetOptionsGreeksBatch gets Greeks for multiple option symbols in batch using REST API.
// Exact conversion of Python get_options_greeks_batch method.
func (ap *AlpacaProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	if len(optionSymbols) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	url := fmt.Sprintf("%s/v1beta1/options/snapshots", ap.DataURL)
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	// Process in batches of 100 (API limit)
	batchSize := 100
	allGreeks := make(map[string]map[string]interface{})

	for i := 0; i < len(optionSymbols); i += batchSize {
		end := min(i+batchSize, len(optionSymbols))
		batchSymbols := optionSymbols[i:end]
		symbolsParam := strings.Join(batchSymbols, ",")

		params := map[string]string{
			"symbols": symbolsParam,
		}

		resp, err := ap.httpClient.Get(ctx, url, headers, params)
		if err != nil {
			ap.LogError("get_options_greeks_batch API call", err)
			continue
		}

		var response struct {
			Snapshots map[string]struct {
				Greeks struct {
					Delta *float64 `json:"delta"`
					Gamma *float64 `json:"gamma"`
					Theta *float64 `json:"theta"`
					Vega  *float64 `json:"vega"`
				} `json:"greeks"`
				ImpliedVolatility *float64 `json:"impliedVolatility"`
			} `json:"snapshots"`
		}

		if err := json.Unmarshal(resp.Body, &response); err != nil {
			ap.LogError("unmarshal greeks batch response", err)
			continue
		}

		for symbol, snapshot := range response.Snapshots {
			greeksData := make(map[string]interface{})

			if snapshot.Greeks.Delta != nil {
				greeksData["delta"] = *snapshot.Greeks.Delta
			} else {
				greeksData["delta"] = nil
			}

			if snapshot.Greeks.Gamma != nil {
				greeksData["gamma"] = *snapshot.Greeks.Gamma
			} else {
				greeksData["gamma"] = nil
			}

			if snapshot.Greeks.Theta != nil {
				greeksData["theta"] = *snapshot.Greeks.Theta
			} else {
				greeksData["theta"] = nil
			}

			if snapshot.Greeks.Vega != nil {
				greeksData["vega"] = *snapshot.Greeks.Vega
			} else {
				greeksData["vega"] = nil
			}

			if snapshot.ImpliedVolatility != nil {
				greeksData["implied_volatility"] = *snapshot.ImpliedVolatility
			} else {
				greeksData["implied_volatility"] = nil
			}

			allGreeks[symbol] = greeksData
		}
	}

	ap.LogInfo(fmt.Sprintf("Retrieved Greeks for %d option symbols", len(allGreeks)))
	return allGreeks, nil
}

// GetStreamingGreeksBatch gets Greeks for multiple option symbols using streaming (new method following TastyTrade pattern).
// This is a new method not in Python, following the TastyTrade streaming pattern.
func (ap *AlpacaProvider) GetStreamingGreeksBatch(ctx context.Context, optionSymbols []string, timeout int) (map[string]map[string]interface{}, error) {
	// TODO: Implement streaming-based Greeks fetching following TastyTrade pattern
	// For now, fall back to REST API
	ap.LogInfo("GetStreamingGreeksBatch not yet implemented, falling back to REST API")
	return ap.GetOptionsGreeksBatch(ctx, optionSymbols)
}

func (ap *AlpacaProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/calendar", baseURL)
	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	today := time.Now().Format("2006-01-02")
	nextYear := time.Now().AddDate(1, 0, 0).Format("2006-01-02")

	params := map[string]string{
		"start": today,
		"end":   nextYear,
	}

	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError("get_next_market_date API call", err)
		return today, nil // Fallback to today
	}

	var response []struct {
		Date string `json:"date"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal next market date response", err)
		return today, nil
	}

	// Find first date >= today
	for _, day := range response {
		if day.Date >= today {
			return day.Date, nil
		}
	}

	// Fallback to today
	return today, nil
}

func (ap *AlpacaProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ap *AlpacaProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	// Use IEX data feed instead of SIP to avoid subscription issues
	// The Python code uses StockHistoricalDataClient which handles this automatically
	// We need to use the IEX endpoint: /v2/stocks/{symbol}/bars with feed=iex parameter

	// Map timeframe to Alpaca format
	timeframeMap := map[string]string{
		"1m":  "1Min",
		"5m":  "5Min",
		"15m": "15Min",
		"30m": "30Min",
		"1h":  "1Hour",
		"4h":  "4Hour",
		"D":   "1Day",
		"W":   "1Week",
		"M":   "1Month",
	}

	alpacaTimeframe, ok := timeframeMap[timeframe]
	if !ok {
		alpacaTimeframe = "1Day" // Default to daily
	}

	// Build URL - use DataURL for market data with IEX feed
	url := fmt.Sprintf("%s/v2/stocks/%s/bars", ap.DataURL, symbol)

	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	// Set default start date if not provided
	now := time.Now()
	var start string
	if startDate != nil && *startDate != "" {
		start = *startDate
	} else {
		// Default based on timeframe
		if timeframe == "1m" || timeframe == "5m" || timeframe == "15m" || timeframe == "30m" || timeframe == "1h" || timeframe == "4h" {
			// For intraday, get last 5 days
			start = now.AddDate(0, 0, -5).Format("2006-01-02")
		} else {
			// For daily+, get last year
			start = now.AddDate(-1, 0, 0).Format("2006-01-02")
		}
	}

	params := map[string]string{
		"timeframe":  alpacaTimeframe,
		"start":      start,
		"limit":      fmt.Sprintf("%d", limit),
		"adjustment": "raw",
		"feed":       "iex", // Use IEX feed to avoid SIP subscription requirement
	}

	// Only add end date if provided
	if endDate != nil && *endDate != "" {
		params["end"] = *endDate
	}

	ap.LogInfo(fmt.Sprintf("Requesting Alpaca bars for %s, timeframe: %s, start: %s, limit: %d (using IEX feed)", symbol, timeframe, start, limit))

	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError(fmt.Sprintf("get_historical_bars for %s %s", symbol, timeframe), err)
		return nil, err
	}

	var response struct {
		Bars []struct {
			Timestamp string  `json:"t"`
			Open      float64 `json:"o"`
			High      float64 `json:"h"`
			Low       float64 `json:"l"`
			Close     float64 `json:"c"`
			Volume    int64   `json:"v"`
		} `json:"bars"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		ap.LogError("unmarshal historical bars response", err)
		return nil, err
	}

	// Transform to Lightweight Charts format
	result := make([]map[string]interface{}, 0, len(response.Bars))

	for _, bar := range response.Bars {
		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, bar.Timestamp)
		if err != nil {
			ap.LogError(fmt.Sprintf("parse timestamp %s", bar.Timestamp), err)
			continue
		}

		// Format time based on timeframe
		var timeStr string
		if timeframe == "1m" || timeframe == "5m" || timeframe == "15m" || timeframe == "30m" || timeframe == "1h" || timeframe == "4h" {
			// Intraday - include time
			timeStr = timestamp.Format("2006-01-02 15:04")
		} else {
			// Daily+ - date only
			timeStr = timestamp.Format("2006-01-02")
		}

		result = append(result, map[string]interface{}{
			"time":   timeStr,
			"open":   bar.Open,
			"high":   bar.High,
			"low":    bar.Low,
			"close":  bar.Close,
			"volume": bar.Volume,
		})
	}

	ap.LogInfo(fmt.Sprintf("Transformed %d bars for %s (%s)", len(result), symbol, timeframe))
	return result, nil
}

func (ap *AlpacaProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/positions", baseURL)

	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	ap.LogInfo(fmt.Sprintf("GetPositions - BaseURL: %s, Cleaned: %s, Full URL: %s", ap.BaseURL, baseURL, url))

	resp, err := ap.httpClient.Get(ctx, url, headers, nil)
	if err != nil {
		ap.LogError("get_positions API call", err)
		return nil, err
	}

	var positions []struct {
		Symbol         string  `json:"symbol"`
		Qty            string  `json:"qty"`
		MarketValue    string  `json:"market_value"`
		CostBasis      string  `json:"cost_basis"`
		UnrealizedPL   string  `json:"unrealized_pl"`
		UnrealizedPLPC string  `json:"unrealized_plpc"`
		CurrentPrice   string  `json:"current_price"`
		AvgEntryPrice  string  `json:"avg_entry_price"`
		AssetClass     string  `json:"asset_class"`
		LastdayPrice   *string `json:"lastday_price"`
	}

	if err := json.Unmarshal(resp.Body, &positions); err != nil {
		ap.LogError("unmarshal positions response", err)
		return nil, err
	}

	var result []*models.Position
	for _, pos := range positions {
		// Parse numeric values
		qty, _ := strconv.ParseFloat(pos.Qty, 64)
		marketValue, _ := strconv.ParseFloat(pos.MarketValue, 64)
		costBasis, _ := strconv.ParseFloat(pos.CostBasis, 64)
		unrealizedPL, _ := strconv.ParseFloat(pos.UnrealizedPL, 64)
		currentPrice, _ := strconv.ParseFloat(pos.CurrentPrice, 64)
		avgEntryPrice, _ := strconv.ParseFloat(pos.AvgEntryPrice, 64)

		// Parse unrealized P/L percentage
		var unrealizedPLPC *float64
		if pos.UnrealizedPLPC != "" {
			if val, err := strconv.ParseFloat(pos.UnrealizedPLPC, 64); err == nil {
				unrealizedPLPC = &val
			}
		}

		// Parse lastday price (if 0, set to nil as it indicates same-day trade)
		var lastdayPrice *float64
		if pos.LastdayPrice != nil && *pos.LastdayPrice != "" {
			if val, err := strconv.ParseFloat(*pos.LastdayPrice, 64); err == nil && val != 0 {
				lastdayPrice = &val
			}
		}

		// Determine side
		side := "long"
		if qty < 0 {
			side = "short"
		}

		position := &models.Position{
			Symbol:         pos.Symbol,
			Qty:            qty,
			Side:           side,
			MarketValue:    marketValue,
			CostBasis:      costBasis,
			UnrealizedPL:   unrealizedPL,
			UnrealizedPLPC: unrealizedPLPC,
			CurrentPrice:   currentPrice,
			AvgEntryPrice:  avgEntryPrice,
			AssetClass:     pos.AssetClass,
			LastdayPrice:   lastdayPrice,
			DateAcquired:   nil,
		}

		// Parse option-specific information if it's an option
		if pos.AssetClass == "us_option" {
			optionInfo := ap.parseOptionSymbol(pos.Symbol)
			if optionInfo != nil {
				if underlying, ok := optionInfo["underlying"].(string); ok {
					position.UnderlyingSymbol = &underlying
				}
				if optType, ok := optionInfo["type"].(string); ok {
					position.OptionType = &optType
				}
				if strike, ok := optionInfo["strike"].(float64); ok {
					position.StrikePrice = &strike
				}
				if expiry, ok := optionInfo["expiry"].(string); ok {
					position.ExpiryDate = &expiry
				}
			}
		}

		result = append(result, position)
	}

	return result, nil
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
	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/orders", baseURL)

	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	params := map[string]string{}

	// Map status filter
	switch strings.ToLower(status) {
	case "open":
		params["status"] = "open"
	case "closed":
		params["status"] = "closed"
	case "filled":
		params["status"] = "filled"
	case "cancelled", "canceled":
		params["status"] = "canceled"
	case "all":
		params["status"] = "all"
	default:
		if status != "" {
			params["status"] = status
		}
	}

	resp, err := ap.httpClient.Get(ctx, url, headers, params)
	if err != nil {
		ap.LogError(fmt.Sprintf("get_orders with status %s", status), err)
		return nil, err
	}

	var orders []struct {
		ID             string  `json:"id"`
		Symbol         string  `json:"symbol"`
		AssetClass     string  `json:"asset_class"`
		Side           string  `json:"side"`
		OrderType      string  `json:"order_type"`
		Qty            *string `json:"qty"`
		FilledQty      *string `json:"filled_qty"`
		LimitPrice     *string `json:"limit_price"`
		StopPrice      *string `json:"stop_price"`
		FilledAvgPrice *string `json:"filled_avg_price"` // Alpaca uses filled_avg_price
		Status         string  `json:"status"`
		TimeInForce    string  `json:"time_in_force"`
		SubmittedAt    string  `json:"submitted_at"`
		FilledAt       *string `json:"filled_at"`
		Legs           []struct {
			Symbol   string `json:"symbol"`
			Side     string `json:"side"`
			Qty      string `json:"qty"`
			RatioQty string `json:"ratio_qty"`
		} `json:"legs"`
	}

	if err := json.Unmarshal(resp.Body, &orders); err != nil {
		ap.LogError("unmarshal orders response", err)
		return nil, err
	}

	var result []*models.Order
	for _, order := range orders {
		// Use transformOrderFromResponse to properly handle multi-leg orders
		transformedOrder := ap.transformOrderFromResponse(order)
		if transformedOrder != nil {
			result = append(result, transformedOrder)
		}
	}

	return result, nil
}

func (ap *AlpacaProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	// The SDK client was initialized with BaseURL that may have /v2 suffix
	// We need to reinitialize it with the correct BaseURL
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	ap.LogInfo(fmt.Sprintf("GetAccount - BaseURL: %s, Cleaned: %s", ap.BaseURL, baseURL))

	// Reinitialize trading client with correct BaseURL
	ap.tradingClient = alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    ap.APIKey,
		APISecret: ap.APISecret,
		BaseURL:   baseURL,
	})

	// Use the Alpaca SDK's trading client
	account, err := ap.tradingClient.GetAccount()
	if err != nil {
		ap.LogError(fmt.Sprintf("get_account API call (BaseURL: %s)", baseURL), err)
		return nil, err
	}

	// Transform SDK account object to our model
	accountNumber := account.AccountNumber
	status := string(account.Status)
	currency := account.Currency
	if currency == "" {
		currency = "USD"
	}
	createdAt := account.CreatedAt.Format(time.RFC3339)
	multiplierStr := account.Multiplier.String()

	// Convert decimal values to float64 pointers
	buyingPower := account.BuyingPower.InexactFloat64()
	cash := account.Cash.InexactFloat64()
	portfolioValue := account.PortfolioValue.InexactFloat64()
	equity := account.Equity.InexactFloat64()
	dayTradingBuyingPower := account.DaytradingBuyingPower.InexactFloat64()
	regtBuyingPower := account.RegTBuyingPower.InexactFloat64() // Correct field name
	longMarketValue := account.LongMarketValue.InexactFloat64()
	shortMarketValue := account.ShortMarketValue.InexactFloat64()
	initialMargin := account.InitialMargin.InexactFloat64()
	maintenanceMargin := account.MaintenanceMargin.InexactFloat64()
	daytradeCount := int(account.DaytradeCount)

	// These fields don't exist in Alpaca SDK, set to nil
	var optionsBuyingPower *float64
	var optionsApprovedLevel *int
	var optionsTradingLevel *int

	return &models.Account{
		AccountID:             account.ID,
		AccountNumber:         &accountNumber,
		Status:                status,
		Currency:              currency,
		BuyingPower:           &buyingPower,
		Cash:                  &cash,
		PortfolioValue:        &portfolioValue,
		Equity:                &equity,
		DayTradingBuyingPower: &dayTradingBuyingPower,
		RegtBuyingPower:       &regtBuyingPower,
		OptionsBuyingPower:    optionsBuyingPower,
		PatternDayTrader:      &account.PatternDayTrader,
		TradingBlocked:        &account.TradingBlocked,
		TransfersBlocked:      &account.TransfersBlocked,
		AccountBlocked:        &account.AccountBlocked,
		CreatedAt:             &createdAt,
		Multiplier:            &multiplierStr,
		LongMarketValue:       &longMarketValue,
		ShortMarketValue:      &shortMarketValue,
		InitialMargin:         &initialMargin,
		MaintenanceMargin:     &maintenanceMargin,
		DaytradeCount:         &daytradeCount,
		OptionsApprovedLevel:  optionsApprovedLevel,
		OptionsTradingLevel:   optionsTradingLevel,
	}, nil
}

func (ap *AlpacaProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	// Exact conversion of Python place_order method
	orderType := orderData["order_type"].(string)
	side := orderData["side"].(string)

	// Extract side prefix (e.g., "buy_to_open" -> "buy")
	sideParts := strings.Split(side, "_")
	sidePrefix := sideParts[0]

	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/orders", baseURL)

	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
		"content-type":        "application/json",
	}

	// Build request body based on order type
	requestBody := map[string]interface{}{
		"symbol":        orderData["symbol"],
		"qty":           orderData["qty"],
		"side":          sidePrefix,
		"time_in_force": orderData["time_in_force"],
		"type":          orderType,
	}

	// Add limit price for limit orders
	if orderType == "limit" {
		if limitPrice, ok := orderData["limit_price"]; ok {
			requestBody["limit_price"] = limitPrice
		}
	}

	// Make API call - Post expects interface{}, not []byte
	resp, err := ap.httpClient.Post(ctx, url, requestBody, headers)
	if err != nil {
		ap.LogError("place_order API call", err)
		return nil, err
	}

	// Parse response
	var orderResponse struct {
		ID             string  `json:"id"`
		Symbol         string  `json:"symbol"`
		AssetClass     string  `json:"asset_class"`
		Side           string  `json:"side"`
		OrderType      string  `json:"order_type"`
		Qty            *string `json:"qty"`
		FilledQty      *string `json:"filled_qty"`
		LimitPrice     *string `json:"limit_price"`
		StopPrice      *string `json:"stop_price"`
		FilledAvgPrice *string `json:"filled_avg_price"`
		Status         string  `json:"status"`
		TimeInForce    string  `json:"time_in_force"`
		SubmittedAt    string  `json:"submitted_at"`
		FilledAt       *string `json:"filled_at"`
	}

	if err := json.Unmarshal(resp.Body, &orderResponse); err != nil {
		ap.LogError("unmarshal place_order response", err)
		return nil, err
	}

	// Transform to our model
	return ap.transformOrderFromResponse(orderResponse), nil
}

func (ap *AlpacaProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	// Exact conversion of Python place_multi_leg_order method
	orderType := orderData["order_type"].(string)

	// DEBUG: Log incoming order data
	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Incoming orderData: %+v", orderData))

	// Remove any trailing /v2 from BaseURL to avoid duplication
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/orders", baseURL)

	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
		"content-type":        "application/json",
	}

	// Build legs array
	legs := orderData["legs"].([]interface{})
	orderLegs := make([]map[string]interface{}, 0, len(legs))

	// Extract the quantity from the first leg (all legs should have the same qty for a standard multi-leg order)
	var legQty int = 1
	if len(legs) > 0 {
		firstLeg := legs[0].(map[string]interface{})
		if qtyVal, ok := firstLeg["qty"]; ok {
			switch v := qtyVal.(type) {
			case float64:
				legQty = int(v)
			case int:
				legQty = v
			case string:
				if parsed, err := strconv.Atoi(v); err == nil {
					legQty = parsed
				}
			}
		}
	}

	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Extracted qty from legs: %d", legQty))

	for _, leg := range legs {
		legMap := leg.(map[string]interface{})
		side := legMap["side"].(string)

		// Extract side prefix (e.g., "buy_to_open" -> "buy")
		sideParts := strings.Split(side, "_")
		sidePrefix := sideParts[0]

		// IMPORTANT: For Alpaca, ratio_qty must be relatively prime (GCD = 1)
		// So we set ratio_qty to 1 for each leg, and use the actual quantity in the order qty field
		orderLegs = append(orderLegs, map[string]interface{}{
			"symbol":    legMap["symbol"],
			"side":      sidePrefix,
			"ratio_qty": 1, // Always 1 for standard multi-leg orders (ratio between legs)
		})
	}

	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Using order qty: %d, ratio_qty: 1 for each leg", legQty))

	// Build request body
	requestBody := map[string]interface{}{
		"qty":           legQty, // Use the qty extracted from legs
		"order_class":   "mleg",
		"time_in_force": orderData["time_in_force"],
		"type":          orderType,
		"legs":          orderLegs,
	}

	// Add limit price for limit orders
	if orderType == "limit" {
		if limitPrice, ok := orderData["limit_price"]; ok {
			requestBody["limit_price"] = limitPrice
		}
	}

	// DEBUG: Log the full request body being sent to Alpaca
	requestBodyJSON, _ := json.MarshalIndent(requestBody, "", "  ")
	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Request body to Alpaca:\n%s", string(requestBodyJSON)))

	// Make API call - Post expects interface{}, not []byte
	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - About to call Alpaca API at %s", url))
	resp, err := ap.httpClient.Post(ctx, url, requestBody, headers)
	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - API call completed, err=%v, resp=%+v", err, resp))
	if err != nil {
		ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Error occurred, type: %T, value: %v", err, err))
		// Check if this is an HTTPError with response details
		if httpErr, ok := err.(*utils.HTTPError); ok {
			ap.LogError("place_multi_leg_order API call", err)
			ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Error is HTTPError, StatusCode: %d, Message: %s", httpErr.StatusCode, httpErr.Message))
			if httpErr.Response != nil && len(httpErr.Response.Body) > 0 {
				ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Alpaca API error response (HTTP %d): %s", httpErr.StatusCode, string(httpErr.Response.Body)))
			} else {
				ap.LogInfo("PlaceMultiLegOrder - HTTPError has no response body")
			}
			return nil, fmt.Errorf("alpaca API error (HTTP %d): %s", httpErr.StatusCode, httpErr.Message)
		}
		ap.LogInfo("PlaceMultiLegOrder - Error is NOT HTTPError")
		ap.LogError("place_multi_leg_order API call", err)
		return nil, fmt.Errorf("alpaca API error: %w", err)
	}

	// Parse response - Alpaca returns legs with ratio_qty field
	var orderResponse struct {
		ID             string  `json:"id"`
		Symbol         string  `json:"symbol"`
		AssetClass     string  `json:"asset_class"`
		Side           string  `json:"side"`
		OrderType      string  `json:"order_type"`
		Qty            *string `json:"qty"`
		FilledQty      *string `json:"filled_qty"`
		LimitPrice     *string `json:"limit_price"`
		StopPrice      *string `json:"stop_price"`
		FilledAvgPrice *string `json:"filled_avg_price"`
		Status         string  `json:"status"`
		TimeInForce    string  `json:"time_in_force"`
		SubmittedAt    string  `json:"submitted_at"`
		FilledAt       *string `json:"filled_at"`
		Legs           []struct {
			Symbol   string `json:"symbol"`
			Side     string `json:"side"`
			Qty      string `json:"qty"`
			RatioQty string `json:"ratio_qty"` // Alpaca uses ratio_qty in response
		} `json:"legs"`
	}

	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - About to unmarshal response body (length: %d)", len(resp.Body)))
	if err := json.Unmarshal(resp.Body, &orderResponse); err != nil {
		ap.LogError("unmarshal place_multi_leg_order response", err)
		ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Failed response body: %s", string(resp.Body[:min(500, len(resp.Body))])))
		return nil, err
	}
	ap.LogInfo(fmt.Sprintf("PlaceMultiLegOrder - Successfully unmarshaled response, order ID: %s, status: %s, legs: %d", orderResponse.ID, orderResponse.Status, len(orderResponse.Legs)))

	// Transform to our model
	return ap.transformOrderFromResponse(orderResponse), nil
}

func (ap *AlpacaProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	// Alpaca provider preview order stub implementation
	// Exact conversion of Python preview_order method stub
	// Alpaca does not provide an order preview endpoint, so we return a stub response
	return map[string]interface{}{
		"status":                "ok",
		"preview_not_available": true,
		"validation_errors":     []string{},
		"commission":            0.0,
		"cost":                  0.0,
		"fees":                  0.0,
		"order_cost":            0.0,
		"margin_change":         0.0,
		"buying_power_effect":   0.0,
		"day_trades":            0,
		"estimated_total":       0.0,
	}, nil
}

func (ap *AlpacaProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	// Exact conversion of Python cancel_order method
	baseURL := strings.TrimSuffix(ap.BaseURL, "/v2")
	url := fmt.Sprintf("%s/v2/orders/%s", baseURL, orderID)

	headers := map[string]string{
		"APCA-API-KEY-ID":     ap.APIKey,
		"APCA-API-SECRET-KEY": ap.APISecret,
		"accept":              "application/json",
	}

	resp, err := ap.httpClient.Delete(ctx, url, headers)
	if err != nil {
		ap.LogError(fmt.Sprintf("cancel_order %s", orderID), err)
		return false, err
	}

	// Check if the response indicates success (204 No Content or 200 OK)
	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		return true, nil
	}

	return false, fmt.Errorf("failed to cancel order: HTTP %d", resp.StatusCode)
}

// transformOrderFromResponse transforms order response to our standard model
func (ap *AlpacaProvider) transformOrderFromResponse(orderResponse interface{}) *models.Order {
	ap.LogInfo(fmt.Sprintf("transformOrderFromResponse - Input type: %T", orderResponse))

	// Handle different response types
	var id, symbol, assetClass, side, orderType, status, timeInForce, submittedAt string
	var qty, filledQty *string
	var limitPrice, stopPrice, filledAvgPrice *string
	var filledAt *string
	var legs []struct {
		Symbol   string `json:"symbol"`
		Side     string `json:"side"`
		Qty      string `json:"qty"`
		RatioQty string `json:"ratio_qty"`
	}

	// Type switch to handle different response structures
	switch resp := orderResponse.(type) {
	case struct {
		ID             string  `json:"id"`
		Symbol         string  `json:"symbol"`
		AssetClass     string  `json:"asset_class"`
		Side           string  `json:"side"`
		OrderType      string  `json:"order_type"`
		Qty            *string `json:"qty"`
		FilledQty      *string `json:"filled_qty"`
		LimitPrice     *string `json:"limit_price"`
		StopPrice      *string `json:"stop_price"`
		FilledAvgPrice *string `json:"filled_avg_price"`
		Status         string  `json:"status"`
		TimeInForce    string  `json:"time_in_force"`
		SubmittedAt    string  `json:"submitted_at"`
		FilledAt       *string `json:"filled_at"`
		Legs           []struct {
			Symbol   string `json:"symbol"`
			Side     string `json:"side"`
			Qty      string `json:"qty"`
			RatioQty string `json:"ratio_qty"`
		} `json:"legs"`
	}:
		ap.LogInfo("transformOrderFromResponse - Matched expected struct type")
		id = resp.ID
		symbol = resp.Symbol
		assetClass = resp.AssetClass
		side = resp.Side
		orderType = resp.OrderType
		qty = resp.Qty
		filledQty = resp.FilledQty
		limitPrice = resp.LimitPrice
		stopPrice = resp.StopPrice
		filledAvgPrice = resp.FilledAvgPrice
		status = resp.Status
		timeInForce = resp.TimeInForce
		submittedAt = resp.SubmittedAt
		filledAt = resp.FilledAt
		legs = resp.Legs
	default:
		ap.LogInfo(fmt.Sprintf("transformOrderFromResponse - No matching type, returning nil for type: %T", orderResponse))
		return nil
	}

	// Parse quantities
	var qtyFloat, filledQtyFloat float64
	if qty != nil {
		qtyFloat, _ = strconv.ParseFloat(*qty, 64)
	}
	if filledQty != nil {
		filledQtyFloat, _ = strconv.ParseFloat(*filledQty, 64)
	}

	// Parse prices
	var limitPriceFloat, stopPriceFloat, avgFillPriceFloat *float64
	if limitPrice != nil && *limitPrice != "" {
		if price, err := strconv.ParseFloat(*limitPrice, 64); err == nil {
			limitPriceFloat = &price
		}
	}
	if stopPrice != nil && *stopPrice != "" {
		if price, err := strconv.ParseFloat(*stopPrice, 64); err == nil {
			stopPriceFloat = &price
		}
	}
	if filledAvgPrice != nil && *filledAvgPrice != "" {
		if price, err := strconv.ParseFloat(*filledAvgPrice, 64); err == nil {
			avgFillPriceFloat = &price
		}
	}

	// Transform legs if present
	var orderLegs []models.OrderLeg
	if len(legs) > 0 {
		orderLegs = make([]models.OrderLeg, 0, len(legs))
		for _, leg := range legs {
			legQty, _ := strconv.ParseFloat(leg.Qty, 64)
			orderLegs = append(orderLegs, models.OrderLeg{
				Symbol: leg.Symbol,
				Side:   leg.Side,
				Qty:    legQty,
			})
		}
	}

	// Use "Multi-leg" as symbol if it's a multi-leg order
	if symbol == "" && len(orderLegs) > 0 {
		symbol = "Multi-leg"
	}

	return &models.Order{
		ID:           id,
		Symbol:       symbol,
		AssetClass:   assetClass,
		Side:         side,
		OrderType:    orderType,
		Qty:          qtyFloat,
		FilledQty:    filledQtyFloat,
		LimitPrice:   limitPriceFloat,
		StopPrice:    stopPriceFloat,
		AvgFillPrice: avgFillPriceFloat,
		Status:       status,
		TimeInForce:  timeInForce,
		SubmittedAt:  submittedAt,
		FilledAt:     filledAt,
		Legs:         orderLegs,
	}
}

func (ap *AlpacaProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	ap.streamMutex.Lock()
	defer ap.streamMutex.Unlock()

	// Prevent multiple concurrent connection attempts
	if ap.connecting {
		ap.LogInfo("Connection attempt already in progress, waiting...")
		return ap.IsConnected, nil
	}

	ap.connecting = true
	defer func() { ap.connecting = false }()

	// Ensure clean state before connecting
	if ap.IsConnected {
		ap.LogInfo("Already connected, disconnecting first for clean reconnection")
		ap.disconnectStreamingInternal()
		time.Sleep(1 * time.Second)
	}

	ap.LogInfo("Initializing streaming connections...")

	// Initialize streaming clients (they will be started when subscribing)
	// Create option stream if not disabled
	if !ap.disableOptionStream {
		ap.optionStream = stream.NewOptionClient(
			marketdata.OPRA,
			stream.WithCredentials(ap.APIKey, ap.APISecret),
		)
	}

	// Create stock stream using IEX (not SIP) to match user's subscription
	ap.stockStream = stream.NewStocksClient(
		marketdata.IEX,
		stream.WithCredentials(ap.APIKey, ap.APISecret),
	)

	ap.IsConnected = true
	ap.LogInfo("Streaming connections initialized (will connect on subscription)")
	return true, nil
}

func (ap *AlpacaProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	ap.streamMutex.Lock()
	defer ap.streamMutex.Unlock()

	return ap.disconnectStreamingInternal(), nil
}

func (ap *AlpacaProvider) disconnectStreamingInternal() bool {
	// Note: Alpaca SDK clients don't have a Close method
	// They disconnect automatically when garbage collected
	ap.stockStream = nil
	ap.optionStream = nil

	ap.IsConnected = false
	ap.LogInfo("Streaming connection closed")
	return true
}

func (ap *AlpacaProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	// For Alpaca, we hit connection limits easily, so we should NOT use streaming for options chain data
	// Instead, just track the subscriptions without actually connecting to streams
	// The frontend will poll for updates via REST API instead

	ap.LogInfo(fmt.Sprintf("Alpaca: Tracking %d symbols (streaming disabled to avoid connection limits)", len(symbols)))

	// Just track the symbols without connecting to streams
	for _, symbol := range symbols {
		ap.subscribedSymbols[symbol] = true
	}

	return true, nil
}

func (ap *AlpacaProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	ap.LogInfo(fmt.Sprintf("Unsubscribing from %d symbols", len(symbols)))

	// Remove symbols from tracking
	for _, symbol := range symbols {
		delete(ap.subscribedSymbols, symbol)
	}

	// If no symbols remain, close all streams
	if len(ap.subscribedSymbols) == 0 {
		ap.LogInfo("No symbols remaining - closing all streams")
		return ap.DisconnectStreaming(ctx)
	}

	// Otherwise, only update tracking and do NOT attempt to reconnect or restart streams.
	// Connection lifecycle and subscription restoration are managed by the external StreamingHealthManager.
	ap.LogInfo(fmt.Sprintf("Updated subscription tracking; %d symbols remain", len(ap.subscribedSymbols)))
	return true, nil
}

// stockQuoteHandler handles stock quotes from the stream
func (ap *AlpacaProvider) stockQuoteHandler(quote stream.Quote) {
	defer func() {
		if r := recover(); r != nil {
			ap.LogInfo(fmt.Sprintf("Alpaca: Stock quote handler panic recovered: %v", r))
		}
	}()

	marketData := &models.MarketData{
		Symbol:    quote.Symbol,
		DataType:  "quote",
		Timestamp: quote.Timestamp.Format(time.RFC3339),
		Data: map[string]interface{}{
			"bid": quote.BidPrice,
			"ask": quote.AskPrice,
		},
	}

	// Send to queue (non-blocking)
	select {
	case ap.streamingQueue <- marketData:
		// Successfully queued
	default:
		// Queue full, skip this update
	}
}

// optionQuoteHandler handles option quotes from the stream
func (ap *AlpacaProvider) optionQuoteHandler(quote stream.OptionQuote) {
	defer func() {
		if r := recover(); r != nil {
			ap.LogInfo(fmt.Sprintf("Alpaca: Option quote handler panic recovered: %v", r))
		}
	}()

	marketData := &models.MarketData{
		Symbol:    quote.Symbol,
		DataType:  "quote",
		Timestamp: quote.Timestamp.Format(time.RFC3339),
		Data: map[string]interface{}{
			"bid": quote.BidPrice,
			"ask": quote.AskPrice,
		},
	}

	// Send to queue (non-blocking)
	select {
	case ap.streamingQueue <- marketData:
		// Successfully queued
	default:
		// Queue full, skip this update
	}
}

func (ap *AlpacaProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	ap.LogInfo(fmt.Sprintf("Testing Alpaca credentials (paper: %v)", ap.UsePaper))

	// Test credentials by getting account information
	account, err := ap.GetAccount(ctx)
	if err != nil {
		errorMsg := strings.ToLower(err.Error())

		// Categorize different types of errors
		if strings.Contains(errorMsg, "401") || strings.Contains(errorMsg, "unauthorized") || strings.Contains(errorMsg, "forbidden") {
			return map[string]interface{}{
				"success":        false,
				"message":        "Invalid API credentials. Please check your API key and secret.",
				"error_category": "authentication",
				"error_code":     "INVALID_CREDENTIALS",
				"details":        map[string]interface{}{"error": err.Error()},
			}, nil
		} else if strings.Contains(errorMsg, "403") {
			return map[string]interface{}{
				"success":        false,
				"message":        "Access forbidden. Please check your account permissions.",
				"error_category": "authorization",
				"error_code":     "ACCESS_FORBIDDEN",
				"details":        map[string]interface{}{"error": err.Error()},
			}, nil
		} else if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "connection") {
			return map[string]interface{}{
				"success":        false,
				"message":        "Connection timeout. Please check your network connection and try again.",
				"error_category": "network",
				"error_code":     "CONNECTION_TIMEOUT",
				"details":        map[string]interface{}{"error": err.Error()},
			}, nil
		} else if strings.Contains(errorMsg, "rate limit") || strings.Contains(errorMsg, "429") {
			return map[string]interface{}{
				"success":        false,
				"message":        "Rate limit exceeded. Please wait a moment and try again.",
				"error_category": "rate_limit",
				"error_code":     "RATE_LIMITED",
				"details":        map[string]interface{}{"error": err.Error()},
			}, nil
		}

		return map[string]interface{}{
			"success":        false,
			"message":        fmt.Sprintf("Connection test failed: %s", err.Error()),
			"error_category": "unknown",
			"error_code":     "UNKNOWN_ERROR",
			"details":        map[string]interface{}{"error": err.Error()},
		}, nil
	}

	accountType := "Paper Trading"
	if !ap.UsePaper {
		accountType = "Live Trading"
	}

	acctNum := "<nil>"
	if account.AccountNumber != nil {
		acctNum = *account.AccountNumber
	}
	ap.LogInfo(fmt.Sprintf("Alpaca credentials valid - Account: %s, Status: %s", acctNum, account.Status))

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Alpaca connection successful (%s)", accountType),
		"details": map[string]interface{}{
			"account_number":     account.AccountNumber,
			"account_status":     account.Status,
			"account_type":       accountType,
			"currency":           account.Currency,
			"buying_power":       account.BuyingPower,
			"equity":             account.Equity,
			"pattern_day_trader": account.PatternDayTrader,
			"trading_blocked":    account.TradingBlocked,
		},
	}, nil
}

// ======================== Order Event Streaming Methods ========================

// StartAccountStream starts the Alpaca trade updates stream
func (ap *AlpacaProvider) StartAccountStream(ctx context.Context) error {
	streamCtx, cancel := context.WithCancel(ctx)
	ap.ctxCancel = cancel

	go ap.streamTradeUpdates(streamCtx)

	ap.LogInfo("Alpaca order event stream started")
	return nil
}

func (ap *AlpacaProvider) streamTradeUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := ap.streamTradeUpdatesOnce(ctx); err != nil {
				ap.LogError("Trade updates stream error", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
			return
		}
	}
}

func (ap *AlpacaProvider) streamTradeUpdatesOnce(ctx context.Context) error {
	url := ap.BaseURL
	if ap.UsePaper {
		url = "https://paper-api.alpaca.markets"
	}
	url = strings.TrimSuffix(url, "/v2")
	wsURL := strings.Replace(url, "https://", "wss://", 1) + "/stream"

	header := http.Header{}
	header.Set("APCA-API-KEY-ID", ap.APIKey)
	header.Set("APCA-API-SECRET-KEY", ap.APISecret)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("failed to connect to Alpaca stream: %w", err)
	}
	defer conn.Close()

	// Subscribe to trade_updates stream (for order events)
	subscribeMsg := map[string]interface{}{
		"action": "listen",
		"data": map[string]interface{}{
			"streams": []string{"trade_updates"},
		},
	}
	if err := conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to subscribe to trade_updates: %w", err)
	}

	ap.LogInfo("Subscribed to Alpaca trade_updates stream")

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					return nil
				}
				return fmt.Errorf("read error: %w", err)
			}

			var message map[string]interface{}
			if err := json.Unmarshal(messageBytes, &message); err != nil {
				continue
			}

			// Handle trade_updates messages
			if stream, ok := message["stream"].(string); ok && stream == "trade_updates" {
				if data, ok := message["data"].(map[string]interface{}); ok {
					ap.processAlpacaTradeUpdate(data)
				} else {
					ap.LogInfo("Alpaca: No data in trade_update message")
				}
			}
		}
	}
}

func (ap *AlpacaProvider) processAlpacaTradeUpdate(data map[string]interface{}) {
	// Extract event type
	eventType, _ := data["event"].(string)

	// Extract order data
	orderData, ok := data["order"].(map[string]interface{})
	if !ok {
		ap.LogInfo("Alpaca: No order data in trade_update")
		return
	}

	orderEvent := ap.parseAlpacaOrderEvent(orderData, eventType)
	if orderEvent != nil {
		// Normalize the event based on status transitions
		normalizedEvent, shouldEmit := models.GetGlobalNormalizer().NormalizeEvent(orderEvent)

		if shouldEmit && normalizedEvent != "" {
			orderEvent.NormalizedEvent = normalizedEvent

			if ap.orderEventCallback != nil {
				ap.LogInfo(fmt.Sprintf("Alpaca: Emitting normalized event - orderID: %s, normalizedEvent: %s, status: %s",
					orderEvent.ID, normalizedEvent, orderEvent.Status))
				ap.orderEventCallback(orderEvent)
			}
		}
	}
}

func (ap *AlpacaProvider) parseAlpacaOrderEvent(orderData map[string]interface{}, eventType string) *models.OrderEvent {
	// Get order ID
	var id interface{}
	if idVal, ok := orderData["id"].(string); ok {
		id = idVal
	} else if idVal, ok := orderData["id"].(float64); ok {
		id = fmt.Sprintf("%.0f", idVal)
	}

	// Get symbol
	symbol, _ := orderData["symbol"].(string)

	// Get status from order data
	orderStatus, _ := orderData["status"].(string)

	// Map status to our standard format
	statusMap := map[string]string{
		"new":            "pending",
		"accepted":       "pending",
		"pending_new":    "pending",
		"open":           "open",
		"fill":           "filled",
		"filled":         "filled",
		"partial":        "partially_filled",
		"partial_fill":   "partially_filled",
		"cancel":         "canceled",
		"canceled":       "canceled",
		"pending_cancel": "canceled",
		"expire":         "expired",
		"expired":        "expired",
		"rejected":       "rejected",
		"done_for_day":   "expired",
		"stopped":        "expired",
		"suspended":      "expired",
	}

	// Use order status if available, otherwise derive from event type
	status := statusMap[orderStatus]
	if status == "" {
		status = statusMap[eventType]
	}
	if status == "" {
		// Fallback: derive from event type
		switch eventType {
		case "new", "accepted", "pending_new":
			status = "pending"
		case "fill":
			status = "filled"
		case "partial_fill":
			status = "partially_filled"
		case "cancel", "canceled":
			status = "canceled"
		case "expired":
			status = "expired"
		case "rejected":
			status = "rejected"
		default:
			status = "open"
		}
	}

	// Get quantities
	qtyStr, _ := orderData["qty"].(string)
	filledQtyStr, _ := orderData["filled_qty"].(string)
	qty, _ := strconv.ParseFloat(qtyStr, 64)
	filledQty, _ := strconv.ParseFloat(filledQtyStr, 64)
	remainingQty := qty - filledQty
	if remainingQty < 0 {
		remainingQty = 0
	}

	// Get price
	priceStr, _ := orderData["limit_price"].(string)
	price, _ := strconv.ParseFloat(priceStr, 64)

	// Get filled average price
	avgPriceStr, _ := orderData["filled_avg_price"].(string)
	avgPrice, _ := strconv.ParseFloat(avgPriceStr, 64)

	// Get timestamps
	var transactionDate, createDate string
	if filledAt, ok := orderData["filled_at"].(string); ok && filledAt != "" {
		transactionDate = filledAt
	} else if updatedAt, ok := orderData["updated_at"].(string); ok && updatedAt != "" {
		transactionDate = updatedAt
	} else if createdAt, ok := orderData["created_at"].(string); ok && createdAt != "" {
		transactionDate = createdAt
	}

	if createdAt, ok := orderData["created_at"].(string); ok {
		createDate = createdAt
	}

	// Get order type
	orderType, _ := orderData["type"].(string)

	// Get side
	side, _ := orderData["side"].(string)

	return &models.OrderEvent{
		ID:                id,
		Event:             "order",
		Status:            status,
		Type:              orderType,
		Symbol:            symbol,
		Side:              side,
		Quantity:          qty,
		ExecutedQuantity:  filledQty,
		RemainingQuantity: remainingQty,
		Price:             price,
		AvgFillPrice:      avgPrice,
		TransactionDate:   transactionDate,
		CreateDate:        createDate,
	}
}

// StopAccountStream stops the Alpaca trade updates stream
func (ap *AlpacaProvider) StopAccountStream() {
	if ap.ctxCancel != nil {
		ap.ctxCancel()
		ap.ctxCancel = nil
	}
	ap.LogInfo("Alpaca order event stream stopped")
}

// SetOrderEventCallback sets the callback for receiving order events
func (ap *AlpacaProvider) SetOrderEventCallback(cb func(*models.OrderEvent)) {
	ap.orderEventCallback = cb
}

// IsAccountStreamConnected checks if account stream is connected
func (ap *AlpacaProvider) IsAccountStreamConnected() bool {
	return ap.ctxCancel != nil
}
