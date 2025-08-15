<template>
  <div class="collapsible-options-chain">
    <!-- Strike Count Filter -->
    <div class="strike-filter-header">
      <div class="filter-group">
        <label for="strike-count" class="filter-label">Strikes to show:</label>
        <select
          id="strike-count"
          v-model="strikeCount"
          class="strike-count-select"
          @change="onStrikeCountChange"
        >
          <option value="10">10 strikes</option>
          <option value="20">20 strikes</option>
          <option value="30">30 strikes</option>
          <option value="50">50 strikes</option>
          <option value="100">100 strikes</option>
        </select>
      </div>
      <div class="filter-info">
        <span class="info-text">{{ strikeCount }} strikes around ATM</span>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state">
      <div class="loading-spinner"></div>
      <span>Loading expiration dates...</span>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <i class="pi pi-exclamation-triangle"></i>
      <span>{{ error }}</span>
    </div>

    <!-- Expiration Groups -->
    <div v-else class="expiration-groups">
      <div
        v-for="expiration in expirationGroups"
        :key="expiration.date"
        class="expiration-group"
        :class="{ expanded: expiration.isExpanded }"
      >
        <!-- Expiration Header -->
        <div
          class="expiration-header"
          @click="toggleExpiration(expiration)"
          :class="{ expanded: expiration.isExpanded }"
        >
          <!-- Chevron Icon -->
          <div
            class="chevron-icon"
            :class="{ expanded: expiration.isExpanded }"
          >
            <i class="pi pi-chevron-down"></i>
          </div>

          <!-- Date Display -->
          <div class="date-display">
            <span class="date-label">{{ expiration.label }}</span>
            <div class="badge-container">
              <span v-if="expiration.isMonthly" class="monthly-badge">M</span>
              <span v-else class="weekly-badge">W</span>
            </div>
          </div>

          <!-- Days to Expiry -->
          <div class="days-display">
            <span class="days-label">{{ expiration.daysToExpiry }}d</span>
          </div>

          <!-- IV Information -->
          <div class="iv-display">
            <span class="iv-label">
              IVx: {{ expiration.ivData.rank }}% ({{
                expiration.ivData.change >= 0 ? "+" : ""
              }}{{ expiration.ivData.change }})
            </span>
          </div>

          <!-- Loading Indicator for this expiration -->
          <div v-if="expiration.isLoading" class="expiration-loading">
            <div class="mini-spinner"></div>
          </div>
        </div>

        <!-- Options Content (Collapsible) -->
        <div
          class="expiration-content"
          :class="{
            expanded: expiration.isExpanded,
            collapsed: !expiration.isExpanded,
          }"
        >
          <div class="content-wrapper">
            <!-- Options Chain for this expiration -->
            <div
              v-if="expiration.hasLoaded && expiration.optionsData.length > 0"
            >
              <!-- Options Chain Header -->
              <div class="options-header">
                <div class="calls-header">
                  <div class="header-cell">Delta</div>
                  <div class="header-cell">Theta</div>
                  <div class="header-cell">Bid</div>
                  <div class="header-cell">Ask</div>
                </div>
                <div class="strike-header">Strike</div>
                <div class="puts-header">
                  <div class="header-cell">Bid</div>
                  <div class="header-cell">Ask</div>
                  <div class="header-cell">Theta</div>
                  <div class="header-cell">Delta</div>
                </div>
              </div>

              <!-- Options Rows -->
              <div class="options-rows">
                <div
                  v-for="strike in getStrikesForExpiration(expiration)"
                  :key="`${expiration.date}-${strike}`"
                  class="option-row"
                  :class="{ 'at-the-money': isAtTheMoney(strike) }"
                >
                  <!-- Call Side -->
                  <div class="call-side">
                    <div
                      v-if="getCallOption(expiration, strike)"
                      class="option-data"
                      :class="getCallSelectionClass(expiration, strike)"
                      @click="selectCallOption(expiration, strike)"
                    >
                      <div class="greek-cell">
                        {{ getCallDelta(expiration, strike) }}
                      </div>
                      <div class="greek-cell">
                        {{ getCallTheta(expiration, strike) }}
                      </div>
                      <div
                        class="price-cell bid"
                        @click.stop="
                          selectCallOption(expiration, strike, 'sell')
                        "
                      >
                        {{ formatPrice(getCallBid(expiration, strike)) }}
                      </div>
                      <div
                        class="price-cell ask"
                        @click.stop="
                          selectCallOption(expiration, strike, 'buy')
                        "
                      >
                        {{ formatPrice(getCallAsk(expiration, strike)) }}
                      </div>
                    </div>
                    <div v-else class="option-data empty">
                      <div class="greek-cell">-</div>
                      <div class="greek-cell">-</div>
                      <div class="price-cell">-</div>
                      <div class="price-cell">-</div>
                    </div>
                  </div>

                  <!-- Strike Price -->
                  <div class="strike-cell">
                    <!-- ITM label above the line (left side) -->
                    <div
                      v-if="isAtTheMoney(strike)"
                      class="itm-label itm-above"
                    >
                      ITM
                    </div>

                    <span class="strike-price">${{ strike.toFixed(0) }}</span>

                    <!-- ITM label below the line (right side) -->
                    <div
                      v-if="isAtTheMoney(strike)"
                      class="itm-label itm-below"
                    >
                      ITM
                    </div>
                  </div>

                  <!-- Put Side -->
                  <div class="put-side">
                    <div
                      v-if="getPutOption(expiration, strike)"
                      class="option-data"
                      :class="getPutSelectionClass(expiration, strike)"
                      @click="selectPutOption(expiration, strike)"
                    >
                      <div
                        class="price-cell bid"
                        @click.stop="
                          selectPutOption(expiration, strike, 'sell')
                        "
                      >
                        {{ formatPrice(getPutBid(expiration, strike)) }}
                      </div>
                      <div
                        class="price-cell ask"
                        @click.stop="selectPutOption(expiration, strike, 'buy')"
                      >
                        {{ formatPrice(getPutAsk(expiration, strike)) }}
                      </div>
                      <div class="greek-cell">
                        {{ getPutTheta(expiration, strike) }}
                      </div>
                      <div class="greek-cell">
                        {{ getPutDelta(expiration, strike) }}
                      </div>
                    </div>
                    <div v-else class="option-data empty">
                      <div class="price-cell">-</div>
                      <div class="price-cell">-</div>
                      <div class="greek-cell">-</div>
                      <div class="greek-cell">-</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Empty State -->
            <div
              v-else-if="
                expiration.hasLoaded && expiration.optionsData.length === 0
              "
              class="empty-state"
            >
              <i class="pi pi-info-circle"></i>
              <span>No options available for this expiration</span>
            </div>

            <!-- Loading State for expiration content -->
            <div v-else-if="expiration.isLoading" class="content-loading">
              <div class="loading-spinner"></div>
              <span>Loading options...</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, reactive, watch, onMounted, onUnmounted } from "vue";
