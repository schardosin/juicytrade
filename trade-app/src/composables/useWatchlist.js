import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import api from "../services/api.js";
import { useSmartMarketData } from "./useSmartMarketData.js";
import notificationService from "../services/notificationService.js";

/**
 * Watchlist Composable
 * 
 * Provides reactive watchlist management with smart data integration.
 * Automatically subscribes to live price updates for visible symbols.
 */
export function useWatchlist() {
  // Smart market data integration
  const { getStockPrice } = useSmartMarketData();

  // Reactive state
  const watchlists = ref({});
  const activeWatchlistId = ref("default");
  const isLoading = ref(false);
  const error = ref(null);

  // Computed properties
  const activeWatchlist = computed(() => {
    return watchlists.value[activeWatchlistId.value] || null;
  });

  const watchlistOptions = computed(() => {
    return Object.values(watchlists.value).map(wl => ({
      value: wl.id,
      label: wl.name
    }));
  });

  const activeSymbols = computed(() => {
    return activeWatchlist.value?.symbols || [];
  });

  // Get live price data for symbols
  const getSymbolPrice = (symbol) => {
    return getStockPrice(symbol);
  };

  // Get symbol data with live pricing
  const getSymbolData = (symbol) => {
    const priceData = getStockPrice(symbol);
    
    return computed(() => {
      const price = priceData.value;
      
      return {
        symbol,
        price: price?.price || 0,
        bid: price?.bid || 0,
        ask: price?.ask || 0,
        change: 0, // TODO: Calculate change from previous close
        changePercent: 0, // TODO: Calculate change percentage
        timestamp: price?.timestamp || null
      };
    });
  };

  // Load all watchlists
  const loadWatchlists = async () => {
    try {
      isLoading.value = true;
      error.value = null;

      const data = await api.getWatchlists();
      
      watchlists.value = data.watchlists;
      activeWatchlistId.value = data.active_watchlist;

      console.log(`📋 Loaded ${data.total_watchlists} watchlists`);
    } catch (err) {
      error.value = err.message;
      console.error("Error loading watchlists:", err);
      
      notificationService.showError(
        "Failed to load watchlists",
        "Load Error"
      );
    } finally {
      isLoading.value = false;
    }
  };

  // Create a new watchlist
  const createWatchlist = async (name, symbols = []) => {
    try {
      isLoading.value = true;
      
      const data = await api.createWatchlist({
        name,
        symbols
      });

      // Add to local state
      watchlists.value[data.watchlist.id] = data.watchlist;

      notificationService.showSuccess(
        `Watchlist "${name}" created successfully`,
        "Watchlist Created"
      );

      return data.watchlist_id;
    } catch (err) {
      console.error("Error creating watchlist:", err);
      
      notificationService.showError(
        `Failed to create watchlist: ${err.message}`,
        "Creation Failed"
      );
      
      throw err;
    } finally {
      isLoading.value = false;
    }
  };

  // Update a watchlist
  const updateWatchlist = async (watchlistId, updates) => {
    try {
      isLoading.value = true;
      
      const data = await api.updateWatchlist(watchlistId, updates);

      // Update local state
      if (data.watchlist) {
        watchlists.value[watchlistId] = data.watchlist;
      }

      notificationService.showSuccess(
        "Watchlist updated successfully",
        "Watchlist Updated"
      );

      return true;
    } catch (err) {
      console.error("Error updating watchlist:", err);
      
      notificationService.showError(
        `Failed to update watchlist: ${err.message}`,
        "Update Failed"
      );
      
      throw err;
    } finally {
      isLoading.value = false;
    }
  };

  // Delete a watchlist
  const deleteWatchlist = async (watchlistId) => {
    try {
      isLoading.value = true;
      
      const watchlistName = watchlists.value[watchlistId]?.name || watchlistId;
      
      await api.deleteWatchlist(watchlistId);

      // Remove from local state
      delete watchlists.value[watchlistId];

      // If we deleted the active watchlist, switch to another one
      if (activeWatchlistId.value === watchlistId) {
        const remainingIds = Object.keys(watchlists.value);
        activeWatchlistId.value = remainingIds.length > 0 ? remainingIds[0] : null;
      }

      notificationService.showSuccess(
        `Watchlist "${watchlistName}" deleted successfully`,
        "Watchlist Deleted"
      );

      return true;
    } catch (err) {
      console.error("Error deleting watchlist:", err);
      
      notificationService.showError(
        `Failed to delete watchlist: ${err.message}`,
        "Delete Failed"
      );
      
      throw err;
    } finally {
      isLoading.value = false;
    }
  };

  // Add symbol to watchlist
  const addSymbol = async (symbol, watchlistId = null) => {
    const targetWatchlistId = watchlistId || activeWatchlistId.value;
    
    if (!targetWatchlistId) {
      notificationService.showError(
        "No watchlist selected",
        "Add Symbol Failed"
      );
      return false;
    }

    try {
      const cleanSymbol = symbol.trim().toUpperCase();
      
      // Check if symbol already exists
      const targetWatchlist = watchlists.value[targetWatchlistId];
      if (targetWatchlist?.symbols.includes(cleanSymbol)) {
        notificationService.showWarning(
          `${cleanSymbol} already exists in watchlist`,
          "Duplicate Symbol"
        );
        return false;
      }

      const data = await api.addSymbolToWatchlist(targetWatchlistId, cleanSymbol);

      // Update local state
      if (data.watchlist) {
        watchlists.value[targetWatchlistId] = data.watchlist;
      }

      notificationService.showSuccess(
        `${cleanSymbol} added to ${targetWatchlist?.name || 'watchlist'}`,
        "Symbol Added"
      );

      return true;
    } catch (err) {
      console.error("Error adding symbol:", err);
      
      notificationService.showError(
        `Failed to add ${symbol}: ${err.message}`,
        "Add Symbol Failed"
      );
      
      return false;
    }
  };

  // Remove symbol from watchlist
  const removeSymbol = async (symbol, watchlistId = null) => {
    const targetWatchlistId = watchlistId || activeWatchlistId.value;
    
    if (!targetWatchlistId) {
      return false;
    }

    try {
      const data = await api.removeSymbolFromWatchlist(targetWatchlistId, symbol);

      // Update local state
      if (data.watchlist) {
        watchlists.value[targetWatchlistId] = data.watchlist;
      }

      notificationService.showSuccess(
        `${symbol} removed from watchlist`,
        "Symbol Removed"
      );

      return true;
    } catch (err) {
      console.error("Error removing symbol:", err);
      
      notificationService.showError(
        `Failed to remove ${symbol}: ${err.message}`,
        "Remove Failed"
      );
      
      return false;
    }
  };

  // Set active watchlist
  const setActiveWatchlist = async (watchlistId) => {
    try {
      await api.setActiveWatchlist(watchlistId);
      activeWatchlistId.value = watchlistId;

      const watchlistName = watchlists.value[watchlistId]?.name || watchlistId;
      
      notificationService.showInfo(
        `Switched to "${watchlistName}"`,
        "Watchlist Changed"
      );

      return true;
    } catch (err) {
      console.error("Error setting active watchlist:", err);
      
      notificationService.showError(
        "Failed to switch watchlist",
        "Switch Failed"
      );
      
      return false;
    }
  };

  // Rename watchlist
  const renameWatchlist = async (watchlistId, newName) => {
    return await updateWatchlist(watchlistId, { name: newName });
  };

  // Search watchlists
  const searchWatchlists = async (query) => {
    try {
      const data = await api.searchWatchlists(query);
      return data.results;
    } catch (err) {
      console.error("Error searching watchlists:", err);
      return [];
    }
  };

  // Validate symbol format
  const validateSymbol = (symbol) => {
    if (!symbol || typeof symbol !== 'string') {
      return false;
    }
    
    const cleanSymbol = symbol.trim().toUpperCase();
    
    // Basic validation - alphanumeric, 1-10 characters
    return /^[A-Z0-9]{1,10}$/.test(cleanSymbol);
  };

  // Get all symbols across all watchlists
  const getAllSymbols = () => {
    const allSymbols = new Set();
    Object.values(watchlists.value).forEach(watchlist => {
      watchlist.symbols.forEach(symbol => allSymbols.add(symbol));
    });
    return Array.from(allSymbols).sort();
  };

  // Initialize on mount
  onMounted(() => {
    loadWatchlists();
  });

  return {
    // State
    watchlists: computed(() => watchlists.value),
    activeWatchlistId: computed(() => activeWatchlistId.value),
    activeWatchlist,
    watchlistOptions,
    activeSymbols,
    isLoading: computed(() => isLoading.value),
    error: computed(() => error.value),

    // Methods
    loadWatchlists,
    createWatchlist,
    updateWatchlist,
    deleteWatchlist,
    addSymbol,
    removeSymbol,
    setActiveWatchlist,
    renameWatchlist,
    searchWatchlists,
    validateSymbol,
    getAllSymbols,

    // Smart data integration
    getSymbolPrice,
    getSymbolData,
  };
}

export default useWatchlist;
