<template>
  <div class="options-trading-container">
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
          :showTradeMode="true"
          @trade-mode-changed="onTradeModeChanged"
        />

        <!-- Options Mode: Options Chain Section -->
        <div v-if="selectedTradeMode === 'options'" class="options-section">
          <!-- Collapsible Options Chain -->
          <div class="options-chain-wrapper">
            <CollapsibleOptionsChain
              :symbol="currentSymbol"
              :underlyingPrice="currentPrice"
              :expirationDates="optionsManager.expirationDates.value"
              :optionsDataByExpiration="optionsManager.dataByExpiration.value"
              :loading="optionsManager.loading.value"
              :error="optionsManager.error.value"
              :expandedExpirations="optionsManager.expandedExpirations.value"
              :currentStrikeCount="optionsManager.strikeCount.value"
              :ivxData="ivxData"
              @expiration-expanded="onExpirationExpanded"
              @expiration-collapsed="onExpirationCollapsed"
              @strike-count-changed="onStrikeCountChanged"
            />
          </div>
        </div>

        <!-- Shares Mode: Chart Section -->
        <div v-if="selectedTradeMode === 'shares'" class="shares-section">
          <LightweightChart
            :symbol="currentSymbol"
            :theme="'dark'"
            :height="chartHeight"
            :enableRealtime="true"
            :livePrice="livePrice"
          />
        </div>
      </div>

      <!-- Right Panel (visible in both Options and Shares modes) -->
      <RightPanel
        :currentSymbol="currentSymbol"
        :currentPrice="currentPrice"
        :priceChange="priceChange"
        :isLivePrice="isLivePrice"
        :chartData="chartData"
        :additionalQuoteData="additionalQuoteData"
        :optionsChainData="optionsManager.flattenedData.value"
        :adjustedNetCredit="adjustedNetCredit"
        :forceExpanded="isRightPanelExpanded"
        :forceSection="rightPanelSection"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>

    <!-- Order Confirmation Dialog -->
    <OrderConfirmationDialog
      :visible="showOrderConfirmation"
      :orderData="orderData"
      :loading="isPlacingOrder"
      @hide="handleOrderCancellation"
      @confirm="handleOrderConfirmation"
      @cancel="handleOrderCancellation"
      @edit="handleOrderEdit"
      @clear-selections="clearAllSelections"
    />

    <!-- Options Trading Panel -->
    <BottomTradingPanel
      v-if="selectedTradeMode === 'options'"
      :visible="showBottomPanel"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      :moveStrike="optionsManager.moveStrike"
      @review-send="onReviewAndSend"
      @price-adjusted="onPriceAdjusted"
    />

    <!-- Shares Trading Panel -->
    <SharesTradingPanel
      v-if="selectedTradeMode === 'shares'"
      :visible="showSharesPanel"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      @review-send="onSharesReviewAndSend"
      @clear-trade="onSharesClearTrade"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue";
import { DateTime } from "luxon";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import CollapsibleOptionsChain from "../components/CollapsibleOptionsChain.vue";
import PayoffChart from "../components/PayoffChart.vue";
import BottomTradingPanel from "../components/BottomTradingPanel.vue";
import SharesTradingPanel from "../components/SharesTradingPanel.vue";
import LightweightChart from "../components/LightweightChart.vue";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import RightPanel from "../components/RightPanel.vue";
import { useOrderManagement } from "../composables/useOrderManagement";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useOptionsChainManager } from "../composables/useOptionsChainManager";
import { useMarketData } from "../composables/useMarketData.js";
import { useSelectedLegs } from "../composables/useSelectedLegs.js";
import { useTradeNavigation } from "../composables/useTradeNavigation.js";
import api from "../services/api";
// webSocketClient no longer needed - global state is automatically updated by SmartMarketDataStore
// import webSocketClient from "../services/webSocketClient";
import { generateMultiLegPayoff } from "../utils/chartUtils";

