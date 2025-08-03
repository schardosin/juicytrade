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
   * Uses periodic updates (every 60 seconds)
   */
  const getOptionGreeks = (symbol) => {
    return smartMarketDataStore.getOptionGreeks(symbol);
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
  const getHistoricalData = async (symbol, timeframe, forceRefresh = false) => {
    const key = `historicalData.${symbol}.${timeframe}`;
    return await smartMarketDataStore.getData(key, { forceRefresh });
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

  return {
    // Real-time data (WebSocket)
    getStockPrice,
    getOptionPrice,
    getOptionGreeks,

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
    lookupSymbols,
    getExpirationDates,

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
