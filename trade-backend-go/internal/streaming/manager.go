package streaming

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
	"trade-backend-go/internal/providers/base"
)

// LatestValueCache is a latest-value cache for real-time market data.
// Only stores the most recent value for each symbol, discarding old data.
// Exact conversion of Python LatestValueCache class.
type LatestValueCache struct {
	data            map[string]*models.MarketData
	lock            sync.RWMutex
	updateCallbacks []func(*models.MarketData) error
	healthManager   *StreamingHealthManager
}

// NewLatestValueCache creates a new latest value cache.
func NewLatestValueCache() *LatestValueCache {
	return &LatestValueCache{
		data:            make(map[string]*models.MarketData),
		updateCallbacks: make([]func(*models.MarketData) error, 0),
	}
}

// Update updates cache with new market data - maximum performance, no deduplication.
// Enhanced with health monitoring callback.
func (c *LatestValueCache) Update(marketData *models.MarketData) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	symbol := marketData.Symbol

	if marketData.TimestampMs == nil {
		now := time.Now().UnixMilli()
		marketData.TimestampMs = &now
	}

	c.data[symbol] = marketData

	// Notify health manager that data was received
	if c.healthManager != nil {
		// Record data received for all registered connections
		// In a production system, we'd track which provider sent which data
		c.healthManager.mutex.RLock()
		for connID := range c.healthManager.connections {
			c.healthManager.RecordDataReceived(connID)
		}
		c.healthManager.mutex.RUnlock()
	}

	// Call update callbacks
	for _, callback := range c.updateCallbacks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic in cache update callback", "panic", r)
				}
			}()
			if err := callback(marketData); err != nil {
				slog.Error("Error in cache update callback", "error", err)
			}
		}()
	}

	return nil
}

// GetLatest gets the latest data for a symbol.
// Exact conversion of Python get_latest method.
func (c *LatestValueCache) GetLatest(symbol string) *models.MarketData {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.data[symbol]
}

// AddUpdateCallback adds callback to be called when data is updated.
func (c *LatestValueCache) AddUpdateCallback(callback func(*models.MarketData)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	// Wrap the callback to match our internal signature
	wrappedCallback := func(data *models.MarketData) error {
		callback(data)
		return nil
	}
	c.updateCallbacks = append(c.updateCallbacks, wrappedCallback)
}

// SetHealthManager sets the health manager for data received tracking
func (c *LatestValueCache) SetHealthManager(hm *StreamingHealthManager) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.healthManager = hm
}

// StreamingManager manages streaming provider connections and aggregates subscriptions from all clients.
// Supports independent quote and Greeks streams from different providers.
// Enhanced with centralized health monitoring and automatic recovery.
type StreamingManager struct {
	quoteProviders      map[string]base.Provider
	greeksProviders     map[string]base.Provider
	latestCache         *LatestValueCache
	quoteSubscriptions  map[string]bool
	greeksSubscriptions map[string]bool
	isConnected         bool
	shutdownEvent       chan struct{}
	lock                sync.RWMutex

	// Health monitoring
	healthManager *StreamingHealthManager
}

// NewStreamingManager creates a new streaming manager.
func NewStreamingManager() *StreamingManager {
	healthManager := GetHealthManager()
	cache := NewLatestValueCache()
	cache.SetHealthManager(healthManager)

	return &StreamingManager{
		quoteProviders:      make(map[string]base.Provider),
		greeksProviders:     make(map[string]base.Provider),
		latestCache:         cache,
		quoteSubscriptions:  make(map[string]bool),
		greeksSubscriptions: make(map[string]bool),
		shutdownEvent:       make(chan struct{}),
		healthManager:       healthManager,
	}
}

