package tradier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
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

// TradierProvider implements the Provider interface for Tradier Brokerage API.
// Exact conversion of Python TradierProvider class.
// StreamingTask manages a streaming task with proper cancellation and completion tracking
type StreamingTask struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// CachedMarketData stores the latest bid, ask, and last prices for a symbol
type CachedMarketData struct {
	Bid  interface{}
	Ask  interface{}
	Last interface{}
}

type TradierProvider struct {
	*base.BaseProviderImpl
	accountID            string
	apiKey               string
	baseURL              string
	streamURL            string
	accountType          string // "live" or "paper"
	client               *utils.HTTPClient
	sessionID            string
	streamConnection     *websocket.Conn
	connectionReady      chan struct{}
	subscribedSymbols    map[string]bool
	streamingQueue       chan *models.MarketData
	streamingCache       base.StreamingCache
	streamMutex          sync.RWMutex
	writeMutex           sync.Mutex
	streamCancel         context.CancelFunc
	IsConnected          bool
	streamingDisabled    bool
	lastConnectionError  time.Time
	connectionInProgress bool      // Prevent recursive connection attempts
	lastSubscriptionTime time.Time // Track recent subscriptions to prevent premature recovery
	recoveryInProgress   bool      // CRITICAL FIX: Track when recovery is happening to ignore old connection errors
	// Channel-based architecture like TastyTrade
	messageChan     chan map[string]interface{}
	errorChan       chan error
	streamingTask   *StreamingTask
	shutdownEvent   chan struct{}
	writeLock       sync.Mutex
	recvLock        sync.Mutex
	marketDataCache map[string]*CachedMarketData
	cacheMutex      sync.RWMutex

	// Account event streaming (trade_account service only)
	accountStream       *AccountStreamClient
	accountStreamURL    string
	accountStreamCancel context.CancelFunc
	orderEventCallback  func(*models.OrderEvent)
}

// NewTradierProvider creates a new Tradier provider instance.
// Exact conversion of Python TradierProvider.__init__ method.
func NewTradierProvider(accountID, apiKey, baseURL, streamURL, accountType string) *TradierProvider {
	return &TradierProvider{
		BaseProviderImpl:  base.NewBaseProvider("Tradier"),
		accountID:         accountID,
		apiKey:            apiKey,
		baseURL:           baseURL,
		streamURL:         streamURL,
		accountType:       accountType,
		client:            utils.NewHTTPClient(),
		connectionReady:   make(chan struct{}),
		subscribedSymbols: make(map[string]bool),
		streamingQueue:    make(chan *models.MarketData, 1000),    // Buffered channel like Python queue
		messageChan:       make(chan map[string]interface{}, 100), // Buffered to avoid blocking reader
		errorChan:         make(chan error, 1),
		shutdownEvent:     make(chan struct{}),
		marketDataCache:   make(map[string]*CachedMarketData),
	}
}

// IsPaperAccount returns true if this is a paper trading account
func (t *TradierProvider) IsPaperAccount() bool {
	return strings.Contains(strings.ToLower(t.accountType), "paper")
}

// TestCredentials tests Tradier credentials by making a real API call.
// Exact conversion of Python test_credentials method.
func (t *TradierProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	slog.Info("🔍 Testing Tradier credentials...")

	account, err := t.GetAccount(ctx)
	if err != nil {
		slog.Error("❌ Tradier credentials validation failed", "error", err)
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Authentication failed: %v", err),
		}, nil
	}

	if account != nil && account.AccountID != "" {
		slog.Info("✅ Tradier credentials are valid")
		return map[string]interface{}{
			"success": true,
			"message": "Successfully connected to Tradier.",
		}, nil
	}

	slog.Error("❌ Tradier credentials validation failed: No account info returned")
	return map[string]interface{}{
		"success": false,
		"message": "Invalid credentials or no account info found.",
	}, nil
}

// GetStockQuote gets a stock quote for a symbol.
// Exact conversion of Python get_stock_quote method.
func (t *TradierProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	quotes, err := t.GetStockQuotes(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}

	if quote, exists := quotes[symbol]; exists {
		return quote, nil
	}

	return nil, fmt.Errorf("quote not found for symbol %s", symbol)
}

// GetStockQuotes gets stock quotes for multiple symbols.
// Exact conversion of Python get_stock_quotes method.
func (t *TradierProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/quotes", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	data := make(map[string]string)
	data["symbols"] = strings.Join(symbols, ",")

	resp, err := t.client.PostForm(ctx, endpoint, headers, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock quotes: %w", err)
	}

	var response struct {
		Quotes struct {
			Quote interface{} `json:"quote"`
		} `json:"quotes"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("failed to parse quotes response: %w", err)
	}

	// Handle both single quote (object) and multiple quotes (array)
	var quotes []map[string]interface{}
	switch v := response.Quotes.Quote.(type) {
	case map[string]interface{}:
		quotes = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if quote, ok := item.(map[string]interface{}); ok {
				quotes = append(quotes, quote)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected quote format")
	}

	result := make(map[string]*models.StockQuote)
	for _, quote := range quotes {
		transformed := t.transformStockQuote(quote)
		if transformed != nil {
			result[transformed.Symbol] = transformed
		}
	}

	return result, nil
}

// transformStockQuote transforms Tradier stock quote to our standard model.
// Exact conversion of Python _transform_stock_quote method.
func (t *TradierProvider) transformStockQuote(rawQuote map[string]interface{}) *models.StockQuote {
	symbol, _ := rawQuote["symbol"].(string)

	var ask, bid, last *float64
	if askVal, ok := rawQuote["ask"].(float64); ok && askVal > 0 {
		ask = &askVal
	}
	if bidVal, ok := rawQuote["bid"].(float64); ok && bidVal > 0 {
		bid = &bidVal
	}
	if lastVal, ok := rawQuote["last"].(float64); ok && lastVal > 0 {
		last = &lastVal
	}

	timestamp := time.Now().Format(time.RFC3339)
	if tradeDate, ok := rawQuote["trade_date"].(float64); ok {
		timestamp = time.Unix(int64(tradeDate/1000), 0).Format(time.RFC3339)
	}

	return &models.StockQuote{
		Symbol:    symbol,
		Ask:       ask,
		Bid:       bid,
		Last:      last,
		Timestamp: timestamp,
	}
}

// GetExpirationDates gets available expiration dates for options on a symbol.
// Exact conversion of Python get_expiration_dates method.
func (t *TradierProvider) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/options/expirations", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	// Determine API symbol to use (handle SPXW -> SPX, NDXP -> NDX, etc.)
	apiSymbol := symbol
	// Check if the requested symbol is a known weekly variant
	// We need to reverse lookup in weeklyMap: if symbol is a value, use the key
	for underlying, weekly := range weeklyMap {
		if symbol == weekly {
			apiSymbol = underlying
			break
		}
	}

	params := make(map[string]string)
	params["symbol"] = apiSymbol
	params["includeAllRoots"] = "true"
	params["expirationType"] = "true"
	params["strikes"] = "true"
	params["contractSize"] = "true"

	resp, err := t.client.Get(ctx, endpoint, headers, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration dates: %w", err)
	}

	var response struct {
		Expirations struct {
			Expiration interface{} `json:"expiration"`
		} `json:"expirations"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse expirations response: %w", err)
	}

	// Handle both single expiration (object) and multiple expirations (array)
	var expirations []map[string]interface{}
	switch v := response.Expirations.Expiration.(type) {
	case map[string]interface{}:
		expirations = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if exp, ok := item.(map[string]interface{}); ok {
				expirations = append(expirations, exp)
			}
		}
	}

	// Extract unique dates and create enhanced structure
	dateSet := make(map[string]bool)
	for _, exp := range expirations {
		if date, ok := exp["date"].(string); ok {
			dateSet[date] = true
		}
	}

	var enhancedDates []map[string]interface{}
	for date := range dateSet {
		enhancedDates = append(enhancedDates, map[string]interface{}{
			"date":   date,
			"symbol": symbol,
			"type":   "unknown",
		})
	}

	// Sort expiration dates chronologically
	sort.Slice(enhancedDates, func(i, j int) bool {
		dateI, okI := enhancedDates[i]["date"].(string)
		dateJ, okJ := enhancedDates[j]["date"].(string)
		if !okI || !okJ {
			return false
		}
		return dateI < dateJ
	})

	return enhancedDates, nil
}

