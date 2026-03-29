<template>
  <div class="right-panel-chart-container">
    <div ref="chartContainer" class="chart-container">
      <!-- Range Switcher Buttons -->
      <div class="range-switcher">
        <button
          v-for="range in ranges"
          :key="range.value"
          :class="['range-btn', { active: selectedRange === range.value }]"
          @click="setChartRange(range.value)"
        >
          {{ range.label }}
        </button>
      </div>
      
      <!-- Loading/Error States -->
      <div v-if="loading" class="chart-overlay">
        <div class="loading-spinner">Loading...</div>
      </div>
      <div v-if="error" class="chart-overlay error">
        <div class="error-message">{{ error }}</div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted, watch, nextTick } from "vue";
import {
  createChart,
  ColorType,
  AreaSeries,
} from "lightweight-charts";
import { useMarketData } from "../composables/useMarketData.js";

export default {
  name: "RightPanelChart",
  props: {
    symbol: {
      type: String,
      required: true,
    },
    currentPrice: {
      type: Number,
      default: 0,
    },
    height: {
      type: Number,
      default: 220,
    },
    livePrice: {
      type: Object,
      default: null,
    },
    enableRealtime: {
      type: Boolean,
      default: true,
    },
  },
  setup(props) {
    const chartContainer = ref(null);
    const loading = ref(false);
    const error = ref("");
    const selectedRange = ref("1D");

    // Use unified market data composable
    const { getHistoricalData, getHistoricalDailySixMonths } = useMarketData();

    let chart = null;
    let areaSeries = null;
    let chartData = new Map(); // Cache data for each range
    let currentDataPoint = null; // Track the current data point being updated

    const ranges = [
      { label: "1D", value: "1D" },
      { label: "1W", value: "1W" },
      { label: "1M", value: "1M" },
    ];

    // Fixed color for all ranges (blue like in the reference image)
    const chartColor = "#2962FF";

    const chartOptions = {
      layout: {
        background: {
          type: ColorType.Solid,
          color: "#0b0d10", // Match right panel background
        },
        textColor: "#d1d4dc",
        attributionLogo: false,
      },
      grid: {
        vertLines: {
          color: "transparent", // Clean look
        },
        horzLines: {
          color: "transparent", // Remove horizontal lines as requested
        },
      },
      crosshair: {
        mode: 1,
        vertLine: {
          color: "rgba(255, 255, 255, 0.3)",
          width: 1,
          style: 2, // Dashed
        },
        horzLine: {
          color: "rgba(255, 255, 255, 0.3)",
          width: 1,
          style: 2, // Dashed
        },
      },
      rightPriceScale: {
        borderColor: "rgba(255, 255, 255, 0.2)",
        textColor: "#d1d4dc",
      },
      timeScale: {
        borderColor: "rgba(255, 255, 255, 0.2)",
        textColor: "#d1d4dc",
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: {
        mouseWheel: false, // Disable scroll in compact view
        pressedMouseMove: false,
      },
      handleScale: {
        axisPressedMouseMove: false,
        mouseWheel: false, // Disable zoom in compact view
        pinch: false,
      },
    };

    const initializeChart = () => {
      if (!chartContainer.value) {
        console.error("Chart container not found");
        return;
      }

      try {
        const containerWidth = chartContainer.value.clientWidth || 600;
        const containerHeight = props.height;

        // Create the chart
        chart = createChart(chartContainer.value, {
          ...chartOptions,
          width: containerWidth,
          height: containerHeight,
        });

        // Create area series with gradient fill like in the reference image
        areaSeries = chart.addSeries(AreaSeries, {
          lineColor: chartColor,
          topColor: chartColor + "80", // 50% opacity for gradient top
          bottomColor: chartColor + "00", // Transparent for gradient bottom
          lineWidth: 2,
          priceFormat: {
            type: "price",
            precision: 2,
            minMove: 0.01,
          },
        });

        // Handle resize
        const resizeObserver = new ResizeObserver((entries) => {
          if (
            entries.length === 0 ||
            entries[0].target !== chartContainer.value ||
            !chart
          )
            return;

          const { width } = entries[0].contentRect;
          if (width > 0) {
            chart.applyOptions({
              width: Math.floor(width),
              height: props.height,
            });
          }
        });

        resizeObserver.observe(chartContainer.value);
      } catch (err) {
        console.error("Error initializing chart:", err);
        error.value = "Failed to initialize chart: " + err.message;
      }
    };

    const calculateDateRange = (range) => {
      const now = new Date();
      let startDate;
      let timeframe = "D"; // Default to daily bars

      switch (range) {
        case "1D":
          // 1D timeframe but show 1 year of daily data
          startDate = new Date(now.getFullYear() - 1, now.getMonth(), now.getDate());
          timeframe = "D"; // Daily bars
          break;
        case "1W":
          // 1W timeframe but show 2 years of weekly data
          startDate = new Date(now.getFullYear() - 2, now.getMonth(), now.getDate());
          timeframe = "W"; // Weekly bars
          break;
        case "1M":
          // 1M timeframe but show 5 years of monthly data
          startDate = new Date(now.getFullYear() - 5, now.getMonth(), now.getDate());
          timeframe = "M"; // Monthly bars
          break;
        default:
          startDate = new Date(now.getFullYear() - 1, now.getMonth(), now.getDate());
          timeframe = "D";
      }

      return {
        startDate: startDate.toISOString().split("T")[0],
        timeframe,
      };
    };

    const loadChartData = async (range) => {
      if (!props.symbol || !areaSeries) return;

      // Check cache first
      const cacheKey = `${props.symbol}-${range}`;
      if (chartData.has(cacheKey)) {
        const cachedData = chartData.get(cacheKey);
        areaSeries.setData(cachedData);
        chart.timeScale().fitContent();
        return;
      }

      loading.value = true;
      error.value = "";

      try {
        let data;
        const { startDate, timeframe } = calculateDateRange(range);

        // Use regular historical data for all ranges
        const limit = range === "1D" ? 365 : (range === "1W" ? 104 : 60); // 1 year daily, 2 years weekly, 5 years monthly
        data = await getHistoricalData(props.symbol, timeframe, {
          limit,
          start_date: startDate,
        });

        if (data && data.bars && data.bars.length > 0) {
          const bars = data.bars;

          // Transform data for area chart
          const chartDataPoints = bars.map((bar) => {
            let time = bar.time;

            // Handle different time formats
            if (typeof time === "string" && time.includes(" ")) {
              const etTimeString = time + ":00 EST";
              const date = new Date(etTimeString);
              time = Math.floor(date.getTime() / 1000);
            }

            return {
              time: time,
              value: bar.close, // Use close price for area chart
            };
          });

          // Cache the data
          chartData.set(cacheKey, chartDataPoints);

          // Set the data
          areaSeries.setData(chartDataPoints);

          // Keep the same color - no color changes needed

          // Fit content to show all data
          chart.timeScale().fitContent();
        } else {
          throw new Error("No data available for this range");
        }
      } catch (err) {
        console.error("Error loading chart data:", err);
        error.value = err.response?.data?.detail || err.message || "Failed to load chart data";
      } finally {
        loading.value = false;
      }
    };

    const updateRealTimeData = (priceData) => {
      if (!areaSeries || !priceData) return;

      try {
        // Get the current price from the streaming data - prioritize last price
        const newPrice =
          priceData.price ||
          priceData.last ||
          priceData.mid ||
          (priceData.bid && priceData.ask
            ? (priceData.bid + priceData.ask) / 2
            : null) ||
          priceData.bid ||
          priceData.ask;

        if (!newPrice) return;

        // Get the current time aligned to the timeframe
        const now = new Date();
        let alignedTime;

        // Align time based on selected timeframe
        switch (selectedRange.value) {
          case "1D":
            // For daily, align to current day
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              0,
              0,
              0,
              0
            );
            break;
          case "1W":
            // For weekly, align to start of current week (Monday)
            const dayOfWeek = now.getDay();
            const daysToMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1;
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate() - daysToMonday,
              0,
              0,
              0,
              0
            );
            break;
          case "1M":
            // For monthly, align to start of current month
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              1,
              0,
              0,
              0,
              0
            );
            break;
          default:
            // Default to daily alignment
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              0,
              0,
              0,
              0
            );
        }

        const timeInSeconds = Math.floor(alignedTime.getTime() / 1000);

        // Check if this is a new data point or updating existing one
        if (!currentDataPoint || currentDataPoint.time !== timeInSeconds) {
          // New data point - create it with current price
          currentDataPoint = {
            time: timeInSeconds,
            value: newPrice,
          };
        } else {
          // Update existing data point with latest price
          currentDataPoint = {
            time: currentDataPoint.time,
            value: newPrice, // Always update to latest price
          };
        }

        // Update the chart with the current data point
        areaSeries.update(currentDataPoint);
      } catch (err) {
        console.error("Error updating real-time data:", err);
      }
    };

    const setChartRange = (range) => {
      selectedRange.value = range;
      currentDataPoint = null; // Reset current data point when range changes
      loadChartData(range);
    };

    const cleanup = () => {
      if (chart) {
        chart.remove();
        chart = null;
        areaSeries = null;
      }
      chartData.clear();
    };

    // Watch for symbol changes
    watch(
      () => props.symbol,
      (newSymbol, oldSymbol) => {
        if (newSymbol && newSymbol !== oldSymbol) {
          // Clear cache when symbol changes
          chartData.clear();
          // Reset current data point when symbol changes
          currentDataPoint = null;
          // Reload data for current range
          loadChartData(selectedRange.value);
        }
      }
    );

    // Watch for live price updates
    watch(
      () => props.livePrice,
      (newPriceData) => {
        if (newPriceData && props.enableRealtime && areaSeries) {
          updateRealTimeData(newPriceData);
        }
      },
      { deep: true }
    );

    onMounted(async () => {
      await nextTick();
      initializeChart();

      if (props.symbol) {
        // Load default range (1D)
        await loadChartData(selectedRange.value);
      }
    });

    onUnmounted(() => {
      cleanup();
    });

    return {
      chartContainer,
      loading,
      error,
      selectedRange,
      ranges,
      setChartRange,
    };
  },
};
</script>

<style scoped>
.right-panel-chart-container {
  width: 100%;
  height: 100%;
  position: relative;
  background-color: #0b0d10;
  border-radius: 8px;
  overflow: hidden;
  margin: 0; /* Ensure no margins */
  padding: 0; /* Ensure no padding */
}

.chart-container {
  width: 100%;
  height: 100%;
  position: relative;
}

.range-switcher {
  position: absolute;
  top: 12px;
  left: 12px;
  display: flex;
  gap: 4px;
  z-index: 10;
}

.range-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #d1d4dc;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 32px;
  text-align: center;
}

.range-btn:hover {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
}

.range-btn.active {
  background: rgba(41, 98, 255, 0.8);
  border-color: #2962FF;
  color: white;
  box-shadow: 0 2px 4px rgba(41, 98, 255, 0.3);
}

.chart-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(11, 13, 16, 0.8);
  z-index: 5;
}

.loading-spinner {
  color: #d1d4dc;
  font-size: 14px;
  animation: pulse 1.5s ease-in-out infinite;
}

.error-message {
  color: #ff4444;
  font-size: 12px;
  text-align: center;
  padding: 0 16px;
}

.chart-overlay.error {
  background: rgba(11, 13, 16, 0.9);
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}
</style>