// Connect connects to all streaming providers specified in the configuration.
// Enhanced with health monitoring startup.
func (sm *StreamingManager) Connect(ctx context.Context) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	slog.Info("Starting streaming manager connection...")

	// Get provider configuration (same as Python)
	providerConfig := providers.GlobalProviderManager.GetConfig()

	// Connect quote providers
	quotesProviderName := providerConfig["streaming_quotes"]
	if quotesProviderName != "" {
		if err := sm.connectQuoteProvider(ctx, quotesProviderName); err != nil {
			slog.Error("Failed to connect quote provider", "provider", quotesProviderName, "error", err)
		}
	}

	// Connect Greeks providers (independent of quotes)
	greeksProviderName := providerConfig["streaming_greeks"]
	if greeksProviderName != "" {
		if err := sm.connectGreeksProvider(ctx, greeksProviderName); err != nil {
			slog.Error("Failed to connect Greeks provider", "provider", greeksProviderName, "error", err)
		}
	}

	// Check overall connection status
	allProviders := make(map[string]base.Provider)
	for name, provider := range sm.quoteProviders {
		allProviders[name] = provider
	}
	for name, provider := range sm.greeksProviders {
		allProviders[name] = provider
	}

	sm.isConnected = false
	for _, provider := range allProviders {
		if provider.IsStreamingConnected() {
			sm.isConnected = true
			break
		}
	}

	var activeProviders []string
	for name := range sm.quoteProviders {
		activeProviders = append(activeProviders, name)
	}
	for name := range sm.greeksProviders {
		found := false
		for _, existing := range activeProviders {
			if existing == name {
				found = true
				break
			}
		}
		if !found {
			activeProviders = append(activeProviders, name)
		}
	}

	slog.Info("Streaming manager connected", "active_providers", activeProviders)

	// Start health monitoring
	sm.healthManager.StartMonitoring(ctx)

	return nil
}

// connectQuoteProvider connects a provider for quote streaming.
// Enhanced with health monitoring registration.
func (sm *StreamingManager) connectQuoteProvider(ctx context.Context, providerName string) error {
	if _, exists := sm.quoteProviders[providerName]; !exists {
		provider := providers.GlobalProviderManager.GetProvider(providerName)
		if provider == nil {
			return fmt.Errorf("provider %s not found", providerName)
		}

		// Register provider with health manager
		sm.healthManager.RegisterProvider(providerName, provider)
		sm.healthManager.RegisterConnection(providerName, providerName, "websocket")

		// The provider already implements base.Provider which includes streaming methods
		provider.SetStreamingCache(sm.latestCache)
		sm.quoteProviders[providerName] = provider

		connected, err := sm.connectWithRetry(ctx, provider)
		if err != nil {
			return fmt.Errorf("failed to connect %s for quotes: %w", providerName, err)
		}

		if connected {
			slog.Info("Provider connected for quote streaming", "provider", providerName)
			sm.healthManager.UpdateConnectionState(providerName, StateConnected)
		} else {
			slog.Error("Failed to connect provider for quotes", "provider", providerName)
			sm.healthManager.UpdateConnectionState(providerName, StateFailed)
		}
	}

	return nil
}

// connectGreeksProvider connects a provider for Greeks streaming.
// Exact conversion of Python _connect_greeks_provider method.
func (sm *StreamingManager) connectGreeksProvider(ctx context.Context, providerName string) error {
	if _, exists := sm.greeksProviders[providerName]; !exists {
		// Check if this provider is already connected for quotes
		if quoteProvider, exists := sm.quoteProviders[providerName]; exists {
			// Reuse the existing connection
			sm.greeksProviders[providerName] = quoteProvider
			slog.Info("Provider reusing connection for Greeks streaming", "provider", providerName)
		} else {
			// Create new connection for Greeks only
			provider := providers.GlobalProviderManager.GetProvider(providerName)
			if provider == nil {
				return fmt.Errorf("provider %s not found", providerName)
			}

			// Register with health manager
			sm.healthManager.RegisterProvider(providerName, provider)
			sm.healthManager.RegisterConnection(providerName+"_greeks", providerName, "websocket")

			// The provider already implements base.Provider which includes streaming methods
			provider.SetStreamingCache(sm.latestCache)
			sm.greeksProviders[providerName] = provider

			connected, err := sm.connectWithRetry(ctx, provider)
			if err != nil {
				return fmt.Errorf("failed to connect %s for Greeks: %w", providerName, err)
			}

			if connected {
				slog.Info("Provider connected for Greeks streaming", "provider", providerName)
				sm.healthManager.UpdateConnectionState(providerName+"_greeks", StateConnected)
			} else {
				slog.Error("Failed to connect provider for Greeks", "provider", providerName)
				sm.healthManager.UpdateConnectionState(providerName+"_greeks", StateFailed)
			}
		}
	}

	return nil
}

