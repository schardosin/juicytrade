<template>
  <div class="trade-management-container">
    <!-- Loading State -->
    <Card v-if="loading" class="loading-card">
      <template #content>
        <div class="loading-content">
          <ProgressSpinner />
          <p>Loading positions...</p>
        </div>
      </template>
    </Card>

    <!-- Error State -->
    <Card v-else-if="error" class="error-card">
      <template #content>
        <Message severity="error" :closable="false">
          <div>
            <h3>Error Loading Positions</h3>
            <p>{{ error }}</p>
            <Button
              label="Retry"
              @click="fetchPositions"
              severity="secondary"
              size="small"
              class="mt-2"
            />
          </div>
        </Message>
      </template>
    </Card>

    <!-- No Positions State -->
    <Card
      v-else-if="!positions || positions.length === 0"
      class="no-positions-card"
    >
      <template #content>
        <Message severity="info" :closable="false">
          <div class="no-positions-content">
            <h3>No Active Positions</h3>
            <p>You don't have any open option positions to manage.</p>
            <p>Use the Trade Setup page to create new butterfly positions.</p>
          </div>
        </Message>
      </template>
    </Card>

    <!-- Active Positions - New Layout -->
    <div v-else class="main-container">
      <div class="main-layout">
        <!-- Left Column (2/3 width) - Chart and Options Chain -->
        <div class="left-column">
          <!-- Payoff Chart (top) -->
          <PayoffChart
            v-if="hasOptionPositions && chartData"
            :chartData="chartData"
            :underlyingPrice="underlyingPrice"
            title="Position Payoff Diagram"
            :showInfo="false"
            height="350px"
            :adjustmentIndicator="
              selectedOptions.length > 0
                ? `(Including ${selectedOptions.length} Selected Adjustment${
                    selectedOptions.length > 1 ? 's' : ''
                  })`
                : ''
            "
            :symbol="underlyingSymbol"
            :isLivePrice="isLivePrice"
          />

          <!-- Options Chain (bottom) -->
          <Card
            v-if="hasOptionPositions && optionsChain.length > 0"
            class="options-chain-card"
          >
            <template #title
              >Options Chain - {{ positionExpiry }} (Select up to 4)</template
            >
            <template #content>
              <div class="options-chain-container">
                <!-- Header Row -->
                <div class="options-header">
                  <div class="calls-header">
                    <div class="symbol-header">Symbol</div>
                    <div class="bid-header">Bid</div>
                    <div class="ask-header">Ask</div>
                  </div>
                  <div class="strike-header">Strike</div>
                  <div class="puts-header">
                    <div class="bid-header">Bid</div>
                    <div class="ask-header">Ask</div>
                    <div class="symbol-header">Symbol</div>
                  </div>
                </div>

                <!-- Options Rows -->
                <div class="options-list">
                  <div
                    v-for="strike in strikeList"
                    :key="strike"
                    class="option-strike-row"
                  >
                    <!-- Call Side -->
                    <div
                      v-if="getCallOption(strike)"
                      :class="[
                        'call-side',
                        {
                          'selected-sell':
                            getSelectionType(getCallOption(strike).symbol) ===
                            'sell',
                          'selected-buy':
                            getSelectionType(getCallOption(strike).symbol) ===
                            'buy',
                          'position-call-long':
                            getCallPositionType(strike) === 'long',
                          'position-call-short':
                            getCallPositionType(strike) === 'short',
                        },
                      ]"
                    >
                      <div class="symbol">
                        {{ getCallOption(strike).symbol }}
                      </div>
                      <div
                        class="bid"
                        @click="selectOption(getCallOption(strike), 'sell')"
                      >
                        ${{ getOptionBid(getCallOption(strike)).toFixed(2) }}
                      </div>
                      <div
                        class="ask"
                        @click="selectOption(getCallOption(strike), 'buy')"
                      >
                        ${{ getOptionAsk(getCallOption(strike)).toFixed(2) }}
                      </div>
                    </div>
                    <div v-else class="call-side empty">
                      <div class="symbol">-</div>
                      <div class="bid">-</div>
                      <div class="ask">-</div>
                    </div>

                    <!-- Strike Price (Center) -->
                    <div class="strike-center">${{ strike.toFixed(0) }}</div>

                    <!-- Put Side -->
                    <div
                      v-if="getPutOption(strike)"
                      :class="[
                        'put-side',
                        {
                          'selected-sell':
                            getSelectionType(getPutOption(strike).symbol) ===
                            'sell',
                          'selected-buy':
                            getSelectionType(getPutOption(strike).symbol) ===
                            'buy',
                          'position-put-long':
                            getPutPositionType(strike) === 'long',
                          'position-put-short':
                            getPutPositionType(strike) === 'short',
                        },
                      ]"
                    >
                      <div
                        class="bid"
                        @click="selectOption(getPutOption(strike), 'sell')"
                      >
                        ${{ getOptionBid(getPutOption(strike)).toFixed(2) }}
                      </div>
                      <div
                        class="ask"
                        @click="selectOption(getPutOption(strike), 'buy')"
                      >
                        ${{ getOptionAsk(getPutOption(strike)).toFixed(2) }}
                      </div>
                      <div class="symbol">
                        {{ getPutOption(strike).symbol }}
                      </div>
                    </div>
                    <div v-else class="put-side empty">
                      <div class="bid">-</div>
                      <div class="ask">-</div>
                      <div class="symbol">-</div>
                    </div>
                  </div>
                </div>
              </div>
            </template>
          </Card>
        </div>

        <!-- Right Column (1/3 width) - Position Summary, Order Ticket, and Active Positions -->
        <div class="right-column">
          <!-- Position Summary (top) -->
          <Card class="summary-card">
            <template #title>Position Summary</template>
            <template #content>
              <div class="summary-grid">
                <div class="summary-item">
                  <strong>Positions:</strong> {{ positions.length }}
                </div>
                <div class="summary-item">
                  <strong>{{ underlyingSymbol }} Price:</strong>
                  <span v-if="underlyingPrice !== null">
                    ${{ underlyingPrice.toFixed(2) }}
                    <span v-if="isLivePrice" class="live-indicator"
                      >(Live)</span
                    >
                  </span>
                  <span v-else>Loading...</span>
                </div>
                <div class="summary-item">
                  <strong>Current P&L:</strong>
                  <span :class="totalUnrealizedPL >= 0 ? 'profit' : 'loss'">
                    ${{ totalUnrealizedPL.toFixed(2) }}
                  </span>
                </div>
                <div class="summary-item">
                  <strong>Total Market Value:</strong> ${{
                    totalMarketValue.toFixed(2)
                  }}
                </div>
                <div v-if="chartData" class="summary-item">
                  <strong>BE Price:</strong>
                  <span v-if="chartData.breakEvenPoints.length > 0">
                    {{
                      chartData.breakEvenPoints
                        .map((p) => "$" + p.toFixed(2))
                        .join(", ")
                    }}
                  </span>
                  <span v-else>None calculated</span>
                </div>
                <div v-if="chartData" class="summary-item">
                  <strong>Max Loss:</strong>
                  <span class="loss">${{ chartData.maxLoss.toFixed(2) }}</span>
                </div>
              </div>
            </template>
          </Card>

          <!-- Compact Order Ticket (middle - only show if options are selected) -->
          <Card
            v-if="selectedOptions.length > 0"
            class="compact-order-ticket-card"
          >
            <template #title>
              <div class="order-ticket-header">
                <span>Order Ticket ({{ selectedOptions.length }}/4)</span>
                <Button
                  icon="pi pi-times"
                  severity="secondary"
                  size="small"
                  text
                  @click="clearAllSelections"
                  class="clear-all-btn"
                />
              </div>
            </template>
            <template #content>
              <div class="compact-order-ticket">
                <!-- Selected Options List -->
                <div class="selected-options-list">
                  <div
                    v-for="symbol in selectedOptions"
                    :key="symbol"
                    class="selected-option-row"
                    :class="getOrderRowClass(symbol)"
                  >
                    <div class="option-info">
                      <div class="option-details">
                        <span class="strike"
                          >${{
                            getOptionBySymbol(symbol)?.strike_price?.toFixed(0)
                          }}</span
                        >
                        <Tag
                          :value="
                            getOptionBySymbol(symbol)?.type?.toUpperCase()
                          "
                          :severity="
                            getOptionBySymbol(symbol)?.type === 'call'
                              ? 'success'
                              : 'info'
                          "
                          size="small"
                        />
                        <Tag
                          :value="getSelectionType(symbol)?.toUpperCase()"
                          :severity="
                            getSelectionType(symbol) === 'buy'
                              ? 'success'
                              : 'danger'
                          "
                          size="small"
                        />
                        <span class="option-price">
                          ${{ getOrderPrice(symbol).toFixed(2) }}
                        </span>
                      </div>
                    </div>
                    <div class="option-controls">
                      <InputNumber
                        :model-value="getOrderQuantity(symbol)"
                        :min="1"
                        :max="10"
                        size="small"
                        class="compact-qty-input"
                        @update:model-value="
                          updateOrderQuantity(symbol, $event)
                        "
                      />
                      <Button
                        icon="pi pi-times"
                        severity="danger"
                        size="small"
                        text
                        @click="removeSelection(symbol)"
                        class="remove-option-btn"
                      />
                    </div>
                  </div>
                </div>

                <!-- Order Summary -->
                <div class="compact-order-summary">
                  <div class="summary-line">
                    <span class="label">Net:</span>
                    <span class="value" :class="getTotalEstimateClass()">
                      ${{ getTotalEstimatedValue().toFixed(2) }}
                    </span>
                  </div>
                  <div class="summary-line">
                    <span class="label">Limit:</span>
                    <div class="limit-input-group">
                      <InputNumber
                        v-model="combinedOrderPrice"
                        :min="-100"
                        :max="100"
                        :step="0.01"
                        size="small"
                        class="compact-limit-input"
                        showButtons
                        buttonLayout="horizontal"
                      />
                      <span
                        class="order-type-badge"
                        :class="getTotalEstimateClass()"
                      >
                        {{ getTotalEstimatedValue() >= 0 ? "CR" : "DB" }}
                      </span>
                    </div>
                  </div>
                </div>

                <!-- Order Controls -->
                <div class="compact-order-controls">
                  <div class="control-row">
                    <Dropdown
                      v-model="orderType"
                      :options="orderTypeOptions"
                      optionLabel="label"
                      optionValue="value"
                      class="compact-dropdown"
                      size="small"
                    />
                    <Dropdown
                      v-model="timeInForce"
                      :options="timeInForceOptions"
                      optionLabel="label"
                      optionValue="value"
                      class="compact-dropdown"
                      size="small"
                    />
                  </div>
                  <Button
                    label="REVIEW & SEND"
                    severity="warning"
                    size="small"
                    class="compact-review-btn"
                    @click="reviewAndSendOrder"
                    :disabled="!canSubmitOrder"
                  />
                </div>
              </div>
            </template>
          </Card>

          <!-- Active Positions Table -->
          <Card class="positions-table-card">
            <template #title>
              <div class="positions-header">
                <span>Active Positions</span>
                <Button
                  v-if="selectedPositions.length > 0"
                  :label="`Close Selected (${selectedPositions.length})`"
                  severity="danger"
                  size="small"
                  @click="submitCloseOrder"
                  class="close-positions-btn"
                />
              </div>
            </template>
            <template #content>
              <DataTable :value="positions" class="p-datatable-sm" stripedRows>
                <Column header="Select" style="width: 60px">
                  <template #body="slotProps">
                    <Checkbox
                      v-model="selectedPositions"
                      :value="slotProps.data.symbol"
                      @change="updateClosePrice"
                    />
                  </template>
                </Column>
                <Column field="symbol" header="Symbol" :sortable="true">
                  <template #body="slotProps">
                    <span class="symbol-cell">{{ slotProps.data.symbol }}</span>
                  </template>
                </Column>
                <Column field="asset_class" header="Type" :sortable="true">
                  <template #body="slotProps">
                    <Tag
                      :value="
                        slotProps.data.asset_class === 'us_option'
                          ? 'Option'
                          : 'Stock'
                      "
                      :severity="
                        slotProps.data.asset_class === 'us_option'
                          ? 'info'
                          : 'success'
                      "
                    />
                  </template>
                </Column>
                <Column field="side" header="Side" :sortable="true">
                  <template #body="slotProps">
                    <Tag
                      :value="slotProps.data.side"
                      :severity="
                        slotProps.data.side === 'long' ? 'success' : 'danger'
                      "
                    />
                  </template>
                </Column>
                <Column field="qty" header="Quantity" :sortable="true">
                  <template #body="slotProps">
                    {{ slotProps.data.qty }}
                  </template>
                </Column>
                <Column
                  v-if="hasOptionPositions"
                  field="strike_price"
                  header="Strike"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    <span v-if="slotProps.data.strike_price">
                      ${{ slotProps.data.strike_price.toFixed(2) }}
                    </span>
                    <span v-else>-</span>
                  </template>
                </Column>
                <Column
                  v-if="hasOptionPositions"
                  field="option_type"
                  header="Type"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    <span v-if="slotProps.data.option_type">
                      {{ slotProps.data.option_type.toUpperCase() }}
                    </span>
                    <span v-else>-</span>
                  </template>
                </Column>
                <Column
                  v-if="hasOptionPositions"
                  field="expiry_date"
                  header="Expiry"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    <span v-if="slotProps.data.expiry_date">
                      {{ formatDate(slotProps.data.expiry_date) }}
                    </span>
                    <span v-else>-</span>
                  </template>
                </Column>
                <Column
                  field="current_price"
                  header="Current Price"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    ${{ slotProps.data.current_price.toFixed(2) }}
                  </template>
                </Column>
                <Column
                  field="market_value"
                  header="Market Value"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    ${{ slotProps.data.market_value.toFixed(2) }}
                  </template>
                </Column>
                <Column
                  field="unrealized_pl"
                  header="Unrealized P&L"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    <span
                      :class="
                        slotProps.data.unrealized_pl >= 0 ? 'profit' : 'loss'
                      "
                    >
                      ${{ slotProps.data.unrealized_pl.toFixed(2) }}
                    </span>
                  </template>
                </Column>
                <Column field="unrealized_plpc" header="P&L %" :sortable="true">
                  <template #body="slotProps">
                    <span
                      :class="
                        slotProps.data.unrealized_plpc >= 0 ? 'profit' : 'loss'
                      "
                    >
                      {{ (slotProps.data.unrealized_plpc * 100).toFixed(2) }}%
                    </span>
                  </template>
                </Column>
              </DataTable>
            </template>
          </Card>

          <!-- Open Orders Table -->
          <Card v-if="openOrders.length > 0" class="open-orders-table-card">
            <template #title>
              <div class="orders-header">
                <span>Open Orders ({{ openOrders.length }})</span>
              </div>
            </template>
            <template #content>
              <DataTable :value="openOrders" class="p-datatable-sm" stripedRows>
                <Column field="asset" header="Asset" :sortable="true">
                  <template #body="slotProps">
                    <span class="symbol-cell">{{ slotProps.data.asset }}</span>
                  </template>
                </Column>
                <Column field="order_type" header="Order Type" :sortable="true">
                  <template #body="slotProps">
                    <span class="order-type-cell">{{
                      slotProps.data.order_type
                    }}</span>
                  </template>
                </Column>
                <Column field="side" header="Side" :sortable="true">
                  <template #body="slotProps">
                    <Tag
                      v-if="slotProps.data.side !== '-'"
                      :value="slotProps.data.side"
                      :severity="
                        slotProps.data.side === 'buy' ? 'success' : 'danger'
                      "
                      size="small"
                    />
                    <span v-else>-</span>
                  </template>
                </Column>
                <Column field="qty" header="Qty" :sortable="true">
                  <template #body="slotProps">
                    {{ slotProps.data.qty }}
                  </template>
                </Column>
                <Column field="filled_qty" header="Filled Qty" :sortable="true">
                  <template #body="slotProps">
                    {{ slotProps.data.filled_qty }}
                  </template>
                </Column>
                <Column
                  field="avg_fill_price"
                  header="Avg Fill Price"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    <span v-if="slotProps.data.avg_fill_price">
                      ${{ slotProps.data.avg_fill_price.toFixed(2) }}
                    </span>
                    <span v-else>-</span>
                  </template>
                </Column>
                <Column field="status" header="Status" :sortable="true">
                  <template #body="slotProps">
                    <Tag
                      :value="slotProps.data.status"
                      :severity="getOrderStatusSeverity(slotProps.data.status)"
                      size="small"
                    />
                  </template>
                </Column>
                <Column field="source" header="Source" :sortable="true">
                  <template #body="slotProps">
                    <span class="source-cell">{{ slotProps.data.source }}</span>
                  </template>
                </Column>
                <Column
                  field="submitted_at"
                  header="Submitted At"
                  :sortable="true"
                >
                  <template #body="slotProps">
                    <span class="time-cell">{{
                      formatDateTime(slotProps.data.submitted_at)
                    }}</span>
                  </template>
                </Column>
                <Column field="filled_at" header="Filled At" :sortable="true">
                  <template #body="slotProps">
                    <span v-if="slotProps.data.filled_at" class="time-cell">
                      {{ formatDateTime(slotProps.data.filled_at) }}
                    </span>
                    <span v-else>-</span>
                  </template>
                </Column>
              </DataTable>
            </template>
          </Card>
        </div>
      </div>
    </div>

    <!-- Centralized Order Confirmation Dialog -->
    <OrderConfirmationDialog
      :visible="showOrderConfirmation"
      :orderData="orderData"
      :loading="isPlacingOrder"
      @hide="handleOrderCancellation"
      @confirm="handleOrderConfirmation"
      @cancel="handleOrderCancellation"
    />

    <!-- Centralized Order Result Dialog -->
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
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from "vue";
import { Chart, registerables } from "chart.js";
import Tag from "primevue/tag";
import Checkbox from "primevue/checkbox";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";
import {
  generateMultiLegPayoff,
  createMultiLegChartConfig,
} from "../utils/chartUtils";
import { useOrderManagement } from "../composables/useOrderManagement";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import OrderResultDialog from "../components/OrderResultDialog.vue";
import PayoffChart from "../components/PayoffChart.vue";

