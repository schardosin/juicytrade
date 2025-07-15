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
          <!-- Expiration Selector -->
          <div class="expiration-selector">
            <label>Expiration:</label>
            <Dropdown
              v-model="selectedExpiry"
              :options="expirationDates"
              optionLabel="label"
              optionValue="value"
              @change="onExpiryChange"
              class="expiry-dropdown"
            />
          </div>

          <!-- Options Chain -->
          <div class="options-chain-wrapper">
            <OptionsChain
              :key="selectedExpiry"
              :optionsData="optionsChainData"
              :underlyingPrice="currentPrice"
              :selectedOptions="selectedOptions"
              @option-selected="onOptionSelected"
              @option-deselected="onOptionDeselected"
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
        :selectedOptions="selectedOptions"
        :chartData="chartData"
        :additionalQuoteData="additionalQuoteData"
        :optionsChainData="optionsChainData"
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

    <!-- Order Result Dialog -->
    <OrderResultDialog
      :visible="showOrderResult"
      :orderResult="orderResult"
      @hide="handleOrderResultClose"
      @close="handleOrderResultClose"
      @viewPositions="handleOrderResultClose"
    />

    <!-- Bottom Trading Panel -->
    <BottomTradingPanel
      :visible="showBottomPanel"
      :selectedOptions="selectedOptions"
      :optionsData="optionsChainData"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      @clear-trade="clearAllSelections"
      @review-send="onReviewAndSend"
      @update-leg-quantity="onUpdateLegQuantity"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import OptionsChain from "../components/OptionsChain.vue";
import OrderTicket from "../components/OrderTicket.vue";
import PayoffChart from "../components/PayoffChart.vue";
import BottomTradingPanel from "../components/BottomTradingPanel.vue";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import OrderResultDialog from "../components/OrderResultDialog.vue";
import RightPanel from "../components/RightPanel.vue";
import { useOrderManagement } from "../composables/useOrderManagement";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";
import { generateMultiLegPayoff } from "../utils/chartUtils";