export default {
  name: "OptionsTrading",
  components: {
    TopBar,
    SideNav,
    SymbolHeader,
    CollapsibleOptionsChain,
    PayoffChart,
    BottomTradingPanel,
    SharesTradingPanel,
    LightweightChart,
    OrderConfirmationDialog,
    RightPanel,
  },
  setup() {
    const { pendingOrder, clearPendingOrder } = useTradeNavigation();
    // Use centralized order management with cleanup callback
    const { getIvxData } = useMarketData();
    const ivxData = getIvxData(
      computed(() => globalSymbolState.currentSymbol),
      computed(() => globalSymbolState.currentPrice)
    );

    const {
      showOrderConfirmation,
      showOrderResult,
      orderData,
      orderResult,
      isPlacingOrder,
      initializeOrder,
      handleOrderConfirmation,
      handleOrderCancellation,
      handleOrderResultClose,
    } = useOrderManagement({
      onOrderSuccess: () => {
        // Clear all selections when order is successful
        clearAllSelections();
      }
    });

    // Use global symbol state with centralized symbol selection
    const { globalSymbolState, updateSymbol, updatePrice, updateMarketStatus, setupSymbolSelectionListener } =
      useGlobalSymbol({
        onSymbolChange: async (symbolData) => {
          // Clear existing data
          clearAllSelections();
          optionsChainData.value = [];
          optionsDataByExpiration.value = {}; // Clear CollapsibleOptionsChain data
          expirationDates.value = [];
          selectedExpiry.value = null;

          // Fetch new data for the selected symbol
          try {
            await fetchSymbolData(symbolData.symbol);
            await fetchExpirationDates(symbolData.symbol);

            if (selectedExpiry.value) {
              onExpiryChange();
            }
          } catch (error) {
            console.error("Error loading data for new symbol:", error);
          }
        }
      });

    // Use centralized selected legs (positions loaded globally by useGlobalSymbol)
    const { selectedLegs, addLeg, clearAll: clearSelectedLegs } = useSelectedLegs();

    const activePositions = ref([]);
    const hasExplicitPositionSelection = ref(false); // Track if user has made checkbox selections

    // Use centralized options chain manager
    const optionsManager = useOptionsChainManager(
      computed(() => globalSymbolState.currentSymbol),
      computed(() => globalSymbolState.currentPrice),
      20 // default strike count
    );

    // Local reactive data (non-symbol related)
    const selectedExpiry = ref(null);
    const chartData = ref(null);
    const selectedTradeMode = ref("options");
    const isRightPanelExpanded = ref(false);
    const showBottomPanel = ref(false);
    const showSharesPanel = ref(false);
    const currentStrikeCount = ref(20);
    const adjustedNetCredit = ref(null);
    const rightPanelControlsChart = ref(false); // Flag to indicate RightPanel is controlling chart

    // Legacy data for backward compatibility (will be removed gradually)
    const optionsChainData = ref([]);
    const optionsDataByExpiration = ref({});
    const expirationDates = ref([]);
    const optionsChainLoading = ref(false);
    const optionsChainError = ref(null);

    // Trade modes
    const tradeModes = [
      { label: "Options", value: "options" },
      { label: "Shares", value: "shares" },
      { label: "Futures", value: "futures" },
    ];

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

    const priceChangeClass = computed(() => ({
      positive: priceChange.value > 0,
      negative: priceChange.value < 0,
      neutral: priceChange.value === 0,
    }));

    const marketStatusClass = computed(() => ({
      open: marketStatus.value === "Market Open",
      closed: marketStatus.value === "Market Closed",
      "pre-market": marketStatus.value === "Pre-Market",
      "after-hours": marketStatus.value === "After Hours",
    }));

    const estimatedCost = computed(() => {
      if (selectedLegs.value.length === 0) return null;

      let totalCost = 0;
      selectedLegs.value.forEach((leg) => {
        const price = leg.side === "buy" ? leg.ask : leg.bid;
        const cost = leg.side === "buy" ? -price : price;
        totalCost += cost * leg.quantity * 100; // Options are in contracts of 100
      });

      return totalCost;
    });

    // Computed property to determine which section to show in right panel
    const rightPanelSection = computed(() => {
      // If options are selected, show analysis section, otherwise null (let user choose)
      return selectedLegs.value.length > 0 || activePositions.value.length > 0 ? "analysis" : null;
    });

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
        // Subscriptions are now handled automatically by the SmartMarketDataStore.
        // No need to manually call webSocketClient here. The SymbolHeader
        // component will trigger the necessary stock subscription.
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    const fetchExpirationDates = async (symbol) => {
      try {
        optionsChainLoading.value = true;
        optionsChainError.value = null;

        const response = await api.getAvailableExpirations(symbol);

        if (response && response.length > 0) {
          // Store raw expiration dates for CollapsibleOptionsChain
          expirationDates.value = response;

          // Also create formatted dates for backward compatibility
          const dates = response.map((expirationEntry) => {
            // Handle both old string format and new object format
            const dateStr = typeof expirationEntry === 'string' ? expirationEntry : expirationEntry.date;
            const [year, month, day] = dateStr.split("-").map(Number);
            const date = new Date(Date.UTC(year, month - 1, day));
            const today = new Date();
            today.setHours(0, 0, 0, 0);

            return {
              label: date.toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
                year:
                  date.getUTCFullYear() !== today.getFullYear()
                    ? "numeric"
                    : undefined,
                timeZone: "UTC",
              }),
              value: dateStr,
            };
          });

          if (dates.length > 0) {
            selectedExpiry.value = dates[0].value;
          }
        } else {
          console.warn("No expiration dates returned from API");
          expirationDates.value = [];
          optionsChainError.value = "No expiration dates available";
        }
      } catch (error) {
        console.error("Error fetching expiration dates from API:", error);
        expirationDates.value = [];
        optionsChainError.value = "Failed to load expiration dates";
      } finally {
        optionsChainLoading.value = false;
      }
    };

    // The startStreaming logic is now handled globally by the SmartMarketDataStore
    // when it is initialized in main.js. We can remove this local function.
    // The onPriceUpdate logic is also handled inside the store.

    const onExpiryChange = async () => {
      if (selectedExpiry.value) {
        // The subscription logic is now handled automatically by the
        // CollapsibleOptionsChain component via the SmartMarketDataStore.
        // No need to manually subscribe here.
      }
    };

    const onOptionSelected = (optionSelection) => {
      const existingIndex = selectedOptions.value.findIndex(
        (sel) => sel.symbol === optionSelection.symbol
      );

      // The optionSelection already contains the correct expiry from CollapsibleOptionsChain
      // Don't override it with selectedExpiry.value which is from the old single-expiration system
      const selectionWithExpiry = {
        ...optionSelection,
        // Keep the expiry that came from the selection (from the correct expiration group)
        expiry: optionSelection.expiry || optionSelection.expiration_date,
      };

      if (existingIndex >= 0) {
        // Update existing selection
        selectedOptions.value[existingIndex] = selectionWithExpiry;
      } else {
        // Add new selection
        selectedOptions.value.push(selectionWithExpiry);
      }

      // Force show bottom panel and expand right panel
      showBottomPanel.value = true;
      isRightPanelExpanded.value = true;
    };

    const onOptionDeselected = (symbol) => {
      const index = selectedOptions.value.findIndex(
        (sel) => sel.symbol === symbol
      );
      if (index >= 0) {
        selectedOptions.value.splice(index, 1);

        // Auto-hide panel if no options left
        if (selectedOptions.value.length === 0) {
          showBottomPanel.value = false;
          isRightPanelExpanded.value = false;
        }
      }
    };

    const clearAllSelections = () => {
      clearSelectedLegs();
      activePositions.value = [];
      chartData.value = null;
      showBottomPanel.value = false;
      isRightPanelExpanded.value = false;
    };

    const updateChartData = async (positions = null) => {
      // Chart ONLY uses positions from RightPanel checkboxes
      // No fallback to selectedLegs - if no positions are checked, show no chart
      
      if (!positions || positions.length === 0) {
        chartData.value = null;
        return;
      }

      try {
        const payoffData = generateMultiLegPayoff(
          positions,
          currentPrice.value,
          adjustedNetCredit.value
        );

        // Force reactivity by creating a new object reference
        chartData.value = {
          ...payoffData,
          timestamp: Date.now(), // Add timestamp to ensure object reference changes
        };
      } catch (error) {
        console.error("Error generating payoff chart:", error);
        chartData.value = null;
      }
    };

    const onOrderPlaced = (orderData) => {
      initializeOrder(orderData);
    };

    const toggleRightPanel = () => {
      isRightPanelExpanded.value = !isRightPanelExpanded.value;
    };

    const onReviewAndSend = (orderData) => {
      // Hide bottom panel and show confirmation dialog
      showBottomPanel.value = false;
      initializeOrder(orderData);
    };

    // onUpdateLegQuantity is no longer needed - quantity updates are handled by the centralized store

    const handleOrderEdit = () => {
      // Hide confirmation dialog and show bottom trading panel
      showBottomPanel.value = true;
      // Also need to hide the confirmation dialog
      handleOrderCancellation();
    };

    const onSymbolSelected = async (symbol) => {
      // Clear existing data
      clearAllSelections();
      optionsChainData.value = [];
      optionsDataByExpiration.value = {}; // Clear CollapsibleOptionsChain data
      expirationDates.value = [];
      selectedExpiry.value = null;

      // Update global symbol state (positions will be loaded automatically by useGlobalSymbol)
      updateSymbol(symbol);

      // Fetch new data for the selected symbol
      try {
        await fetchSymbolData(symbol.symbol);
        await fetchExpirationDates(symbol.symbol);

        if (selectedExpiry.value) {
          onExpiryChange();
        }
      } catch (error) {
        console.error("Error loading data for new symbol:", error);
      }
    };

    const onTradeModeChanged = (mode) => {
      selectedTradeMode.value = mode;
      
      // Clear selections when switching modes
      clearAllSelections();
      
      // Show appropriate panels based on mode
      if (mode === "shares") {
        showBottomPanel.value = false;
        showSharesPanel.value = true;
        isRightPanelExpanded.value = false;
      } else if (mode === "options") {
        showSharesPanel.value = false;
        // showBottomPanel will be controlled by options selection logic
      }
    };

    // Shares-specific methods
    const onSharesReviewAndSend = (orderData) => {
      // Hide shares panel and show confirmation dialog
      showSharesPanel.value = false;
      initializeOrder(orderData);
    };

    const onSharesClearTrade = () => {
      // Clear any shares-specific state if needed
      console.log("Shares trade cleared");
    };

    // Window height tracking for responsive chart
    const windowHeight = ref(window.innerHeight);
    
    // Dynamic chart height calculation
    const chartHeight = computed(() => {
      // Calculate available space dynamically
      // TopBar: ~60px, SymbolHeader: ~80px
      const fixedElements = 140;
      
      // Trading panel height varies based on mode and visibility
      let tradingPanelHeight = 0;
      if (selectedTradeMode.value === 'shares' && showSharesPanel.value) {
        // SharesTradingPanel without stats header is smaller (~160px)
        tradingPanelHeight = 160;
      } else if (selectedTradeMode.value === 'options' && showBottomPanel.value) {
        // BottomTradingPanel with stats is larger (~200px)
        tradingPanelHeight = 200;
      }
      
      // Chart controls at bottom: ~60px
      const chartControls = 60;
      
      // Calculate remaining space for chart using reactive window height
      const availableHeight = windowHeight.value - fixedElements - tradingPanelHeight - chartControls;
      
      // Ensure minimum height
      return Math.max(availableHeight, 300);
    });

    // Live price data for chart
    const livePrice = computed(() => {
      // This would come from the smart market data store
      return {
        price: currentPrice.value,
        bid: currentPrice.value - 0.01,
        ask: currentPrice.value + 0.01,
      };
    });

    const onRightPanelCollapsed = () => {
      // Right panel collapsed
    };

    const onPositionsChanged = (checkedPositions) => {
      activePositions.value = checkedPositions;
      // The watcher on activePositions will handle the chart update
    };

    // CollapsibleOptionsChain event handlers - now using centralized manager
    const onExpirationExpanded = async (expirationKey, expirationData) => {
      await optionsManager.expandExpiration(expirationKey, expirationData);
    };

    const onExpirationCollapsed = (expirationDate) => {
      optionsManager.collapseExpiration(expirationDate);
    };

    const onStrikeCountChanged = (newStrikeCount) => {
      currentStrikeCount.value = newStrikeCount;
      optionsManager.updateStrikeCount(newStrikeCount);
    };

    const onPriceAdjusted = (data) => {
      adjustedNetCredit.value = data.adjustedNetCredit;
    };

    // Helper function to check if we have existing positions for current symbol
    const hasExistingPositionsForSymbol = () => {
      // This would need to check if there are existing positions
      // For now, return false to let selectedOptions watcher work
      // In a real implementation, this would check the positions API
      return false;
    };

    // Additional quote data for the right panel
    const additionalQuoteData = computed(() => ({
      // Mock data - in real implementation, this would come from API
      open: 623.16,
      close: 623.62,
      high: 625.16,
      low: 621.8,
      yearHigh: 626.87,
      yearLow: 481.8,
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

    const processPendingOrder = async (order) => {
      if (!order) return;

      clearAllSelections();

      const { legs, isOpposite } = order;
      if (!legs || legs.length === 0) return;

      // 1. Expand Expiration Dates
      const expirationDatesToExpand = new Set(
        legs.map((leg) => {
          const parsed = parseOptionSymbol(leg.symbol);
          return parsed ? parsed.expiry.substring(0, 10) : null;
        })
      );

      expirationDatesToExpand.forEach((expiryDate) => {
        if (expiryDate) {
          const expirationEntry = optionsManager.expirationDates.value.find(
            (e) => e.date === expiryDate
          );
          if (expirationEntry) {
            const uniqueKey = `${expirationEntry.date}-${expirationEntry.type}-${expirationEntry.symbol}`;
            optionsManager.expandExpiration(uniqueKey, expirationEntry);
          }
        }
      });

      // 2. Select the Legs
      legs.forEach((leg) => {
        const parsed = parseOptionSymbol(leg.symbol);
        if (parsed) {
          let side = leg.side.includes("buy") ? "buy" : "sell";
          if (isOpposite) {
            side = side === "buy" ? "sell" : "buy";
          }

          const selection = {
            symbol: leg.symbol,
            strike_price: parsed.strike,
            type: parsed.type,
            expiry: parsed.expiry,
            side: side,
            quantity: Math.abs(leg.qty),
          };
          addLeg(selection);
        }
      });
    };

    const parseOptionSymbol = (symbol) => {
      try {
        const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
        if (!match) return null;

        const [, underlying, dateStr, type, strikeStr] = match;

        const year = 2000 + parseInt(dateStr.substring(0, 2));
        const month = dateStr.substring(2, 4);
        const day = dateStr.substring(4, 6);
        const expiry = `${year}-${month.padStart(2, "0")}-${day.padStart(
          2,
          "0"
        )}`;

        const strike = parseInt(strikeStr) / 1000;

        return {
          underlying,
          expiry,
          type: type === "C" ? "call" : "put",
          strike,
        };
      } catch (error) {
        console.error("Error parsing option symbol:", symbol, error);
        return null;
      }
    };

    // Window resize handler for responsive chart
    const handleWindowResize = () => {
      windowHeight.value = window.innerHeight;
    };

    // Lifecycle hooks
    onMounted(async () => {
      // The SmartMarketDataStore is already connected via main.js.
      // We just need to fetch the initial data for the default symbol.
      await fetchSymbolData(currentSymbol.value);
      await fetchExpirationDates(currentSymbol.value);
      if (selectedExpiry.value) {
        onExpiryChange();
      }

      // Set up centralized symbol selection listener
      const cleanup = setupSymbolSelectionListener();

      // Listen for system recovery events to refresh data
      const handleSystemRecovery = async (event) => {
        try {
          // Refresh all critical data after recovery
          await fetchSymbolData(currentSymbol.value);
          await fetchExpirationDates(currentSymbol.value);
          
          // If we had a selected expiry, refresh its options chain
          if (selectedExpiry.value) {
          }
          
          // Refresh options manager data
          await optionsManager.refreshAllData();
        } catch (error) {
          console.error('❌ Error refreshing OptionsTrading data after recovery:', error);
        }
      };

      window.addEventListener("websocket-recovered", handleSystemRecovery);
      
      // Add window resize listener for responsive chart
      window.addEventListener('resize', handleWindowResize);
    });

    onUnmounted(() => {
      const cleanup = setupSymbolSelectionListener();
      cleanup();
      
      // Remove window resize listener
      window.removeEventListener('resize', handleWindowResize);
      // No need to remove the websocket-recovered event listener here as it's tied to the component's lifecycle
    });

    // Watchers

    // This single watcher now handles all chart updates.
    // It reacts to changes in selected legs, checked positions, price, and adjusted credit.
    watch(
      pendingOrder,
      async (newOrder) => {
        if (newOrder) {
          // Ensure expiration dates are loaded
          if (optionsManager.expirationDates.value.length === 0) {
            await optionsManager.loadExpirationDates();
          }
          await processPendingOrder(newOrder);
          clearPendingOrder();
        }
      },
      { immediate: true }
    );

    // Track previous leg count to detect changes
    const previousLegCount = ref(0);

    watch(
      [selectedLegs, activePositions, currentPrice, adjustedNetCredit],
      () => {
        // Reset adjustedNetCredit when number of legs changes
        const currentLegCount = selectedLegs.value.length;
        if (currentLegCount !== previousLegCount.value) {
          adjustedNetCredit.value = null; // Reset to use natural mid price
          previousLegCount.value = currentLegCount;
        }

        // PRIORITY SYSTEM for chart data:
        // 1. If user has explicitly checked positions in RightPanel, use those (activePositions)
        // 2. If no explicit position selections but we have selectedLegs, use those for chart
        // This ensures chart updates immediately when quantities change in BottomTradingPanel
        
        let positionsForChart = [];
        
        if (activePositions.value.length > 0) {
          // User has made explicit checkbox selections in RightPanel
          // Sync quantities for selected (non-existing) legs with the latest store values
          const legsMap = new Map(selectedLegs.value.map(leg => [leg.symbol, leg]));
          positionsForChart = activePositions.value.map((pos) => {
            if (!pos.isExisting && pos.asset_class === "us_option" && legsMap.has(pos.symbol)) {
              const leg = legsMap.get(pos.symbol);
              return {
                ...pos,
                // Ensure qty reflects current leg quantity and side
                qty: leg.side === "buy" ? leg.quantity : -leg.quantity,
                // Keep pricing in sync where possible (fallback to existing pos)
                current_price: leg.current_price || ((leg.bid + leg.ask) / 2) || pos.current_price || 0,
                avg_entry_price: leg.avg_entry_price ?? pos.avg_entry_price,
              };
            }
            return pos;
          });
        } else if (selectedLegs.value.length > 0) {
          // No explicit selections, but we have legs from options chain - convert for chart
          positionsForChart = selectedLegs.value.map((leg, index) => ({
            id: `selected:${leg.symbol}:${index}`,
            symbol: leg.symbol,
            asset_class: "us_option",
            qty: leg.side === "buy" ? leg.quantity : -leg.quantity,
            strike_price: leg.strike_price,
            option_type: leg.type,
            expiry_date: leg.expiry,
            current_price: leg.current_price || ((leg.bid + leg.ask) / 2) || 0,
            avg_entry_price: leg.avg_entry_price,
            unrealized_pl: 0,
            isExisting: false,
            isSelected: true,
          }));
        }
        
        updateChartData(positionsForChart);
        
        const hasLegsForChart = positionsForChart.length > 0;
        const hasLegsForTicket = selectedLegs.value.length > 0;

        showBottomPanel.value = hasLegsForTicket;
        
        if (hasLegsForChart) {
          isRightPanelExpanded.value = true;
        } else if (selectedLegs.value.length > 0) {
          // Expand panel when options are selected (so user can see position details)
          isRightPanelExpanded.value = true;
        } else {
          isRightPanelExpanded.value = false;
        }
      },
      { deep: true, immediate: true }
    );

    watch(selectedExpiry, (newExpiry, oldExpiry) => {
      if (newExpiry && newExpiry !== oldExpiry) {
        onExpiryChange();
      }
    });


    // Watch for time and update marketStatus accordingly (use US/Eastern time)
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
      updateMarketStatus(isOpen ? "Market Open" : "Market Closed");
    }
    updateMarketStatusNow(); // run immediately on load
    const marketStatusInterval = setInterval(updateMarketStatusNow, 60000); // check every minute

    // Store cleanup for unmount
    const cleanupOnUnmounted = () => {
      clearInterval(marketStatusInterval);
      if (optionsManager) {
        optionsManager.clearAllData();
      }
    };

    onUnmounted(() => {
      cleanupOnUnmounted();
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
      selectedExpiry,
      expirationDates,
      optionsChainData,
      selectedLegs,
      chartData,
      selectedTradeMode,
      tradeModes,
      isRightPanelExpanded,
      showBottomPanel,
      showSharesPanel,

      // Options Manager
      optionsManager,

      // Computed
      priceChangeClass,
      marketStatusClass,
      estimatedCost,
      rightPanelSection,
      livePrice,
      chartHeight,

      // Methods
      onExpiryChange,
      onOptionSelected,
      onOptionDeselected,
      clearAllSelections,
      onOrderPlaced,
      toggleRightPanel,
      onReviewAndSend,
      handleOrderEdit,
      onSymbolSelected,
      onTradeModeChanged,
      onSharesReviewAndSend,
      onSharesClearTrade,
      onRightPanelCollapsed,
      onPositionsChanged,
      onExpirationExpanded,
      onExpirationCollapsed,
      onStrikeCountChanged,
      onPriceAdjusted,
      adjustedNetCredit,
      additionalQuoteData,

      // New props for CollapsibleOptionsChain
      optionsDataByExpiration,
      optionsChainLoading,
      optionsChainError,

      ivxData,

      // Order management
      showOrderConfirmation,
      showOrderResult,
      orderData,
      orderResult,
      isPlacingOrder,
      handleOrderConfirmation,
      handleOrderCancellation,
      handleOrderResultClose,
    };
  },
};
</script>

<style scoped>
.options-trading-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: #141519;
  color: #ffffff;
}

