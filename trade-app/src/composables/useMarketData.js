import { computed } from "vue";
import smartMarketDataStore from "../services/smartMarketDataStore.js";

/**
 * Unified Market Data Composable
 *
 * Provides consistent interface for all data regardless of source strategy:
 * - WebSocket data (real-time prices) - existing functionality
 * - Periodic API data (account balance, positions) - auto-refreshing
 * - Cached API data (account info, user profile) - one-time fetch
 * - On-demand API data (historical data, options chains) - TTL caching
 *
 * Components use identical patterns regardless of data source strategy.
 */
export function useMarketData() {
  // Data sources are now configured at app startup in main.js
  // No need to configure here anymore

  // ===== WEBSOCKET DATA (Real-time) =====

  /**
   * Get reactive stock price for a symbol
   * Uses existing WebSocket functionality
   */
  const getStockPrice = (symbol) => {
    return smartMarketDataStore.getStockPrice(symbol);
  };

  /**
   * Get reactive option price for a symbol
   * Uses existing WebSocket functionality
   */
  const getOptionPrice = (symbol) => {
    return smartMarketDataStore.getOptionPrice(symbol);
  };

  /**
   * Get reactive option Greeks for a symbol
   * Uses streaming updates (same pattern as prices)
   */
  const getOptionGreeks = (symbol) => {
    return smartMarketDataStore.getOptionGreeks(symbol);
  };

  /**
   * Get reactive IVx data for a symbol - STREAMING-based approach (RECOMMENDED)
   * Uses WebSocket streaming with automatic subscription management
   * Same pattern as getOptionGreeks() for consistency
   */
  const getIvxDataStreaming = (symbol) => {
    return smartMarketDataStore.getIvxDataStreaming(symbol);
  };

  /**
   * Get reactive IVx data for a symbol - LEGACY API-based approach
   * Uses TTL caching for optimal performance
   * DEPRECATED: Use getIvxDataStreaming() instead
   */
  const getIvxData = (symbol) => {
    return smartMarketDataStore.getIvxData(symbol);
  };

  // ===== PERIODIC DATA (Auto-refreshing) =====

  /**
   * Get account balance (updates every 60 seconds)
   */
  const getBalance = () => {
    return smartMarketDataStore.getReactiveData("balance");
  };

  /**
   * Get positions (updates every 30 seconds)
   */
  const getPositions = () => {
    return smartMarketDataStore.getReactiveData("positions");
  };

  /**
   * Get positions filtered by symbol (reactive computed)
   */
  const getPositionsForSymbol = (symbol) => {
    // Return the reactive computed directly from the store
    return smartMarketDataStore.getFilteredPositions(symbol);
  };

  /**
   * Force refresh positions data
   * Useful when switching symbols to ensure we have the latest data
   */
  const refreshPositionsData = async () => {
    return await smartMarketDataStore.refreshPositions();
  };

  /**
   * Get orders by status (On-Demand Fresh strategy - always fresh data)
   */
  const getOrdersByStatus = async (status = "all") => {
    return await smartMarketDataStore.getOrdersByStatus(status);
  };

  // ===== ONE-TIME DATA (Static cached) =====

  /**
   * Get account info (fetched once and cached)
   */
  const getAccountInfo = () => {
    return smartMarketDataStore.getReactiveData("accountInfo");
  };

  // ===== ON-DEMAND DATA (TTL cached) =====

  /**
   * Get options chain (cached for 5 minutes)
   */
  const getOptionsChain = async (symbol, expiry, forceRefresh = false) => {
    const key = `optionsChain.${symbol}.${expiry}`;
    return await smartMarketDataStore.getData(key, { forceRefresh });
  };

  /**
   * Get historical data (cached for 5 minutes)
   */
  const getHistoricalData = async (symbol, timeframe, options = {}) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    
    // Call API directly with all parameters to ensure start_date is passed
    return await api.default.getHistoricalData(symbol, timeframe, options);
  };

  /**
   * Get 6 months of daily candles with cache and current-day merge
   * Specialized fast path used by chart default (Daily + 6M)
   */
  const getHistoricalDailySixMonths = async (symbol, options = {}) => {
    return await smartMarketDataStore.getDaily6MHistory(symbol, options);
  };

  /**
   * Symbol lookup - call API directly (not through data store)
   */
  const lookupSymbols = async (query, forceRefresh = false) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    return await api.default.lookupSymbols(query);
  };

  /**
   * Get expiration dates (cached for 1 hour)
   */
  const getExpirationDates = async (symbol, forceRefresh = false) => {
    const key = `expirationDates.${symbol}`;
    return await smartMarketDataStore.getData(key, { forceRefresh });
  };

  /**
   * Get available expiration dates for options (cached)
   */
  const getAvailableExpirations = async (symbol, forceRefresh = false) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    return await api.default.getAvailableExpirations(symbol);
  };

  /**
   * Get basic options chain (cached for 5 minutes)
   */
  const getOptionsChainBasic = async (symbol, expiry, underlyingPrice = null, strikeCount = 20, type = null) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    return await api.default.getOptionsChainBasic(symbol, expiry, underlyingPrice, strikeCount, type);
  };

  /**
   * Get previous close price (cached)
   */
  const getPreviousClose = async (symbol, forceRefresh = false) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    return await api.default.getPreviousClose(symbol);
  };

  /**
   * Get 52-week range (cached for 1 hour)
   */
  const get52WeekRange = async (symbol, forceRefresh = false) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    return await api.default.get52WeekRange(symbol);
  };

  /**
   * Get average volume (cached for 1 hour)
   */
  const getAverageVolume = async (symbol, days = 20, forceRefresh = false) => {
    // Import API service dynamically to avoid circular dependencies
    const api = await import('../services/api.js');
    return await api.default.getAverageVolume(symbol, days);
  };

  // ===== PROVIDER CONFIGURATION DATA =====

  /**
   * Get available providers (cached for 5 minutes)
   */
  const getAvailableProviders = () => {
    return smartMarketDataStore.getReactiveData("providers.available");
  };

  /**
   * Get current provider configuration (updates every 30 seconds)
   */
  const getProviderConfig = () => {
    return smartMarketDataStore.getReactiveData("providers.config");
  };

  /**
   * Force refresh provider data
   */
  const refreshProviderData = async () => {
    await smartMarketDataStore.getData("providers.available", { forceRefresh: true });
    await smartMarketDataStore.getData("providers.config", { forceRefresh: true });
  };

  /**
   * Update provider configuration
   */
  const updateProviderConfig = async (newConfig) => {
    return await smartMarketDataStore.updateProviderConfig(newConfig);
  };

  // ===== UTILITY METHODS =====

  /**
   * Check if data is loading
   */
  const isLoading = (key) => {
    return smartMarketDataStore.isLoading(key);
  };

  /**
   * Get error state for data
   */
  const getError = (key) => {
    return smartMarketDataStore.getError(key);
  };

  /**
   * Manual refresh methods for periodic data
   */
  const refreshBalance = () =>
    smartMarketDataStore.getData("balance", { forceRefresh: true });
  const refreshPositions = () =>
    smartMarketDataStore.getData("positions", { forceRefresh: true });
  const refreshOrders = () =>
    smartMarketDataStore.getData("orders", { forceRefresh: true });

  /**
   * Get debug information (for development)
   */
  const getDebugInfo = () => {
    return smartMarketDataStore.getDebugInfo();
  };

  /**
   * Get preloaded Overview data for a symbol
   * Returns static data that was preloaded when symbol was selected
   */
  const getOverviewData = (symbol) => {
    return smartMarketDataStore.getOverviewData(symbol);
  };

  /**
   * Force refresh Overview data for a symbol
   * Useful for manual refresh of static data
   */
  const refreshOverviewData = async (symbol) => {
    return await smartMarketDataStore.refreshOverviewData(symbol);
  };

  return {
    // Real-time data (WebSocket)
    getStockPrice,
    getOptionPrice,
    getOptionGreeks,
    getIvxDataStreaming, // NEW: Streaming-based IVx (recommended)
    getIvxData, // LEGACY: API-based IVx (deprecated)

    // Auto-updating data (Periodic)
    getBalance,
    getPositions,
    getPositionsForSymbol,

    // On-demand fresh data (Orders)
    getOrdersByStatus,

    // Static data (One-time)
    getAccountInfo,

    // On-demand data (Cached)
    getOptionsChain,
    getHistoricalData,
    getHistoricalDailySixMonths,
    lookupSymbols,
    getExpirationDates,
    getAvailableExpirations,
    getOptionsChainBasic,
    getPreviousClose,
    get52WeekRange,
    getAverageVolume,

    // Overview data (Preloaded static data)
    getOverviewData,
    refreshOverviewData,

    // Provider configuration data
    getAvailableProviders,
    getProviderConfig,
    refreshProviderData,
    updateProviderConfig,

    // Utility methods
    isLoading,
    getError,
    refreshBalance,
    refreshPositions,
    getDebugInfo,
  };
}

export default useMarketData;
