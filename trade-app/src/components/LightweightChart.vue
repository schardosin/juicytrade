<template>
  <div class="lightweight-chart-container">
    <div ref="chartContainer" class="chart-container"></div>
    <div v-if="!hideControls" class="chart-controls">
      <div class="control-section">
        <div class="timeframe-buttons">
          <button
            v-for="tf in timeframes"
            :key="tf.value"
            :class="[
              'timeframe-btn',
              { active: selectedTimeframe === tf.value },
            ]"
            @click="changeTimeframe(tf.value)"
          >
            {{ tf.label }}
          </button>
        </div>
        <div class="date-range-selector">
          <label class="range-label">Range:</label>
          <select
            v-model="selectedDateRange"
            @change="onDateRangeChange"
            class="range-dropdown"
          >
            <option
              v-for="range in dateRanges"
              :key="range.value"
              :value="range.value"
            >
              {{ range.label }}
            </option>
          </select>
        </div>
      </div>
      <div class="chart-info">
        <span class="symbol-info">{{ symbol }}</span>
        <span v-if="loading" class="loading-indicator">Loading...</span>
        <span v-if="error" class="error-indicator">{{ error }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted, watch, nextTick } from "vue";
import {
  createChart,
  ColorType,
  CandlestickSeries,
  HistogramSeries,
} from "lightweight-charts";
import { useMarketData } from "../composables/useMarketData.js";

