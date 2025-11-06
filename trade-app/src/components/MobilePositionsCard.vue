<template>
  <div class="mobile-positions-card">
    <!-- Symbol Group Header -->
    <div 
      class="symbol-group-header"
      @click="toggleExpand"
      :class="{ expanded: isExpanded }"
    >
      <div class="symbol-info">
        <div class="symbol-row">
          <span class="expand-icon">
            {{ isExpanded ? "▼" : "▶" }}
          </span>
          <span class="symbol-name">{{ group.symbol }}</span>
          <span class="strategy-count">{{ group.strategies?.length || 0 }} Strategies</span>
        </div>
        <div class="symbol-totals">
          <div class="total-item">
            <span class="total-label">Day P/L:</span>
            <span 
              class="total-value"
              :class="symbolPlDay >= 0 ? 'profit' : 'loss'"
            >
              {{ formatCurrency(symbolPlDay) }}
            </span>
          </div>
          <div class="total-item">
            <span class="total-label">Total P/L:</span>
            <span 
              class="total-value"
              :class="symbolPlOpen >= 0 ? 'profit' : 'loss'"
            >
              {{ formatCurrency(symbolPlOpen) }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Expanded Content -->
    <div v-if="isExpanded" class="expanded-content">
      <!-- Strategies -->
      <div 
        v-for="strategy in group.strategies" 
        :key="strategy.name"
        class="strategy-section"
      >
        <!-- Strategy Header -->
        <div class="strategy-header">
          <div class="strategy-name">{{ strategy.name }}</div>
          <div class="strategy-totals">
            <span 
              class="strategy-pl"
              :class="getStrategyPlDay(strategy) >= 0 ? 'profit' : 'loss'"
            >
              {{ formatCurrency(getStrategyPlDay(strategy)) }}
            </span>
            <span class="strategy-pl-label">Day</span>
            <span 
              class="strategy-pl"
              :class="getStrategyPlOpen(strategy) >= 0 ? 'profit' : 'loss'"
            >
              {{ formatCurrency(getStrategyPlOpen(strategy)) }}
            </span>
            <span class="strategy-pl-label">Total</span>
          </div>
        </div>

        <!-- Strategy Legs -->
        <div class="legs-container">
          <div 
            v-for="(leg, legIndex) in strategy.legs"
            :key="`${strategy.name}-leg-${legIndex}`"
            class="leg-card"
            :class="{ 
              selected: isLegSelected(leg.symbol),
              'equity-leg': leg.asset_class === 'us_equity'
            }"
            @click="toggleLegSelection(leg, strategy, group)"
          >
            <!-- Leg Header -->
            <div class="leg-header">
              <div class="leg-qty" :class="leg.qty > 0 ? 'long' : 'short'">
                {{ formatLegQty(leg.qty) }}
              </div>
              <div class="leg-pl">
                <div 
                  class="leg-pl-day"
                  :class="getLegPlDay(leg) >= 0 ? 'profit' : 'loss'"
                >
                  {{ formatCurrency(getLegPlDay(leg)) }}
                </div>
                <div 
                  class="leg-pl-total"
                  :class="getLegPL(leg) >= 0 ? 'profit' : 'loss'"
                >
                  {{ formatCurrency(getLegPL(leg)) }}
                </div>
              </div>
            </div>

            <!-- Leg Details -->
            <div class="leg-details">
              <template v-if="leg.asset_class === 'us_equity'">
                <div class="equity-details">
                  <span class="shares-label">Shares</span>
                  <span class="entry-price">Entry: {{ formatCurrency(leg.avg_entry_price) }}</span>
                </div>
              </template>
              <template v-else>
                <div class="option-details">
                  <div class="option-main">
                    <span class="expiry-date">{{ formatLegDate(leg) }}</span>
                    <span class="day-indicator">{{ formatLegDays(leg) }}</span>
                    <span class="strike-price">{{ formatLegStrike(leg) }}</span>
                    <span 
                      class="option-type"
                      :class="getLegTypeClass(leg)"
                    >
                      {{ formatLegType(leg) }}
                    </span>
                    <span v-if="isLegITM(leg)" class="itm-indicator">ITM</span>
                  </div>
                  <div class="option-prices">
                    <span class="price-item">Bid: {{ getLegBid(leg) }}</span>
                    <span class="price-item">Ask: {{ getLegAsk(leg) }}</span>
                    <span class="price-item">Entry: {{ formatCurrency(leg.avg_entry_price) }}</span>
                  </div>
                </div>
              </template>
            </div>

            <!-- Days Open -->
            <div class="leg-footer">
              <span class="days-open">{{ formatDaysOpen(getLegDaysOpen(leg)) }} open</span>
              <span v-if="leg.asset_class === 'us_option'" class="dte">
                {{ formatDTE(getStrategyDTE(strategy)) }} DTE
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from 'vue';

