import { ref, reactive, computed, watch } from "vue";
import { useSmartMarketData } from "./useSmartMarketData.js";
import { useMarketData } from "./useMarketData.js";
import smartMarketDataStore from "../services/smartMarketDataStore.js";

// Load persisted symbol data from localStorage
const loadPersistedSymbolData = () => {
  try {
    const saved = localStorage.getItem("globalSymbolState");
    if (saved) {
      const parsed = JSON.parse(saved);
      return {
        currentSymbol: parsed.currentSymbol || "SPY",
        companyName: parsed.companyName || "SPDR S&P 500 ETF Trust",
        exchange: parsed.exchange || "NYSE Arca",
        currentPrice: null, // Always reset price data on page load
        priceChange: 0,
        priceChangePercent: 0,
        isLivePrice: false,
        marketStatus: "Market Closed",
      };
    }
  } catch (error) {
    console.warn("Failed to load persisted symbol data:", error);
  }

  // Default fallback
  return {
    currentSymbol: "SPY",
    companyName: "SPDR S&P 500 ETF Trust",
    exchange: "NYSE Arca",
    currentPrice: null,
    priceChange: 0,
    priceChangePercent: 0,
    isLivePrice: false,
    marketStatus: "Market Closed",
  };
};

// Global symbol state - shared across all components with persistence
const globalSymbolState = reactive(loadPersistedSymbolData());

// Smart market data integration - automatically update global state with live prices
const { getStockPrice } = useSmartMarketData();

// Get live price for current symbol
const liveStockPrice = computed(() => {
  if (!globalSymbolState.currentSymbol) return null;
  return getStockPrice(globalSymbolState.currentSymbol);
});

// Watch for live price updates and automatically update global state
watch(
  liveStockPrice,
  (newPriceRef) => {
    if (newPriceRef?.value?.price) {
      const oldPrice = globalSymbolState.currentPrice;
      const newPrice = newPriceRef.value.price;

      // Update current price
      globalSymbolState.currentPrice = newPrice;
      globalSymbolState.isLivePrice = true;

      // Calculate price change if we have a previous price
      if (oldPrice !== null && oldPrice !== 0) {
        globalSymbolState.priceChange = newPrice - oldPrice;
        globalSymbolState.priceChangePercent =
          (globalSymbolState.priceChange / oldPrice) * 100;
      }
    }
  },
  { deep: true, immediate: true }
);

// Save symbol data to localStorage (only symbol info, not price data)
const persistSymbolData = () => {
  try {
    const dataToSave = {
      currentSymbol: globalSymbolState.currentSymbol,
      companyName: globalSymbolState.companyName,
      exchange: globalSymbolState.exchange,
    };
    localStorage.setItem("globalSymbolState", JSON.stringify(dataToSave));
  } catch (error) {
    console.warn("Failed to persist symbol data:", error);
  }
};

import MarketHoursUtil from "../utils/marketHours.js";