export default {
  name: "LightweightChart",
  props: {
    symbol: {
      type: String,
      default: "SPY",
    },
    theme: {
      type: String,
      default: "dark",
    },
    height: {
      type: Number,
      default: null,
    },
    enableRealtime: {
      type: Boolean,
      default: true,
    },
    livePrice: {
      type: Object,
      default: null,
    },
    hideControls: {
      type: Boolean,
      default: false,
    },
  },
  setup(props) {
    const chartContainer = ref(null);
    const loading = ref(false);
    const error = ref("");
    const selectedTimeframe = ref("D");
    const selectedDateRange = ref("6M");

    // Use unified market data composable
    const { getHistoricalData, getHistoricalDailySixMonths } = useMarketData();

    let chart = null;
    let candlestickSeries = null;
    let volumeSeries = null;
    let wsConnection = null;
    let currentCandle = null; // Track the current candle being updated

    const timeframes = [
      { label: "1m", value: "1m" },
      { label: "5m", value: "5m" },
      { label: "15m", value: "15m" },
      { label: "1h", value: "1h" },
      { label: "4h", value: "4h" },
      { label: "1D", value: "D" },
      { label: "1W", value: "W" },
      { label: "1M", value: "M" },
    ];

    const dateRanges = [
      { label: "1 Day", value: "1D" },
      { label: "1 Week", value: "1W" },
      { label: "1 Month", value: "1M" },
      { label: "6 Months", value: "6M" },
      { label: "1 Year", value: "1Y" },
      { label: "5 Years", value: "5Y" },
      { label: "10 Years", value: "10Y" },
      { label: "20 Years", value: "20Y" },
    ];

    const chartOptions = {
      layout: {
        background: {
          type: "solid",
          color: props.theme === "dark" ? "#141519" : "#ffffff",
        },
        textColor: props.theme === "dark" ? "#d1d4dc" : "#191919",
        attributionLogo: false,
      },
      grid: {
        vertLines: {
          color: props.theme === "dark" ? "#141519" : "#e1e3e6",
        },
        horzLines: {
          color: props.theme === "dark" ? "#141519" : "#e1e3e6",
        },
      },
      crosshair: {
        mode: 1, // Normal crosshair mode
      },
      rightPriceScale: {
        borderColor: props.theme === "dark" ? "#485c7b" : "#cccccc",
      },
      timeScale: {
        borderColor: props.theme === "dark" ? "#485c7b" : "#cccccc",
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: {
        mouseWheel: true,
        pressedMouseMove: true,
      },
      handleScale: {
        axisPressedMouseMove: true,
        mouseWheel: true,
        pinch: true,
      },
    };

    const candlestickOptions = {
      upColor: "#26a69a",
      downColor: "#ef5350",
      borderVisible: false,
      wickUpColor: "#26a69a",
      wickDownColor: "#ef5350",
    };

    const volumeOptions = {
      color: "#26a69a",
      priceFormat: {
        type: "volume",
      },
      priceScaleId: "",
      scaleMargins: {
        top: 0.85,
        bottom: 0,
      },
    };

    const initializeChart = () => {
      if (!chartContainer.value) {
        console.error("Chart container not found");
        return;
      }

      try {
        // Ensure container has dimensions
        const containerWidth = chartContainer.value.clientWidth || 800;
        const containerHeight = props.height || chartContainer.value.clientHeight || 300;

        // Create the chart
        chart = createChart(chartContainer.value, {
          ...chartOptions,
          width: containerWidth,
          height: containerHeight,
        });

        // Create candlestick series
        candlestickSeries = chart.addSeries(
          CandlestickSeries,
          candlestickOptions
        );

        // Create volume series
        volumeSeries = chart.addSeries(HistogramSeries, volumeOptions);

        // Apply scale margins to volume series price scale (following official example)
        volumeSeries.priceScale().applyOptions({
          scaleMargins: {
            top: 0.7, // highest point of the series will be 70% away from the top
            bottom: 0,
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

          const { width, height } = entries[0].contentRect;
          if (width > 0 && height > 0) {
            chart.applyOptions({
              width: Math.floor(width),
              height: props.height || Math.floor(height),
            });
          }
        });

        resizeObserver.observe(chartContainer.value);
      } catch (err) {
        console.error("Error initializing chart:", err);
        error.value = "Failed to initialize chart: " + err.message;
      }
    };

    const calculateStartDate = (dateRange) => {
      const now = new Date();
      let startDate;

      switch (dateRange) {
        case "1D":
          startDate = new Date(now.getTime() - 24 * 60 * 60 * 1000);
          break;
        case "1W":
          startDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
          break;
        case "1M":
          startDate = new Date(
            now.getFullYear(),
            now.getMonth() - 1,
            now.getDate()
          );
          break;
        case "6M":
          startDate = new Date(
            now.getFullYear(),
            now.getMonth() - 6,
            now.getDate()
          );
          break;
        case "1Y":
          startDate = new Date(
            now.getFullYear() - 1,
            now.getMonth(),
            now.getDate()
          );
          break;
        case "5Y":
          startDate = new Date(
            now.getFullYear() - 5,
            now.getMonth(),
            now.getDate()
          );
          break;
        case "10Y":
          startDate = new Date(
            now.getFullYear() - 10,
            now.getMonth(),
            now.getDate()
          );
          break;
        case "20Y":
          startDate = new Date(
            now.getFullYear() - 20,
            now.getMonth(),
            now.getDate()
          );
          break;
        default:
          startDate = new Date(
            now.getFullYear(),
            now.getMonth() - 1,
            now.getDate()
          );
          break;
      }

      return startDate.toISOString().split("T")[0];
    };

    const calculateLimit = (timeframe, dateRange) => {
      // Calculate appropriate limit based on timeframe and date range
      const now = new Date();
      const startDate = new Date(calculateStartDate(dateRange));
      const daysDiff = Math.ceil((now - startDate) / (1000 * 60 * 60 * 24));

      // For very long ranges, we need higher limits
      const isLongRange = ["5Y", "10Y", "20Y"].includes(dateRange);
      const maxLimit = isLongRange ? 10000 : 2000;

      switch (timeframe) {
        case "1m":
          return Math.min(daysDiff * 390, isLongRange ? 5000 : 2000); // ~390 minutes per trading day
        case "5m":
          return Math.min(daysDiff * 78, isLongRange ? 3000 : 1500); // ~78 5-minute bars per trading day
        case "15m":
          return Math.min(daysDiff * 26, isLongRange ? 2000 : 1000); // ~26 15-minute bars per trading day
        case "1h":
          return Math.min(daysDiff * 6.5, isLongRange ? 1500 : 800); // ~6.5 hours per trading day
        case "4h":
          return Math.min(daysDiff * 2, isLongRange ? 1000 : 400); // ~2 4-hour bars per trading day
        case "D":
          return Math.min(daysDiff, isLongRange ? 8000 : 2000); // 1 bar per day - much higher for long ranges
        case "W":
          return Math.min(Math.ceil(daysDiff / 7), isLongRange ? 1200 : 500); // 1 bar per week
        case "M":
          return Math.min(Math.ceil(daysDiff / 30), isLongRange ? 300 : 120); // 1 bar per month
        default:
          return isLongRange ? 5000 : 1000;
      }
    };

    const loadHistoricalData = async (
      symbol,
      timeframe,
      dateRange = selectedDateRange.value
    ) => {
      if (!symbol || !candlestickSeries || !volumeSeries) return;

      loading.value = true;
      error.value = "";

      try {
        // Calculate start date and limit based on user selection
        const start_date = calculateStartDate(dateRange);
        const limit = calculateLimit(timeframe, dateRange);

        // Choose fast path for default Daily + 6M to leverage cache + live updates
        let data;
        if (timeframe === "D" && dateRange === "6M") {
          data = await getHistoricalDailySixMonths(symbol);
        } else {
          // Use unified data access instead of direct axios call
          data = await getHistoricalData(symbol, timeframe, {
            limit,
            start_date,
          });
        }

        if (data && data.bars) {
          const bars = data.bars;

          if (bars.length === 0) {
            error.value = "No data available for this symbol/timeframe";
            return;
          }

          // Transform data for Lightweight Charts
          const candlestickData = bars.map((bar) => {
            // Handle different time formats from backend
            let time = bar.time;

            // If time contains space (datetime format like "2025-01-06 09:30"), convert to timestamp
            if (typeof time === "string" && time.includes(" ")) {
              // Backend sends Eastern Time datetime strings for intraday data
              // Parse as Eastern Time by appending EST timezone
              const etTimeString = time + ":00 EST"; // Assume EST for now
              const date = new Date(etTimeString);
              time = Math.floor(date.getTime() / 1000);
            }

            return {
              time: time,
              open: bar.open,
              high: bar.high,
              low: bar.low,
              close: bar.close,
            };
          });

          const volumeData = bars.map((bar) => {
            // Handle different time formats from backend
            let time = bar.time;

            // If time contains space (datetime format like "2025-01-06 09:30"), convert to timestamp
            if (typeof time === "string" && time.includes(" ")) {
              // Backend sends Eastern Time datetime strings for intraday data
              // Parse as Eastern Time by appending EST timezone
              const etTimeString = time + ":00 EST"; // Assume EST for now
              const date = new Date(etTimeString);
              time = Math.floor(date.getTime() / 1000);
            }

            return {
              time: time,
              value: bar.volume,
              color: bar.close >= bar.open ? "#26a69a80" : "#ef535080",
            };
          });

          // Set the data
          candlestickSeries.setData(candlestickData);
          volumeSeries.setData(volumeData);

          // Fit content to show all data
          chart.timeScale().fitContent();

        } else {
          throw new Error("Failed to load data");
        }
      } catch (err) {
        console.error("Error loading historical data:", err);
        error.value =
          err.response?.data?.detail ||
          err.message ||
          "Failed to load chart data";
      } finally {
        loading.value = false;
      }
    };

    const changeTimeframe = (timeframe) => {
      selectedTimeframe.value = timeframe;
      currentCandle = null; // Reset current candle when timeframe changes
      loadHistoricalData(props.symbol, timeframe, selectedDateRange.value);
    };

    const onDateRangeChange = () => {
      currentCandle = null; // Reset current candle when date range changes
      loadHistoricalData(
        props.symbol,
        selectedTimeframe.value,
        selectedDateRange.value
      );
    };

    // Real-time updates are now handled by the parent component through the global state system
    // The chart will receive price updates through props or other mechanisms
    // No direct WebSocket connection needed in the chart component

    const updateRealTimeData = (priceData) => {
      if (!candlestickSeries || !priceData) return;

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
        switch (selectedTimeframe.value) {
          case "1m":
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              now.getHours(),
              now.getMinutes(),
              0,
              0
            );
            break;
          case "5m":
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              now.getHours(),
              Math.floor(now.getMinutes() / 5) * 5,
              0,
              0
            );
            break;
          case "15m":
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              now.getHours(),
              Math.floor(now.getMinutes() / 15) * 15,
              0,
              0
            );
            break;
          case "1h":
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              now.getHours(),
              0,
              0,
              0
            );
            break;
          case "4h":
            alignedTime = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate(),
              Math.floor(now.getHours() / 4) * 4,
              0,
              0,
              0
            );
            break;
          case "D":
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
          default:
            // For daily and above, just use current day
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

        // Check if this is a new candle or updating existing one
        if (!currentCandle || currentCandle.time !== timeInSeconds) {
          // New candle - create it with current price as OHLC
          currentCandle = {
            time: timeInSeconds,
            open: newPrice,
            high: newPrice,
            low: newPrice,
            close: newPrice,
          };
        } else {
          // Update existing candle - adjust high, low, and close
          currentCandle = {
            time: currentCandle.time,
            open: currentCandle.open, // Keep original open
            high: Math.max(currentCandle.high, newPrice),
            low: Math.min(currentCandle.low, newPrice),
            close: newPrice, // Always update close to latest price
          };
        }

        // Update the chart with the current candle
        candlestickSeries.update(currentCandle);
      } catch (err) {
        console.error("Error updating real-time data:", err);
      }
    };

    const cleanup = () => {
      if (chart) {
        chart.remove();
        chart = null;
        candlestickSeries = null;
        volumeSeries = null;
      }
    };

    // Watch for symbol changes
    watch(
      () => props.symbol,
      (newSymbol, oldSymbol) => {
        if (newSymbol && candlestickSeries) {
          // CRITICAL: Reset current candle when symbol changes to prevent price contamination
          if (newSymbol !== oldSymbol) {
            currentCandle = null;
          }

          loadHistoricalData(newSymbol, selectedTimeframe.value);
          // WebSocket subscription is handled by the shared client
          console.log("Chart symbol changed to:", newSymbol);
        }
      }
    );

    // Watch for live price updates
    watch(
      () => props.livePrice,
      (newPriceData) => {
        if (newPriceData && props.enableRealtime && candlestickSeries) {
          updateRealTimeData(newPriceData);
        }
      },
      { deep: true }
    );

    // Watch for theme changes
    watch(
      () => props.theme,
      (newTheme) => {
        if (chart) {
          const newOptions = {
            layout: {
              background: {
                type: "solid",
                color: newTheme === "dark" ? "#1e1e1e" : "#ffffff",
              },
              textColor: newTheme === "dark" ? "#d1d4dc" : "#191919",
            },
            grid: {
              vertLines: {
                color: newTheme === "dark" ? "#2B2B43" : "#e1e3e6",
              },
              horzLines: {
                color: newTheme === "dark" ? "#2B2B43" : "#e1e3e6",
              },
            },
            rightPriceScale: {
              borderColor: newTheme === "dark" ? "#485c7b" : "#cccccc",
            },
            timeScale: {
              borderColor: newTheme === "dark" ? "#485c7b" : "#cccccc",
            },
          };
          chart.applyOptions(newOptions);
        }
      }
    );

    onMounted(async () => {
      await nextTick();
      initializeChart();

      if (props.symbol) {
        await loadHistoricalData(props.symbol, selectedTimeframe.value);
      }

      // Real-time updates are now handled by the parent component through global state
      // No direct WebSocket connection needed here
    });

    onUnmounted(() => {
      cleanup();
    });

    return {
      chartContainer,
      loading,
      error,
      selectedTimeframe,
      selectedDateRange,
      timeframes,
      dateRanges,
      changeTimeframe,
      onDateRangeChange,
    };
  },
};
</script>