.main-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
  position: relative;
}

.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: #141519;
  transition: margin-right 0.3s ease;
}

.symbol-search-section {
  min-width: 300px;
  margin-right: 24px;
}

.symbol-info {
  display: flex;
  align-items: center;
  gap: 24px;
}

.symbol-details h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #ffffff;
}

.symbol-meta {
  display: flex;
  gap: 12px;
  margin-top: 4px;
}

.company-name {
  color: #cccccc;
  font-size: 14px;
}

.exchange {
  color: #888888;
  font-size: 14px;
}

.price-info {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.current-price {
  display: flex;
  align-items: center;
  gap: 8px;
}

.price {
  font-size: 20px;
  font-weight: 600;
  color: #ffffff;
}

.change,
.change-percent {
  font-size: 14px;
  font-weight: 500;
}

.change.positive,
.change-percent.positive {
  color: #00c851;
}

.change.negative,
.change-percent.negative {
  color: #ff4444;
}

.change.neutral,
.change-percent.neutral {
  color: #cccccc;
}

.market-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-indicator.open {
  background-color: #00c851;
}

.status-indicator.closed {
  background-color: #ff4444;
}

.status-indicator.pre-market,
.status-indicator.after-hours {
  background-color: #ffbb33;
}

.status-text {
  font-size: 12px;
  color: #cccccc;
}

.trade-mode-selector {
  display: flex;
  align-items: center;
}

.mode-tabs {
  display: flex;
  background-color: #444444;
  border-radius: 6px;
  overflow: hidden;
}

.mode-tab {
  padding: 8px 16px;
  background: none;
  border: none;
  color: #cccccc;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mode-tab:hover {
  background-color: #555555;
  color: #ffffff;
}

.mode-tab.active {
  background-color: #007bff;
  color: #ffffff;
}

.options-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.shares-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chart-wrapper {
  flex: 1;
  padding: 16px 24px;
  overflow: hidden;
}

.expiration-selector {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  background-color: #141519;
  margin-bottom: 0;
}

.expiration-selector label {
  font-size: 14px;
  font-weight: 500;
  color: #cccccc;
}

.expiry-dropdown {
  min-width: 150px;
}

.options-chain-wrapper {
  flex: 1;
  overflow: hidden;
  padding: 16px 24px;
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
  background-color: #1a1d23;
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
  background-color: #1a1d23;
  border-bottom: 1px solid #2a2d33;
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

.chart-section {
  padding: 16px;
  border-bottom: 1px solid #444444;
}

.order-ticket-section {
  padding: 16px;
  border-bottom: 1px solid #444444;
}

.position-details-section {
  padding: 16px;
  flex: 1;
}

.position-card {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

.position-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.summary-item .label {
  color: #cccccc;
  font-size: 14px;
}

.summary-item .value {
  font-weight: 600;
  font-size: 14px;
}

.summary-item .value.credit {
  color: #00c851;
}

.summary-item .value.debit {
  color: #ff4444;
}

/* Dark theme overrides for PrimeVue components */
:deep(.p-dropdown) {
  background-color: #1a1d23;
  border: 1px solid #2a2d33;
  color: #ffffff;
}

:deep(.p-dropdown:not(.p-disabled):hover) {
  border-color: #3a3d43;
}

:deep(.p-dropdown-panel) {
  background-color: #1a1d23;
  border: 1px solid #2a2d33;
}

:deep(.p-dropdown-item) {
  color: #ffffff;
}

:deep(.p-dropdown-item:not(.p-highlight):not(.p-disabled):hover) {
  background-color: #2a2d33;
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
