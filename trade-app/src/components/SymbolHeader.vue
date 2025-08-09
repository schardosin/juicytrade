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
import { computed, watch, onMounted, onUnmounted } from "vue";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";

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
    // Legacy props - kept for backward compatibility
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
    // New prop to enable smart price fetching
    enableSmartPricing: {
      type: Boolean,
      default: true,
    },
  },
  emits: ["trade-mode-changed"],
  setup(props) {
    // Component registration system
    const componentId = `SymbolHeader-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // Smart Market Data integration
    const { getStockPrice, getPreviousClose, getDebugInfo } = useSmartMarketData();

    // Single registration method per component to prevent double registration
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    // Get live price data when smart pricing is enabled and symbol is available
    const livePrice = computed(() => {
      if (!props.enableSmartPricing || !props.currentSymbol) {
        return null;
      }
      
      // Ensure symbol is registered for component tracking
      ensureSymbolRegistration(props.currentSymbol);
      
      return getStockPrice(props.currentSymbol);
    });

    // Get previous close price (cached data - fetched once per symbol)
    const previousClose = computed(() => {
      if (!props.enableSmartPricing || !props.currentSymbol) {
        return null;
      }
      return getPreviousClose(props.currentSymbol);
    });

    // Use live price if available, otherwise fall back to props
    const effectivePrice = computed(() => {
      const livePriceData = livePrice.value?.value;
      if (livePriceData?.price) {
        return livePriceData.price;
      }
      return props.currentPrice;
    });

    // Calculate price change using live price and cached previous close
    const effectivePriceChange = computed(() => {
      const current = effectivePrice.value;
      const prevClose = previousClose.value?.value;
      
      if (current && prevClose && typeof current === 'number' && typeof prevClose === 'number') {
        return current - prevClose;
      }
      
      // Fallback to prop value
      return props.priceChange;
    });

    const effectivePriceChangePercent = computed(() => {
      const current = effectivePrice.value;
      const prevClose = previousClose.value?.value;
      
      if (current && prevClose && typeof current === 'number' && typeof prevClose === 'number' && prevClose !== 0) {
        return ((current - prevClose) / prevClose) * 100;
      }
      
      // Fallback to prop value
      return props.priceChangePercent;
    });

    // Determine if we're showing live data
    const isShowingLiveData = computed(() => {
      return props.enableSmartPricing && livePrice.value?.value?.price != null;
    });

    // Computed properties for styling
    const priceChangeClass = computed(() => ({
      positive: effectivePriceChange.value > 0,
      negative: effectivePriceChange.value < 0,
      neutral: effectivePriceChange.value === 0,
    }));

    const marketStatusClass = computed(() => ({
      open: props.marketStatus === "Market Open",
      closed: props.marketStatus === "Market Closed",
      "pre-market": props.marketStatus === "Pre-Market",
      "after-hours": props.marketStatus === "After Hours",
    }));

    // Component cleanup system
    const cleanupComponentRegistrations = () => {
      console.log(`📝 SymbolHeader: Unregistering component ${componentId} with ${registeredSymbols.size} symbols`);
      
      // Unregister all symbols this component was using
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      
      // Clear local tracking
      registeredSymbols.clear();
    };

    // Watch for symbol changes to clean up old registrations
    watch(
      () => props.currentSymbol,
      (newSymbol, oldSymbol) => {
        if (newSymbol !== oldSymbol) {
          console.log(`📝 SymbolHeader: Symbol changed from ${oldSymbol} to ${newSymbol}. Cleaning up component registrations.`);
          
          // Unregister all current symbols
          cleanupComponentRegistrations();
        }
      },
      { immediate: true }
    );

    // Clean up when the component is unmounted
    onUnmounted(() => {
      console.log(`📝 SymbolHeader unmounted. Cleaning up component registrations.`);
      cleanupComponentRegistrations();
    });

    // Watch for live price updates
    watch(
      livePrice,
      (newPrice) => {
        if (newPrice?.value?.price) {
          // console.log(
          //   `💰 Live price update for ${
          //     props.currentSymbol
          //   }: $${newPrice.value.price.toFixed(2)}`
          // );
        }
      },
      { deep: true }
    );

    return {
      // Effective values (live or fallback)
      currentPrice: effectivePrice,
      priceChange: effectivePriceChange,
      priceChangePercent: effectivePriceChangePercent,

      // Status indicators
      isLivePrice: isShowingLiveData,

      // Styling
      priceChangeClass,
      marketStatusClass,

      // Debug (for development)
      livePrice,
      getDebugInfo,
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
  background-color: var(--color-brand);
  color: var(--text-primary);
}
</style>
