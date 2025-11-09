<template>
  <div class="chart-view-container">
    <!-- Top Bar -->
    <TopBar @toggle-mobile-nav="showMobileNav = true" />

    <!-- Mobile Navigation Drawer -->
    <MobileNavDrawer
      v-if="isMobile"
      :visible="showMobileNav"
      @close="showMobileNav = false"
      @navigate="onMobileNavigation"
    />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Left Navigation (hidden on mobile) -->
      <SideNav v-if="!isMobile" />

      <!-- Content Area -->
      <div class="content-area" :class="{ 'mobile-layout': isMobile }">
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
          @open-mobile-search="onOpenMobileSearch"
        />

        <!-- Chart Section -->
        <div class="chart-section">
          <LightweightChart
            :symbol="currentSymbol"
            :theme="'dark'"
            :enableRealtime="true"
            :livePrice="livePrice"
            :hideControls="false"
          />
        </div>
      </div>

      <!-- Right Panel (desktop only) -->
      <RightPanel
        v-if="!isMobile"
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
        :isMobile="isMobile"
        :isTablet="isTablet"
        :isDesktop="isDesktop"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>

    <!-- Mobile Bottom Button Bar -->
    <MobileBottomButtonBar
      v-if="isMobile"
      :activeSection="showMobileOverlay ? activeMobileSection : null"
      @section-selected="onMobileSectionSelected"
    />

    <!-- Mobile Full Screen Overlay -->
    <MobileFullScreenOverlay
      v-if="isMobile"
      :visible="showMobileOverlay"
      :activeSection="activeMobileSection"
      :currentSymbol="currentSymbol"
      :currentPrice="currentPrice"
      :priceChange="priceChange"
      :isLivePrice="isLivePrice"
      :chartData="null"
      :additionalQuoteData="additionalQuoteData"
      :allPositions="[]"
      :checkedPositions="new Set()"
      :isAllSelected="false"
      :isIndeterminate="false"
      @close="closeMobileOverlay"
      @toggle-position-check="onMobileTogglePositionCheck"
      @toggle-select-all="onMobileToggleSelectAll"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import MobileNavDrawer from "../components/MobileNavDrawer.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import LightweightChart from "../components/LightweightChart.vue";
import RightPanel from "../components/RightPanel.vue";
import MobileBottomButtonBar from "../components/MobileBottomButtonBar.vue";
import MobileFullScreenOverlay from "../components/MobileFullScreenOverlay.vue";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import { useMobileDetection } from "../composables/useMobileDetection.js";
import api from "../services/api";

