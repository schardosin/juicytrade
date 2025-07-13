import { ref, reactive } from "vue";

// Global symbol state - shared across all components
const globalSymbolState = reactive({
  currentSymbol: "SPY",
  companyName: "SPDR S&P 500 ETF Trust",
  exchange: "NYSE Arca",
  currentPrice: null,
  priceChange: 0,
  priceChangePercent: 0,
  isLivePrice: false,
  marketStatus: "Market Closed",
});

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