// connectWithRetry connects a provider with exponential backoff retry logic.
// Exact conversion of Python _connect_with_retry method.
func (sm *StreamingManager) connectWithRetry(ctx context.Context, provider base.Provider) (bool, error) {
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		slog.Info("Connecting provider", "provider", provider.GetName(), "attempt", attempt+1, "max_retries", maxRetries)

		connected, err := provider.ConnectStreaming(ctx)
		if err == nil && connected {
			slog.Info("Provider connected successfully", "provider", provider.GetName())
			return true, nil
		}

		if err != nil {
			slog.Error("Connection error for provider", "provider", provider.GetName(), "error", err)
		}

		if attempt < maxRetries-1 {
			delay := time.Duration(1<<attempt) * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			slog.Info("Waiting before retry", "delay", delay)
			time.Sleep(delay)
		}
	}

	return false, fmt.Errorf("failed to connect after %d attempts", maxRetries)
}

// UpdateGlobalSubscriptions aggregates all client subscriptions and updates both quote and Greeks providers.
// Exact conversion of Python update_global_subscriptions method.
func (sm *StreamingManager) UpdateGlobalSubscriptions(ctx context.Context, clientSubscriptions map[interface{}]map[string]bool) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	// 1. Aggregate all symbols from all clients
	globalNewSymbols := make(map[string]bool)
	for _, clientSubs := range clientSubscriptions {
		for symbol := range clientSubs {
			globalNewSymbols[symbol] = true
		}
	}

	// 2. Separate stock and option symbols
	stockSymbols := make(map[string]bool)
	optionSymbols := make(map[string]bool)

	for symbol := range globalNewSymbols {
		if sm.isOptionSymbol(symbol) {
			optionSymbols[symbol] = true
		} else {
			stockSymbols[symbol] = true
		}
	}

	// 3. Update quote subscriptions (stocks + options)
	if err := sm.updateQuoteSubscriptions(ctx, globalNewSymbols); err != nil {
		return fmt.Errorf("failed to update quote subscriptions: %w", err)
	}

	// 4. Update Greeks subscriptions (options only)
	if err := sm.updateGreeksSubscriptions(ctx, optionSymbols); err != nil {
		return fmt.Errorf("failed to update Greeks subscriptions: %w", err)
	}

	slog.Info("Global subscriptions updated", "quotes", len(globalNewSymbols), "greeks", len(optionSymbols))
	return nil
}

// isOptionSymbol checks if symbol is an option symbol.
func (sm *StreamingManager) isOptionSymbol(symbol string) bool {
	return len(symbol) > 10 &&
		(containsChar(symbol, 'C') || containsChar(symbol, 'P')) &&
		hasDigitsInLast8(symbol)
}

// containsChar checks if string contains a character
func containsChar(s string, c rune) bool {
	for _, char := range s {
		if char == c {
			return true
		}
	}
	return false
}