// GetOptionsChainBasic gets basic options chain data.
// Exact conversion of Python get_options_chain_basic method.
func (t *TradierProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	// If no expiry specified, fetch all expirations and combine results
	// This matches TastyTrade behavior for consistency
	if expiry == "" {
		slog.Info("Tradier: Fetching full chain (all expirations)", "symbol", symbol)

		// Get all expirations
		expirations, err := t.GetExpirationDates(ctx, symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get expirations: %w", err)
		}

		// Fetch chain for each expiration and combine
		var allContracts []*models.OptionContract
		for _, expData := range expirations {
			expDate, ok := expData["date"].(string)
			if !ok || expDate == "" {
				continue
			}

			// Recursive call with specific expiration (no strike filtering here)
			contracts, err := t.GetOptionsChainBasic(ctx, symbol, expDate, underlyingPrice, 0, optionType, underlyingSymbol)
			if err != nil {
				slog.Warn("Tradier: Failed to get chain for expiry", "symbol", symbol, "expiry", expDate, "error", err)
				continue
			}
			allContracts = append(allContracts, contracts...)
		}

		// Apply strike filtering to combined results if requested
		if underlyingPrice != nil && strikeCount > 0 && len(allContracts) > 0 {
			allContracts = t.filterByStrikeCount(allContracts, *underlyingPrice, strikeCount)
		}

		slog.Info("Tradier: Fetched full chain", "symbol", symbol, "contracts", len(allContracts), "expirations", len(expirations))
		return allContracts, nil
	}

	// Original implementation for specific expiry
	endpoint := fmt.Sprintf("%s/v1/markets/options/chains", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	// Determine API symbol to use (handle SPXW -> SPX, NDXP -> NDX, etc.)
	apiSymbol := symbol
	if underlyingSymbol != nil {
		apiSymbol = *underlyingSymbol
	} else {
		// Check if the requested symbol is a known weekly variant
		// We need to reverse lookup in weeklyMap: if symbol is a value, use the key
		for underlying, weekly := range weeklyMap {
			if symbol == weekly {
				apiSymbol = underlying
				break
			}
		}
	}

	params := make(map[string]string)
	params["symbol"] = apiSymbol
	params["expiration"] = expiry
	params["greeks"] = "false"
	params["includeAllRoots"] = "true" // Ensure we get all roots (e.g. SPXW for SPX)

	resp, err := t.client.Get(ctx, endpoint, headers, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get options chain: %w", err)
	}

	var response struct {
		Options struct {
			Option interface{} `json:"option"`
		} `json:"options"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse options chain response: %w", err)
	}

	// Handle both single option (object) and multiple options (array)
	var options []map[string]interface{}
	switch v := response.Options.Option.(type) {
	case map[string]interface{}:
		options = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if opt, ok := item.(map[string]interface{}); ok {
				options = append(options, opt)
			}
		}
	}

	// Filter contracts by root symbol
	var filteredOptions []map[string]interface{}
	upperSymbol := strings.ToUpper(symbol)

	for _, option := range options {
		optionSymbol, ok := option["symbol"].(string)
		if !ok || len(optionSymbol) < 15 {
			continue
		}

		// Extract root from option symbol (OCC format)
		root := strings.ToUpper(optionSymbol[:len(optionSymbol)-15])

		// Check for exact match
		if root == upperSymbol {
			filteredOptions = append(filteredOptions, option)
			continue
		}

		// Check weekly map: if requested symbol maps to a weekly root, accept that root
		if weeklyRoot, ok := weeklyMap[upperSymbol]; ok && root == weeklyRoot {
			filteredOptions = append(filteredOptions, option)
			continue
		}

		// Check reverse: if root is in weekly map and maps to requested symbol
		for indexSymbol, weeklyRoot := range weeklyMap {
			if root == indexSymbol && upperSymbol == weeklyRoot {
				filteredOptions = append(filteredOptions, option)
				break
			}
		}
	}
	// Diagnostic: if filtering removed everything, log raw response details to help debug root/format mismatches
	if len(filteredOptions) == 0 {
		var sampleSymbols []string
		for i, opt := range options {
			if i >= 10 {
				break
			}
			if osym, ok := opt["symbol"].(string); ok {
				root := ""
				if len(osym) >= 15 {
					root = strings.ToUpper(osym[:len(osym)-15])
				}
				sampleSymbols = append(sampleSymbols, fmt.Sprintf("%s(root=%s)", osym, root))
			}
		}
		slog.Warn("Tradier: Filtered options empty after root filtering", "requestedSymbol", symbol, "apiSymbol", apiSymbol, "expiry", expiry, "totalOptions", len(options), "sampleSymbols", sampleSymbols)
	}

	// Transform contracts
	var contracts []*models.OptionContract
	for _, option := range filteredOptions {
		contract := t.transformOptionContract(option)
		if contract != nil {
			contracts = append(contracts, contract)
		}
	}

	// Apply strike filtering if underlying price is provided
	if underlyingPrice != nil && len(contracts) > 0 {
		contracts = t.filterByStrikeCount(contracts, *underlyingPrice, strikeCount)
	} else {
		// Sort all contracts by strike price and option type for consistent ordering
		sort.Slice(contracts, func(i, j int) bool {
			if contracts[i].StrikePrice != contracts[j].StrikePrice {
				return contracts[i].StrikePrice < contracts[j].StrikePrice
			}
			// If strikes are equal, sort by option type (calls first)
			return contracts[i].Type == "call" && contracts[j].Type == "put"
		})
	}

	return contracts, nil
}

// transformOptionContract transforms Tradier option contract to our standard model.
// Exact conversion of Python _transform_option_contract_basic method.
func (t *TradierProvider) transformOptionContract(rawContract map[string]interface{}) *models.OptionContract {
	symbol, _ := rawContract["symbol"].(string)
	underlying, _ := rawContract["underlying"].(string)
	expirationDate, _ := rawContract["expiration_date"].(string)
	optionType, _ := rawContract["option_type"].(string)

	strike, _ := rawContract["strike"].(float64)

	var bid, ask, closePrice *float64
	var volume, openInterest *int

	if bidVal, ok := rawContract["bid"].(float64); ok && bidVal > 0 {
		bid = &bidVal
	}
	if askVal, ok := rawContract["ask"].(float64); ok && askVal > 0 {
		ask = &askVal
	}
	if closeVal, ok := rawContract["close"].(float64); ok && closeVal > 0 {
		closePrice = &closeVal
	}
	if volVal, ok := rawContract["volume"].(float64); ok {
		vol := int(volVal)
		volume = &vol
	}
	if oiVal, ok := rawContract["open_interest"].(float64); ok {
		oi := int(oiVal)
		openInterest = &oi
	}

	// Extract Greeks if present (when greeks=true is passed to API)
	var impliedVolatility, delta, gamma, theta, vega *float64
	if greeksData, ok := rawContract["greeks"].(map[string]interface{}); ok {
		// DEBUG: Log that we found Greeks data
		keys := make([]string, 0, len(greeksData))
		for k := range greeksData {
			keys = append(keys, k)
		}
		slog.Debug("Tradier: Found greeks object in contract", "symbol", symbol, "greeksKeys", keys)

		if midIV, ok := greeksData["mid_iv"].(float64); ok {
			impliedVolatility = &midIV
			slog.Debug("Tradier: Extracted mid_iv", "symbol", symbol, "iv", midIV)
		} else {
			slog.Debug("Tradier: mid_iv not found or wrong type", "symbol", symbol, "greeksData", greeksData)
		}
		if deltaVal, ok := greeksData["delta"].(float64); ok {
			delta = &deltaVal
		}
		if gammaVal, ok := greeksData["gamma"].(float64); ok {
			gamma = &gammaVal
		}
		if thetaVal, ok := greeksData["theta"].(float64); ok {
			theta = &thetaVal
		}
		if vegaVal, ok := greeksData["vega"].(float64); ok {
			vega = &vegaVal
		}
	} else {
		// DEBUG: Log that Greeks object was not found
		slog.Debug("Tradier: No greeks object in contract", "symbol", symbol, "hasGreeksKey", rawContract["greeks"] != nil)
	}

	return &models.OptionContract{
		Symbol:            symbol,
		UnderlyingSymbol:  underlying,
		ExpirationDate:    expirationDate,
		StrikePrice:       strike,
		Type:              strings.ToLower(optionType),
		Bid:               bid,
		Ask:               ask,
		ClosePrice:        closePrice,
		Volume:            volume,
		OpenInterest:      openInterest,
		ImpliedVolatility: impliedVolatility,
		Delta:             delta,
		Gamma:             gamma,
		Theta:             theta,
		Vega:              vega,
	}
}

// filterByStrikeCount filters options to focus on ATM strikes.
// Helper method for strike count filtering.
func (t *TradierProvider) filterByStrikeCount(contracts []*models.OptionContract, underlyingPrice float64, strikeCount int) []*models.OptionContract {
	if len(contracts) == 0 {
		return contracts
	}

	// Get unique strikes
	strikeSet := make(map[float64]bool)
	for _, contract := range contracts {
		strikeSet[contract.StrikePrice] = true
	}

	var strikes []float64
	for strike := range strikeSet {
		strikes = append(strikes, strike)
	}

	// Sort strikes
	sort.Float64s(strikes)

	// Find ATM strike
	minDiff := abs(strikes[0] - underlyingPrice)
	atmIndex := 0

	for i, strike := range strikes {
		diff := abs(strike - underlyingPrice)
		if diff < minDiff {
			minDiff = diff
			atmIndex = i
		}
	}

	// Calculate range
	strikesPerSide := strikeCount / 2
	startIndex := max(0, atmIndex-strikesPerSide)
	endIndex := min(len(strikes), atmIndex+strikesPerSide+1)

	selectedStrikes := make(map[float64]bool)
	for i := startIndex; i < endIndex; i++ {
		selectedStrikes[strikes[i]] = true
	}

	// Filter contracts
	var filtered []*models.OptionContract
	for _, contract := range contracts {
		if selectedStrikes[contract.StrikePrice] {
			filtered = append(filtered, contract)
		}
	}

	// Sort filtered contracts by strike price and option type for consistent ordering
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].StrikePrice != filtered[j].StrikePrice {
			return filtered[i].StrikePrice < filtered[j].StrikePrice
		}
		// If strikes are equal, sort by option type (calls first)
		return filtered[i].Type == "call" && filtered[j].Type == "put"
	})

	return filtered
}

// GetOptionsGreeksBatch gets Greeks for multiple option symbols.
// Exact conversion of Python get_options_greeks_batch method.
func (t *TradierProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	slog.Debug("GetOptionsGreeksBatch called", "symbolCount", len(optionSymbols))

	// Group symbols by underlying and expiration
	symbolGroups := make(map[string]struct {
		underlying string
		expiry     string
		symbols    []string
	})

	for _, optionSymbol := range optionSymbols {
		parsed := t.parseOptionSymbol(optionSymbol)
		if parsed == nil {
			slog.Warn("Failed to parse option symbol", "symbol", optionSymbol)
			continue
		}

		key := fmt.Sprintf("%s_%s", parsed.Underlying, parsed.Expiry)
		group := symbolGroups[key]
		group.underlying = parsed.Underlying
		group.expiry = parsed.Expiry
		group.symbols = append(group.symbols, optionSymbol)
		symbolGroups[key] = group
	}

	slog.Debug("Grouped symbols", "groupCount", len(symbolGroups))

	// Fetch Greeks for each group
	greeksData := make(map[string]map[string]interface{})

	for _, group := range symbolGroups {
		// Calculate strike count needed based on requested symbols
		// Get all unique strikes from the requested symbols
		strikesNeeded := make(map[float64]bool)
		for _, symbol := range group.symbols {
			parsed := t.parseOptionSymbol(symbol)
			if parsed != nil {
				strikesNeeded[parsed.Strike] = true
			}
		}

		// Use a strike count that covers all requested strikes plus some buffer
		// This ensures we get all the strikes the frontend is asking for
		strikeCount := max(len(strikesNeeded)*2, 50) // At least 50 strikes or 2x requested

		slog.Debug("Fetching options chain with Greeks", "underlying", group.underlying, "expiry", group.expiry, "strikeCount", strikeCount, "requestedSymbols", len(group.symbols))

		// CRITICAL FIX: Call GetOptionsChainSmart with include_greeks=true to get Greeks data
		contracts, err := t.GetOptionsChainSmart(ctx, group.underlying, group.expiry, nil, strikeCount, true, false)
		if err != nil {
			slog.Error("Failed to get options chain for Greeks", "underlying", group.underlying, "expiry", group.expiry, "error", err)
			continue
		}

		slog.Debug("Received contracts", "count", len(contracts))

		// Extract Greeks for requested symbols
		matchedCount := 0
		for _, contract := range contracts {
			for _, requestedSymbol := range group.symbols {
				if contract.Symbol == requestedSymbol {
					matchedCount++
					// Ensure we store a plain numeric float64 (or nil) for implied_volatility
					var ivVal interface{}
					if contract.ImpliedVolatility != nil {
						ivVal = *contract.ImpliedVolatility
					} else {
						ivVal = nil
					}
					greeksData[contract.Symbol] = map[string]interface{}{
						"delta":              contract.Delta,
						"theta":              contract.Theta,
						"gamma":              contract.Gamma,
						"vega":               contract.Vega,
						"implied_volatility": ivVal,
					}

					// Log what we extracted at DEBUG level
					ivValue := "nil"
					if contract.ImpliedVolatility != nil {
						ivValue = fmt.Sprintf("%.4f", *contract.ImpliedVolatility)
					}
					slog.Debug("Extracted Greeks", "symbol", contract.Symbol, "iv", ivValue)
				}
			}
		}

		slog.Debug("Matched symbols", "matched", matchedCount, "requested", len(group.symbols))
	}

	slog.Debug("GetOptionsGreeksBatch complete", "totalGreeksReturned", len(greeksData))
	return greeksData, nil
}

// GetOptionsChainSmart gets smart options chain data.
// Exact conversion of Python get_options_chain_smart method.
func (t *TradierProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/options/chains", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	// Determine API symbol to use
	apiSymbol := symbol
	if len(symbol) > 3 && (strings.HasSuffix(symbol, "W") || strings.HasSuffix(symbol, "P")) {
		apiSymbol = symbol[:len(symbol)-1]
	}

	params := make(map[string]string)
	params["symbol"] = apiSymbol
	params["expiration"] = expiry
	// CRITICAL: Set greeks parameter based on includeGreeks flag
	if includeGreeks {
		params["greeks"] = "true"
		slog.Debug("Tradier: Requesting options chain WITH Greeks", "symbol", symbol, "expiry", expiry)
	} else {
		params["greeks"] = "false"
		slog.Debug("Tradier: Requesting options chain WITHOUT Greeks", "symbol", symbol, "expiry", expiry)
	}

	resp, err := t.client.Get(ctx, endpoint, headers, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get options chain: %w", err)
	}

	// DEBUG: Log raw response to see what Tradier is actually returning
	slog.Debug("Tradier: Raw API response", "bodyLength", len(resp.Body), "includeGreeks", includeGreeks)

	var response struct {
		Options struct {
			Option interface{} `json:"option"`
		} `json:"options"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse options chain response: %w", err)
	}

	// Handle both single option (object) and multiple options (array)
	var options []map[string]interface{}
	switch v := response.Options.Option.(type) {
	case map[string]interface{}:
		options = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if opt, ok := item.(map[string]interface{}); ok {
				options = append(options, opt)
			}
		}
	}

	// Filter contracts by root symbol
	var filteredOptions []map[string]interface{}
	upperSymbol := strings.ToUpper(symbol)

	for _, option := range options {
		optionSymbol, ok := option["symbol"].(string)
		if !ok || len(optionSymbol) < 15 {
			continue
		}

		// Extract root from option symbol (OCC format)
		root := strings.ToUpper(optionSymbol[:len(optionSymbol)-15])

		// Check for exact match
		if root == upperSymbol {
			filteredOptions = append(filteredOptions, option)
			continue
		}

		// Check weekly map: if requested symbol maps to a weekly root, accept that root
		if weeklyRoot, ok := weeklyMap[upperSymbol]; ok && root == weeklyRoot {
			filteredOptions = append(filteredOptions, option)
			continue
		}

		// Check reverse: if root is in weekly map and maps to requested symbol
		for indexSymbol, weeklyRoot := range weeklyMap {
			if root == indexSymbol && upperSymbol == weeklyRoot {
				filteredOptions = append(filteredOptions, option)
				break
			}
		}
	}
	// Diagnostic: if filtering removed everything, log raw response details to help debug root/format mismatches
	if len(filteredOptions) == 0 {
		var sampleSymbols []string
		for i, opt := range options {
			if i >= 10 {
				break
			}
			if osym, ok := opt["symbol"].(string); ok {
				root := ""
				if len(osym) >= 15 {
					root = strings.ToUpper(osym[:len(osym)-15])
				}
				sampleSymbols = append(sampleSymbols, fmt.Sprintf("%s(root=%s)", osym, root))
			}
		}
		slog.Warn("Tradier: Filtered options empty after root filtering", "requestedSymbol", symbol, "apiSymbol", apiSymbol, "totalOptions", len(options), "sampleSymbols", sampleSymbols)
	}

	// Transform contracts with Greeks if requested
	var contracts []*models.OptionContract
	greeksFoundCount := 0
	for i, option := range filteredOptions {
		contract := t.transformOptionContract(option)
		if contract != nil {
			contracts = append(contracts, contract)
			// DEBUG: Check if Greeks were extracted
			if contract.ImpliedVolatility != nil {
				greeksFoundCount++
				if i < 3 { // Log first 3 contracts with Greeks
					slog.Debug("Tradier: Contract with Greeks",
						"symbol", contract.Symbol,
						"iv", *contract.ImpliedVolatility,
						"delta", contract.Delta,
						"gamma", contract.Gamma,
						"theta", contract.Theta,
						"vega", contract.Vega)
				}
			}
		}
	}

	slog.Debug("Tradier: Greeks extraction summary",
		"totalContracts", len(contracts),
		"contractsWithGreeks", greeksFoundCount,
		"includeGreeks", includeGreeks)

	// Apply strike filtering if underlying price is provided
	if underlyingPrice != nil && len(contracts) > 0 {
		contracts = t.filterByStrikeCount(contracts, *underlyingPrice, atmRange)
	} else {
		// Sort all contracts by strike price and option type for consistent ordering
		sort.Slice(contracts, func(i, j int) bool {
			if contracts[i].StrikePrice != contracts[j].StrikePrice {
				return contracts[i].StrikePrice < contracts[j].StrikePrice
			}
			// If strikes are equal, sort by option type (calls first)
			return contracts[i].Type == "call" && contracts[j].Type == "put"
		})
	}

	return contracts, nil
}

