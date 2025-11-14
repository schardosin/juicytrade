package tradier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/utils"
)

// TradierProvider implements the Provider interface for Tradier Brokerage API.
// Exact conversion of Python TradierProvider class.
type TradierProvider struct {
	accountID string
	apiKey    string
	baseURL   string
	streamURL string
	client    *utils.HTTPClient
}

// NewTradierProvider creates a new Tradier provider instance.
// Exact conversion of Python TradierProvider.__init__ method.
func NewTradierProvider(accountID, apiKey, baseURL, streamURL string) *TradierProvider {
	return &TradierProvider{
		accountID: accountID,
		apiKey:    apiKey,
		baseURL:   baseURL,
		streamURL: streamURL,
		client:    utils.NewHTTPClient(),
	}
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
	
	params := make(map[string]string)
	params["symbol"] = symbol
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
	
	return enhancedDates, nil
}

// GetOptionsChainBasic gets basic options chain data.
// Exact conversion of Python get_options_chain_basic method.
func (t *TradierProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	endpoint := fmt.Sprintf("%s/v1/markets/options/chains", t.baseURL)
	
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", t.apiKey),
		"Accept":        "application/json",
	}
	
	// Determine API symbol to use
	apiSymbol := symbol
	if underlyingSymbol != nil {
		apiSymbol = *underlyingSymbol
	} else if len(symbol) > 3 && strings.HasSuffix(symbol, "W") || strings.HasSuffix(symbol, "P") {
		apiSymbol = symbol[:len(symbol)-1]
	}
	
	params := make(map[string]string)
	params["symbol"] = apiSymbol
	params["expiration"] = expiry
	params["greeks"] = "false"
	
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
		
		if root == upperSymbol {
			filteredOptions = append(filteredOptions, option)
		}
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
	
	return &models.OptionContract{
		Symbol:           symbol,
		UnderlyingSymbol: underlying,
		ExpirationDate:   expirationDate,
		StrikePrice:      strike,
		Type:             strings.ToLower(optionType),
		Bid:              bid,
		Ask:              ask,
		ClosePrice:       closePrice,
		Volume:           volume,
		OpenInterest:     openInterest,
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
	for i := 0; i < len(strikes)-1; i++ {
		for j := i + 1; j < len(strikes); j++ {
			if strikes[i] > strikes[j] {
				strikes[i], strikes[j] = strikes[j], strikes[i]
			}
		}
	}
	
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
	
	return filtered
}

// GetOptionsGreeksBatch gets Greeks for multiple option symbols.
// Exact conversion of Python get_options_greeks_batch method.
func (t *TradierProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	// Group symbols by underlying and expiration
	symbolGroups := make(map[string]struct {
		underlying string
		expiry     string
		symbols    []string
	})
	
	for _, optionSymbol := range optionSymbols {
		parsed := t.parseOptionSymbol(optionSymbol)
		if parsed == nil {
			continue
		}
		
		key := fmt.Sprintf("%s_%s", parsed.Underlying, parsed.Expiry)
		group := symbolGroups[key]
		group.underlying = parsed.Underlying
		group.expiry = parsed.Expiry
		group.symbols = append(group.symbols, optionSymbol)
		symbolGroups[key] = group
	}
	
	// Fetch Greeks for each group
	greeksData := make(map[string]map[string]interface{})
	
	for _, group := range symbolGroups {
		// Calculate strike count needed
		strikeCount := max(len(group.symbols)*2, 50)
		
		contracts, err := t.GetOptionsChainBasic(ctx, group.underlying, group.expiry, nil, strikeCount, nil, nil)
		if err != nil {
			slog.Error("Failed to get options chain for Greeks", "underlying", group.underlying, "expiry", group.expiry, "error", err)
			continue
		}
		
		// Extract Greeks for requested symbols
		for _, contract := range contracts {
			for _, requestedSymbol := range group.symbols {
				if contract.Symbol == requestedSymbol {
					greeksData[contract.Symbol] = map[string]interface{}{
						"delta":              contract.Delta,
						"theta":              contract.Theta,
						"gamma":              contract.Gamma,
						"vega":               contract.Vega,
						"implied_volatility": contract.ImpliedVolatility,
					}
				}
			}
		}
	}
	
	return greeksData, nil
}

// GetOptionsChainSmart gets smart options chain data.
// Exact conversion of Python get_options_chain_smart method.
func (t *TradierProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	if includeGreeks {
		// Use full options chain with Greeks - not implemented in basic version
		return t.GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, atmRange, nil, nil)
	}
	
	return t.GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, atmRange, nil, nil)
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
	
	var response struct {
		Orders struct {
			Order interface{} `json:"order"`
		} `json:"orders"`
	}
	
	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}
	
	// Handle both single order (object) and multiple orders (array)
	var orders []map[string]interface{}
	switch v := response.Orders.Order.(type) {
	case map[string]interface{}:
		orders = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if order, ok := item.(map[string]interface{}); ok {
				orders = append(orders, order)
			}
		}
	}
	
	var result []*models.Order
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
	if avgPrice, ok := rawOrder["avg_fill_price"].(float64); ok && avgPrice > 0 {
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
		AccountID:                t.accountID,
		AccountNumber:            accountNumberPtr,
		Status:                   "active",
		Currency:                 "USD",
		BuyingPower:              buyingPower,
		Cash:                     cashAvailable,
		PortfolioValue:           portfolioValue,
		Equity:                   equity,
		DayTradingBuyingPower:    buyingPower,
		RegtBuyingPower:          stockBuyingPower,
		OptionsBuyingPower:       optionsBuyingPower,
		PatternDayTrader:         &isPDT,
		LongMarketValue:          longMarketValue,
		ShortMarketValue:         shortMarketValue,
		InitialMargin:            initialMargin,
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

// ConnectStreaming connects to streaming data (placeholder implementation).
// Tradier streaming would require WebSocket implementation.
func (t *TradierProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	// Placeholder - Tradier streaming would require WebSocket implementation
	// For now, return false to indicate streaming not implemented
	return false, nil
}

// DisconnectStreaming disconnects from streaming data (placeholder implementation).
func (t *TradierProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	// Placeholder - Tradier streaming would require WebSocket implementation
	return false, nil
}

// SubscribeToSymbols subscribes to real-time data for symbols (placeholder implementation).
func (t *TradierProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	// Placeholder - Tradier streaming would require WebSocket implementation
	return false, nil
}

// UnsubscribeFromSymbols unsubscribes from real-time data for symbols (placeholder implementation).
func (t *TradierProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	// Placeholder - Tradier streaming would require WebSocket implementation
	return false, nil
}

// GetSubscribedSymbols gets currently subscribed symbols (placeholder implementation).
func (t *TradierProvider) GetSubscribedSymbols() map[string]bool {
	// Placeholder - return empty map
	return make(map[string]bool)
}

// IsStreamingConnected checks if streaming connection is active (placeholder implementation).
func (t *TradierProvider) IsStreamingConnected() bool {
	// Placeholder - streaming not implemented
	return false
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

// SetStreamingQueue sets the queue for streaming data (placeholder implementation).
func (t *TradierProvider) SetStreamingQueue(queue chan *models.MarketData) {
	// Placeholder - streaming not implemented
}

// SetStreamingCache sets the streaming cache for this provider (placeholder implementation).
func (t *TradierProvider) SetStreamingCache(cache base.StreamingCache) {
	// Placeholder - streaming not implemented
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
			
			return &ParsedOption{
				Underlying: root,
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

func getFloat(data map[string]interface{}, key string) float64 {
	if val, ok := data[key].(float64); ok {
		return val
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