// hasDigitsInLast8 checks if last 8 characters contain digits
func hasDigitsInLast8(symbol string) bool {
	if len(symbol) < 8 {
		return false
	}
	lastEight := symbol[len(symbol)-8:]
	for _, char := range lastEight {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}

// updateQuoteSubscriptions updates quote subscriptions on quote providers.
// Enhanced to send complete symbol list to providers (they decide whether to replace or add).
func (sm *StreamingManager) updateQuoteSubscriptions(ctx context.Context, symbols map[string]bool) error {
	// Convert current subscriptions to map for easier comparison
	currentSubs := make(map[string]bool)
	for symbol := range sm.quoteSubscriptions {
		currentSubs[symbol] = true
	}

	// Find symbols to add and remove
	symbolsToAdd := make([]string, 0)
	symbolsToRemove := make([]string, 0)

	for symbol := range symbols {
		if !currentSubs[symbol] {
			symbolsToAdd = append(symbolsToAdd, symbol)
		}
	}

	for symbol := range currentSubs {
		if !symbols[symbol] {
			symbolsToRemove = append(symbolsToRemove, symbol)
		}
	}

	// Unsubscribe from removed symbols
	if len(symbolsToRemove) > 0 {
		slog.Info("Unsubscribing quotes from symbols", "count", len(symbolsToRemove))
		if err := sm.unsubscribeQuotesSafe(ctx, symbolsToRemove); err != nil {
			return err
		}
	}

	// Subscribe to new symbols - IMPORTANT: Pass ALL symbols (existing + new)
	// Each provider decides whether to replace or add based on their API requirements
	if len(symbolsToAdd) > 0 {
		// Build complete list of all symbols that should be subscribed
		allSymbols := make([]string, 0, len(symbols))
		for symbol := range symbols {
			allSymbols = append(allSymbols, symbol)
		}

		slog.Info("Subscribing quotes to symbols",
			"new_count", len(symbolsToAdd),
			"total_count", len(allSymbols))

		// Pass ALL symbols to providers - they handle replace vs add internally
		if err := sm.subscribeQuotesSafe(ctx, allSymbols); err != nil {
			return err
		}
	}

	// CRITICAL FIX: Always update HealthManager with the new full state for all quote providers
	// This ensures that if we ONLY unsubscribed, the HealthManager still knows the new smaller list.
	// This prevents "zombie" subscriptions from reappearing during recovery.
	allSymbolsList := make([]string, 0, len(symbols))
	for symbol := range symbols {
		allSymbolsList = append(allSymbolsList, symbol)
	}
	for providerName := range sm.quoteProviders {
		sm.healthManager.UpdateSubscriptions(providerName, allSymbolsList)
	}

	sm.quoteSubscriptions = symbols
	return nil
}

// updateGreeksSubscriptions updates Greeks subscriptions on Greeks providers.
// Enhanced to send complete symbol list to providers (they decide whether to replace or add).
func (sm *StreamingManager) updateGreeksSubscriptions(ctx context.Context, optionSymbols map[string]bool) error {
	// Convert current subscriptions to map for easier comparison
	currentSubs := make(map[string]bool)
	for symbol := range sm.greeksSubscriptions {
		currentSubs[symbol] = true
	}

	// Find symbols to add and remove
	symbolsToAdd := make([]string, 0)
	symbolsToRemove := make([]string, 0)

	for symbol := range optionSymbols {
		if !currentSubs[symbol] {
			symbolsToAdd = append(symbolsToAdd, symbol)
		}
	}

	for symbol := range currentSubs {
		if !optionSymbols[symbol] {
			symbolsToRemove = append(symbolsToRemove, symbol)
		}
	}

	// Unsubscribe from removed symbols
	if len(symbolsToRemove) > 0 {
		slog.Info("Unsubscribing Greeks from symbols", "count", len(symbolsToRemove))
		if err := sm.unsubscribeGreeksSafe(ctx, symbolsToRemove); err != nil {
			return err
		}
	}

	// Subscribe to new symbols - IMPORTANT: Pass ALL symbols (existing + new)
	// Each provider decides whether to replace or add based on their API requirements
	if len(symbolsToAdd) > 0 {
		// Build complete list of all symbols that should be subscribed
		allSymbols := make([]string, 0, len(optionSymbols))
		for symbol := range optionSymbols {
			allSymbols = append(allSymbols, symbol)
		}

		slog.Info("Subscribing Greeks to symbols",
			"new_count", len(symbolsToAdd),
			"total_count", len(allSymbols))

		// Pass ALL symbols to providers - they handle replace vs add internally
		if err := sm.subscribeGreeksSafe(ctx, allSymbols); err != nil {
			return err
		}
	}

	// CRITICAL FIX: Always update HealthManager with the new full state for all Greeks providers
	allGreeksList := make([]string, 0, len(optionSymbols))
	for symbol := range optionSymbols {
		allGreeksList = append(allGreeksList, symbol)
	}
	for providerName := range sm.greeksProviders {
		sm.healthManager.UpdateSubscriptions(providerName, allGreeksList)
	}

	sm.greeksSubscriptions = optionSymbols
	return nil
}

// subscribeQuotesSafe safely subscribes to quote data on quote providers.
// Enhanced to update health manager with subscription tracking.
func (sm *StreamingManager) subscribeQuotesSafe(ctx context.Context, symbols []string) error {
	for providerName, provider := range sm.quoteProviders {
		// Convert symbols to provider format if needed
		providerSymbols := sm.convertSymbolsToProviderFormat(symbols, providerName)

		// Update health manager with current subscriptions for recovery
		sm.healthManager.UpdateSubscriptions(providerName, symbols)

		// Ensure connection is healthy before subscribing (Python-style)
		if healthyProvider, ok := provider.(interface{ EnsureHealthyConnection(context.Context) error }); ok {
			if err := healthyProvider.EnsureHealthyConnection(ctx); err != nil {
				slog.Error("Failed to ensure healthy connection", "provider", providerName, "error", err)
				continue
			}
		}

		// Subscribe to both Quote and Trade events - Trade provides "last" price
		// which is essential for indices like NDX that don't have valid bid/ask
		_, err := provider.SubscribeToSymbols(ctx, providerSymbols, []string{"Quote", "Trade"})
		if err != nil {
			slog.Error("Error subscribing quotes on provider", "provider", providerName, "error", err)
			continue
		}

		slog.Debug("Subscribed to quotes on provider", "provider", providerName, "symbols", len(symbols))
	}
	return nil
}

// unsubscribeQuotesSafe safely unsubscribes from quote data on quote providers.
// Exact conversion of Python _unsubscribe_quotes_safe method.
func (sm *StreamingManager) unsubscribeQuotesSafe(ctx context.Context, symbols []string) error {
	for providerName, provider := range sm.quoteProviders {
		// Convert symbols to provider format if needed
		providerSymbols := sm.convertSymbolsToProviderFormat(symbols, providerName)

		// Unsubscribe from both Quote and Trade events (matching subscribe)
		_, err := provider.UnsubscribeFromSymbols(ctx, providerSymbols, []string{"Quote", "Trade"})
		if err != nil {
			slog.Error("Error unsubscribing quotes on provider", "provider", providerName, "error", err)
			continue
		}

		slog.Debug("Unsubscribed from quotes on provider", "provider", providerName, "symbols", len(symbols))
	}
	return nil
}

// subscribeGreeksSafe safely subscribes to Greeks data on Greeks providers.
// Enhanced to update health manager with subscription tracking.
func (sm *StreamingManager) subscribeGreeksSafe(ctx context.Context, symbols []string) error {
	slog.Info("subscribeGreeksSafe called", "greeks_providers_count", len(sm.greeksProviders), "symbols_count", len(symbols))

	for providerName, provider := range sm.greeksProviders {
		slog.Info("Subscribing Greeks on provider", "provider", providerName, "symbols", len(symbols))

		// Convert symbols to provider format if needed
		providerSymbols := sm.convertSymbolsToProviderFormat(symbols, providerName)

		// Update subscriptions FIRST before attempting to subscribe
		sm.healthManager.UpdateSubscriptions(providerName, symbols)

		// Ensure connection is healthy before subscribing (Python-style)
		if healthyProvider, ok := provider.(interface{ EnsureHealthyConnection(context.Context) error }); ok {
			if err := healthyProvider.EnsureHealthyConnection(ctx); err != nil {
				slog.Error("Failed to ensure healthy connection", "provider", providerName, "error", err)
				continue
			}
		}

		// Call subscribe_to_symbols with Greeks-only data types
		_, err := provider.SubscribeToSymbols(ctx, providerSymbols, []string{"Greeks"})
		if err != nil {
			slog.Error("Error subscribing Greeks on provider", "provider", providerName, "error", err)
			continue
		}

		slog.Info("Subscribed to Greeks-only streaming on provider", "provider", providerName, "symbols", len(symbols))
	}

	if len(sm.greeksProviders) == 0 {
		slog.Warn("No Greeks providers available for subscription")
	}

	return nil
}

// unsubscribeGreeksSafe safely unsubscribes from Greeks data on Greeks providers.
// Exact conversion of Python _unsubscribe_greeks_safe method.
func (sm *StreamingManager) unsubscribeGreeksSafe(ctx context.Context, symbols []string) error {
	for providerName, provider := range sm.greeksProviders {
		// Convert symbols to provider format if needed
		providerSymbols := sm.convertSymbolsToProviderFormat(symbols, providerName)

		_, err := provider.UnsubscribeFromSymbols(ctx, providerSymbols, []string{"Greeks"})
		if err != nil {
			slog.Error("Error unsubscribing Greeks on provider", "provider", providerName, "error", err)
			continue
		}

		slog.Info("Unsubscribed from Greeks-only streaming on provider", "provider", providerName, "symbols", len(symbols))
	}
	return nil
}

// convertSymbolsToProviderFormat converts symbols to provider format (placeholder for now).
func (sm *StreamingManager) convertSymbolsToProviderFormat(symbols []string, providerName string) []string {
	// For now, return symbols as-is. In the Python version, this uses SymbolConverter
	// which would need to be implemented separately
	return symbols
}

// GetSubscriptionStatus gets the current global subscription status.
// Exact conversion of Python get_subscription_status method.
func (sm *StreamingManager) GetSubscriptionStatus() map[string]interface{} {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	quoteSubscriptions := make([]string, 0, len(sm.quoteSubscriptions))
	for symbol := range sm.quoteSubscriptions {
		quoteSubscriptions = append(quoteSubscriptions, symbol)
	}

	greeksSubscriptions := make([]string, 0, len(sm.greeksSubscriptions))
	for symbol := range sm.greeksSubscriptions {
		greeksSubscriptions = append(greeksSubscriptions, symbol)
	}

	quoteProviders := make([]string, 0, len(sm.quoteProviders))
	for name := range sm.quoteProviders {
		quoteProviders = append(quoteProviders, name)
	}

	greeksProviders := make([]string, 0, len(sm.greeksProviders))
	for name := range sm.greeksProviders {
		greeksProviders = append(greeksProviders, name)
	}

	return map[string]interface{}{
		"quote_subscriptions":        quoteSubscriptions,
		"greeks_subscriptions":       greeksSubscriptions,
		"total_quote_subscriptions":  len(sm.quoteSubscriptions),
		"total_greeks_subscriptions": len(sm.greeksSubscriptions),
		"is_connected":               sm.isConnected,
		"quote_providers":            quoteProviders,
		"greeks_providers":           greeksProviders,
	}
}

// GetHealthStats gets detailed health statistics for streaming connections.
// Enhanced to include health manager stats.
func (sm *StreamingManager) GetHealthStats() map[string]interface{} {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	quoteProviders := make(map[string]bool)
	for name, provider := range sm.quoteProviders {
		quoteProviders[name] = provider.IsStreamingConnected()
	}

	greeksProviders := make(map[string]bool)
	for name, provider := range sm.greeksProviders {
		greeksProviders[name] = provider.IsStreamingConnected()
	}

	// Count unique providers
	allProviders := make(map[string]bool)
	for name := range sm.quoteProviders {
		allProviders[name] = true
	}
	for name := range sm.greeksProviders {
		allProviders[name] = true
	}

	// Get health manager stats
	healthStatus := sm.healthManager.GetHealthStatus()

	return map[string]interface{}{
		"is_connected":     sm.isConnected,
		"quote_providers":  quoteProviders,
		"greeks_providers": greeksProviders,
		"total_providers":  len(allProviders),
		"subscription_counts": map[string]int{
			"quotes": len(sm.quoteSubscriptions),
			"greeks": len(sm.greeksSubscriptions),
		},
		"health": healthStatus,
	}
}

// Disconnect disconnects all streaming providers.
// Enhanced with health monitoring shutdown.
func (sm *StreamingManager) Disconnect(ctx context.Context) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	return sm.disconnectInternal(ctx)
}