<style scoped>
.lightweight-chart-container {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-primary);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.chart-container {
  flex: 1;
  min-height: 200px;
  width: 100%;
  height: 100%;
  position: relative;
}

/* Mobile responsive adjustments */
@media (max-width: 768px) {
  .chart-container {
    min-height: 150px;
  }
}

.chart-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--bg-secondary);
  border-top: 1px solid var(--border-primary);
  flex-shrink: 0;
  min-height: 60px;
}

.control-section {
  display: flex;
  align-items: center;
  gap: 20px;
}

.timeframe-buttons {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.date-range-selector {
  display: flex;
  align-items: center;
  gap: 8px;
}

.range-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  white-space: nowrap;
}

.range-dropdown {
  padding: 6px 10px;
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: all var(--transition-normal);
  min-width: 100px;
}

.range-dropdown:hover {
  background-color: var(--bg-quaternary);
  border-color: var(--border-tertiary);
}

.range-dropdown:focus {
  outline: none;
  border-color: var(--color-info);
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.range-dropdown option {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.timeframe-btn {
  padding: var(--spacing-sm) var(--spacing-md);
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  transition: all var(--transition-normal);
  white-space: nowrap;
  min-width: 40px;
  text-align: center;
}

.timeframe-btn:hover {
  background-color: var(--bg-quaternary);
  border-color: var(--border-tertiary);
}

.timeframe-btn.active {
  background-color: var(--color-brand);
  border-color: var(--color-brand);
  color: var(--text-primary);
  box-shadow: 0 2px 4px rgba(255, 107, 53, 0.3);
}

.chart-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.symbol-info {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.loading-indicator {
  color: var(--color-warning);
  font-size: var(--font-size-sm);
  animation: pulse 1.5s ease-in-out infinite;
}

.error-indicator {
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

/* Light theme overrides */
.lightweight-chart-container.light {
  background-color: #ffffff;
}

.lightweight-chart-container.light .chart-controls {
  background-color: #f5f5f5;
  border-top-color: #e0e0e0;
}

.lightweight-chart-container.light .timeframe-btn {
  background-color: #ffffff;
  color: #333333;
  border-color: #cccccc;
}

.lightweight-chart-container.light .timeframe-btn:hover {
  background-color: #f0f0f0;
  border-color: #999999;
}

.lightweight-chart-container.light .symbol-info {
  color: #333333;
}
</style>
