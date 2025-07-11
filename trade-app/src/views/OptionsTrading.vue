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
        <div class="symbol-header">
          <div class="symbol-info">
            <div class="symbol-details">
              <h2 class="symbol-name">{{ currentSymbol }}</h2>
              <div class="symbol-meta">
                <span class="company-name">{{ companyName }}</span>
                <span class="exchange">{{ exchange }}</span>
              </div>
            </div>
            <div class="price-info">
              <div class="current-price">
                <span class="price"
                  >${{ currentPrice?.toFixed(2) || "--" }}</span
                >
                <span class="change" :class="priceChangeClass">
                  {{ priceChange >= 0 ? "+" : ""
                  }}{{ priceChange?.toFixed(2) || "--" }}
                </span>
                <span class="change-percent" :class="priceChangeClass">
                  ({{ priceChangePercent >= 0 ? "+" : ""
                  }}{{ priceChangePercent?.toFixed(2) || "--" }}%)
                </span>
              </div>
              <div class="market-status">
                <span
                  class="status-indicator"
                  :class="marketStatusClass"
                ></span>
                <span class="status-text">{{ marketStatus }}</span>
              </div>
            </div>
          </div>

          <!-- Trade Mode Selector -->
          <div class="trade-mode-selector">
            <div class="mode-tabs">
              <button
                v-for="mode in tradeModes"
                :key="mode.value"
                :class="[
                  'mode-tab',
                  { active: selectedTradeMode === mode.value },
                ]"
                @click="selectedTradeMode = mode.value"
              >
                {{ mode.label }}
              </button>
            </div>
          </div>
        </div>

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
      <div class="right-panel" :class="{ expanded: isRightPanelExpanded }">
        <!-- Panel Header with Expand/Collapse Button -->
        <div class="panel-header">
          <h3 class="panel-title">Analysis & Trading</h3>
          <button
            class="expand-toggle-btn"
            @click="toggleRightPanel"
            :title="isRightPanelExpanded ? 'Collapse Panel' : 'Expand Panel'"
          >
            {{ isRightPanelExpanded ? "▶" : "◀" }}
          </button>
        </div>

        <!-- Scrollable Content Area -->
        <div class="panel-content">
          <!-- Chart Section -->
          <div class="chart-section">
            <PayoffChart
              v-if="chartData"
              :chartData="chartData"
              :underlyingPrice="currentPrice"
              title="Position P&L"
              :showInfo="false"
              :height="isRightPanelExpanded ? '400px' : '300px'"
              :symbol="currentSymbol"
              :isLivePrice="isLivePrice"
            />
          </div>

          <!-- Order Ticket -->
          <div class="order-ticket-section" v-if="selectedOptions.length > 0">
            <OrderTicket
              :selectedOptions="selectedOptions"
              :optionsData="optionsChainData"
              :underlyingSymbol="currentSymbol"
              :underlyingPrice="currentPrice"
              @order-placed="onOrderPlaced"
              @clear-selections="clearAllSelections"
            />
          </div>

          <!-- Position Details -->
          <div class="position-details-section">
            <Card class="position-card">
              <template #title>Position Details</template>
              <template #content>
                <div class="position-summary">
                  <div class="summary-item">
                    <span class="label">Selected Contracts:</span>
                    <span class="value">{{ selectedOptions.length }}</span>
                  </div>
                  <div class="summary-item" v-if="estimatedCost !== null">
                    <span class="label">Estimated Cost:</span>
                    <span
                      class="value"
                      :class="estimatedCost >= 0 ? 'credit' : 'debit'"
                    >
                      ${{ Math.abs(estimatedCost).toFixed(2) }}
                      {{ estimatedCost >= 0 ? "CR" : "DB" }}
                    </span>
                  </div>
                </div>
              </template>
            </Card>
          </div>
        </div>
      </div>
    </div>

    <!-- Order Confirmation Dialog -->
    <OrderConfirmationDialog
      :visible="showOrderConfirmation"
      :orderData="orderData"
      :loading="isPlacingOrder"
      @hide="handleOrderCancellation"
      @confirm="handleOrderConfirmation"
      @cancel="handleOrderCancellation"
    />

    <!-- Order Result Dialog -->
    <OrderResultDialog
      :visible="showOrderResult"
      :orderResult="orderResult"
      @hide="handleOrderResultClose"
      @close="handleOrderResultClose"
      @viewPositions="handleOrderResultClose"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import OptionsChain from "../components/OptionsChain.vue";
import OrderTicket from "../components/OrderTicket.vue";
import PayoffChart from "../components/PayoffChart.vue";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import OrderResultDialog from "../components/OrderResultDialog.vue";
import { useOrderManagement } from "../composables/useOrderManagement";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";
import { generateMultiLegPayoff } from "../utils/chartUtils";