Chart.register(...registerables);

export default {
  name: "TradeManagement",
  components: {
    Tag,
    Checkbox,
    OrderConfirmationDialog,
    OrderResultDialog,
    PayoffChart,
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
      buildMultiLegOrderData,
      buildCloseOrderData,
    } = useOrderManagement();
    // Reactive data
    const loading = ref(true);
    const error = ref(null);
    const positions = ref([]);
    const openOrders = ref([]); // New reactive state for open orders
    const underlyingPrice = ref(null); // NO DEFAULT - wait for real data from Alpaca
    const isLivePrice = ref(false); // Track if price is live from WebSocket
    const symbol = ref("SPY"); // Default underlying symbol
    const chartData = ref(null);
    const streamingPrices = ref({});
    const isStreaming = ref(false);
    const optionsChain = ref([]);
    const selectedOptions = ref([]);
    const selectedOptionsMap = ref({}); // Track selection type: { symbol: 'buy'|'sell' }
    const orderQuantities = ref({}); // Track quantities for each selected option
    const orderPrices = ref({}); // Track custom prices for each selected option
    const orderType = ref("Limit");
    const timeInForce = ref("Day");
    const combinedOrderPrice = ref(0);

    // Position closing functionality
    const selectedPositions = ref([]);
    const closeOrderPrice = ref(0);

    // Computed properties
    const optionPositions = computed(() => {
      return positions.value.filter((pos) => pos.asset_class === "us_option");
    });

    const hasOptionPositions = computed(() => {
      return optionPositions.value.length > 0;
    });

    const underlyingSymbol = computed(() => {
      if (optionPositions.value.length > 0) {
        return optionPositions.value[0].underlying_symbol || "SPY";
      }
      return "SPY";
    });

    const totalUnrealizedPL = computed(() => {
      return positions.value.reduce(
        (sum, pos) => sum + (pos.unrealized_pl || 0),
        0
      );
    });

    const totalMarketValue = computed(() => {
      return positions.value.reduce(
        (sum, pos) => sum + Math.abs(pos.market_value || 0),
        0
      );
    });

    // Options chain computed properties
    const positionExpiry = computed(() => {
      if (optionPositions.value.length > 0) {
        return optionPositions.value[0].expiry_date || "";
      }
      return "";
    });

    const positionStrikes = computed(() => {
      return optionPositions.value
        .map((pos) => pos.strike_price)
        .filter(Boolean);
    });

    const strikeRange = computed(() => {
      if (positionStrikes.value.length === 0) return { min: 0, max: 0 };

      const min = Math.min(...positionStrikes.value);
      const max = Math.max(...positionStrikes.value);

      // Add 5 strikes above and below the extreme positions
      return {
        min: min - 5,
        max: max + 5,
      };
    });

    const callOptions = computed(() => {
      return optionsChain.value
        .filter(
          (option) =>
            option.type === "call" &&
            option.strike_price >= strikeRange.value.min &&
            option.strike_price <= strikeRange.value.max
        )
        .sort((a, b) => a.strike_price - b.strike_price);
    });

    const putOptions = computed(() => {
      return optionsChain.value
        .filter(
          (option) =>
            option.type === "put" &&
            option.strike_price >= strikeRange.value.min &&
            option.strike_price <= strikeRange.value.max
        )
        .sort((a, b) => a.strike_price - b.strike_price);
    });

    // Strike list for the new layout
    const strikeList = computed(() => {
      const strikes = new Set();

      // Add all strikes from the range
      for (
        let strike = strikeRange.value.min;
        strike <= strikeRange.value.max;
        strike++
      ) {
        strikes.add(strike);
      }

      return Array.from(strikes).sort((a, b) => a - b);
    });

    // Methods
    const fetchPositions = async () => {
      loading.value = true;
      error.value = null;

      try {
        // Connect to WebSocket first
        await webSocketClient.connect();

        // Set up position handlers
        webSocketClient.onPositionsUpdate((positionsData) => {
          //console.log("Positions received via WebSocket:", positionsData);

          if (positionsData.success) {
            positions.value = positionsData.positions || [];

            // If we have option positions, generate chart with default price
            // Real price will come from WebSocket
            if (hasOptionPositions.value) {
              generateChart();
            }

            loading.value = false;

            // Automatically start streaming for all position symbols
            startStreamingForPositions();
          } else {
            error.value = positionsData.error || "Failed to fetch positions";
            loading.value = false;
          }
        });

        webSocketClient.onPositionsError((errorMsg) => {
          console.error("Positions error via WebSocket:", errorMsg);
          error.value = errorMsg || "Failed to fetch positions";
          loading.value = false;
        });

        // Set up open orders handler
        webSocketClient.onOpenOrdersUpdate((ordersData) => {
          console.log("Open orders received via WebSocket:", ordersData);
          if (ordersData.success) {
            openOrders.value = ordersData.orders || [];
            console.log(
              "Open orders set to:",
              openOrders.value.length,
              "orders"
            );
          }
        });

        // Request positions via WebSocket
        webSocketClient.requestPositions();

        // Request open orders via WebSocket
        webSocketClient.requestOpenOrders();
      } catch (err) {
        console.error("Error fetching positions:", err);
        error.value = err.message || "Failed to fetch positions";
        loading.value = false;
      }
    };

    const fetchUnderlyingPrice = async () => {
      try {
        const symbol = underlyingSymbol.value;
        const price = await api.getUnderlyingPrice(symbol);
        if (price !== null && !isNaN(price) && price > 0) {
          underlyingPrice.value = price;
        }
      } catch (err) {
        console.error("Error fetching underlying price:", err);
        // Keep default price if fetch fails
      }
    };

    const generateChart = async () => {
      if (!hasOptionPositions.value) {
        return;
      }

      try {
        // Start with current positions
        let combinedPositions = [...positions.value];

        // Add selected options as new positions using ADJUSTED ORDER PRICES
        if (selectedOptions.value.length > 0) {
          // Calculate price adjustment factor based on limit price vs natural price
          const naturalPrice = Math.abs(getTotalEstimatedValue()) / 100;
          const limitPrice = Math.abs(combinedOrderPrice.value) || naturalPrice;
          const priceAdjustmentFactor =
            naturalPrice > 0 ? limitPrice / naturalPrice : 1;

          console.log(
            `Chart: Price adjustment factor: ${priceAdjustmentFactor.toFixed(
              4
            )} (Limit: $${limitPrice.toFixed(
              2
            )}, Natural: $${naturalPrice.toFixed(2)})`
          );

          selectedOptions.value.forEach((symbol) => {
            const selectionType = selectedOptionsMap.value[symbol];
            const option = optionsChain.value.find(
              (opt) => opt.symbol === symbol
            );

            if (option && selectionType) {
              // Get base order price and apply adjustment factor
              const baseOrderPrice = getOrderPrice(symbol);
              const adjustedOrderPrice = baseOrderPrice * priceAdjustmentFactor;
              const quantity = getOrderQuantity(symbol);

              // For buy: qty = +quantity (long position)
              // For sell: qty = -quantity (short position)
              const positionQuantity =
                selectionType === "buy" ? quantity : -quantity;

              // Calculate cost basis using the adjusted order price
              const costBasis =
                adjustedOrderPrice * Math.abs(positionQuantity) * 100;

              const syntheticPosition = {
                symbol: option.symbol,
                asset_class: "us_option",
                side: selectionType === "buy" ? "long" : "short",
                qty: positionQuantity,
                strike_price: option.strike_price,
                option_type: option.type,
                expiry_date: positionExpiry.value,
                current_price: adjustedOrderPrice, // Use adjusted order price
                market_value:
                  adjustedOrderPrice * Math.abs(positionQuantity) * 100,
                avg_entry_price: adjustedOrderPrice, // Key field for chart calculation - use adjusted price
                cost_basis: costBasis,
                unrealized_pl: 0,
                unrealized_plpc: 0,
                underlying_symbol: underlyingSymbol.value,
                is_synthetic: true,
              };

              console.log(
                `Chart: Added ${selectionType} ${quantity} contracts of ${
                  option.symbol
                } at ADJUSTED PRICE $${adjustedOrderPrice.toFixed(
                  2
                )} (base: $${baseOrderPrice.toFixed(
                  2
                )}, factor: ${priceAdjustmentFactor.toFixed(4)})`
              );
              combinedPositions.push(syntheticPosition);
            }
          });
        }

        // Generate payoff data for combined positions (current + selected)
        const payoffData = generateMultiLegPayoff(
          combinedPositions,
          underlyingPrice.value
        );

        if (payoffData) {
          chartData.value = payoffData;
          // console.log(
          //   `Chart updated with ${combinedPositions.length} positions (${selectedOptions.value.length} adjustments)`
          // );
        }
      } catch (err) {
        console.error("Error generating chart:", err);
      }
    };

    const formatDate = (dateString) => {
      if (!dateString) return "";
      const date = new Date(dateString);
      return date.toLocaleDateString();
    };

    const formatDateTime = (dateString) => {
      if (!dateString) return "";
      const date = new Date(dateString);
      return date.toLocaleString("en-US", {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    };

    const getOrderStatusSeverity = (status) => {
      switch (status?.toLowerCase()) {
        case "new":
        case "accepted":
        case "pending_new":
          return "info";
        case "partially_filled":
          return "warning";
        case "filled":
          return "success";
        case "canceled":
        case "cancelled":
        case "rejected":
        case "expired":
          return "danger";
        default:
          return "secondary";
      }
    };

    // Streaming methods
    const startStreaming = async () => {
      if (isStreaming.value || !positions.value.length) {
        return;
      }

      try {
        //console.log("Starting WebSocket streaming for positions...");
        isStreaming.value = true;

        // Connect to WebSocket
        await webSocketClient.connect();

        // Collect all symbols to subscribe to
        const symbols = [];

        // Add underlying symbol
        if (underlyingSymbol.value) {
          symbols.push(underlyingSymbol.value);
        }

        // Add all position symbols
        positions.value.forEach((pos) => {
          if (pos.symbol) {
            symbols.push(pos.symbol);
          }
        });

        //console.log("Subscribing to symbols:", symbols);

        // Subscribe to all symbols
        webSocketClient.subscribe(symbols);

        // Set up price update handler
        webSocketClient.onPriceUpdate((data) => {
          //console.log("WebSocket price update received:", data);
          streamingPrices.value[data.symbol] = data.price;

          // Update underlying price if it's the underlying symbol
          if (data.symbol === underlyingSymbol.value) {
            underlyingPrice.value = data.price;
          }

          // Update position prices and recalculate P&L
          updatePositionPrices();
        });

        // Set up subscription confirmation handler
        webSocketClient.onSubscriptionConfirmed((message) => {
          //console.log("WebSocket subscription confirmed:", message);
        });

        //console.log("WebSocket streaming setup complete");
      } catch (err) {
        console.error("Error starting WebSocket streaming:", err);
        isStreaming.value = false;
      }
    };

    const stopStreaming = async () => {
      if (!isStreaming.value) {
        return;
      }

      try {
        console.log("Stopping WebSocket streaming...");
        webSocketClient.disconnect();
        isStreaming.value = false;
        streamingPrices.value = {};
      } catch (err) {
        console.error("Error stopping WebSocket streaming:", err);
      }
    };

    const startStreamingForPositions = async () => {
      if (isStreaming.value || !positions.value.length) {
        return;
      }

      try {
        console.log("Starting streaming for loaded positions...");
        isStreaming.value = true;

        // Collect all symbols from positions
        const symbols = [];

        // Add underlying symbol
        if (underlyingSymbol.value) {
          symbols.push(underlyingSymbol.value);
        }

        // Add all position symbols
        positions.value.forEach((pos) => {
          if (pos.symbol) {
            symbols.push(pos.symbol);
          }
        });

        //console.log("Auto-subscribing to position symbols:", symbols);

        // Subscribe to all symbols
        webSocketClient.subscribe(symbols);

        // Set up price update handler
        webSocketClient.onPriceUpdate((data) => {
          //console.log("Real-time price update:", data.symbol, data.price);
          streamingPrices.value[data.symbol] = data.price;

          // Update underlying price if it's the underlying symbol
          if (data.symbol === underlyingSymbol.value) {
            underlyingPrice.value = data.price;
          }

          // Update position prices and recalculate P&L
          updatePositionPrices();
        });

        // Set up subscription confirmation handler
        webSocketClient.onSubscriptionConfirmed((message) => {
          //console.log("Position symbols subscription confirmed:", message);
        });

        console.log("Position streaming setup complete");
      } catch (err) {
        console.error("Error starting position streaming:", err);
        isStreaming.value = false;
      }
    };

    const updatePositionPrices = () => {
      // Update position prices with streaming data
      positions.value.forEach((pos) => {
        if (streamingPrices.value[pos.symbol]) {
          const newPrice = streamingPrices.value[pos.symbol];
          const oldPrice = pos.current_price;

          // Update current price
          pos.current_price = newPrice;

          // Recalculate market value based on position side
          const multiplier = pos.asset_class === "us_option" ? 100 : 1;

          if (pos.side === "long") {
            // Long position: positive market value
            pos.market_value = newPrice * Math.abs(pos.qty) * multiplier;
          } else {
            // Short position: negative market value (you owe this amount)
            pos.market_value = -newPrice * Math.abs(pos.qty) * multiplier;
          }

          // Recalculate unrealized P&L
          const costBasis = pos.cost_basis || 0;

          // For P&L calculation:
          // Long: Current Market Value - Cost Basis
          // Short: Current Market Value - Cost Basis (where cost basis is negative for short)
          pos.unrealized_pl = pos.market_value - costBasis;
          pos.unrealized_plpc =
            Math.abs(costBasis) !== 0
              ? pos.unrealized_pl / Math.abs(costBasis)
              : 0;

          console.log(`Updated ${pos.symbol} (${pos.side}):`, {
            qty: pos.qty,
            currentPrice: newPrice,
            marketValue: pos.market_value,
            costBasis: costBasis,
            unrealizedPL: pos.unrealized_pl,
          });
        }
      });

      // Regenerate chart with new prices
      if (hasOptionPositions.value) {
        generateChart();
      }
    };

    // Options chain methods
    const fetchOptionsChain = async () => {
      if (!hasOptionPositions.value || !positionExpiry.value) {
        return;
      }

      try {
        // console.log(
        //   "Fetching options chain for:",
        //   underlyingSymbol.value,
        //   positionExpiry.value
        // );
        const chain = await api.getOptionsChain(
          underlyingSymbol.value,
          positionExpiry.value,
          "IRON Butterfly" // Use IRON Butterfly to get both calls and puts
        );

        if (chain && chain.length > 0) {
          // Transform the chain data to include strike_price as number and type
          optionsChain.value = chain.map((option) => ({
            ...option,
            strike_price: parseFloat(option.strike_price),
            type: option.type || (option.symbol.includes("C") ? "call" : "put"),
          }));

          // console.log(
          //   "Options chain loaded:",
          //   optionsChain.value.length,
          //   "options"
          // );

          // Subscribe to options chain symbols for real-time prices
          const chainSymbols = optionsChain.value
            .filter(
              (option) =>
                option.strike_price >= strikeRange.value.min &&
                option.strike_price <= strikeRange.value.max
            )
            .map((option) => option.symbol);

          if (chainSymbols.length > 0) {
            // console.log(
            //   "Subscribing to options chain symbols:",
            //   chainSymbols.length
            // );
            webSocketClient.subscribe(chainSymbols);
          }
        }
      } catch (error) {
        console.error("Error fetching options chain:", error);
      }
    };

    const isPositionStrike = (strike) => {
      return positionStrikes.value.some(
        (posStrike) => Math.abs(posStrike - strike) < 0.01
      );
    };

    // Check if there's a call position at this strike and return position type
    const getCallPositionType = (strike) => {
      const position = optionPositions.value.find(
        (pos) =>
          Math.abs(pos.strike_price - strike) < 0.01 &&
          pos.option_type === "call"
      );
      if (!position) return null;
      // Long position (qty > 0) = bought call, Short position (qty < 0) = sold call
      return position.qty > 0 ? "long" : "short";
    };

    // Check if there's a put position at this strike and return position type
    const getPutPositionType = (strike) => {
      const position = optionPositions.value.find(
        (pos) =>
          Math.abs(pos.strike_price - strike) < 0.01 &&
          pos.option_type === "put"
      );
      if (!position) return null;
      // Long position (qty > 0) = bought put, Short position (qty < 0) = sold put
      return position.qty > 0 ? "long" : "short";
    };

    // Legacy functions for backward compatibility
    const hasCallPosition = (strike) => {
      return getCallPositionType(strike) !== null;
    };

    const hasPutPosition = (strike) => {
      return getPutPositionType(strike) !== null;
    };

    const getOptionDisplayPrice = (option) => {
      // Try to get real-time price first
      if (streamingPrices.value[option.symbol]) {
        return streamingPrices.value[option.symbol];
      }

      // Fall back to close price
      return parseFloat(option.close_price || 0);
    };

    const toggleOptionSelection = (option) => {
      const symbol = option.symbol;
      const index = selectedOptions.value.indexOf(symbol);

      if (index > -1) {
        // Remove if already selected
        selectedOptions.value.splice(index, 1);
      } else if (selectedOptions.value.length < 4) {
        // Add if under limit
        selectedOptions.value.push(symbol);
      } else {
        // Show message if at limit
        console.log("Maximum 4 options can be selected");
      }
    };

    const removeSelection = (symbol) => {
      const index = selectedOptions.value.indexOf(symbol);
      if (index > -1) {
        selectedOptions.value.splice(index, 1);
        delete selectedOptionsMap.value[symbol];
      }
    };

    // New methods for buy/sell selection
    const selectOption = (option, type) => {
      const symbol = option.symbol;

      if (selectedOptionsMap.value[symbol] === type) {
        // If clicking the same type, deselect
        removeSelection(symbol);
      } else if (selectedOptionsMap.value[symbol]) {
        // If already selected with different type, change type
        selectedOptionsMap.value[symbol] = type;
      } else if (selectedOptions.value.length < 4) {
        // New selection
        selectedOptions.value.push(symbol);
        selectedOptionsMap.value[symbol] = type;
      } else {
        console.log("Maximum 4 options can be selected");
      }
    };

    const getSelectionType = (symbol) => {
      return selectedOptionsMap.value[symbol] || null;
    };

    // Order ticket methods
    const clearAllSelections = () => {
      selectedOptions.value = [];
      selectedOptionsMap.value = {};
      orderQuantities.value = {};
      orderPrices.value = {};
    };

    const getOrderQuantity = (symbol) => {
      return orderQuantities.value[symbol] || 1;
    };

    const updateOrderQuantity = (symbol, value) => {
      orderQuantities.value[symbol] = value || 1;
    };

    const getOrderPrice = (symbol) => {
      if (orderPrices.value[symbol]) {
        return orderPrices.value[symbol];
      }
      // Default to current bid/ask price
      const option = optionsChain.value.find((opt) => opt.symbol === symbol);
      const selectionType = selectedOptionsMap.value[symbol];
      if (option && selectionType) {
        return selectionType === "buy"
          ? getOptionAsk(option)
          : getOptionBid(option);
      }
      return 0;
    };

    const updateOrderPrice = (symbol, value) => {
      orderPrices.value[symbol] = value || 0;
    };

    const getOptionBySymbol = (symbol) => {
      return optionsChain.value.find((opt) => opt.symbol === symbol);
    };

    const formatExpiry = (dateString) => {
      if (!dateString) return "";
      const date = new Date(dateString);
      return date.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
      });
    };

    const getOrderRowClass = (symbol) => {
      const selectionType = selectedOptionsMap.value[symbol];
      return {
        "buy-row": selectionType === "buy",
        "sell-row": selectionType === "sell",
      };
    };

    const getEstimateClass = (symbol) => {
      const selectionType = selectedOptionsMap.value[symbol];
      const price = getOrderPrice(symbol);
      const quantity = getOrderQuantity(symbol);
      const value =
        selectionType === "buy"
          ? -price * quantity * 100
          : price * quantity * 100;

      return {
        credit: value > 0,
        debit: value < 0,
      };
    };

    const getEstimatedValue = (symbol) => {
      const selectionType = selectedOptionsMap.value[symbol];
      const price = getOrderPrice(symbol);
      const quantity = getOrderQuantity(symbol);

      // For buy: negative value (debit)
      // For sell: positive value (credit)
      return selectionType === "buy"
        ? -price * quantity * 100
        : price * quantity * 100;
    };

    const getTotalEstimateClass = () => {
      const total = getTotalEstimatedValue();
      return {
        credit: total > 0,
        debit: total < 0,
      };
    };

    const getTotalEstimatedValue = () => {
      return selectedOptions.value.reduce((total, symbol) => {
        return total + getEstimatedValue(symbol);
      }, 0);
    };

    // Order type and time in force options
    const orderTypeOptions = [
      { label: "Market", value: "Market" },
      { label: "Limit", value: "Limit" },
    ];

    const timeInForceOptions = [
      { label: "Day", value: "Day" },
      { label: "GTC", value: "GTC" },
      { label: "IOC", value: "IOC" },
      { label: "FOK", value: "FOK" },
    ];

    const canSubmitOrder = computed(() => {
      return (
        selectedOptions.value.length > 0 &&
        selectedOptions.value.every(
          (symbol) => getOrderQuantity(symbol) > 0 && getOrderPrice(symbol) > 0
        )
      );
    });

    const reviewAndSendOrder = () => {
      if (!canSubmitOrder.value) {
        console.warn("Cannot submit order: missing required fields");
        return;
      }

      // Build order data using the centralized composable
      const orderDataToSubmit = buildMultiLegOrderData({
        symbol: underlyingSymbol.value,
        expiry: positionExpiry.value,
        selectedOptions: selectedOptions.value,
        selectedOptionsMap: selectedOptionsMap.value,
        orderQuantities: orderQuantities.value,
        orderPrices: orderPrices.value,
        optionsChain: optionsChain.value,
        combinedOrderPrice: combinedOrderPrice.value,
        orderType: orderType.value,
        timeInForce: timeInForce.value,
        // Rich data for dialog display
        underlyingPrice: underlyingPrice.value,
        positions: positions.value,
      });

      // Initialize the centralized order flow
      initializeOrder(orderDataToSubmit);
    };

    // Helper methods for the new options chain layout
    const getCallOption = (strike) => {
      return callOptions.value.find(
        (option) => Math.abs(option.strike_price - strike) < 0.01
      );
    };

    const getPutOption = (strike) => {
      return putOptions.value.find(
        (option) => Math.abs(option.strike_price - strike) < 0.01
      );
    };

    const getOptionBid = (option) => {
      if (!option) return 0;

      // Try to get real-time bid price first
      if (
        streamingPrices.value[option.symbol] &&
        streamingPrices.value[option.symbol].bid
      ) {
        return streamingPrices.value[option.symbol].bid;
      }

      // Fall back to close price as estimate (real implementation would have bid/ask)
      return parseFloat(option.close_price || 0) * 0.98; // Estimate bid as 98% of close
    };

    const getOptionAsk = (option) => {
      if (!option) return 0;

      // Try to get real-time ask price first
      if (
        streamingPrices.value[option.symbol] &&
        streamingPrices.value[option.symbol].ask
      ) {
        return streamingPrices.value[option.symbol].ask;
      }

      // Fall back to close price as estimate (real implementation would have bid/ask)
      return parseFloat(option.close_price || 0) * 1.02; // Estimate ask as 102% of close
    };

    // Position closing methods
    const closeOrderType = ref("");

    const updateClosePrice = () => {
      if (selectedPositions.value.length === 0) {
        closeOrderPrice.value = 0;
        closeOrderType.value = "";
        return;
      }

      // Calculate net proceeds from closing all selected positions
      let netProceeds = 0;
      let debugInfo = [];

      selectedPositions.value.forEach((symbol) => {
        const position = positions.value.find((pos) => pos.symbol === symbol);
        if (position) {
          // For closing positions:
          // - Long position: we sell (receive money) = +price
          // - Short position: we buy (pay money) = -price
          const proceeds =
            position.qty > 0
              ? position.current_price * Math.abs(position.qty) // Long: sell = receive money (+)
              : -position.current_price * Math.abs(position.qty); // Short: buy = pay money (-)

          netProceeds += proceeds;

          debugInfo.push({
            symbol: position.symbol,
            qty: position.qty,
            price: position.current_price,
            action: position.qty > 0 ? "sell" : "buy",
            proceeds: proceeds,
          });
        }
      });

      // Determine order type
      closeOrderType.value = netProceeds >= 0 ? "CREDIT" : "DEBIT";

      console.log("Close order calculation:", {
        positions: debugInfo,
        netProceeds: netProceeds,
        orderType: closeOrderType.value,
        limitPrice: Math.abs(netProceeds),
      });

      // Set the signed price value
      // Positive netProceeds = Credit order (we receive money) = negative price for API
      // Negative netProceeds = Debit order (we pay money) = positive price for API
      // For API: credit orders need negative values, debit orders need positive values
      closeOrderPrice.value = parseFloat((-netProceeds).toFixed(2));
    };

    const submitCloseOrder = () => {
      if (selectedPositions.value.length === 0) {
        console.warn("Please select positions to close");
        return;
      }

      // Calculate a default close price based on current market values
      updateClosePrice();

      // Use the calculated close price or default to 1.0 if calculation fails
      const defaultClosePrice = closeOrderPrice.value || 1.0;

      // Build order data using the centralized composable
      const orderDataToSubmit = buildCloseOrderData({
        symbol: underlyingSymbol.value,
        expiry: positionExpiry.value,
        selectedPositions: selectedPositions.value,
        positions: positions.value,
        closeOrderPrice: defaultClosePrice,
        orderType: "limit",
        timeInForce: "day",
        // Rich data for dialog display
        underlyingPrice: underlyingPrice.value,
      });

      // Initialize the centralized order flow
      initializeOrder(orderDataToSubmit);
    };

    // Lifecycle hooks
    onMounted(async () => {
      await fetchPositions();

      // Start streaming after positions are loaded
      if (positions.value.length > 0) {
        await startStreaming();
      }
    });

    // Watch for underlying price changes to update chart
    watch(underlyingPrice, () => {
      if (hasOptionPositions.value && chartData.value) {
        generateChart();
      }
    });

    // Watch for selected options changes to update chart and combined price
    watch(
      [selectedOptions, selectedOptionsMap, orderQuantities, orderPrices],
      () => {
        if (hasOptionPositions.value) {
          generateChart();
        }
        // Only auto-calculate limit price when legs change, preserve manual edits
        const totalCost = getTotalEstimatedValue() / 100; // Convert from cents to dollars, keep sign
        // For limit price: positive net = credit = negative limit price
        // For limit price: negative net = debit = positive limit price
        const newPrice = parseFloat((-totalCost).toFixed(2)); // Invert the sign for limit price

        // Only update if the price has actually changed or if it's the initial calculation
        if (
          combinedOrderPrice.value === 0 ||
          Math.abs(Math.abs(combinedOrderPrice.value) - Math.abs(newPrice)) >
            0.01
        ) {
          combinedOrderPrice.value = newPrice; // Use inverted sign for limit price display
        }
      },
      { deep: true }
    );

    // Separate watcher for combinedOrderPrice changes (for chart updates only)
    watch(combinedOrderPrice, () => {
      if (hasOptionPositions.value) {
        generateChart();
      }
    });

    // Watch for positions to load options chain
    watch(
      [hasOptionPositions, positionExpiry],
      async ([hasPositions, expiry]) => {
        if (hasPositions && expiry) {
          await fetchOptionsChain();
        }
      }
    );

    onUnmounted(async () => {
      await stopStreaming();
    });

    return {
      // Reactive data
      loading,
      error,
      positions,
      openOrders,
      underlyingPrice,
      isLivePrice,
      symbol,
      chartData,
      optionsChain,
      selectedOptions,

      // Computed
      optionPositions,
      hasOptionPositions,
      underlyingSymbol,
      totalUnrealizedPL,
      totalMarketValue,
      positionExpiry,
      callOptions,
      putOptions,
      strikeList,

      // Methods
      fetchPositions,
      formatDate,
      formatDateTime,
      getOrderStatusSeverity,
      isPositionStrike,
      hasCallPosition,
      hasPutPosition,
      getCallPositionType,
      getPutPositionType,
      getOptionDisplayPrice,
      toggleOptionSelection,
      removeSelection,
      selectOption,
      getSelectionType,
      getCallOption,
      getPutOption,
      getOptionBid,
      getOptionAsk,

      // Order ticket methods
      clearAllSelections,
      getOrderQuantity,
      updateOrderQuantity,
      getOrderPrice,
      updateOrderPrice,
      getOptionBySymbol,
      formatExpiry,
      getOrderRowClass,
      getEstimateClass,
      getEstimatedValue,
      getTotalEstimateClass,
      getTotalEstimatedValue,
      orderType,
      timeInForce,
      orderTypeOptions,
      timeInForceOptions,
      canSubmitOrder,
      reviewAndSendOrder,
      combinedOrderPrice,

      // Position closing methods
      selectedPositions,
      closeOrderPrice,
      closeOrderType,
      updateClosePrice,
      submitCloseOrder,

      // Centralized order management
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
.trade-management-container {
  width: 100vw;
  height: 100vh;
  margin: 0;
  padding: 2px;
  box-sizing: border-box;
  overflow: hidden;
}

/* New Layout Structure */
.main-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: calc(100vh - 4px);
  gap: 10px;
}

.main-layout {
  display: flex;
  width: 100%;
  flex: 1;
  gap: 10px;
  min-height: 0;
}

/* Order ticket positioning */
.order-ticket-card {
  flex: 0 0 auto;
  margin-top: 10px;
}

.left-column {
  flex: 2;
  display: flex;
  flex-direction: column;
  gap: 5px;
  overflow: hidden;
}

.right-column {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 5px;
  overflow: hidden;
}

/* Adjust card heights for fixed layout */
.chart-card {
  flex: 1;
  min-height: 0;
}

.chart-card .chart-container {
  height: 350px;
}

.options-chain-card {
  flex: 1;
  min-height: 0;
}

.options-chain-card .options-chain-container {
  max-height: 400px;
  overflow-y: auto;
}

.summary-card {
  flex: 0 0 auto;
  min-height: 150px;
}

.positions-table-card {
  flex: 1;
  min-height: 0;
}

.positions-table-card :deep(.p-datatable-wrapper) {
  max-height: 400px;
  overflow-y: auto;
}

.header {
  text-align: center;
  margin-bottom: 30px;
}

.header h1 {
  color: #2c3e50;
  margin-bottom: 10px;
}

.header h2 {
  color: #7f8c8d;
  font-weight: normal;
}

.loading-card,
.error-card,
.no-positions-card,
.summary-card,
.positions-table-card,
.options-chain-card,
.chart-card {
  margin-bottom: 0px;
}

.loading-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px;
}

