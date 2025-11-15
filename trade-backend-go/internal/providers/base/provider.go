package base

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"trade-backend-go/internal/models"
)

// Provider defines the interface that all trading data providers must implement.
// This is an exact conversion of the Python BaseProvider abstract class.
type Provider interface {
	// === Market Data Methods ===
	
	// GetStockQuote gets the latest stock quote for a symbol.
	// Exact conversion of Python get_stock_quote method.
	GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error)
	
	// GetStockQuotes gets stock quotes for multiple symbols.
	// Exact conversion of Python get_stock_quotes method.
	GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error)
	
	// GetExpirationDates gets available expiration dates for options on a symbol with universal enhanced structure.
	// Exact conversion of Python get_expiration_dates method.
	GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error)
	
	// GetOptionsChainBasic gets basic options chain (no Greeks) for fast loading, ATM-focused.
	// Exact conversion of Python get_options_chain_basic method.
	GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error)
	
	// GetOptionsGreeksBatch gets Greeks for multiple option symbols in batch.
	// Exact conversion of Python get_options_greeks_batch method.
	GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error)
	
	// GetOptionsChainSmart gets smart options chain with configurable loading.
	// Exact conversion of Python get_options_chain_smart method.
	GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error)
	
	// GetNextMarketDate gets the next trading date.
	// Exact conversion of Python get_next_market_date method.
	GetNextMarketDate(ctx context.Context) (string, error)
	
	// LookupSymbols searches for symbols matching the query.
	// Exact conversion of Python lookup_symbols method.
	LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error)
	
	// GetHistoricalBars gets historical OHLCV bars for charting.
	// Exact conversion of Python get_historical_bars method.
	GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error)
	
	// === Account & Portfolio Methods ===
	
	// GetPositions gets all current positions.
	// Exact conversion of Python get_positions method.
	GetPositions(ctx context.Context) ([]*models.Position, error)
	
	// GetPositionsEnhanced gets enhanced positions with hierarchical grouping.
	// Exact conversion of Python get_positions_enhanced method.
	GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error)
	
	// GetOrders gets orders with optional status filter.
	// Exact conversion of Python get_orders method.
	GetOrders(ctx context.Context, status string) ([]*models.Order, error)
	
	// GetAccount gets account information including balance and buying power.
	// Exact conversion of Python get_account method.
	GetAccount(ctx context.Context) (*models.Account, error)
	
	// === Order Management Methods ===
	
	// PlaceOrder places a trading order.
	// Exact conversion of Python place_order method.
	PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error)
	
	// PlaceMultiLegOrder places a multi-leg trading order.
	// Exact conversion of Python place_multi_leg_order method.
	PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error)
	
	// PreviewOrder previews a trading order to get cost estimates and validation.
	// Exact conversion of Python preview_order method.
	PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error)
	
	// CancelOrder cancels an existing order.
	// Exact conversion of Python cancel_order method.
	CancelOrder(ctx context.Context, orderID string) (bool, error)
	
	// === Streaming Methods ===
	
	// ConnectStreaming connects to the provider's streaming service.
	// Exact conversion of Python connect_streaming method.
	ConnectStreaming(ctx context.Context) (bool, error)
	
	// DisconnectStreaming disconnects from the provider's streaming service.
	// Exact conversion of Python disconnect_streaming method.
	DisconnectStreaming(ctx context.Context) (bool, error)
	
	// SubscribeToSymbols subscribes to real-time data for symbols.
	// Exact conversion of Python subscribe_to_symbols method.
	SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error)
	
	// UnsubscribeFromSymbols unsubscribes from real-time data for symbols.
	// Exact conversion of Python unsubscribe_from_symbols method.
	UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error)
	
	// === Utility Methods ===
	
	// GetSubscribedSymbols gets currently subscribed symbols.
	// Exact conversion of Python get_subscribed_symbols method.
	GetSubscribedSymbols() map[string]bool
	
	// IsStreamingConnected checks if streaming connection is active.
	// Exact conversion of Python is_streaming_connected method.
	IsStreamingConnected() bool
	
	// HealthCheck performs a health check on the provider.
	// Exact conversion of Python health_check method.
	HealthCheck(ctx context.Context) (map[string]interface{}, error)
	
	// TestCredentials tests provider credentials by making a real API call.
	// Exact conversion of Python test_credentials method.
	TestCredentials(ctx context.Context) (map[string]interface{}, error)
	
	// === Provider Info Methods ===
	
	// GetName returns the provider name.
	GetName() string
	
	// SetStreamingQueue sets the queue for streaming data.
	// Exact conversion of Python set_streaming_queue method.
	SetStreamingQueue(queue chan *models.MarketData)
	
	// SetStreamingCache sets the streaming cache for this provider.
	// Exact conversion of Python set_streaming_cache method.
	SetStreamingCache(cache StreamingCache)
}

