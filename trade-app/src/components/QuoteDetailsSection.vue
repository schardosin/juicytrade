<template>
  <div class="quote-details">
    <div class="quote-grid">
      <!-- Live Quote Section -->
      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Current</span>
          <span class="value" :class="priceChangeClass">{{ formatPrice(currentPrice) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Change</span>
          <span class="value" :class="priceChangeClass">{{ formatPriceChange(dailyChange, dailyChangePercent) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Bid</span>
          <span class="value bid-price">{{ formatPrice(liveQuote?.bid) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Ask</span>
          <span class="value ask-price">{{ formatPrice(liveQuote?.ask) }}</span>
        </div>
      </div>

      <!-- Today's Trading -->
      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Open</span>
          <span class="value">{{ formatPrice(todayData?.open) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Prev Close</span>
          <span class="value">{{ formatPrice(previousClose) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">High</span>
          <span class="value">{{ formatPrice(todayData?.high) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Low</span>
          <span class="value">{{ formatPrice(todayData?.low) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Volume</span>
          <span class="value">{{ formatVolume(todayData?.volume) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Avg Volume</span>
          <span class="value">{{ formatVolume(averageVolume) }}</span>
        </div>
      </div>

      <!-- 52-Week Range -->
      <div v-if="weekRange52" class="quote-row">
        <div class="quote-item">
          <span class="label">52W High</span>
          <span class="value">{{ formatPrice(weekRange52.high) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">52W Low</span>
          <span class="value">{{ formatPrice(weekRange52.low) }}</span>
        </div>
      </div>

      <!-- Account Context (if available) -->
      <div v-if="accountInfo" class="quote-row">
        <div class="quote-item">
          <span class="label">Buying Power</span>
          <span class="value">{{ formatCurrency(accountInfo.buying_power) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Account Value</span>
          <span class="value">{{ formatCurrency(accountInfo.total_equity) }}</span>
        </div>
      </div>

      <!-- Position Context (if user has position in this symbol) -->
      <div v-if="userPosition" class="quote-row">
        <div class="quote-item">
          <span class="label">Your Position</span>
          <span class="value">{{ formatPosition(userPosition) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Unrealized P&L</span>
          <span class="value" :class="getPLClass(userPosition.unrealized_pl)">
            {{ formatCurrency(userPosition.unrealized_pl) }}
          </span>
        </div>
      </div>

      <!-- Options Context (if symbol has options and we have data) -->
      <div v-if="hasOptionsData" class="quote-row">
        <div class="quote-item">
          <span class="label">Implied Vol</span>
          <span class="value">{{ formatPercent(impliedVolatility) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Next Expiry</span>
          <span class="value">{{ formatDate(nextExpiration) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, ref, watch } from "vue";
import { useMarketData } from "../composables/useMarketData.js";

export default {
  name: "QuoteDetailsSection",
  props: {
    symbol: {
      type: String,
      required: true,
    },
    currentPrice: {
      type: Number,
      default: 0,
    },
    priceChange: {
      type: Number,
      default: 0,
    },
    additionalQuoteData: {
      type: Object,
      default: () => ({}),
    },
  },
  setup(props) {
    // ✅ UNIFIED SMART DATA SYSTEM - All data comes from preloaded Overview data
    const { 
      getStockPrice, 
      getBalance, 
      getPositions,
      getOverviewData
    } = useMarketData();
    
    // ✅ Live streaming data (bid/ask updates during market hours)
    const rawLiveQuote = getStockPrice(props.symbol);
    const balance = getBalance(); // Auto-refreshes every 60s
    const positions = getPositions(); // Auto-refreshes every 30s
    
    // ✅ Validate live quote data matches current symbol
    const liveQuote = computed(() => {
      const quote = rawLiveQuote.value;
      // Only return quote data if we have a valid current price for this symbol
      // This prevents showing stale bid/ask from previous symbol
      return (quote && props.currentPrice > 0) ? quote : null;
    });
    
    // ✅ Get ALL preloaded Overview data (includes everything we need)
    const overviewData = computed(() => getOverviewData(props.symbol).value);

    // Get account info from balance data
    const accountInfo = computed(() => {
      return balance.value ? {
        buying_power: balance.value.buying_power,
        total_equity: balance.value.total_equity
      } : null;
    });

    // Find user position for current symbol
    const userPosition = computed(() => {
      if (!positions.value?.positions) return null;
      return positions.value.positions.find(pos => 
        pos.underlying_symbol === props.symbol || pos.symbol === props.symbol
      ) || null;
    });

    // ✅ UNIFIED SMART DATA SYSTEM - All data from preloaded Overview data
    const previousClose = computed(() => {
      const data = overviewData.value;
      // Only return data if it matches the current symbol
      return (data && data.symbol === props.symbol) ? data.previousClose : null;
    });
    
    const weekRange52 = computed(() => {
      const data = overviewData.value;
      // Only return data if it matches the current symbol
      return (data && data.symbol === props.symbol) ? data.weekRange52 : null;
    });
    
    const averageVolume = computed(() => {
      const data = overviewData.value;
      // Only return data if it matches the current symbol
      return (data && data.symbol === props.symbol) ? data.averageVolume : null;
    });
    
    // ✅ Today's OHLCV data from preloaded Overview data
    const todayData = computed(() => {
      const data = overviewData.value;
      // Only return data if it matches the current symbol
      return (data && data.symbol === props.symbol) ? data.todayData : null;
    });
    
    const expirations = computed(() => {
      const data = overviewData.value;
      // Only return data if it matches the current symbol
      return (data && data.symbol === props.symbol) ? data.expirations || [] : [];
    });

    // Get next expiration (filter for future dates only)
    const nextExpiration = computed(() => {
      if (!expirations.value?.length) return null;
      
      const today = new Date();
      today.setHours(0, 0, 0, 0); // Reset time to start of day for comparison
      
      // Find the first expiration that is today or in the future
      const futureExpiration = expirations.value.find(exp => {
        const expDate = typeof exp === 'string' ? exp : exp.date || exp;
        const expirationDate = new Date(expDate);
        expirationDate.setHours(0, 0, 0, 0);
        return expirationDate >= today;
      });
      
      if (!futureExpiration) return null;
      
      // Extract the date string from the expiration object
      return typeof futureExpiration === 'string' 
        ? futureExpiration 
        : futureExpiration.date || futureExpiration;
    });

    // ✅ Get implied volatility from preloaded options chain
    const impliedVolatility = computed(() => {
      const data = overviewData.value;
      if (!data || data.symbol !== props.symbol || !data.optionsChain?.length) return null;
      
      // Find ATM option and get its IV
      const atmOption = data.optionsChain.find(opt => 
        Math.abs(opt.strike_price - props.currentPrice) < 5
      );
      
      return atmOption?.implied_volatility ? atmOption.implied_volatility * 100 : null;
    });

    // Computed values
    const dailyChange = computed(() => {
      if (!props.currentPrice || !previousClose.value) return 0;
      return props.currentPrice - previousClose.value;
    });

    const dailyChangePercent = computed(() => {
      if (!previousClose.value || dailyChange.value === 0) return 0;
      return (dailyChange.value / previousClose.value) * 100;
    });

    const priceChangeClass = computed(() => {
      if (dailyChange.value > 0) return 'positive';
      if (dailyChange.value < 0) return 'negative';
      return '';
    });

    const hasOptionsData = computed(() => {
      return impliedVolatility.value !== null || nextExpiration.value !== null;
    });

    // Formatting functions
    const formatPrice = (value) => {
      if (value === null || value === undefined || isNaN(value)) return "--";
      return value.toFixed(2);
    };

    const formatPriceChange = (change, changePercent) => {
      if (!change || isNaN(change)) return "--";
      const sign = change >= 0 ? '+' : '';
      return `${sign}${change.toFixed(2)} (${sign}${changePercent.toFixed(2)}%)`;
    };

    const formatVolume = (value) => {
      if (value === null || value === undefined || isNaN(value)) return "--";
      if (value >= 1000000) {
        return (value / 1000000).toFixed(1) + "M";
      }
      if (value >= 1000) {
        return (value / 1000).toFixed(1) + "K";
      }
      return value.toLocaleString();
    };

    const formatCurrency = (value) => {
      if (value === null || value === undefined || isNaN(value)) return "--";
      return new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
      }).format(value);
    };

    const formatPercent = (value) => {
      if (value === null || value === undefined || isNaN(value)) return "--";
      return value.toFixed(1) + "%";
    };

    const formatPosition = (position) => {
      if (!position) return "--";
      if (position.asset_class === 'us_option') {
        return `${position.qty} contracts`;
      }
      return `${position.qty} shares`;
    };

    const formatDate = (dateStr) => {
      if (!dateStr) return "--";
      try {
        const date = new Date(dateStr);
        return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
      } catch {
        return dateStr;
      }
    };

    const getPLClass = (pl) => {
      if (!pl || isNaN(pl)) return '';
      return pl >= 0 ? 'positive' : 'negative';
    };

    return {
      liveQuote,
      previousClose,
      todayData,
      accountInfo,
      userPosition,
      averageVolume,
      nextExpiration,
      impliedVolatility,
      weekRange52,
      dailyChange,
      dailyChangePercent,
      priceChangeClass,
      hasOptionsData,
      formatPrice,
      formatPriceChange,
      formatVolume,
      formatCurrency,
      formatPercent,
      formatPosition,
      formatDate,
      getPLClass,
    };
  },
};
</script>

<style scoped>
.quote-details {
  padding: 16px;
}

.quote-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.quote-row {
  display: flex;
  gap: 16px;
}

.quote-row.single-row {
  justify-content: flex-start;
}

.quote-item {
  flex: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-width: 0;
}

.quote-item.full-width {
  flex: none;
  width: 100%;
}

.label {
  font-size: 12px;
  color: var(--text-secondary, #cccccc);
  font-weight: 400;
}

.value {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  font-weight: 500;
  text-align: right;
}

.bid-price {
  color: var(--color-danger, #ff4444);
}

.ask-price {
  color: var(--color-success, #00c851);
}

.liquidity {
  color: var(--color-warning, #ffbb33);
  font-family: monospace;
}

.positive {
  color: var(--color-success, #00c851);
}

.negative {
  color: var(--color-danger, #ff4444);
}
</style>