export default {
  name: 'MobilePositionsCard',
  props: {
    group: {
      type: Object,
      required: true
    },
    isExpanded: {
      type: Boolean,
      default: false
    },
    currentPrice: {
      type: Number,
      default: 0
    },
    // Functions passed from parent
    getStrategyPlDay: {
      type: Function,
      required: true
    },
    getStrategyPlOpen: {
      type: Function,
      required: true
    },
    getLegPL: {
      type: Function,
      required: true
    },
    getLegPlDay: {
      type: Function,
      required: true
    },
    getLegBid: {
      type: Function,
      required: true
    },
    getLegAsk: {
      type: Function,
      required: true
    },
    getLegDaysOpen: {
      type: Function,
      required: true
    },
    getStrategyDTE: {
      type: Function,
      required: true
    },
    formatLegDate: {
      type: Function,
      required: true
    },
    formatLegDays: {
      type: Function,
      required: true
    },
    formatLegStrike: {
      type: Function,
      required: true
    },
    formatLegType: {
      type: Function,
      required: true
    },
    formatDTE: {
      type: Function,
      required: true
    },
    formatDaysOpen: {
      type: Function,
      required: true
    },
    getLegTypeClass: {
      type: Function,
      required: true
    },
    isLegITM: {
      type: Function,
      required: true
    },
    isLegSelected: {
      type: Function,
      required: true
    }
  },
  emits: ['toggle-expand', 'toggle-leg-selection'],
  setup(props, { emit }) {
    // Computed properties for symbol totals
    const symbolPlDay = computed(() => {
      if (!props.group.strategies) return 0;
      return props.group.strategies.reduce((sum, strategy) => sum + props.getStrategyPlDay(strategy), 0);
    });

    const symbolPlOpen = computed(() => {
      if (!props.group.strategies) return 0;
      return props.group.strategies.reduce((sum, strategy) => sum + props.getStrategyPlOpen(strategy), 0);
    });

    const toggleExpand = () => {
      emit('toggle-expand', props.group.id);
    };

    const toggleLegSelection = (leg, strategy, group) => {
      emit('toggle-leg-selection', leg, strategy, group);
    };

    const formatCurrency = (value) => {
      if (value === null || value === undefined) return "--";
      return new Intl.NumberFormat("en-US", {
        style: "currency",
        currency: "USD",
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }).format(value);
    };

    const formatLegQty = (qty) => {
      return qty > 0 ? `+${qty}` : qty.toString();
    };

    return {
      symbolPlDay,
      symbolPlOpen,
      toggleExpand,
      toggleLegSelection,
      formatCurrency,
      formatLegQty
    };
  }
};
</script>

<style scoped>
.mobile-positions-card {
  background-color: #1a1d23;
  border-radius: 8px;
  margin-bottom: 12px;
  border: 1px solid #2a2d33;
  overflow: visible; /* Allow content to expand beyond card boundaries if needed */
}

.symbol-group-header {
  padding: 16px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  border-bottom: 1px solid #2a2d33;
}

.symbol-group-header:hover {
  background-color: #1f2329;
}

.symbol-group-header.expanded {
  background-color: #1f2329;
}