// StreamingCache interface for streaming data caching
// Matches the Python streaming cache interface
type StreamingCache interface {
	Update(marketData *models.MarketData) error
	AddUpdateCallback(callback func(*models.MarketData))
}

// BaseProviderImpl provides common functionality for all providers.
// This is an exact conversion of the Python BaseProvider class implementation.
type BaseProviderImpl struct {
	Name               string
	IsConnected        bool
	SubscribedSymbols  map[string]bool
	StreamingQueue     chan *models.MarketData
	StreamingCache     StreamingCache
	LastDataTime       *time.Time
	Logger             *slog.Logger
}

// NewBaseProvider creates a new base provider instance.
// Exact conversion of Python BaseProvider.__init__ method.
func NewBaseProvider(name string) *BaseProviderImpl {
	return &BaseProviderImpl{
		Name:              name,
		IsConnected:       false,
		SubscribedSymbols: make(map[string]bool),
		Logger:            slog.Default().With("provider", name),
	}
}

// GetName returns the provider name.
func (bp *BaseProviderImpl) GetName() string {
	return bp.Name
}

// SetStreamingQueue sets the queue for streaming data.
// Exact conversion of Python set_streaming_queue method.
func (bp *BaseProviderImpl) SetStreamingQueue(queue chan *models.MarketData) {
	bp.StreamingQueue = queue
}

// SetStreamingCache sets the streaming cache for this provider.
// Exact conversion of Python set_streaming_cache method.
func (bp *BaseProviderImpl) SetStreamingCache(cache StreamingCache) {
	bp.StreamingCache = cache
	
	// Add callback to update health tracking when data is received
	// Exact conversion of Python callback logic
	cache.AddUpdateCallback(func(marketData *models.MarketData) {
		now := time.Now()
		bp.LastDataTime = &now
	})
}

// GetSubscribedSymbols gets currently subscribed symbols.
// Exact conversion of Python get_subscribed_symbols method.
func (bp *BaseProviderImpl) GetSubscribedSymbols() map[string]bool {
	// Return a copy to prevent external modification (same as Python)
	result := make(map[string]bool)
	for symbol, subscribed := range bp.SubscribedSymbols {
		result[symbol] = subscribed
	}
	return result
}

// IsStreamingConnected checks if streaming connection is active.
// Exact conversion of Python is_streaming_connected method.
func (bp *BaseProviderImpl) IsStreamingConnected() bool {
	return bp.IsConnected
}

// HealthCheck performs a health check on the provider.
// Exact conversion of Python health_check method.
func (bp *BaseProviderImpl) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	subscribedCount := 0
	for _, subscribed := range bp.SubscribedSymbols {
		if subscribed {
			subscribedCount++
		}
	}
	
	return map[string]interface{}{
		"provider":           bp.Name,
		"connected":          bp.IsConnected,
		"subscribed_symbols": subscribedCount,
		"timestamp":          time.Now().Format(time.RFC3339),
	}, nil
}

// CreateAPIResponse creates standardized API responses.
// Exact conversion of Python _create_api_response method.
func (bp *BaseProviderImpl) CreateAPIResponse(success bool, data interface{}, errorMsg, message *string) *models.ApiResponse {
	return models.NewApiResponse(success, data, errorMsg, message)
}

// LogError logs errors with consistent formatting.
// Exact conversion of Python _log_error method.
func (bp *BaseProviderImpl) LogError(operation string, err error) {
	bp.Logger.Error(fmt.Sprintf("%s provider error in %s", bp.Name, operation), "error", err.Error())
}

// LogInfo logs info messages with consistent formatting.
// Exact conversion of Python _log_info method.
func (bp *BaseProviderImpl) LogInfo(message string) {
	bp.Logger.Info(fmt.Sprintf("%s provider: %s", bp.Name, message))
}

