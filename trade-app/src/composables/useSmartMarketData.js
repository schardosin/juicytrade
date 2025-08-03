import { inject } from "vue";
import smartMarketDataStore from "../services/smartMarketDataStore.js";

/**
 * Smart Market Data Composable
 *
 * Provides easy access to the Smart Market Data Store for Vue components.
 * Components can simply call getStockPrice() or getOptionPrice() and get
 * reactive data with automatic subscription management.
 *
 * Usage:
 * ```javascript
 * const { getStockPrice, getOptionPrice } = useSmartMarketData();
 * const aaplPrice = getStockPrice('AAPL');
 * const optionPrice = getOptionPrice('AAPL250718C00150000');
 * ```
 */
export function useSmartMarketData() {
  // Ensure store is initialized (safe to call multiple times)
  smartMarketDataStore.initialize();

  /**
   * Get reactive stock price for a symbol
   * @param {string} symbol - Stock symbol (e.g., 'AAPL', 'SPY')
   * @returns {ComputedRef} Reactive price data object or null
   */
  const getStockPrice = (symbol) => {
    return smartMarketDataStore.getStockPrice(symbol);
  };

  /**
   * Get reactive option price for a symbol
   * @param {string} symbol - Option symbol (e.g., 'AAPL250718C00150000')
   * @returns {ComputedRef} Reactive price data object or null
   */
  const getOptionPrice = (symbol) => {
    return smartMarketDataStore.getOptionPrice(symbol);
  };

  /**
   * Get price for any symbol (auto-detects stock vs option)
   * @param {string} symbol - Any symbol
   * @returns {ComputedRef} Reactive price data object or null
   */
  const getPrice = (symbol) => {
    if (!symbol) return null;

    // Auto-detect symbol type
    if (smartMarketDataStore.isOptionSymbol(symbol)) {
      return getOptionPrice(symbol);
    } else {
      return getStockPrice(symbol);
    }
  };

  /**
   * Get debug information about current subscriptions
   * @returns {Object} Debug information
   */
  const getDebugInfo = () => {
    return smartMarketDataStore.getDebugInfo();
  };

  /**
   * Get previous close price for a symbol
   * @param {string} symbol - Stock symbol (e.g., 'AAPL', 'SPY')
   * @returns {ComputedRef} Reactive previous close price or null
   */
  const getPreviousClose = (symbol) => {
    return smartMarketDataStore.getPreviousClose(symbol);
  };

  /**
   * Force cleanup of all subscriptions (for testing)
   */
  const forceCleanup = () => {
    smartMarketDataStore.forceCleanup();
  };

  return {
    // Main methods
    getStockPrice,
    getOptionPrice,
    getPrice,
    getPreviousClose,

    // Utility methods
    getDebugInfo,
    forceCleanup,
  };
}

export default useSmartMarketData;
