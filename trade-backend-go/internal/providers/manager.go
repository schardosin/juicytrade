package providers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/alpaca"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/providers/tastytrade"
	"trade-backend-go/internal/providers/tradier"
)

// ProviderManager manages multiple provider instances and routes operations.
// Exact conversion of Python ProviderManager class.
type ProviderManager struct {
	providers       map[string]base.Provider
	credentialStore *CredentialStore
	configManager   *ConfigManager
	mutex           sync.RWMutex
}

// NewProviderManager creates a new provider manager.
// Exact conversion of Python ProviderManager.__init__ method.
func NewProviderManager() *ProviderManager {
	pm := &ProviderManager{
		providers:       make(map[string]base.Provider),
		credentialStore: NewCredentialStore(),
		configManager:   NewConfigManager(),
	}
	
	pm.initializeActiveProviders()
	return pm
}

// initializeActiveProviders initializes only active provider instances from credential store.
// Exact conversion of Python _initialize_active_providers method.
func (pm *ProviderManager) initializeActiveProviders() {
	slog.Info("🔄 Initializing active provider instances...")
	
	// Clear existing providers
	pm.mutex.Lock()
	pm.providers = make(map[string]base.Provider)
	pm.mutex.Unlock()
	
	// Get active instances from credential store
	activeInstances := pm.credentialStore.GetActiveInstances()
	
	if len(activeInstances) == 0 {
		slog.Warn("⚠️ No active provider instances found. Please configure provider instances through the API.")
		return
	}
	
	// Initialize each active instance
	for instanceID, instanceData := range activeInstances {
		providerType, _ := instanceData["provider_type"].(string)
		accountType, _ := instanceData["account_type"].(string)
		credentials, _ := instanceData["credentials"].(map[string]interface{})
		displayName, _ := instanceData["display_name"].(string)
		
		if displayName == "" {
			displayName = instanceID
		}
		
		// Apply default values to credentials
		credentials = ApplyDefaults(providerType, accountType, credentials)
		
		// Create provider instance
		provider := pm.createProviderInstance(providerType, accountType, credentials)
		
		if provider != nil {
			pm.mutex.Lock()
			pm.providers[instanceID] = provider
			pm.mutex.Unlock()
			slog.Info(fmt.Sprintf("✅ Initialized provider instance: %s (%s)", displayName, instanceID))
		} else {
			slog.Error(fmt.Sprintf("❌ Failed to create provider instance: %s", instanceID))
		}
	}
	
	slog.Info(fmt.Sprintf("🎯 Initialized %d active provider instances", len(pm.providers)))
}

// createProviderInstance creates a provider instance based on type and credentials.
// Exact conversion of Python _create_provider_instance method.
func (pm *ProviderManager) createProviderInstance(providerType, accountType string, credentials map[string]interface{}) base.Provider {
	switch providerType {
	case "alpaca":
		apiKey, _ := credentials["api_key"].(string)
		apiSecret, _ := credentials["api_secret"].(string)
		baseURL, _ := credentials["base_url"].(string)
		dataURL, _ := credentials["data_url"].(string)
		usePaper := (accountType == "paper")
		
		return alpaca.NewAlpacaProvider(apiKey, apiSecret, baseURL, dataURL, usePaper)
		
	case "tastytrade":
		accountID, _ := credentials["account_id"].(string)
		baseURL, _ := credentials["base_url"].(string)
		clientID, _ := credentials["client_id"].(string)
		clientSecret, _ := credentials["client_secret"].(string)
		refreshToken, _ := credentials["refresh_token"].(string)
		authCode, _ := credentials["authorization_code"].(string)
		redirectURI, _ := credentials["redirect_uri"].(string)
		
		return tastytrade.NewTastyTradeProvider(accountID, baseURL, clientID, clientSecret, refreshToken, authCode, redirectURI)
		
	case "tradier":
		accountID, _ := credentials["account_id"].(string)
		apiKey, _ := credentials["api_key"].(string)
		baseURL, _ := credentials["base_url"].(string)
		streamURL, _ := credentials["stream_url"].(string)
		
		return tradier.NewTradierProvider(accountID, apiKey, baseURL, streamURL)
		
	// TODO: Add other providers (public)
	// case "public":
	//     return public.NewPublicProvider(...)
	
	default:
		slog.Error(fmt.Sprintf("❌ Unknown provider type: %s", providerType))
		return nil
	}
}

// GetAvailableProviderInstances gets all available provider instances with their metadata.
// Exact conversion of Python get_available_provider_instances method.
func (pm *ProviderManager) GetAvailableProviderInstances() map[string]map[string]interface{} {
	instances := make(map[string]map[string]interface{})
	
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	for instanceID := range pm.providers {
		instanceData := pm.credentialStore.GetInstance(instanceID)
		if instanceData != nil {
			instances[instanceID] = map[string]interface{}{
				"provider_type": instanceData["provider_type"],
				"account_type":  instanceData["account_type"],
				"display_name":  instanceData["display_name"],
				"active":        instanceData["active"],
				"created_at":    instanceData["created_at"],
				"updated_at":    instanceData["updated_at"],
			}
		}
	}
	
	return instances
}