export default {
  name: "OptionsTrading",
  components: {
    TopBar,
    SideNav,
    SymbolHeader,
    OptionsChain,
    OrderTicket,
    PayoffChart,
    BottomTradingPanel,
    OrderConfirmationDialog,
    OrderResultDialog,
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

    // Local reactive data (non-symbol related)
    const selectedExpiry = ref(null);
    const expirationDates = ref([]);
    const optionsChainData = ref([]);
    const selectedOptions = ref([]);
    const chartData = ref(null);
    const selectedTradeMode = ref("options");
    const isRightPanelExpanded = ref(false);
    const showBottomPanel = ref(false);

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
      if (selectedOptions.value.length === 0) return null;

      let totalCost = 0;
      selectedOptions.value.forEach((selection) => {
        const option = optionsChainData.value.find(
          (opt) => opt.symbol === selection.symbol
        );
        if (option) {
          const price = selection.side === "buy" ? option.ask : option.bid;
          const cost = selection.side === "buy" ? -price : price;
          totalCost += cost * selection.quantity * 100; // Options are in contracts of 100
        }
      });

      return totalCost;
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
        } else {
          console.log(
            "Skipping API call - already have live price data for",
            symbol
          );
        }

        // Use unified subscription replacement
        if (webSocketClient.isConnected) {
          // Replace with new underlying symbol and clear options (will be set when expiration is selected)
          webSocketClient.replaceAllSubscriptions(symbol, []);
        } else {
          console.log(
            "⚠️ WebSocket not connected, will subscribe when connected"
          );
        }
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    const fetchExpirationDates = async (symbol) => {
      try {
        const response = await api.getAvailableExpirations(symbol);

        if (response && response.length > 0) {
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

          expirationDates.value = dates;
          if (dates.length > 0) {
            selectedExpiry.value = dates[0].value;
          }
        } else {
          console.warn("No expiration dates returned from API");
          expirationDates.value = [];
        }
      } catch (error) {
        console.error("Error fetching expiration dates from API:", error);
        expirationDates.value = [];
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

    const startStreaming = async (symbol) => {
      try {
        await webSocketClient.connect();

        // Use unified subscription replacement
        webSocketClient.replaceAllSubscriptions(symbol, []);

        // Ensure persistent subscriptions for orders and positions
        webSocketClient.ensurePersistentSubscriptions(["orders", "positions"]);

        webSocketClient.onPriceUpdate((data) => {
          // CRITICAL: Always check against current symbol, not the symbol from when callback was registered
          if (data.symbol === currentSymbol.value) {
            const newPrice =
              data.price ||
              data.data?.last ||
              data.data?.mid ||
              (data.data?.bid && data.data?.ask
                ? (data.data?.bid + data.data?.ask) / 2
                : null) ||
              data.data?.bid ||
              data.data?.ask;

            updatePrice({ price: newPrice, isLive: true });

            // Update market status based on time (simplified)
            const now = new Date();
            const hour = now.getHours();
            let status;
            if (hour >= 9 && hour < 16) {
              status = "Market Open";
            } else if (hour >= 4 && hour < 9) {
              status = "Pre-Market";
            } else {
              status = "Market Closed";
            }
            updateMarketStatus(status);
          } else {
            // Handle option price updates - only for current symbol's options
            const optionIndex = optionsChainData.value.findIndex(
              (o) => o.symbol === data.symbol
            );
            if (optionIndex !== -1) {
              //console.log(`Updating price for ${data.symbol}:`, data.data);
              const newBid = data.data.bid;
              const newAsk = data.data.ask;

              // Create a new object to ensure reactivity
              const updatedOption = {
                ...optionsChainData.value[optionIndex],
                bid: newBid,
                ask: newAsk,
              };

              // Replace the old object with the new one
              const newOptionsData = [...optionsChainData.value];
              newOptionsData[optionIndex] = updatedOption;
              optionsChainData.value = newOptionsData;
            }
          }
        });
      } catch (error) {
        console.error("Error starting streaming:", error);
      }
    };

    const onExpiryChange = async () => {
      if (selectedExpiry.value) {
        await fetchOptionsChain(currentSymbol.value, selectedExpiry.value);

        // Use unified subscription replacement with underlying + options
        if (optionsChainData.value.length > 0) {
          const optionSymbols = optionsChainData.value.map(
            (option) => option.symbol
          );
          webSocketClient.replaceAllSubscriptions(
            currentSymbol.value,
            optionSymbols
          );
        } else {
          // Clear options subscriptions if no options available, keep underlying
          console.log(
            "Clearing options subscriptions (no options available), keeping underlying"
          );
          webSocketClient.replaceAllSubscriptions(currentSymbol.value, []);
        }
      }
    };

    const onOptionSelected = (optionSelection) => {
      const existingIndex = selectedOptions.value.findIndex(
        (sel) => sel.symbol === optionSelection.symbol
      );

      // Add expiry date to the selection
      const selectionWithExpiry = {
        ...optionSelection,
        expiry: selectedExpiry.value,
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
      updateChartData();
    };

    const onOptionDeselected = (symbol) => {
      const index = selectedOptions.value.findIndex(
        (sel) => sel.symbol === symbol
      );
      if (index >= 0) {
        selectedOptions.value.splice(index, 1);
        updateChartData();

        // Auto-hide panel if no options left
        if (selectedOptions.value.length === 0) {
          showBottomPanel.value = false;
          isRightPanelExpanded.value = false;
        }
      }
    };

    const clearAllSelections = () => {
      selectedOptions.value = [];
      chartData.value = null;
      showBottomPanel.value = false;
      isRightPanelExpanded.value = false;
    };

    const updateChartData = () => {
      if (selectedOptions.value.length === 0) {
        chartData.value = null;
        return;
      }

      try {
        // Convert selected options to position format for chart
        const positions = selectedOptions.value
          .map((selection) => {
            const option = optionsChainData.value.find(
              (opt) => opt.symbol === selection.symbol
            );
            if (!option) return null;

            const price = selection.side === "buy" ? option.ask : option.bid;
            const quantity =
              selection.side === "buy"
                ? selection.quantity
                : -selection.quantity;

            return {
              symbol: option.symbol,
              asset_class: "us_option",
              side: selection.side === "buy" ? "long" : "short",
              qty: quantity,
              strike_price: option.strike_price,
              option_type: option.type,
              expiry_date: selectedExpiry.value,
              current_price: price,
              avg_entry_price: price,
              cost_basis: price * Math.abs(quantity) * 100,
              market_value: price * quantity * 100,
              unrealized_pl: 0,
              underlying_symbol: currentSymbol.value,
              is_synthetic: true,
            };
          })
          .filter(Boolean);

        if (positions.length > 0) {
          const payoffData = generateMultiLegPayoff(
            positions,
            currentPrice.value
          );
          chartData.value = payoffData;
        }
      } catch (error) {
        console.error("Error updating chart data:", error);
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

    const onUpdateLegQuantity = ({ symbol, quantity }) => {
      const optionIndex = selectedOptions.value.findIndex(
        (sel) => sel.symbol === symbol
      );
      if (optionIndex >= 0) {
        selectedOptions.value[optionIndex].quantity = quantity;
        updateChartData();
      }
    };

    const handleOrderEdit = () => {
      // Hide confirmation dialog and show bottom trading panel
      showBottomPanel.value = true;
      // Also need to hide the confirmation dialog
      handleOrderCancellation();
    };

    const onSymbolSelected = async (symbol) => {
      console.log("Symbol selected:", symbol);

      // Clear existing data
      clearAllSelections();
      optionsChainData.value = [];
      expirationDates.value = [];
      selectedExpiry.value = null;

      // Update global symbol state
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
      console.log("Right panel collapsed");
    };

    const onPositionsChanged = (checkedPositions) => {
      // Update chart data based on checked positions
      if (checkedPositions.length === 0) {
        chartData.value = null;
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
          };
        });

        if (positions.length > 0) {
          const payoffData = generateMultiLegPayoff(
            positions,
            currentPrice.value
          );
          chartData.value = payoffData;
        }
      } catch (error) {
        console.error("Error updating chart data from positions:", error);
        console.error("Error details:", error.stack);
      }
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
      // First, establish WebSocket connection and set up streaming
      await startStreaming(currentSymbol.value);

      // Then fetch symbol data (this will use replaceStockSubscription for subsequent changes)
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

      window.addEventListener("symbol-selected", handleSymbolSelection);

      // Store the handler for cleanup
      window._symbolSelectionHandler = handleSymbolSelection;
    });

    onUnmounted(() => {
      // Clean up event listener
      if (window._symbolSelectionHandler) {
        window.removeEventListener(
          "symbol-selected",
          window._symbolSelectionHandler
        );
        delete window._symbolSelectionHandler;
      }
    });

    // Watchers
    watch(selectedOptions, updateChartData, { deep: true });

    watch(selectedExpiry, (newExpiry, oldExpiry) => {
      if (newExpiry && newExpiry !== oldExpiry) {
        onExpiryChange();
      }
    });

    // Show/hide bottom panel based on selected options
    watch(
      selectedOptions,
      (newOptions) => {
        showBottomPanel.value = newOptions.length > 0;
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
      selectedOptions,
      chartData,
      selectedTradeMode,
      tradeModes,
      isRightPanelExpanded,
      showBottomPanel,

      // Computed
      priceChangeClass,
      marketStatusClass,
      estimatedCost,

      // Methods
      onExpiryChange,
      onOptionSelected,
      onOptionDeselected,
      clearAllSelections,
      onOrderPlaced,
      toggleRightPanel,
      onReviewAndSend,
      onUpdateLegQuantity,
      handleOrderEdit,
      onSymbolSelected,
      onTradeModeChanged,
      onRightPanelCollapsed,
      onPositionsChanged,
      additionalQuoteData,

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
