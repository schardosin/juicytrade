<template>
  <!-- Compact Bottom Trading Panel -->
  <div v-if="visible" class="bottom-panel slide-up">
    <!-- Top Stats Row -->
    <div class="stats-row">
      <div class="stat-group">
        <span class="stat-label">POP</span>
        <span class="stat-value">{{ stats.pop }}%</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">EXT</span>
        <span class="stat-value">{{ stats.ext }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">P50</span>
        <span class="stat-value">{{ stats.p50 || "-" }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Delta</span>
        <span class="stat-value">{{ stats.delta }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Theta</span>
        <span class="stat-value negative">{{ stats.theta }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Max Profit</span>
        <span class="stat-value positive">{{ stats.maxProfit }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">Max Loss</span>
        <span class="stat-value negative">{{ stats.maxLoss }}</span>
      </div>
      <div class="stat-group">
        <span class="stat-label">BP Eff.</span>
        <span class="stat-value"
          >{{ stats.bpEff }} <span class="unit">db</span></span
        >
      </div>
    </div>

    <!-- Controls Row -->
    <div class="controls-row">
      <div class="control-group">
        <span class="control-label">Strike</span>
        <div class="control-buttons">
          <button class="ctrl-btn">⬆</button>
          <button class="ctrl-btn">⬇</button>
        </div>
      </div>
      <div class="control-group">
        <span class="control-label">Width</span>
        <div class="control-buttons">
          <button class="ctrl-btn">⬆</button>
          <button class="ctrl-btn">⬇</button>
        </div>
      </div>
      <div class="control-group">
        <span class="control-label">Quantity</span>
        <div class="control-buttons">
          <button class="ctrl-btn">⬇</button>
          <button class="ctrl-btn">⬆</button>
        </div>
      </div>
      <div class="control-group">
        <span class="control-label">Expiration</span>
        <div class="control-buttons">
          <button class="ctrl-btn">↩</button>
          <button class="ctrl-btn">↪</button>
        </div>
      </div>
      <div class="control-group">
        <span class="control-label">Swap</span>
        <div class="control-buttons">
          <button class="ctrl-btn">🔄</button>
        </div>
      </div>
      <div class="control-group">
        <span class="control-label">Undo/Redo</span>
        <div class="control-buttons">
          <button class="ctrl-btn">↶</button>
          <button class="ctrl-btn">↷</button>
        </div>
      </div>
    </div>

    <!-- Main Content Row -->
    <div class="content-row">
      <!-- Order Details -->
      <div class="order-section">
        <div class="section-header">
          <div class="section-title">Order Details</div>
          <div class="symbol-display">{{ symbol }}</div>
        </div>
        <div class="order-legs">
          <div v-for="(leg, index) in orderLegs" :key="index" class="order-leg">
            <span class="leg-qty">{{ leg.quantity }}</span>
            <span class="leg-date">{{ leg.date }}</span>
            <span class="leg-strike">{{ leg.strike }}</span>
            <span class="leg-type">{{ leg.type }}</span>
            <span class="leg-action" :class="leg.action.toLowerCase()">{{
              leg.action
            }}</span>
            <span class="leg-price">${{ leg.price }}</span>
          </div>
        </div>
      </div>

      <!-- Trading Controls -->
      <div class="trading-section">
        <!-- Price Controls Row -->
        <div class="price-controls-row">
          <div class="limit-price-section">
            <span class="price-label">Limit Price</span>
            <div class="price-input-group">
              <input
                v-model="limitPrice"
                type="number"
                step="0.01"
                class="price-input"
              />
              <button class="price-btn">-</button>
              <button class="price-btn">+</button>
            </div>
          </div>

          <div class="order-type-section">
            <span class="config-label">Order Type</span>
            <div class="config-value">{{ selectedOrderType }} ></div>
          </div>

          <div class="time-force-section">
            <span class="config-label">Time in Force</span>
            <div class="config-value">{{ selectedTimeInForce }} ></div>
          </div>
        </div>

        <!-- Progress Bar Row with Review Button -->
        <div class="progress-row">
          <div class="bid-section">
            <span class="price-label">BID (NAT)</span>
            <div class="price-display">
              <span class="price-dot bid"></span>
              <span class="price-text"
                >{{ parseFloat(bidPrice).toFixed(2) }} cr</span
              >
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
                <span class="price-text highlight"
                  >{{ parseFloat(midPrice).toFixed(2) }} cr</span
                >
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
            <span class="price-label">ASK (OPP)</span>
            <div class="price-display">
              <span class="price-dot ask"></span>
              <span class="price-text"
                >{{ parseFloat(askPrice).toFixed(2) }} cr</span
              >
            </div>
          </div>

          <button
            class="review-btn"
            @click="handleReviewSend"
            :disabled="!canSubmit"
          >
            Review & Send
          </button>

          <button class="clear-btn" @click="handleCancel">Clear Trade</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, toRefs } from "vue";

export default {
  name: "BottomTradingPanel",
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
    selectedOptions: {
      type: Array,
      default: () => [],
    },
    optionsData: {
      type: Array,
      default: () => [],
    },
    symbol: {
      type: String,
      default: "SPX",
    },
    underlyingPrice: {
      type: Number,
      default: null,
    },
  },
  emits: ["clear-trade", "review-send", "update-leg-quantity"],
  setup(props, { emit }) {
    const { selectedOptions, optionsData } = toRefs(props);

    const limitPrice = ref(0);
    const selectedOrderType = ref("limit");
    const selectedTimeInForce = ref("day");

    const getOptionPrice = (selection, priceType) => {
      const option = optionsData.value.find(
        (opt) => opt.symbol === selection.symbol
      );
      if (!option) return 0;

      if (priceType === "bid") return option.bid;
      if (priceType === "ask") return option.ask;
      if (priceType === "mid") return (option.bid + option.ask) / 2;

      // Default to natural price for the side
      return selection.side === "buy" ? option.ask : option.bid;
    };

    const netPremium = computed(() => {
      let total = 0;
      selectedOptions.value.forEach((selection) => {
        const price = getOptionPrice(selection, "mid"); // Use mid for a neutral estimate
        const premium = selection.side === "buy" ? -price : price;
        total += premium * selection.quantity;
      });
      return total;
    });

    const bidPrice = computed(() => {
      let total = 0;
      selectedOptions.value.forEach((selection) => {
        const price =
          selection.side === "buy"
            ? getOptionPrice(selection, "ask")
            : getOptionPrice(selection, "bid");
        const premium = selection.side === "buy" ? -price : price;
        total += premium * selection.quantity;
      });
      return total.toFixed(2);
    });

    const askPrice = computed(() => {
      let total = 0;
      selectedOptions.value.forEach((selection) => {
        const price =
          selection.side === "buy"
            ? getOptionPrice(selection, "bid")
            : getOptionPrice(selection, "ask");
        const premium = selection.side === "buy" ? -price : price;
        total += premium * selection.quantity;
      });
      return total.toFixed(2);
    });

    const midPrice = computed(() => {
      return (
        (parseFloat(bidPrice.value) + parseFloat(askPrice.value)) /
        2
      ).toFixed(2);
    });

    const stats = computed(() => {
      const maxProfit =
        netPremium.value > 0 ? netPremium.value * 100 : Infinity;
      const maxLoss = netPremium.value < 0 ? netPremium.value * 100 : -Infinity;

      return {
        pop: 59, // Placeholder
        ext: (netPremium.value * 100).toFixed(0),
        p50: null, // Placeholder
        delta: 0, // Placeholder
        theta: 0, // Placeholder
        maxProfit: maxProfit.toFixed(0),
        maxLoss: maxLoss.toFixed(0),
        bpEff: (Math.abs(netPremium.value) * 100).toFixed(0),
      };
    });

    const orderLegs = computed(() => {
      return props.selectedOptions.map((option) => {
        const liveOption = props.optionsData.find(
          (o) => o.symbol === option.symbol
        );

        // Use natural price: bid when selling, ask when buying
        let price = 0;
        if (liveOption) {
          price = option.side === "buy" ? liveOption.ask : liveOption.bid;
        }

        return {
          symbol: option.symbol,
          quantity: option.quantity,
          date: formatDate(option.expiry),
          strike: option.strike?.toString() || "-",
          type: option.type?.charAt(0).toUpperCase() || "P",
          action: option.side === "buy" ? "BTO" : "STO",
          price: price.toFixed(2),
        };
      });
    });

    const canSubmit = computed(() => {
      return props.selectedOptions.length > 0;
    });

    const priceProgressPercent = computed(() => {
      const bid = parseFloat(bidPrice.value);
      const ask = parseFloat(askPrice.value);
      const current = parseFloat(limitPrice.value);
      const mid = parseFloat(midPrice.value);

      if (bid >= ask) return 50;

      // Use mid price as the center point (50%)
      const progress = ((current - mid) / (ask - bid)) * 100 + 50;
      return Math.max(0, Math.min(100, progress));
    });

    const leftProgressPercent = computed(() => {
      const bid = parseFloat(bidPrice.value);
      const ask = parseFloat(askPrice.value);
      const current = parseFloat(limitPrice.value);
      const mid = parseFloat(midPrice.value);

      if (bid >= ask) return 0;

      // Show green bar when current price is below mid
      if (current < mid) {
        const maxDistance = mid - bid;
        const currentDistance = mid - current;
        return Math.min(100, (currentDistance / maxDistance) * 100);
      }
      return 0;
    });

    const rightProgressPercent = computed(() => {
      const bid = parseFloat(bidPrice.value);
      const ask = parseFloat(askPrice.value);
      const current = parseFloat(limitPrice.value);
      const mid = parseFloat(midPrice.value);

      if (bid >= ask) return 0;

      // Show red bar when current price is above mid
      if (current > mid) {
        const maxDistance = ask - mid;
        const currentDistance = current - mid;
        return Math.min(100, (currentDistance / maxDistance) * 100);
      }
      return 0;
    });

    const formatDate = (date) => {
      if (!date) return "Jul 11";

      // Handle different date formats
      let dateObj;
      if (typeof date === "string") {
        // Handle YYYY-MM-DD format - parse as UTC to avoid timezone issues
        if (date.match(/^\d{4}-\d{2}-\d{2}$/)) {
          const [year, month, day] = date.split("-").map(Number);
          dateObj = new Date(Date.UTC(year, month - 1, day));
        } else {
          dateObj = new Date(date);
        }
      } else if (date instanceof Date) {
        dateObj = date;
      } else {
        // If it's a timestamp or other format
        dateObj = new Date(date);
      }

      // Check if date is valid
      if (isNaN(dateObj.getTime())) {
        console.warn("Invalid date:", date);
        return "Jul 11";
      }

      return dateObj.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        timeZone: "UTC",
      });
    };

    const handleCancel = () => {
      emit("clear-trade");
    };

    const handleReviewSend = () => {
      const orderData = {
        symbol: props.symbol,
        legs: props.selectedOptions.map((leg) => {
          const liveOption = props.optionsData.find(
            (o) => o.symbol === leg.symbol
          );
          return {
            symbol: leg.symbol,
            displaySymbol: leg.symbol,
            side: leg.side,
            quantity: leg.quantity,
            ratio_qty: leg.quantity,
            type: leg.type || "Call",
            strike: leg.strike,
            date: formatDate(leg.expiry),
            expiry: leg.expiry,
            price: liveOption ? (liveOption.bid + liveOption.ask) / 2 : 0,
          };
        }),
        orderType: selectedOrderType.value,
        timeInForce: selectedTimeInForce.value,
        limitPrice: limitPrice.value,
        netPremium: netPremium.value,
        underlyingPrice: props.underlyingPrice,
        accountName: "Paper Trading Account",
        maxReward: Math.abs(netPremium.value) * 100,
        maxRisk: Math.abs(netPremium.value) * 100,
        netDelta: 0, // Placeholder - would need Greeks calculation
        netTheta: 0, // Placeholder - would need Greeks calculation
      };
      emit("review-send", orderData);
    };

    watch(
      netPremium,
      (newValue) => {
        limitPrice.value = parseFloat(newValue.toFixed(2));
      },
      { immediate: true }
    );

    watch(
      () => props.selectedOptions,
      () => {
        limitPrice.value = parseFloat(netPremium.value.toFixed(2));
      },
      { deep: true, immediate: true }
    );

    return {
      limitPrice,
      selectedOrderType,
      selectedTimeInForce,
      stats,
      orderLegs,
      bidPrice,
      midPrice,
      askPrice,
      canSubmit,
      priceProgressPercent,
      leftProgressPercent,
      rightProgressPercent,
      netPremium,
      handleCancel,
      handleReviewSend,
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
  background-color: #1a1a1a;
  border-top: 1px solid #333;
  color: #ffffff;
  z-index: 1000;
  box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.3);
  transform: translateY(100%);
  transition: transform 0.3s ease-out;
  margin-bottom: 5px;
}

.bottom-panel.slide-up {
  transform: translateY(0);
}

/* Stats Row - Very Compact */
.stats-row {
  display: flex;
  justify-content: space-between;
  padding: 6px 16px;
  background-color: #2a2a2a;
  border-bottom: 1px solid #333;
}

.stat-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 60px;
}

.stat-label {
  font-size: 9px;
  color: #888;
  font-weight: 500;
  margin-bottom: 2px;
}

.stat-value {
  font-size: 11px;
  font-weight: 600;
  color: #fff;
}

.stat-value.positive {
  color: #00c851;
}

.stat-value.negative {
  color: #ff4444;
}

.unit {
  font-size: 8px;
  color: #888;
}

/* Controls Row - Compact */
.controls-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 16px;
  background-color: #1a1a1a;
  border-bottom: 1px solid #333;
}

