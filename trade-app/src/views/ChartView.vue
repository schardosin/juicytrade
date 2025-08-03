<template>
  <div class="chart-view-container">
    <!-- Top Bar -->
    <TopBar />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Left Navigation -->
      <SideNav />

      <!-- Content Area -->
      <div class="content-area">
        <!-- Symbol Header -->
        <SymbolHeader
          :currentSymbol="currentSymbol"
          :companyName="companyName"
          :exchange="exchange"
          :currentPrice="currentPrice"
          :priceChange="priceChange"
          :priceChangePercent="priceChangePercent"
          :isLivePrice="isLivePrice"
          :marketStatus="marketStatus"
          :selectedTradeMode="selectedTradeMode"
          :tradeModes="tradeModes"
          :showTradeMode="false"
          @trade-mode-changed="onTradeModeChanged"
        />

        <!-- Chart Section -->
        <div class="chart-section">
          <!-- Lightweight Charts -->
          <LightweightChart
            :symbol="currentSymbol"
            :theme="'dark'"
            :height="chartHeight"
            :enableRealtime="true"
            :livePrice="livePrice"
          />
        </div>
      </div>

      <!-- Right Panel -->
      <RightPanel
        :currentSymbol="currentSymbol"
        :currentPrice="currentPrice"
        :priceChange="priceChange"
        :isLivePrice="isLivePrice"
        :selectedOptions="[]"
        :chartData="null"
        :additionalQuoteData="additionalQuoteData"
        :optionsChainData="[]"
        :forceExpanded="isRightPanelExpanded"
        :forceSection="rightPanelSection"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import LightweightChart from "../components/LightweightChart.vue";
import RightPanel from "../components/RightPanel.vue";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import api from "../services/api";
// webSocketClient no longer needed - global state is automatically updated by SmartMarketDataStore
// import webSocketClient from "../services/webSocketClient";

