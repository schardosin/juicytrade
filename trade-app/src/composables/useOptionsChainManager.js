import { ref, computed, watch, nextTick } from "vue";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";

// Load persisted strike count from localStorage
const loadPersistedStrikeCount = () => {
  try {
    const saved = localStorage.getItem("globalStrikeCount");
    if (saved) {
      const parsed = parseInt(saved, 10);
      if (!isNaN(parsed) && parsed > 0) {
        return parsed;
      }
    }
  } catch (error) {
    console.warn("Failed to load persisted strike count:", error);
  }
  return 20; // Default fallback
};

// Save strike count to localStorage
const persistStrikeCount = (strikeCount) => {
  try {
    localStorage.setItem("globalStrikeCount", strikeCount.toString());
  } catch (error) {
    console.warn("Failed to persist strike count:", error);
  }
};

/**
 * Centralized Options Chain Data Manager
 *
 * Manages all options data across multiple expirations with smart websocket subscriptions.
 * Provides unified data access for all components that need options data.
 */
export function useOptionsChainManager(
  symbol,
  underlyingPrice,
  initialStrikeCount = 20
) {
  // ===== Core State =====

  // Raw options data organized by expiration date
  const dataByExpiration = ref({});

  // Set of currently expanded (visible) expirations
  const expandedExpirations = ref(new Set());

  // Set of currently subscribed option symbols
  const subscribedSymbols = ref(new Set());

  // Loading and error states
  const loading = ref(false);
  const error = ref(null);

  // Available expiration dates
  const expirationDates = ref([]);

  // Strike count as reactive ref - load from localStorage or use initial value
  const strikeCount = ref(loadPersistedStrikeCount());

  // ===== Computed Properties =====

  // Flattened view of all loaded options data for easy lookup
  const flattenedData = computed(() => {
    const allOptions = [];
    for (const [expiration, options] of Object.entries(
      dataByExpiration.value
    )) {
      if (options && Array.isArray(options)) {
        allOptions.push(
          ...options.map((option) => ({
            ...option,
            expiration_date: expiration,
          }))
        );
      }
    }
    return allOptions;
  });

  // Quick lookup map: symbol -> option data
  const symbolToOption = computed(() => {
    const map = new Map();
    flattenedData.value.forEach((option) => {
      if (option.symbol) {
        map.set(option.symbol, option);
      }
    });
    return map;
  });

  // Quick lookup map: symbol -> expiration date
  const symbolToExpiration = computed(() => {
    const map = new Map();
    flattenedData.value.forEach((option) => {
      if (option.symbol && option.expiration_date) {
        map.set(option.symbol, option.expiration_date);
      }
    });
    return map;
  });

  // Get all currently subscribed option symbols
  const allSubscribedSymbols = computed(() => {
    const symbols = [];
    expandedExpirations.value.forEach((expiration) => {
      const options = dataByExpiration.value[expiration];
      if (options && Array.isArray(options)) {
        options.forEach((option) => {
          if (option.symbol) {
            symbols.push(option.symbol);
          }
        });
      }
    });
    return symbols;
  });

  // ===== Core Methods =====

  /**
   * Load expiration dates for the current symbol
   */
  const loadExpirationDates = async () => {
    if (!symbol.value) return;

    try {
      loading.value = true;
      error.value = null;

      const dates = await api.getAvailableExpirations(symbol.value);
      if (dates && dates.length > 0) {
        expirationDates.value = dates;
      } else {
        expirationDates.value = [];
        error.value = "No expiration dates available";
      }
    } catch (err) {
      console.error("Error loading expiration dates:", err);
      error.value = "Failed to load expiration dates";
      expirationDates.value = [];
    } finally {
      loading.value = false;
    }
  };

  /**
   * Load options data for a specific expiration
   */
  const loadOptionsForExpiration = async (expiration) => {
    if (!symbol.value || !expiration) return;

    try {
      // Set loading state for this expiration
      dataByExpiration.value = {
        ...dataByExpiration.value,
        [expiration]: [], // Empty array indicates loading
      };

      const chain = await api.getOptionsChainBasic(
        symbol.value,
        expiration,
        underlyingPrice.value,
        strikeCount.value
      );

      if (chain && chain.length > 0) {
        const processedChain = chain.map((option) => ({
          ...option,
          strike_price: parseFloat(option.strike_price),
          bid: parseFloat(option.bid || option.close_price || 0),
          ask: parseFloat(option.ask || option.close_price || 0),
          expiration_date: expiration,
        }));

        // Store the processed data
        dataByExpiration.value = {
          ...dataByExpiration.value,
          [expiration]: processedChain,
        };

      } else {
        // No options found
        dataByExpiration.value = {
          ...dataByExpiration.value,
          [expiration]: [],
        };
        console.log(`⚠️ No options found for ${expiration}`);
      }
    } catch (err) {
      console.error(`Error loading options for ${expiration}:`, err);
      dataByExpiration.value = {
        ...dataByExpiration.value,
        [expiration]: [],
      };
    }
  };

  /**
   * Expand an expiration (load data and subscribe to prices)
   */
  const expandExpiration = async (expiration) => {
    // Add to expanded set
    expandedExpirations.value.add(expiration);

    // Load data if not already loaded
    if (!dataByExpiration.value[expiration]) {
      await loadOptionsForExpiration(expiration);
    }

    // Update websocket subscriptions
    await updateSubscriptions();
  };

  /**
   * Collapse an expiration (unsubscribe from prices, optionally keep data)
   */
  const collapseExpiration = async (expiration) => {
    // Remove from expanded set
    expandedExpirations.value.delete(expiration);

    // Update websocket subscriptions
    await updateSubscriptions();
  };

  /**
   * Update websocket subscriptions based on expanded expirations
   */
  const updateSubscriptions = async () => {
    const newSymbols = allSubscribedSymbols.value;

    // The first symbol should be the underlying
    const allSymbols = [symbol.value, ...newSymbols];

    if (allSymbols.length > 1) { // Only subscribe if we have more than just the underlying
      webSocketClient.replaceAllSubscriptions(allSymbols);
    } else {
      // If only the underlying is left, we can clear option subscriptions
      webSocketClient.replaceAllSubscriptions([symbol.value]);
    }

    // Update our tracking set
    subscribedSymbols.value = new Set(newSymbols);
  };

  /**
   * Update option price from websocket data
   */
  const updateOptionPrice = (optionSymbol, priceData) => {
    // Find which expiration this option belongs to
    const expiration = symbolToExpiration.value.get(optionSymbol);
    if (!expiration) return;

    const options = dataByExpiration.value[expiration];
    if (!options || !Array.isArray(options)) return;

    const optionIndex = options.findIndex((opt) => opt.symbol === optionSymbol);
    if (optionIndex === -1) return;

    // Update the option with new price data
    const updatedOption = {
      ...options[optionIndex],
      bid: priceData.bid || options[optionIndex].bid,
      ask: priceData.ask || options[optionIndex].ask,
    };

    // Create new array with updated option
    const newOptions = [...options];
    newOptions[optionIndex] = updatedOption;

    // Update the state
    dataByExpiration.value = {
      ...dataByExpiration.value,
      [expiration]: newOptions,
    };
  };

  /**
   * Get option data by symbol (unified lookup)
   */
  const getOptionBySymbol = (optionSymbol) => {
    return symbolToOption.value.get(optionSymbol) || null;
  };

  /**
   * Get all options for a specific expiration
   */
  const getOptionsForExpiration = (expiration) => {
    return dataByExpiration.value[expiration] || [];
  };

  /**
   * Clear all data (for symbol changes)
   */
  const clearAllData = () => {
    dataByExpiration.value = {};
    expandedExpirations.value.clear();
    subscribedSymbols.value.clear();
    expirationDates.value = [];
    error.value = null;
    // After clearing data, wait for the next DOM update cycle before updating subscriptions.
    // This ensures that computed properties like `allSubscribedSymbols` have been recalculated.
    nextTick(() => {
      updateSubscriptions();
    });
  };

  /**
   * Update strike count and reload expanded expirations
   */
  const updateStrikeCount = async (newStrikeCount) => {
    console.log(`🎯 Updating strike count to: ${newStrikeCount}`);
    strikeCount.value = newStrikeCount;

    // Persist the new strike count to localStorage
    persistStrikeCount(newStrikeCount);

    // Reload all expanded expirations with new strike count
    const expandedList = Array.from(expandedExpirations.value);
    for (const expiration of expandedList) {
      await loadOptionsForExpiration(expiration);
    }

    // Update subscriptions
    await updateSubscriptions();
  };

  /**
   * Refresh all data after system recovery
   */
  const refreshAllData = async () => {
    console.log('🔄 Refreshing all options chain data after recovery...');
    
    try {
      // Reload expiration dates
      await loadExpirationDates();
      
      // Reload all expanded expirations
      const expandedList = Array.from(expandedExpirations.value);
      for (const expiration of expandedList) {
        await loadOptionsForExpiration(expiration);
      }
      
      // Update subscriptions
      await updateSubscriptions();
      
      console.log('✅ Options chain data refresh completed');
    } catch (error) {
      console.error('❌ Error refreshing options chain data:', error);
    }
  };

  // ===== Watchers =====

  // Watch for symbol changes
  watch(
    symbol,
    async (newSymbol, oldSymbol) => {
      if (newSymbol !== oldSymbol) {
        clearAllData();
        if (newSymbol) {
          await loadExpirationDates();
        }
      }
    },
    { immediate: true }
  );

  // ===== Public API =====

  return {
    // State
    dataByExpiration,
    flattenedData,
    expandedExpirations,
    expirationDates,
    loading,
    error,
    strikeCount, // Expose current strike count

    // Computed
    symbolToOption,
    symbolToExpiration,
    allSubscribedSymbols,

    // Methods
    loadExpirationDates,
    loadOptionsForExpiration,
    expandExpiration,
    collapseExpiration,
    updateSubscriptions,
    updateOptionPrice,
    getOptionBySymbol,
    getOptionsForExpiration,
    clearAllData,
    updateStrikeCount,
    refreshAllData,
  };
}