// GetPositions gets all current positions.
// Exact conversion of Python get_positions method.
func (t *TradierProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/positions", t.baseURL, t.accountID)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	resp, err := t.client.Get(ctx, endpoint, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var response struct {
		Positions interface{} `json:"positions"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse positions response: %w", err)
	}

	// Handle different response structures
	var positions []map[string]interface{}

	switch v := response.Positions.(type) {
	case map[string]interface{}:
		if posArray, ok := v["position"]; ok {
			switch posArray := posArray.(type) {
			case map[string]interface{}:
				positions = []map[string]interface{}{posArray}
			case []interface{}:
				for _, item := range posArray {
					if pos, ok := item.(map[string]interface{}); ok {
						positions = append(positions, pos)
					}
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if pos, ok := item.(map[string]interface{}); ok {
				positions = append(positions, pos)
			}
		}
	}

	var result []*models.Position
	for _, position := range positions {
		transformed := t.transformPosition(position)
		if transformed != nil {
			result = append(result, transformed)
		}
	}

	return result, nil
}

// GetPositionsEnhanced gets enhanced positions grouped by underlying symbol and strategy.
// Exact conversion of Python get_positions_enhanced method.
func (t *TradierProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	slog.Debug("Tradier: Getting enhanced positions...")

	// 1. Get current positions
	currentPositions, err := t.GetPositions(ctx)
	if err != nil {
		return models.NewEnhancedPositionsResponse(), nil
	}

	if len(currentPositions) == 0 {
		slog.Info("Tradier: No current positions found")
		return models.NewEnhancedPositionsResponse(), nil
	}

	// 2. Use base provider's conversion logic
	return t.BaseProviderImpl.ConvertPositionsToEnhanced(currentPositions), nil
}

// transformPosition transforms Tradier position to our standard model.
// Exact conversion of Python _transform_position method.
func (t *TradierProvider) transformPosition(rawPosition map[string]interface{}) *models.Position {
	symbol, _ := rawPosition["symbol"].(string)
	costBasis, _ := rawPosition["cost_basis"].(float64)
	quantity, _ := rawPosition["quantity"].(float64)
	dateAcquired, _ := rawPosition["date_acquired"].(string)

	// Calculate average entry price
	var avgEntryPrice float64
	if quantity != 0 {
		if t.isOptionSymbol(symbol) {
			// For options: cost_basis is total cost, divide by quantity and then by 100
			avgEntryPrice = (costBasis / quantity) / 100
		} else {
			// For stocks: cost_basis is total cost, divide by quantity
			avgEntryPrice = costBasis / quantity
		}
	}

	side := "long"
	if quantity < 0 {
		side = "short"
	}

	assetClass := "us_equity"
	if t.isOptionSymbol(symbol) {
		assetClass = "us_option"
	}

	var unrealizedPLPC *float64
	zero := 0.0
	unrealizedPLPC = &zero

	var dateAcquiredPtr *string
	if dateAcquired != "" {
		dateAcquiredPtr = &dateAcquired
	}

	return &models.Position{
		Symbol:         symbol,
		Qty:            quantity,
		Side:           side,
		CostBasis:      costBasis,
		MarketValue:    0, // Will be calculated later
		UnrealizedPL:   0, // Will be calculated later
		UnrealizedPLPC: unrealizedPLPC,
		CurrentPrice:   0, // Will be calculated later
		AvgEntryPrice:  avgEntryPrice,
		AssetClass:     assetClass,
		DateAcquired:   dateAcquiredPtr,
	}
}

// GetOrders gets orders with optional status filter.
// Exact conversion of Python get_orders method.
func (t *TradierProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/orders", t.baseURL, t.accountID)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	resp, err := t.client.Get(ctx, endpoint, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	// Tradier can return "orders": "" (empty string) when no orders exist
	// So we use interface{} for Orders field to handle both cases
	var response struct {
		Orders interface{} `json:"orders"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}

	// Handle the orders field which can be string, object, or null
	var orders []map[string]interface{}

	switch ordersVal := response.Orders.(type) {
	case string:
		// Tradier returns "" (empty string) when no orders exist
		if ordersVal == "" {
			return []*models.Order{}, nil
		}
	case map[string]interface{}:
		// Orders is an object with an "order" field
		if orderField, ok := ordersVal["order"]; ok {
			switch v := orderField.(type) {
			case map[string]interface{}:
				orders = []map[string]interface{}{v}
			case []interface{}:
				for _, item := range v {
					if order, ok := item.(map[string]interface{}); ok {
						orders = append(orders, order)
					}
				}
			case string:
				// order field is empty string
				if v == "" {
					return []*models.Order{}, nil
				}
			}
		}
	case nil:
		// Handle null case
		return []*models.Order{}, nil
	}

	result := []*models.Order{} // Initialize as empty slice, not nil
	for _, order := range orders {
		orderStatus, _ := order["status"].(string)

		// Filter orders based on status
		includeOrder := false
		switch strings.ToLower(status) {
		case "all":
			includeOrder = true
		case "pending":
			includeOrder = orderStatus == "open" || orderStatus == "pending"
		case "open":
			includeOrder = orderStatus == "open"
		case "canceled", "cancelled":
			includeOrder = orderStatus == "canceled" || orderStatus == "cancelled" || orderStatus == "rejected"
		case "filled":
			includeOrder = orderStatus == "filled"
		case "rejected":
			includeOrder = orderStatus == "rejected"
		default:
			includeOrder = orderStatus == strings.ToLower(status)
		}

		if includeOrder {
			transformed := t.transformOrder(order)
			if transformed != nil {
				result = append(result, transformed)
			}
		}
	}

	return result, nil
}

// transformOrder transforms Tradier order to our standard model.
// Exact conversion of Python _transform_order method.
func (t *TradierProvider) transformOrder(rawOrder map[string]interface{}) *models.Order {
	id, _ := rawOrder["id"].(float64)
	symbol, _ := rawOrder["symbol"].(string)
	assetClass, _ := rawOrder["class"].(string)
	side, _ := rawOrder["side"].(string)
	orderType, _ := rawOrder["type"].(string)
	quantity, _ := rawOrder["quantity"].(float64)
	execQuantity, _ := rawOrder["exec_quantity"].(float64)
	status, _ := rawOrder["status"].(string)
	duration, _ := rawOrder["duration"].(string)
	createDate, _ := rawOrder["create_date"].(string)
	transactionDate, _ := rawOrder["transaction_date"].(string)

	var limitPrice, avgFillPrice *float64
	if price, ok := rawOrder["price"].(float64); ok {
		// If order type is credit, make limit price negative
		if strings.ToLower(orderType) == "credit" {
			price = -abs(price)
		}
		limitPrice = &price
	}
	if avgPrice, ok := rawOrder["avg_fill_price"].(float64); ok {
		avgFillPrice = &avgPrice
	}

	// Handle legs
	var legs []map[string]interface{}
	if legData, ok := rawOrder["leg"].([]interface{}); ok {
		for _, leg := range legData {
			if legMap, ok := leg.(map[string]interface{}); ok {
				transformedLeg := map[string]interface{}{
					"symbol": legMap["option_symbol"],
					"side":   legMap["side"],
					"qty":    legMap["quantity"],
				}
				legs = append(legs, transformedLeg)
			}
		}
	}

	return &models.Order{
		ID:           fmt.Sprintf("%.0f", id),
		Symbol:       symbol,
		AssetClass:   assetClass,
		Side:         side,
		OrderType:    orderType,
		Qty:          quantity,
		FilledQty:    execQuantity,
		LimitPrice:   limitPrice,
		StopPrice:    nil, // Tradier doesn't provide stop price in this format
		AvgFillPrice: avgFillPrice,
		Status:       status,
		TimeInForce:  duration,
		SubmittedAt:  createDate,
		FilledAt:     getStringPtr(transactionDate),
		Legs:         convertLegsToOrderLegs(legs),
	}
}

// GetAccount gets account information.
// Exact conversion of Python get_account method.
func (t *TradierProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/balances", t.baseURL, t.accountID)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	resp, err := t.client.Get(ctx, endpoint, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	var response struct {
		Balances map[string]interface{} `json:"balances"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	if len(response.Balances) > 0 {
		return t.transformAccount(response.Balances), nil
	}

	return nil, fmt.Errorf("no account balances found")
}

// transformAccount transforms Tradier account balances to our standard model.
// Exact conversion of Python _transform_account method.
func (t *TradierProvider) transformAccount(rawBalances map[string]interface{}) *models.Account {
	accountType, _ := rawBalances["account_type"].(string)
	accountType = strings.ToLower(accountType)

	var stockBuyingPower, optionsBuyingPower, cashAvailable *float64

	// Get buying power based on account type
	switch accountType {
	case "margin":
		if marginData, ok := rawBalances["margin"].(map[string]interface{}); ok {
			if sbp, ok := marginData["stock_buying_power"].(float64); ok {
				stockBuyingPower = &sbp
			}
			if obp, ok := marginData["option_buying_power"].(float64); ok {
				optionsBuyingPower = &obp
			}
		}
	case "cash":
		if cashData, ok := rawBalances["cash"].(map[string]interface{}); ok {
			if ca, ok := cashData["cash_available"].(float64); ok {
				cashAvailable = &ca
				stockBuyingPower = &ca
				optionsBuyingPower = &ca
			}
		}
	case "pdt":
		if pdtData, ok := rawBalances["pdt"].(map[string]interface{}); ok {
			if sbp, ok := pdtData["stock_buying_power"].(float64); ok {
				stockBuyingPower = &sbp
			}
			if obp, ok := pdtData["option_buying_power"].(float64); ok {
				optionsBuyingPower = &obp
			}
		}
	}

	// Fallback for cash available
	if cashAvailable == nil {
		if totalCash, ok := rawBalances["total_cash"].(float64); ok {
			cashAvailable = &totalCash
		}
	}

	// Determine overall buying power
	var buyingPower *float64
	if optionsBuyingPower != nil {
		buyingPower = optionsBuyingPower
	} else if stockBuyingPower != nil {
		buyingPower = stockBuyingPower
	}

	var portfolioValue, equity, longMarketValue, shortMarketValue, initialMargin *float64
	if te, ok := rawBalances["total_equity"].(float64); ok {
		portfolioValue = &te
		equity = &te
	}
	if lmv, ok := rawBalances["long_market_value"].(float64); ok {
		longMarketValue = &lmv
	}
	if smv, ok := rawBalances["short_market_value"].(float64); ok {
		shortMarketValue = &smv
	}
	if im, ok := rawBalances["current_requirement"].(float64); ok {
		initialMargin = &im
	}

	accountNumber, _ := rawBalances["account_number"].(string)
	if accountNumber == "" {
		accountNumber = t.accountID
	}

	var accountNumberPtr *string
	if accountNumber != "" {
		accountNumberPtr = &accountNumber
	}

	isPDT := accountType == "pdt"

	return &models.Account{
		AccountID:             t.accountID,
		AccountNumber:         accountNumberPtr,
		Status:                "active",
		Currency:              "USD",
		BuyingPower:           buyingPower,
		Cash:                  cashAvailable,
		PortfolioValue:        portfolioValue,
		Equity:                equity,
		DayTradingBuyingPower: buyingPower,
		RegtBuyingPower:       stockBuyingPower,
		OptionsBuyingPower:    optionsBuyingPower,
		PatternDayTrader:      &isPDT,
		LongMarketValue:       longMarketValue,
		ShortMarketValue:      shortMarketValue,
		InitialMargin:         initialMargin,
	}
}

// PlaceOrder places a trading order.
// Exact conversion of Python place_order method.
func (t *TradierProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/orders", t.baseURL, t.accountID)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	symbol, _ := orderData["symbol"].(string)
	orderType, _ := orderData["order_type"].(string)
	if orderType == "" {
		orderType = "market"
	}

	data := url.Values{}

	if t.isOptionSymbol(symbol) {
		// Option order
		parsed := t.parseOptionSymbol(symbol)
		if parsed == nil {
			return nil, fmt.Errorf("invalid option symbol: %s", symbol)
		}

		data.Set("class", "option")
		data.Set("symbol", parsed.Underlying)
		data.Set("option_symbol", symbol)
		data.Set("side", orderData["side"].(string))
		data.Set("quantity", fmt.Sprintf("%.0f", orderData["qty"].(float64)))
		data.Set("type", orderType)
		data.Set("duration", getStringOrDefault(orderData, "time_in_force", "day"))

		if orderType == "limit" {
			if limitPrice, ok := orderData["limit_price"].(float64); ok {
				data.Set("price", fmt.Sprintf("%.2f", limitPrice))
			}
		}
	} else {
		// Equity order
		side, _ := orderData["side"].(string)
		if side == "sell" {
			if isShortSell, ok := orderData["is_short_sell"].(bool); ok && isShortSell {
				side = "sell_short"
			}
		}

		data.Set("class", "equity")
		data.Set("symbol", symbol)
		data.Set("side", side)
		data.Set("quantity", fmt.Sprintf("%.0f", orderData["qty"].(float64)))
		data.Set("type", orderType)
		data.Set("duration", getStringOrDefault(orderData, "time_in_force", "day"))

		if orderType == "limit" || orderType == "stop_limit" {
			if limitPrice, ok := orderData["limit_price"].(float64); ok {
				data.Set("price", fmt.Sprintf("%.2f", limitPrice))
			}
		}
		if orderType == "stop" || orderType == "stop_limit" {
			if stopPrice, ok := orderData["stop_price"].(float64); ok {
				data.Set("stop", fmt.Sprintf("%.2f", stopPrice))
			}
		}
	}

	// Convert url.Values to map[string]string
	formData := make(map[string]string)
	for key, values := range data {
		if len(values) > 0 {
			formData[key] = values[0]
		}
	}

	resp, err := t.client.PostForm(ctx, endpoint, headers, formData)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	var response struct {
		Order struct {
			ID interface{} `json:"id"`
		} `json:"order"`
		Errors interface{} `json:"errors"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if response.Order.ID != nil {
		orderID := fmt.Sprintf("%v", response.Order.ID)

		assetClass := "us_equity"
		if t.isOptionSymbol(symbol) {
			assetClass = "us_option"
		}

		return &models.Order{
			ID:          orderID,
			Symbol:      symbol,
			AssetClass:  assetClass,
			Side:        orderData["side"].(string),
			OrderType:   orderType,
			Qty:         orderData["qty"].(float64),
			FilledQty:   0,
			LimitPrice:  getFloatPtr(orderData, "limit_price"),
			StopPrice:   getFloatPtr(orderData, "stop_price"),
			Status:      "submitted",
			TimeInForce: getStringOrDefault(orderData, "time_in_force", "day"),
			SubmittedAt: time.Now().Format(time.RFC3339),
		}, nil
	}

	// Handle errors
	errorMsg := "Unknown error"
	if response.Errors != nil {
		switch errors := response.Errors.(type) {
		case map[string]interface{}:
			if errStr, ok := errors["error"].(string); ok {
				errorMsg = errStr
			}
		case []interface{}:
			var errStrs []string
			for _, err := range errors {
				if errStr, ok := err.(string); ok {
					errStrs = append(errStrs, errStr)
				}
			}
			if len(errStrs) > 0 {
				errorMsg = strings.Join(errStrs, ", ")
			}
		}
	}

	return nil, fmt.Errorf("failed to place order: %s", errorMsg)
}

