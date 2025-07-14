<template>
  <div class="quote-details">
    <div class="quote-grid">
      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Open</span>
          <span class="value">{{ formatPrice(quoteData.open) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Close</span>
          <span class="value">{{ formatPrice(quoteData.close) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">High</span>
          <span class="value">{{ formatPrice(quoteData.high) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Low</span>
          <span class="value">{{ formatPrice(quoteData.low) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">1Y High</span>
          <span class="value">{{ formatPrice(quoteData.yearHigh) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">1Y Low</span>
          <span class="value">{{ formatPrice(quoteData.yearLow) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Bid</span>
          <span class="value bid-price">{{ formatPrice(quoteData.bid) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Ask</span>
          <span class="value ask-price">{{ formatPrice(quoteData.ask) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Bid Size</span>
          <span class="value">{{ formatNumber(quoteData.bidSize) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Ask Size</span>
          <span class="value">{{ formatNumber(quoteData.askSize) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">IV Rank</span>
          <span class="value">{{ formatPercent(quoteData.ivRank) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">IV Index</span>
          <span class="value">{{ formatPercent(quoteData.ivIndex) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Volume</span>
          <span class="value">{{ formatVolume(quoteData.volume) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">MKT Cap</span>
          <span class="value">{{ formatMarketCap(quoteData.marketCap) }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">P/E Ratio</span>
          <span class="value">{{ formatRatio(quoteData.peRatio) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">EPS (TTM)</span>
          <span class="value">{{ formatPrice(quoteData.eps) }}</span>
        </div>
      </div>

      <div class="quote-row single-row">
        <div class="quote-item full-width">
          <span class="label">Earnings</span>
          <span class="value">{{ quoteData.earnings || "--" }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Dividend</span>
          <span class="value">{{ formatDividend(quoteData.dividend) }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Yield</span>
          <span class="value">{{
            formatPercent(quoteData.dividendYield)
          }}</span>
        </div>
      </div>

      <div class="quote-row">
        <div class="quote-item">
          <span class="label">Correlation</span>
          <span class="value">{{
            formatCorrelation(quoteData.correlation)
          }}</span>
        </div>
        <div class="quote-item">
          <span class="label">Liquidity</span>
          <span class="value liquidity">{{
            formatLiquidity(quoteData.liquidity)
          }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from "vue";

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
    // Additional quote data would come from API
    additionalQuoteData: {
      type: Object,
      default: () => ({}),
    },
  },
  setup(props) {
    // Mock data for now - in real implementation, this would come from API
    const quoteData = computed(() => ({
      open: props.additionalQuoteData.open || 623.16,
      close: props.additionalQuoteData.close || 623.62,
      high: props.additionalQuoteData.high || 625.16,
      low: props.additionalQuoteData.low || 621.8,
      yearHigh: props.additionalQuoteData.yearHigh || 626.87,
      yearLow: props.additionalQuoteData.yearLow || 481.8,
      bid: props.additionalQuoteData.bid || props.currentPrice - 0.05,
      ask: props.additionalQuoteData.ask || props.currentPrice + 0.05,
      bidSize: props.additionalQuoteData.bidSize || 400,
      askSize: props.additionalQuoteData.askSize || 300,
      ivRank: props.additionalQuoteData.ivRank || 13.2,
      ivIndex: props.additionalQuoteData.ivIndex || 17.5,
      volume: props.additionalQuoteData.volume || 51500000,
      marketCap: props.additionalQuoteData.marketCap || null,
      peRatio: props.additionalQuoteData.peRatio || null,
      eps: props.additionalQuoteData.eps || null,
      earnings: props.additionalQuoteData.earnings || null,
      dividend: props.additionalQuoteData.dividend || 1.76,
      dividendYield: props.additionalQuoteData.dividendYield || 1.13,
      correlation: props.additionalQuoteData.correlation || 1.0,
      liquidity: props.additionalQuoteData.liquidity || 4,
    }));

    const formatPrice = (value) => {
      if (value === null || value === undefined) return "--";
      return value.toFixed(2);
    };

    const formatNumber = (value) => {
      if (value === null || value === undefined) return "--";
      return value.toLocaleString();
    };

    const formatPercent = (value) => {
      if (value === null || value === undefined) return "--";
      return value.toFixed(1);
    };

    const formatVolume = (value) => {
      if (value === null || value === undefined) return "--";
      if (value >= 1000000) {
        return (value / 1000000).toFixed(1) + "M";
      }
      if (value >= 1000) {
        return (value / 1000).toFixed(1) + "K";
      }
      return value.toLocaleString();
    };

    const formatMarketCap = (value) => {
      if (value === null || value === undefined) return "--";
      if (value >= 1000000000000) {
        return "$" + (value / 1000000000000).toFixed(1) + "T";
      }
      if (value >= 1000000000) {
        return "$" + (value / 1000000000).toFixed(1) + "B";
      }
      if (value >= 1000000) {
        return "$" + (value / 1000000).toFixed(1) + "M";
      }
      return "$" + value.toLocaleString();
    };

    const formatRatio = (value) => {
      if (value === null || value === undefined) return "--";
      return value.toFixed(2);
    };

    const formatDividend = (value) => {
      if (value === null || value === undefined) return "--";
      return value.toFixed(2);
    };

    const formatCorrelation = (value) => {
      if (value === null || value === undefined) return "--";
      return value.toFixed(2);
    };

    const formatLiquidity = (value) => {
      if (value === null || value === undefined) return "--";
      const stars = "★".repeat(Math.min(Math.max(value, 0), 5));
      return stars;
    };

    return {
      quoteData,
      formatPrice,
      formatNumber,
      formatPercent,
      formatVolume,
      formatMarketCap,
      formatRatio,
      formatDividend,
      formatCorrelation,
      formatLiquidity,
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
</style>