// Global symbol management composable
export function useGlobalSymbol(options = {}) {
  const { onSymbolChange } = options;
  // Load positions for the initial/default symbol on first use
  const initializePositions = async () => {
    try {
      await smartMarketDataStore.refreshPositions();
    } catch (error) {
      console.error(`❌ Failed to load initial positions for ${globalSymbolState.currentSymbol}:`, error);
    }
  };

  // Initialize positions for the current symbol (only once per app load)
  if (!globalSymbolState._positionsInitialized) {
    globalSymbolState._positionsInitialized = true;
    // Use setTimeout to avoid blocking the composable setup
    setTimeout(initializePositions, 0);
  }

  // Update symbol information
  const updateSymbol = async (symbolData) => {
    const oldSymbol = globalSymbolState.currentSymbol;
    const newSymbol = symbolData.symbol;
    
    globalSymbolState.currentSymbol = newSymbol;
    globalSymbolState.companyName = symbolData.description || "";
    globalSymbolState.exchange = symbolData.exchange || "";

    // Reset price data when symbol changes
    globalSymbolState.currentPrice = null;
    globalSymbolState.priceChange = 0;
    globalSymbolState.priceChangePercent = 0;
    globalSymbolState.isLivePrice = false;
    globalSymbolState.marketStatus = "Market Closed";

    // 🆕 IMMEDIATELY load positions for new symbol (only if symbol actually changed)
    if (oldSymbol !== newSymbol) {
      try {
        await smartMarketDataStore.refreshPositions();
      } catch (error) {
        console.error(`❌ Failed to load positions for ${newSymbol}:`, error);
      }
    }

    // Persist the symbol data to localStorage
    persistSymbolData();

  };

  const fetchSymbolDetails = async (symbol) => {
    // Prevent fetching if details are already present
    if (globalSymbolState.currentSymbol === symbol && globalSymbolState.companyName) {
      return;
    }

    try {
      const { lookupSymbols } = useMarketData();
      const results = await lookupSymbols(symbol);
      if (results && results.length > 0) {
        const symbolData = results[0];
        globalSymbolState.companyName = symbolData.description;
        globalSymbolState.exchange = symbolData.exchange;
        persistSymbolData();
      }
    } catch (error) {
      console.error(`Error fetching details for symbol ${symbol}:`, error);
    }
  };

  // Update price information
  const updatePrice = (priceData) => {
    const oldPrice = globalSymbolState.currentPrice;
    globalSymbolState.currentPrice = priceData.price;
    globalSymbolState.isLivePrice = priceData.isLive || false;

    if (oldPrice !== null && priceData.price !== null) {
      globalSymbolState.priceChange = priceData.price - oldPrice;
      globalSymbolState.priceChangePercent =
        (globalSymbolState.priceChange / oldPrice) * 100;
    }
  };

  // Update market status
  const updateMarketStatus = (status) => {
    globalSymbolState.marketStatus = status;
  };

  // Get current symbol data
  const getSymbolData = () => {
    return {
      symbol: globalSymbolState.currentSymbol,
      description: globalSymbolState.companyName,
      exchange: globalSymbolState.exchange,
    };
  };

  // Global market status updater (runs always)
  function updateMarketStatusNow() {
    globalSymbolState.marketStatus = MarketHoursUtil.getMarketStatusText();
  }
  updateMarketStatusNow();
  setInterval(updateMarketStatusNow, 60000);

  // Centralized symbol selection handler
  const setupSymbolSelectionListener = () => {
    const handleSymbolSelection = async (event) => {
      const symbolData = event.detail;
      
      try {
        // Update global symbol state (this triggers all the existing logic)
        await updateSymbol(symbolData);
        
        // Call view-specific callback if provided
        if (onSymbolChange) {
          await onSymbolChange(symbolData);
        }
      } catch (error) {
        console.error('❌ Error handling symbol selection:', error);
      }
    };

    // Set up event listener
    window.addEventListener("symbol-selected", handleSymbolSelection);
    
    // Return cleanup function
    return () => {
      window.removeEventListener("symbol-selected", handleSymbolSelection);
    };
  };

  return {
    // Reactive state
    globalSymbolState,

    // Methods
    updateSymbol,
    updatePrice,
    updateMarketStatus,
    getSymbolData,
    setupSymbolSelectionListener,
    fetchSymbolDetails,

    // Computed-like getters for convenience
    currentSymbol: () => globalSymbolState.currentSymbol,
    companyName: () => globalSymbolState.companyName,
    exchange: () => globalSymbolState.exchange,
    currentPrice: () => globalSymbolState.currentPrice,
    priceChange: () => globalSymbolState.priceChange,
    priceChangePercent: () => globalSymbolState.priceChangePercent,
    isLivePrice: () => globalSymbolState.isLivePrice,
    marketStatus: () => globalSymbolState.marketStatus,
  };
}
