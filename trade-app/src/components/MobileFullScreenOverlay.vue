<template>
  <div v-if="visible" class="mobile-fullscreen-overlay">
    <!-- Header -->
    <div class="overlay-header">
      <h2 class="overlay-title">{{ getSectionTitle(activeSection) }}</h2>
      <button class="close-button" @click="closeOverlay">
        <i class="pi pi-times"></i>
      </button>
    </div>

    <!-- Content -->
    <div class="overlay-content">
      <!-- Overview Section -->
      <div v-if="activeSection === 'overview'" class="section-content">
        <!-- Price Chart Section -->
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-chart-line"></i>
            Price Chart
          </h3>
          <RightPanelChart
            :symbol="currentSymbol"
            :currentPrice="currentPrice"
            :height="220"
            :livePrice="livePrice"
            :enableRealtime="true"
          />
        </div>

        <!-- Quote Details Section -->
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-list"></i>
            Quote Details
          </h3>
          <QuoteDetailsSection
            :symbol="currentSymbol"
            :currentPrice="currentPrice"
            :priceChange="priceChange"
            :additionalQuoteData="additionalQuoteData"
          />
        </div>
      </div>

      <!-- Analysis Section -->
      <div v-else-if="activeSection === 'analysis'" class="section-content">
        <!-- Payoff Chart Section -->
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-chart-line"></i>
            Payoff Chart
          </h3>
          <div class="chart-wrapper">
            <PayoffChart
              v-if="chartData"
              :chartData="chartData"
              :underlyingPrice="currentPrice"
              title="Position P&L"
              :showInfo="false"
              height="300px"
              :symbol="currentSymbol"
              :isLivePrice="isLivePrice"
            />
            <div v-else class="no-chart-message">
              <p>Select options to view payoff chart</p>
            </div>
          </div>
        </div>

        <!-- Position Details Section -->
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-briefcase"></i>
            Position Details
          </h3>
          <div class="enhanced-position-details">
            <!-- Header with Total P/L -->
            <div class="position-header">
              <div class="header-row">
                <!-- Select All Checkbox -->
                <div class="header-checkbox">
                  <input
                    type="checkbox"
                    id="mobile-select-all-positions"
                    :checked="isAllSelected"
                    :indeterminate="isIndeterminate"
                    @change="toggleSelectAll"
                    class="custom-checkbox"
                  />
                  <label
                    for="mobile-select-all-positions"
                    class="checkbox-label"
                  ></label>
                </div>

                <!-- Header labels aligned with position fields -->
                <span class="header-label qty-header">QTY</span>
                <span class="header-label description-header">DESCRIPTION</span>
                <span class="header-label price-header">PRICE</span>
              </div>
            </div>

            <!-- Position Rows -->
            <div class="position-list">
              <div v-if="allPositions.length === 0" class="no-positions">
                No positions to show
              </div>
              <div v-else>
                <div
                  v-for="position in allPositions"
                  :key="position.id"
                  class="enhanced-position-row"
                  :class="{
                    'existing-position': position.isExisting,
                    'selected-position': position.isSelected,
                  }"
                >
                  <!-- Checkbox -->
                  <div class="position-checkbox">
                    <input
                      type="checkbox"
                      :id="`mobile-pos-${position.id}`"
                      :checked="checkedPositions.has(position.id)"
                      @change="togglePositionCheck(position.id)"
                      class="custom-checkbox"
                    />
                    <label
                      :for="`mobile-pos-${position.id}`"
                      class="checkbox-label"
                    ></label>
                  </div>

                  <!-- Quantity -->
                  <div class="position-qty">
                    {{ position.qty }}
                  </div>

                  <!-- Description -->
                  <div class="position-description">
                    <div class="description-main">
                      <span class="expiry-date">{{
                        formatEnhancedOptionDescription(position).expiry
                      }}</span>
                      <span class="day-indicator">{{
                        formatEnhancedOptionDescription(position).daysToExpiry
                      }}d</span>
                      <span class="strike-price">{{
                        formatEnhancedOptionDescription(position).strike
                      }}</span>
                      <span
                        class="option-type"
                        :class="{
                          'call-type':
                            formatEnhancedOptionDescription(position).type === 'C',
                          'put-type':
                            formatEnhancedOptionDescription(position).type === 'P',
                        }"
                      >
                        {{ formatEnhancedOptionDescription(position).type }}
                      </span>
                      <span
                        v-if="formatEnhancedOptionDescription(position).isITM"
                        class="itm-indicator"
                      >
                        ITM
                      </span>
                    </div>
                  </div>

                  <!-- Price -->
                  <div class="position-price">
                    {{ (position.current_price || 0).toFixed(2) }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Activity Section -->
      <div v-else-if="activeSection === 'activity'" class="section-content">
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-chart-line"></i>
            Activity
          </h3>
          <ActivitySection :currentSymbol="currentSymbol" />
        </div>
      </div>

      <!-- Watchlist Section -->
      <div v-else-if="activeSection === 'watchlist'" class="section-content">
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-star"></i>
            Watchlist
          </h3>
          <WatchlistSection />
        </div>
      </div>

      <!-- Other Sections (Placeholder) -->
      <div v-else class="section-content">
        <div class="mobile-section">
          <h3 class="section-title">
            <i class="pi pi-info-circle"></i>
            {{ getSectionTitle(activeSection) }}
          </h3>
          <div class="placeholder-content">
            <p>This section is coming soon...</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from 'vue';
import QuoteDetailsSection from './QuoteDetailsSection.vue';
import RightPanelChart from './RightPanelChart.vue';
import PayoffChart from './PayoffChart.vue';
import ActivitySection from './ActivitySection.vue';
import WatchlistSection from './WatchlistSection.vue';
import { useSmartMarketData } from '../composables/useSmartMarketData.js';

export default {
  name: 'MobileFullScreenOverlay',
  components: {
    QuoteDetailsSection,
    RightPanelChart,
    PayoffChart,
    ActivitySection,
    WatchlistSection,
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    activeSection: {
      type: String,
      default: 'overview'
    },
    currentSymbol: {
      type: String,
      required: true
    },
    currentPrice: {
      type: Number,
      default: 0
    },
    priceChange: {
      type: Number,
      default: 0
    },
    isLivePrice: {
      type: Boolean,
      default: false
    },
    chartData: {
      type: Object,
      default: null
    },
    additionalQuoteData: {
      type: Object,
      default: () => ({})
    },
    allPositions: {
      type: Array,
      default: () => []
    },
    checkedPositions: {
      type: Set,
      default: () => new Set()
    },
    isAllSelected: {
      type: Boolean,
      default: false
    },
    isIndeterminate: {
      type: Boolean,
      default: false
    }
  },
  emits: ['close', 'toggle-position-check', 'toggle-select-all'],
  setup(props, { emit }) {
    const { getStockPrice } = useSmartMarketData();

    // Live price subscription for chart updates (similar to ChartView and RightPanel)
    const livePrice = computed(() => {
      if (!props.currentSymbol) return null;
      const priceData = getStockPrice(props.currentSymbol);
      return priceData?.value || null;
    });

    const getSectionTitle = (section) => {
      const titles = {
        overview: 'Overview',
        analysis: 'Analysis',
        activity: 'Activity',
        watchlist: 'Watchlist',
        alerts: 'Alerts',
        news: 'News'
      };
      return titles[section] || 'Section';
    };

    const closeOverlay = () => {
      emit('close');
    };

    const togglePositionCheck = (positionId) => {
      emit('toggle-position-check', positionId);
    };

    const toggleSelectAll = () => {
      emit('toggle-select-all');
    };

    // Method to determine if option is ITM
    const isInTheMoney = (position) => {
      if (!position.strike_price || !props.currentPrice) return false;

      if (position.option_type === 'call' || position.option_type === 'C') {
        return props.currentPrice > position.strike_price;
      } else if (position.option_type === 'put' || position.option_type === 'P') {
        return props.currentPrice < position.strike_price;
      }
      return false;
    };

    // Method to format option description with visual indicators
    const formatEnhancedOptionDescription = (position) => {
      const type = position.option_type?.toUpperCase() || 'CALL';
      const strike = position.strike_price || 0;
      const expiry = position.expiry_date || '';

      // Format expiry date (e.g., "Jul 14")
      let formattedExpiry = expiry;
      let daysToExpiry = 0;

      if (expiry && expiry.includes('-')) {
        // Handle YYYY-MM-DD format - parse as UTC to avoid timezone issues
        const [year, month, day] = expiry.split('-').map(Number);
        const expiryDate = new Date(Date.UTC(year, month - 1, day));
        
        // Get current date for comparison
        const currentDate = new Date();
        currentDate.setHours(0, 0, 0, 0);

        // Calculate days difference
        const timeDiff = expiryDate.getTime() - currentDate.getTime();
        daysToExpiry = Math.ceil(timeDiff / (1000 * 3600 * 24));

        // Format the date using UTC timezone
        formattedExpiry = expiryDate.toLocaleDateString('en-US', {
          month: 'short',
          day: 'numeric',
          timeZone: 'UTC',
        });
      }

      return {
        type: type === 'CALL' || type === 'C' ? 'C' : 'P',
        strike,
        expiry: formattedExpiry,
        daysToExpiry: Math.max(0, daysToExpiry), // Don't show negative days
        isITM: isInTheMoney(position),
      };
    };

    return {
      livePrice, // Live price data for chart updates
      getSectionTitle,
      closeOverlay,
      togglePositionCheck,
      toggleSelectAll,
      formatEnhancedOptionDescription
    };
  }
};
</script>

<style scoped>
.mobile-fullscreen-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #141519;
  z-index: 2000;
  display: flex;
  flex-direction: column;
}