.symbol-info {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.symbol-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.expand-icon {
  color: #888888;
  font-size: 14px;
  width: 16px;
  transition: transform 0.2s ease;
}

.symbol-name {
  font-weight: 600;
  font-size: 18px;
  color: #ffffff;
}

.strategy-count {
  color: #888888;
  font-size: 14px;
  margin-left: auto;
}

.symbol-totals {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.total-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.total-label {
  font-size: 12px;
  color: #888888;
  text-transform: uppercase;
}

.total-value {
  font-size: 16px;
  font-weight: 600;
}

.total-value.profit {
  color: #00c851;
}

.total-value.loss {
  color: #ff4444;
}

.expanded-content {
  background-color: #141519;
  max-height: none; /* Allow unlimited height */
  overflow: visible; /* Ensure content is not clipped */
  height: auto; /* Allow natural height expansion */
}

.strategy-section {
  border-bottom: 1px solid #2a2d33;
  overflow: visible; /* Ensure strategy content is not clipped */
  height: auto; /* Allow natural height expansion */
}

.strategy-section:last-child {
  border-bottom: none;
}

.strategy-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #1a1d23;
  border-bottom: 1px solid #2a2d33;
}

.strategy-name {
  font-weight: 500;
  font-size: 16px;
  color: #ffffff;
  font-style: italic;
}

.strategy-totals {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.strategy-pl {
  font-weight: 600;
}

.strategy-pl.profit {
  color: #00c851;
}

.strategy-pl.loss {
  color: #ff4444;
}

.strategy-pl-label {
  color: #888888;
  font-size: 12px;
}

.legs-container {
  padding: 8px;
}

.leg-card {
  background-color: #0b0d10;
  border-radius: 6px;
  padding: 12px;
  margin-bottom: 8px;
  border: 1px solid #2a2d33;
  cursor: pointer;
  transition: all 0.2s ease;
}

.leg-card:last-child {
  margin-bottom: 0;
}

.leg-card:hover {
  background-color: #1a1d23;
  border-color: #444444;
}

.leg-card.selected {
  background-color: rgba(255, 215, 0, 0.1);
  border-color: #ffd700;
  box-shadow: 0 0 0 1px rgba(255, 215, 0, 0.3);
}

.leg-card.equity-leg {
  border-left: 3px solid #17a2b8;
}

.leg-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.leg-qty {
  font-weight: 600;
  font-size: 16px;
  padding: 4px 8px;
  border-radius: 4px;
  min-width: 50px;
  text-align: center;
}

.leg-qty.long {
  background-color: rgba(0, 200, 81, 0.2);
  color: #00c851;
}

.leg-qty.short {
  background-color: rgba(255, 68, 68, 0.2);
  color: #ff4444;
}

.leg-pl {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.leg-pl-day,
.leg-pl-total {
  font-size: 14px;
  font-weight: 600;
}

.leg-pl-day.profit,
.leg-pl-total.profit {
  color: #00c851;
}

.leg-pl-day.loss,
.leg-pl-total.loss {
  color: #ff4444;
}

.leg-details {
  margin-bottom: 8px;
}

.equity-details {
  display: flex;
  align-items: center;
  gap: 12px;
}

.shares-label {
  background-color: #17a2b8;
  color: white;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 12px;
  font-weight: 500;
}

.entry-price {
  color: #888888;
  font-size: 14px;
}

.option-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.option-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.expiry-date {
  font-size: 12px;
  color: #ffffff;
  background-color: #1a1d23;
  padding: 3px 6px;
  border-radius: 3px;
}

.day-indicator {
  font-size: 10px;
  color: #888888;
  background-color: #141519;
  padding: 2px 4px;
  border-radius: 2px;
}

.strike-price {
  font-size: 14px;
  color: #ffffff;
  font-weight: 600;
}

.option-type {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 5px;
  border-radius: 2px;
  min-width: 18px;
  text-align: center;
}

.option-type.buy-type {
  background-color: #28a745;
  color: white;
}

.option-type.sell-type {
  background-color: #dc3545;
  color: white;
}

.itm-indicator {
  font-size: 9px;
  font-weight: 600;
  background-color: #ffc107;
  color: #000;
  padding: 2px 4px;
  border-radius: 2px;
  text-transform: uppercase;
}

.option-prices {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.price-item {
  font-size: 12px;
  color: #888888;
}

.leg-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: #888888;
}

.days-open,
.dte {
  background-color: #1a1d23;
  padding: 2px 6px;
  border-radius: 3px;
}

/* Touch-friendly adjustments */
@media (hover: none) and (pointer: coarse) {
  .symbol-group-header {
    padding: 20px 16px;
  }
  
  .leg-card {
    padding: 16px;
    margin-bottom: 12px;
  }
  
  .leg-header {
    margin-bottom: 12px;
  }
  
  .leg-qty {
    padding: 6px 10px;
    font-size: 18px;
    min-width: 60px;
  }
  
  .leg-pl-day,
  .leg-pl-total {
    font-size: 16px;
  }
}

/* Smaller mobile screens */
@media (max-width: 480px) {
  .symbol-totals {
    flex-direction: column;
    gap: 8px;
  }
  
  .total-item {
    flex-direction: row;
    justify-content: space-between;
    width: 100%;
  }
  
  .strategy-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .strategy-totals {
    align-self: flex-end;
  }
  
  .option-main {
    gap: 6px;
  }
  
  .option-prices {
    gap: 8px;
  }
}

/* Very small screens */
@media (max-width: 360px) {
  .mobile-positions-card {
    margin-bottom: 8px;
  }
  
  .symbol-group-header {
    padding: 16px 12px;
  }
  
  .legs-container {
    padding: 6px;
  }
  
  .leg-card {
    padding: 12px 8px;
  }
  
  .symbol-name {
    font-size: 16px;
  }
  
  .strategy-count {
    font-size: 12px;
  }
}
</style>