// TestProviderCredentials tests provider credentials by making a real API call.
// Exact conversion of Python test_provider_credentials method.
func (pm *ProviderManager) TestProviderCredentials(ctx context.Context, providerType, accountType string, credentials map[string]interface{}) map[string]interface{} {
	// Apply defaults to credentials
	testCredentials := ApplyDefaults(providerType, accountType, credentials)
	
	// Create temporary provider instance
	provider := pm.createProviderInstance(providerType, accountType, testCredentials)
	
	if provider == nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to create %s provider instance", providerType),
		}
	}
	
	// Test credentials using the provider's test method
	result, err := provider.TestCredentials(ctx)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}
	return result
}

// getProvider gets a provider for a specific operation based on configuration.
// Exact conversion of Python _get_provider method.
func (pm *ProviderManager) getProvider(operation string) base.Provider {
	config := pm.configManager.GetConfig()
	providerName, exists := config[operation]
	
	if !exists || providerName == "" {
		slog.Error(fmt.Sprintf("No provider configured for operation: %s", operation))
		return nil
	}
	
	pm.mutex.RLock()
	provider, exists := pm.providers[providerName]
	pm.mutex.RUnlock()
	
	if !exists {
		slog.Error(fmt.Sprintf("Provider '%s' not initialized.", providerName))
		return nil
	}
	
	return provider
}

// GetProvider gets a provider instance by its instance ID.
// Exact conversion of Python get_provider method.
func (pm *ProviderManager) GetProvider(instanceID string) base.Provider {
	pm.mutex.RLock()
	provider, exists := pm.providers[instanceID]
	pm.mutex.RUnlock()
	
	if !exists {
		slog.Error(fmt.Sprintf("Provider instance '%s' not found or not initialized.", instanceID))
		return nil
	}
	
	return provider
}

// === Market Data Methods - Exact conversions ===

// GetExpirationDates gets expiration dates for options on a symbol with universal enhanced structure.
// Exact conversion of Python get_expiration_dates method.
func (pm *ProviderManager) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	provider := pm.getProvider("options_chain")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for options_chain")
	}
	
	return provider.GetExpirationDates(ctx, symbol)
}

// GetStockQuote gets a stock quote for a symbol.
// Exact conversion of Python get_stock_quote method.
func (pm *ProviderManager) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	provider := pm.getProvider("stock_quotes")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for stock_quotes")
	}
	
	return provider.GetStockQuote(ctx, symbol)
}

// GetStockQuotes gets stock quotes for multiple symbols.
// Exact conversion of Python get_stock_quotes method.
func (pm *ProviderManager) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	provider := pm.getProvider("stock_quotes")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for stock_quotes")
	}
	
	return provider.GetStockQuotes(ctx, symbols)
}

// GetOptionsChainBasic gets basic options chain data.
// Exact conversion of Python get_options_chain_basic method.
func (pm *ProviderManager) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	provider := pm.getProvider("options_chain")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for options_chain")
	}
	
	return provider.GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, strikeCount, optionType, underlyingSymbol)
}

// GetOptionsGreeksBatch gets Greeks for multiple option symbols.
// Exact conversion of Python get_options_greeks_batch method.
func (pm *ProviderManager) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	provider := pm.getProvider("options_chain")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for options_chain")
	}
	
	return provider.GetOptionsGreeksBatch(ctx, optionSymbols)
}

// GetOptionsChainSmart gets smart options chain data.
// Exact conversion of Python get_options_chain_smart method.
func (pm *ProviderManager) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	provider := pm.getProvider("options_chain")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for options_chain")
	}
	
	return provider.GetOptionsChainSmart(ctx, symbol, expiry, underlyingPrice, atmRange, includeGreeks, strikesOnly)
}

// === Trading Methods - Exact conversions ===

// GetPositions gets account positions.
// Exact conversion of Python get_positions method.
func (pm *ProviderManager) GetPositions(ctx context.Context) ([]*models.Position, error) {
	provider := pm.getProvider("trade_account")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for trade_account")
	}
	
	return provider.GetPositions(ctx)
}

// GetOrders gets account orders.
// Exact conversion of Python get_orders method.
func (pm *ProviderManager) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	provider := pm.getProvider("trade_account")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for trade_account")
	}
	
	return provider.GetOrders(ctx, status)
}

// GetAccount gets account information.
// Exact conversion of Python get_account method.
func (pm *ProviderManager) GetAccount(ctx context.Context) (*models.Account, error) {
	provider := pm.getProvider("trade_account")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for trade_account")
	}
	
	return provider.GetAccount(ctx)
}