.overlay-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background-color: #1a1d23;
  border-bottom: 1px solid #2a2d33;
  flex-shrink: 0;
}

.overlay-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #ffffff;
}

.close-button {
  background: none;
  border: none;
  color: #cccccc;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
}

.close-button:hover {
  background-color: #2a2d33;
  color: #ffffff;
}

.close-button i {
  font-size: 18px;
}

.overlay-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding-bottom: 20px;
}

.overlay-content::-webkit-scrollbar {
  width: 6px;
}

.overlay-content::-webkit-scrollbar-track {
  background: #141519;
}

.overlay-content::-webkit-scrollbar-thumb {
  background: #2a2d33;
  border-radius: 3px;
}

.overlay-content::-webkit-scrollbar-thumb:hover {
  background: #3a3d43;
}

.section-content {
  height: 100%;
}

.mobile-section {
  margin-bottom: 24px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 16px 0;
  padding: 12px 16px;
  font-size: 16px;
  font-weight: 600;
  color: #ffffff;
  background-color: #1a1d23;
  border-bottom: 1px solid #2a2d33;
}

.section-title i {
  color: #007bff;
}

.chart-wrapper {
  padding: 0;
}

.no-chart-message {
  padding: 40px 16px;
  text-align: center;
  color: #888888;
}

