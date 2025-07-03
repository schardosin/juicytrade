<template>
  <div class="trade-management-container">
    <div class="header">
      <h1>Trade Management</h1>
      <h2>Step 2: Monitor and Adjust Active Trades</h2>
    </div>

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

    <!-- Active Positions -->
    <div v-else class="positions-section">
      <!-- Position Summary -->
      <Card class="summary-card">
        <template #title>Position Summary</template>
        <template #content>
          <div class="summary-grid">
            <div class="summary-item">
              <strong>Total Positions:</strong> {{ positions.length }}
            </div>
            <div class="summary-item">
              <strong>Option Positions:</strong> {{ optionPositions.length }}
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
          </div>
        </template>
      </Card>

      <!-- Positions Table -->
      <Card class="positions-table-card">
        <template #title>Active Positions</template>
        <template #content>
          <DataTable :value="positions" class="p-datatable-sm" stripedRows>
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
            <Column field="market_value" header="Market Value" :sortable="true">
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
                  :class="slotProps.data.unrealized_pl >= 0 ? 'profit' : 'loss'"
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

      <!-- Options Chain (only show if we have option positions) -->
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
                :class="[
                  'option-strike-row',
                  { 'position-strike': isPositionStrike(strike) },
                ]"
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
                    },
                  ]"
                >
                  <div class="symbol">{{ getCallOption(strike).symbol }}</div>
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
                        getSelectionType(getPutOption(strike).symbol) === 'buy',
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
                  <div class="symbol">{{ getPutOption(strike).symbol }}</div>
                </div>
                <div v-else class="put-side empty">
                  <div class="bid">-</div>
                  <div class="ask">-</div>
                  <div class="symbol">-</div>
                </div>
              </div>
            </div>
          </div>

          <!-- Selected Options Summary -->
          <div v-if="selectedOptions.length > 0" class="selected-summary mt-3">
            <h5>Selected Options ({{ selectedOptions.length }}/4):</h5>
            <div class="selected-list">
              <span
                v-for="symbol in selectedOptions"
                :key="symbol"
                class="selected-option"
                @click="removeSelection(symbol)"
              >
                {{ symbol }} ✕
              </span>
            </div>
          </div>
        </template>
      </Card>

      <!-- Payoff Chart (only show if we have option positions) -->
      <Card v-if="hasOptionPositions && chartData" class="chart-card">
        <template #title>Position Payoff Diagram</template>
        <template #content>
          <div class="chart-container">
            <canvas ref="chartCanvas"></canvas>
          </div>
          <div class="chart-info mt-3">
            <div class="info-grid">
              <div>
                <strong>Current {{ underlyingSymbol }} Price:</strong>
                ${{ underlyingPrice.toFixed(2) }}
              </div>
              <div>
                <strong>Break-Even Points:</strong>
                <span v-if="chartData.breakEvenPoints.length > 0">
                  {{
                    chartData.breakEvenPoints
                      .map((p) => "$" + p.toFixed(2))
                      .join(", ")
                  }}
                </span>
                <span v-else>None calculated</span>
              </div>
              <div>
                <strong>Max Profit:</strong>
                <span class="profit"
                  >${{ chartData.maxProfit.toFixed(2) }}</span
                >
              </div>
              <div>
                <strong>Max Loss:</strong>
                <span class="loss">${{ chartData.maxLoss.toFixed(2) }}</span>
              </div>
              <div>
                <strong>Current Unrealized P&L:</strong>
                <span
                  :class="
                    chartData.currentUnrealizedPL >= 0 ? 'profit' : 'loss'
                  "
                >
                  ${{ chartData.currentUnrealizedPL.toFixed(2) }}
                </span>
              </div>
            </div>
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from "vue";
import { Chart, registerables } from "chart.js";
import Tag from "primevue/tag";
import api from "../services/api";
import webSocketClient from "../services/webSocketClient";
import {
  generateMultiLegPayoff,
  createMultiLegChartConfig,
} from "../utils/chartUtils";

Chart.register(...registerables);