// Symbol Conversion Methods - exact conversions of Python methods
// Note: These would need the SymbolConverter utility to be implemented

// ConvertSymbolsToProviderFormat converts symbols to this provider's expected format.
// Exact conversion of Python convert_symbols_to_provider_format method.
func (bp *BaseProviderImpl) ConvertSymbolsToProviderFormat(symbols []string) []string {
	providerType := bp.GetProviderType()
	// TODO: Implement SymbolConverter.BatchConvertToProviderFormat
	// For now, return symbols as-is
	_ = providerType
	return symbols
}

// ConvertSymbolsToStandardFormat converts symbols from this provider's format to standard OCC format.
// Exact conversion of Python convert_symbols_to_standard_format method.
func (bp *BaseProviderImpl) ConvertSymbolsToStandardFormat(symbols []string) []string {
	// TODO: Implement SymbolConverter.BatchConvertToStandardOCC
	// For now, return symbols as-is
	return symbols
}

// ConvertSymbolToProviderFormat converts a single symbol to this provider's expected format.
// Exact conversion of Python convert_symbol_to_provider_format method.
func (bp *BaseProviderImpl) ConvertSymbolToProviderFormat(symbol string) string {
	providerType := bp.GetProviderType()
	// TODO: Implement SymbolConverter.ToProviderFormat
	// For now, return symbol as-is
	_ = providerType
	return symbol
}

// ConvertSymbolToStandardFormat converts a single symbol from this provider's format to standard OCC format.
// Exact conversion of Python convert_symbol_to_standard_format method.
func (bp *BaseProviderImpl) ConvertSymbolToStandardFormat(symbol string) string {
	// TODO: Implement SymbolConverter.ToStandardOCC
	// For now, return symbol as-is
	return symbol
}

// GetProviderType gets the provider type for symbol conversion.
// Exact conversion of Python _get_provider_type method.
func (bp *BaseProviderImpl) GetProviderType() string {
	return strings.ToLower(bp.Name)
}

// GetPositionsEnhanced provides a default implementation that converts regular positions to enhanced format.
// Individual providers can override this if they have native enhanced support.
// Exact conversion of Python fallback logic.
func (bp *BaseProviderImpl) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	// This is a default implementation that should be overridden by concrete providers
	// For now, return an error indicating the method needs to be implemented
	return nil, fmt.Errorf("GetPositionsEnhanced not implemented for provider %s", bp.Name)
}

// ConvertPositionsToEnhanced converts regular positions to enhanced hierarchical format
// Exact conversion of Python enhanced positions logic
func (bp *BaseProviderImpl) ConvertPositionsToEnhanced(positions []*models.Position) *models.EnhancedPositionsResponse {
	if len(positions) == 0 {
		return models.NewEnhancedPositionsResponse()
	}

	// Group positions by underlying symbol
	symbolGroups := make(map[string]*models.SymbolGroup)
	
	// First, group positions by underlying symbol and date acquired to identify strategies
	positionsBySymbolAndTime := make(map[string]map[string][]*models.Position)
	
	for _, position := range positions {
		underlyingSymbol := bp.getUnderlyingSymbol(position)
		
		if _, exists := positionsBySymbolAndTime[underlyingSymbol]; !exists {
			positionsBySymbolAndTime[underlyingSymbol] = make(map[string][]*models.Position)
		}
		
		// Group by acquisition time (truncated to minute for grouping related trades)
		timeKey := bp.getTimeGroupKey(position.DateAcquired)
		if _, exists := positionsBySymbolAndTime[underlyingSymbol][timeKey]; !exists {
			positionsBySymbolAndTime[underlyingSymbol][timeKey] = []*models.Position{}
		}
		
		positionsBySymbolAndTime[underlyingSymbol][timeKey] = append(positionsBySymbolAndTime[underlyingSymbol][timeKey], position)
	}
	
	// Process each symbol group
	for underlyingSymbol, timeGroups := range positionsBySymbolAndTime {
		// Collect all positions for this symbol to determine asset class
		var allPositionsForSymbol []*models.Position
		for _, groupPositions := range timeGroups {
			allPositionsForSymbol = append(allPositionsForSymbol, groupPositions...)
		}
		
		assetClass := bp.determineAssetClass(allPositionsForSymbol)
		symbolGroup := models.NewSymbolGroup(underlyingSymbol, assetClass)
		
		// Process each time group as a potential strategy
		for timeKey, groupPositions := range timeGroups {
			strategy := bp.detectAndCreateStrategy(groupPositions, timeKey)
			symbolGroup.AddStrategy(*strategy)
		}
		
		// Calculate DTE for strategies
		bp.calculateDTEForStrategies(symbolGroup)
		symbolGroups[underlyingSymbol] = symbolGroup
	}
	
	// Convert map to slice
	response := models.NewEnhancedPositionsResponse()
	for _, group := range symbolGroups {
		response.AddSymbolGroup(*group)
	}
	
	bp.Logger.Info(fmt.Sprintf("✅ Processed %d positions into %d symbol groups with enhanced structure", len(positions), len(response.SymbolGroups)))
	
	return response
}