// PlaceMultiLegOrder places a multi-leg trading order.
// Exact conversion of Python place_multi_leg_order method.
func (t *TradierProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/orders", t.baseURL, t.accountID)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	legs, _ := orderData["legs"].([]interface{})
	if len(legs) == 0 {
		return nil, fmt.Errorf("no legs provided for multi-leg order")
	}

	// Get underlying symbol from first leg
	firstLeg := legs[0].(map[string]interface{})
	firstSymbol, _ := firstLeg["symbol"].(string)
	parsed := t.parseOptionSymbol(firstSymbol)
	if parsed == nil {
		return nil, fmt.Errorf("invalid option symbol in first leg: %s", firstSymbol)
	}

	limitPrice, _ := orderData["limit_price"].(float64)
	orderType := "debit"
	if limitPrice < 0 {
		orderType = "credit"
	}

	data := url.Values{}
	data.Set("class", "multileg")
	data.Set("symbol", parsed.Underlying)
	data.Set("type", orderType)
	data.Set("duration", getStringOrDefault(orderData, "time_in_force", "day"))
	data.Set("price", fmt.Sprintf("%.2f", abs(limitPrice)))

	for i, leg := range legs {
		legMap := leg.(map[string]interface{})
		data.Set(fmt.Sprintf("option_symbol[%d]", i), legMap["symbol"].(string))
		data.Set(fmt.Sprintf("side[%d]", i), strings.ToLower(legMap["side"].(string)))
		data.Set(fmt.Sprintf("quantity[%d]", i), fmt.Sprintf("%.0f", legMap["qty"].(float64)))
	}

	// Convert url.Values to map[string]string
	formData := make(map[string]string)
	for key, values := range data {
		if len(values) > 0 {
			formData[key] = values[0]
		}
	}

	resp, err := t.client.PostForm(ctx, endpoint, headers, formData)
	if err != nil {
		return nil, fmt.Errorf("failed to place multi-leg order: %w", err)
	}

	var response struct {
		Order struct {
			ID interface{} `json:"id"`
		} `json:"order"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("failed to parse multi-leg order response: %w", err)
	}

	if response.Order.ID != nil {
		orderID := fmt.Sprintf("%v", response.Order.ID)

		// Convert legs to the expected format
		var orderLegs []map[string]interface{}
		for _, leg := range legs {
			legMap := leg.(map[string]interface{})
			orderLegs = append(orderLegs, map[string]interface{}{
				"symbol": legMap["symbol"],
				"side":   legMap["side"],
				"qty":    legMap["qty"],
			})
		}

		return &models.Order{
			ID:          orderID,
			Symbol:      "Multi-leg",
			AssetClass:  "us_option",
			Side:        orderType,
			OrderType:   orderType,
			Qty:         getFloatOrDefault(orderData, "qty", 1),
			FilledQty:   0,
			LimitPrice:  &limitPrice,
			Status:      "submitted",
			TimeInForce: getStringOrDefault(orderData, "time_in_force", "day"),
			SubmittedAt: time.Now().Format(time.RFC3339),
			Legs:        convertLegsToOrderLegs(orderLegs),
		}, nil
	}

	return nil, fmt.Errorf("failed to place multi-leg order")
}

// PreviewOrder previews an order without placing it.
// Exact conversion of Python preview_order method.
func (t *TradierProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	slog.Info("🔍 Tradier: PreviewOrder called", "orderData", orderData)

	endpoint := fmt.Sprintf("%s/v1/accounts/%s/orders", t.baseURL, t.accountID)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	// Determine if this is an equity order or options order
	legs, _ := orderData["legs"].([]interface{})
	symbol, _ := orderData["symbol"].(string)
	side, _ := orderData["side"].(string)

	slog.Info("🔍 Tradier: Processing preview", "legs", len(legs), "symbol", symbol, "side", side)

	var payload map[string]string

	// Check if this is a simple equity order
	if symbol != "" && side != "" && (len(legs) == 0) && !t.isOptionSymbol(symbol) {
		// Single equity order preview
		orderSide := side
		if side == "sell" {
			if isShortSell, ok := orderData["is_short_sell"].(bool); ok && isShortSell {
				orderSide = "sell_short"
			}
		}

		payload = map[string]string{
			"class":    "equity",
			"symbol":   symbol,
			"side":     orderSide,
			"quantity": fmt.Sprintf("%.0f", getQuantity(orderData)),
			"type":     getString(orderData, "order_type"),
			"duration": getString(orderData, "time_in_force"),
			"preview":  "true",
		}

		// Add price parameters based on order type
		orderType := getString(orderData, "order_type")
		if orderType == "limit" || orderType == "stop_limit" {
			if limitPrice := getFloat(orderData, "limit_price"); limitPrice != 0 {
				payload["price"] = fmt.Sprintf("%.2f", limitPrice)
			}
		}
		if orderType == "stop" || orderType == "stop_limit" {
			if stopPrice := getFloat(orderData, "stop_price"); stopPrice != 0 {
				payload["stop"] = fmt.Sprintf("%.2f", stopPrice)
			}
		}
	} else if len(legs) == 1 {
		// Single-leg option order preview
		leg := legs[0].(map[string]interface{})
		legSymbol := getString(leg, "symbol")

		if t.isOptionSymbol(legSymbol) {
			// Option order
			parsed := t.parseOptionSymbol(legSymbol)
			if parsed == nil {
				return map[string]interface{}{
					"status":              "error",
					"validation_errors":   []string{fmt.Sprintf("Invalid option symbol: %s", legSymbol)},
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

			payload = map[string]string{
				"class":         "option",
				"symbol":        parsed.Underlying,
				"option_symbol": legSymbol,
				"side":          getString(leg, "side"),
				"quantity":      fmt.Sprintf("%.0f", getFloat(leg, "qty")),
				"type":          getString(orderData, "order_type"),
				"duration":      getString(orderData, "time_in_force"),
				"preview":       "true",
			}

			if limitPrice := getFloat(orderData, "limit_price"); limitPrice != 0 {
				payload["price"] = fmt.Sprintf("%.2f", abs(limitPrice))
			}
		} else {
			// Single equity order from legs format
			legSide := getString(leg, "side")
			if legSide == "sell" {
				if isShortSell, ok := orderData["is_short_sell"].(bool); ok && isShortSell {
					legSide = "sell_short"
				}
			}

			payload = map[string]string{
				"class":    "equity",
				"symbol":   legSymbol,
				"side":     legSide,
				"quantity": fmt.Sprintf("%.0f", getFloat(leg, "qty")),
				"type":     getString(orderData, "order_type"),
				"duration": getString(orderData, "time_in_force"),
				"preview":  "true",
			}

			// Add price parameters based on order type
			orderType := getString(orderData, "order_type")
			if orderType == "limit" || orderType == "stop_limit" {
				if limitPrice := getFloat(orderData, "limit_price"); limitPrice != 0 {
					payload["price"] = fmt.Sprintf("%.2f", limitPrice)
				}
			}
			if orderType == "stop" || orderType == "stop_limit" {
				if stopPrice := getFloat(orderData, "stop_price"); stopPrice != 0 {
					payload["stop"] = fmt.Sprintf("%.2f", stopPrice)
				}
			}
		}
	} else {
		// Multi-leg option order preview
		if len(legs) == 0 {
			return map[string]interface{}{
				"status":              "error",
				"validation_errors":   []string{"No legs provided for multi-leg order"},
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

		firstLeg := legs[0].(map[string]interface{})
		firstSymbol := getString(firstLeg, "symbol")
		parsed := t.parseOptionSymbol(firstSymbol)
		if parsed == nil {
			return map[string]interface{}{
				"status":              "error",
				"validation_errors":   []string{fmt.Sprintf("Invalid option symbol in first leg: %s", firstSymbol)},
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

		limitPrice := getFloat(orderData, "limit_price")
		orderType := "debit"
		if limitPrice < 0 {
			orderType = "credit"
		}

		payload = map[string]string{
			"class":    "multileg",
			"symbol":   parsed.Underlying,
			"type":     orderType,
			"duration": getString(orderData, "time_in_force"),
			"price":    fmt.Sprintf("%.2f", abs(limitPrice)),
			"preview":  "true",
		}

		for i, leg := range legs {
			legMap := leg.(map[string]interface{})
			payload[fmt.Sprintf("option_symbol[%d]", i)] = getString(legMap, "symbol")
			payload[fmt.Sprintf("side[%d]", i)] = strings.ToLower(getString(legMap, "side"))
			payload[fmt.Sprintf("quantity[%d]", i)] = fmt.Sprintf("%.0f", getFloat(legMap, "qty"))
		}
	}

	slog.Info("🔍 Tradier: Previewing order with payload", "payload", payload)

	resp, err := t.client.PostForm(ctx, endpoint, headers, payload)
	if err != nil {
		slog.Error("❌ Tradier: preview_order failed", "error", err)
		return map[string]interface{}{
			"status":              "error",
			"validation_errors":   []string{fmt.Sprintf("Preview failed: %s", err.Error())},
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

	var response struct {
		Order  map[string]interface{} `json:"order"`
		Errors interface{}            `json:"errors"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return map[string]interface{}{
			"status":              "error",
			"validation_errors":   []string{fmt.Sprintf("Failed to parse preview response: %s", err.Error())},
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

	slog.Info("📊 Tradier: Preview response", "response", response)

	// Extract preview data
	orderPreview := response.Order
	if status, ok := orderPreview["status"].(string); ok && status == "ok" {
		// For equity orders, order_cost is the actual cost, not in cents
		orderCost := getFloat(orderPreview, "order_cost")

		// For equity orders, don't divide by 100 (that's for options)
		finalOrderCost := orderCost
		if payload["class"] != "equity" {
			finalOrderCost = orderCost / 100
		}

		commission := getFloat(orderPreview, "commission")
		cost := getFloat(orderPreview, "cost")
		fees := getFloat(orderPreview, "fees")
		marginChange := getFloat(orderPreview, "margin_change")
		dayTrades := int(getFloat(orderPreview, "day_trades"))
		estimatedTotal := finalOrderCost + commission + fees

		return map[string]interface{}{
			"status":                "ok",
			"preview_not_available": false,
			"commission":            commission,
			"cost":                  cost,
			"fees":                  fees,
			"order_cost":            finalOrderCost,
			"margin_change":         marginChange,
			"buying_power_effect":   cost,
			"day_trades":            dayTrades,
			"validation_errors":     []string{},
			"estimated_total":       estimatedTotal,
		}, nil
	} else {
		errorMsg := "Unknown error"
		if response.Errors != nil {
			switch errors := response.Errors.(type) {
			case map[string]interface{}:
				if errStr, ok := errors["error"].(string); ok {
					errorMsg = errStr
				}
			case []interface{}:
				var errStrs []string
				for _, err := range errors {
					if errStr, ok := err.(string); ok {
						errStrs = append(errStrs, errStr)
					}
				}
				if len(errStrs) > 0 {
					errorMsg = strings.Join(errStrs, ", ")
				}
			}
		}

		return map[string]interface{}{
			"status":              "error",
			"validation_errors":   []string{errorMsg},
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

// CancelOrder cancels an existing order.
// Exact conversion of Python cancel_order method.
func (t *TradierProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	endpoint := fmt.Sprintf("%s/v1/accounts/%s/orders/%s", t.baseURL, t.accountID, orderID)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	resp, err := t.client.Delete(ctx, endpoint, headers)
	if err != nil {
		return false, fmt.Errorf("failed to cancel order: %w", err)
	}

	var response struct {
		Order struct {
			Status string `json:"status"`
		} `json:"order"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return false, fmt.Errorf("failed to parse cancel order response: %w", err)
	}

	return response.Order.Status == "ok", nil
}

// LookupSymbols searches for symbols matching the query.
// Exact conversion of Python lookup_symbols method.
func (t *TradierProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/lookup", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	params := url.Values{}
	params.Set("q", query)

	// Convert url.Values to map[string]string
	paramMap := make(map[string]string)
	for key, values := range params {
		if len(values) > 0 {
			paramMap[key] = values[0]
		}
	}

	resp, err := t.client.Get(ctx, endpoint, headers, paramMap)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup symbols: %w", err)
	}

	var response struct {
		Securities interface{} `json:"securities"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse lookup response: %w", err)
	}

	// Handle different response structures
	var securities []map[string]interface{}

	switch v := response.Securities.(type) {
	case map[string]interface{}:
		if secArray, ok := v["security"]; ok {
			switch secArray := secArray.(type) {
			case map[string]interface{}:
				securities = []map[string]interface{}{secArray}
			case []interface{}:
				for _, item := range secArray {
					if sec, ok := item.(map[string]interface{}); ok {
						securities = append(securities, sec)
					}
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if sec, ok := item.(map[string]interface{}); ok {
				securities = append(securities, sec)
			}
		}
	}

	// Sort by relevance
	t.sortSecuritiesByRelevance(securities, query)

	var result []*models.SymbolSearchResult
	for _, security := range securities {
		transformed := t.transformSymbolSearchResult(security)
		if transformed != nil {
			result = append(result, transformed)
		}
	}

	// Limit to 50 results
	if len(result) > 50 {
		result = result[:50]
	}

	return result, nil
}

// GetHistoricalBars gets historical OHLCV bars for charting.
// Exact conversion of Python get_historical_bars method.
func (t *TradierProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	intradayTimeframes := []string{"1m", "5m", "15m", "30m", "1h", "4h"}
	dailyTimeframes := []string{"D", "W", "M"}

	isIntraday := false
	for _, tf := range intradayTimeframes {
		if timeframe == tf {
			isIntraday = true
			break
		}
	}

	if isIntraday {
		return t.getIntradayBars(ctx, symbol, timeframe, startDate, endDate, limit)
	}

	for _, tf := range dailyTimeframes {
		if timeframe == tf {
			return t.getDailyBars(ctx, symbol, timeframe, startDate, endDate, limit)
		}
	}

	return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
}

// createSession creates a Tradier streaming session.
// Exact conversion of Python _create_session method.
func (t *TradierProvider) createSession(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/markets/events/session", t.baseURL)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	resp, err := t.client.Post(ctx, url, nil, headers)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	var response struct {
		Stream struct {
			SessionID string `json:"sessionid"`
		} `json:"stream"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return fmt.Errorf("failed to parse session response: %w", err)
	}

	if response.Stream.SessionID == "" {
		return fmt.Errorf("no session ID in response")
	}

	t.sessionID = response.Stream.SessionID
	slog.Info("Tradier streaming session created", "sessionID", t.sessionID)
	return nil
}

// ConnectStreaming connects to streaming data.
// Enhanced version based on Python implementation with proper WebSocket lifecycle management.
func (t *TradierProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	slog.Info("Tradier: ConnectStreaming called")

	// Lock for connection management
	t.streamMutex.Lock()
	defer t.streamMutex.Unlock()

	// Prevent recursive connection attempts
	if t.connectionInProgress {
		return false, fmt.Errorf("connection already in progress")
	}
	t.connectionInProgress = true
	defer func() { t.connectionInProgress = false }()

	// Clear previous connection state
	slog.Info("Tradier: Clearing previous connection state")
	t.IsConnected = false

	// Reset connection ready channel if needed
	select {
	case <-t.connectionReady:
		t.connectionReady = make(chan struct{})
	default:
	}

	// Validate stream URL is set
	if t.streamURL == "" {
		slog.Error("Tradier: Stream URL is not configured")
		return false, fmt.Errorf("stream URL is not configured")
	}

	// Check if streaming is temporarily disabled due to recent failures
	if t.streamingDisabled && time.Since(t.lastConnectionError) < 5*time.Minute {
		slog.Warn("Tradier: Streaming temporarily disabled due to recent failures")
		return false, fmt.Errorf("streaming temporarily disabled")
	}
	t.streamingDisabled = false

	// CRITICAL FIX: Only create a NEW session if we don't have one already
	// Python implementation reuses sessions - we must do the same to avoid "too many sessions" error
	if t.sessionID == "" {
		slog.Info("Tradier: Creating NEW session...")
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := t.createSession(ctxWithTimeout); err != nil {
			slog.Error("Tradier: Failed to create streaming session", "error", err)
			return false, err
		}
		slog.Info("Tradier: NEW session created successfully", "sessionID", t.sessionID)
	} else {
		slog.Info("Tradier: REUSING existing session", "sessionID", t.sessionID)
	}

	// CRITICAL: Cancel previous stream handler FIRST to stop the reader
	if t.streamCancel != nil {
		slog.Info("Tradier: Cancelling old connection")
		t.streamCancel()

		// IMPORTANT: Set a very short read deadline to unblock any pending ReadMessage() call
		if t.streamConnection != nil {
			t.streamConnection.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

			// Wait for reader to detect cancellation and exit
			time.Sleep(500 * time.Millisecond)

			// Now close the old connection
			closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer closeCancel()

			doneCh := make(chan struct{})
			go func() {
				defer close(doneCh)
				t.streamConnection.Close()
			}()

			select {
			case <-doneCh:
				slog.Debug("Tradier: Old connection closed")
			case <-closeCtx.Done():
				slog.Warn("Tradier: Timeout closing old connection")
			}

			t.streamConnection = nil
		}

		// CRITICAL FIX: Drain old error channel to prevent old errors from affecting new connection
		drained := 0
	drainLoop:
		for {
			select {
			case <-t.errorChan:
				drained++
			case <-time.After(100 * time.Millisecond):
				break drainLoop
			}
		}
		if drained > 0 {
			slog.Debug("Tradier: Drained stale errors", "count", drained)
		}
	}

	// CRITICAL: Create FRESH channels for the new connection
	// This prevents old reader errors from affecting the new stream handler
	t.messageChan = make(chan map[string]interface{}, 100)
	t.errorChan = make(chan error, 10)

	// Configure WebSocket dialer with proper settings matching Python implementation
	dialer := &websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		ReadBufferSize:   1024 * 1024, // 1MB read buffer
		WriteBufferSize:  1024 * 1024, // 1MB write buffer
	}

	// Connect to WebSocket with timeout
	slog.Info("Tradier: Connecting to WebSocket", "url", t.streamURL)
	connCtx, connCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer connCancel()

	conn, resp, err := dialer.DialContext(connCtx, t.streamURL, nil)
	if err != nil {
		slog.Error("Tradier: Failed to connect to WebSocket", "error", err)
		if resp != nil {
			slog.Error("Tradier: WebSocket connection response", "status", resp.Status)
		}
		// Mark streaming as temporarily disabled on connection failure
		t.streamingDisabled = true
		t.lastConnectionError = time.Now()
		return false, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	slog.Info("Tradier: WebSocket connected successfully")

	// Set connection options for better stability matching Python websockets library settings
	conn.SetReadLimit(2 * 1024 * 1024) // 2MB message limit (matches Python max_size)

	// Set up ping/pong handling like Python implementation
	conn.SetPingHandler(func(appData string) error {
		slog.Debug("Tradier: Received ping, sending pong")
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	conn.SetPongHandler(func(appData string) error {
		slog.Debug("Tradier: Received pong")
		return nil
	})

	t.streamConnection = conn

	// Keep-alive: send pings every 25 seconds and reset read deadline on pong
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		slog.Debug("Tradier: Received pong")
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker in its own goroutine (cancelled by streamCtx)
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.writeMutex.Lock()
				if t.streamConnection != nil {
					if err := t.streamConnection.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
						slog.Warn("Tradier: Failed to send ping", "error", err)
					}
				}
				t.writeMutex.Unlock()
			}
		}
	}()

	// Start stream handler with proper context - it will signal ready when truly ready
	streamCtx, cancel := context.WithCancel(context.Background())
	t.streamCancel = cancel

	// Mark as connected before starting handler (handler needs this flag)
	t.IsConnected = true

	go t.streamHandler(streamCtx)

	// Wait for stream handler to signal ready (it will do so after reader is started)
	select {
	case <-t.connectionReady:
		slog.Info("Tradier: Stream handler ready")
	case <-time.After(10 * time.Second):
		slog.Error("Tradier: Timeout waiting for stream handler to be ready")
		cancel()
		return false, fmt.Errorf("timeout waiting for stream handler")
	case <-streamCtx.Done():
		slog.Error("Tradier: Stream handler failed during initialization")
		return false, fmt.Errorf("stream handler failed during initialization")
	}

	// CRITICAL: Add delay to allow Tradier server to fully initialize the session
	// Tradier needs time to associate the WebSocket connection with the session ID
	time.Sleep(2 * time.Second)

	slog.Info("Tradier: Successfully connected to streaming", "sessionID", t.sessionID)
	return true, nil
}

// DisconnectStreaming disconnects from streaming data.
// Exact conversion of Python disconnect_streaming method with proper cleanup.
func (t *TradierProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	slog.Info("Tradier: Disconnecting from streaming...")

	// Lock for connection management
	t.streamMutex.Lock()
	defer t.streamMutex.Unlock()

	// Clear connection state
	t.IsConnected = false
	t.sessionID = ""

	// Cancel stream handler
	if t.streamCancel != nil {
		t.streamCancel()
		t.streamCancel = nil
	}

	// Close WebSocket connection
	if t.streamConnection != nil {
		t.streamConnection.Close()
		t.streamConnection = nil
	}

	// Clear subscribed symbols
	for symbol := range t.subscribedSymbols {
		delete(t.subscribedSymbols, symbol)
	}

	// Clear market data cache
	t.cacheMutex.Lock()
	for symbol := range t.marketDataCache {
		delete(t.marketDataCache, symbol)
	}
	t.cacheMutex.Unlock()

	slog.Info("Tradier: Successfully disconnected from streaming")
	return true, nil
}

// readerGoroutine performs blocking reads from WebSocket and sends messages/errors to channels
func (t *TradierProvider) readerGoroutine(ctx context.Context, done chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Tradier: Reader panic recovered", "panic", r)
		}
		slog.Debug("Tradier: Reader goroutine exiting")
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Tradier: Reader context cancelled, exiting")
			return
		case <-t.shutdownEvent:
			slog.Debug("Tradier: Reader shutdown requested")
			return
		default:
			// Check if connection is still valid before reading
			t.streamMutex.RLock()
			conn := t.streamConnection
			t.streamMutex.RUnlock()

			if conn == nil {
				slog.Debug("Tradier: Connection is nil, reader exiting")
				return
			}

			// No timeout here - blocking read
			t.recvLock.Lock()
			_, messageBytes, err := conn.ReadMessage()
			t.recvLock.Unlock()

			if err != nil {
				// Check if context was cancelled - if so, this is expected
				select {
				case <-ctx.Done():
					slog.Debug("Tradier: Read error after context cancel (expected)")
					return
				default:
					// Real error - report it
					slog.Error("Tradier: Reader encountered error", "error", err, "errorType", fmt.Sprintf("%T", err))
					select {
					case t.errorChan <- err:
						slog.Error("Tradier: Sent error to processing loop", "error", err)
					default:
						slog.Warn("Tradier: Error channel full, could not send error")
					}
					return // Exit reader on any error
				}
			}

			// Parse JSON message
			var message map[string]interface{}
			if parseErr := json.Unmarshal(messageBytes, &message); parseErr != nil {
				slog.Debug("Tradier: Failed to parse JSON message", "error", parseErr)
				continue
			}

			select {
			case t.messageChan <- message:
			case <-ctx.Done():
				return
			default:
				slog.Warn("Tradier: Message channel full, dropping message")
			}
		}
	}
}

func (t *TradierProvider) streamHandler(ctx context.Context) {
	slog.Info("Tradier: Starting stream processor with channel-based architecture")

	// Create a unique handler ID to track if this handler has been superseded
	handlerID := time.Now().UnixNano()
	slog.Info("Tradier: Stream handler started", "handlerID", handlerID)

	var readerDone chan struct{}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("Tradier: Stream processor panic recovered", "panic", r, "sessionID", t.sessionID, "handlerID", handlerID)
		}

		// Wait for reader goroutine to finish
		if readerDone != nil {
			slog.Info("Tradier: Waiting for reader goroutine to exit...", "handlerID", handlerID)
			select {
			case <-readerDone:
				slog.Info("Tradier: Reader goroutine exited", "handlerID", handlerID)
			case <-time.After(5 * time.Second):
				slog.Warn("Tradier: Timed out waiting for reader goroutine", "handlerID", handlerID)
			}
		}

		slog.Info("Tradier: Stream processor stopped", "handlerID", handlerID)
	}()

	readerStarted := false
	readySignaled := false

	for {
		// CRITICAL: Check if context is cancelled at the start of each loop iteration
		select {
		case <-ctx.Done():
			slog.Info("Tradier: Stream processor cancelled", "handlerID", handlerID)
			return
		case <-t.shutdownEvent:
			slog.Info("Tradier: Stream processor shutdown requested", "handlerID", handlerID)
			return
		default:
		}

		if t.IsConnected && t.streamConnection != nil {
			// Start reader only once per connection
			if !readerStarted {
				readerDone = make(chan struct{})
				go t.readerGoroutine(ctx, readerDone)
				readerStarted = true

				// Wait a moment for reader to initialize, then signal ready
				time.Sleep(200 * time.Millisecond)

				if !readySignaled {
					select {
					case <-t.connectionReady:
						// Already closed
					default:
						close(t.connectionReady)
						readySignaled = true
						slog.Info("Tradier: Stream processor ready", "handlerID", handlerID)
					}
				}
			}

			// CRITICAL FIX: Check for errors FIRST before processing messages
			// This prevents the race condition where a message is processed right before an error
			select {
			case <-ctx.Done():
				slog.Info("Tradier: Context cancelled (priority check)", "handlerID", handlerID)
				return
			case <-t.shutdownEvent:
				slog.Info("Tradier: Shutdown (priority check)", "handlerID", handlerID)
				return
			case err := <-t.errorChan:
				// If we get an error from the reader, mark disconnected and exit so the health manager can recover.
				t.streamMutex.RLock()
				recoveryActive := t.recoveryInProgress
				t.streamMutex.RUnlock()

				if recoveryActive {
					slog.Debug("Tradier: Ignoring error from old connection during recovery", "error", err, "handlerID", handlerID)
					continue
				}

				select {
				case <-ctx.Done():
					slog.Debug("Tradier: Ignoring error from cancelled handler", "error", err)
					return
				default:
				}

				slog.Warn("Tradier: Reader error, closing connection and exiting stream handler", "error", err)
				// Update state and cleanup; let external health manager handle reconnection and subscription restoration.
				t.streamMutex.Lock()
				t.IsConnected = false
				if t.streamConnection != nil {
					_ = t.streamConnection.Close()
					t.streamConnection = nil
				}
				t.streamMutex.Unlock()
				return
			default:
				// No error pending, proceed to check for messages
			}

			// Now process messages (only if no error was pending)
			select {
			case <-ctx.Done():
				slog.Info("Tradier: Context cancelled in message loop", "handlerID", handlerID)
				return
			case <-t.shutdownEvent:
				slog.Info("Tradier: Shutdown in message loop", "handlerID", handlerID)
				return
			case message := <-t.messageChan:
				// Normal message - process it
				t.processStreamMessage(message)

				continue // CRITICAL FIX: Continue to next iteration instead of falling through to recovery

			case err := <-t.errorChan:
				// Reader reported an error while waiting for messages. Clean up and exit so health manager can perform recovery.
				t.streamMutex.RLock()
				recoveryActive := t.recoveryInProgress
				t.streamMutex.RUnlock()

				if recoveryActive {
					slog.Debug("Tradier: Ignoring error from old connection during recovery", "error", err, "handlerID", handlerID)
					continue
				}

				select {
				case <-ctx.Done():
					slog.Info("Tradier: Ignoring error from cancelled handler", "error", err, "handlerID", handlerID)
					return
				default:
				}

				slog.Error("Tradier: Reader error detected, cleaning up and exiting stream handler", "error", err, "handlerID", handlerID)
				t.streamMutex.Lock()
				t.IsConnected = false
				if t.streamConnection != nil {
					_ = t.streamConnection.Close()
					t.streamConnection = nil
				}
				t.streamMutex.Unlock()
				return

			case <-time.After(90 * time.Second):
				// Idle timeout - just log, do NOT recover
				slog.Debug("Tradier: Idle - no traffic (normal when no quotes)", "handlerID", handlerID)
				continue
			}

		} else {
			slog.Debug("Tradier: Not connected, waiting...", "handlerID", handlerID, "isConnected", t.IsConnected, "hasConn", t.streamConnection != nil)
			time.Sleep(1 * time.Second)
			continue
		}

		// recovery label removed - provider no longer performs internal recovery.
		// Exit the stream handler here so the external StreamingHealthManager can take recovery actions.
		return
	}
}

// processStreamMessage processes individual stream messages
func (t *TradierProvider) processStreamMessage(message map[string]interface{}) {
	msgType, ok := message["type"].(string)
	if !ok {
		msgType = "unknown"
	}

	// Log messages at DEBUG level for troubleshooting (quotes are routine operations)
	slog.Debug("Tradier: Received message", "type", msgType, "data", message)

	switch msgType {
	case "quote", "trade":
		symbol, _ := message["symbol"].(string)

		// Validate subscription
		if !t.subscribedSymbols[symbol] {
			// Only warn for non-option symbols
			if !t.isOptionSymbol(symbol) {
				slog.Debug("Tradier: Received data for unsubscribed symbol", "symbol", symbol)
			}
			return
		}

		// Create market data object
		timestamp := ""
		if biddate, ok := message["biddate"].(string); ok {
			timestamp = biddate
		} else if date, ok := message["date"].(string); ok {
			timestamp = date
		} else if ts, ok := message["timestamp"].(string); ok {
			timestamp = ts
		}

		// Update local cache to ensure we always send bid, ask, and last
		t.cacheMutex.Lock()
		if _, exists := t.marketDataCache[symbol]; !exists {
			t.marketDataCache[symbol] = &CachedMarketData{}
		}
		cache := t.marketDataCache[symbol]

		// Update cache with new values if they exist
		if bid := message["bid"]; bid != nil {
			cache.Bid = bid
		}
		if ask := message["ask"]; ask != nil {
			cache.Ask = ask
		}
		if last := message["last"]; last != nil {
			cache.Last = last
		} else if price := message["price"]; price != nil && msgType == "trade" {
			// Fallback to price if last is missing in trade event
			cache.Last = price
		}

		// Construct price data from cache
		priceData := map[string]interface{}{
			"bid":       cache.Bid,
			"ask":       cache.Ask,
			"last":      cache.Last,
			"volume":    message["volume"],
			"timestamp": timestamp,
		}

		// If volume is missing in message (e.g. quote), try to get from cache or keep nil?
		// User didn't ask to cache volume, but trade usually has volume (size/cvol).
		// Let's stick to what was requested: bid, ask, last.

		t.cacheMutex.Unlock()

		marketData := &models.MarketData{
			Symbol:    symbol,
			DataType:  "quote", // Normalize to quote for internal consumption
			Timestamp: timestamp,
			Data:      priceData,
		}

		// Send to cache or queue (like Python implementation)
		if err := t.sendToCacheOrQueue(marketData); err != nil {
			slog.Warn("Tradier: Failed to send market data", "error", err, "symbol", symbol)
		}

	case "heartbeat":
		slog.Debug("Tradier: Received heartbeat")

	case "session", "status":
		slog.Info("Tradier: Received session/status confirmation", "type", msgType, "data", message)

	case "error":
		slog.Error("Tradier: Received error message", "data", message)

	default:
		slog.Debug("Tradier: Unknown message type", "type", msgType, "data", message)
	}
}

// handleConnectionLoss handles connection loss by updating state and preparing for recovery
func (t *TradierProvider) handleConnectionLoss(reason string, currentSubscriptions map[string]bool) {
	// Skip if we're already connecting/recovering to prevent feedback loops
	if t.connectionInProgress {
		slog.Debug("Tradier: Ignoring connection loss during connection process", "reason", reason)
		return
	}

	slog.Warn("Tradier: Connection lost", "reason", reason)

	// Update connection state
	t.IsConnected = false
	t.connectionReady = make(chan struct{})

	// Store current subscriptions for recovery
	if t.subscribedSymbols != nil {
		for symbol := range t.subscribedSymbols {
			currentSubscriptions[symbol] = true
		}
		slog.Info("Tradier: Stored subscriptions for recovery", "count", len(currentSubscriptions))
	}

	// Clean up connection
	if t.streamConnection != nil {
		t.streamConnection.Close()
		t.streamConnection = nil
	}
}

// SubscribeToSymbols subscribes to real-time data for symbols.
// Enhanced version matching Python implementation with better error handling and connection management.
func (t *TradierProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	slog.Info("Tradier: SubscribeToSymbols called", "symbolCount", len(symbols))

	// Check if we need to reconnect
	t.streamMutex.RLock()
	isConnected := t.IsConnected
	hasSession := t.sessionID != ""
	hasConn := t.streamConnection != nil
	t.streamMutex.RUnlock()

	if !isConnected || !hasSession || !hasConn {
		// Do not attempt to reconnect inside provider-level subscribe. The external health manager is responsible for reconnection.
		slog.Error("Tradier: Stream not connected. Subscriptions must be requested when provider is connected")
		return false, fmt.Errorf("stream not connected")
	}

	// Wait for connection to be ready (like Python implementation)
	select {
	case <-t.connectionReady:
		// Connection ready
	case <-ctx.Done():
		slog.Error("Tradier: Context cancelled while waiting for connection")
		return false, ctx.Err()
	case <-time.After(15 * time.Second):
		slog.Error("Tradier: Timeout waiting for connection to be ready")
		return false, fmt.Errorf("timeout waiting for connection")
	}

	// Additional stability delay after ready signal to ensure reader is fully operational
	time.Sleep(200 * time.Millisecond)

	// CRITICAL FIX: Read session ID and connection at the EXACT moment of sending
	// This prevents race condition where recovery creates new session while we're subscribing
	t.streamMutex.RLock()
	currentSessionID := t.sessionID
	conn := t.streamConnection
	t.streamMutex.RUnlock()

	if conn == nil {
		slog.Error("Tradier: WebSocket connection is nil during subscription")
		return false, fmt.Errorf("WebSocket connection is nil")
	}

	if currentSessionID == "" {
		slog.Error("Tradier: Session ID is empty during subscription")
		return false, fmt.Errorf("session ID is empty")
	}

	// Create subscription payload with the CURRENT session ID
	payload := map[string]interface{}{
		"symbols":   symbols,
		"sessionid": currentSessionID,
		"filter":    []string{"quote", "trade"},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal subscription payload: %w", err)
	}

	slog.Info("Tradier: Sending subscription", "symbols", symbols, "sessionID", currentSessionID)

	// Use write lock to ensure thread-safe write
	if err := t.writeMessage(websocket.TextMessage, payloadBytes); err != nil {
		slog.Error("Tradier: Failed to send subscription message", "error", err)
		// Mark as disconnected on write failure like Python implementation
		t.streamMutex.Lock()
		t.IsConnected = false
		t.streamMutex.Unlock()
		return false, fmt.Errorf("failed to send subscription message: %w", err)
	}

	// Track subscription time to prevent premature recovery
	t.lastSubscriptionTime = time.Now()

	// IMPORTANT: Replace the entire subscription set like Python implementation
	// Clear existing subscriptions first
	for symbol := range t.subscribedSymbols {
		delete(t.subscribedSymbols, symbol)
	}

	// Add new subscriptions
	for _, symbol := range symbols {
		t.subscribedSymbols[symbol] = true
	}

	slog.Info("Tradier: Successfully subscribed to symbols", "count", len(symbols), "totalSubscribed", len(t.subscribedSymbols))
	return true, nil
}

// UnsubscribeFromSymbols unsubscribes from real-time data for symbols.
// Exact conversion of Python unsubscribe_from_symbols method.
func (t *TradierProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	slog.Info("Tradier: Unsubscribing from symbols", "count", len(symbols))

	// Remove from subscribed symbols tracking
	for _, symbol := range symbols {
		delete(t.subscribedSymbols, symbol)
	}

	slog.Info("Tradier: Successfully unsubscribed from symbols", "count", len(symbols))
	return true, nil
}

// GetSubscribedSymbols gets currently subscribed symbols.
// Exact conversion of Python get_subscribed_symbols method.
func (t *TradierProvider) GetSubscribedSymbols() map[string]bool {
	// Return a copy to prevent external modification
	result := make(map[string]bool)
	for symbol, subscribed := range t.subscribedSymbols {
		result[symbol] = subscribed
	}
	return result
}

// IsStreamingConnected checks if streaming connection is active.
// Exact conversion of Python is_streaming_connected method with WebSocket state check.
func (t *TradierProvider) IsStreamingConnected() bool {
	t.streamMutex.RLock()
	defer t.streamMutex.RUnlock()

	return t.IsConnected &&
		t.sessionID != "" &&
		t.streamConnection != nil
}

// Ping sends a heartbeat message to verify connection health.
func (t *TradierProvider) Ping(ctx context.Context) error {
	t.streamMutex.RLock()
	conn := t.streamConnection
	isConnected := t.IsConnected
	t.streamMutex.RUnlock()

	if !isConnected || conn == nil {
		return fmt.Errorf("streaming not connected")
	}

	// Send WebSocket ping frame
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
		return fmt.Errorf("failed to send ping: %w", err)
	}

	return nil
}

// writeMessage safely writes a message to the WebSocket connection.
func (t *TradierProvider) writeMessage(messageType int, data []byte) error {
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	t.streamMutex.RLock()
	conn := t.streamConnection
	t.streamMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket connection is nil")
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(messageType, data)
}

// HealthCheck performs a health check on the provider.
func (t *TradierProvider) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	// Test credentials to verify provider health
	return t.TestCredentials(ctx)
}

// GetName returns the provider name.
func (t *TradierProvider) GetName() string {
	return "tradier"
}

// SetStreamingQueue sets the queue for streaming data.
// Exact conversion of Python set_streaming_queue method.
func (t *TradierProvider) SetStreamingQueue(queue chan *models.MarketData) {
	t.streamingQueue = queue
}

// SetStreamingCache sets the streaming cache for this provider.
// Exact conversion of Python set_streaming_cache method.
func (t *TradierProvider) SetStreamingCache(cache base.StreamingCache) {
	t.streamingCache = cache
}

// GetNextMarketDate gets the next trading date.
// Exact conversion of Python get_next_market_date method.
func (t *TradierProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/calendar", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	resp, err := t.client.Get(ctx, endpoint, headers, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get market calendar: %w", err)
	}

	var response struct {
		Calendar struct {
			Days struct {
				Day interface{} `json:"day"`
			} `json:"days"`
		} `json:"calendar"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return "", fmt.Errorf("failed to parse calendar response: %w", err)
	}

	// Handle both single day (object) and multiple days (array)
	var days []map[string]interface{}
	switch v := response.Calendar.Days.Day.(type) {
	case map[string]interface{}:
		days = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if day, ok := item.(map[string]interface{}); ok {
				days = append(days, day)
			}
		}
	}

	for _, day := range days {
		if status, ok := day["status"].(string); ok && status == "open" {
			if date, ok := day["date"].(string); ok {
				return date, nil
			}
		}
	}

	// Fallback to today
	return time.Now().Format("2006-01-02"), nil
}

// Helper methods

func (t *TradierProvider) isOptionSymbol(symbol string) bool {
	return len(symbol) > 10 && (strings.Contains(symbol, "C") || strings.Contains(symbol, "P")) &&
		len(symbol) >= 15 && strings.ContainsAny(symbol[len(symbol)-8:], "0123456789")
}

type ParsedOption struct {
	Underlying string
	Type       string
	Strike     float64
	Expiry     string
}

func (t *TradierProvider) parseOptionSymbol(symbol string) *ParsedOption {
	if len(symbol) < 15 {
		return nil
	}

	// Find date part (6 consecutive digits)
	for i := 0; i <= len(symbol)-15; i++ {
		datePart := symbol[i : i+6]
		if isAllDigits(datePart) {
			root := symbol[:i]
			optionType := symbol[i+6]
			strikePart := symbol[i+7 : i+15]

			// Parse expiry date
			year := 2000 + parseInt(datePart[:2])
			month := parseInt(datePart[2:4])
			day := parseInt(datePart[4:6])
			expiryDate := fmt.Sprintf("%04d-%02d-%02d", year, month, day)

			// Parse strike price
			strikePrice := float64(parseInt(strikePart)) / 1000

			optType := "call"
			if optionType == 'P' {
				optType = "put"
			}

			// Handle special case: NDXP weekly options have NDX as underlying
			underlying := root
			if root == "NDXP" {
				underlying = "NDX"
			}

			return &ParsedOption{
				Underlying: underlying,
				Type:       optType,
				Strike:     strikePrice,
				Expiry:     expiryDate,
			}
		}
	}

	return nil
}

func (t *TradierProvider) sortSecuritiesByRelevance(securities []map[string]interface{}, query string) {
	queryUpper := strings.ToUpper(query)

	// Simple bubble sort by relevance
	for i := 0; i < len(securities)-1; i++ {
		for j := i + 1; j < len(securities); j++ {
			score1 := t.getRelevanceScore(securities[i], queryUpper)
			score2 := t.getRelevanceScore(securities[j], queryUpper)

			if score1 > score2 {
				securities[i], securities[j] = securities[j], securities[i]
			}
		}
	}
}

func (t *TradierProvider) getRelevanceScore(security map[string]interface{}, queryUpper string) int {
	symbol, _ := security["symbol"].(string)
	symbol = strings.ToUpper(symbol)

	// Exact match gets highest priority
	if symbol == queryUpper {
		return 0
	}

	// Starts with query gets second priority
	if strings.HasPrefix(symbol, queryUpper) {
		return 1000 + len(symbol)
	}

	// Contains query gets third priority
	if strings.Contains(symbol, queryUpper) {
		return 2000 + strings.Index(symbol, queryUpper) + len(symbol)
	}

	// Description contains query gets fourth priority
	description, _ := security["description"].(string)
	description = strings.ToUpper(description)
	if strings.Contains(description, queryUpper) {
		return 3000 + strings.Index(description, queryUpper) + len(description)
	}

	// Everything else
	return 4000
}

func (t *TradierProvider) transformSymbolSearchResult(rawSecurity map[string]interface{}) *models.SymbolSearchResult {
	symbol, _ := rawSecurity["symbol"].(string)
	description, _ := rawSecurity["description"].(string)
	exchange, _ := rawSecurity["exchange"].(string)
	secType, _ := rawSecurity["type"].(string)

	// Handle nil values
	if description == "" {
		description = ""
	}
	if exchange == "" {
		exchange = ""
	}
	if secType == "" {
		secType = ""
	}

	return &models.SymbolSearchResult{
		Symbol:      symbol,
		Description: description,
		Exchange:    exchange,
		Type:        secType,
	}
}

func (t *TradierProvider) getIntradayBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/timesales", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	intervalMap := map[string]string{
		"1m":  "1min",
		"5m":  "5min",
		"15m": "15min",
		"30m": "15min",
		"1h":  "15min",
		"4h":  "15min",
	}

	params := make(map[string]string)
	params["symbol"] = symbol
	params["interval"] = intervalMap[timeframe]
	params["session_filter"] = "all"

	if startDate != nil {
		params["start"] = fmt.Sprintf("%s 09:30", *startDate)
	}
	if endDate != nil {
		params["end"] = fmt.Sprintf("%s 16:00", *endDate)
	}

	resp, err := t.client.Get(ctx, endpoint, headers, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get intraday bars: %w", err)
	}

	var response struct {
		Series struct {
			Data []map[string]interface{} `json:"data"`
		} `json:"series"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse intraday bars response: %w", err)
	}

	var result []map[string]interface{}
	for _, bar := range response.Series.Data {
		timestamp, _ := bar["timestamp"].(float64)
		timeStr := time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04")
		if timeStr == "" {
			timeStr, _ = bar["time"].(string)
		}

		result = append(result, map[string]interface{}{
			"time":   timeStr,
			"open":   getFloat(bar, "open"),
			"high":   getFloat(bar, "high"),
			"low":    getFloat(bar, "low"),
			"close":  getFloat(bar, "close"),
			"volume": getInt(bar, "volume"),
		})
	}

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}

	return result, nil
}

func (t *TradierProvider) getDailyBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/history", t.baseURL)

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}

	intervalMap := map[string]string{
		"D": "daily",
		"W": "weekly",
		"M": "monthly",
	}

	params := make(map[string]string)
	params["symbol"] = symbol
	params["interval"] = intervalMap[timeframe]
	params["session_filter"] = "all"

	if startDate != nil {
		params["start"] = *startDate
	}
	if endDate != nil {
		params["end"] = *endDate
	}

	resp, err := t.client.Get(ctx, endpoint, headers, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily bars: %w", err)
	}

	var response struct {
		History interface{} `json:"history"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse daily bars response: %w", err)
	}

	var bars []map[string]interface{}
	if historyMap, ok := response.History.(map[string]interface{}); ok {
		if dayData, ok := historyMap["day"].([]interface{}); ok {
			for _, item := range dayData {
				if bar, ok := item.(map[string]interface{}); ok {
					bars = append(bars, bar)
				}
			}
		}
	}

	var result []map[string]interface{}
	for _, bar := range bars {
		result = append(result, map[string]interface{}{
			"time":   getString(bar, "date"),
			"open":   getFloat(bar, "open"),
			"high":   getFloat(bar, "high"),
			"low":    getFloat(bar, "low"),
			"close":  getFloat(bar, "close"),
			"volume": getInt(bar, "volume"),
		})
	}

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}

	return result, nil
}

// Utility functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getStringOrDefault(data map[string]interface{}, key, defaultValue string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return defaultValue
}

func getFloatOrDefault(data map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := data[key].(float64); ok {
		return val
	}
	return defaultValue
}

func getFloatPtr(data map[string]interface{}, key string) *float64 {
	if val, ok := data[key].(float64); ok {
		return &val
	}
	return nil
}

func getQuantity(data map[string]interface{}) float64 {
	// Try "qty" first (Python uses this)
	if val, ok := data["qty"].(float64); ok {
		return val
	}
	if val, ok := data["qty"].(int); ok {
		return float64(val)
	}

	// Try "quantity" as fallback (frontend sends this)
	if val, ok := data["quantity"].(float64); ok {
		return val
	}
	if val, ok := data["quantity"].(int); ok {
		return float64(val)
	}

	// Try to parse from string
	if val, ok := data["qty"].(string); ok {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	if val, ok := data["quantity"].(string); ok {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}

	return 0
}

func getFloat(data map[string]interface{}, key string) float64 {
	if val, ok := data[key].(float64); ok {
		return val
	}
	// Try to convert from other numeric types
	if val, ok := data[key].(int); ok {
		return float64(val)
	}
	if val, ok := data[key].(int64); ok {
		return float64(val)
	}
	if val, ok := data[key].(float32); ok {
		return float64(val)
	}
	// Try to parse from string
	if val, ok := data[key].(string); ok {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func getInt(data map[string]interface{}, key string) int {
	if val, ok := data[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}

func getStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func convertLegsToOrderLegs(legs []map[string]interface{}) []models.OrderLeg {
	var orderLegs []models.OrderLeg
	for _, leg := range legs {
		symbol, _ := leg["symbol"].(string)
		side, _ := leg["side"].(string)
		qty, _ := leg["qty"].(float64)

		orderLegs = append(orderLegs, models.OrderLeg{
			Symbol: symbol,
			Side:   side,
			Qty:    qty,
		})
	}
	return orderLegs
}

// sendToCacheOrQueue sends market data to cache if available, otherwise to queue.
// Exact conversion of Python _send_to_cache_or_queue method.
func (t *TradierProvider) sendToCacheOrQueue(marketData *models.MarketData) error {
	// CRITICAL: Send to cache first (this triggers WebSocket broadcasts)
	if t.streamingCache != nil {
		if err := t.streamingCache.Update(marketData); err != nil {
			slog.Error("Tradier: Failed to update streaming cache", "error", err, "symbol", marketData.Symbol)
			return err
		}
		return nil // Cache update successful - this will trigger WebSocket broadcast
	}

	// Fallback: Send to queue if cache not available (legacy support)
	if t.streamingQueue != nil {
		select {
		case t.streamingQueue <- marketData:
			return nil
		case <-time.After(time.Second):
			slog.Error("Tradier: Queue put timeout - queue may be full", "symbol", marketData.Symbol)
			return fmt.Errorf("queue put timeout - queue may be full")
		default:
			slog.Error("Tradier: Queue full, skipping data point", "symbol", marketData.Symbol)
			return fmt.Errorf("queue full, skipping data point")
		}
	}

	slog.Error("Tradier: No streaming queue or cache available!")
	return fmt.Errorf("no streaming queue or cache available")
}

// ======================== Account Stream Client ========================

type AccountStreamClient struct {
	provider        *TradierProvider
	uri             string
	sessionID       string
	conn            *websocket.Conn
	eventCallback   func(*models.OrderEvent)
	mu              sync.RWMutex
	IsConnected     bool
	shutdownEvent   chan struct{}
	connectionReady chan struct{}
	streamCancel    context.CancelFunc

	writeLock sync.Mutex
	recvLock  sync.Mutex

	messageChan chan []byte
	errorChan   chan error

	maxReconnectDelay time.Duration
	initialDelay      time.Duration
	reconnectionCount int
}

func NewAccountStreamClient(provider *TradierProvider, uri string) *AccountStreamClient {
	return &AccountStreamClient{
		provider:          provider,
		uri:               uri,
		shutdownEvent:     make(chan struct{}),
		connectionReady:   make(chan struct{}),
		messageChan:       make(chan []byte, 100),
		errorChan:         make(chan error, 10),
		maxReconnectDelay: 5 * time.Minute,
		initialDelay:      1 * time.Second,
	}
}

func (c *AccountStreamClient) SetEventCallback(callback func(*models.OrderEvent)) {
	c.eventCallback = callback
}

func (c *AccountStreamClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	slog.Info("[ORDER-EVENT] Starting account stream client...")

	if c.IsConnected {
		slog.Info("[ORDER-EVENT] Account stream already connected")
		return nil
	}

	c.IsConnected = false

	select {
	case <-c.connectionReady:
		c.connectionReady = make(chan struct{})
	default:
	}

	if err := c.createAccountSession(ctx); err != nil {
		return fmt.Errorf("failed to create account session: %w", err)
	}

	if err := c.connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.IsConnected = true

	c.messageChan = make(chan []byte, 100)
	c.errorChan = make(chan error, 10)

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPingHandler(func(appData string) error {
		return c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})
	c.conn.SetPongHandler(func(appData string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go c.keepalivePing(ctx)

	streamCtx, cancel := context.WithCancel(ctx)
	c.streamCancel = cancel

	go c.streamHandler(streamCtx)

	select {
	case <-c.connectionReady:
	case <-time.After(5 * time.Second):
		slog.Warn("[ORDER-EVENT] Timeout waiting for account stream to be ready")
	}

	slog.Info("[ORDER-EVENT] Account stream client started", "uri", c.uri)
	return nil
}

func (c *AccountStreamClient) keepalivePing(ctx context.Context) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdownEvent:
			return
		case <-ticker.C:
			c.writeLock.Lock()
			if c.conn != nil {
				if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					slog.Warn("[ORDER-EVENT] Failed to send ping", "error", err)
				}
			}
			c.writeLock.Unlock()
		}
	}
}

func (c *AccountStreamClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	slog.Info("[ORDER-EVENT] Stopping account stream client...")

	if c.streamCancel != nil {
		c.streamCancel()
		c.streamCancel = nil
	}

	select {
	case c.shutdownEvent <- struct{}{}:
		slog.Info("[ORDER-EVENT] Shutdown signal sent")
	default:
	}

	c.writeLock.Lock()
	if c.conn != nil {
		slog.Info("[ORDER-EVENT] Closing WebSocket connection")
		c.conn.Close()
		c.conn = nil
	}
	c.writeLock.Unlock()

	c.IsConnected = false
	slog.Info("Account stream client stopped")
}

func (c *AccountStreamClient) createAccountSession(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/accounts/events/session", c.provider.baseURL)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", c.provider.apiKey),
		"Accept":        "application/json",
	}

	resp, err := c.provider.client.Post(ctx, url, nil, headers)
	if err != nil {
		return fmt.Errorf("failed to create account session: %w", err)
	}

	var response struct {
		Stream struct {
			SessionID string `json:"sessionid"`
		} `json:"stream"`
	}

	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return fmt.Errorf("failed to parse account session response: %w", err)
	}

	if response.Stream.SessionID == "" {
		return fmt.Errorf("no session ID in account session response")
	}

	c.sessionID = response.Stream.SessionID
	slog.Info("Tradier account session created", "sessionID", c.sessionID)
	return nil
}

