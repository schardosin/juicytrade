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