import { useMarketData } from "../composables/useMarketData.js";
import { useSelectedLegs } from "../composables/useSelectedLegs.js";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";

export default {
  name: "CollapsibleOptionsChain",
  props: {
    symbol: {
      type: String,
      required: true,
    },
    underlyingPrice: {
      type: Number,
      default: null,
    },
    expirationDates: {
      type: Array,
      default: () => [],
    },
    optionsDataByExpiration: {
      type: Object,
      default: () => ({}),
    },
    loading: {
      type: Boolean,
      default: false,
    },
    error: {
      type: String,
      default: null,
    },
    currentStrikeCount: {
      type: Number,
      default: 20,
    },
  },
  emits: [
    "expiration-expanded",
    "expiration-collapsed",
    "strike-count-changed",
  ],
  setup(props, { emit }) {
    const { getOptionPrice, getOptionGreeks } = useMarketData();
    const { 
      isSelected, 
      addFromOptionsChain, 
      removeLeg, 
      getSelectionClass 
    } = useSelectedLegs();

    // Reactive state - sync with parent's current strike count
    const strikeCount = ref(props.currentStrikeCount || 20);
    const expandedExpirations = ref(new Set());
    const liveOptionPrices = reactive(new Map());
    const liveOptionGreeks = reactive(new Map());

    // Component registration system
    const componentId = `CollapsibleOptionsChain-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // Computed properties
    const expirationGroups = computed(() => {
      return props.expirationDates.map((expirationEntry) => {
        // Always expect enhanced format - object with date, symbol, type
        const dateStr = expirationEntry.date;
        const symbol = expirationEntry.symbol;
        const type = expirationEntry.type;
        const isMonthly = (type === 'monthly');

        const [year, month, day] = dateStr.split("-").map(Number);
        const date = new Date(Date.UTC(year, month - 1, day));
        const today = new Date();
        today.setHours(0, 0, 0, 0);

        // Calculate days to expiry
        const timeDiff = date.getTime() - today.getTime();
        const daysToExpiry = Math.max(
          0,
          Math.ceil(timeDiff / (1000 * 60 * 60 * 24))
        );

        // Create unique key for this expiration entry
        const uniqueKey = `${dateStr}-${type}-${symbol}`;

        const optionsData = props.optionsDataByExpiration[uniqueKey] || [];
        const hasLoaded = optionsData.length > 0;
        const isLoading = expandedExpirations.value.has(uniqueKey) && !hasLoaded;

        return {
          date: dateStr,
          uniqueKey: uniqueKey,
          symbol: symbol,
          type: type,
          label: date.toLocaleDateString("en-US", {
            month: "short",
            day: "numeric",
            year:
              date.getUTCFullYear() !== today.getFullYear()
                ? "numeric"
                : undefined,
            timeZone: "UTC",
          }),
          daysToExpiry,
          isExpanded: expandedExpirations.value.has(uniqueKey),
          isLoading: expandedExpirations.value.has(uniqueKey) && !hasLoaded,
          hasLoaded,
          optionsData,
          isMonthly,
          // Static IV data to avoid unnecessary re-renders
          ivData: {
            rank: 45.2,
            change: 2.1,
            current: 18.7,
          },
        };
      });
    });

    const hasExpandedExpirations = computed(() => {
      return expandedExpirations.value.size > 0;
    });

    // Methods
    const toggleExpiration = (expiration) => {
      const wasExpanded = expiration.isExpanded;

      if (wasExpanded) {
        // Collapse expiration - use uniqueKey for tracking
        expandedExpirations.value.delete(expiration.uniqueKey);
        emit("expiration-collapsed", expiration.uniqueKey);

        // Clean up the local caches for the collapsed expiration's symbols
        if (expiration.optionsData && expiration.optionsData.length > 0) {
          expiration.optionsData.forEach(option => {
            if (option.symbol) {
              liveOptionPrices.delete(option.symbol);
              liveOptionGreeks.delete(option.symbol);
            }
          });
        }
      } else {
        // Expand expiration - use uniqueKey for tracking but pass clean date and symbol info
        expandedExpirations.value.add(expiration.uniqueKey);
        emit("expiration-expanded", expiration.uniqueKey, expiration);
      }
    };

    // Option data methods (similar to original OptionsChain)
    const getStrikesForExpiration = (expiration) => {
      if (!expiration.optionsData.length) return [];

      const strikes = [
        ...new Set(expiration.optionsData.map((opt) => opt.strike_price)),
      ];
      return strikes.sort((a, b) => a - b);
    };

    const getCallOption = (expiration, strike) => {
      return expiration.optionsData.find(
        (opt) =>
          (opt.type === "call" || opt.type === "c") && Math.abs(opt.strike_price - strike) < 0.01
      );
    };

    const getPutOption = (expiration, strike) => {
      return expiration.optionsData.find(
        (opt) =>
          (opt.type === "put" || opt.type === "p") && Math.abs(opt.strike_price - strike) < 0.01
      );
    };

    // Single registration method per component to prevent double registration
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    const getLivePrice = (symbol) => {
      if (!symbol) return null;

      if (!liveOptionPrices.has(symbol)) {
        // Ensure symbol is registered (only once per component)
        ensureSymbolRegistration(symbol);
        
        // Call getOptionPrice only once to set up the subscription
        liveOptionPrices.set(symbol, getOptionPrice(symbol));
      }
      
      const livePrice = liveOptionPrices.get(symbol)?.value;
      return livePrice;
    };

    const getCallBid = (expiration, strike) => {
      const option = getCallOption(expiration, strike);
      if (!option) return null;
      
      const livePrice = getLivePrice(option.symbol);
      return livePrice?.bid ?? option.bid;
    };

    const getCallAsk = (expiration, strike) => {
      const option = getCallOption(expiration, strike);
      if (!option) return null;
      const livePrice = getLivePrice(option.symbol);
      return livePrice?.ask ?? option.ask;
    };

    const getPutBid = (expiration, strike) => {
      const option = getPutOption(expiration, strike);
      if (!option) return null;
      const livePrice = getLivePrice(option.symbol);
      return livePrice?.bid ?? option.bid;
    };

    const getPutAsk = (expiration, strike) => {
      const option = getPutOption(expiration, strike);
      if (!option) return null;
      const livePrice = getLivePrice(option.symbol);
      return livePrice?.ask ?? option.ask;
    };

    const getLiveGreeks = (symbol) => {
      if (!symbol) return null;

      if (!liveOptionGreeks.has(symbol)) {
        // Ensure symbol is registered (only once per component)
        ensureSymbolRegistration(symbol);
        
        // Call getOptionGreeks only once to set up the subscription
        liveOptionGreeks.set(symbol, getOptionGreeks(symbol));
      }
      
      const liveGreeks = liveOptionGreeks.get(symbol)?.value;
      return liveGreeks;
    };

    const getCallDelta = (expiration, strike) => {
      const option = getCallOption(expiration, strike);
      if (!option) return "-";
      
      // Use live Greeks if available, fallback to static data
      const liveGreeks = getLiveGreeks(option.symbol);
      const delta = liveGreeks?.delta ?? option.delta;
      return delta ? delta.toFixed(2) : "-";
    };

    const getCallTheta = (expiration, strike) => {
      const option = getCallOption(expiration, strike);
      if (!option) return "-";
      
      // Use live Greeks if available, fallback to static data
      const liveGreeks = getLiveGreeks(option.symbol);
      const theta = liveGreeks?.theta ?? option.theta;
      return theta ? theta.toFixed(2) : "-";
    };

    const getPutDelta = (expiration, strike) => {
      const option = getPutOption(expiration, strike);
      if (!option) return "-";
      
      // Use live Greeks if available, fallback to static data
      const liveGreeks = getLiveGreeks(option.symbol);
      const delta = liveGreeks?.delta ?? option.delta;
      return delta ? delta.toFixed(2) : "-";
    };

    const getPutTheta = (expiration, strike) => {
      const option = getPutOption(expiration, strike);
      if (!option) return "-";
      
      // Use live Greeks if available, fallback to static data
      const liveGreeks = getLiveGreeks(option.symbol);
      const theta = liveGreeks?.theta ?? option.theta;
      return theta ? theta.toFixed(2) : "-";
    };

    const formatPrice = (price) => {
      if (price === null || price === undefined) return "-";
      return price.toFixed(2);
    };

    const isAtTheMoney = (strike) => {
      if (!props.underlyingPrice) return false;
      // Find the single closest strike to the underlying price
      const strikes = getStrikesForExpiration(
        expirationGroups.value.find(
          (exp) => exp.hasLoaded && exp.optionsData.length > 0
        ) || { optionsData: [] }
      );
      if (strikes.length === 0) return false;

      const closestStrike = strikes.reduce((prev, curr) =>
        Math.abs(curr - props.underlyingPrice) <
        Math.abs(prev - props.underlyingPrice)
          ? curr
          : prev
      );
      return strike === closestStrike;
    };

    // ITM/OTM detection functions
    const isCallITM = (strike) => {
      if (!props.underlyingPrice) return false;
      return strike < props.underlyingPrice;
    };

    const isPutITM = (strike) => {
      if (!props.underlyingPrice) return false;
      return strike > props.underlyingPrice;
    };

    // Get ITM label for display
    const getITMLabel = (strike, optionType) => {
      if (!props.underlyingPrice) return null;

      const isITM =
        optionType === "call" ? isCallITM(strike) : isPutITM(strike);
      if (!isITM) return null;

      // Calculate how far ITM (in standard deviations or simple distance)
      const distance = Math.abs(strike - props.underlyingPrice);

      // Simple distance-based labels (can be enhanced with volatility later)
      if (distance >= 100) return "2 SD";
      if (distance >= 50) return "1 SD";
      return "ITM";
    };

    const getCallSelectionClass = (expiration, strike) => {
      const option = getCallOption(expiration, strike);
      if (!option) return "";

      return getSelectionClass(option.symbol);
    };

    const getPutSelectionClass = (expiration, strike) => {
      const option = getPutOption(expiration, strike);
      if (!option) return "";

      return getSelectionClass(option.symbol);
    };

    const selectCallOption = (expiration, strike, side = "buy") => {
      const option = getCallOption(expiration, strike);
      if (!option) return;

      if (isSelected(option.symbol)) {
        // If already selected, remove it
        removeLeg(option.symbol);
      } else {
        // Add new selection with expiry from the expiration group
        addFromOptionsChain({
          ...option,
          expiry: expiration.date,
          expiration_date: expiration.date
        }, side);
      }
    };

    const selectPutOption = (expiration, strike, side = "buy") => {
      const option = getPutOption(expiration, strike);
      if (!option) return;

      if (isSelected(option.symbol)) {
        // If already selected, remove it
        removeLeg(option.symbol);
      } else {
        // Add new selection with expiry from the expiration group
        addFromOptionsChain({
          ...option,
          expiry: expiration.date,
          expiration_date: expiration.date
        }, side);
      }
    };

    // Strike count change handler
    const onStrikeCountChange = () => {
      // Emit event to parent to handle strike count change
      emit("strike-count-changed", strikeCount.value);
    };

    // --- Component Registration System Lifecycle ---

    const cleanupComponentRegistrations = () => {
      // Unregister all symbols this component was using
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      
      // Clear local tracking
      registeredSymbols.clear();
      liveOptionPrices.clear();
      liveOptionGreeks.clear();
    };

    // Watch for the underlying symbol changing
    watch(() => props.symbol, (newSymbol, oldSymbol) => {      
      if (newSymbol !== oldSymbol) {
        // Unregister all current symbols
        cleanupComponentRegistrations();
        
        // Collapse all sections for the new symbol
        expandedExpirations.value.clear();        
      }
    }, { immediate: true });

    // Clean up when the component is unmounted
    onUnmounted(() => {
      cleanupComponentRegistrations();
    });

    return {
      // State
      expirationGroups,
      strikeCount,

      // Computed
      hasExpandedExpirations,

      // Methods
      toggleExpiration,
      onStrikeCountChange,
      getStrikesForExpiration,
      getCallOption,
      getPutOption,
      getCallBid,
      getCallAsk,
      getPutBid,
      getPutAsk,
      getCallDelta,
      getCallTheta,
      getPutDelta,
      getPutTheta,
      formatPrice,
      isAtTheMoney,
      getITMLabel,
      getCallSelectionClass,
      getPutSelectionClass,
      selectCallOption,
      selectPutOption,
    };
  },
};
</script>

<style scoped>
.collapsible-options-chain {
  display: flex;
  flex-direction: column;
  height: 100%;
  background-color: var(--options-grid-bg);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

/* Strike Filter Header */
.strike-filter-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
  gap: var(--spacing-md);
}

.filter-group {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.filter-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  white-space: nowrap;
}

.strike-count-select {
  padding: var(--spacing-xs) var(--spacing-sm);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-normal);
  min-width: 120px;
}

.strike-count-select:hover {
  border-color: var(--border-tertiary);
  background-color: var(--bg-quaternary);
}

.strike-count-select:focus {
  outline: none;
  border-color: var(--color-info);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.filter-info {
  display: flex;
  align-items: center;
}

.info-text {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-style: italic;
}

/* Loading and Error States */
.loading-state,
.error-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-md);
  padding: var(--spacing-xxl);
  color: var(--text-secondary);
}

.loading-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-info);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.error-state {
  color: var(--color-danger);
}

/* Expiration Groups */
.expiration-groups {
  flex: 1;
  overflow-y: auto;
}

.expiration-group {
  border-bottom: 1px solid var(--border-primary);
}

.expiration-group:last-child {
  border-bottom: none;
}

/* Expiration Header */
.expiration-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--bg-secondary);
  cursor: pointer;
  transition: var(--transition-normal);
  border-bottom: 1px solid var(--border-secondary);
  position: relative;
}

.expiration-header:hover {
  background-color: var(--bg-tertiary);
}

.expiration-header.expanded {
  background-color: var(--bg-tertiary);
  border-bottom-color: var(--border-primary);
}

.chevron-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  color: var(--text-secondary);
  transition: transform 0.3s ease;
}

.chevron-icon.expanded {
  transform: rotate(180deg);
}

.date-display {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  min-width: 150px;
}

.date-label {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  width: 100px;
  text-align: left;
}

.badge-container {
  width: 30px;
  display: flex;
  justify-content: center;
}

.monthly-badge {
  background-color: var(--color-info);
  color: var(--text-primary);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-bold);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  text-transform: uppercase;
}

.weekly-badge {
  background-color: var(--color-warning);
  color: var(--text-primary);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-bold);
  padding: 2px 4px;
  border-radius: var(--radius-sm);
  text-transform: uppercase;
}

.days-display {
  min-width: 50px;
  display: flex;
  justify-content: center;
  align-items: center;
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
}

.days-label {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  background-color: var(--bg-quaternary);
  padding: 4px 8px;
  border-radius: var(--radius-md);
  line-height: 1;
}

.iv-display {
  text-align: right;
  margin-left: auto;
}

.iv-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.expiration-loading {
  display: flex;
  align-items: center;
}

.mini-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-info);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

/* Expiration Content (Collapsible) */
.expiration-content {
  max-height: 0;
  overflow: hidden;
  transition: max-height 0.3s ease-out;
  background-color: var(--options-grid-bg);
}

.expiration-content.expanded {
  max-height: 2000px; /* Large enough for content */
}

.content-wrapper {
  padding: 0;
}

/* Options Chain Styles (similar to original) */
.options-header {
  display: grid;
  grid-template-columns: 1fr 100px 1fr;
  background-color: var(--options-grid-bg);
  border-bottom: 2px solid var(--border-primary);
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-base);
  color: var(--text-secondary);
  position: sticky;
  top: 0;
  z-index: 1;
}

.calls-header,
.puts-header {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: var(--spacing-sm);
  padding: var(--spacing-md) var(--spacing-lg);
}

.puts-header {
  text-align: right;
}

.strike-header {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--options-strike-bg);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  border-left: 1px solid var(--border-secondary);
  border-right: 1px solid var(--border-secondary);
}

.header-cell {
  text-align: center;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-tertiary);
}

.options-rows {
  /* Remove inner scroll - let outer scroll handle everything */
}

.option-row {
  display: grid;
  grid-template-columns: 1fr 100px 1fr;
  border-bottom: 1px solid var(--border-primary);
  transition: var(--transition-normal);
}

.option-row:hover {
  background-color: var(--options-row-hover);
}

.option-row.at-the-money {
  position: relative;
}

.option-row.at-the-money::before {
  content: "";
  position: absolute;
  top: -1px;
  left: 0;
  right: 0;
  height: 1px;
  background-color: #fbbf24; /* Yellow line */
  z-index: 2;
  pointer-events: none;
}

.call-side,
.put-side {
  display: flex;
  align-items: center;
  position: relative;
}

/* ITM Labels positioned outside the strike cell, connected to the yellow line */
.strike-cell {
  position: relative;
}

.itm-label {
  position: absolute;
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-bold);
  padding: 2px 6px;
  border-radius: var(--radius-xs);
  background-color: #fbbf24; /* Same yellow as the line */
  color: var(--bg-primary); /* Dark text on yellow background */
  z-index: 4;
  pointer-events: none;
  text-transform: uppercase;
  white-space: nowrap;
}

.itm-above {
  top: -16px; /* Aligned with the yellow line */
  left: -32px; /* To the left of the strike cell */
}

.itm-below {
  top: 0px; /* Aligned with the yellow line */
  right: -32px; /* To the right of the strike cell */
}

.option-data {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: var(--spacing-sm);
  padding: var(--spacing-md) var(--spacing-lg);
  width: 100%;
  cursor: pointer;
  transition: background-color 0.1s ease;
}

.option-data:hover:not(.empty) {
  background-color: var(--bg-quaternary);
}

.option-data.selected-buy {
  background-color: var(--options-selected-buy);
  border-left: 3px solid var(--color-success);
}

.option-data.selected-sell {
  background-color: var(--options-selected-sell);
  border-left: 3px solid var(--color-danger);
}

.option-data.empty {
  cursor: default;
  color: var(--text-quaternary);
}

.greek-cell,
.price-cell {
  font-size: var(--font-size-base);
  text-align: center;
  padding: var(--spacing-xs);
  border-radius: var(--radius-sm);
  transition: var(--transition-normal);
}

.greek-cell {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.price-cell {
  color: var(--text-primary);
  font-weight: var(--font-weight-semibold);
  cursor: pointer;
}

.price-cell:hover {
  background-color: rgba(255, 255, 255, 0.1);
}

.price-cell.bid:hover {
  background-color: rgba(255, 68, 68, 0.3);
}

.price-cell.ask:hover {
  background-color: rgba(0, 200, 81, 0.3);
}

.strike-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--options-strike-bg);
  border-left: 1px solid var(--border-secondary);
  border-right: 1px solid var(--border-secondary);
}

.strike-price {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
}

/* Empty and Loading States */
.empty-state,
.content-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-md);
  padding: var(--spacing-xl);
  color: var(--text-secondary);
}

/* Scrollbar styling */
.expiration-groups::-webkit-scrollbar,
.options-rows::-webkit-scrollbar {
  width: 6px;
}

.expiration-groups::-webkit-scrollbar-track,
.options-rows::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

.expiration-groups::-webkit-scrollbar-thumb,
.options-rows::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.expiration-groups::-webkit-scrollbar-thumb:hover,
.options-rows::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}

/* Animations */
@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Responsive adjustments */
@media (max-width: 1200px) {
  .expiration-header {
    padding: var(--spacing-sm) var(--spacing-md);
    gap: var(--spacing-sm);
  }

  .date-display {
    min-width: 100px;
  }

  .options-header {
    font-size: var(--font-size-sm);
  }

  .calls-header,
  .puts-header {
    padding: var(--spacing-sm) var(--spacing-md);
    gap: var(--spacing-xs);
  }

  .option-data {
    padding: 6px var(--spacing-md);
    gap: var(--spacing-xs);
  }

  .greek-cell,
  .price-cell {
    font-size: var(--font-size-sm);
    padding: 2px;
  }
}
</style>
