<template>
  <div class="lightweight-chart-container">
    <div ref="chartContainer" class="chart-container"></div>
    <div class="chart-controls">
      <div class="timeframe-buttons">
        <button
          v-for="tf in timeframes"
          :key="tf.value"
          :class="['timeframe-btn', { active: selectedTimeframe === tf.value }]"
          @click="changeTimeframe(tf.value)"
        >
          {{ tf.label }}
        </button>
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
import axios from "axios";

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
      default: 400,
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
    const selectedTimeframe = ref("D");

    let chart = null;
    let candlestickSeries = null;
    let volumeSeries = null;
    let wsConnection = null;

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

    const chartOptions = {
      layout: {
        background: {
          type: "solid",
          color: props.theme === "dark" ? "#1e1e1e" : "#ffffff",
        },
        textColor: props.theme === "dark" ? "#d1d4dc" : "#191919",
      },
      grid: {
        vertLines: {
          color: props.theme === "dark" ? "#2B2B43" : "#e1e3e6",
        },
        horzLines: {
          color: props.theme === "dark" ? "#2B2B43" : "#e1e3e6",
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
        console.log("Initializing Lightweight Charts...");
        console.log("Container dimensions:", {
          width: chartContainer.value.clientWidth,
          height: chartContainer.value.clientHeight,
          offsetWidth: chartContainer.value.offsetWidth,
          offsetHeight: chartContainer.value.offsetHeight,
        });

        // Ensure container has dimensions
        const containerWidth = chartContainer.value.clientWidth || 800;
        const containerHeight = props.height || 400;

        // Create the chart
        chart = createChart(chartContainer.value, {
          ...chartOptions,
          width: containerWidth,
          height: containerHeight,
        });

        console.log("Chart created successfully");

        // Create candlestick series
        candlestickSeries = chart.addSeries(
          CandlestickSeries,
          candlestickOptions
        );
        console.log("Candlestick series added");

        // Create volume series
        volumeSeries = chart.addSeries(HistogramSeries, volumeOptions);

        // Apply scale margins to volume series price scale (following official example)
        volumeSeries.priceScale().applyOptions({
          scaleMargins: {
            top: 0.7, // highest point of the series will be 70% away from the top
            bottom: 0,
          },
        });

        console.log("Volume series added with proper scale margins");

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

        console.log("Lightweight Charts initialized successfully");
      } catch (err) {
        console.error("Error initializing chart:", err);
        error.value = "Failed to initialize chart: " + err.message;
      }
    };

    const loadHistoricalData = async (symbol, timeframe) => {
      if (!symbol || !candlestickSeries || !volumeSeries) return;

      loading.value = true;
      error.value = "";

      try {
        console.log(`Loading historical data for ${symbol} (${timeframe})`);

        // Determine limit and date range based on timeframe
        let limit, start_date;
        const now = new Date();

        switch (timeframe) {
          case "M":
            // For monthly data, get 10 years of history
            limit = 120;
            start_date = new Date(now.getFullYear() - 10, now.getMonth(), 1)
              .toISOString()
              .split("T")[0];
            break;
          case "W":
            // For weekly data, get 3 years of history
            limit = 156;
            start_date = new Date(
              now.getFullYear() - 3,
              now.getMonth(),
              now.getDate()
            )
              .toISOString()
              .split("T")[0];
            break;
          case "D":
            // For daily data, get 2 years of history
            limit = 730;
            start_date = new Date(
              now.getFullYear() - 2,
              now.getMonth(),
              now.getDate()
            )
              .toISOString()
              .split("T")[0];
            break;
          case "4h":
            // For 4-hour data, get more data
            limit = 2000;
            start_date = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate() - 30
            )
              .toISOString()
              .split("T")[0];
            break;
          case "1h":
            // For hourly data, get more data
            limit = 3000;
            start_date = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate() - 14
            )
              .toISOString()
              .split("T")[0];
            break;
          default:
            // For intraday data (1m, 5m, 15m), get more recent data
            limit = 1000;
            start_date = new Date(
              now.getFullYear(),
              now.getMonth(),
              now.getDate() - 7
            )
              .toISOString()
              .split("T")[0];
            break;
        }

        const params = {
          timeframe,
          limit,
        };

        // Add start_date if we calculated one
        if (start_date) {
          params.start_date = start_date;
        }

        const response = await axios.get(`/api/chart/historical/${symbol}`, {
          params,
        });

        if (response.data.success && response.data.data.bars) {
          const bars = response.data.data.bars;
          console.log(`Received ${bars.length} bars for ${symbol}`);

          if (bars.length === 0) {
            error.value = "No data available for this symbol/timeframe";
            return;
          }

          // Transform data for Lightweight Charts
          const candlestickData = bars.map((bar) => {
            // Handle different time formats from backend
            let time = bar.time;

            // If time contains space (datetime format like "2025-07-11 19:55"), convert to timestamp
            if (typeof time === "string" && time.includes(" ")) {
              // Convert datetime string to Unix timestamp
              const date = new Date(time);
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

            // If time contains space (datetime format like "2025-07-11 19:55"), convert to timestamp
            if (typeof time === "string" && time.includes(" ")) {
              // Convert datetime string to Unix timestamp
              const date = new Date(time);
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

          console.log(`Chart updated with ${bars.length} data points`);
        } else {
          throw new Error(response.data.error || "Failed to load data");
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
      loadHistoricalData(props.symbol, timeframe);
    };

    const connectWebSocket = () => {
      if (!props.enableRealtime) return;

      try {
        const wsUrl = `ws://localhost:8008/ws`;
        wsConnection = new WebSocket(wsUrl);

        wsConnection.onopen = () => {
          console.log("WebSocket connected for real-time updates");
          // Subscribe to symbol updates
          wsConnection.send(
            JSON.stringify({
              type: "subscribe",
              symbols: [props.symbol],
            })
          );
        };

        wsConnection.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            if (data.type === "price_update" && data.symbol === props.symbol) {
              // Update the last candle with real-time data
              updateRealTimeData(data.data);
            }
          } catch (err) {
            console.error("Error processing WebSocket message:", err);
          }
        };

        wsConnection.onclose = () => {
          console.log("WebSocket connection closed");
          // Attempt to reconnect after 5 seconds
          setTimeout(connectWebSocket, 5000);
        };

        wsConnection.onerror = (err) => {
          console.error("WebSocket error:", err);
        };
      } catch (err) {
        console.error("Error connecting to WebSocket:", err);
      }
    };

    const updateRealTimeData = (priceData) => {
      if (!candlestickSeries || !priceData) return;

      try {
        // For real-time updates, we would update the last candle
        // This is a simplified implementation
        const currentTime = Math.floor(Date.now() / 1000);
        const lastPrice = priceData.bid || priceData.ask || priceData.price;

        if (lastPrice) {
          candlestickSeries.update({
            time: currentTime,
            close: lastPrice,
          });
        }
      } catch (err) {
        console.error("Error updating real-time data:", err);
      }
    };

    const cleanup = () => {
      if (wsConnection) {
        wsConnection.close();
        wsConnection = null;
      }
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
      (newSymbol) => {
        if (newSymbol && candlestickSeries) {
          loadHistoricalData(newSymbol, selectedTimeframe.value);

          // Update WebSocket subscription
          if (wsConnection && wsConnection.readyState === WebSocket.OPEN) {
            wsConnection.send(
              JSON.stringify({
                type: "subscribe",
                symbols: [newSymbol],
              })
            );
          }
        }
      }
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

      if (props.enableRealtime) {
        connectWebSocket();
      }
    });

    onUnmounted(() => {
      cleanup();
    });

    return {
      chartContainer,
      loading,
      error,
      selectedTimeframe,
      timeframes,
      changeTimeframe,
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
  background-color: #1e1e1e;
  border-radius: 8px;
  overflow: hidden;
}

.chart-container {
  flex: 1;
  min-height: 400px;
  width: 100%;
  height: 100%;
  position: relative;
}

.chart-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #2a2a2a;
  border-top: 1px solid #3a3a3a;
  flex-shrink: 0;
  min-height: 60px;
}

.timeframe-buttons {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.timeframe-btn {
  padding: 8px 12px;
  background-color: #3a3a3a;
  color: #d1d4dc;
  border: 1px solid #4a4a4a;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: all 0.2s ease;
  white-space: nowrap;
  min-width: 40px;
  text-align: center;
}

.timeframe-btn:hover {
  background-color: #4a4a4a;
  border-color: #5a5a5a;
}

.timeframe-btn.active {
  background-color: #2962ff;
  border-color: #2962ff;
  color: white;
  box-shadow: 0 2px 4px rgba(41, 98, 255, 0.3);
}

.chart-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.symbol-info {
  font-weight: 600;
  color: #d1d4dc;
  font-size: 14px;
}

.loading-indicator {
  color: #ffa726;
  font-size: 12px;
  animation: pulse 1.5s ease-in-out infinite;
}

.error-indicator {
  color: #ef5350;
  font-size: 12px;
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
