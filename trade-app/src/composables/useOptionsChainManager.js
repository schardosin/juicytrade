import { ref, computed, watch, nextTick } from "vue";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";
import { smartMarketDataStore } from "../services/smartMarketDataStore";
import authService from "../services/authService";

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
  const loading = ref(false); // Only true when the entire component should be blocked
  const expirationDatesLoading = ref(false); // Separate loading state for expiration dates
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
    expandedExpirations.value.forEach((expirationKey) => {
      const options = dataByExpiration.value[expirationKey];
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
    if (!symbol.value) {
      return;
    }

    // CRITICAL: Check if services are running before making API calls
    if (smartMarketDataStore.isServicesStopping) {
      console.warn(`⚠️ Options Manager: Skipping expiration dates load for ${symbol.value} - services stopping`);
      return;
    }

    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.warn(`🔒 Options Manager: Skipping expiration dates load for ${symbol.value} - not authenticated`);
      return;
    }

    console.log(`📅 Options Manager: Loading expiration dates for ${symbol.value} (auth: ${authService.isAuthenticated()}, services running: ${smartMarketDataStore.isServicesRunning()})`);

    try {
      expirationDatesLoading.value = true; // Use separate loading state
      error.value = null;

      const dates = await api.getAvailableExpirations(symbol.value);
      
      if (dates && dates.length > 0) {
        expirationDates.value = dates;
        console.log(`✅ Options Manager: Successfully loaded ${dates.length} expiration dates for ${symbol.value}`);
      } else {
        expirationDates.value = [];
        error.value = "No expiration dates available";
        console.warn(`⚠️ Options Manager: No expiration dates available for ${symbol.value}`);
      }
    } catch (err) {
      console.error(`❌ Options Manager: Error loading expiration dates for ${symbol.value}:`, err);
      error.value = "Failed to load expiration dates";
      expirationDates.value = [];
    } finally {
      expirationDatesLoading.value = false;
    }
  };

  /**
   * Load options data for a specific expiration using the correct symbol and type
   */
  const loadOptionsForExpiration = async (expirationKey, expirationData) => {
    if (!expirationKey || !expirationData) return;
  
    const { date, symbol: requestSymbol, type: requestType } = expirationData;
  
    try {
      // Set loading state for this expiration
      dataByExpiration.value = {
        ...dataByExpiration.value,
        [expirationKey]: [], // Empty array indicates loading
      };
  
      const chain = await api.getOptionsChainBasic(
        requestSymbol,
        date,
        underlyingPrice.value,
        strikeCount.value,
        requestType,
        symbol.value
      );
  
      if (chain && chain.length > 0) {
        const processedChain = chain.map((option) => ({
          ...option,
          strike_price: parseFloat(option.strike_price),
          bid: parseFloat(option.bid || option.close_price || 0),
          ask: parseFloat(option.ask || option.close_price || 0),
          expiration_date: date,
        }));
  
        // Store the processed data
        dataByExpiration.value = {
          ...dataByExpiration.value,
          [expirationKey]: processedChain,
        };
      } else {
        // No options found
        dataByExpiration.value = {
          ...dataByExpiration.value,
          [expirationKey]: [],
        };
      }
    } catch (err) {
      console.error(`Error loading options for ${expirationKey}:`, err);
      dataByExpiration.value = {
        ...dataByExpiration.value,
        [expirationKey]: [],
      };
    }
  };

  /**
   * Expand an expiration (load data and subscribe to prices)
   */
  const expandExpiration = async (expirationKey, expirationData = null) => {
    // Add to expanded set
    expandedExpirations.value.add(expirationKey);
  
    // Load data if not already loaded
    if (!dataByExpiration.value[expirationKey]) {
      await loadOptionsForExpiration(expirationKey, expirationData);
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
    // CRITICAL FIX: Don't update subscriptions if services are stopping or user not authenticated
    if (smartMarketDataStore.isServicesStopping) {
      console.log("🚫 Skipping subscription update - services are stopping");
      return;
    }
    
    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.log("🔒 Skipping subscription update - user not authenticated");
      return;
    }

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
  const getOptionsForExpiration = (expirationKey) => {
    return dataByExpiration.value[expirationKey] || [];
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
    strikeCount.value = newStrikeCount;

    // Persist the new strike count to localStorage
    persistStrikeCount(newStrikeCount);

    // Reload all expanded expirations with new strike count
    const expandedList = Array.from(expandedExpirations.value);
    for (const expirationKey of expandedList) {
      // Find the corresponding expiration data from expirationDates
      const expirationData = expirationDates.value.find(exp => {
        const uniqueKey = `${exp.date}-${exp.type}-${exp.symbol}`;
        return uniqueKey === expirationKey;
      });
      
      if (expirationData) {
        await loadOptionsForExpiration(expirationKey, expirationData);
      }
    }

    // Update subscriptions
    await updateSubscriptions();
  };

  /**
   * Refresh all data after system recovery
   */
  const refreshAllData = async () => {    
    try {
      // Reload expiration dates
      await loadExpirationDates();
      
      // Reload all expanded expirations
      const expandedList = Array.from(expandedExpirations.value);
      for (const expirationKey of expandedList) {
        // Find the corresponding expiration data from expirationDates
        const expirationData = expirationDates.value.find(exp => {
          const uniqueKey = `${exp.date}-${exp.type}-${exp.symbol}`;
          return uniqueKey === expirationKey;
        });
        
        if (expirationData) {
          await loadOptionsForExpiration(expirationKey, expirationData);
        }
      }
      
      // Update subscriptions
      await updateSubscriptions();
      
    } catch (error) {
      console.error('❌ Error refreshing options chain data:', error);
    }
  };

    /**
   * Move strike price up or down for a given option symbol.
   * @param {string} optionSymbol - The symbol of the option to move.
   * @param {string} direction - 'up' (lower strike) or 'down' (higher strike).
   * @returns {Object|null} The new option data or null if not found.
   */
  const moveStrike = (optionSymbol, direction) => {
    const currentOption = getOptionBySymbol(optionSymbol);
    if (!currentOption) {
      console.warn(`moveStrike: Option not found for symbol ${optionSymbol}`);
      return null;
    }

    const { expiration_date, type, strike_price } = currentOption;
    const allOptionsForExpiration = getOptionsForExpiration(expiration_date);

    const isCall = type.toLowerCase().startsWith('c');

    const sameTypeStrikes = allOptionsForExpiration
      .filter(opt => opt.type.toLowerCase().startsWith(isCall ? 'c' : 'p'))
      .sort((a, b) => a.strike_price - b.strike_price);

    const currentIndex = sameTypeStrikes.findIndex(opt => opt.strike_price === strike_price);

    if (currentIndex === -1) {
      console.warn(`moveStrike: Current strike not found in chain for ${optionSymbol}`);
      return null;
    }

    let newIndex;
    if (direction === 'up') { // Lower strike
      newIndex = currentIndex - 1;
    } else { // 'down', higher strike
      newIndex = currentIndex + 1;
    }

    if (newIndex >= 0 && newIndex < sameTypeStrikes.length) {
      return sameTypeStrikes[newIndex];
    }

    console.log(`moveStrike: No further strike available in direction '${direction}' for ${optionSymbol}`);
    return null;
  };

    // ===== Watchers =====
  
    // Watch for symbol changes - NON-BLOCKING VERSION
    watch(
    symbol,
    (newSymbol, oldSymbol) => {
      if (newSymbol !== oldSymbol && newSymbol) {
        clearAllData();
        // Load expiration dates asynchronously in next tick to avoid blocking
        nextTick(() => {
          loadExpirationDates().catch(error => {
            console.error(`Failed to load expiration dates for ${newSymbol}:`, error);
          });
        });
      }
    }
    // Remove immediate: true to avoid blocking on component creation
  );

  // Load initial data for the current symbol if it exists
  if (symbol.value) {
    nextTick(() => {
      loadExpirationDates().catch(error => {
        console.error(`Failed to load initial expiration dates for ${symbol.value}:`, error);
      });
    });
  }

  // ===== Public API =====

  return {
    // State
    dataByExpiration,
    flattenedData,
    expandedExpirations,
    expirationDates,
    loading,
    expirationDatesLoading, // Expose separate loading state
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
    moveStrike,
  };
}