func (c *AccountStreamClient) connect(ctx context.Context) error {
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", c.provider.apiKey))

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.uri, header)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	c.conn = conn

	subscribeMsg := map[string]interface{}{
		"events":          []string{"order"},
		"sessionid":       c.sessionID,
		"excludeAccounts": []string{},
	}

	msgBytes, _ := json.Marshal(subscribeMsg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send subscribe message: %w", err)
	}

	slog.Info("Connected to Tradier account events WebSocket")
	return nil
}

func (c *AccountStreamClient) streamHandler(ctx context.Context) {
	slog.Info("[ORDER-EVENT] Stream handler started")

	var readerDone chan struct{}
	reconnectAttempts := 0

	defer func() {
		if r := recover(); r != nil {
			slog.Error("[ORDER-EVENT] Stream processor panic recovered", "panic", r)
		}

		if readerDone != nil {
			slog.Info("[ORDER-EVENT] Waiting for reader goroutine to exit...")
			select {
			case <-readerDone:
				slog.Info("[ORDER-EVENT] Reader goroutine exited")
			case <-time.After(5 * time.Second):
				slog.Warn("[ORDER-EVENT] Timed out waiting for reader goroutine")
			}
		}

		slog.Info("[ORDER-EVENT] Stream handler stopped")
	}()

	readerStarted := false
	readySignaled := false

	for {
		select {
		case <-ctx.Done():
			slog.Info("[ORDER-EVENT] Stream processor cancelled")
			return
		case <-c.shutdownEvent:
			slog.Info("[ORDER-EVENT] Stream processor shutdown requested")
			return
		default:
		}

		if c.IsConnected && c.conn != nil {
			if !readerStarted {
				readerDone = make(chan struct{})
				go c.readerGoroutine(ctx, readerDone)
				readerStarted = true

				time.Sleep(200 * time.Millisecond)

				if !readySignaled {
					select {
					case <-c.connectionReady:
					default:
						close(c.connectionReady)
						readySignaled = true
						slog.Info("[ORDER-EVENT] Stream processor ready")
					}
				}

				// Reset reconnect attempts on successful connection
				reconnectAttempts = 0
			}

			select {
			case <-ctx.Done():
				return
			case <-c.shutdownEvent:
				return
			case err := <-c.errorChan:
				slog.Warn("[ORDER-EVENT] Reader error, will attempt reconnection", "error", err)
				c.mu.Lock()
				c.IsConnected = false
				if c.conn != nil {
					c.writeLock.Lock()
					c.conn.Close()
					c.writeLock.Unlock()
					c.conn = nil
				}
				c.mu.Unlock()

				// Wait for reader to exit
				if readerDone != nil {
					select {
					case <-readerDone:
					case <-time.After(2 * time.Second):
					}
				}
				readerStarted = false
				readerDone = nil

				// Attempt reconnection with exponential backoff
				if c.attemptReconnect(ctx, &reconnectAttempts) {
					continue // Successfully reconnected, continue processing
				}
				return // Failed to reconnect after max attempts

			case message := <-c.messageChan:
				c.handleMessage(message)
				continue
			default:
			}
		} else {
			// Not connected - attempt to reconnect
			if c.attemptReconnect(ctx, &reconnectAttempts) {
				continue
			}
			// Brief sleep to avoid busy loop
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// attemptReconnect tries to reconnect with exponential backoff
// Returns true if reconnection succeeded, false if should give up
func (c *AccountStreamClient) attemptReconnect(ctx context.Context, attempts *int) bool {
	maxAttempts := 10

	if *attempts >= maxAttempts {
		slog.Error("[ORDER-EVENT] Max reconnection attempts reached, giving up", "attempts", *attempts)
		return false
	}

	*attempts++

	// Calculate backoff delay: 1s, 2s, 4s, 8s, 16s, 30s (capped)
	backoffSeconds := 1 << uint(*attempts-1) // 2^(attempts-1)
	if backoffSeconds > 30 {
		backoffSeconds = 30
	}
	delay := time.Duration(backoffSeconds) * time.Second

	slog.Info("[ORDER-EVENT] Attempting reconnection", "attempt", *attempts, "maxAttempts", maxAttempts, "delay", delay)

	select {
	case <-ctx.Done():
		return false
	case <-c.shutdownEvent:
		return false
	case <-time.After(delay):
	}

	// Create new session
	if err := c.createAccountSession(ctx); err != nil {
		slog.Error("[ORDER-EVENT] Failed to create session during reconnect", "error", err, "attempt", *attempts)
		return c.attemptReconnect(ctx, attempts) // Retry
	}

	// Connect
	if err := c.connect(ctx); err != nil {
		slog.Error("[ORDER-EVENT] Failed to connect during reconnect", "error", err, "attempt", *attempts)
		return c.attemptReconnect(ctx, attempts) // Retry
	}

	c.mu.Lock()
	c.IsConnected = true
	// Recreate channels for new connection
	c.messageChan = make(chan []byte, 100)
	c.errorChan = make(chan error, 10)

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPingHandler(func(appData string) error {
		return c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})
	c.conn.SetPongHandler(func(appData string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	c.mu.Unlock()

	slog.Info("[ORDER-EVENT] Successfully reconnected", "attempt", *attempts, "totalReconnections", c.reconnectionCount+1)
	c.reconnectionCount++
	return true
}

func (c *AccountStreamClient) readerGoroutine(ctx context.Context, done chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[ORDER-EVENT] Reader panic recovered", "panic", r)
		}
		slog.Debug("[ORDER-EVENT] Reader goroutine exiting")
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			slog.Debug("[ORDER-EVENT] Reader context cancelled, exiting")
			return
		case <-c.shutdownEvent:
			slog.Debug("[ORDER-EVENT] Reader shutdown requested")
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				slog.Debug("[ORDER-EVENT] Connection is nil, reader exiting")
				return
			}

			c.recvLock.Lock()
			_, messageBytes, err := conn.ReadMessage()
			c.recvLock.Unlock()

			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					slog.Error("[ORDER-EVENT] Reader encountered error", "error", err)
					select {
					case c.errorChan <- err:
					default:
						slog.Warn("[ORDER-EVENT] Error channel full")
					}
					return
				}
			}

			select {
			case c.messageChan <- messageBytes:
			default:
				slog.Warn("[ORDER-EVENT] Message channel full, dropping message")
			}
		}
	}
}

