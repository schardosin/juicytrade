<template>
  <div class="smart-market-test">
    <div class="test-header">
      <h3>🧪 Smart Market Data Store Test</h3>
      <p>Testing automatic subscription management and live price updates</p>
    </div>

    <div class="test-controls">
      <div class="symbol-input">
        <label>Test Symbol:</label>
        <input
          v-model="testSymbol"
          placeholder="Enter symbol (e.g., AAPL, SPY)"
          @keyup.enter="updateSymbol"
        />
        <button @click="updateSymbol">Update</button>
      </div>

      <div class="test-buttons">
        <button @click="showDebugInfo">Show Debug Info</button>
        <button @click="forceCleanup" class="danger">Force Cleanup</button>
      </div>
    </div>

    <div class="test-results">
      <div class="price-display">
        <h4>Current Symbol: {{ currentSymbol || "None" }}</h4>
        <div v-if="currentSymbol" class="price-info">
          <div class="price-row">
            <span class="label">Live Price:</span>
            <span class="value">
              {{
                livePrice?.price
                  ? `$${livePrice.price.toFixed(2)}`
                  : "Loading..."
              }}
            </span>
          </div>
          <div class="price-row">
            <span class="label">Bid:</span>
            <span class="value">{{
              livePrice?.bid ? `$${livePrice.bid.toFixed(2)}` : "--"
            }}</span>
          </div>
          <div class="price-row">
            <span class="label">Ask:</span>
            <span class="value">{{
              livePrice?.ask ? `$${livePrice.ask.toFixed(2)}` : "--"
            }}</span>
          </div>
          <div class="price-row">
            <span class="label">Last Update:</span>
            <span class="value">{{ lastUpdateTime }}</span>
          </div>
        </div>
      </div>

      <div class="debug-info" v-if="showDebug">
        <h4>Debug Information</h4>
        <pre>{{ JSON.stringify(debugInfo, null, 2) }}</pre>
      </div>
    </div>

    <div class="test-log">
      <h4>Activity Log</h4>
      <div class="log-entries">
        <div
          v-for="(entry, index) in logEntries"
          :key="index"
          class="log-entry"
          :class="entry.type"
        >
          <span class="timestamp">{{ entry.timestamp }}</span>
          <span class="message">{{ entry.message }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted } from "vue";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";

export default {
  name: "SmartMarketDataTest",
  setup() {
    const { getStockPrice, getDebugInfo, forceCleanup } = useSmartMarketData();

    // Test state
    const testSymbol = ref("AAPL");
    const currentSymbol = ref("");
    const showDebug = ref(false);
    const debugInfo = ref({});
    const logEntries = ref([]);

    // Get live price for current symbol
    const livePriceRef = computed(() => {
      if (!currentSymbol.value) return null;
      return getStockPrice(currentSymbol.value);
    });

    const livePrice = computed(() => {
      return livePriceRef.value?.value || null;
    });

    const lastUpdateTime = computed(() => {
      if (!livePrice.value?.timestamp) return "--";
      return new Date(livePrice.value.timestamp).toLocaleTimeString();
    });

    // Methods
    const updateSymbol = () => {
      const newSymbol = testSymbol.value.trim().toUpperCase();
      if (newSymbol && newSymbol !== currentSymbol.value) {
        addLogEntry("info", `Switching to symbol: ${newSymbol}`);
        currentSymbol.value = newSymbol;
        testSymbol.value = ""; // Clear input after update
      }
    };

    const showDebugInfo = () => {
      debugInfo.value = getDebugInfo();
      showDebug.value = !showDebug.value;
      addLogEntry("debug", "Debug info toggled");
    };

    const handleForceCleanup = () => {
      forceCleanup();
      addLogEntry("warning", "Force cleanup executed");
    };

    const addLogEntry = (type, message) => {
      const timestamp = new Date().toLocaleTimeString();
      logEntries.value.unshift({
        type,
        message,
        timestamp,
      });

      // Keep only last 20 entries
      if (logEntries.value.length > 20) {
        logEntries.value = logEntries.value.slice(0, 20);
      }
    };

    // Watchers
    watch(currentSymbol, (newSymbol, oldSymbol) => {
      if (newSymbol !== oldSymbol) {
        if (oldSymbol) {
          addLogEntry("info", `Unsubscribed from: ${oldSymbol}`);
        }
        if (newSymbol) {
          addLogEntry("success", `Subscribed to: ${newSymbol}`);
        }
      }
    });

    watch(
      livePrice,
      (newPrice, oldPrice) => {
        if (newPrice?.price && newPrice.price !== oldPrice?.price) {
          addLogEntry(
            "success",
            `Price update: ${currentSymbol.value} = $${newPrice.price.toFixed(
              2
            )}`
          );
        }
      },
      { deep: true }
    );

    // Initialize
    onMounted(() => {
      addLogEntry("info", "Smart Market Data Test initialized");
      updateSymbol(); // Set initial symbol
    });

    return {
      // State
      testSymbol,
      currentSymbol,
      showDebug,
      debugInfo,
      logEntries,

      // Computed
      livePrice,
      lastUpdateTime,

      // Methods
      updateSymbol,
      showDebugInfo,
      forceCleanup: handleForceCleanup,
    };
  },
};
</script>