// getUnderlyingSymbol extracts the underlying symbol from a position
func (bp *BaseProviderImpl) getUnderlyingSymbol(position *models.Position) string {
	// For options, use the underlying symbol if available
	if position.UnderlyingSymbol != nil && *position.UnderlyingSymbol != "" {
		return *position.UnderlyingSymbol
	}
	
	// For options without explicit underlying symbol, extract from option symbol
	if position.AssetClass == "us_option" {
		return bp.extractUnderlyingFromOptionSymbol(position.Symbol)
	}
	
	// For stocks and other assets, use the symbol itself
	return position.Symbol
}

// extractUnderlyingFromOptionSymbol extracts underlying symbol from option symbol
func (bp *BaseProviderImpl) extractUnderlyingFromOptionSymbol(optionSymbol string) string {
	// Find the first digit (start of date)
	for i, char := range optionSymbol {
		if char >= '0' && char <= '9' {
			return optionSymbol[:i]
		}
	}
	return optionSymbol
}

// determineAssetClass determines the asset class for the symbol group
func (bp *BaseProviderImpl) determineAssetClass(positions []*models.Position) string {
	// Check if any position in the group is an option
	for _, position := range positions {
		if position.AssetClass == "us_option" {
			return "options"
		}
	}
	// If no options found, it's stocks
	return "stocks"
}

// convertPositionToLeg converts a Position to a PositionLeg
func (bp *BaseProviderImpl) convertPositionToLeg(position *models.Position) models.PositionLeg {
	return models.PositionLeg{
		Symbol:           position.Symbol,
		Qty:              position.Qty,
		AvgEntryPrice:    position.AvgEntryPrice,
		CostBasis:        position.CostBasis,
		AssetClass:       position.AssetClass,
		LastdayPrice:     position.LastdayPrice,
		DateAcquired:     position.DateAcquired,
		UnderlyingSymbol: position.UnderlyingSymbol,
		OptionType:       position.OptionType,
		StrikePrice:      position.StrikePrice,
		ExpiryDate:       position.ExpiryDate,
	}
}

// addPositionToStrategy adds a position leg to the appropriate strategy
func (bp *BaseProviderImpl) addPositionToStrategy(symbolGroup *models.SymbolGroup, leg models.PositionLeg, position *models.Position) {
	strategyName := bp.detectStrategy(position)
	
	// Find existing strategy or create new one
	var targetStrategy *models.Strategy
	for i := range symbolGroup.Strategies {
		if symbolGroup.Strategies[i].Name == strategyName {
			targetStrategy = &symbolGroup.Strategies[i]
			break
		}
	}
	
	if targetStrategy == nil {
		// Create new strategy
		newStrategy := models.NewStrategy(strategyName)
		symbolGroup.AddStrategy(*newStrategy)
		targetStrategy = &symbolGroup.Strategies[len(symbolGroup.Strategies)-1]
	}
	
	// Add leg to strategy
	targetStrategy.AddLeg(leg)
}

// detectStrategy detects the trading strategy based on position
func (bp *BaseProviderImpl) detectStrategy(position *models.Position) string {
	// For equity positions
	if position.AssetClass != "us_option" {
		if position.Qty > 0 {
			return "Long Stock"
		}
		return "Short Stock"
	}
	
	// For single option positions
	if position.OptionType != nil {
		if position.Qty > 0 {
			return fmt.Sprintf("Long %s", strings.Title(*position.OptionType))
		}
		return fmt.Sprintf("Short %s", strings.Title(*position.OptionType))
	}
	
	return "Option Position"
}

