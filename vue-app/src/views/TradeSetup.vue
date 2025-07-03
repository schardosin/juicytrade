<template>
  <div class="trade-setup-container">
    <div class="header">
      <h1>Robust Iron Butterfly Trade</h1>
      <h2>Step 1: Configure Butterfly Trade</h2>
    </div>

    <!-- Configuration Form -->
    <Card class="config-card">
      <template #title>Trade Configuration</template>
      <template #content>
        <div class="form-grid">
          <div class="field">
            <label for="expiry">Option Expiry Date</label>
            <Calendar
              id="expiry"
              v-model="expiry"
              dateFormat="yy-mm-dd"
              :minDate="new Date()"
            />
          </div>

          <div class="field">
            <label for="strategy">Butterfly Strategy Type</label>
            <Dropdown
              id="strategy"
              v-model="strategyType"
              :options="strategyOptions"
              optionLabel="label"
              optionValue="value"
            />
          </div>

          <div class="field">
            <label for="legWidth">Butterfly Leg Width</label>
            <InputNumber
              id="legWidth"
              v-model="legWidth"
              :min="1"
              :max="100"
              :step="1"
            />
          </div>

          <div class="field">
            <label for="orderOffset">Order Price Offset</label>
            <InputNumber
              id="orderOffset"
              v-model="orderOffset"
              :min="-10"
              :max="10"
              :step="0.01"
              :minFractionDigits="2"
              :maxFractionDigits="2"
            />
            <small
              >Amount to add/subtract from the calculated butterfly price to set
              your order price.</small
            >
          </div>

          <div class="field">
            <label for="symbol">Underlying Symbol</label>
            <InputText
              id="symbol"
              v-model="symbol"
              :maxlength="10"
              @blur="fetchUnderlyingPrice"
            />
          </div>
        </div>
      </template>
    </Card>

    <!-- Live Butterfly Data -->
    <div v-if="butterflyInfo" class="butterfly-section">
      <!-- Legs Table -->
      <Card class="legs-card">
        <template #title>Selected Butterfly Strikes (Live)</template>
        <template #content>
          <DataTable :value="legsTableData" class="p-datatable-sm">
            <Column field="action" header="Action"></Column>
            <Column field="type" header="Type"></Column>
            <Column field="strike" header="Strike">
              <template #body="slotProps">
                ${{ slotProps.data.strike.toFixed(2) }}
              </template>
            </Column>
            <Column field="price" header="Price">
              <template #body="slotProps">
                ${{ slotProps.data.price.toFixed(4) }}
              </template>
            </Column>
          </DataTable>
        </template>
      </Card>

      <!-- Payoff Chart -->
      <Card class="chart-card">
        <template #title>Butterfly Payoff Diagram</template>
        <template #content>
          <div class="chart-container">
            <canvas ref="chartCanvas"></canvas>
          </div>
          <div class="chart-info mt-3">
            <div class="info-grid">
              <div>
                <strong
                  >{{ symbol }} Price {{ isLivePrice ? "(Live)" : "" }}:</strong
                >
                ${{ underlyingPrice.toFixed(2) }}
              </div>
              <div>
                <strong>ATM Strike (Center):</strong> ${{
                  butterflyInfo.atm_strike.toFixed(2)
                }}
              </div>
              <div>
                <strong>Lower Break Even:</strong> ${{
                  chartData?.lowerBreakEven.toFixed(2)
                }}
              </div>
              <div>
                <strong>Upper Break Even:</strong> ${{
                  chartData?.upperBreakEven.toFixed(2)
                }}
              </div>
            </div>
          </div>
        </template>
      </Card>
    </div>

    <Divider />

    <!-- Place Order Section -->
    <Card class="order-card">
      <template #title>Place Order</template>
      <template #content>
        <Button
          label="Place Butterfly Order"
          @click="showOrderConfirmation = true"
          :disabled="!canPlaceOrder"
          :loading="placingOrder"
          severity="success"
          size="large"
        />
      </template>
    </Card>

    <!-- Order Confirmation Dialog -->
    <Dialog
      v-model:visible="showOrderConfirmation"
      modal
      header="Order Confirmation"
      :style="{ width: '50rem' }"
      @show="initializeOrderPrice"
    >
      <div v-if="orderDetails" class="order-confirmation">
        <div class="order-summary">
          <div class="summary-item">
            <strong>Current Options Price (legs combined):</strong> ${{
              orderDetails.currentOptionsPrice.toFixed(2)
            }}
          </div>
          <div class="summary-item">
            <strong>Calculated Order Price:</strong> ${{
              orderDetails.calculatedOrderPrice.toFixed(2)
            }}
          </div>
          <div class="summary-item">
            <strong>Underlying Price:</strong> ${{ underlyingPrice.toFixed(2) }}
          </div>
        </div>

        <!-- Editable Order Price -->
        <div class="order-price-section">
          <h4>Order Price</h4>
          <div class="field">
            <label for="manualOrderPrice">Your Order Price:</label>
            <InputNumber
              id="manualOrderPrice"
              v-model="manualOrderPrice"
              :min="-10"
              :max="10"
              :step="0.01"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              showButtons
              @input="updateOrderCalculations"
            />
            <small
              >Adjust the order price as needed before placing the order.</small
            >
          </div>
        </div>

        <!-- Updated calculations based on manual price -->
        <div class="order-summary">
          <div class="summary-item">
            <strong>Max Profit:</strong> ${{
              orderDetails.maxProfit.toFixed(2)
            }}
          </div>
          <div class="summary-item">
            <strong>Max Loss:</strong> ${{ orderDetails.maxLoss.toFixed(2) }}
          </div>
        </div>

        <h4>Legs to be Traded:</h4>
        <DataTable :value="orderDetails.legs" class="p-datatable-sm">
          <Column field="action" header="Action"></Column>
          <Column field="symbol" header="Symbol"></Column>
          <Column field="date" header="Date"></Column>
          <Column field="type" header="Type"></Column>
          <Column field="strike" header="Strike"></Column>
          <Column field="price" header="Price"></Column>
        </DataTable>
      </div>

      <template #footer>
        <Button
          label="Cancel"
          @click="showOrderConfirmation = false"
          severity="secondary"
        />
        <Button
          label="Confirm Order"
          @click="confirmOrder"
          :loading="placingOrder"
          severity="success"
        />
      </template>
    </Dialog>

    <!-- Order Result Dialog -->
    <Dialog
      v-model:visible="showOrderResult"
      modal
      :header="orderResult?.success ? 'Order Success' : 'Order Failed'"
      :style="{ width: '40rem' }"
    >
      <div v-if="orderResult?.success">
        <Message severity="success" :closable="false">
          Order Submitted Successfully!
        </Message>
        <pre class="order-result-json">{{
          JSON.stringify(orderResult.order, null, 2)
        }}</pre>
      </div>
      <div v-else>
        <Message severity="error" :closable="false">
          Order Failed: {{ orderResult?.error }}
        </Message>
      </div>

      <template #footer>
        <Button
          label="Close"
          @click="showOrderResult = false"
          severity="secondary"
        />
      </template>
    </Dialog>

    <!-- Service Status Footer -->
    <Divider />
    <Card class="footer-status">
      <template #title>Streaming Service Info</template>
      <template #content>
        <div v-if="serviceStatus">
          <pre>{{ JSON.stringify(serviceStatus, null, 2) }}</pre>
        </div>
        <div v-else>
          <Message severity="error" :closable="false">
            Streaming service is not running or not responding
          </Message>
        </div>
      </template>
    </Card>
  </div>
