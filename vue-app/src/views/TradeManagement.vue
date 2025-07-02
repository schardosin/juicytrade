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
import streamingClient from "../services/streamingClient";
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

    // Methods
    const fetchPositions = async () => {
      loading.value = true;
      error.value = null;

      try {
        const response = await api.getPositions();

        if (response.success) {
          positions.value = response.positions || [];

          // If we have option positions, fetch underlying price and generate chart
          if (hasOptionPositions.value) {
            await fetchUnderlyingPrice();
            generateChart();
          }
        } else {
          error.value = response.error || "Failed to fetch positions";
        }
      } catch (err) {
        console.error("Error fetching positions:", err);
        error.value = err.message || "Failed to fetch positions";
      } finally {
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
        console.log("Starting streaming for positions...");
        isStreaming.value = true;

        // Subscribe to underlying symbol (stock)
        if (underlyingSymbol.value) {
          console.log("Subscribing to underlying:", underlyingSymbol.value);
          streamingClient.subscribeStock(
            underlyingSymbol.value,
            (symbol, priceData) => {
              console.log("Stock price update received:", symbol, priceData);
              const price = priceData.ask || priceData.bid || priceData.last;
              if (price) {
                streamingPrices.value[symbol] = price;
                underlyingPrice.value = price;
                updatePositionPrices();
              }
            }
          );
        }

        // Subscribe to option symbols
        const optionSymbols = positions.value
          .filter((pos) => pos.asset_class === "us_option")
          .map((pos) => pos.symbol);

        optionSymbols.forEach((symbol) => {
          console.log("Subscribing to option:", symbol);
          streamingClient.subscribeOption(symbol, (symbolName, priceData) => {
            console.log("Option price update received:", symbolName, priceData);
            const price = priceData.ask || priceData.bid || priceData.last;
            if (price) {
              streamingPrices.value[symbolName] = price;
              updatePositionPrices();
            }
          });
        });

        console.log("Streaming subscriptions set up for:", {
          underlying: underlyingSymbol.value,
          options: optionSymbols,
        });
      } catch (err) {
        console.error("Error starting streaming:", err);
        isStreaming.value = false;
      }
    };

    const stopStreaming = async () => {
      if (!isStreaming.value) {
        return;
      }

      try {
        console.log("Stopping streaming...");

        // Unsubscribe from underlying
        if (underlyingSymbol.value) {
          streamingClient.unsubscribeStock(underlyingSymbol.value);
        }

        // Unsubscribe from options
        positions.value
          .filter((pos) => pos.asset_class === "us_option")
          .forEach((pos) => {
            streamingClient.unsubscribeOption(pos.symbol);
          });

        streamingClient.stopPolling();
        isStreaming.value = false;
        streamingPrices.value = {};
      } catch (err) {
        console.error("Error stopping streaming:", err);
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

      // Computed
      optionPositions,
      hasOptionPositions,
      underlyingSymbol,
      totalUnrealizedPL,
      totalMarketValue,

      // Methods
      fetchPositions,
      formatDate,
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
</style>
