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
  padding: 16px 24px;
  background-color: #333333;
  border-bottom: 1px solid #444444;
}

.symbol-info {
  display: flex;
  align-items: center;
  gap: 24px;
}

.symbol-details h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #ffffff;
}

.symbol-meta {
  display: flex;
  gap: 12px;
  margin-top: 4px;
}

.company-name {
  color: #cccccc;
  font-size: 14px;
}

.exchange {
  color: #888888;
  font-size: 14px;
}

.price-info {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.current-price {
  display: flex;
  align-items: center;
  gap: 8px;
}

.price {
  font-size: 20px;
  font-weight: 600;
  color: #ffffff;
}

.change,
.change-percent {
  font-size: 14px;
  font-weight: 500;
}

.change.positive,
.change-percent.positive {
  color: #00c851;
}

.change.negative,
.change-percent.negative {
  color: #ff4444;
}

.change.neutral,
.change-percent.neutral {
  color: #cccccc;
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
  background-color: #00c851;
}

.status-indicator.closed {
  background-color: #ff4444;
}

.status-indicator.pre-market,
.status-indicator.after-hours {
  background-color: #ffbb33;
}

.status-text {
  font-size: 12px;
  color: #cccccc;
}

.trade-mode-selector {
  display: flex;
  align-items: center;
}

.mode-tabs {
  display: flex;
  background-color: #444444;
  border-radius: 6px;
  overflow: hidden;
}

.mode-tab {
  padding: 8px 16px;
  background: none;
  border: none;
  color: #cccccc;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mode-tab:hover {
  background-color: #555555;
  color: #ffffff;
}

.mode-tab.active {
  background-color: #007bff;
  color: #ffffff;
}
</style>
