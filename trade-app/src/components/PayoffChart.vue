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
              calculatedInfo.breakEvenPoints && calculatedInfo.breakEvenPoints.length > 0
            "
          >
            <strong>Break-Even Points:</strong>
            <span>
              {{
                calculatedInfo.breakEvenPoints
                  .map((p) => "$" + p.toFixed(2))
                  .join(", ")
              }}
            </span>
          </div>
          <div v-if="calculatedInfo.maxProfit !== undefined">
            <strong>Max Profit:</strong>
            <span class="profit">
              {{ calculatedInfo.maxProfit === Infinity ? 'Unlimited' : '$' + calculatedInfo.maxProfit.toFixed(2) }}
            </span>
          </div>
          <div v-if="calculatedInfo.maxLoss !== undefined">
            <strong>Max Loss:</strong>
            <span class="loss">
              {{ calculatedInfo.maxLoss === Infinity ? 'Unlimited' : '$' + calculatedInfo.maxLoss.toFixed(2) }}
            </span>
          </div>
          <div v-if="calculatedInfo.currentUnrealizedPL !== undefined">
            <strong>Current Unrealized P&L:</strong>
            <span
              :class="calculatedInfo.currentUnrealizedPL >= 0 ? 'profit' : 'loss'"
            >
              ${{ calculatedInfo.currentUnrealizedPL.toFixed(2) }}
            </span>
          </div>
        </div>
      </div>
    </template>
  </Card>
</template>

