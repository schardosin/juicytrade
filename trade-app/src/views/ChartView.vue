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
          />
        </div>
      </div>

      <!-- Right Panel -->
      <div class="right-panel" :class="{ expanded: isRightPanelExpanded }">
        <!-- Collapsed Menu -->
        <div v-if="!isRightPanelExpanded" class="collapsed-menu">
          <div class="menu-item" @click="toggleRightPanel">
            <i class="pi pi-th-large"></i>
          </div>
          <div class="menu-item" @click="toggleRightPanel">
            <i class="pi pi-chart-bar"></i>
          </div>
          <div class="menu-item" @click="toggleRightPanel">
            <i class="pi pi-inbox"></i>
          </div>
          <div class="menu-item" @click="toggleRightPanel">
            <i class="pi pi-bolt"></i>
          </div>
          <div class="menu-item" @click="toggleRightPanel">
            <i class="pi pi-heart"></i>
          </div>
          <div class="menu-item" @click="toggleRightPanel">
            <i class="pi pi-bell"></i>
          </div>
        </div>

        <!-- Expanded Panel -->
        <div v-if="isRightPanelExpanded" class="expanded-content">
          <!-- Panel Header with Expand/Collapse Button -->
          <div class="panel-header">
            <h3 class="panel-title">Chart Analysis</h3>
            <button
              class="expand-toggle-btn"
              @click="toggleRightPanel"
              :title="isRightPanelExpanded ? 'Collapse Panel' : 'Expand Panel'"
            >
              ▶
            </button>
          </div>

          <!-- Scrollable Content Area -->
          <div class="panel-content">
            <!-- Chart Tools Section -->
            <div class="chart-tools-section">
              <Card class="tools-card">
                <template #title>Chart Tools</template>
                <template #content>
                  <div class="tools-grid">
                    <div class="tool-item">
                      <span class="tool-label">Timeframe:</span>
                      <Dropdown
                        v-model="selectedTimeframe"
                        :options="timeframes"
                        optionLabel="label"
                        optionValue="value"
                        class="timeframe-dropdown"
                      />
                    </div>
                    <div class="tool-item">
                      <span class="tool-label">Chart Type:</span>
                      <Dropdown
                        v-model="selectedChartType"
                        :options="chartTypes"
                        optionLabel="label"
                        optionValue="value"
                        class="chart-type-dropdown"
                      />
                    </div>
                  </div>
                </template>
              </Card>
            </div>

            <!-- Market Info Section -->
            <div class="market-info-section">
              <Card class="info-card">
                <template #title>Market Information</template>
                <template #content>
                  <div class="info-grid">
                    <div class="info-item">
                      <span class="info-label">Volume:</span>
                      <span class="info-value">{{ formatVolume(volume) }}</span>
                    </div>
                    <div class="info-item">
                      <span class="info-label">Avg Volume:</span>
                      <span class="info-value">{{
                        formatVolume(avgVolume)
                      }}</span>
                    </div>
                    <div class="info-item">
                      <span class="info-label">Market Cap:</span>
                      <span class="info-value">{{
                        formatMarketCap(marketCap)
                      }}</span>
                    </div>
                    <div class="info-item">
                      <span class="info-label">P/E Ratio:</span>
                      <span class="info-value">{{ peRatio || "--" }}</span>
                    </div>
                  </div>
                </template>
              </Card>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import LightweightChart from "../components/LightweightChart.vue";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";