// disconnectInternal performs the disconnect without acquiring the lock (must be called with lock held)
func (sm *StreamingManager) disconnectInternal(ctx context.Context) error {
	slog.Info("Disconnecting streaming manager...")

	// Stop health monitoring
	slog.Info("🔄 [Disconnect] Stopping health monitoring...")
	sm.healthManager.StopMonitoring()
	slog.Info("✅ [Disconnect] Health monitoring stopped")

	// Safely close shutdown event channel
	slog.Info("🔄 [Disconnect] Closing shutdown event...")
	select {
	case <-sm.shutdownEvent:
		// Already closed
		slog.Debug("Shutdown event already closed")
	default:
		close(sm.shutdownEvent)
	}
	slog.Info("✅ [Disconnect] Shutdown event closed")

	// Disconnect all providers (avoiding duplicates)
	allProviders := make(map[string]base.Provider)
	for name, provider := range sm.quoteProviders {
		allProviders[name] = provider
	}
	for name, provider := range sm.greeksProviders {
		allProviders[name] = provider
	}

	slog.Info("🔄 [Disconnect] Disconnecting providers...", "count", len(allProviders))
	for name, provider := range allProviders {
		slog.Info("🔄 [Disconnect] Disconnecting provider", "provider", name)
		if _, err := provider.DisconnectStreaming(ctx); err != nil {
			slog.Error("Error disconnecting provider", "provider", name, "error", err)
		}
		slog.Info("✅ [Disconnect] Provider disconnected", "provider", name)
	}

	// Clear all state
	sm.quoteProviders = make(map[string]base.Provider)
	sm.greeksProviders = make(map[string]base.Provider)
	sm.quoteSubscriptions = make(map[string]bool)
	sm.greeksSubscriptions = make(map[string]bool)
	sm.isConnected = false

	slog.Info("✅ [Disconnect] Streaming manager disconnected")
	return nil
}