// calculateDTEForStrategies calculates days to expiration for strategies
func (bp *BaseProviderImpl) calculateDTEForStrategies(symbolGroup *models.SymbolGroup) {
	for i := range symbolGroup.Strategies {
		strategy := &symbolGroup.Strategies[i]
		
		// Find the nearest expiration date among all legs
		var nearestExpiry *string
		var nearestDate time.Time
		
		for _, leg := range strategy.Legs {
			if leg.ExpiryDate != nil && *leg.ExpiryDate != "" {
				if expiryDate, err := time.Parse("2006-01-02", *leg.ExpiryDate); err == nil {
					if nearestExpiry == nil || expiryDate.Before(nearestDate) {
						nearestExpiry = leg.ExpiryDate
						nearestDate = expiryDate
					}
				}
			}
		}
		
		// Calculate DTE
		if nearestExpiry != nil {
			dte := bp.calculateDTE(*nearestExpiry)
			strategy.DTE = &dte
		}
	}
}

// calculateDTE calculates days to expiration
func (bp *BaseProviderImpl) calculateDTE(expiryDate string) int {
	expiry, err := time.Parse("2006-01-02", expiryDate)
	if err != nil {
		return -1
	}
	
	now := time.Now()
	diff := expiry.Sub(now)
	days := int(diff.Hours() / 24)
	
	// If same day, return 0 instead of negative
	if days < 0 {
		return 0
	}
	
	return days
}

// getTimeGroupKey creates a time-based key for grouping related positions
func (bp *BaseProviderImpl) getTimeGroupKey(dateAcquired *string) string {
	if dateAcquired == nil || *dateAcquired == "" {
		return "unknown"
	}
	
	// Parse the date and truncate to minute for grouping
	if t, err := time.Parse(time.RFC3339, *dateAcquired); err == nil {
		// Group by minute to catch related trades
		return t.Truncate(time.Minute).Format("2006-01-02 15:04")
	}
	
	return *dateAcquired
}

// detectAndCreateStrategy analyzes a group of positions and creates the appropriate strategy
func (bp *BaseProviderImpl) detectAndCreateStrategy(positions []*models.Position, timeKey string) *models.Strategy {
	if len(positions) == 0 {
		return models.NewStrategy("Unknown Strategy")
	}
	
	// Convert positions to legs
	legs := make([]models.PositionLeg, len(positions))
	for i, position := range positions {
		legs[i] = bp.convertPositionToLeg(position)
	}
	
	// Detect strategy type based on the positions
	strategyName := bp.detectMultiLegStrategy(positions)
	strategy := models.NewStrategy(strategyName)
	
	// Add all legs to the strategy
	for _, leg := range legs {
		strategy.AddLeg(leg)
	}
	
	// Set strategy-level properties
	strategy.DateAcquired = &timeKey
	
	// Calculate total quantity and cost basis
	totalQty := 0.0
	totalCostBasis := 0.0
	for _, leg := range legs {
		totalQty += leg.Qty
		totalCostBasis += leg.CostBasis
	}
	strategy.TotalQty = totalQty
	strategy.CostBasis = totalCostBasis
	
	return strategy
}

// detectMultiLegStrategy detects the strategy type for multiple positions
func (bp *BaseProviderImpl) detectMultiLegStrategy(positions []*models.Position) string {
	if len(positions) == 1 {
		return bp.detectStrategy(positions[0])
	}
	
	// Separate calls and puts
	calls := []*models.Position{}
	puts := []*models.Position{}
	stocks := []*models.Position{}
	
	for _, pos := range positions {
		if pos.AssetClass != "us_option" {
			stocks = append(stocks, pos)
		} else {
			// Try to get option type from the field first, then extract from symbol
			optionType := ""
			if pos.OptionType != nil && *pos.OptionType != "" {
				optionType = *pos.OptionType
			} else {
				// Extract option type from symbol (C or P in the symbol)
				optionType = bp.extractOptionTypeFromSymbol(pos.Symbol)
			}
			
			if optionType == "call" {
				calls = append(calls, pos)
			} else if optionType == "put" {
				puts = append(puts, pos)
			}
		}
	}
	
	bp.LogInfo(fmt.Sprintf("Strategy detection: %d calls, %d puts, %d stocks", len(calls), len(puts), len(stocks)))
	
	// Detect spreads
	if len(calls) == 2 && len(puts) == 0 {
		strategyName := bp.detectCallSpread(calls)
		bp.LogInfo(fmt.Sprintf("Detected call spread: %s", strategyName))
		return strategyName
	}
	
	if len(puts) == 2 && len(calls) == 0 {
		strategyName := bp.detectPutSpread(puts)
		bp.LogInfo(fmt.Sprintf("Detected put spread: %s", strategyName))
		return strategyName
	}
	
	if len(calls) == 1 && len(puts) == 1 {
		return bp.detectStraddleOrStrangle(calls[0], puts[0])
	}
	
	if len(calls) == 2 && len(puts) == 2 {
		return "Iron Condor"
	}
	
	if len(stocks) > 0 && (len(calls) > 0 || len(puts) > 0) {
		return "Covered Position"
	}
	
	bp.LogInfo("Falling back to Multi-Leg Strategy")
	return "Multi-Leg Strategy"
}