export default {
  name: "TradeManagement",
  components: {
    Tag,
  },
  setup() {
    // Reactive data
    const loading = ref(true);
    const error = ref(null);
    const positions = ref([]);
    const underlyingPrice = ref(620); // Default SPY price, will be updated
    const chartData = ref(null);
    const chartCanvas = ref(null);
    const chart = ref(null);
    const streamingPrices = ref({});
    const isStreaming = ref(false);
    const optionsChain = ref([]);
    const selectedOptions = ref([]);
    const selectedOptionsMap = ref({}); // Track selection type: { symbol: 'buy'|'sell' }

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
          console.log("Positions received via WebSocket:", positionsData);

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

        // Request positions via WebSocket
        webSocketClient.requestPositions();
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
        // Generate payoff data for all positions
        const payoffData = generateMultiLegPayoff(
          positions.value,
          underlyingPrice.value
        );

        if (payoffData) {
          chartData.value = payoffData;
        }
      } catch (err) {
        console.error("Error generating chart:", err);
      }
    };

    const updateChart = async () => {
      if (!chartCanvas.value || !chartData.value) {
        console.warn("Chart not rendered: missing canvas or chartData", {
          hasCanvas: !!chartCanvas.value,
          hasChartData: !!chartData.value,
        });
        return;
      }

      try {
        // Destroy existing chart
        if (chart.value) {
          chart.value.destroy();
          chart.value = null;
        }

        // Create new chart
        const ctx = chartCanvas.value.getContext("2d");
        const config = createMultiLegChartConfig(
          chartData.value,
          underlyingPrice.value
        );
        console.log(
          "Creating Chart.js chart with config:",
          config,
          "Canvas:",
          chartCanvas.value
        );
        if (ctx && config) {
          chart.value = new Chart(chartCanvas.value, config);
        } else {
          console.error("Chart.js not created: missing ctx or config", {
            ctx,
            config,
          });
        }
      } catch (err) {
        console.error("Error updating chart:", err);
      }
    };

    const formatDate = (dateString) => {
      if (!dateString) return "";
      const date = new Date(dateString);
      return date.toLocaleDateString();
    };

    // Streaming methods
    const startStreaming = async () => {
      if (isStreaming.value || !positions.value.length) {
        return;
      }

      try {
        console.log("Starting WebSocket streaming for positions...");
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

        console.log("Subscribing to symbols:", symbols);

        // Subscribe to all symbols
        webSocketClient.subscribe(symbols);

        // Set up price update handler
        webSocketClient.onPriceUpdate((data) => {
          console.log("WebSocket price update received:", data);
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
          console.log("WebSocket subscription confirmed:", message);
        });

        console.log("WebSocket streaming setup complete");
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

        console.log("Auto-subscribing to position symbols:", symbols);

        // Subscribe to all symbols
        webSocketClient.subscribe(symbols);

        // Set up price update handler
        webSocketClient.onPriceUpdate((data) => {
          console.log("Real-time price update:", data.symbol, data.price);
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
          console.log("Position symbols subscription confirmed:", message);
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

          // Recalculate market value
          pos.market_value =
            newPrice *
            Math.abs(pos.qty) *
            (pos.asset_class === "us_option" ? 100 : 1);

          // Recalculate unrealized P&L
          const costBasis =
            pos.cost_basis ||
            pos.avg_entry_price *
              Math.abs(pos.qty) *
              (pos.asset_class === "us_option" ? 100 : 1);
          pos.unrealized_pl = pos.market_value - costBasis;
          pos.unrealized_plpc =
            costBasis !== 0 ? pos.unrealized_pl / Math.abs(costBasis) : 0;
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
        console.log(
          "Fetching options chain for:",
          underlyingSymbol.value,
          positionExpiry.value
        );
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

          console.log(
            "Options chain loaded:",
            optionsChain.value.length,
            "options"
          );

          // Subscribe to options chain symbols for real-time prices
          const chainSymbols = optionsChain.value
            .filter(
              (option) =>
                option.strike_price >= strikeRange.value.min &&
                option.strike_price <= strikeRange.value.max
            )
            .map((option) => option.symbol);

          if (chainSymbols.length > 0) {
            console.log(
              "Subscribing to options chain symbols:",
              chainSymbols.length
            );
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

    // Lifecycle hooks
    onMounted(async () => {
      await fetchPositions();

      // Start streaming after positions are loaded
      if (positions.value.length > 0) {
        await startStreaming();
      }
    });

    // Watch for both chartData and chartCanvas to be ready, then update chart
    watch([chartData, chartCanvas], async ([newChartData, newChartCanvas]) => {
      if (newChartData && newChartCanvas) {
        await nextTick();
        updateChart();
      }
    });

    // Watch for underlying price changes to update chart
    watch(underlyingPrice, () => {
      if (hasOptionPositions.value && chartData.value) {
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
      if (chart.value) {
        chart.value.destroy();
      }
      await stopStreaming();
    });

    return {
      // Reactive data
      loading,
      error,
      positions,
      underlyingPrice,
      chartData,
      chartCanvas,
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
      isPositionStrike,
      getOptionDisplayPrice,
      toggleOptionSelection,
      removeSelection,
      selectOption,
      getSelectionType,
      getCallOption,
      getPutOption,
      getOptionBid,
      getOptionAsk,
    };
  },
};
</script>

<style scoped>
.trade-management-container {
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

.loading-card,
.error-card,
.no-positions-card,
.summary-card,
.positions-table-card,
.options-chain-card,
.chart-card {
  margin-bottom: 20px;
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
  background-color: #ffebee;
  border-left: 4px solid #f44336;
}

.call-side.selected-buy,
.put-side.selected-buy {
  background-color: #e8f5e8;
  border-left: 4px solid #4caf50;
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
}
</style>