// RestartWithNewConfig restarts streaming connections with new configuration while preserving subscriptions.
// This method returns immediately and performs reconnection asynchronously.
func (sm *StreamingManager) RestartWithNewConfig(ctx context.Context) error {
	slog.Info("🔄 [RestartWithNewConfig] Acquiring lock...")
	sm.lock.Lock()
	slog.Info("✅ [RestartWithNewConfig] Lock acquired")
	defer func() {
		sm.lock.Unlock()
		slog.Info("✅ [RestartWithNewConfig] Lock released")
	}()

	slog.Info("Restarting streaming manager with new configuration...")

	// Store current subscriptions to restore them after restart
	currentQuoteSubscriptions := make(map[string]bool)
	for symbol := range sm.quoteSubscriptions {
		currentQuoteSubscriptions[symbol] = true
	}

	currentGreeksSubscriptions := make(map[string]bool)
	for symbol := range sm.greeksSubscriptions {
		currentGreeksSubscriptions[symbol] = true
	}

	slog.Info("Preserving subscriptions", "quotes", len(currentQuoteSubscriptions), "greeks", len(currentGreeksSubscriptions))

	// Disconnect all current connections synchronously (should be fast)
	// Use internal method since we already hold the lock
	slog.Info("🔄 [RestartWithNewConfig] Calling disconnectInternal...")
	if err := sm.disconnectInternal(ctx); err != nil {
		slog.Warn("Error during disconnect", "error", err)
	}
	slog.Info("✅ [RestartWithNewConfig] Disconnect completed")

	// Reset shutdown event for reconnection
	sm.shutdownEvent = make(chan struct{})

	// Return immediately, perform reconnection asynchronously
	slog.Info("🔄 [RestartWithNewConfig] Starting async reconnection...")
	go sm.asyncReconnectWithSubscriptions(ctx, currentQuoteSubscriptions, currentGreeksSubscriptions)

	slog.Info("✅ [RestartWithNewConfig] Returning immediately")
	return nil
}