</template>

<script>
import {
  ref,
  reactive,
  computed,
  onMounted,
  onUnmounted,
  watch,
  nextTick,
} from "vue";
import { Chart, registerables } from "chart.js";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";
import {
  generateButterflyPayoff,
  createChartConfig,
} from "../utils/chartUtils";

Chart.register(...registerables);

export default {
  name: "TradeSetup",
  setup() {
    // Reactive data
    const serviceStatus = ref(null);
    const restartingService = ref(false);
    const expiry = ref(new Date());
    const strategyType = ref("IRON Butterfly");
    const legWidth = ref(2);
    const orderOffset = ref(0.05);
    const symbol = ref("SPY");
    const underlyingPrice = ref(null);
    const isLivePrice = ref(false);
    const optionsChain = ref([]);
    const butterflyInfo = ref(null);
    const chartData = ref(null);
    const showOrderConfirmation = ref(false);
    const showOrderResult = ref(false);
    const placingOrder = ref(false);
    const orderResult = ref(null);
    const chartCanvas = ref(null);
    const chart = ref(null);
    const manualOrderPrice = ref(null); // For user-editable order price

    // Options for dropdowns
    const strategyOptions = [
      { label: "IRON Butterfly", value: "IRON Butterfly" },
      { label: "CALL Butterfly", value: "CALL Butterfly" },
      { label: "PUT Butterfly", value: "PUT Butterfly" },
    ];

    // Computed properties
    const canPlaceOrder = computed(() => {
      return (
        butterflyInfo.value &&
        underlyingPrice.value !== null &&
        optionsChain.value.length > 0
      );
    });

    const orderDetails = computed(() => {
      if (!butterflyInfo.value || !chartData.value) return null;

      const netPremium = chartData.value.netPremium;
      const calculatedOrderPrice = netPremium + orderOffset.value;

      // Use manual order price if set, otherwise use calculated price
      const finalOrderPrice =
        manualOrderPrice.value !== null
          ? manualOrderPrice.value
          : calculatedOrderPrice;

      let maxProfit, maxLoss;
      if (strategyType.value === "IRON Butterfly") {
        maxProfit = Math.abs(finalOrderPrice) * 100;
        maxLoss = (legWidth.value - Math.abs(finalOrderPrice)) * 100;
      } else {
        maxProfit = (legWidth.value - Math.abs(finalOrderPrice)) * 100;
        maxLoss = Math.abs(finalOrderPrice) * 100;
      }

      let currentOptionsPrice;
      if (strategyType.value === "IRON Butterfly") {
        currentOptionsPrice =
          butterflyInfo.value.lower_price -
          butterflyInfo.value.atm_put_price -
          butterflyInfo.value.atm_call_price +
          butterflyInfo.value.upper_price;
      } else {
        currentOptionsPrice =
          butterflyInfo.value.lower_price -
          2 * butterflyInfo.value.atm_price +
          butterflyInfo.value.upper_price;
      }

      const legs = generateOrderLegs();

      return {
        orderPrice: finalOrderPrice,
        calculatedOrderPrice,
        currentOptionsPrice,
        maxProfit,
        maxLoss,
        legs,
      };
    });

    const legsTableData = computed(() => {
      if (!butterflyInfo.value) return [];

      if (strategyType.value === "IRON Butterfly") {
        return [
          {
            action: "Buy",
            type: "Put",
            strike: butterflyInfo.value.lower_strike,
            price: butterflyInfo.value.lower_price,
          },
          {
            action: "Sell",
            type: "Put",
            strike: butterflyInfo.value.atm_strike,
            price: butterflyInfo.value.atm_put_price || 0,
          },
          {
            action: "Sell",
            type: "Call",
            strike: butterflyInfo.value.atm_strike,
            price: butterflyInfo.value.atm_call_price || 0,
          },
          {
            action: "Buy",
            type: "Call",
            strike: butterflyInfo.value.upper_strike,
            price: butterflyInfo.value.upper_price,
          },
        ];
      } else if (strategyType.value === "CALL Butterfly") {
        return [
          {
            action: "Buy",
            type: "Call",
            strike: butterflyInfo.value.lower_strike,
            price: butterflyInfo.value.lower_price,
          },
          {
            action: "Sell",
            type: "Call",
            strike: butterflyInfo.value.atm_strike,
            price: butterflyInfo.value.atm_price || 0,
          },
          {
            action: "Buy",
            type: "Call",
            strike: butterflyInfo.value.upper_strike,
            price: butterflyInfo.value.upper_price,
          },
        ];
      } else {
        return [
          {
            action: "Buy",
            type: "Put",
            strike: butterflyInfo.value.lower_strike,
            price: butterflyInfo.value.lower_price,
          },
          {
            action: "Sell",
            type: "Put",
            strike: butterflyInfo.value.atm_strike,
            price: butterflyInfo.value.atm_price || 0,
          },
          {
            action: "Buy",
            type: "Put",
            strike: butterflyInfo.value.upper_strike,
            price: butterflyInfo.value.upper_price,
          },
        ];
      }
    });

    // Methods
    const checkServiceStatus = async () => {
      try {
        // Get WebSocket connection status
        const status = webSocketClient.getConnectionStatus();
        serviceStatus.value = {
          service_status: "running",
          websocket_connected: status.isConnected,
          subscribed_symbols: status.subscribedSymbols,
          reconnect_attempts: status.reconnectAttempts,
        };
      } catch (error) {
        console.error("Error checking service status:", error);
        serviceStatus.value = null;
      }
    };

    const restartStreamingService = async () => {
      restartingService.value = true;
      try {
        // Disconnect and reconnect WebSocket
        webSocketClient.disconnect();
        await webSocketClient.connect();
        await checkServiceStatus();
      } catch (error) {
        console.error("Error restarting service:", error);
      } finally {
        restartingService.value = false;
      }
    };

    const fetchUnderlyingPrice = async () => {
      if (!symbol.value) return;

      // Use WebSocket to get live price instead of HTTP
      console.log("Setting up WebSocket for underlying price:", symbol.value);

      try {
        // Connect to WebSocket and subscribe to underlying symbol
        await webSocketClient.connect();
        webSocketClient.subscribe([symbol.value]);

        // Set up price update handler for underlying
        webSocketClient.onPriceUpdate((data) => {
          if (data.symbol === symbol.value) {
            underlyingPrice.value = data.price;
            isLivePrice.value = true;
            console.log("WebSocket underlying price update:", data.price);
          }
        });

        // Set a default price while waiting for WebSocket data
        underlyingPrice.value = 620; // Default SPY price
        isLivePrice.value = false;
        console.log(
          "Set default underlying price, waiting for WebSocket updates"
        );
      } catch (error) {
        console.error(
          "Error setting up WebSocket for underlying price:",
          error
        );
        // Set a fallback price
        underlyingPrice.value = 620; // Default SPY price
        isLivePrice.value = false;
        console.warn("Using fallback price due to WebSocket error");
      }
    };

    const fetchOptionsChain = async () => {
      if (!symbol.value || !expiry.value) return;
      try {
        const expiryStr = expiry.value.toISOString().split("T")[0];
        const chain = await api.getOptionsChain(
          symbol.value,
          expiryStr,
          strategyType.value
        );
        optionsChain.value = chain || [];

        if (optionsChain.value.length > 0) {
          await calculateButterflyInfo();
        }
      } catch (error) {
        console.error("Error fetching options chain:", error);
        // Use mock data for testing when backend is unavailable
        optionsChain.value = generateMockOptionsChain();
        console.warn("Using mock options chain for testing");
        if (optionsChain.value.length > 0) {
          await calculateButterflyInfo();
        }
      }
    };

    const generateMockOptionsChain = () => {
      const basePrice = underlyingPrice.value || 450;
      const mockChain = [];

      // Generate strikes around the current price
      for (let i = -10; i <= 10; i++) {
        const strike = Math.round((basePrice + i * 2) / 5) * 5; // Round to nearest $5

        // Add PUT options
        mockChain.push({
          symbol: `SPY${expiry.value.toISOString().slice(2, 4)}${expiry.value
            .toISOString()
            .slice(5, 7)}${expiry.value.toISOString().slice(8, 10)}P${strike
            .toString()
            .padStart(8, "0")}`,
          strike_price: strike.toString(),
          type: "put",
          close_price: Math.max(0.1, Math.random() * 10).toFixed(2),
        });

        // Add CALL options
        mockChain.push({
          symbol: `SPY${expiry.value.toISOString().slice(2, 4)}${expiry.value
            .toISOString()
            .slice(5, 7)}${expiry.value.toISOString().slice(8, 10)}C${strike
            .toString()
            .padStart(8, "0")}`,
          strike_price: strike.toString(),
          type: "call",
          close_price: Math.max(0.1, Math.random() * 10).toFixed(2),
        });
      }

      return mockChain;
    };

    const calculateButterflyInfo = async () => {
      console.log("calculateButterflyInfo called with:", {
        optionsChainLength: optionsChain.value.length,
        underlyingPrice: underlyingPrice.value,
        isNaN: isNaN(underlyingPrice.value),
      });

      if (
        optionsChain.value.length === 0 ||
        underlyingPrice.value === null ||
        isNaN(underlyingPrice.value)
      ) {
        console.log("Early return: invalid options chain or underlying price");
        return;
      }

      const strikes = getStrikePrices();
      console.log("Available strikes:", strikes);
      if (strikes.length < 3) {
        console.log("Early return: not enough strikes", strikes.length);
        return;
      }

      const atmStrike = findClosestStrike(strikes, underlyingPrice.value);
      const lowerStrike = findLowerStrike(strikes, atmStrike);
      const upperStrike = findUpperStrike(strikes, atmStrike);

      console.log("Strike selection:", {
        atmStrike,
        lowerStrike,
        upperStrike,
      });

      if (lowerStrike === atmStrike || upperStrike === atmStrike) {
        console.log("Early return: invalid strike selection");
        return;
      }

      // Build initial butterfly info with strikes
      if (strategyType.value === "IRON Butterfly") {
        const putLower = findOption(lowerStrike, "put");
        const putAtm = findOption(atmStrike, "put");
        const callAtm = findOption(atmStrike, "call");
        const callUpper = findOption(upperStrike, "call");

        butterflyInfo.value = {
          lower_strike: lowerStrike,
          atm_strike: atmStrike,
          upper_strike: upperStrike,
          lower_price: parseFloat(putLower?.close_price || 0),
          atm_put_price: parseFloat(putAtm?.close_price || 0),
          atm_call_price: parseFloat(callAtm?.close_price || 0),
          upper_price: parseFloat(callUpper?.close_price || 0),
        };
      } else {
        const lowerOption = findOption(
          lowerStrike,
          strategyType.value === "CALL Butterfly" ? "call" : "put"
        );
        const atmOption = findOption(
          atmStrike,
          strategyType.value === "CALL Butterfly" ? "call" : "put"
        );
        const upperOption = findOption(
          upperStrike,
          strategyType.value === "CALL Butterfly" ? "call" : "put"
        );

        butterflyInfo.value = {
          lower_strike: lowerStrike,
          atm_strike: atmStrike,
          upper_strike: upperStrike,
          lower_price: parseFloat(lowerOption?.close_price || 0),
          atm_price: parseFloat(atmOption?.close_price || 0),
          upper_price: parseFloat(upperOption?.close_price || 0),
        };
      }

      // Start streaming - prices will be updated via WebSocket
      await startStreaming();

      // Generate chart data
      chartData.value = generateButterflyPayoff(
        butterflyInfo.value,
        strategyType.value,
        legWidth.value
      );

      // Update chart
      await nextTick();
      updateChart();
    };

    const getStrikePrices = () => {
      const strikes = new Set();
      for (const opt of optionsChain.value) {
        if (strategyType.value === "IRON Butterfly") {
          strikes.add(parseFloat(opt.strike_price));
        } else if (
          strategyType.value === "CALL Butterfly" &&
          opt.type === "call"
        ) {
          strikes.add(parseFloat(opt.strike_price));
        } else if (
          strategyType.value === "PUT Butterfly" &&
          opt.type === "put"
        ) {
          strikes.add(parseFloat(opt.strike_price));
        }
      }
      return Array.from(strikes).sort((a, b) => a - b);
    };

    const findClosestStrike = (strikes, target) => {
      return strikes.reduce((prev, curr) =>
        Math.abs(curr - target) < Math.abs(prev - target) ? curr : prev
      );
    };

    const findLowerStrike = (strikes, atmStrike) => {
      // Find strike that is legWidth away from ATM
      const targetLowerStrike = atmStrike - legWidth.value;

      // Find the closest available strike to the target
      const availableLowerStrikes = strikes.filter((s) => s < atmStrike);
      if (availableLowerStrikes.length > 0) {
        const result = availableLowerStrikes.reduce((prev, curr) =>
          Math.abs(curr - targetLowerStrike) <
          Math.abs(prev - targetLowerStrike)
            ? curr
            : prev
        );
        console.log(
          `Selected lower strike: ${result} (target was ${targetLowerStrike}, legWidth: ${legWidth.value})`
        );
        return result;
      }

      // Fallback: pick the lowest available strike
      const notAtm = strikes.filter((s) => s !== atmStrike);
      if (notAtm.length > 0) {
        const result = Math.min(...notAtm);
        console.log("Fallback lower strike (not ATM):", result);
        return result;
      }

      // As last resort, return ATM
      return atmStrike;
    };

    const findUpperStrike = (strikes, atmStrike) => {
      // Find strike that is legWidth away from ATM
      const targetUpperStrike = atmStrike + legWidth.value;

      // Find the closest available strike to the target
      const availableUpperStrikes = strikes.filter((s) => s > atmStrike);
      if (availableUpperStrikes.length > 0) {
        const result = availableUpperStrikes.reduce((prev, curr) =>
          Math.abs(curr - targetUpperStrike) <
          Math.abs(prev - targetUpperStrike)
            ? curr
            : prev
        );
        console.log(
          `Selected upper strike: ${result} (target was ${targetUpperStrike}, legWidth: ${legWidth.value})`
        );
        return result;
      }

      // Fallback: pick the highest available strike
      const notAtm = strikes.filter((s) => s !== atmStrike);
      if (notAtm.length > 0) {
        const result = Math.max(...notAtm);
        console.log("Fallback upper strike (not ATM):", result);
        return result;
      }

      // As last resort, return ATM
      return atmStrike;
    };

    const findOption = (strike, type) => {
      return optionsChain.value.find(
        (opt) =>
          Math.abs(parseFloat(opt.strike_price) - strike) < 0.01 &&
          opt.type === type
      );
    };

    const getOptionPrice = (option, prices) => {
      if (!option) return 0;
      const priceData = prices[option.symbol];
      if (priceData && priceData.mid !== undefined) {
        return priceData.mid;
      }
      return parseFloat(option.close_price || 0);
    };

    const getOptionSymbolsForLegs = (lowerStrike, atmStrike, upperStrike) => {
      const symbols = [];

      if (strategyType.value === "IRON Butterfly") {
        const putLower = findOption(lowerStrike, "put");
        const putAtm = findOption(atmStrike, "put");
        const callAtm = findOption(atmStrike, "call");
        const callUpper = findOption(upperStrike, "call");

        if (putLower) symbols.push(putLower.symbol);
        if (putAtm) symbols.push(putAtm.symbol);
        if (callAtm) symbols.push(callAtm.symbol);
        if (callUpper) symbols.push(callUpper.symbol);
      } else {
        const type = strategyType.value === "CALL Butterfly" ? "call" : "put";
        const lowerOption = findOption(lowerStrike, type);
        const atmOption = findOption(atmStrike, type);
        const upperOption = findOption(upperStrike, type);

        if (lowerOption) symbols.push(lowerOption.symbol);
        if (atmOption) symbols.push(atmOption.symbol);
        if (upperOption) symbols.push(upperOption.symbol);
      }

      return symbols;
    };

    const startStreaming = async () => {
      console.log("Starting WebSocket streaming subscriptions...");

      try {
        // Connect to WebSocket
        await webSocketClient.connect();

        // Collect all symbols to subscribe to
        const symbols = [];

        // Add underlying symbol
        if (symbol.value) {
          symbols.push(symbol.value);
        }

        // Add option symbols
        if (butterflyInfo.value) {
          const optionSymbols = getOptionSymbolsForLegs(
            butterflyInfo.value.lower_strike,
            butterflyInfo.value.atm_strike,
            butterflyInfo.value.upper_strike
          );
          symbols.push(...optionSymbols);
        }

        console.log("Subscribing to symbols via WebSocket:", symbols);

        // Subscribe to all symbols
        webSocketClient.subscribe(symbols);

        // Set up price update handler
        webSocketClient.onPriceUpdate((data) => {
          console.log("Real-time price update:", data.symbol, data.price);

          // Update underlying price if it's the underlying symbol
          if (data.symbol === symbol.value) {
            underlyingPrice.value = data.price;
            isLivePrice.value = true;
            throttledChartUpdate();
          } else {
            // Update option price
            updateOptionPrice(data.symbol, data.data);
          }
        });

        // Set up subscription confirmation handler
        webSocketClient.onSubscriptionConfirmed((message) => {
          console.log("WebSocket subscription confirmed:", message);
        });

        console.log("WebSocket streaming setup complete");
      } catch (error) {
        console.error("Error starting WebSocket streaming:", error);
      }
    };

    // Throttle chart updates to improve performance and prevent crashes
    let chartUpdateTimeout = null;
    let isUpdatingChart = false;

    const throttledChartUpdate = () => {
      // Prevent multiple simultaneous updates
      if (isUpdatingChart) {
        return;
      }

      if (chartUpdateTimeout) {
        clearTimeout(chartUpdateTimeout);
      }

      chartUpdateTimeout = setTimeout(async () => {
        try {
          isUpdatingChart = true;

          // Only update if we have valid data
          if (!butterflyInfo.value || !legWidth.value) {
            return;
          }

          // Recalculate chart data
          const newChartData = generateButterflyPayoff(
            butterflyInfo.value,
            strategyType.value,
            legWidth.value
          );

          // Only update if data actually changed
          if (
            JSON.stringify(newChartData) !== JSON.stringify(chartData.value)
          ) {
            chartData.value = newChartData;
            await updateChart();
          }
        } catch (error) {
          console.error("Error in throttled chart update:", error);
        } finally {
          isUpdatingChart = false;
        }
      }, 200); // Increased from 100ms to 200ms for more stability
    };

    const updateOptionPrice = (symbol, priceData) => {
      if (!butterflyInfo.value) return;

      // Find which leg this option belongs to and update the price
      const option = optionsChain.value.find((opt) => opt.symbol === symbol);
      if (!option) return;

      const strike = parseFloat(option.strike_price);
      const type = option.type;

      // Defensive: handle missing or invalid price data
      let midPrice = null;
      if (
        priceData &&
        typeof priceData.ask === "number" &&
        typeof priceData.bid === "number"
      ) {
        midPrice = (priceData.ask + priceData.bid) / 2;
      } else if (priceData && typeof priceData.ask === "number") {
        midPrice = priceData.ask;
      } else if (priceData && typeof priceData.bid === "number") {
        midPrice = priceData.bid;
      } else if (option && option.close_price) {
        midPrice = parseFloat(option.close_price);
      } else {
        midPrice = 0;
      }

      // Check if price actually changed to avoid unnecessary updates
      let priceChanged = false;

      if (strategyType.value === "IRON Butterfly") {
        if (
          Math.abs(strike - butterflyInfo.value.lower_strike) < 0.01 &&
          type === "put"
        ) {
          if (Math.abs(butterflyInfo.value.lower_price - midPrice) > 0.001) {
            butterflyInfo.value.lower_price = midPrice;
            priceChanged = true;
          }
        } else if (
          Math.abs(strike - butterflyInfo.value.atm_strike) < 0.01 &&
          type === "put"
        ) {
          if (Math.abs(butterflyInfo.value.atm_put_price - midPrice) > 0.001) {
            butterflyInfo.value.atm_put_price = midPrice;
            priceChanged = true;
          }
        } else if (
          Math.abs(strike - butterflyInfo.value.atm_strike) < 0.01 &&
          type === "call"
        ) {
          if (Math.abs(butterflyInfo.value.atm_call_price - midPrice) > 0.001) {
            butterflyInfo.value.atm_call_price = midPrice;
            priceChanged = true;
          }
        } else if (
          Math.abs(strike - butterflyInfo.value.upper_strike) < 0.01 &&
          type === "call"
        ) {
          if (Math.abs(butterflyInfo.value.upper_price - midPrice) > 0.001) {
            butterflyInfo.value.upper_price = midPrice;
            priceChanged = true;
          }
        }
      } else {
        if (Math.abs(strike - butterflyInfo.value.lower_strike) < 0.01) {
          if (Math.abs(butterflyInfo.value.lower_price - midPrice) > 0.001) {
            butterflyInfo.value.lower_price = midPrice;
            priceChanged = true;
          }
        } else if (Math.abs(strike - butterflyInfo.value.atm_strike) < 0.01) {
          if (Math.abs(butterflyInfo.value.atm_price - midPrice) > 0.001) {
            butterflyInfo.value.atm_price = midPrice;
            priceChanged = true;
          }
        } else if (Math.abs(strike - butterflyInfo.value.upper_strike) < 0.01) {
          if (Math.abs(butterflyInfo.value.upper_price - midPrice) > 0.001) {
            butterflyInfo.value.upper_price = midPrice;
            priceChanged = true;
          }
        }
      }

      // Only update chart if price actually changed significantly
      if (priceChanged) {
        throttledChartUpdate();
      }
    };

    const updateChart = async () => {
      console.log("updateChart called with:", {
        hasCanvas: !!chartCanvas.value,
        hasChartData: !!chartData.value,
        underlyingPrice: underlyingPrice.value,
        chartExists: !!chart.value,
      });

      if (
        !chartCanvas.value ||
        !chartData.value ||
        underlyingPrice.value === null
      ) {
        console.log("Early return from updateChart - missing required data");
        return;
      }

      // Wait a moment to ensure DOM is ready
      await nextTick();

      try {
        // If chart doesn't exist, create it
        if (!chart.value) {
          console.log("Creating new chart");
          const ctx = chartCanvas.value.getContext("2d");
          if (!ctx) {
            console.error("Canvas context not available");
            return;
          }

          const config = createChartConfig(
            chartData.value,
            underlyingPrice.value
          );
          chart.value = new Chart(chartCanvas.value, config);
          console.log("New chart created successfully");
        } else {
          // Update existing chart data
          console.log("Updating existing chart data");
          const { prices, payoffs } = chartData.value;

          // Create new data points
          const chartPoints = prices.map((price, index) => ({
            x: price,
            y: payoffs[index],
          }));

          const zeroLinePoints = prices.map((price) => ({
            x: price,
            y: 0,
          }));

          // Update the datasets
          chart.value.data.datasets[0].data = chartPoints;
          chart.value.data.datasets[1].data = zeroLinePoints;

          // Update the chart
          chart.value.update("none"); // 'none' disables animation for faster updates
          console.log("Chart data updated successfully");
        }
      } catch (error) {
        console.error("Error updating chart:", error);
        console.error("Error stack:", error.stack);

        // If update fails, try recreating the chart
        console.log("Attempting to recreate chart due to error");
        try {
          if (chart.value) {
            chart.value.destroy();
            chart.value = null;
          }

          const ctx = chartCanvas.value.getContext("2d");
          if (ctx) {
            const config = createChartConfig(
              chartData.value,
              underlyingPrice.value
            );
            chart.value = new Chart(chartCanvas.value, config);
            console.log("Chart recreated successfully after error");
          }
        } catch (recreateError) {
          console.error("Failed to recreate chart:", recreateError);
        }
      }
    };

    const generateOrderLegs = () => {
      if (!butterflyInfo.value) return [];

      const legs = [];

      if (strategyType.value === "IRON Butterfly") {
        const putLower = findOption(butterflyInfo.value.lower_strike, "put");
        const putAtm = findOption(butterflyInfo.value.atm_strike, "put");
        const callAtm = findOption(butterflyInfo.value.atm_strike, "call");
        const callUpper = findOption(butterflyInfo.value.upper_strike, "call");

        if (putLower && putAtm && callAtm && callUpper) {
          legs.push(
            {
              action: "Buy",
              symbol: symbol.value,
              date: formatDate(expiry.value),
              type: "Put",
              strike: `$${butterflyInfo.value.lower_strike.toFixed(2)}`,
              price: `$${butterflyInfo.value.lower_price.toFixed(2)}`,
            },
            {
              action: "Sell",
              symbol: symbol.value,
              date: formatDate(expiry.value),
              type: "Put",
              strike: `$${butterflyInfo.value.atm_strike.toFixed(2)}`,
              price: `$${butterflyInfo.value.atm_put_price.toFixed(2)}`,
            },
            {
              action: "Sell",
              symbol: symbol.value,
              date: formatDate(expiry.value),
              type: "Call",
              strike: `$${butterflyInfo.value.atm_strike.toFixed(2)}`,
              price: `$${butterflyInfo.value.atm_call_price.toFixed(2)}`,
            },
            {
              action: "Buy",
              symbol: symbol.value,
              date: formatDate(expiry.value),
              type: "Call",
              strike: `$${butterflyInfo.value.upper_strike.toFixed(2)}`,
              price: `$${butterflyInfo.value.upper_price.toFixed(2)}`,
            }
          );
        }
      } else {
        const type = strategyType.value === "CALL Butterfly" ? "Call" : "Put";
        legs.push(
          {
            action: "Buy",
            symbol: symbol.value,
            date: formatDate(expiry.value),
            type,
            strike: `$${butterflyInfo.value.lower_strike.toFixed(2)}`,
            price: `$${butterflyInfo.value.lower_price.toFixed(2)}`,
          },
          {
            action: "Sell",
            symbol: symbol.value,
            date: formatDate(expiry.value),
            type,
            strike: `$${butterflyInfo.value.atm_strike.toFixed(2)}`,
            price: `$${butterflyInfo.value.atm_price.toFixed(2)}`,
          },
          {
            action: "Buy",
            symbol: symbol.value,
            date: formatDate(expiry.value),
            type,
            strike: `$${butterflyInfo.value.upper_strike.toFixed(2)}`,
            price: `$${butterflyInfo.value.upper_price.toFixed(2)}`,
          }
        );
      }

      return legs;
    };

    const confirmOrder = async () => {
      if (!butterflyInfo.value || !orderDetails.value) return;

      placingOrder.value = true;
      try {
        // Build order payload similar to Streamlit app
        const orderPayload = {
          symbol: symbol.value,
          expiry: expiry.value.toISOString().split("T")[0],
          strategy_type: strategyType.value,
          legs: buildOrderLegsPayload(),
          order_price: orderDetails.value.orderPrice,
          order_offset: orderOffset.value,
          qty: 1,
          time_in_force: "day",
          order_type: "limit",
        };

        const result = await api.placeButterflyOrder(orderPayload);
        orderResult.value = result;
        showOrderResult.value = true;
        showOrderConfirmation.value = false;
      } catch (error) {
        console.error("Error placing order:", error);
        orderResult.value = { success: false, error: error.message };
        showOrderResult.value = true;
        showOrderConfirmation.value = false;
      } finally {
        placingOrder.value = false;
      }
    };

    const buildOrderLegsPayload = () => {
      if (!butterflyInfo.value) return [];

      const legs = [];

      if (strategyType.value === "IRON Butterfly") {
        const putLower = findOption(butterflyInfo.value.lower_strike, "put");
        const putAtm = findOption(butterflyInfo.value.atm_strike, "put");
        const callAtm = findOption(butterflyInfo.value.atm_strike, "call");
        const callUpper = findOption(butterflyInfo.value.upper_strike, "call");

        if (putLower && putAtm && callAtm && callUpper) {
          legs.push(
            { symbol: putLower.symbol, side: "buy", ratio_qty: "1" },
            { symbol: putAtm.symbol, side: "sell", ratio_qty: "1" },
            { symbol: callAtm.symbol, side: "sell", ratio_qty: "1" },
            { symbol: callUpper.symbol, side: "buy", ratio_qty: "1" }
          );
        }
      } else {
        const type = strategyType.value === "CALL Butterfly" ? "call" : "put";
        const lowerOption = findOption(butterflyInfo.value.lower_strike, type);
        const atmOption = findOption(butterflyInfo.value.atm_strike, type);
        const upperOption = findOption(butterflyInfo.value.upper_strike, type);

        if (lowerOption && atmOption && upperOption) {
          legs.push(
            { symbol: lowerOption.symbol, side: "buy", ratio_qty: "1" },
            { symbol: atmOption.symbol, side: "sell", ratio_qty: "2" },
            { symbol: upperOption.symbol, side: "buy", ratio_qty: "1" }
          );
        }
      }

      return legs;
    };

    const initializeOrderPrice = () => {
      // Set manual order price to the calculated price when dialog opens
      if (orderDetails.value) {
        manualOrderPrice.value = orderDetails.value.calculatedOrderPrice;
      }
    };

    const updateOrderCalculations = () => {
      // This will trigger the computed property to recalculate
      // The reactive system will handle the updates automatically
      console.log("Manual order price updated to:", manualOrderPrice.value);
    };

    const formatDate = (date) => {
      if (!date) return "";
      return date.toISOString().split("T")[0];
    };

    // Watchers
    watch([symbol, expiry, strategyType, legWidth], async () => {
      if (symbol.value && expiry.value) {
        await fetchUnderlyingPrice();
        await fetchOptionsChain();
      }
    });

    // Lifecycle hooks
    onMounted(async () => {
      console.log("TradeSetup mounted, starting initialization...");

      // Set default expiry to next market date
      try {
        const nextMarketDate = await api.getNextMarketDate();
        expiry.value = new Date(nextMarketDate);
        console.log("Set expiry date to:", nextMarketDate);
      } catch (error) {
        console.error("Error fetching next market date:", error);
      }

      // Initial data fetch
      console.log("Fetching initial data...");
      await fetchUnderlyingPrice();
      console.log("Underlying price after fetch:", underlyingPrice.value);
      await fetchOptionsChain();
      console.log(
        "Options chain length after fetch:",
        optionsChain.value.length
      );

      // Check service status after WebSocket connection is established
      await checkServiceStatus();
    });

    onUnmounted(() => {
      webSocketClient.disconnect();
      if (chart.value) {
        chart.value.destroy();
      }
    });

    return {
      // Reactive data
      serviceStatus,
      restartingService,
      expiry,
      strategyType,
      legWidth,
      orderOffset,
      symbol,
      underlyingPrice,
      isLivePrice,
      optionsChain,
      butterflyInfo,
      chartData,
      showOrderConfirmation,
      showOrderResult,
      placingOrder,
      orderResult,
      chartCanvas,
      manualOrderPrice,

      // Options
      strategyOptions,

      // Computed
      canPlaceOrder,
      orderDetails,
      legsTableData,

      // Methods
      checkServiceStatus,
      restartStreamingService,
      fetchUnderlyingPrice,
      fetchOptionsChain,
      confirmOrder,
      formatDate,
      initializeOrderPrice,
      updateOrderCalculations,
    };
  },
};
</script>

<style scoped>
.trade-setup-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
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

.config-card,
.options-info-card,
.legs-card,
.chart-card,
.order-card,
.footer-status {
  margin-bottom: 20px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 20px;
}

.field {
  display: flex;
  flex-direction: column;
}

.field label {
  font-weight: 600;
  margin-bottom: 8px;
  color: #2c3e50;
}

.field small {
  margin-top: 5px;
  color: #6c757d;
  font-size: 0.875rem;
}

.butterfly-section {
  margin: 30px 0;
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
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 10px;
}

.order-confirmation {
  padding: 20px 0;
}

.order-summary {
  margin-bottom: 20px;
}

.summary-item {
  margin-bottom: 10px;
  padding: 8px;
  background: #f8f9fa;
  border-radius: 4px;
}

.order-result-json {
  background: #f8f9fa;
  padding: 15px;
  border-radius: 6px;
  font-size: 0.875rem;
  max-height: 300px;
  overflow-y: auto;
}

.mt-3 {
  margin-top: 1rem;
}
</style>