// PlaceOrder places a trading order.
// Exact conversion of Python place_order method.
func (pm *ProviderManager) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	provider := pm.getProvider("trade_account")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for trade_account")
	}
	
	return provider.PlaceOrder(ctx, orderData)
}

// PlaceMultiLegOrder places a multi-leg trading order.
// Exact conversion of Python place_multi_leg_order method.
func (pm *ProviderManager) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	provider := pm.getProvider("trade_account")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for trade_account")
	}
	
	return provider.PlaceMultiLegOrder(ctx, orderData)
}

// CancelOrder cancels a trading order.
// Exact conversion of Python cancel_order method.
func (pm *ProviderManager) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	provider := pm.getProvider("trade_account")
	if provider == nil {
		return false, fmt.Errorf("no provider configured for trade_account")
	}
	
	return provider.CancelOrder(ctx, orderID)
}

// === Other Methods - Exact conversions ===

// LookupSymbols searches for symbols.
// Exact conversion of Python lookup_symbols method.
func (pm *ProviderManager) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	provider := pm.getProvider("symbol_lookup")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for symbol_lookup")
	}
	
	return provider.LookupSymbols(ctx, query)
}

// GetHistoricalBars gets historical bar data.
// Exact conversion of Python get_historical_bars method.
func (pm *ProviderManager) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	provider := pm.getProvider("historical_data")
	if provider == nil {
		return nil, fmt.Errorf("no provider configured for historical_data")
	}
	
	// Map weekly symbols to their standard equivalents for historical data
	chartSymbol := pm.mapSymbolForHistoricalData(symbol)
	if chartSymbol != symbol {
		slog.Info(fmt.Sprintf("📊 Mapping symbol for historical data: %s → %s", symbol, chartSymbol))
	}
	
	return provider.GetHistoricalBars(ctx, chartSymbol, timeframe, startDate, endDate, limit)
}

// GetNextMarketDate gets the next market date.
// Exact conversion of Python get_next_market_date method.
func (pm *ProviderManager) GetNextMarketDate(ctx context.Context) (string, error) {
	provider := pm.getProvider("market_calendar")
	if provider == nil {
		return "", fmt.Errorf("no provider configured for market_calendar")
	}
	
	return provider.GetNextMarketDate(ctx)
}

// HealthCheck performs health check on all providers.
// Exact conversion of Python health_check method.
func (pm *ProviderManager) HealthCheck(ctx context.Context) map[string]interface{} {
	healthStatus := make(map[string]interface{})
	
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	for name, provider := range pm.providers {
		result, err := provider.TestCredentials(ctx)
		if err != nil {
			healthStatus[name] = map[string]interface{}{
				"success": false,
				"message": err.Error(),
			}
		} else {
			healthStatus[name] = result
		}
	}
	
	return healthStatus
}

// mapSymbolForHistoricalData maps symbols to their standard equivalents for historical data.
// Exact conversion of Python _map_symbol_for_historical_data method.
func (pm *ProviderManager) mapSymbolForHistoricalData(symbol string) string {
	// Weekly symbols that need to be mapped to their standard equivalents
	weeklySymbolMap := map[string]string{
		"SPXW": "SPX", // SPX Weeklys → SPX
		// Add other weekly symbols here if needed
		// "NDXP": "NDX",  // Example: NDX Weeklys (if they exist)
		// "RUTW": "RUT",  // Example: RUT Weeklys (if they exist)
	}
	
	// Return mapped symbol or original if no mapping exists
	if mapped, exists := weeklySymbolMap[symbol]; exists {
		return mapped
	}
	return symbol
}

// === Configuration Methods - Exact conversions ===

// GetConfig gets the current provider configuration.
// Exact conversion of Python get_config method.
func (pm *ProviderManager) GetConfig() map[string]string {
	return pm.configManager.GetConfig()
}

// UpdateConfig updates the provider configuration.
// Exact conversion of Python update_config method.
func (pm *ProviderManager) UpdateConfig(newConfig map[string]interface{}) bool {
	return pm.configManager.UpdateConfig(newConfig)
}

// ResetConfig resets the provider configuration to defaults.
// Exact conversion of Python reset_config method.
func (pm *ProviderManager) ResetConfig() {
	pm.configManager.ResetConfig()
}

// GetAvailableProviders gets available providers with their capabilities.
// Exact conversion of Python get_available_providers method.
func (pm *ProviderManager) GetAvailableProviders() map[string]map[string]interface{} {
	return pm.configManager.GetAvailableProviders()
}

// InitializeActiveProviders reinitializes active providers (public method).
// Exact conversion of Python initialize_active_providers method.
func (pm *ProviderManager) InitializeActiveProviders() {
	pm.initializeActiveProviders()
}

// Global provider manager instance
var GlobalProviderManager *ProviderManager

// InitializeProviderManager initializes the global provider manager
func InitializeProviderManager() {
	GlobalProviderManager = NewProviderManager()
}
