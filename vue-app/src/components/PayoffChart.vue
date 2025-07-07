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
import { createMultiLegChartConfig } from "../utils/chartUtils";

Chart.register(...registerables);

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
        props.underlyingPrice === null
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
          console.log("PayoffChart: Chart created successfully");
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

    // Cleanup
    onUnmounted(() => {
      if (chart.value) {
        chart.value.destroy();
        chart.value = null;
      }
    });

    return {
      chartCanvas,
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
