import { computed, onMounted } from "vue";
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
  // Configure data sources on first use
  onMounted(() => {
    configureDataSources();
  });

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
   * Get orders (updates every 15 seconds)
   */
  const getOrders = () => {
    return smartMarketDataStore.getReactiveData("orders");
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
   * Symbol lookup (cached for 10 minutes)
   */
  const lookupSymbols = async (query, forceRefresh = false) => {
    const key = `symbolLookup.${query}`;
    return await smartMarketDataStore.getData(key, { forceRefresh });
  };

  /**
   * Get expiration dates (cached for 1 hour)
   */
  const getExpirationDates = async (symbol, forceRefresh = false) => {
    const key = `expirationDates.${symbol}`;
    return await smartMarketDataStore.getData(key, { forceRefresh });
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

    // Auto-updating data (Periodic)
    getBalance,
    getPositions,
    getOrders,

    // Static data (One-time)
    getAccountInfo,

    // On-demand data (Cached)
    getOptionsChain,
    getHistoricalData,
    lookupSymbols,
    getExpirationDates,

    // Utility methods
    isLoading,
    getError,
    refreshBalance,
    refreshPositions,
    refreshOrders,
    getDebugInfo,
  };
}

/**
 * Configure all data sources with their strategies
 * Called automatically on first use
 */
function configureDataSources() {
  // Skip if already configured
  if (smartMarketDataStore.strategies.size > 0) {
    return;
  }

  // Auto-updating data (Periodic strategy)
  smartMarketDataStore.registerDataSource("balance", {
    strategy: "periodic",
    method: "getAccount",
    interval: 60000, // 1 minute
  });

  smartMarketDataStore.registerDataSource("positions", {
    strategy: "periodic",
    method: "getPositions",
    interval: 30000, // 30 seconds
  });

  smartMarketDataStore.registerDataSource("orders", {
    strategy: "periodic",
    method: "getOrders",
    interval: 15000, // 15 seconds
  });

  // Static data (One-time strategy)
  smartMarketDataStore.registerDataSource("accountInfo", {
    strategy: "once",
    method: "getAccount",
  });

  // On-demand data (Cached strategy)
  smartMarketDataStore.registerDataSource("optionsChain.*", {
    strategy: "on-demand",
    method: "getOptionsChain",
    ttl: 300000, // 5 minutes
  });

  smartMarketDataStore.registerDataSource("historicalData.*", {
    strategy: "on-demand",
    method: "getHistoricalData",
    ttl: 300000, // 5 minutes
  });

  smartMarketDataStore.registerDataSource("symbolLookup.*", {
    strategy: "on-demand",
    method: "lookupSymbols",
    ttl: 600000, // 10 minutes
  });

  smartMarketDataStore.registerDataSource("expirationDates.*", {
    strategy: "on-demand",
    method: "getAvailableExpirations",
    ttl: 3600000, // 1 hour
  });

  console.log("📋 All data sources configured");
}

export default useMarketData;
