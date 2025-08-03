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

        <!-- Options Chain Section -->
        <div class="options-section">
          <!-- Collapsible Options Chain -->
          <div class="options-chain-wrapper">
            <CollapsibleOptionsChain
              :symbol="currentSymbol"
              :underlyingPrice="currentPrice"
              :expirationDates="optionsManager.expirationDates.value"
              :optionsDataByExpiration="optionsManager.dataByExpiration.value"
              :loading="optionsManager.loading.value"
              :error="optionsManager.error.value"
              :currentStrikeCount="optionsManager.strikeCount.value"
              @expiration-expanded="onExpirationExpanded"
              @expiration-collapsed="onExpirationCollapsed"
              @strike-count-changed="onStrikeCountChanged"
            />
          </div>
        </div>
      </div>

      <!-- Right Panel -->
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

    <!-- Bottom Trading Panel -->
    <BottomTradingPanel
      :visible="showBottomPanel"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      @review-send="onReviewAndSend"
      @price-adjusted="onPriceAdjusted"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { DateTime } from "luxon";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import CollapsibleOptionsChain from "../components/CollapsibleOptionsChain.vue";
import PayoffChart from "../components/PayoffChart.vue";
import BottomTradingPanel from "../components/BottomTradingPanel.vue";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import RightPanel from "../components/RightPanel.vue";
import { useOrderManagement } from "../composables/useOrderManagement";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useOptionsChainManager } from "../composables/useOptionsChainManager";
import { useMarketData } from "../composables/useMarketData.js";
import { useSelectedLegs } from "../composables/useSelectedLegs.js";
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
    OrderConfirmationDialog,
    RightPanel,
  },
  setup() {
    // Use centralized order management
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
    } = useOrderManagement();

    // Use global symbol state
    const { globalSymbolState, updateSymbol, updatePrice, updateMarketStatus } =
      useGlobalSymbol();

    // Use centralized selected legs (positions loaded globally by useGlobalSymbol)
    const { selectedLegs, clearAll: clearSelectedLegs } = useSelectedLegs();

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
      return selectedLegs.value.length > 0 ? "analysis" : null;
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
          const dates = response.map((dateStr) => {
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

    const fetchOptionsChain = async (symbol, expiry) => {
      if (!symbol || !expiry) return;

      try {
        const chain = await api.getOptionsChain(symbol, expiry, "ALL");
        if (chain && chain.length > 0) {
          optionsChainData.value = chain.map((option) => ({
            ...option,
            strike_price: parseFloat(option.strike_price),
            bid: parseFloat(option.bid || option.close_price || 0),
            ask: parseFloat(option.ask || option.close_price || 0),
          }));
        }
      } catch (error) {
        console.error("Error fetching options chain:", error);
        optionsChainData.value = [];
      }
    };

    // The startStreaming logic is now handled globally by the SmartMarketDataStore
    // when it is initialized in main.js. We can remove this local function.
    // The onPriceUpdate logic is also handled inside the store.

    const onExpiryChange = async () => {
      if (selectedExpiry.value) {
        await fetchOptionsChain(currentSymbol.value, selectedExpiry.value);

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
      chartData.value = null;
      showBottomPanel.value = false;
      isRightPanelExpanded.value = false;
    };

    const updateChartData = (positions = null) => {
      // If positions are provided (from RightPanel), use them directly
      // Otherwise, use selectedLegs (from centralized store)
      let positionsToUse = positions;

      if (!positionsToUse) {
        if (selectedLegs.value.length === 0) {
          chartData.value = null;
          return;
        }

        try {
          // Convert selected legs to position format for chart
          positionsToUse = selectedLegs.value
            .map((leg) => {
              const price = leg.side === "buy" ? leg.ask : leg.bid;
              const quantity =
                leg.side === "buy" ? leg.quantity : -leg.quantity;

              return {
                symbol: leg.symbol,
                asset_class: "us_option",
                side: leg.side === "buy" ? "long" : "short",
                qty: quantity,
                strike_price: leg.strike_price,
                option_type: leg.type,
                expiry_date: leg.expiry,
                current_price: price,
                avg_entry_price: price,
                cost_basis: price * quantity * 100, // Keep the sign for proper cost calculation
                market_value: price * quantity * 100,
                unrealized_pl: 0,
                underlying_symbol: currentSymbol.value,
                is_synthetic: true,
              };
            })
            .filter(Boolean);
        } catch (error) {
          console.error(
            "Error converting selected legs to positions:",
            error
          );
          chartData.value = null;
          return;
        }
      }

      if (!positionsToUse || positionsToUse.length === 0) {
        chartData.value = null;
        return;
      }

      try {
        const payoffData = generateMultiLegPayoff(
          positionsToUse,
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
          await fetchOptionsChain(symbol.symbol, selectedExpiry.value);
          onExpiryChange();
        }
      } catch (error) {
        console.error("Error loading data for new symbol:", error);
      }
    };

    const onTradeModeChanged = (mode) => {
      selectedTradeMode.value = mode;
    };

    const onRightPanelCollapsed = () => {
      // Right panel collapsed
    };

    const onPositionsChanged = (checkedPositions) => {
      // Update chart data based on checked positions
      if (checkedPositions.length === 0) {
        updateChartData([]); // Use unified function with empty array
        return;
      }

      try {
        // Convert checked positions to chart format
        const positions = checkedPositions.map((position) => {
          return {
            symbol: position.symbol,
            asset_class: "us_option",
            side: position.qty > 0 ? "long" : "short",
            qty: position.qty, // Keep the original signed quantity
            strike_price: position.strike_price,
            option_type:
              position.option_type?.toUpperCase() ||
              position.type?.toUpperCase(),
            expiry_date: position.expiry_date || position.expiry,
            current_price: position.current_price,
            avg_entry_price: position.avg_entry_price,
            cost_basis: position.avg_entry_price * Math.abs(position.qty) * 100,
            market_value: position.current_price * position.qty * 100,
            unrealized_pl: position.unrealized_pl || 0,
            underlying_symbol: currentSymbol.value,
            is_synthetic: !position.isExisting, // Existing positions are not synthetic
            isExisting: position.isExisting, // Pass through the existing flag
          };
        });

        // Determine how to handle adjustedNetCredit based on position mix
        const hasExistingPositions = positions.some(pos => pos.isExisting);
        const hasNewPositions = positions.some(pos => !pos.isExisting);
        
        let effectiveAdjustedNetCredit = null;
        
        if (hasExistingPositions && hasNewPositions) {
          // Mixed scenario: existing + new positions
          // Use adjustedNetCredit only for the new positions
          // The chart generation will handle this by applying adjustedNetCredit
          // only to new positions while using actual prices for existing ones
          effectiveAdjustedNetCredit = adjustedNetCredit.value;
        } else if (hasExistingPositions && !hasNewPositions) {
          // Only existing positions: use null to calculate from actual prices
          effectiveAdjustedNetCredit = null;
        } else if (!hasExistingPositions && hasNewPositions) {
          // Only new positions: use adjustedNetCredit for limit price functionality
          effectiveAdjustedNetCredit = adjustedNetCredit.value;
        }

        try {
          const payoffData = generateMultiLegPayoff(
            positions,
            currentPrice.value,
            effectiveAdjustedNetCredit
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
      } catch (error) {
        console.error("Error updating chart data from positions:", error);
        console.error("Error details:", error.stack);
        // Ensure chart is cleared on error
        chartData.value = null;
      }
    };

    // CollapsibleOptionsChain event handlers - now using centralized manager
    const onExpirationExpanded = async (expirationDate) => {
      await optionsManager.expandExpiration(expirationDate);
    };

    const onExpirationCollapsed = (expirationDate) => {
      optionsManager.collapseExpiration(expirationDate);
    };

    const onStrikeCountChanged = (newStrikeCount) => {
      currentStrikeCount.value = newStrikeCount;
      optionsManager.updateStrikeCount(newStrikeCount);
    };

    const onPriceAdjusted = (data) => {
      // console.log("📊 OptionsTrading: Received price adjustment", {
      //   data: data,
      //   adjustedNetCredit: data.adjustedNetCredit,
      //   previousValue: adjustedNetCredit.value
      // });
      
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

    // Lifecycle hooks
    onMounted(async () => {
      // The SmartMarketDataStore is already connected via main.js.
      // We just need to fetch the initial data for the default symbol.
      await fetchSymbolData(currentSymbol.value);
      await fetchExpirationDates(currentSymbol.value);
      if (selectedExpiry.value) {
        await fetchOptionsChain(currentSymbol.value, selectedExpiry.value);
        onExpiryChange();
      }

      // Listen for symbol selection events from TopBar
      const handleSymbolSelection = (event) => {
        onSymbolSelected(event.detail);
      };

      // Listen for system recovery events to refresh data
      const handleSystemRecovery = async (event) => {
        try {
          // Refresh all critical data after recovery
          await fetchSymbolData(currentSymbol.value);
          await fetchExpirationDates(currentSymbol.value);
          
          // If we had a selected expiry, refresh its options chain
          if (selectedExpiry.value) {
            await fetchOptionsChain(currentSymbol.value, selectedExpiry.value);
          }
          
          // Refresh options manager data
          await optionsManager.refreshAllData();
        } catch (error) {
          console.error('❌ Error refreshing OptionsTrading data after recovery:', error);
        }
      };

      window.addEventListener("symbol-selected", handleSymbolSelection);
      window.addEventListener("websocket-recovered", handleSystemRecovery);

      // Store the handlers for cleanup
      window._symbolSelectionHandler = handleSymbolSelection;
      window._systemRecoveryHandler = handleSystemRecovery;
    });

    onUnmounted(() => {
      // Clean up event listeners
      if (window._symbolSelectionHandler) {
        window.removeEventListener(
          "symbol-selected",
          window._symbolSelectionHandler
        );
        delete window._symbolSelectionHandler;
      }
      
      if (window._systemRecoveryHandler) {
        window.removeEventListener(
          "websocket-recovered",
          window._systemRecoveryHandler
        );
        delete window._systemRecoveryHandler;
      }
    });

    // Watchers
    // Note: Chart generation is now handled entirely by RightPanel via onPositionsChanged
    // The selectedOptions watcher has been removed to prevent conflicts

    // Watch for underlying price changes to update chart
    watch(currentPrice, (newPrice, oldPrice) => {
      if (newPrice !== oldPrice && chartData.value) {
        // Regenerate chart data with new underlying price
        if (selectedLegs.value.length > 0) {
          updateChartData(); // This will use selectedLegs
        } else {
          // If we have chart data from positions, we need to trigger a positions update
          // The chart will automatically update via the PayoffChart watcher
        }
      }
    });

    // Watch for adjusted net credit changes to update chart
    watch(adjustedNetCredit, (newCredit, oldCredit) => {      
      if (newCredit !== oldCredit && (chartData.value || selectedLegs.value.length > 0)) {
        // Regenerate chart data with new adjusted net credit
        updateChartData();
      }
    });

    watch(selectedExpiry, (newExpiry, oldExpiry) => {
      if (newExpiry && newExpiry !== oldExpiry) {
        onExpiryChange();
      }
    });

    // Show/hide bottom panel and expand right panel based on selected legs
    watch(
      selectedLegs,
      (newLegs) => {
        const hasLegs = newLegs.length > 0;
        showBottomPanel.value = hasLegs;
        
        // Auto-expand right panel to Analysis tab when legs are selected
        if (hasLegs) {
          isRightPanelExpanded.value = true;
        } else {
          // Auto-collapse when no legs selected
          isRightPanelExpanded.value = false;
        }
      },
      { immediate: true }
    );

    // Clear selections when order is successfully placed
    watch(orderResult, (newResult) => {
      if (newResult && newResult.success) {
        // Order was successful, clear all selections
        clearAllSelections();
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
    setInterval(updateMarketStatusNow, 60000); // check every minute

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

      // Options Manager
      optionsManager,

      // Computed
      priceChangeClass,
      marketStatusClass,
      estimatedCost,
      rightPanelSection,

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
