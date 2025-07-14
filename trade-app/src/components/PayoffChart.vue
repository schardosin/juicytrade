<template>
  <Card class="payoff-chart-card">
    <template #title>
      {{ title }}
      <span v-if="adjustmentIndicator" class="adjustment-indicator">
        {{ adjustmentIndicator }}
      </span>
    </template>
    <template #content>
      <div class="chart-container" :style="{ height: height }">
        <canvas ref="chartCanvas"></canvas>
      </div>
      <div class="chart-controls">
        <small class="chart-instructions">
          💡 <strong>Interactive Chart:</strong> Mouse wheel to zoom
          horizontally, drag to pan left/right, double-click to reset
        </small>
        <button
          class="reset-zoom-btn"
          @click="resetZoom"
          title="Reset zoom to fit all data"
        >
          🔍 Reset
        </button>
      </div>
      <div v-if="showInfo && chartData" class="chart-info mt-3">
        <div class="info-grid">
          <div v-if="underlyingPrice !== null">
            <strong
              >{{ symbol }} Price {{ isLivePrice ? "(Live)" : "" }}:</strong
            >
            <span>${{ underlyingPrice.toFixed(2) }}</span>
          </div>
          <div
            v-if="
              chartData.breakEvenPoints && chartData.breakEvenPoints.length > 0
            "
          >
            <strong>Break-Even Points:</strong>
            <span>
              {{
                chartData.breakEvenPoints
                  .map((p) => "$" + p.toFixed(2))
                  .join(", ")
              }}
            </span>
          </div>
          <div v-if="chartData.maxProfit !== undefined">
            <strong>Max Profit:</strong>
            <span class="profit">${{ chartData.maxProfit.toFixed(2) }}</span>
          </div>
          <div v-if="chartData.maxLoss !== undefined">
            <strong>Max Loss:</strong>
            <span class="loss">${{ chartData.maxLoss.toFixed(2) }}</span>
          </div>
          <div v-if="chartData.currentUnrealizedPL !== undefined">
            <strong>Current Unrealized P&L:</strong>
            <span
              :class="chartData.currentUnrealizedPL >= 0 ? 'profit' : 'loss'"
            >
              ${{ chartData.currentUnrealizedPL.toFixed(2) }}
            </span>
          </div>
        </div>
      </div>
    </template>
  </Card>
</template>

<script>
import { ref, watch, nextTick, onMounted, onUnmounted } from "vue";
import { Chart, registerables } from "chart.js";
import zoomPlugin from "chartjs-plugin-zoom";
import { createMultiLegChartConfig } from "../utils/chartUtils";

Chart.register(...registerables, zoomPlugin);

export default {
  name: "PayoffChart",
  props: {
    chartData: {
      type: Object,
      required: true,
    },
    underlyingPrice: {
      type: Number,
      required: false,
      default: 0,
    },
    title: {
      type: String,
      default: "Payoff Diagram",
    },
    showInfo: {
      type: Boolean,
      default: false,
    },
    height: {
      type: String,
      default: "400px",
    },
    adjustmentIndicator: {
      type: String,
      default: "",
    },
    symbol: {
      type: String,
      default: "SPY",
    },
    isLivePrice: {
      type: Boolean,
      default: false,
    },
  },
  setup(props) {
    const chartCanvas = ref(null);
    const chart = ref(null);

    const updateChart = async () => {
      if (
        !chartCanvas.value ||
        !props.chartData ||
        props.underlyingPrice === null ||
        props.underlyingPrice === undefined
      ) {
        console.warn(
          "Chart not rendered: missing canvas, chartData, or underlyingPrice",
          {
            hasCanvas: !!chartCanvas.value,
            hasChartData: !!props.chartData,
            underlyingPrice: props.underlyingPrice,
          }
        );
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
          props.chartData,
          props.underlyingPrice
        );

        if (ctx && config) {
          chart.value = new Chart(chartCanvas.value, config);
        } else {
          console.error(
            "PayoffChart: Chart.js not created - missing ctx or config",
            {
              ctx,
              config,
            }
          );
        }
      } catch (err) {
        console.error("PayoffChart: Error updating chart:", err);
      }
    };

    // Watch for changes in chart data or underlying price
    watch(
      [() => props.chartData, () => props.underlyingPrice],
      async ([newChartData, newUnderlyingPrice]) => {
        if (newChartData && newUnderlyingPrice !== null) {
          await nextTick();
          updateChart();
        }
      },
      { deep: true }
    );

    // Initial chart creation
    onMounted(async () => {
      if (props.chartData && props.underlyingPrice !== null) {
        await nextTick();
        updateChart();
      }
    });

    // Reset zoom function
    const resetZoom = () => {
      if (chart.value) {
        chart.value.resetZoom();
      }
    };

    // Cleanup
    onUnmounted(() => {
      if (chart.value) {
        chart.value.destroy();
        chart.value = null;
      }
    });

    return {
      chartCanvas,
      resetZoom,
    };
  },
};
</script>

<style scoped>
.payoff-chart-card {
  margin-bottom: 0px;
}

.chart-container {
  position: relative;
}

.chart-controls {
  margin-top: 8px;
  padding: 8px 12px;
  background: var(--bg-tertiary, rgba(55, 55, 55, 0.8));
  border-radius: 6px;
  border: 1px solid var(--border-secondary, rgba(255, 255, 255, 0.1));
  display: flex;
  flex-direction: row;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.chart-instructions {
  color: var(--text-secondary, #b0b0b0);
  font-size: 11px;
  line-height: 1.2;
  display: block;
  margin: 0;
}

.reset-zoom-btn {
  background: var(--color-primary, #007bff);
  color: var(--text-primary, white);
  border: none;
  padding: 6px 10px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.reset-zoom-btn:hover {
  background: var(--color-primary-dark, #0056b3);
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
}

.reset-zoom-btn:active {
  background: var(--color-primary-darker, #004085);
  transform: translateY(0);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
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

.adjustment-indicator {
  color: #007bff;
  font-weight: normal;
  font-size: 0.9em;
  margin-left: 8px;
}

.profit {
  color: #28a745;
  font-weight: 600;
}

.loss {
  color: #dc3545;
  font-weight: 600;
}

.mt-3 {
  margin-top: 1rem;
}
</style>