// asyncReconnectWithSubscriptions handles reconnection and subscription restoration in background.
func (sm *StreamingManager) asyncReconnectWithSubscriptions(
	ctx context.Context,
	quoteSubs map[string]bool,
	greeksSubs map[string]bool,
) {
	// Reconnect with new configuration (may take time)
	if err := sm.Connect(ctx); err != nil {
		slog.Error("Failed to reconnect streaming manager", "error", err)
		return
	}

	// Restore subscriptions asynchronously
	if len(quoteSubs) > 0 || len(greeksSubs) > 0 {
		slog.Info("Restoring subscriptions asynchronously...", "quotes", len(quoteSubs), "greeks", len(greeksSubs))

		restoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Restore quote subscriptions
		if len(quoteSubs) > 0 {
			if err := sm.updateQuoteSubscriptions(restoreCtx, quoteSubs); err != nil {
				slog.Error("Failed to restore quote subscriptions", "error", err)
			} else {
				slog.Info("Restored quote subscriptions", "count", len(quoteSubs))
			}
		}

		// Restore Greeks subscriptions
		if len(greeksSubs) > 0 {
			if err := sm.updateGreeksSubscriptions(restoreCtx, greeksSubs); err != nil {
				slog.Error("Failed to restore Greeks subscriptions", "error", err)
			} else {
				slog.Info("Restored Greeks subscriptions", "count", len(greeksSubs))
			}
		}

		slog.Info("Subscription restoration completed")
	}
}

