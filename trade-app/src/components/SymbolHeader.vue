<template>
  <div class="symbol-header">
    <div class="symbol-info">
      <div class="symbol-details">
        <h2 class="symbol-name">{{ currentSymbol }}</h2>
        <div class="symbol-meta">
          <span class="company-name">{{ companyName }}</span>
          <span class="exchange">{{ exchange }}</span>
        </div>
      </div>
      <div class="price-info">
        <div class="current-price">
          <span class="price">${{ currentPrice?.toFixed(2) || "--" }}</span>
          <span class="change" :class="priceChangeClass">
            {{ priceChange >= 0 ? "+" : ""
            }}{{ priceChange?.toFixed(2) || "--" }}
          </span>
          <span class="change-percent" :class="priceChangeClass">
            ({{ priceChangePercent >= 0 ? "+" : ""
            }}{{ priceChangePercent?.toFixed(2) || "--" }}%)
          </span>
        </div>
        <div class="market-status">
          <span class="status-indicator" :class="marketStatusClass"></span>
          <span class="status-text">{{ marketStatus }}</span>
        </div>
      </div>
    </div>

    <!-- Trade Mode Selector -->
    <div v-if="showTradeMode" class="trade-mode-selector">
      <div class="mode-tabs">
        <button
          v-for="mode in tradeModes"
          :key="mode.value"
          :class="['mode-tab', { active: selectedTradeMode === mode.value }]"
          @click="$emit('trade-mode-changed', mode.value)"
        >
          {{ mode.label }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from "vue";

export default {
  name: "SymbolHeader",
  props: {
    currentSymbol: {
      type: String,
      default: "",
    },
    companyName: {
      type: String,
      default: "",
    },
    exchange: {
      type: String,
      default: "",
    },
    currentPrice: {
      type: Number,
      default: null,
    },
    priceChange: {
      type: Number,
      default: 0,
    },
    priceChangePercent: {
      type: Number,
      default: 0,
    },
    isLivePrice: {
      type: Boolean,
      default: false,
    },
    marketStatus: {
      type: String,
      default: "Market Closed",
    },
    selectedTradeMode: {
      type: String,
      default: "options",
    },
    tradeModes: {
      type: Array,
      default: () => [
        { label: "Options", value: "options" },
        { label: "Shares", value: "shares" },
        { label: "Futures", value: "futures" },
      ],
    },
    showTradeMode: {
      type: Boolean,
      default: true,
    },
  },
  emits: ["trade-mode-changed"],
  setup(props) {
    // Computed properties
    const priceChangeClass = computed(() => ({
      positive: props.priceChange > 0,
      negative: props.priceChange < 0,
      neutral: props.priceChange === 0,
    }));

    const marketStatusClass = computed(() => ({
      open: props.marketStatus === "Market Open",
      closed: props.marketStatus === "Market Closed",
      "pre-market": props.marketStatus === "Pre-Market",
      "after-hours": props.marketStatus === "After Hours",
    }));

    return {
      priceChangeClass,
      marketStatusClass,
    };
  },
};
</script>

<style scoped>
.symbol-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg) var(--spacing-xl);
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-primary);
}

.symbol-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-xl);
}

.symbol-details h2 {
  margin: 0;
  font-size: var(--font-size-xxl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.symbol-meta {
  display: flex;
  gap: var(--spacing-md);
  margin-top: var(--spacing-xs);
}

.company-name {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.exchange {
  color: var(--text-tertiary);
  font-size: var(--font-size-md);
}

.price-info {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--spacing-xs);
}

.current-price {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.price {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.change,
.change-percent {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
}

.change.positive,
.change-percent.positive {
  color: var(--color-success);
}

.change.negative,
.change-percent.negative {
  color: var(--color-danger);
}

.change.neutral,
.change-percent.neutral {
  color: var(--text-secondary);
}

.market-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-indicator.open {
  background-color: var(--color-success);
}

.status-indicator.closed {
  background-color: var(--color-danger);
}

.status-indicator.pre-market,
.status-indicator.after-hours {
  background-color: var(--color-warning);
}

.status-text {
  font-size: var(--font-size-base);
  color: var(--text-secondary);
}

.trade-mode-selector {
  display: flex;
  align-items: center;
}

.mode-tabs {
  display: flex;
  background-color: var(--bg-tertiary);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.mode-tab {
  padding: var(--spacing-sm) var(--spacing-lg);
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-normal);
}

.mode-tab:hover {
  background-color: var(--bg-quaternary);
  color: var(--text-primary);
}

.mode-tab.active {
  background-color: var(--color-primary);
  color: var(--text-primary);
}
</style>
