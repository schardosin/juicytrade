import { ref, reactive, computed, watch } from "vue";
import { useSmartMarketData } from "./useSmartMarketData.js";
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

import { DateTime } from "luxon";

// Global symbol management composable
export function useGlobalSymbol() {
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
    globalSymbolState.companyName = symbolData.description;
    globalSymbolState.exchange = symbolData.exchange || "Unknown";

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
    const now = DateTime.now().setZone("America/New_York");
    const day = now.weekday; // 1=Monday, 7=Sunday
    const hour = now.hour;
    const minute = now.minute;
    // US market open: Mon-Fri, 9:30am-4:00pm ET
    const isWeekday = day >= 1 && day <= 5;
    const isOpen = isWeekday && (
      (hour > 9 || (hour === 9 && minute >= 30)) &&
      (hour < 16)
    );
    globalSymbolState.marketStatus = isOpen ? "Market Open" : "Market Closed";
  }
  updateMarketStatusNow();
  setInterval(updateMarketStatusNow, 60000);

  return {
    // Reactive state
    globalSymbolState,

    // Methods
    updateSymbol,
    updatePrice,
    updateMarketStatus,
    getSymbolData,

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