// GetLatestCache returns the latest value cache
func (sm *StreamingManager) GetLatestCache() *LatestValueCache {
	return sm.latestCache
}

// AddSymbolsToSubscriptions adds symbols to the existing subscriptions without replacing them.
// This is useful for automation/background tasks that need to subscribe without disrupting other clients.
func (sm *StreamingManager) AddSymbolsToSubscriptions(ctx context.Context, symbols []string) error {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if len(symbols) == 0 {
		return nil
	}

	// Add to existing subscriptions
	optionSymbols := make([]string, 0)
	stockSymbols := make([]string, 0)

	for _, symbol := range symbols {
		if sm.isOptionSymbol(symbol) {
			if !sm.greeksSubscriptions[symbol] {
				sm.greeksSubscriptions[symbol] = true
				optionSymbols = append(optionSymbols, symbol)
			}
		}
		if !sm.quoteSubscriptions[symbol] {
			sm.quoteSubscriptions[symbol] = true
			if sm.isOptionSymbol(symbol) {
				optionSymbols = append(optionSymbols, symbol)
			} else {
				stockSymbols = append(stockSymbols, symbol)
			}
		}
	}

	// Subscribe to new quotes (all symbols need quotes)
	if len(stockSymbols) > 0 || len(optionSymbols) > 0 {
		allQuoteSymbols := make([]string, 0, len(sm.quoteSubscriptions))
		for s := range sm.quoteSubscriptions {
			allQuoteSymbols = append(allQuoteSymbols, s)
		}
		if err := sm.subscribeQuotesSafe(ctx, allQuoteSymbols); err != nil {
			slog.Warn("Failed to subscribe quotes for added symbols", "error", err)
		}
	}

	// Subscribe to new Greeks (options only)
	if len(optionSymbols) > 0 {
		allGreeksSymbols := make([]string, 0, len(sm.greeksSubscriptions))
		for s := range sm.greeksSubscriptions {
			allGreeksSymbols = append(allGreeksSymbols, s)
		}
		if err := sm.subscribeGreeksSafe(ctx, allGreeksSymbols); err != nil {
			slog.Warn("Failed to subscribe Greeks for added symbols", "error", err)
		}
	}

	slog.Debug("Added symbols to subscriptions", "added_quotes", len(stockSymbols)+len(optionSymbols), "added_greeks", len(optionSymbols))
	return nil
}

// Singleton instance
var streamingManagerInstance *StreamingManager
var streamingManagerOnce sync.Once

// GetStreamingManager returns the singleton streaming manager instance
func GetStreamingManager() *StreamingManager {
	streamingManagerOnce.Do(func() {
		streamingManagerInstance = NewStreamingManager()
	})
	return streamingManagerInstance
}
