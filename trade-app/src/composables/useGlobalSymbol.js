import { ref, reactive, computed, watch } from "vue";
import { useSmartMarketData } from "./useSmartMarketData.js";

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

// Global symbol management composable
export function useGlobalSymbol() {
  // Update symbol information
  const updateSymbol = (symbolData) => {
    globalSymbolState.currentSymbol = symbolData.symbol;
    globalSymbolState.companyName = symbolData.description;
    globalSymbolState.exchange = symbolData.exchange || "Unknown";

    // Reset price data when symbol changes
    globalSymbolState.currentPrice = null;
    globalSymbolState.priceChange = 0;
    globalSymbolState.priceChangePercent = 0;
    globalSymbolState.isLivePrice = false;
    globalSymbolState.marketStatus = "Market Closed";

    // Persist the symbol data to localStorage
    persistSymbolData();

    console.log("Global symbol updated:", symbolData.symbol);
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