export default {
  name: "ChartView",
  components: {
    TopBar,
    SideNav,
    SymbolHeader,
    LightweightChart,
  },
  setup() {
    // Use global symbol state
    const { globalSymbolState, updateSymbol, updatePrice, updateMarketStatus } =
      useGlobalSymbol();

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
        // Fetch current price and basic info
        const price = await api.getUnderlyingPrice(symbol);
        if (price !== null) {
          updatePrice({ price, isLive: false });
        }

        // Start WebSocket streaming for the symbol
        await startStreaming(symbol);
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    const startStreaming = async (symbol) => {
      try {
        await webSocketClient.connect();
        webSocketClient.subscribe([symbol]);

        webSocketClient.onPriceUpdate((data) => {
          if (data.symbol === symbol) {
            const newPrice =
              data.price ||
              data.data?.mid ||
              (data.data?.bid + data.data?.ask) / 2;

            updatePrice({ price: newPrice, isLive: true });

            // Update market status based on time (simplified)
            const now = new Date();
            const hour = now.getHours();
            let status;
            if (hour >= 9 && hour < 16) {
              status = "Market Open";
            } else if (hour >= 4 && hour < 9) {
              status = "Pre-Market";
            } else if (hour >= 16 && hour < 20) {
              status = "After Hours";
            } else {
              status = "Market Closed";
            }
            updateMarketStatus(status);
          }
        });
      } catch (error) {
        console.error("Error starting streaming:", error);
      }
    };

    const onSymbolSelected = async (symbol) => {
      console.log("Chart view - Symbol selected:", symbol);

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

    // Lifecycle hooks
    onMounted(async () => {
      await fetchSymbolData(currentSymbol.value);

      // Listen for symbol selection events from TopBar
      const handleSymbolSelection = (event) => {
        onSymbolSelected(event.detail);
      };

      window.addEventListener("symbol-selected", handleSymbolSelection);

      // Store the handler for cleanup
      window._chartSymbolSelectionHandler = handleSymbolSelection;
    });

    onUnmounted(() => {
      // Clean up event listener
      if (window._chartSymbolSelectionHandler) {
        window.removeEventListener(
          "symbol-selected",
          window._chartSymbolSelectionHandler
        );
        delete window._chartSymbolSelectionHandler;
      }
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

      // Methods
      onTradeModeChanged,
      toggleRightPanel,
      formatVolume,
      formatMarketCap,
    };
  },
};
</script>

<style scoped>
.chart-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: #1a1a1a;
  color: #ffffff;
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
  background-color: #2a2a2a;
}

.chart-section {
  flex: 1;
  overflow: hidden;
  background-color: rgba(51, 51, 51, 1);
  position: relative;
}

.chart-toggle {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  gap: 8px;
  z-index: 10;
  background-color: rgba(42, 42, 42, 0.9);
  border-radius: 6px;
  padding: 4px;
  backdrop-filter: blur(8px);
}

.toggle-btn {
  padding: 8px 16px;
  background-color: #3a3a3a;
  color: #cccccc;
  border: 1px solid #4a4a4a;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.toggle-btn:hover {
  background-color: #4a4a4a;
  border-color: #5a5a5a;
  color: #ffffff;
}

.toggle-btn.active {
  background-color: #2962ff;
  border-color: #2962ff;
  color: #ffffff;
  box-shadow: 0 2px 4px rgba(41, 98, 255, 0.3);
}

.toggle-btn.active:hover {
  background-color: #1e53e5;
  border-color: #1e53e5;
}

.right-panel {
  width: 60px;
  background-color: #333333;
  border-left: 1px solid #444444;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: width 0.3s ease;
}

.right-panel.expanded {
  width: 400px;
}

.collapsed-menu {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 12px;
  gap: 16px;
}

.menu-item {
  font-size: 18px;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: background-color 0.2s ease;
  color: #cccccc;
}

.menu-item:hover {
  background-color: #444444;
}

.expanded-content {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #3a3a3a;
  border-bottom: 1px solid #444444;
  flex-shrink: 0;
}

.panel-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #ffffff;
}

.expand-toggle-btn {
  background: #444444;
  border: 1px solid #555555;
  color: #ffffff;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  font-size: 16px;
  font-weight: bold;
}

.expand-toggle-btn:hover {
  background-color: #007bff;
  border-color: #007bff;
  color: #ffffff;
  transform: scale(1.05);
}

.expand-toggle-btn:active {
  transform: scale(0.95);
}

.panel-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

.panel-content::-webkit-scrollbar {
  width: 8px;
}

.panel-content::-webkit-scrollbar-track {
  background: #2a2a2a;
  border-radius: 4px;
}

.panel-content::-webkit-scrollbar-thumb {
  background: #555555;
  border-radius: 4px;
}

.panel-content::-webkit-scrollbar-thumb:hover {
  background: #666666;
}

.chart-tools-section,
.market-info-section {
  padding: 16px;
  border-bottom: 1px solid #444444;
}

.tools-card,
.info-card {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

.tools-grid,
.info-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tool-item,
.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tool-label,
.info-label {
  color: #cccccc;
  font-size: 14px;
  font-weight: 500;
}

.info-value {
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
}

.timeframe-dropdown,
.chart-type-dropdown {
  min-width: 120px;
}

/* Dark theme overrides for PrimeVue components */
:deep(.p-dropdown) {
  background-color: #444444;
  border: 1px solid #555555;
  color: #ffffff;
}

:deep(.p-dropdown:not(.p-disabled):hover) {
  border-color: #666666;
}

:deep(.p-dropdown-panel) {
  background-color: #444444;
  border: 1px solid #555555;
}

:deep(.p-dropdown-item) {
  color: #ffffff;
}

:deep(.p-dropdown-item:not(.p-highlight):not(.p-disabled):hover) {
  background-color: #555555;
}

:deep(.p-card) {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

:deep(.p-card .p-card-title) {
  color: #ffffff;
}

:deep(.p-card .p-card-content) {
  color: #cccccc;
}
</style>