// detectCallSpread detects call spread type
func (bp *BaseProviderImpl) detectCallSpread(calls []*models.Position) string {
	if len(calls) != 2 {
		return "Call Spread"
	}
	
	// Determine if it's a debit or credit spread based on quantities
	pos1, pos2 := calls[0], calls[1]
	
	// Credit spread: sell higher strike, buy lower strike
	// Debit spread: buy lower strike, sell higher strike
	if (pos1.Qty > 0 && pos2.Qty < 0) || (pos1.Qty < 0 && pos2.Qty > 0) {
		// One long, one short - it's a spread
		if pos1.Qty < 0 || pos2.Qty < 0 {
			return "Call Credit Spread"
		}
		return "Call Debit Spread"
	}
	
	return "Call Spread"
}

// detectPutSpread detects put spread type
func (bp *BaseProviderImpl) detectPutSpread(puts []*models.Position) string {
	if len(puts) != 2 {
		return "Put Spread"
	}
	
	// Determine if it's a debit or credit spread based on quantities
	pos1, pos2 := puts[0], puts[1]
	
	// Credit spread: sell higher strike, buy lower strike
	// Debit spread: buy higher strike, sell lower strike
	if (pos1.Qty > 0 && pos2.Qty < 0) || (pos1.Qty < 0 && pos2.Qty > 0) {
		// One long, one short - it's a spread
		if pos1.Qty < 0 || pos2.Qty < 0 {
			return "Put Credit Spread"
		}
		return "Put Debit Spread"
	}
	
	return "Put Spread"
}

// detectStraddleOrStrangle detects straddle or strangle
func (bp *BaseProviderImpl) detectStraddleOrStrangle(call, put *models.Position) string {
	// Check if strikes are the same (straddle) or different (strangle)
	if call.StrikePrice != nil && put.StrikePrice != nil {
		if *call.StrikePrice == *put.StrikePrice {
			if call.Qty > 0 && put.Qty > 0 {
				return "Long Straddle"
			}
			return "Short Straddle"
		} else {
			if call.Qty > 0 && put.Qty > 0 {
				return "Long Strangle"
			}
			return "Short Strangle"
		}
	}
	
	return "Straddle/Strangle"
}

// extractOptionTypeFromSymbol extracts option type (call/put) from option symbol
func (bp *BaseProviderImpl) extractOptionTypeFromSymbol(symbol string) string {
	// Option symbol format: [UNDERLYING][YYMMDD][C/P][STRIKE:8]
	// Look for 'C' or 'P' in the symbol
	if strings.Contains(symbol, "C") {
		// Find the position of 'C' - it should be after the date part
		for i := len(symbol) - 9; i >= 0; i-- { // Start from position where C/P should be
			if symbol[i] == 'C' {
				// Verify this is the option type indicator by checking if followed by 8 digits
				if i+9 == len(symbol) {
					return "call"
				}
			}
		}
	}
	
	if strings.Contains(symbol, "P") {
		// Find the position of 'P' - it should be after the date part
		for i := len(symbol) - 9; i >= 0; i-- { // Start from position where C/P should be
			if symbol[i] == 'P' {
				// Verify this is the option type indicator by checking if followed by 8 digits
				if i+9 == len(symbol) {
					return "put"
				}
			}
		}
	}
	
	return ""
}

// Helper method to check if a symbol is an option symbol
// This matches the Python _is_option_symbol logic
func (bp *BaseProviderImpl) IsOptionSymbol(symbol string) bool {
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