export default {
  name: "OptionsTrading",
  components: {
    TopBar,
    SideNav,
    OptionsChain,
    OrderTicket,
    PayoffChart,
    OrderConfirmationDialog,
    OrderResultDialog,
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

    // Reactive data
    const currentSymbol = ref("SPY");
    const companyName = ref("SPDR S&P 500 ETF Trust");
    const exchange = ref("NYSE Arca");
    const currentPrice = ref(null);
    const priceChange = ref(0);
    const priceChangePercent = ref(0);
    const isLivePrice = ref(false);
    const marketStatus = ref("Market Closed");
    const selectedExpiry = ref(null);
    const expirationDates = ref([]);
    const optionsChainData = ref([]);
    const selectedOptions = ref([]);
    const chartData = ref(null);
    const selectedTradeMode = ref("options");
    const isRightPanelExpanded = ref(false);

    // Trade modes
    const tradeModes = [
      { label: "Options", value: "options" },
      { label: "Shares", value: "shares" },
      { label: "Futures", value: "futures" },
    ];

    // Computed properties
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
        // Fetch current price and basic info
        const price = await api.getUnderlyingPrice(symbol);
        if (price !== null) {
          currentPrice.value = price;
          isLivePrice.value = false; // Will be updated by WebSocket
        }

        // Start WebSocket streaming for the symbol
        await startStreaming(symbol);
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    const fetchExpirationDates = async (symbol) => {
      try {
        console.log(`Fetching real expiration dates for ${symbol} from API...`);
        const response = await api.getAvailableExpirations(symbol);

        if (response && response.length > 0) {
          console.log(
            `Found ${response.length} expiration dates from API:`,
            response
          );

          const dates = response.map((dateStr) => {
            const date = new Date(dateStr);
            const today = new Date();
            return {
              label: date.toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
                year:
                  date.getFullYear() !== today.getFullYear()
                    ? "numeric"
                    : undefined,
              }),
              value: dateStr,
            };
          });

          expirationDates.value = dates;
          if (dates.length > 0) {
            selectedExpiry.value = dates[0].value;
          }

          console.log(`Successfully loaded ${dates.length} expiration dates`);
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
        webSocketClient.subscribe([symbol]);

        webSocketClient.onPriceUpdate((data) => {
          if (data.symbol === symbol) {
            const oldPrice = currentPrice.value;
            currentPrice.value =
              data.price ||
              data.data?.mid ||
              (data.data?.bid + data.data?.ask) / 2;
            isLivePrice.value = true;

            if (oldPrice !== null) {
              priceChange.value = currentPrice.value - oldPrice;
              priceChangePercent.value = (priceChange.value / oldPrice) * 100;
            }

            // Update market status based on time (simplified)
            const now = new Date();
            const hour = now.getHours();
            if (hour >= 9 && hour < 16) {
              marketStatus.value = "Market Open";
            } else if (hour >= 4 && hour < 9) {
              marketStatus.value = "Pre-Market";
            } else if (hour >= 16 && hour < 20) {
              marketStatus.value = "After Hours";
            } else {
              marketStatus.value = "Market Closed";
            }
          }
        });
      } catch (error) {
        console.error("Error starting streaming:", error);
      }
    };

    const onExpiryChange = () => {
      if (selectedExpiry.value) {
        fetchOptionsChain(currentSymbol.value, selectedExpiry.value);
      }
    };

    const onOptionSelected = (optionSelection) => {
      const existingIndex = selectedOptions.value.findIndex(
        (sel) => sel.symbol === optionSelection.symbol
      );

      if (existingIndex >= 0) {
        // Update existing selection
        selectedOptions.value[existingIndex] = optionSelection;
      } else {
        // Add new selection
        selectedOptions.value.push(optionSelection);
      }

      updateChartData();
    };

    const onOptionDeselected = (symbol) => {
      const index = selectedOptions.value.findIndex(
        (sel) => sel.symbol === symbol
      );
      if (index >= 0) {
        selectedOptions.value.splice(index, 1);
        updateChartData();
      }
    };

    const clearAllSelections = () => {
      selectedOptions.value = [];
      chartData.value = null;
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

    // Lifecycle hooks
    onMounted(async () => {
      await fetchSymbolData(currentSymbol.value);
      await fetchExpirationDates(currentSymbol.value);
      if (selectedExpiry.value) {
        await fetchOptionsChain(currentSymbol.value, selectedExpiry.value);
      }
    });

    onUnmounted(() => {
      webSocketClient.disconnect();
    });

    // Watchers
    watch(selectedOptions, updateChartData, { deep: true });

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

.symbol-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background-color: #333333;
  border-bottom: 1px solid #444444;
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
  padding: 16px 24px;
}

.expiration-selector {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
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
}

.right-panel {
  width: 400px;
  background-color: #333333;
  border-left: 1px solid #444444;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: width 0.3s ease;
}

.right-panel.expanded {
  width: 700px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #3a3a3a;
  border-bottom: 1px solid #444444;
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
  min-width: 32px;
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
