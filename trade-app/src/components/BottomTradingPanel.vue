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
          <button class="ctrl-btn" @click="decrementQuantity">⬇</button>
          <button class="ctrl-btn" @click="incrementQuantity">⬆</button>
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
          <button class="ctrl-btn">↔</button>
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
          <div
            v-for="(leg, index) in orderLegs"
            :key="index"
            class="order-leg"
            :class="{ selected: selectedLegs.includes(leg.symbol) }"
            @click="toggleLegSelection(leg.symbol)"
          >
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
                class="price-input"
              />
              <button class="price-btn" @click="decrementPrice">-</button>
              <button class="price-btn" @click="incrementPrice">+</button>
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
                >{{ Math.abs(parseFloat(bidPrice)).toFixed(2) }}
                {{ parseFloat(bidPrice) >= 0 ? "cr" : "db" }}</span
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
                  :class="parseFloat(midPrice) >= 0 ? 'credit' : 'debit'"
                  :style="{ width: leftProgressPercent + '%' }"
                ></div>
              </div>
              <!-- Center Indicator -->
              <div class="center-indicator">
                <span
                  class="price-text"
                  :class="
                    parseFloat(midPrice) >= 0
                      ? 'highlight-credit'
                      : 'highlight-debit'
                  "
                  >{{ Math.abs(parseFloat(midPrice)).toFixed(2) }}
                  {{ parseFloat(midPrice) >= 0 ? "cr" : "db" }}</span
                >
              </div>
              <!-- Right Progress Bar -->
              <div class="progress-bar right">
                <div
                  class="progress-fill-right"
                  :class="parseFloat(midPrice) >= 0 ? 'credit' : 'debit'"
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
                >{{ Math.abs(parseFloat(askPrice)).toFixed(2) }}
                {{ parseFloat(askPrice) >= 0 ? "cr" : "db" }}</span
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
import {
  calculateMultiLegProfitLoss,
  calculateBuyingPowerEffect,
  formatCurrency,
  getCreditDebitInfo,
  calculateGreeks,
} from "../services/optionsCalculator.js";

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
    const selectedLegs = ref([]);
    const priceLocked = ref(false);

    const getOptionPrice = (selection, priceType) => {
      // First try to find in the passed optionsData (for backward compatibility)
      let option = optionsData.value.find(
        (opt) => opt.symbol === selection.symbol
      );

      // If not found and we have access to the parent's flattened data, try that
      if (!option && props.flattenedOptionsData) {
        option = props.flattenedOptionsData.find(
          (opt) => opt.symbol === selection.symbol
        );
      }

      if (!option) return 0;

      if (priceType === "bid") return option.bid || 0;
      if (priceType === "ask") return option.ask || 0;
      if (priceType === "mid")
        return ((option.bid || 0) + (option.ask || 0)) / 2;

      // Default to natural price for the side
      return selection.side === "buy" ? option.ask || 0 : option.bid || 0;
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

    // Calculate comprehensive P&L analysis using centralized calculator
    const profitLossAnalysis = computed(() => {
      return calculateMultiLegProfitLoss(
        selectedOptions.value,
        optionsData.value,
        props.underlyingPrice
      );
    });

    // Calculate Greeks
    const greeks = computed(() => {
      if (profitLossAnalysis.value.positions.length > 0) {
        return calculateGreeks(profitLossAnalysis.value.positions);
      }
      return { delta: 0, theta: 0 };
    });

    const stats = computed(() => {
      const analysis = profitLossAnalysis.value;
      const creditDebitInfo = getCreditDebitInfo(analysis.netPremium);
      const bpEffect = calculateBuyingPowerEffect(analysis);

      return {
        pop: 59, // Placeholder - would need probability calculation
        ext: formatCurrency(Math.abs(analysis.maxProfit), 0),
        p50: null, // Placeholder
        delta: greeks.value.delta,
        theta: greeks.value.theta,
        maxProfit: formatCurrency(Math.abs(analysis.maxProfit), 0),
        maxLoss: formatCurrency(Math.abs(analysis.maxLoss), 0),
        bpEff: formatCurrency(bpEffect, 0),
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
          strike: (option.strike_price || option.strike)?.toString() || "-",
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

    const incrementPrice = () => {
      limitPrice.value = parseFloat(
        (parseFloat(limitPrice.value) + 0.01).toFixed(2)
      );
    };

    const decrementPrice = () => {
      limitPrice.value = parseFloat(
        (parseFloat(limitPrice.value) - 0.01).toFixed(2)
      );
    };

    const toggleLegSelection = (symbol) => {
      const index = selectedLegs.value.indexOf(symbol);
      if (index >= 0) {
        // Remove from selection
        selectedLegs.value.splice(index, 1);
      } else {
        // Add to selection
        selectedLegs.value.push(symbol);
      }
    };

    const incrementQuantity = () => {
      // If legs are selected, only affect selected legs; otherwise affect all legs
      const legsToUpdate =
        selectedLegs.value.length > 0
          ? props.selectedOptions.filter((option) =>
              selectedLegs.value.includes(option.symbol)
            )
          : props.selectedOptions;

      legsToUpdate.forEach((option) => {
        if (option.quantity < 10) {
          emit("update-leg-quantity", {
            symbol: option.symbol,
            quantity: option.quantity + 1,
          });
        }
      });
    };

    const decrementQuantity = () => {
      // If legs are selected, only affect selected legs; otherwise affect all legs
      const legsToUpdate =
        selectedLegs.value.length > 0
          ? props.selectedOptions.filter((option) =>
              selectedLegs.value.includes(option.symbol)
            )
          : props.selectedOptions;

      legsToUpdate.forEach((option) => {
        if (option.quantity > 1) {
          emit("update-leg-quantity", {
            symbol: option.symbol,
            quantity: option.quantity - 1,
          });
        }
      });
    };

    const togglePriceLock = () => {
      priceLocked.value = !priceLocked.value;
    };

    const handleReviewSend = () => {
      // Get the expiry date from the first selected option (all should have the same expiry)
      const expiry =
        props.selectedOptions.length > 0
          ? props.selectedOptions[0].expiry
          : null;

      // Get comprehensive analysis from centralized calculator
      const analysis = profitLossAnalysis.value;
      const greeksData = greeks.value;

      // Calculate broker limit price: negative for credits, positive for debits
      const brokerLimitPrice =
        analysis.netPremium >= 0
          ? -Math.abs(limitPrice.value) // Credit: negative
          : Math.abs(limitPrice.value); // Debit: positive

      const orderData = {
        symbol: props.symbol,
        expiry: expiry,
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
        limitPrice: brokerLimitPrice,
        displayLimitPrice: limitPrice.value, // Keep UI display price for confirmation dialog
        netPremium: analysis.netPremium,
        underlyingPrice: props.underlyingPrice,
        accountName: "Paper Trading Account",
        maxReward: Math.abs(analysis.maxProfit),
        maxRisk: Math.abs(analysis.maxLoss),
        netDelta: greeksData.delta,
        netTheta: greeksData.theta,
      };
      emit("review-send", orderData);
    };

    watch(
      netPremium,
      (newValue) => {
        if (!priceLocked.value) {
          // Always display limit price as positive in UI
          limitPrice.value = parseFloat(Math.abs(newValue).toFixed(2));
        }
      },
      { immediate: true }
    );

    watch(
      () => props.selectedOptions,
      (newOptions, oldOptions) => {
        // Simple approach: unlock on ANY change if currently locked
        if (priceLocked.value) {
          console.log(
            "🔓 Auto-unlocking because price was locked and options changed"
          );
          priceLocked.value = false;
        }

        if (!priceLocked.value) {
          // Always display limit price as positive in UI
          limitPrice.value = parseFloat(Math.abs(netPremium.value).toFixed(2));
        }
      },
      { deep: true, immediate: true }
    );

    return {
      limitPrice,
      selectedOrderType,
      selectedTimeInForce,
      selectedLegs,
      priceLocked,
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
      incrementPrice,
      decrementPrice,
      incrementQuantity,
      decrementQuantity,
      toggleLegSelection,
      togglePriceLock,
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

.unit {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

/* Controls Row - Responsive like Tasty Trade */
.controls-row {
  display: flex;
  padding: var(--spacing-sm) var(--spacing-lg);
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-primary);
  gap: var(--spacing-xs);
}

.control-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: var(--spacing-xs);
}

.control-label {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-medium);
  text-align: center;
  white-space: nowrap;
}

.control-buttons {
  display: flex;
  gap: 2px;
  width: 100%;
  height: 32px;
}

.ctrl-btn {
  background-color: var(--bg-quaternary);
  border: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  flex: 1;
  height: 100%;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  transition: var(--transition-normal);
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
}

.ctrl-btn:hover {
  background-color: var(--border-secondary);
  color: var(--text-primary);
}

/* For single button groups (like Swap), the button should still fill the space */
.control-group .control-buttons .ctrl-btn:only-child {
  flex: 1;
}

/* Content Row - Compact */
.content-row {
  display: flex;
  padding: var(--spacing-md) var(--spacing-lg);
  gap: var(--spacing-xl);
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-primary);
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
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.order-leg:hover {
  background-color: #333333;
  border-color: #444444;
}

.order-leg.selected {
  background-color: rgba(255, 215, 0, 0.1);
  border-color: #ffd700;
  box-shadow: 0 0 0 1px rgba(255, 215, 0, 0.3);
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
  flex: 1;
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
  display: flex;
  align-items: center;
  justify-content: center;
}

.price-btn:hover {
  background-color: #444;
  color: #fff;
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

.price-input {
  background-color: #333;
  border: 1px solid #444;
  color: #fff;
  padding: 6px 12px;
  border-radius: 4px;
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  flex: 1;
  min-width: 80px;
}

.price-input:focus {
  outline: none;
  border-color: #007bff;
}

/* Hide number input arrows/spinners */
.price-input::-webkit-outer-spin-button,
.price-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.price-input[type="number"] {
  -moz-appearance: textfield;
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

.price-text.highlight-credit {
  color: #ff6b35;
  font-size: 14px;
}

.price-text.highlight-debit {
  color: #00c851;
  font-size: 14px;
}

.review-btn {
  background-color: #4ecdc4;
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
  background-color: #3db8b0;
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
  border-radius: 2px;
  transition: width 0.3s ease;
}

.progress-fill-right {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s ease;
}

/* Dynamic colors based on credit/debit */
.progress-fill-left.credit,
.progress-fill-right.credit {
  background-color: #ff6b35;
}

.progress-fill-left.debit,
.progress-fill-right.debit {
  background-color: #00c851;
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
