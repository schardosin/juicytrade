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
          💡 <strong>Interactive Chart:</strong> Use +/- buttons to zoom, drag
          to pan left/right
        </small>
        <div class="chart-buttons">
          <button
            class="zoom-btn zoom-out-btn"
            @click="zoomOut"
            title="Zoom out to see wider price range"
          >
            −
          </button>
          <button
            class="zoom-btn zoom-in-btn"
            @click="zoomIn"
            title="Zoom in for more detail"
          >
            +
          </button>
          <button
            class="zoom-btn reset-zoom-btn"
            @click="resetZoom"
            title="Reset zoom to fit all data"
          >
            ⟲
          </button>
        </div>
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
    const savedZoomState = ref(null);

    const saveZoomState = () => {
      if (chart.value && chart.value.scales) {
        try {
          const xScale = chart.value.scales.x;
          if (xScale) {
            savedZoomState.value = {
              min: xScale.min,
              max: xScale.max,
            };
          }
        } catch (error) {
          console.warn("Could not save zoom state:", error);
        }
      }
    };

    const restoreZoomState = () => {
      if (chart.value && savedZoomState.value) {
        try {
          chart.value.zoomScale(
            "x",
            {
              min: savedZoomState.value.min,
              max: savedZoomState.value.max,
            },
            "none"
          );
        } catch (error) {
          console.warn("Could not restore zoom state:", error);
        }
      }
    };

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
        // Save zoom state before destroying chart
        if (chart.value) {
          saveZoomState();
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

          // Restore zoom state after chart is created
          await nextTick();
          restoreZoomState();
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

    // Zoom functions
    const zoomIn = () => {
      if (chart.value) {
        chart.value.zoom(1.5); // Zoom in by 50%
      }
    };

    const zoomOut = () => {
      if (chart.value) {
        chart.value.zoom(0.75); // Zoom out by 25%
      }
    };

    // Reset zoom function
    const resetZoom = () => {
      if (chart.value) {
        chart.value.resetZoom();
        // Clear saved zoom state when user explicitly resets
        savedZoomState.value = null;
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
      zoomIn,
      zoomOut,
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

.chart-buttons {
  display: flex;
  gap: 6px;
  align-items: center;
}

.zoom-btn {
  background: var(--bg-secondary, rgba(70, 70, 70, 0.9));
  color: var(--text-primary, white);
  border: 1px solid var(--border-secondary, rgba(255, 255, 255, 0.2));
  width: 28px;
  height: 28px;
  border-radius: 4px;
  font-size: 16px;
  font-weight: bold;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.zoom-btn:hover {
  background: var(--bg-primary, rgba(90, 90, 90, 0.9));
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
}

.zoom-btn:active {
  background: var(--bg-primary-dark, rgba(50, 50, 50, 0.9));
  transform: translateY(0);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}

.zoom-in-btn {
  font-size: 14px;
}

.zoom-out-btn {
  font-size: 18px;
}

.reset-zoom-btn {
  font-size: 14px;
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