.loading-content p {
  margin-top: 10px;
  color: #6c757d;
}

.no-positions-content h3 {
  margin-bottom: 15px;
  color: #2c3e50;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 15px;
}

.summary-item {
  padding: 10px;
  background: #f8f9fa;
  border-radius: 6px;
}

.symbol-cell {
  font-family: monospace;
  font-weight: 600;
}

.profit {
  color: #28a745;
  font-weight: 600;
}

.loss {
  color: #dc3545;
  font-weight: 600;
}

.live-indicator {
  color: #28a745;
  font-size: 0.8em;
  font-weight: 500;
  margin-left: 4px;
}

.chart-container {
  height: 400px;
  position: relative;
}

.chart-info {
  background: #f8f9fa;
  padding: 15px;
  border-radius: 6px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 10px;
}

.mt-2 {
  margin-top: 0.5rem;
}

.mt-3 {
  margin-top: 1rem;
}

/* Options Chain Styles */
.options-chain-container {
  border: 1px solid #e9ecef;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 20px;
}

.options-header {
  display: grid;
  grid-template-columns: 1fr 120px 1fr;
  background: #f8f9fa;
  border-bottom: 2px solid #e9ecef;
  font-weight: 600;
  color: #495057;
}

.calls-header {
  display: grid;
  grid-template-columns: 1fr 60px 60px;
  gap: 8px;
  padding: 12px 16px;
  font-size: 0.875rem;
}