export default {
  name: "ChartView",
  components: {
    TopBar,
    SideNav,
    SymbolHeader,
    LightweightChart,
    RightPanel,
  },
  setup() {
    // Use global symbol state with centralized symbol selection
    const { globalSymbolState, updateSymbol, updatePrice, updateMarketStatus, setupSymbolSelectionListener } =
      useGlobalSymbol({
        onSymbolChange: async (symbolData) => {
          // Fetch new data for the selected symbol
          try {
            await fetchSymbolData(symbolData.symbol);
          } catch (error) {
            console.error("Error loading data for new symbol:", error);
          }
        }
      });

    // Use SmartMarketData for live price subscriptions
    const { getStockPrice } = useSmartMarketData();

    // Local reactive data (non-symbol related)
    const selectedTradeMode = ref("chart");
    const isRightPanelExpanded = ref(false);
    const selectedTimeframe = ref("D");
    const selectedChartType = ref("1");
    const volume = ref(0);
    const avgVolume = ref(0);
    const marketCap = ref(0);
    const peRatio = ref(null);

    // Computed properties for global symbol state
    const currentSymbol = computed(() => globalSymbolState.currentSymbol);
    const companyName = computed(() => globalSymbolState.companyName);
    const exchange = computed(() => globalSymbolState.exchange);
    const currentPrice = computed(() => globalSymbolState.currentPrice);
    const priceChange = computed(() => globalSymbolState.priceChange);
    const priceChangePercent = computed(
      () => globalSymbolState.priceChangePercent
    );
    const isLivePrice = computed(() => globalSymbolState.isLivePrice);
    const marketStatus = computed(() => globalSymbolState.marketStatus);

    // Live price subscription for chart updates
    const livePrice = computed(() => {
      if (!currentSymbol.value) return null;
      const priceData = getStockPrice(currentSymbol.value);
      return priceData?.value || null;
    });

    // Chart configuration
    const chartProvider = ref("lightweight");
    const chartHeight = computed(() => {
      // Calculate chart height based on available space
      // Account for TopBar (~60px), SymbolHeader (~80px), and chart controls (~60px)
      return window.innerHeight - 200;
    });

    // Trade modes (for header compatibility)
    const tradeModes = [
      { label: "Chart", value: "chart" },
      { label: "Analysis", value: "analysis" },
    ];

    // Timeframe options
    const timeframes = [
      { label: "1 Min", value: "1" },
      { label: "5 Min", value: "5" },
      { label: "15 Min", value: "15" },
      { label: "30 Min", value: "30" },
      { label: "1 Hour", value: "60" },
      { label: "4 Hour", value: "240" },
      { label: "Daily", value: "D" },
      { label: "Weekly", value: "W" },
      { label: "Monthly", value: "M" },
    ];

    // Chart type options
    const chartTypes = [
      { label: "Candlestick", value: "1" },
      { label: "Line", value: "2" },
      { label: "Area", value: "3" },
      { label: "Bars", value: "0" },
    ];

    // Methods
    const fetchSymbolData = async (symbol) => {
      try {
        // Only fetch price via API if we don't have live data already
        if (!isLivePrice.value || currentPrice.value === 0) {
          const price = await api.getUnderlyingPrice(symbol);
          if (price !== null) {
            updatePrice({ price, isLive: false });
          }
        }

        // WebSocket streaming is now handled automatically by SmartMarketDataStore
        // via the global state integration - no manual streaming needed
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    // Streaming is now handled automatically by SmartMarketDataStore via global state
    // No manual WebSocket management needed

    const onSymbolSelected = async (symbol) => {
      // Update global symbol state
      updateSymbol(symbol);

      // Fetch new data for the selected symbol
      try {
        await fetchSymbolData(symbol.symbol);
      } catch (error) {
        console.error("Error loading data for new symbol:", error);
      }
    };

    const onTradeModeChanged = (mode) => {
      selectedTradeMode.value = mode;
    };

    const toggleRightPanel = () => {
      isRightPanelExpanded.value = !isRightPanelExpanded.value;
    };

    const formatVolume = (vol) => {
      if (!vol) return "--";
      if (vol >= 1000000) {
        return `${(vol / 1000000).toFixed(1)}M`;
      } else if (vol >= 1000) {
        return `${(vol / 1000).toFixed(1)}K`;
      }
      return vol.toLocaleString();
    };

    const formatMarketCap = (cap) => {
      if (!cap) return "--";
      if (cap >= 1000000000000) {
        return `$${(cap / 1000000000000).toFixed(1)}T`;
      } else if (cap >= 1000000000) {
        return `$${(cap / 1000000000).toFixed(1)}B`;
      } else if (cap >= 1000000) {
        return `$${(cap / 1000000).toFixed(1)}M`;
      }
      return `$${cap.toLocaleString()}`;
    };

    // RightPanel event handlers
    const onRightPanelCollapsed = () => {
      isRightPanelExpanded.value = false;
    };

    const onPositionsChanged = (checkedPositions) => {
      // Chart view doesn't need to handle position changes for payoff chart
      // since it's primarily for price charts, not options analysis
    };

    // Computed properties for RightPanel
    const rightPanelSection = computed(() => {
      // Default to overview section for chart view
      return null; // Let user choose which section to view
    });

    // Additional quote data for the right panel
    const additionalQuoteData = computed(() => ({
      // Mock data - in real implementation, this would come from API
      open: currentPrice.value * 0.998,
      close: currentPrice.value * 1.001,
      high: currentPrice.value * 1.005,
      low: currentPrice.value * 0.995,
      yearHigh: currentPrice.value * 1.25,
      yearLow: currentPrice.value * 0.75,
      bid: currentPrice.value - 0.05,
      ask: currentPrice.value + 0.05,
      bidSize: 400,
      askSize: 300,
      ivRank: 13.2,
      ivIndex: 17.5,
      volume: volume.value,
      marketCap: marketCap.value,
      peRatio: peRatio.value,
      eps: null,
      earnings: null,
      dividend: 1.76,
      dividendYield: 1.13,
      correlation: 1.0,
      liquidity: 4,
    }));

    // Lifecycle hooks
    onMounted(async () => {
      await fetchSymbolData(currentSymbol.value);

      // Set up centralized symbol selection listener
      const cleanup = setupSymbolSelectionListener();
      
      // Store cleanup for unmount
      onUnmounted(cleanup);
    });

    return {
      // Reactive data
      currentSymbol,
      companyName,
      exchange,
      currentPrice,
      priceChange,
      priceChangePercent,
      isLivePrice,
      marketStatus,
      selectedTradeMode,
      tradeModes,
      isRightPanelExpanded,
      selectedTimeframe,
      selectedChartType,
      timeframes,
      chartTypes,
      volume,
      avgVolume,
      marketCap,
      peRatio,
      chartProvider,
      chartHeight,

      // Live price data for chart
      livePrice,

      // Computed properties for RightPanel
      rightPanelSection,
      additionalQuoteData,

      // Methods
      onTradeModeChanged,
      toggleRightPanel,
      formatVolume,
      formatMarketCap,
      onRightPanelCollapsed,
      onPositionsChanged,
    };
  },
};
</script>

<style scoped>
.chart-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: var(--bg-secondary);
  color: var(--text-primary);
}

.main-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: var(--bg-secondary);
}

.chart-section {
  flex: 1;
  overflow: hidden;
  background-color: var(--bg-primary);
  position: relative;
}

/* Chart view specific styles - most panel styles are now handled by RightPanel component */
</style>
