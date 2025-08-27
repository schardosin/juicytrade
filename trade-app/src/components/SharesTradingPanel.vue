<template>
  <!-- Simplified Bottom Trading Panel for Shares -->
  <div v-if="visible" class="bottom-panel slide-up">
    <!-- Top Stats Row -->
    <div class="stats-row">
      <div class="stat-group">
        <span class="stat-label">Position</span>
        <span class="stat-value">{{ stats.position }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Avg Cost</span>
        <span class="stat-value">${{ stats.avgCost }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Day P&L</span>
        <span class="stat-value" :class="stats.dayPL >= 0 ? 'positive' : 'negative'">${{ stats.dayPL }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Total P&L</span>
        <span class="stat-value" :class="stats.totalPL >= 0 ? 'positive' : 'negative'">${{ stats.totalPL }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Day Change</span>
        <span class="stat-value" :class="stats.dayChange >= 0 ? 'positive' : 'negative'">{{ stats.dayChange }}%</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Market Cap</span>
        <span class="stat-value">{{ stats.marketCap }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Volume</span>
        <span class="stat-value">{{ stats.volume }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Buying Power</span>
        <span class="stat-value">${{ stats.buyingPower }}</span>
      </div>
    </div>

    <!-- Main Trading Row -->
    <div class="trading-row">
      <!-- Left Side: Order Side and Quantity -->
      <div class="left-controls">
        <div class="order-side-section">
          <span class="control-label">Order Side</span>
          <div class="side-buttons">
            <button 
              class="side-btn buy-btn" 
              :class="{ active: orderSide === 'buy' }"
              @click="orderSide = 'buy'"
            >
              Buy
            </button>
            <button 
              class="side-btn sell-btn" 
              :class="{ active: orderSide === 'sell' }"
              @click="orderSide = 'sell'"
            >
              Sell
            </button>
          </div>
        </div>

        <div class="quantity-section">
          <span class="control-label">Quantity</span>
          <div class="quantity-input-group">
            <input 
              v-model="quantity" 
              type="number" 
              class="quantity-input"
              min="1"
              step="1"
            />
            <button class="qty-btn" @click="decrementQuantity">-</button>
            <button class="qty-btn" @click="incrementQuantity">+</button>
          </div>
        </div>
      </div>

      <!-- Center: Price Progress Bar -->
      <div class="price-section">
        <div class="price-row">
          <div class="bid-section">
            <span class="price-label">BID (OPP)</span>
            <div class="price-display">
              <span class="price-dot bid"></span>
              <span class="price-text">${{ bidPrice.toFixed(2) }} db</span>
            </div>
          </div>

          <div class="progress-section">
            <span class="price-label">MID</span>
            <div class="dual-progress-bar">
              <!-- Left Progress Bar -->
              <div class="progress-bar left">
                <div
                  class="progress-fill-left"
                  :style="{ width: leftProgressPercent + '%' }"
                ></div>
              </div>
              <!-- Center Indicator -->
              <div class="center-indicator">
                <span class="price-text highlight">${{ midPrice.toFixed(2) }} db</span>
              </div>
              <!-- Right Progress Bar -->
              <div class="progress-bar right">
                <div
                  class="progress-fill-right"
                  :style="{ width: rightProgressPercent + '%' }"
                ></div>
              </div>
            </div>
          </div>

          <div class="ask-section">
            <span class="price-label">ASK (NAT)</span>
            <div class="price-display">
              <span class="price-dot ask"></span>
              <span class="price-text">${{ askPrice.toFixed(2) }} db</span>
            </div>
          </div>
        </div>

        <!-- Limit Price Section (only shown for limit orders) -->
        <div v-if="needsLimitPrice" class="limit-price-row">
          <span class="limit-label">Limit Price</span>
          <div class="limit-input-group">
            <button
              class="price-btn lock-btn"
              :class="{ locked: priceLocked }"
              @click="togglePriceLock"
              :title="
                priceLocked
                  ? 'Unlock automatic price updates'
                  : 'Lock price (disable automatic updates)'
              "
            >
              <div class="lock-icon" :class="{ locked: priceLocked }">
                <div class="lock-body"></div>
                <div
                  class="lock-shackle"
                  :class="{ open: !priceLocked }"
                ></div>
              </div>
            </button>
            <input
              v-model="limitPrice"
              type="number"
              step="0.01"
              class="limit-input"
            />
            <button class="price-btn" @click="decrementPrice">-</button>
            <button class="price-btn" @click="incrementPrice">+</button>
          </div>
        </div>
      </div>

      <!-- Right Side: Order Type, Time in Force, and Action Buttons -->
      <div class="right-controls">
        <div class="config-row">
          <div class="order-type-section">
            <span class="config-label">Order Type</span>
            <select v-model="selectedOrderType" class="config-select">
              <option value="market">Market</option>
              <option value="limit">Limit</option>
              <option value="stop_market">Stop Market</option>
              <option value="stop_limit">Stop Limit</option>
            </select>
          </div>

          <div class="time-force-section">
            <span class="config-label">Time in Force</span>
            <select v-model="selectedTimeInForce" class="config-select">
              <option value="day">Day</option>
              <option value="gtc">GTC</option>
            </select>
          </div>
        </div>

        <div class="action-buttons">
          <button
            class="review-btn"
            @click="handleReviewSend"
            :disabled="!canSubmit"
          >
            Review & Send
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import { useGlobalSymbol } from "../composables/useGlobalSymbol.js";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";

export default {
  name: "SharesTradingPanel",
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
    symbol: {
      type: String,
      default: "SPY",
    },
    underlyingPrice: {
      type: Number,
      default: null,
    },
  },
  emits: ["review-send", "clear-trade"],
  setup(props, { emit }) {
    const { globalSymbolState } = useGlobalSymbol();

    // Component registration system
    const componentId = `SharesTradingPanel-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // Smart Market Data integration
    const { getStockPrice } = useSmartMarketData();

    // Single registration method per component to prevent double registration
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    // Trading state - Default quantity is 1 as requested
    const quantity = ref(1);
    const orderSide = ref("buy");
    const selectedOrderType = ref("market");
    const selectedTimeInForce = ref("day");
    const limitPrice = ref(0);
    const stopPrice = ref(0);
    const priceLocked = ref(false);

    // Get live price data
    const livePrice = computed(() => {
      if (!props.symbol) return null;
      
      // Ensure symbol is registered for component tracking
      ensureSymbolRegistration(props.symbol);
      
      return getStockPrice(props.symbol);
    });

    // Price calculations
    const bidPrice = computed(() => {
      const live = livePrice.value?.value;
      return live?.bid || props.underlyingPrice - 0.01 || 0;
    });

    const askPrice = computed(() => {
      const live = livePrice.value?.value;
      return live?.ask || props.underlyingPrice + 0.01 || 0;
    });

    const midPrice = computed(() => {
      return (bidPrice.value + askPrice.value) / 2;
    });

    // Order type helpers
    const needsLimitPrice = computed(() => {
      return selectedOrderType.value === "limit" || selectedOrderType.value === "stop_limit";
    });

    const needsStopPrice = computed(() => {
      return selectedOrderType.value === "stop_market" || selectedOrderType.value === "stop_limit";
    });

    // Progress bar calculations
    const leftProgressPercent = computed(() => {
      if (!needsLimitPrice.value) return 0;
      const current = parseFloat(limitPrice.value);
      const mid = midPrice.value;
      const bid = bidPrice.value;
      
      if (current < mid && bid < mid) {
        const maxDistance = mid - bid;
        const currentDistance = mid - current;
        return Math.min(100, (currentDistance / maxDistance) * 100);
      }
      return 0;
    });

    const rightProgressPercent = computed(() => {
      if (!needsLimitPrice.value) return 0;
      const current = parseFloat(limitPrice.value);
      const mid = midPrice.value;
      const ask = askPrice.value;
      
      if (current > mid && ask > mid) {
        const maxDistance = ask - mid;
        const currentDistance = current - mid;
        return Math.min(100, (currentDistance / maxDistance) * 100);
      }
      return 0;
    });

    // Stats calculations (mock data for now)
    const stats = computed(() => {
      // In a real implementation, this would come from positions data
      return {
        position: "0", // Current position quantity
        avgCost: "0.00", // Average cost basis
        dayPL: 0, // Day P&L
        totalPL: 0, // Total P&L
        dayChange: 0, // Day change percentage
        marketCap: "2.1T", // Market cap (mock)
        volume: "51.5M", // Volume (mock)
        buyingPower: "10,000", // Available buying power (mock)
      };
    });

    const canSubmit = computed(() => {
      return quantity.value > 0 && props.symbol;
    });

    // Methods
    const incrementQuantity = () => {
      quantity.value += 1;
    };

    const decrementQuantity = () => {
      if (quantity.value > 1) {
        quantity.value -= 1;
      }
    };

    const incrementPrice = () => {
      limitPrice.value = parseFloat((parseFloat(limitPrice.value) + 0.01).toFixed(2));
    };

    const decrementPrice = () => {
      limitPrice.value = parseFloat((parseFloat(limitPrice.value) - 0.01).toFixed(2));
    };

    const incrementStopPrice = () => {
      stopPrice.value = parseFloat((parseFloat(stopPrice.value) + 0.01).toFixed(2));
    };

    const decrementStopPrice = () => {
      stopPrice.value = parseFloat((parseFloat(stopPrice.value) - 0.01).toFixed(2));
    };

    const togglePriceLock = () => {
      priceLocked.value = !priceLocked.value;
    };

    const handleReviewSend = () => {
      const orderData = {
        symbol: props.symbol,
        side: orderSide.value,
        quantity: quantity.value,
        orderType: selectedOrderType.value,
        timeInForce: selectedTimeInForce.value,
        limitPrice: needsLimitPrice.value ? limitPrice.value : null,
        stopPrice: needsStopPrice.value ? stopPrice.value : null,
        estimatedCost: calculateEstimatedCost(),
        accountName: "Paper Trading Account", // This would come from account data
      };
      
      emit("review-send", orderData);
    };

    const handleClear = () => {
      quantity.value = 1; // Reset to default of 1
      orderSide.value = "buy";
      selectedOrderType.value = "market";
      selectedTimeInForce.value = "day";
      limitPrice.value = 0;
      stopPrice.value = 0;
      priceLocked.value = false;
      emit("clear-trade");
    };

    const calculateEstimatedCost = () => {
      let price = 0;
      
      if (selectedOrderType.value === "market") {
        price = orderSide.value === "buy" ? askPrice.value : bidPrice.value;
      } else if (needsLimitPrice.value) {
        price = limitPrice.value;
      } else {
        price = midPrice.value;
      }
      
      return price * quantity.value;
    };

    // Watch for price updates when not locked
    watch(
      midPrice,
      (newValue) => {
        if (!priceLocked.value && needsLimitPrice.value) {
          limitPrice.value = parseFloat(newValue.toFixed(2));
        }
      }
    );

    // Watch for order type changes
    watch(
      selectedOrderType,
      (newType) => {
        if (newType === "limit" || newType === "stop_limit") {
          limitPrice.value = parseFloat(midPrice.value.toFixed(2));
        }
        if (newType === "stop_market" || newType === "stop_limit") {
          stopPrice.value = parseFloat(midPrice.value.toFixed(2));
        }
      }
    );

    // Component cleanup system
    const cleanupComponentRegistrations = () => {
      // Unregister all symbols this component was using
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      
      // Clear local tracking
      registeredSymbols.clear();
    };

    // Watch for symbol changes to clean up old registrations
    watch(
      () => props.symbol,
      (newSymbol, oldSymbol) => {
        if (newSymbol !== oldSymbol) {
          // Unregister all current symbols
          cleanupComponentRegistrations();
        }
      },
      { immediate: true }
    );

    // Clean up when the component is unmounted
    onUnmounted(() => {
      cleanupComponentRegistrations();
    });

    return {
      // State
      quantity,
      orderSide,
      selectedOrderType,
      selectedTimeInForce,
      limitPrice,
      stopPrice,
      priceLocked,

      // Computed
      bidPrice,
      askPrice,
      midPrice,
      needsLimitPrice,
      needsStopPrice,
      leftProgressPercent,
      rightProgressPercent,
      stats,
      canSubmit,

      // Methods
      incrementQuantity,
      decrementQuantity,
      incrementPrice,
      decrementPrice,
      incrementStopPrice,
      decrementStopPrice,
      togglePriceLock,
      handleReviewSend,
      handleClear,
    };
  },
};
</script>

<style scoped>
.bottom-panel {
  position: relative;
  bottom: 0;
  left: 0;
  right: 0;
  background-color: var(--bg-secondary);
  border-top: 1px solid var(--border-primary);
  color: var(--text-primary);
  z-index: 1000;
  box-shadow: var(--shadow-lg);
  transform: translateY(100%);
  transition: var(--transition-slow);
  margin-bottom: var(--spacing-xs);
}

.bottom-panel.slide-up {
  transform: translateY(0);
}

/* Stats Row - Very Compact */
.stats-row {
  display: flex;
  justify-content: space-between;
  padding: 6px var(--spacing-lg);
  background-color: var(--bg-primary);
  border-bottom: 1px solid var(--border-primary);
}

.stat-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 60px;
}

.stat-label {
  font-size: 9px;
  color: var(--text-tertiary);
  font-weight: var(--font-weight-medium);
  margin-bottom: 2px;
}

.stat-value {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.stat-value.positive {
  color: var(--color-success);
}

.stat-value.negative {
  color: var(--color-danger);
}

/* Main Trading Row */
.trading-row {
  display: flex;
  align-items: center;
  padding: 16px 24px;
  background-color: var(--bg-secondary);
  gap: 24px;
}

/* Left Controls */
.left-controls {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 250px;
}

.order-side-section,
.quantity-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.control-label {
  font-size: 12px;
  color: var(--text-tertiary);
  font-weight: 500;
}

.side-buttons {
  display: flex;
  gap: 4px;
  width: 350px;
}

.side-btn {
  flex: 1;
  padding: 8px 16px;
  border: 1px solid var(--border-secondary);
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  transition: all 0.2s ease;
}

.buy-btn {
  background-color: var(--bg-quaternary);
  color: var(--text-secondary);
}

.buy-btn.active {
  background-color: var(--color-success);
  border-color: var(--color-success);
  color: white;
}

.sell-btn {
  background-color: var(--bg-quaternary);
  color: var(--text-secondary);
}

.sell-btn.active {
  background-color: var(--color-danger);
  border-color: var(--color-danger);
  color: white;
}

.side-btn:hover:not(.active) {
  background-color: var(--border-secondary);
  color: var(--text-primary);
}

.quantity-input-group {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 350px;
}

.qty-btn {
  background-color: var(--bg-quaternary);
  border: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  width: 40px;
  height: 32px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 16px;
  font-weight: 600;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.qty-btn:hover {
  background-color: var(--border-secondary);
  color: var(--text-primary);
}

.quantity-input {
  background-color: var(--bg-quaternary);
  border: 1px solid var(--border-secondary);
  color: var(--text-primary);
  padding: 8px 12px;
  border-radius: 4px;
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  flex: 1;
}

.quantity-input:focus {
  outline: none;
  border-color: var(--color-brand);
}

/* Hide number input arrows/spinners */
.quantity-input::-webkit-outer-spin-button,
.quantity-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.quantity-input[type="number"] {
  -moz-appearance: textfield;
  appearance: textfield;
}

/* Price Section */
.price-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.price-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.bid-section,
.ask-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  min-width: 100px;
}

.progress-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.price-label {
  font-size: 11px;
  color: var(--text-tertiary);
  font-weight: 500;
}

.dual-progress-bar {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 0;
}

.progress-bar {
  height: 4px;
  flex: 1;
  background-color: #333;
  border-radius: 2px;
  overflow: hidden;
  position: relative;
}

.progress-bar.left {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
}

.progress-bar.right {
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
}

.progress-fill-left {
  position: absolute;
  top: 0;
  right: 0;
  height: 100%;
  background-color: var(--color-success);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.progress-fill-right {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  background-color: var(--color-danger);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.center-indicator {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 0 8px;
  position: relative;
}

.center-indicator::before {
  content: "";
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translate(-50%, 4px);
  width: 2px;
  height: 8px;
  background-color: #ff6b35;
  border-radius: 1px;
}

.price-display {
  display: flex;
  align-items: center;
  gap: 6px;
}

.price-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.price-dot.bid {
  background-color: var(--color-success);
}

.price-dot.ask {
  background-color: var(--color-danger);
}

.price-text {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.price-text.highlight {
  color: #ff6b35;
  font-size: 14px;
}

/* Limit Price Row */
.limit-price-row {
  display: flex;
  align-items: center;
  gap: 12px;
  justify-content: center;
}

.limit-label {
  font-size: 12px;
  color: var(--text-tertiary);
  font-weight: 500;
  min-width: 80px;
}

.limit-input-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.price-btn {
  background-color: var(--bg-quaternary);
  border: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  width: 28px;
  height: 28px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.price-btn:hover {
  background-color: var(--border-secondary);
  color: var(--text-primary);
}

.lock-btn.locked {
  color: #ffd700;
  background-color: rgba(255, 215, 0, 0.1);
}

.lock-btn.locked:hover {
  background-color: rgba(255, 215, 0, 0.2);
}

/* Custom Lock Icon */
.lock-icon {
  position: relative;
  width: 14px;
  height: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.lock-body {
  width: 8px;
  height: 6px;
  background-color: currentColor;
  border-radius: 1px;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -25%);
  z-index: 2;
}

.lock-shackle {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -75%);
  width: 6px;
  height: 6px;
  border: 1.5px solid currentColor;
  border-bottom: none;
  border-radius: 3px 3px 0 0;
  background: transparent;
  transition: all 0.2s ease;
}

.lock-shackle.open {
  transform: translate(-50%, -75%) rotate(-30deg);
  transform-origin: bottom left;
}

.limit-input {
  background-color: var(--bg-quaternary);
  border: 1px solid var(--border-secondary);
  color: var(--text-primary);
  padding: 6px 12px;
  border-radius: 4px;
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  width: 100px;
}

.limit-input:focus {
  outline: none;
  border-color: var(--color-brand);
}

/* Hide number input arrows/spinners */
.limit-input::-webkit-outer-spin-button,
.limit-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.limit-input[type="number"] {
  -moz-appearance: textfield;
  appearance: textfield;
}

/* Right Controls */
.right-controls {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 300px;
}

.config-row {
  display: flex;
  gap: 16px;
}

.order-type-section,
.time-force-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.config-label {
  font-size: 12px;
  color: var(--text-tertiary);
  font-weight: 500;
}

.config-select {
  background-color: var(--bg-quaternary);
  border: 1px solid var(--border-secondary);
  color: var(--text-primary);
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  width: 100%;
  appearance: none;
  background-image: url("data:image/svg+xml;charset=UTF-8,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23ccc' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3e%3cpolyline points='6,9 12,15 18,9'%3e%3c/polyline%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 8px center;
  background-size: 16px;
  padding-right: 32px;
}

.config-select:hover {
  background-color: var(--border-secondary);
}

.config-select:focus {
  outline: none;
  border-color: var(--color-brand);
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.action-buttons {
  display: flex;
  gap: 8px;
}

.bracket-btn,
.clear-btn {
  background: none;
  border: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  padding: 10px 16px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
  flex: 1;
}

.bracket-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.clear-btn:hover {
  background-color: var(--border-secondary);
  color: var(--text-primary);
}

.review-btn {
  background-color: var(--color-brand);
  border: none;
  color: white;
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  flex: 2;
}

.review-btn:hover:not(:disabled) {
  background-color: var(--color-brand-hover);
  transform: translateY(-1px);
}

.review-btn:disabled {
  background-color: #555;
  color: #888;
  cursor: not-allowed;
}

/* Responsive */
@media (max-width: 768px) {
  .trading-row {
    flex-direction: column;
    gap: 16px;
  }

  .left-controls,
  .right-controls {
    min-width: auto;
    width: 100%;
  }

  .stats-row {
    flex-wrap: wrap;
    gap: 4px;
  }

  .config-row {
    flex-direction: column;
    gap: 12px;
  }

  .action-buttons {
    flex-direction: column;
    gap: 8px;
  }
}
</style>