.puts-header {
  display: grid;
  grid-template-columns: 60px 60px 1fr;
  gap: 8px;
  padding: 12px 16px;
  font-size: 0.875rem;
}

.calls-label,
.puts-label {
  padding: 12px 16px;
  text-align: center;
  font-weight: 700;
  font-size: 0.9rem;
  background: #e9ecef;
  display: flex;
  align-items: center;
  justify-content: center;
}

.strike-header {
  padding: 12px 16px;
  text-align: center;
  font-weight: 700;
  font-size: 0.9rem;
  background: #dee2e6;
  min-width: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.bid-header,
.ask-header {
  font-size: 0.8rem;
  color: #6c757d;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
}

.calls-header .symbol-header {
  font-size: 0.8rem;
  color: #6c757d;
  text-align: left;
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

.puts-header .symbol-header {
  font-size: 0.8rem;
  color: #6c757d;
  text-align: right;
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.options-list {
  /* Full list display without scrolling */
}

.option-strike-row {
  display: grid;
  grid-template-columns: 1fr 120px 1fr;
  border-bottom: 1px solid #f1f3f4;
  transition: all 0.2s ease;
}

.option-strike-row.position-strike {
  background-color: #fff3cd;
}

/* Position highlighting - different colors for long vs short */
.call-side.position-call-long,
.put-side.position-put-long {
  background-color: #e3f2fd !important;
  border-left: 3px solid #2196f3;
}

.call-side.position-call-short,
.put-side.position-put-short {
  background-color: #fff3e0 !important;
  border-left: 3px solid #ff9800;
}

.call-side.position-call-long:hover,
.put-side.position-put-long:hover {
  background-color: #e3f2fd !important;
}

.call-side.position-call-short:hover,
.put-side.position-put-short:hover {
  background-color: #fff3e0 !important;
}

.call-side {
  display: grid;
  grid-template-columns: 1fr 60px 60px;
  gap: 8px;
  padding: 8px 16px;
  cursor: pointer;
  transition: all 0.2s ease;
  align-items: center;
}

.put-side {
  display: grid;
  grid-template-columns: 60px 60px 1fr;
  gap: 8px;
  padding: 8px 16px;
  cursor: pointer;
  transition: all 0.2s ease;
  align-items: center;
}

.call-side:hover,
.put-side:hover {
  background-color: #f8f9fa;
}

.call-side.selected-sell,
.put-side.selected-sell {
  background-color: #ffebee !important;
  border-left: 4px solid #f44336 !important;
}

.call-side.selected-buy,
.put-side.selected-buy {
  background-color: #e8f5e8 !important;
  border-left: 4px solid #4caf50 !important;
}

.call-side.empty,
.put-side.empty {
  color: #adb5bd;
  cursor: default;
}

.call-side.empty:hover,
.put-side.empty:hover {
  background-color: transparent;
}

.strike-center {
  padding: 8px 16px;
  text-align: center;
  font-weight: 700;
  color: #2c3e50;
  background: #f8f9fa;
  border-left: 1px solid #e9ecef;
  border-right: 1px solid #e9ecef;
  min-width: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.bid,
.ask {
  font-weight: 500;
  color: #495057;
  text-align: right;
  font-size: 0.875rem;
}

.call-side .symbol {
  font-family: monospace;
  font-size: 0.75em;
  color: #6c757d;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.put-side .symbol {
  font-family: monospace;
  font-size: 0.75em;
  color: #6c757d;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
}

.selected-summary {
  background: #f8f9fa;
  padding: 15px;
  border-radius: 6px;
  border: 1px solid #e9ecef;
}

.selected-summary h5 {
  margin: 0 0 10px 0;
  color: #495057;
}

.selected-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.selected-option {
  background: #007bff;
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.875rem;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.selected-option:hover {
  background: #0056b3;
}

.adjustment-indicator {
  color: #007bff;
  font-weight: normal;
  font-size: 0.9em;
  margin-left: 8px;
}

/* Order Ticket Styles */
.order-ticket {
  background: #2c2c2c;
  color: #ffffff;
  border-radius: 8px;
  padding: 20px;
  margin-top: 20px;
}

.order-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  border-bottom: 1px solid #444;
  padding-bottom: 15px;
}

.order-header h5 {
  color: #ffffff;
  margin: 0;
  font-size: 1.1rem;
}

.clear-btn {
  background: #6c757d !important;
  border: none !important;
}

.order-table-container {
  overflow-x: auto;
  margin-bottom: 20px;
}

.order-table {
  width: 100%;
  border-collapse: collapse;
  background: #333;
  border-radius: 6px;
  overflow: hidden;
}

.order-table th {
  background: #444;
  color: #ccc;
  padding: 12px 8px;
  text-align: center;
  font-size: 0.85rem;
  font-weight: 600;
  border-bottom: 1px solid #555;
}

.order-table td {
  padding: 10px 8px;
  text-align: center;
  border-bottom: 1px solid #444;
  vertical-align: middle;
}

.order-table tr.buy-row {
  background: rgba(76, 175, 80, 0.1);
}

.order-table tr.sell-row {
  background: rgba(244, 67, 54, 0.1);
}

.qty-cell :deep(.p-inputnumber) {
  width: 40px !important;
  min-width: 40px !important;
  max-width: 40px !important;
}

.qty-cell :deep(.p-inputnumber-input) {
  width: 40px !important;
  min-width: 40px !important;
  max-width: 40px !important;
  background: #555 !important;
  border: 1px solid #666 !important;
  color: #fff !important;
  text-align: center !important;
  padding: 2px 4px !important;
  font-size: 0.8rem !important;
}

.qty-cell :deep(.p-inputnumber-button-group) {
  display: none !important;
}

.price-input {
  width: 60px !important;
  background: #555 !important;
  border: 1px solid #666 !important;
  color: #fff !important;
}

.qty-input input,
.price-input input {
  background: #555 !important;
  color: #fff !important;
  text-align: center;
}

.type-tag,
.side-tag {
  font-size: 0.75rem;
}

.exp-cell,
.strike-cell {
  font-family: monospace;
  font-size: 0.9rem;
}

.credit-cell .credit {
  color: #4caf50;
  font-weight: 600;
}

.credit-cell .debit {
  color: #f44336;
  font-weight: 600;
}

.remove-btn {
  color: #f44336 !important;
}

.order-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 15px;
  margin-bottom: 20px;
  padding: 15px;
  background: #3a3a3a;
  border-radius: 6px;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.summary-label {
  color: #ccc;
  font-size: 0.9rem;
}

.summary-value {
  color: #fff;
  font-weight: 600;
}

.summary-value.credit {
  color: #4caf50;
}

.summary-value.debit {
  color: #f44336;
}

.order-type-dropdown,
.tif-dropdown {
  min-width: 120px;
}

.order-actions {
  text-align: center;
}

.review-send-btn {
  background: #ff9800 !important;
  border: none !important;
  color: #000 !important;
  font-weight: 700 !important;
  font-size: 1rem !important;
  padding: 12px 40px !important;
  border-radius: 6px !important;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.review-send-btn:hover:not(:disabled) {
  background: #f57c00 !important;
}

.review-send-btn:disabled {
  background: #666 !important;
  color: #999 !important;
}

/* Combined Price Section */
.combined-price-section {
  background: #3a3a3a;
  border-radius: 6px;
  padding: 15px;
  margin-bottom: 20px;
  border: 1px solid #555;
}

.price-row {
  display: flex;
  align-items: center;
  gap: 15px;
  justify-content: space-between;
}

.price-label {
  color: #ccc;
  font-size: 0.95rem;
  font-weight: 600;
  min-width: 140px;
}

.price-input-container {
  flex: 1;
  max-width: 150px;
}

.combined-price-input {
  width: 100% !important;
  background: #555 !important;
  border: 1px solid #666 !important;
  color: #fff !important;
}

.combined-price-input input {
  background: #555 !important;
  color: #fff !important;
  text-align: center;
  font-weight: 600;
}

.price-type {
  font-weight: 700;
  font-size: 0.9rem;
  padding: 4px 12px;
  border-radius: 4px;
  min-width: 60px;
  text-align: center;
}

.price-type.credit {
  color: #4caf50;
  background: rgba(76, 175, 80, 0.1);
  border: 1px solid #4caf50;
}

.price-type.debit {
  color: #f44336;
  background: rgba(244, 67, 54, 0.1);
  border: 1px solid #f44336;
}

.price-cell .price-label {
  color: #ccc;
  font-family: monospace;
  font-size: 0.9rem;
  font-weight: 600;
}

/* Compact Limit Price Input */
.limit-price-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.compact-price-input {
  width: 80px !important;
  min-width: 80px !important;
  max-width: 80px !important;
}

.compact-price-input :deep(.p-inputnumber) {
  width: 80px !important;
  min-width: 80px !important;
  max-width: 80px !important;
}

.compact-price-input :deep(.p-inputnumber-input) {
  width: 50px !important;
  background: #555 !important;
  border: 1px solid #666 !important;
  color: #fff !important;
  text-align: center !important;
  padding: 4px 2px !important;
  font-size: 0.85rem !important;
  font-weight: 600 !important;
}

.compact-price-input :deep(.p-inputnumber-button-up),
.compact-price-input :deep(.p-inputnumber-button-down) {
  background: #666 !important;
  border: 1px solid #777 !important;
  color: #fff !important;
  width: 15px !important;
  height: 15px !important;
}

.compact-price-input :deep(.p-inputnumber-button-up):hover,
.compact-price-input :deep(.p-inputnumber-button-down):hover {
  background: #777 !important;
}

.compact-price-input :deep(.p-inputnumber-button-up .p-button-icon),
.compact-price-input :deep(.p-inputnumber-button-down .p-button-icon) {
  font-size: 0.6rem !important;
}

.price-type-compact {
  font-weight: 600;
  font-size: 0.8rem;
  padding: 2px 8px;
  border-radius: 3px;
  min-width: 50px;
  text-align: center;
}

.price-type-compact.credit {
  color: #4caf50;
  background: rgba(76, 175, 80, 0.1);
  border: 1px solid #4caf50;
}

.price-type-compact.debit {
  color: #f44336;
  background: rgba(244, 67, 54, 0.1);
  border: 1px solid #f44336;
}

@media (max-width: 768px) {
  .options-chain-container {
    grid-template-columns: 1fr;
    gap: 15px;
  }

  .option-row {
    grid-template-columns: 60px 70px 1fr;
    gap: 8px;
    padding: 6px 10px;
  }

  .option-row .symbol {
    font-size: 0.8em;
  }

  .order-summary {
    grid-template-columns: 1fr;
  }

  .order-table-container {
    font-size: 0.8rem;
  }
}

/* Close Order Panel Styles */
.close-order-card {
  background: #2c2c2c;
  border: 1px solid #444;
}

.close-order-panel {
  padding: 15px;
}

.close-order-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.close-price-section {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
}

.close-price-label {
  color: #fff;
  font-weight: 600;
  min-width: 100px;
}

.close-price-input {
  width: 100px !important;
  min-width: 100px !important;
  max-width: 100px !important;
}

.close-price-input :deep(.p-inputnumber) {
  width: 100px !important;
  min-width: 100px !important;
  max-width: 100px !important;
}

.close-price-input :deep(.p-inputnumber-input) {
  width: 70px !important;
  background: #555 !important;
  border: 1px solid #666 !important;
  color: #fff !important;
  text-align: center !important;
  padding: 6px 4px !important;
  font-size: 0.9rem !important;
  font-weight: 600 !important;
}

.close-price-input :deep(.p-inputnumber-button-up),
.close-price-input :deep(.p-inputnumber-button-down) {
  background: #666 !important;
  border: 1px solid #777 !important;
  color: #fff !important;
  width: 15px !important;
  height: 20px !important;
}

.close-price-input :deep(.p-inputnumber-button-up):hover,
.close-price-input :deep(.p-inputnumber-button-down):hover {
  background: #777 !important;
}

.mid-price-note {
  color: #ccc;
  font-size: 0.8rem;
  font-style: italic;
}

.close-send-btn {
  background: #dc3545 !important;
  border: none !important;
  color: #fff !important;
  font-weight: 700 !important;
  font-size: 1rem !important;
  padding: 10px 30px !important;
  border-radius: 6px !important;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.close-send-btn:hover:not(:disabled) {
  background: #c82333 !important;
}

.close-send-btn:disabled {
  background: #666 !important;
  color: #999 !important;
}

.close-order-type {
  font-weight: 700;
  font-size: 0.85rem;
  padding: 3px 10px;
  border-radius: 4px;
  text-align: center;
  min-width: 60px;
  margin-left: 5px;
}

.close-order-type.credit {
  color: #4caf50;
  background: rgba(76, 175, 80, 0.15);
  border: 1px solid #4caf50;
}

.close-order-type.debit {
  color: #f44336;
  background: rgba(244, 67, 54, 0.15);
  border: 1px solid #f44336;
}

/* Positions Header Styles */
.positions-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.close-positions-btn {
  background: #dc3545 !important;
  border: none !important;
  color: #fff !important;
  font-weight: 600 !important;
  padding: 6px 12px !important;
  border-radius: 4px !important;
  font-size: 0.875rem !important;
}

.close-positions-btn:hover:not(:disabled) {
  background: #c82333 !important;
}

/* Compact Order Ticket Styles */
.compact-order-ticket-card {
  flex: 0 0 auto;
}

.order-ticket-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.clear-all-btn {
  color: #6c757d !important;
}

.compact-order-ticket {
  padding: 0;
}

.selected-options-list {
  margin-bottom: 15px;
}

.selected-option-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  margin-bottom: 8px;
  border-radius: 6px;
  border: 1px solid #e9ecef;
  background: #f8f9fa;
}

.selected-option-row.buy-row {
  background: rgba(76, 175, 80, 0.1);
  border-color: rgba(76, 175, 80, 0.3);
}

.selected-option-row.sell-row {
  background: rgba(244, 67, 54, 0.1);
  border-color: rgba(244, 67, 54, 0.3);
}

.option-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.option-details {
  display: flex;
  align-items: center;
  gap: 6px;
}

.option-details .strike {
  font-family: monospace;
  font-weight: 600;
  font-size: 0.9rem;
  color: #2c3e50;
}

.option-price {
  font-family: monospace;
  font-weight: 600;
  font-size: 0.85rem;
  color: #6c757d;
}

.option-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.compact-qty-input {
  width: 50px !important;
  min-width: 50px !important;
  max-width: 50px !important;
}

.compact-qty-input :deep(.p-inputnumber) {
  width: 50px !important;
  min-width: 50px !important;
  max-width: 50px !important;
}

.compact-qty-input :deep(.p-inputnumber-input) {
  width: 50px !important;
  min-width: 50px !important;
  max-width: 50px !important;
  text-align: center !important;
  padding: 4px 2px !important;
  font-size: 0.8rem !important;
}

.compact-qty-input :deep(.p-inputnumber-button-group) {
  display: none !important;
}

.remove-option-btn {
  color: #dc3545 !important;
  padding: 2px !important;
}

.compact-order-summary {
  background: #f8f9fa;
  border-radius: 6px;
  padding: 12px;
  margin-bottom: 15px;
}

.summary-line {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.summary-line:last-child {
  margin-bottom: 0;
}

.summary-line .label {
  font-size: 0.85rem;
  font-weight: 600;
  color: #495057;
}

.summary-line .value {
  font-size: 0.85rem;
  font-weight: 600;
}

.summary-line .value.credit {
  color: #28a745;
}

.summary-line .value.debit {
  color: #dc3545;
}

.limit-input-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.compact-limit-input {
  width: 70px !important;
  min-width: 70px !important;
  max-width: 70px !important;
}

.compact-limit-input :deep(.p-inputnumber) {
  width: 70px !important;
  min-width: 70px !important;
  max-width: 70px !important;
}

.compact-limit-input :deep(.p-inputnumber-input) {
  width: 45px !important;
  text-align: center !important;
  padding: 4px 2px !important;
  font-size: 0.8rem !important;
  font-weight: 600 !important;
}

.compact-limit-input :deep(.p-inputnumber-button-up),
.compact-limit-input :deep(.p-inputnumber-button-down) {
  width: 12px !important;
  height: 12px !important;
}

.compact-limit-input :deep(.p-inputnumber-button-up .p-button-icon),
.compact-limit-input :deep(.p-inputnumber-button-down .p-button-icon) {
  font-size: 0.5rem !important;
}

.order-type-badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 3px;
  text-align: center;
  min-width: 25px;
}

.order-type-badge.credit {
  color: #28a745;
  background: rgba(40, 167, 69, 0.1);
  border: 1px solid #28a745;
}

.order-type-badge.debit {
  color: #dc3545;
  background: rgba(220, 53, 69, 0.1);
  border: 1px solid #dc3545;
}

.compact-order-controls {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.control-row {
  display: flex;
  gap: 8px;
}

.compact-dropdown {
  flex: 1;
  min-width: 0;
}

.compact-dropdown :deep(.p-dropdown) {
  font-size: 0.8rem !important;
}

.compact-dropdown :deep(.p-dropdown-label) {
  padding: 4px 8px !important;
  font-size: 0.8rem !important;
}

.compact-review-btn {
  background: #ff9800 !important;
  border: none !important;
  color: #000 !important;
  font-weight: 700 !important;
  font-size: 0.85rem !important;
  padding: 8px 16px !important;
  border-radius: 4px !important;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  width: 100%;
}

.compact-review-btn:hover:not(:disabled) {
  background: #f57c00 !important;
}

.compact-review-btn:disabled {
  background: #6c757d !important;
  color: #adb5bd !important;
}
</style>