func (c *AccountStreamClient) handleMessage(message []byte) {
	var rawMsg map[string]interface{}
	if err := json.Unmarshal(message, &rawMsg); err != nil {
		slog.Error("[ORDER-EVENT] Failed to parse as JSON", "error", err)
		return
	}

	var orderEvent models.OrderEvent
	if err := json.Unmarshal(message, &orderEvent); err != nil {
		slog.Error("[ORDER-EVENT] Failed to parse as OrderEvent", "error", err)
		return
	}

	// Log order events at info level (these are important)
	if orderEvent.Event == "order" {
		slog.Info("[ORDER-EVENT] Order event received",
			"id", orderEvent.ID,
			"status", orderEvent.Status,
			"type", orderEvent.Type)
	}

	c.mu.RLock()
	callback := c.eventCallback
	c.mu.RUnlock()

	if callback != nil {
		callback(&orderEvent)
	}
}

// ======================== Order Event Streaming Methods ========================

// StartAccountStream starts the Tradier account events WebSocket stream
func (t *TradierProvider) StartAccountStream(ctx context.Context) error {
	if t.accountStream != nil {
		return nil
	}

	t.accountStreamURL = "wss://ws.tradier.com/v1/accounts/events"
	if t.IsPaperAccount() {
		t.accountStreamURL = "wss://sandbox-ws.tradier.com/v1/accounts/events"
	}

	t.accountStream = NewAccountStreamClient(t, t.accountStreamURL)

	if t.orderEventCallback != nil {
		t.accountStream.SetEventCallback(t.orderEventCallback)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	t.accountStreamCancel = cancel

	return t.accountStream.Start(streamCtx)
}

// StopAccountStream stops the Tradier account events WebSocket stream
func (t *TradierProvider) StopAccountStream() {
	if t.accountStreamCancel != nil {
		t.accountStreamCancel()
		t.accountStreamCancel = nil
	}
	if t.accountStream != nil {
		t.accountStream.Stop()
		t.accountStream = nil
	}
}

// SetOrderEventCallback sets the callback for receiving order events
func (t *TradierProvider) SetOrderEventCallback(cb func(*models.OrderEvent)) {
	t.orderEventCallback = cb
	if t.accountStream != nil {
		t.accountStream.SetEventCallback(cb)
	}
}

// IsAccountStreamConnected checks if the account stream is connected
func (t *TradierProvider) IsAccountStreamConnected() bool {
	return t.accountStream != nil && t.accountStream.IsConnected
}