.placeholder-content {
  padding: 40px 16px;
  text-align: center;
}

.placeholder-content p {
  margin: 0;
  color: #888888;
  font-size: 16px;
}

/* Enhanced Position Details Styles - Mobile Optimized */
.enhanced-position-details {
  background-color: #0b0d10;
}

.position-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #141519;
  border-bottom: 1px solid #2a2d33;
}

.header-row {
  display: flex;
  align-items: center;
  flex: 1;
}

.header-checkbox {
  position: relative;
  width: 20px;
  height: 20px;
  margin-right: 12px;
}

.header-label {
  font-size: 12px;
  font-weight: 600;
  color: #cccccc;
  text-transform: uppercase;
}

.qty-header {
  min-width: 40px;
  text-align: center;
  margin-right: 12px;
}

.description-header {
  flex: 1;
  margin-right: 12px;
}

.price-header {
  min-width: 60px;
  text-align: right;
}

.position-list {
  max-height: none; /* Allow full height on mobile */
  overflow-y: visible;
}

.enhanced-position-row {
  display: flex;
  align-items: center;
  padding: 12px 16px; /* Increased padding for mobile */
  border-bottom: 1px solid #2a2d33;
  transition: background-color 0.2s ease;
  gap: 12px;
  min-height: 60px; /* Ensure touch-friendly height */
}