<style scoped>
.smart-market-test {
  max-width: 800px;
  margin: 20px auto;
  padding: 20px;
  background: var(--bg-secondary, #1a1a1a);
  border-radius: 8px;
  color: var(--text-primary, #ffffff);
}

.test-header {
  text-align: center;
  margin-bottom: 20px;
}

.test-header h3 {
  margin: 0 0 8px 0;
  color: var(--color-brand, #007bff);
}

.test-header p {
  margin: 0;
  color: var(--text-secondary, #888);
  font-size: 14px;
}

.test-controls {
  display: flex;
  gap: 20px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.symbol-input {
  display: flex;
  align-items: center;
  gap: 8px;
}

.symbol-input label {
  font-weight: 500;
}

.symbol-input input {
  padding: 8px 12px;
  border: 1px solid var(--border-color, #333);
  border-radius: 4px;
  background: var(--bg-tertiary, #2a2a2a);
  color: var(--text-primary, #fff);
  min-width: 150px;
}

.test-buttons {
  display: flex;
  gap: 8px;
}

button {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  background: var(--color-brand, #007bff);
  color: white;
  cursor: pointer;
  font-size: 14px;
  transition: background-color 0.2s;
}

button:hover {
  background: var(--color-brand-hover, #0056b3);
}

button.danger {
  background: var(--color-danger, #dc3545);
}

button.danger:hover {
  background: var(--color-danger-hover, #c82333);
}

.test-results {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  margin-bottom: 20px;
}

.price-display,
.debug-info {
  padding: 16px;
  background: var(--bg-tertiary, #2a2a2a);
  border-radius: 6px;
}

.price-display h4,
.debug-info h4 {
  margin: 0 0 12px 0;
  color: var(--color-brand, #007bff);
}

.price-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.price-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.price-row .label {
  font-weight: 500;
  color: var(--text-secondary, #888);
}

.price-row .value {
  font-weight: 600;
  color: var(--text-primary, #fff);
}

.debug-info pre {
  background: var(--bg-quaternary, #1a1a1a);
  padding: 12px;
  border-radius: 4px;
  font-size: 12px;
  overflow-x: auto;
  margin: 0;
}

.test-log {
  margin-top: 20px;
}

.test-log h4 {
  margin: 0 0 12px 0;
  color: var(--color-brand, #007bff);
}

.log-entries {
  max-height: 200px;
  overflow-y: auto;
  background: var(--bg-tertiary, #2a2a2a);
  border-radius: 6px;
  padding: 12px;
}

.log-entry {
  display: flex;
  gap: 12px;
  padding: 4px 0;
  font-size: 13px;
  border-bottom: 1px solid var(--border-color, #333);
}

.log-entry:last-child {
  border-bottom: none;
}

.log-entry .timestamp {
  color: var(--text-tertiary, #666);
  font-family: monospace;
  min-width: 80px;
}

.log-entry.success .message {
  color: var(--color-success, #28a745);
}

.log-entry.warning .message {
  color: var(--color-warning, #ffc107);
}

.log-entry.debug .message {
  color: var(--color-info, #17a2b8);
}

.log-entry.info .message {
  color: var(--text-primary, #fff);
}

@media (max-width: 768px) {
  .test-results {
    grid-template-columns: 1fr;
  }

  .test-controls {
    flex-direction: column;
  }
}
</style>