export default {
  name: "ChartView",
  components: {
    TopBar,
    SideNav,
    MobileNavDrawer,
    SymbolHeader,
    LightweightChart,
    RightPanel,
    MobileBottomButtonBar,
    MobileFullScreenOverlay,
  },
  setup() {
    // Use global symbol state
    const { globalSymbolState, updateSymbol, updatePrice, setupSymbolSelectionListener } =
      useGlobalSymbol({
        onSymbolChange: async (symbolData) => {
          try {
            await fetchSymbolData(symbolData.symbol);
          } catch (error) {
            console.error("Error loading data for new symbol:", error);
          }
        }
      });

    // Use SmartMarketData for live price subscriptions
    const { getStockPrice } = useSmartMarketData();

    // Mobile detection
    const { isMobile, isTablet, isDesktop } = useMobileDetection();

    // Mobile navigation state
    const showMobileNav = ref(false);

    // Mobile overlay state
    const showMobileOverlay = ref(false);
    const activeMobileSection = ref('overview');

    // Local reactive data
    const selectedTradeMode = ref("chart");
    const isRightPanelExpanded = ref(false);

    // Computed properties for global symbol state
    const currentSymbol = computed(() => globalSymbolState.currentSymbol);
    const companyName = computed(() => globalSymbolState.companyName);
    const exchange = computed(() => globalSymbolState.exchange);
    const currentPrice = computed(() => globalSymbolState.currentPrice);
    const priceChange = computed(() => globalSymbolState.priceChange);
    const priceChangePercent = computed(() => globalSymbolState.priceChangePercent);
    const isLivePrice = computed(() => globalSymbolState.isLivePrice);
    const marketStatus = computed(() => globalSymbolState.marketStatus);

    // Live price subscription for chart updates
    const livePrice = computed(() => {
      if (!currentSymbol.value) return null;
      const priceData = getStockPrice(currentSymbol.value);
      return priceData?.value || null;
    });

    // Trade modes (for header compatibility)
    const tradeModes = [
      { label: "Chart", value: "chart" },
      { label: "Analysis", value: "analysis" },
    ];

    // Methods
    const fetchSymbolData = async (symbol) => {
      try {
        if (!isLivePrice.value || currentPrice.value === 0) {
          const price = await api.getUnderlyingPrice(symbol);
          if (price !== null) {
            updatePrice({ price, isLive: false });
          }
        }
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    const onTradeModeChanged = (mode) => {
      selectedTradeMode.value = mode;
    };

    const onOpenMobileSearch = () => {
      window.dispatchEvent(new CustomEvent('open-mobile-search'));
    };

    const onRightPanelCollapsed = () => {
      isRightPanelExpanded.value = false;
    };

    const onPositionsChanged = (checkedPositions) => {
      // Chart view doesn't need position changes for payoff chart
    };

    const rightPanelSection = computed(() => null);

    // Mobile overlay methods
    const onMobileSectionSelected = (sectionKey) => {
      activeMobileSection.value = sectionKey;
      showMobileOverlay.value = true;
    };

    const closeMobileOverlay = () => {
      showMobileOverlay.value = false;
    };

    const onMobileTogglePositionCheck = (positionId) => {
      console.log("Mobile position check:", positionId);
    };

    const onMobileToggleSelectAll = () => {
      console.log("Mobile select all");
    };

    // Mobile navigation method
    const onMobileNavigation = (route) => {
      showMobileNav.value = false;
      // Navigation is handled by the router
      console.log("Mobile navigation to:", route);
    };

    // Additional quote data for the right panel
    const additionalQuoteData = computed(() => ({
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
      volume: 51500000,
      marketCap: null,
      peRatio: null,
      eps: null,
      earnings: null,
      dividend: 1.76,
      dividendYield: 1.13,
      correlation: 1.0,
      liquidity: 4,
    }));

    // Lifecycle hooks
    let cleanup = null;
    
    onMounted(async () => {
      await fetchSymbolData(currentSymbol.value);
      cleanup = setupSymbolSelectionListener();
    });

    onUnmounted(() => {
      if (cleanup) {
        cleanup();
      }
    });

    return {
      // Mobile detection
      isMobile,
      isTablet,
      isDesktop,

      // Mobile navigation state
      showMobileNav,

      // Mobile overlay state
      showMobileOverlay,
      activeMobileSection,

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

      // Live price data for chart
      livePrice,

      // Computed properties for RightPanel
      rightPanelSection,
      additionalQuoteData,

      // Methods
      onTradeModeChanged,
      onOpenMobileSearch,
      onRightPanelCollapsed,
      onPositionsChanged,

      // Mobile navigation methods
      onMobileNavigation,

      // Mobile overlay methods
      onMobileSectionSelected,
      closeMobileOverlay,
      onMobileTogglePositionCheck,
      onMobileToggleSelectAll,
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
  display: flex;
  flex-direction: column;
  background-color: var(--bg-primary);
  min-height: 0; /* Important for flex child to shrink */
}

/* Mobile layout adjustments */
@media (max-width: 768px) {
  .main-layout {
    flex-direction: column;
  }
  
  .content-area {
    margin-right: 0 !important;
    /* Reserve space for mobile bottom button bar (80px) */
    padding-bottom: 80px;
  }
  
  .chart-section {
    /* Ensure chart shows properly on mobile */
    flex: 1;
    min-height: 300px;
  }
}

/* Tablet adjustments */
@media (max-width: 1024px) and (min-width: 769px) {
  .content-area {
    margin-right: 0;
  }
}
</style>