<script>
import { ref, watch, nextTick, onMounted, onUnmounted, shallowRef } from "vue";
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
    const chart = shallowRef(null);
    const previousChartData = ref(null);

    // Enhanced state management for interaction tracking
    const chartState = ref({
      isPanning: false,
      isZooming: false,
      frozenUpdates: false,
      viewCenter: null, // Strike price at center of view
      viewRange: null, // Price range width
      lastInteraction: null,
    });

    const calculatedInfo = ref({
      maxProfit: undefined,
      maxLoss: undefined,
      breakEvenPoints: [],
      currentUnrealizedPL: undefined,
    });

    const pendingUpdate = ref(null);
    const updateDebounceTimer = ref(null);

    // Strike-based view state management
    const saveViewState = () => {
      if (chart.value && chart.value.scales && chart.value.scales.x) {
        try {
          const xScale = chart.value.scales.x;
          const min = xScale.min;
          const max = xScale.max;

          if (min !== undefined && max !== undefined) {
            chartState.value.viewCenter = (min + max) / 2;
            chartState.value.viewRange = max - min;
          }
        } catch (error) {
          console.warn("Could not save view state:", error);
        }
      }
    };

    const restoreViewState = () => {
      if (
        chart.value &&
        chartState.value.viewCenter !== null &&
        chartState.value.viewRange !== null
      ) {
        try {
          const halfRange = chartState.value.viewRange / 2;
          const min = chartState.value.viewCenter - halfRange;
          const max = chartState.value.viewCenter + halfRange;

          chart.value.zoomScale("x", { min, max }, "none");
        } catch (error) {
          console.warn("Could not restore view state:", error);
        }
      }
    };

    // Interaction event handlers
    const onPanStart = () => {
      chartState.value.isPanning = true;
      chartState.value.frozenUpdates = true;
      chartState.value.lastInteraction = Date.now();

      console.log("🔒 Chart updates frozen - user is panning");
    };

    const onPanComplete = () => {
      chartState.value.isPanning = false;
      saveViewState(); // Save the new position

      // Auto-resume updates after 1 second of inactivity
      setTimeout(() => {
        if (!chartState.value.isPanning && !chartState.value.isZooming) {
          chartState.value.frozenUpdates = false;
          console.log("🔓 Chart updates resumed - applying pending updates");

          // Apply any pending updates
          if (pendingUpdate.value) {
            applyPendingUpdate();
          }
        }
      }, 1000);
    };

    const onZoomStart = () => {
      chartState.value.isZooming = true;
      chartState.value.frozenUpdates = true;
      chartState.value.lastInteraction = Date.now();

      console.log("🔒 Chart updates frozen - user is zooming");
    };

    const onZoomComplete = () => {
      chartState.value.isZooming = false;
      saveViewState(); // Save the new zoom level

      // Auto-resume updates after 1 second of inactivity
      setTimeout(() => {
        if (!chartState.value.isPanning && !chartState.value.isZooming) {
          chartState.value.frozenUpdates = false;
          console.log("🔓 Chart updates resumed - applying pending updates");

          // Apply any pending updates
          if (pendingUpdate.value) {
            applyPendingUpdate();
          }
        }
      }, 1000);
    };

    // Apply pending updates when chart is unfrozen
    const applyPendingUpdate = async () => {
      if (pendingUpdate.value && !chartState.value.frozenUpdates) {
        const pendingType = pendingUpdate.value.type;
        pendingUpdate.value = null;

        console.log("📊 Applying pending chart update of type:", pendingType);

        if (pendingType === 'full') {
          await performChartUpdate();
        } else if (pendingType === 'minor') {
          await performMinorUpdate();
        } else if (pendingType === 'price') {
          minimalPriceUpdate();
        }
      }
    };

    // Check if chartData change is minor (only prices changed, structure same)
    const isMinorChartDataUpdate = (newData, prevData) => {
      if (!prevData || !newData.legs || !prevData.legs || newData.legs.length !== prevData.legs.length) {
        return false;
      }
      for (let i = 0; i < newData.legs.length; i++) {
        const newLeg = newData.legs[i];
        const prevLeg = prevData.legs[i];
        // Compare structural fields; adjust based on your leg properties (e.g., strike, type, position/side, quantity)
        if (
          newLeg.strike !== prevLeg.strike ||
          newLeg.type !== prevLeg.type ||
          newLeg.position !== prevLeg.position ||
          newLeg.quantity !== prevLeg.quantity
        ) {
          return false;
        }
      }
      return true;
    };

    // Debounced update functions
    const debouncedUpdate = (updateType) => {
      if (updateDebounceTimer.value) {
        clearTimeout(updateDebounceTimer.value);
      }

      updateDebounceTimer.value = setTimeout(() => {
        if (chartState.value.frozenUpdates) {
          // Store as pending update
          pendingUpdate.value = { type: updateType };
          console.log(`⏸️ ${updateType} chart update queued - user is interacting`);
        } else {
          if (updateType === 'full') {
            performChartUpdate();
          } else if (updateType === 'minor') {
            performMinorUpdate();
          } else if (updateType === 'price') {
            minimalPriceUpdate();
          }
        }
      }, 200); // 200ms debounce
    };

    // Minimal update for price-only changes (update annotations only)
    const minimalPriceUpdate = () => {
      if (!chart.value || props.underlyingPrice === null || props.underlyingPrice === undefined) return;

      try {
        // Regenerate only the annotations part with new price
        const config = createMultiLegChartConfig(props.chartData, props.underlyingPrice);
        if (config.options?.plugins?.annotation?.annotations) {
          chart.value.options.plugins.annotation.annotations = config.options.plugins.annotation.annotations;
        }
        chart.value.update("none");
        computeFromChart();
      } catch (error) {
        console.warn("Minimal price update failed, falling back to full update:", error);
        performChartUpdate();
      }
    };

    const computeFromChart = () => {
      if (!chart.value) return;

      const datasets = chart.value.data.datasets;
      const positionDataset = datasets.find((d) => d.label.startsWith("Position Payoff"));

      if (positionDataset) {
        let labels = chart.value.data.labels;
        let payoffs = positionDataset.data;

        // Ensure labels are numbers
        labels = labels.map(Number);

        // Compute maxProfit and maxLoss from data points
        let maxP = Number.NEGATIVE_INFINITY;
        let minL = Number.POSITIVE_INFINITY;
        for (const y of payoffs) {
          if (y > maxP) maxP = y;
          if (y < minL) minL = y;
        }

        let tempMaxProfit = maxP > 0 ? maxP : 0;
        let tempMaxLoss = minL < 0 ? -minL : 0;

        // Detect unlimited profit/loss based on end slopes
        if (labels.length >= 2) {
          const leftSlope =
            (payoffs[1] - payoffs[0]) / (labels[1] - labels[0]);
          const rightSlope =
            (payoffs[payoffs.length - 1] - payoffs[payoffs.length - 2]) /
            (labels[labels.length - 1] - labels[labels.length - 2]);

          if (leftSlope > 0) {
            tempMaxProfit = Infinity;
          } else if (leftSlope < 0) {
            tempMaxLoss = Infinity;
          }

          if (rightSlope > 0) {
            tempMaxProfit = Infinity;
          } else if (rightSlope < 0) {
            tempMaxLoss = Infinity;
          }
        }

        calculatedInfo.value.maxProfit = tempMaxProfit;
        calculatedInfo.value.maxLoss = tempMaxLoss;

        // Compute break-even points
        const bePoints = [];
        for (let i = 1; i < payoffs.length; i++) {
          const y1 = payoffs[i - 1];
          const y2 = payoffs[i];
          if (y1 * y2 <= 0) {
            let p;
            if (y1 === 0) {
              p = labels[i - 1];
            } else if (y2 === 0) {
              p = labels[i];
            } else {
              const ratio = -y1 / (y2 - y1);
              p = labels[i - 1] + ratio * (labels[i] - labels[i - 1]);
            }
            bePoints.push(p);
          }
        }
        calculatedInfo.value.breakEvenPoints = bePoints.sort((a, b) => a - b);

        // Compute current unrealized P&L with interpolation/extrapolation
        const currX = props.underlyingPrice;
        let currY;
        if (currX <= labels[0]) {
          const slopeLeft = (payoffs[1] - payoffs[0]) / (labels[1] - labels[0]);
          currY = payoffs[0] + slopeLeft * (currX - labels[0]);
        } else if (currX >= labels[labels.length - 1]) {
          const slopeRight =
            (payoffs[payoffs.length - 1] - payoffs[payoffs.length - 2]) /
            (labels[labels.length - 1] - labels[labels.length - 2]);
          currY =
            payoffs[payoffs.length - 1] + slopeRight * (currX - labels[labels.length - 1]);
        } else {
          for (let i = 1; i < labels.length; i++) {
            if (labels[i - 1] <= currX && currX <= labels[i]) {
              const ratio = (currX - labels[i - 1]) / (labels[i] - labels[i - 1]);
              currY = payoffs[i - 1] + ratio * (payoffs[i] - payoffs[i - 1]);
              break;
            }
          }
        }
        calculatedInfo.value.currentUnrealizedPL = currY;
      }
    };

    // Unified full chart update function (for major chartData changes or initial)
    const performChartUpdate = async () => {
      if (
        !chartCanvas.value ||
        !props.chartData ||
        props.underlyingPrice === null ||
        props.underlyingPrice === undefined
      ) {
        return;
      }

      try {
        if (!chart.value) {
          // Create new chart if it doesn't exist
          await createChart();
        } else {
          // Update existing chart data
          await updateChartData();
        }

        await nextTick();

        // Restore view state or reset if no saved state
        if (
          chartState.value.viewCenter === null &&
          chartState.value.viewRange === null
        ) {
          chart.value.resetZoom();
          console.log("🔄 Resetting zoom to apply new scale suggestions");
        } else {
          restoreViewState();
        }

        computeFromChart();
      } catch (error) {
        console.error("Error in performChartUpdate:", error);
        // Fallback to chart recreation
        await createChart();
      }
    };

    // Minor update for chartData changes (mutate data in place for smoothness)
    const performMinorUpdate = async () => {
      if (
        !chart.value ||
        !props.chartData ||
        props.underlyingPrice === null ||
        props.underlyingPrice === undefined
      ) {
        return;
      }

      try {
        saveViewState(); // Save current view

        const config = createMultiLegChartConfig(props.chartData, props.underlyingPrice);

        if (config.data.labels.length !== chart.value.data.labels.length) {
          console.warn("Labels length changed, falling back to full update");
          await performChartUpdate();
          return;
        }

        // Mutate existing datasets data in place
        config.data.datasets.forEach((newDs, j) => {
          const ds = chart.value.data.datasets[j];
          if (ds && ds.label === newDs.label) { // Match by label
            newDs.data.forEach((y, k) => {
              ds.data[k] = y;
            });
          }
        });

        // Update annotations
        if (config.options?.plugins?.annotation?.annotations) {
          chart.value.options.plugins.annotation.annotations = config.options.plugins.annotation.annotations;
        }

        // Update y-scale options
        if (config.options.scales?.y) {
          chart.value.options.scales.y = { ...config.options.scales.y };
        }

        // Restore view state (using zoomScale)
        restoreViewState();

        // No need for additional update, as zoomScale handles it

        await nextTick();

        computeFromChart();
      } catch (error) {
        console.error("Error in performMinorUpdate:", error);
        // Fallback to full update
        await performChartUpdate();
      }
    };

    const createChart = async () => {
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
        // Only destroy if chart exists
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

        // Add interaction callbacks to the config
        if (config && config.options && config.options.plugins && config.options.plugins.zoom) {
          config.options.plugins.zoom.pan = {
            ...config.options.plugins.zoom.pan,
            onPanStart,
            onPanComplete,
          };
          config.options.plugins.zoom.zoom = {
            ...config.options.plugins.zoom.zoom,
            onZoomStart,
            onZoomComplete,
          };
        }

        if (ctx && config) {
          chart.value = new Chart(chartCanvas.value, config);
          // Restore view state after chart is created
          await nextTick();
          if (
            chartState.value.viewCenter === null &&
            chartState.value.viewRange === null
          ) {
            chart.value.resetZoom();
            console.log("🔄 Initial reset zoom for new chart");
          } else {
            restoreViewState();
          }
          computeFromChart();
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
        console.error("PayoffChart: Error creating chart:", err);
        console.error("PayoffChart: Error stack:", err.stack);
      }
    };

    const updateChartData = async () => {
      if (!chart.value || !props.chartData || props.underlyingPrice === null) {
        return;
      }

      try {
        // Update existing chart with full config (for major changes)
        const config = createMultiLegChartConfig(
          props.chartData,
          props.underlyingPrice
        );

        if (config && config.data) {
          // Update labels and datasets
          chart.value.data.labels = config.data.labels;
          chart.value.data.datasets = config.data.datasets;

          // Update the vertical line annotation for current price
          if (config.options?.plugins?.annotation?.annotations) {
            chart.value.options.plugins.annotation.annotations =
              config.options.plugins.annotation.annotations;
          }

          // Update y-scale options to adjust range based on new data
          if (config.options.scales?.y) {
            chart.value.options.scales.y = { ...config.options.scales.y };
          }

          // Update x-scale suggestedMin/Max if present (for resetZoom to use new values)
          if (config.options.scales?.x) {
            chart.value.options.scales.x.suggestedMin =
              config.options.scales.x.suggestedMin;
            chart.value.options.scales.x.suggestedMax =
              config.options.scales.x.suggestedMax;
          }

          // Update the chart without animation
          chart.value.update("none");
        }
      } catch (err) {
        console.error("PayoffChart: Error updating chart data:", err);
        // If update fails, recreate the chart
        await createChart();
      }
    };

    // Separate watches for better control
    watch(
      () => props.chartData,
      (newChartData) => {
        if (newChartData) {
          const prev = previousChartData.value;
          const isMinor = isMinorChartDataUpdate(newChartData, prev);

          let updateType = 'full';

          if (isMinor) {
            updateType = 'minor';
            console.log("📈 Minor chartData update (prices only) - preserving view");
          } else {
            // Reset view state on major changes (legs change)
            chartState.value.viewCenter = null;
            chartState.value.viewRange = null;
            console.log("🔄 Resetting view state due to major chartData change");
          }

          if (props.underlyingPrice !== null) {
            debouncedUpdate(updateType);
          }

          // Update previous after handling
          previousChartData.value = JSON.parse(JSON.stringify(newChartData)); // Deep copy
        } else {
          if (chart.value) {
            chart.value.destroy();
            chart.value = null;
          }
          previousChartData.value = null;
        }
      },
      { deep: true, immediate: true }
    );

    watch(
      () => props.underlyingPrice,
      (newUnderlyingPrice) => {
        if (props.chartData && newUnderlyingPrice !== null) {
          debouncedUpdate('price');
        }
      },
      { immediate: true }
    );

    // Initial chart creation
    onMounted(async () => {
      if (props.chartData && props.underlyingPrice !== null) {
        await nextTick();
        await createChart();
        previousChartData.value = JSON.parse(JSON.stringify(props.chartData));
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
        // Clear saved view state when user explicitly resets
        chartState.value.viewCenter = null;
        chartState.value.viewRange = null;
        console.log("🔄 View state cleared - chart reset to default view");
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
      chartState,
      calculatedInfo,
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