.control-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.control-label {
  font-size: 10px;
  color: #888;
  font-weight: 500;
}

.control-buttons {
  display: flex;
  gap: 2px;
}

.ctrl-btn {
  background-color: #333;
  border: 1px solid #444;
  color: #ccc;
  width: 40px;
  height: 32px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ctrl-btn:hover {
  background-color: #444;
  color: #fff;
}

/* Content Row - Compact */
.content-row {
  display: flex;
  padding: 12px 16px;
  gap: 24px;
  background-color: #1a1a1a;
  border-bottom: 1px solid #333;
}

.order-section {
  flex: 1;
  max-width: 400px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.section-title {
  font-size: 12px;
  color: #ccc;
  font-weight: 500;
}

.section-header .symbol-display {
  font-size: 16px;
  color: #fff;
  font-weight: 600;
}

.order-legs {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.order-leg {
  display: grid;
  grid-template-columns: 30px 50px 50px 20px 40px 60px;
  gap: 8px;
  align-items: center;
  padding: 6px 8px;
  background-color: #2a2a2a;
  border-radius: 4px;
  font-size: 11px;
}

.leg-qty {
  font-weight: 600;
  color: #fff;
}

.leg-date {
  color: #ccc;
  font-size: 10px;
}

.leg-strike {
  color: #fff;
  font-weight: 500;
}

.leg-type {
  color: #888;
  font-weight: 600;
  text-align: center;
}

.leg-action {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 3px;
  text-align: center;
}

.leg-action.bto {
  background-color: rgba(0, 200, 81, 0.2);
  color: #00c851;
}

.leg-action.sto {
  background-color: rgba(255, 68, 68, 0.2);
  color: #ff4444;
}

.leg-price {
  color: #fff;
  font-weight: 500;
  text-align: right;
}

.trading-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.symbol-display {
  font-size: 18px;
  color: #fff;
  font-weight: 600;
  text-align: center;
}

.price-controls {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.price-label {
  font-size: 11px;
  color: #888;
  font-weight: 500;
}

.price-input-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.price-btn {
  background-color: #333;
  border: 1px solid #444;
  color: #ccc;
  width: 28px;
  height: 28px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
}

.price-btn:hover {
  background-color: #444;
  color: #fff;
}

.price-input {
  background-color: #333;
  border: 1px solid #444;
  color: #fff;
  padding: 6px 12px;
  border-radius: 4px;
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  width: 80px;
}

.price-input:focus {
  outline: none;
  border-color: #007bff;
}

.order-config {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.config-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.config-label {
  font-size: 11px;
  color: #888;
  font-weight: 500;
}

.config-value {
  background-color: #333;
  border: 1px solid #444;
  color: #fff;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

/* New Price Controls Row Layout */
.price-controls-row {
  display: flex;
  align-items: flex-start;
  gap: 24px;
}

.limit-price-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.order-type-section,
.time-force-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 120px;
}

.order-type-section .config-value,
.time-force-section .config-value {
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.order-type-section .config-value:hover,
.time-force-section .config-value:hover {
  background-color: #444;
}

/* Progress Bar */
.price-progress-bar {
  margin-top: 6px;
}

.progress-track {
  position: relative;
  width: 100%;
  height: 4px;
  background-color: #333;
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  background: linear-gradient(90deg, #00c851 0%, #ff6b35 50%, #ff4444 100%);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.progress-indicator {
  position: absolute;
  top: -2px;
  width: 8px;
  height: 8px;
  background-color: #ff6b35;
  border: 2px solid #1a1a1a;
  border-radius: 50%;
  transform: translateX(-50%);
  transition: left 0.3s ease;
}

/* Price Bar - Compact */
.price-bar {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  background-color: #2a2a2a;
  gap: 16px;
}

.price-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.price-item.mid {
  flex: 1;
}

.price-label {
  font-size: 10px;
  color: #888;
  font-weight: 500;
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
  background-color: #00c851;
}

.price-dot.mid {
  background-color: #ff6b35;
}

.price-dot.ask {
  background-color: #ff4444;
}

.price-text {
  font-size: 12px;
  font-weight: 600;
  color: #fff;
}

.price-text.highlight {
  color: #ff6b35;
  font-size: 14px;
}

.review-btn {
  background-color: #ff6b35;
  border: none;
  color: #fff;
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.review-btn:hover:not(:disabled) {
  background-color: #e55a2b;
  transform: translateY(-1px);
}

.review-btn:disabled {
  background-color: #555;
  color: #888;
  cursor: not-allowed;
}

.clear-btn {
  background: none;
  border: 1px solid #444;
  color: #ccc;
  padding: 10px 16px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
}

.clear-btn:hover {
  background-color: #333;
  color: #fff;
}

/* Progress Row Layout */
.progress-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 12px;
}

.bid-section,
.ask-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  min-width: 80px;
}

.progress-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
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
  background-color: #00c851;
  border-radius: 2px;
  transition: width 0.3s ease;
}

.progress-fill-right {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  background-color: #ff4444;
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

/* Responsive */
@media (max-width: 768px) {
  .content-row {
    flex-direction: column;
    gap: 16px;
  }

  .order-section {
    max-width: none;
  }

  .stats-row {
    flex-wrap: wrap;
    gap: 4px;
  }

  .controls-row {
    flex-wrap: wrap;
    gap: 8px;
  }

  .price-bar {
    flex-wrap: wrap;
    gap: 12px;
  }
}
</style>