.enhanced-position-row:hover {
  background-color: #1a1d23;
}

.enhanced-position-row.selected-position {
  background-color: rgba(0, 123, 255, 0.1);
  border-left: 3px solid #007bff;
}

.enhanced-position-row.existing-position {
  background-color: rgba(255, 193, 7, 0.05);
  border-left: 3px solid rgba(255, 193, 7, 0.3);
}

.enhanced-position-row.existing-position .position-qty {
  color: rgba(255, 193, 7, 0.9);
}

.enhanced-position-row.existing-position .expiry-date {
  background-color: rgba(255, 193, 7, 0.1);
  border: 1px solid rgba(255, 193, 7, 0.3);
}

.position-checkbox {
  position: relative;
  width: 24px; /* Larger for mobile */
  height: 24px;
}

.custom-checkbox {
  opacity: 0;
  position: absolute;
  width: 100%;
  height: 100%;
  cursor: pointer;
}

.checkbox-label {
  position: absolute;
  top: 0;
  left: 0;
  width: 20px; /* Larger for mobile */
  height: 20px;
  border: 2px solid #2a2d33;
  border-radius: 3px;
  background-color: #0b0d10;
  cursor: pointer;
  transition: all 0.2s ease;
}

.custom-checkbox:checked + .checkbox-label {
  background-color: #007bff;
  border-color: #007bff;
}

.custom-checkbox:checked + .checkbox-label::after {
  content: '✓';
  position: absolute;
  top: -2px;
  left: 3px;
  color: white;
  font-size: 14px; /* Larger for mobile */
  font-weight: bold;
}

/* Indeterminate checkbox styling */
.custom-checkbox:indeterminate + .checkbox-label {
  background-color: #007bff;
  border-color: #007bff;
}

.custom-checkbox:indeterminate + .checkbox-label::after {
  content: '−';
  position: absolute;
  top: -4px;
  left: 3px;
  color: white;
  font-size: 16px;
  font-weight: bold;
}

.position-qty {
  min-width: 40px;
  text-align: center;
  font-weight: 600;
  color: #ffffff;
  font-size: 14px; /* Larger for mobile */
}

.position-description {
  flex: 1;
  min-width: 0;
}

.description-main {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.expiry-date {
  font-size: 12px;
  color: #ffffff;
  background-color: #1a1d23;
  padding: 3px 6px; /* Slightly larger for mobile */
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
  font-size: 12px;
  color: #ffffff;
  font-weight: 600;
}

.option-type {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 5px; /* Slightly larger for mobile */
  border-radius: 2px;
  min-width: 18px;
  text-align: center;
}

.option-type.call-type {
  background-color: #28a745;
  color: white;
}

.option-type.put-type {
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

.position-price {
  min-width: 60px;
  text-align: right;
  font-family: monospace;
  font-size: 14px; /* Larger for mobile */
  font-weight: 600;
  color: #ffffff;
}

.no-positions {
  text-align: center;
  padding: 40px 16px;
  color: #888888;
  font-size: 16px;
}
</style>
