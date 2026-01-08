<template>
  <div class="right-panel" :class="{ expanded: isExpanded }">
    <!-- Left Icon Menu (Always Visible) -->
    <div class="left-icon-menu">
      <div
        class="menu-icon"
        :class="{ active: activeSection === 'overview' }"
        @click="toggleSection('overview')"
        title="Overview"
      >
        <i class="pi pi-th-large"></i>
        <span class="icon-label">OVW</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'analysis' }"
        @click="toggleSection('analysis')"
        title="Analysis"
      >
        <i class="pi pi-chart-bar"></i>
        <span class="icon-label">ANL</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'activity' }"
        @click="toggleSection('activity')"
        title="Activity"
      >
        <i class="pi pi-chart-line"></i>
        <span class="icon-label">ACT</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'watchlist' }"
        @click="toggleSection('watchlist')"
        title="Watchlist"
      >
        <i class="pi pi-star"></i>
        <span class="icon-label">WLT</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'alerts' }"
        @click="toggleSection('alerts')"
        title="Alerts"
      >
        <i class="pi pi-clock"></i>
        <span class="icon-label">ALR</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'news' }"
        @click="toggleSection('news')"
        title="News"
      >
        <i class="pi pi-file"></i>
        <span class="icon-label">NWS</span>
      </div>
    </div>

    <!-- Expandable Content Area -->
    <div class="content-area" :class="{ visible: isExpanded }">
      <div class="content-header">
        <h3 class="content-title">{{ getSectionTitle(activeSection) }}</h3>
        <button
          class="collapse-btn"
          @click="collapsePanel"
          title="Collapse Panel"
        >
          <i class="pi pi-chevron-right"></i>
        </button>
      </div>

      <div class="content-body">
        <!-- Overview Section -->
        <div v-if="activeSection === 'overview'" class="section-content">
          <!-- Price Chart Section -->
          <RightPanelSection
            title="Price Chart"
            icon="pi pi-chart-line"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <RightPanelChart
              :symbol="currentSymbol"
              :currentPrice="currentPrice"
              :height="220"
              :livePrice="livePrice"
              :enableRealtime="true"
            />
          </RightPanelSection>

          <!-- Quote Details Section -->
          <RightPanelSection
            title="Quote Details"
            icon="pi pi-list"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <QuoteDetailsSection
              :symbol="currentSymbol"
              :currentPrice="currentPrice"
              :priceChange="priceChange"
              :additionalQuoteData="additionalQuoteData"
            />
          </RightPanelSection>
        </div>

        <!-- Analysis Section -->
        <div v-else-if="activeSection === 'analysis'" class="section-content">
          <RightPanelSection
            title="Payoff Chart"
            icon="pi pi-chart-line"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <div class="chart-wrapper">
              <PayoffChart
                v-if="chartData"
                :chartData="chartData"
                :underlyingPrice="currentPrice"
                title="Position P&L"
                :showInfo="false"
                height="350px"
                :symbol="currentSymbol"
                :isLivePrice="isLivePrice"
              />
              <div v-else class="no-chart-message">
                <p>Select options to view payoff chart</p>
              </div>
            </div>
          </RightPanelSection>

          <RightPanelSection
            title="Position Detail"
            icon="pi pi-briefcase"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <div class="enhanced-position-details">
              <!-- Header with Total P/L -->
              <div class="position-header">
                <div class="header-row">
                  <!-- Select All Checkbox -->
                  <div class="header-checkbox">
                    <input
                      type="checkbox"
                      id="select-all-positions"
                      :checked="isAllSelected"
                      :indeterminate="isIndeterminate"
                      @change="toggleSelectAll"
                      class="custom-checkbox"
                    />
                    <label
                      for="select-all-positions"
                      class="checkbox-label"
                    ></label>
                  </div>

                  <!-- Header labels aligned with position fields -->
                  <span class="header-label qty-header">QTY</span>
                  <span class="header-label description-header"
                    >DESCRIPTION</span
                  >
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
                        :id="`pos-${position.id}`"
                        :checked="checkedPositions.has(position.id)"
                        @change="togglePositionCheck(position.id)"
                        class="custom-checkbox"
                      />
                      <label
                        :for="`pos-${position.id}`"
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
                        <span class="day-indicator"
                          >{{
                            formatEnhancedOptionDescription(position)
                              .daysToExpiry
                          }}d</span
                        >
                        <span class="strike-price">{{
                          formatEnhancedOptionDescription(position).strike
                        }}</span>
                        <span
                          class="option-type"
                          :class="{
                            'call-type':
                              formatEnhancedOptionDescription(position).type ===
                              'C',
                            'put-type':
                              formatEnhancedOptionDescription(position).type ===
                              'P',
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
          </RightPanelSection>
        </div>

        <!-- Activity Section -->
        <div v-else-if="activeSection === 'activity'" class="section-content">
          <ActivitySection :currentSymbol="currentSymbol" />
        </div>

        <!-- Watchlist Section -->
        <div v-else-if="activeSection === 'watchlist'" class="section-content">
          <WatchlistSection />
        </div>

        <!-- Other Sections (Placeholder) -->
        <div v-else class="section-content">
          <div class="placeholder-content">
            <h4>{{ getSectionTitle(activeSection) }}</h4>
            <p>This section is coming soon...</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, reactive } from "vue";
import RightPanelSection from "./RightPanelSection.vue";
import QuoteDetailsSection from "./QuoteDetailsSection.vue";
import RightPanelChart from "./RightPanelChart.vue";
import PayoffChart from "./PayoffChart.vue";
import ActivitySection from "./ActivitySection.vue";
import WatchlistSection from "./WatchlistSection.vue";
import { useMarketData } from "../composables/useMarketData.js";
import { useSelectedLegs } from "../composables/useSelectedLegs.js";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";
import { generateMultiLegPayoff } from "../utils/chartUtils";
import { mapToRootSymbol, isWeeklySymbol, mapToWeeklySymbol } from "../utils/symbolMapping.js";

export default {
  name: "RightPanel",
  components: {
    RightPanelSection,
    QuoteDetailsSection,
    RightPanelChart,
    PayoffChart,
    ActivitySection,
    WatchlistSection,
  },
  props: {
    currentSymbol: {
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
    isLivePrice: {
      type: Boolean,
      default: false,
    },
    chartData: {
      type: Object,
      default: null,
    },
    additionalQuoteData: {
      type: Object,
      default: () => ({}),
    },
    optionsChainData: {
      type: Array,
      default: () => [],
    },
    adjustedNetCredit: {
      type: Number,
      default: null,
    },
    forceExpanded: {
      type: Boolean,
      default: false,
    },
    forceSection: {
      type: String,
      default: null,
    },
  },
  emits: ["panel-collapsed", "positions-changed"],
  setup(props, { emit }) {
    // Use unified market data composable and centralized selected legs
    const { getPositionsForSymbol } = useMarketData();
    const { selectedLegs } = useSelectedLegs();
    const { getOptionPrice: getSmartOptionPrice, getStockPrice } = useSmartMarketData();
    const liveOptionPrices = reactive(new Map());
    const componentId = `RightPanel-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    const activeSection = ref("overview");
    const isExpanded = ref(false);
    const existingPositions = ref([]);
    const checkedPositions = ref(new Set());
    const internalChartData = ref(null); // Internal chart data state

    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    const getLivePrice = (symbol) => {
      if (!symbol) return null;

      if (!liveOptionPrices.has(symbol)) {
        ensureSymbolRegistration(symbol);
        liveOptionPrices.set(symbol, getSmartOptionPrice(symbol));
      }
      return liveOptionPrices.get(symbol)?.value;
    };

    const toggleSection = (section) => {
      if (activeSection.value === section && isExpanded.value) {
        // If clicking the same active section, collapse
        isExpanded.value = false;
      } else {
        // Switch to new section and expand
        activeSection.value = section;
        isExpanded.value = true;
      }
    };

    const collapsePanel = () => {
      isExpanded.value = false;
      emit("panel-collapsed");
    };

    const getSectionTitle = (section) => {
      const titles = {
        overview: "Overview",
        positions: "Positions",
        activity: "Activity",
        watchlist: "Watchlist",
        alerts: "Alerts",
        options: "Options",
        "options-helper": "Options Chain Helper",
        news: "News",
        analysis: "Analysis",
      };
      return titles[section] || "Section";
    };

    const onSectionToggle = (data) => {
      console.log("Section toggled:", data);
    };

    const formatOptionDescription = (option) => {
      const type = option.type?.toUpperCase() || "CALL";
      const strike = option.strike_price || option.strike || 0;
      const expiry = option.expiry || "";
      return `${type} ${strike} ${expiry}`;
    };

    const formatPrice = (price) => {
      if (price === null || price === undefined) return "0.00";
      return Number(price).toFixed(2);
    };

    // Computed property to combine existing positions with selected legs from centralized store
    const allPositions = computed(() => {
      const positions = [];
      const existingSymbols = new Set();
      const symbolGroup = getSymbolGroup(props.currentSymbol);


      // Add existing positions for the current symbol group
      existingPositions.value.forEach((position) => {
        if (symbolGroup.includes(position.underlying_symbol)) {
          // Look up current market price from options chain data for existing positions
          const chainOption = props.optionsChainData.find(
            (opt) => opt.symbol === position.symbol
          );

          // Use entry price as fallback if current price is 0
          let currentPrice =
            position.current_price || position.avg_entry_price || 0;
          let unrealizedPL = position.unrealized_pl || 0;

          // If we don't have current P&L but have entry price, estimate it
          if (
            unrealizedPL === 0 &&
            position.avg_entry_price &&
            position.avg_entry_price > 0
          ) {
            // For now, assume current price equals entry price (no P&L change)
            currentPrice = position.avg_entry_price;
            unrealizedPL = 0; // No change assumed
          }

          // Update current price and P&L if we have chain data
          if (
            chainOption &&
            position.avg_entry_price &&
            chainOption.bid &&
            chainOption.ask
          ) {
            // Use mid price for current market value
            currentPrice = (chainOption.bid + chainOption.ask) / 2;

            // Recalculate P&L with current market price
            const qty = position.qty;
            if (qty > 0) {
              // Long position: (current_price - entry_price) * qty * 100
              unrealizedPL =
                (currentPrice - position.avg_entry_price) * qty * 100;
            } else {
              // Short position: (entry_price - current_price) * |qty| * 100
              unrealizedPL =
                (position.avg_entry_price - currentPrice) * Math.abs(qty) * 100;
            }

          }

          const positionData = {
            ...position,
            id: `existing:${position.symbol || position.id}`,
            current_price: currentPrice,
            unrealized_pl: unrealizedPL,
            isExisting: true,
            isSelected: false,
          };

          positions.push(positionData);
          existingSymbols.add(position.symbol);
        }
      });

      // Add selected legs from centralized store as new positions (allow duplicates for closing positions)
      selectedLegs.value.forEach((leg, index) => {
        const livePrice = getLivePrice(leg.symbol);
        let currentPrice = 0;
        if (livePrice) {
            currentPrice = livePrice.price ?? 0;
        } else {
            currentPrice = leg.current_price || ((leg.bid + leg.ask) / 2) || 0;
        }

        positions.push({
          id: `selected:${leg.symbol}:${index}`, // FIXED: Include index to make IDs unique for duplicate symbols
          symbol: leg.symbol,
          asset_class: "us_option", // Required for chart
          qty: leg.side === "buy" ? leg.quantity : -leg.quantity,
          strike_price: leg.strike_price,
          option_type: leg.type,
          expiry_date: leg.expiry,
          current_price: currentPrice,
          avg_entry_price: leg.avg_entry_price,
          unrealized_pl: 0,
          isExisting: false,
          isSelected: true,
        });
      });


      return positions;
    });

    // Method to toggle position checkbox
    const togglePositionCheck = (positionId) => {
      if (checkedPositions.value.has(positionId)) {
        checkedPositions.value.delete(positionId);
      } else {
        checkedPositions.value.add(positionId);
      }
      // Manually trigger chart update after user interaction
      updateChartWithCheckedPositions();
    };

    // Method to update chart based on checked positions
    const updateChartWithCheckedPositions = () => {
      // Force fresh price lookup by clearing and re-registering live price cache
      // This ensures we get current market prices, just like manual checkbox interactions
      liveOptionPrices.clear();
      
      const checkedPositionsList = allPositions.value.filter((pos) =>
        checkedPositions.value.has(pos.id)
      );

      
      // Always emit positions-changed, even if empty array
      // This tells the parent that user has made explicit checkbox selections
      emit("positions-changed", checkedPositionsList);
    };

    // Computed property for select-all checkbox state
    const isAllSelected = computed(() => {
      if (allPositions.value.length === 0) return false;
      return allPositions.value.every((pos) =>
        checkedPositions.value.has(pos.id)
      );
    });

    // Computed property for indeterminate state (some but not all selected)
    const isIndeterminate = computed(() => {
      if (allPositions.value.length === 0) return false;
      const checkedCount = allPositions.value.filter((pos) =>
        checkedPositions.value.has(pos.id)
      ).length;
      return checkedCount > 0 && checkedCount < allPositions.value.length;
    });

    // Method to toggle select all positions
    const toggleSelectAll = () => {
      if (isAllSelected.value) {
        // Uncheck all
        checkedPositions.value.clear();
      } else {
        // Check all
        allPositions.value.forEach((pos) => {
          checkedPositions.value.add(pos.id);
        });
      }
      updateChartWithCheckedPositions();
    };

    // Method to determine if option is ITM
    const isInTheMoney = (position) => {
      if (!position.strike_price || !props.currentPrice) return false;

      if (position.option_type === "call" || position.option_type === "C") {
        return props.currentPrice > position.strike_price;
      } else if (
        position.option_type === "put" ||
        position.option_type === "P"
      ) {
        return props.currentPrice < position.strike_price;
      }
      return false;
    };

    // Method to format option description with visual indicators
    const formatEnhancedOptionDescription = (position) => {
      const type = position.option_type?.toUpperCase() || "CALL";
      const strike = position.strike_price || 0;
      const expiry = position.expiry_date || "";


      // Format expiry date (e.g., "Jul 14")
      let formattedExpiry = expiry;
      let daysToExpiry = 0;

      if (expiry && expiry.includes("-")) {
        // Handle YYYY-MM-DD format - parse as UTC to avoid timezone issues (same as BottomTradingPanel)
        const [year, month, day] = expiry.split("-").map(Number);
        const expiryDate = new Date(Date.UTC(year, month - 1, day));
        
        // Get current date for comparison
        const currentDate = new Date();
        currentDate.setHours(0, 0, 0, 0);

        // Calculate days difference
        const timeDiff = expiryDate.getTime() - currentDate.getTime();
        daysToExpiry = Math.ceil(timeDiff / (1000 * 3600 * 24));

        // Format the date using UTC timezone (same as BottomTradingPanel)
        formattedExpiry = expiryDate.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
          timeZone: "UTC",
        });
      }

      return {
        type: type === "CALL" || type === "C" ? "C" : "P",
        strike,
        expiry: formattedExpiry,
        daysToExpiry: Math.max(0, daysToExpiry), // Don't show negative days
        isITM: isInTheMoney(position),
      };
    };

    // Helper function to get symbol group (handles weekly symbols grouping)
    const getSymbolGroup = (symbol) => {
      if (!symbol) return [symbol];

      // If symbol is a weekly variant (e.g., SPXW, NDXP), include both it and its root
      if (isWeeklySymbol(symbol)) {
        const rootSymbol = mapToRootSymbol(symbol);
        return [symbol, rootSymbol];
      }

      // If symbol is a root with a weekly equivalent (e.g., SPX, NDX), include both
      const weeklySymbol = mapToWeeklySymbol(symbol);
      if (weeklySymbol !== symbol) {
        return [symbol, weeklySymbol];
      }

      // No weekly equivalent
      return [symbol];
    };

    // Helper function to extract underlying symbol from option symbol
    const extractUnderlyingFromOptionSymbol = (optionSymbol) => {
      // Option symbols are like "SPY250714C00624000"
      // Extract the underlying symbol (SPY) from the beginning
      const match = optionSymbol.match(/^([A-Z]+)/);
      return match ? match[1] : null;
    };

    // Helper function to parse option symbol for details
    const parseOptionSymbol = (optionSymbol) => {
      try {
        // Handle multiple option symbol formats
        // Format 1: SPY250714C00624000 (SYMBOL + YYMMDD + C/P + STRIKE with 8 digits)
        let match = optionSymbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
        if (match) {
          const [, underlying, dateStr, type, strikeStr] = match;

          // Parse date: YYMMDD -> YYYY-MM-DD
          const year = 2000 + parseInt(dateStr.substring(0, 2));
          const month = dateStr.substring(2, 4);
          const day = dateStr.substring(4, 6);
          const expiry = `${year}-${month}-${day}`;

          // Parse strike: 8 digits with 3 decimal places (e.g., 00624000 = 624.000)
          const strike = parseInt(strikeStr) / 1000;

          return {
            underlying_symbol: underlying,
            option_type: type === "C" ? "call" : "put",
            strike_price: strike,
            expiry_date: expiry,
          };
        }

        // Format 2: SPY_250714_C_624 (SYMBOL_YYMMDD_C/P_STRIKE)
        match = optionSymbol.match(/^([A-Z]+)_(\d{6})_([CP])_(\d+(?:\.\d+)?)$/);
        if (match) {
          const [, underlying, dateStr, type, strikeStr] = match;

          // Parse date: YYMMDD -> YYYY-MM-DD
          const year = 2000 + parseInt(dateStr.substring(0, 2));
          const month = dateStr.substring(2, 4);
          const day = dateStr.substring(4, 6);
          const expiry = `${year}-${month}-${day}`;

          const strike = parseFloat(strikeStr);

          return {
            underlying_symbol: underlying,
            option_type: type === "C" ? "call" : "put",
            strike_price: strike,
            expiry_date: expiry,
          };
        }

        // Format 3: Try to extract from any format with underscores or other separators
        // Look for patterns like SYMBOL_anything_C/P_STRIKE or similar
        const parts = optionSymbol.split(/[_\-]/);
        if (parts.length >= 3) {
          const underlying = parts[0];
          let type = null;
          let strike = null;

          // Find C or P in the parts
          for (let i = 1; i < parts.length; i++) {
            if (parts[i] === "C" || parts[i] === "P") {
              type = parts[i] === "C" ? "call" : "put";
              // Look for strike in the next part
              if (i + 1 < parts.length) {
                const strikeCandidate = parseFloat(parts[i + 1]);
                if (!isNaN(strikeCandidate)) {
                  strike = strikeCandidate;
                }
              }
              break;
            }
          }

          if (type && strike) {
            return {
              underlying_symbol: underlying,
              option_type: type,
              strike_price: strike,
              expiry_date: null, // Will be filled from position data if available
            };
          }
        }

        console.warn(`Could not parse option symbol: ${optionSymbol}`);
        return null;
      } catch (error) {
        console.error("Error parsing option symbol:", optionSymbol, error);
        return null;
      }
    };


    // Watch for changes in positions to update checked state
    watch(
      () => allPositions.value,
      (newPositions, oldPositions) => {
        let hasNewPositions = false;
        let hasRemovedPositions = false;
        
        // Only auto-check NEW positions that weren't there before
        // Don't re-check positions that the user has manually unchecked
        if (oldPositions) {
          const oldIds = new Set(oldPositions.map(pos => pos.id));
          const newIds = new Set(newPositions.map(pos => pos.id));
          
          // Check for new positions to auto-check
          newPositions.forEach((pos) => {
            // Only auto-check if this is a completely new position AND it's a selected leg (new trade)
            // Do NOT auto-check existing positions - let user manually check them if they want to see impact
            if (!oldIds.has(pos.id) && pos.isSelected && !pos.isExisting) {
              checkedPositions.value.add(pos.id);
              hasNewPositions = true;
            }
          });
          
          // CRITICAL FIX: Remove checked positions that no longer exist (legs removed from options chain)
          oldPositions.forEach((pos) => {
            if (!newIds.has(pos.id) && checkedPositions.value.has(pos.id)) {
              checkedPositions.value.delete(pos.id);
              hasRemovedPositions = true;
            }
          });
        } else {
          // Initial load - only check selected legs (new trades), NOT existing positions
          newPositions.forEach((pos) => {
            if (pos.isSelected && !pos.isExisting) {
              checkedPositions.value.add(pos.id);
              hasNewPositions = true;
            }
          });
        }
        
        // If we auto-checked new positions OR removed positions, force fresh pricing and chart update
        // This ensures the chart reflects the current leg selection from options chain
        if ((hasNewPositions || hasRemovedPositions) && (checkedPositions.value.size > 0 || hasRemovedPositions)) {
          // Use nextTick to ensure the DOM and reactive state are updated, then force fresh pricing
          setTimeout(() => {
            updateChartWithCheckedPositions();
          }, 0);
        }
      },
      { deep: true }
    );

    // Ensure parent clears any cached/checked positions when there are no rows to show
    // This prevents stale "fake legs" from continuing to drive the payoff chart.
    watch(
      () => allPositions.value.length,
      (len) => {
        if (len === 0) {
          checkedPositions.value.clear();
          emit("positions-changed", []);
        }
      }
    );

    // Defensive: if parent clears chartData, also clear any checked state
    // so a subsequent selection starts from a clean slate.
    watch(
      () => props.chartData,
      (val) => {
        if (!val) {
          checkedPositions.value.clear();
        }
      }
    );

    // Watch for position data changes (reactive updates from global store)
    // This single watcher handles both symbol changes and position data updates
    watch(
      () => {
        const positionsComputed = getPositionsForSymbol(props.currentSymbol);
        return positionsComputed.value; // Get the actual value from the computed
      },
      (positionsData) => {
        // Only clear checked positions on symbol change, not on data refresh
        // We'll let the allPositions watcher handle the checkbox state properly
        
        if (positionsData?.positions && Array.isArray(positionsData.positions)) {          
          existingPositions.value = positionsData.positions.map((pos) => {
            // Parse option symbol to get missing details if needed
            const parsedOption = parseOptionSymbol(pos.symbol);
            
            return {
              id: pos.symbol,
              symbol: pos.symbol,
              asset_class: pos.asset_class || "us_option",
              underlying_symbol: pos.underlying_symbol,
              qty: pos.qty,
              strike_price: parsedOption?.strike_price || pos.strike_price,
              option_type: parsedOption?.option_type || pos.option_type,
              expiry_date: parsedOption?.expiry_date || pos.expiry_date,
              current_price: pos.current_price,
              avg_entry_price: pos.avg_entry_price || 
                (pos.cost_basis ? Math.abs(pos.cost_basis / (pos.qty * 100)) : 0),
              unrealized_pl: pos.unrealized_pl || 0,
              isExisting: true,
              isSelected: false,
            };
          });
          
        } else {
          existingPositions.value = [];
        }
      },
      { deep: true, immediate: true }
    );

    // Watch for symbol changes to clear checkboxes only when symbol changes
    // BUT preserve checkboxes for selected legs that should remain checked
    watch(
      () => props.currentSymbol,
      (newSymbol, oldSymbol) => {
        if (newSymbol !== oldSymbol) {
          // Don't clear checkboxes if we have selected legs that should remain checked
          // This prevents the race condition where symbol change clears selections
          // before the leg selection has been processed
          if (selectedLegs.value.length === 0) {
            checkedPositions.value.clear();
          }
        }
      }
    );

    // Watch for forced expansion and section changes
    watch(
      () => props.forceExpanded,
      (newValue) => {
        isExpanded.value = newValue;
      },
      { immediate: true }
    );

    watch(
      () => props.forceSection,
      (newSection) => {
        if (newSection) {
          activeSection.value = newSection;
        }
      },
      { immediate: true }
    );

    // Component mounted - positions will be automatically loaded via watcher
    onMounted(() => {

    });

    // Chart data is solely controlled by the parent (OptionsTrading)
    // Avoid any internal caching here to prevent stale charts when no legs are selected.
    const chartData = computed(() => props.chartData);

    // Live price subscription for chart updates (similar to ChartView)
    const livePrice = computed(() => {
      if (!props.currentSymbol) return null;
      const priceData = getStockPrice(props.currentSymbol);
      return priceData?.value || null;
    });

    // REMOVED: This watcher was causing race conditions with the parent's flag system
    // The parent (OptionsTrading) now properly manages the priority between selectedLegs and position selections
    // watch(
    //   selectedLegs,
    //   (newLegs) => {
    //     if (newLegs.length === 0) {
    //       checkedPositions.value.clear();
    //       emit("positions-changed", []);
    //     }
    //   },
    //   { deep: true }
    // );

    return {
      activeSection,
      isExpanded,
      existingPositions,
      checkedPositions,
      allPositions,
      isAllSelected,
      isIndeterminate,
      chartData, // Use computed chartData instead of prop
      livePrice, // Live price data for chart updates
      toggleSection,
      collapsePanel,
      getSectionTitle,
      onSectionToggle,
      formatOptionDescription,
      formatPrice,
      togglePositionCheck,
      toggleSelectAll,
      updateChartWithCheckedPositions,
      isInTheMoney,
      formatEnhancedOptionDescription,
    };
  },
};
</script>

<style scoped>
.right-panel {
  display: flex;
  background-color: var(--bg-primary, #0b0d10);
  border-left: 1px solid var(--border-primary, #1a1d23);
  height: 100%;
  position: relative;
  z-index: 10;
}

.right-panel.expanded {
  width: 700px;
  max-width: calc(100vw - 60px); /* Prevent overflow, account for SideNav */
}

/* Responsive breakpoints to prevent black bar issue */
@media (max-width: 1200px) {
  .right-panel.expanded {
    width: 500px;
  }
}

@media (max-width: 900px) {
  .right-panel.expanded {
    width: 400px;
  }
}

@media (max-width: 768px) {
  .right-panel {
    display: none; /* Hide on mobile - use mobile overlay instead */
  }
}

.left-icon-menu {
  width: 60px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-primary, #0b0d10);
  border-right: 1px solid var(--border-secondary, #2a2d33);
  padding-top: 8px;
  height: 100%;
}

.menu-icon {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 60px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  border-radius: 4px;
  margin: 2px 4px;
  position: relative;
}

.menu-icon:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.menu-icon.active {
  background-color: var(--bg-quaternary, #2a2d33);
}

.menu-icon i {
  font-size: 16px;
  color: var(--text-secondary, #cccccc);
  margin-bottom: 2px;
}

.menu-icon.active i {
  color: var(--text-primary, #ffffff);
}

.icon-label {
  font-size: 9px;
  color: var(--text-tertiary, #888888);
  font-weight: 500;
  text-align: center;
  line-height: 1;
}

.menu-icon.active .icon-label {
  color: var(--text-secondary, #cccccc);
}

.content-area {
  display: none; /* Hidden by default */
  flex-direction: column;
  width: 640px; /* Fixed width */
  flex-shrink: 0;
  background-color: #141519;
  border-left: 1px solid #2a2d33;
}

.content-area.visible {
  display: flex; /* Shown when visible */
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: var(--bg-secondary, #141519);
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
  flex-shrink: 0;
}

.content-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
}

.collapse-btn {
  background: var(--bg-tertiary, #1a1d23);
  border: 1px solid var(--border-secondary, #2a2d33);
  color: var(--text-secondary, #cccccc);
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
}

.collapse-btn:hover {
  background-color: var(--bg-quaternary, #2a2d33);
  color: var(--text-primary, #ffffff);
}

.content-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

.content-body::-webkit-scrollbar {
  width: 6px;
}

.content-body::-webkit-scrollbar-track {
  background: var(--bg-secondary, #141519);
}

.content-body::-webkit-scrollbar-thumb {
  background: var(--border-secondary, #2a2d33);
  border-radius: 3px;
}

.content-body::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary, #3a3d43);
}

.section-content {
  height: 100%;
}

.chart-wrapper {
  padding: 0;
}

.no-chart-message {
  padding: 40px 16px;
  text-align: center;
  color: var(--text-tertiary, #888888);
}

.position-details {
  padding: 16px;
}

.position-summary {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.summary-row {
  display: grid;
  grid-template-columns: 60px 1fr 80px 80px;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #cccccc);
}

.position-row {
  display: grid;
  grid-template-columns: 60px 1fr 80px 80px;
  gap: 12px;
  padding: 8px 0;
  font-size: 12px;
  color: var(--text-primary, #ffffff);
}

.no-positions {
  text-align: center;
  padding: 40px 16px;
  color: var(--text-tertiary, #888888);
  font-size: 14px;
}

.placeholder-content {
  padding: 40px 16px;
  text-align: center;
}

.placeholder-content h4 {
  margin: 0 0 16px 0;
  color: var(--text-primary, #ffffff);
}

.placeholder-content p {
  margin: 0;
  color: var(--text-tertiary, #888888);
}

.qty {
  text-align: center;
}

.description {
  font-size: 11px;
}

.trade-price,
.mark {
  text-align: right;
  font-family: monospace;
}

/* Enhanced Position Details Styles */
.enhanced-position-details {
  background-color: var(--bg-primary, #0b0d10);
}

.position-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: var(--bg-secondary, #141519);
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
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
  color: var(--text-secondary, #cccccc);
  text-transform: uppercase;
}

.qty-header {
  min-width: 30px;
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

/* Indeterminate checkbox styling */
.custom-checkbox:indeterminate + .checkbox-label {
  background-color: #007bff;
  border-color: #007bff;
}

.custom-checkbox:indeterminate + .checkbox-label::after {
  content: "−";
  position: absolute;
  top: -4px;
  left: 2px;
  color: white;
  font-size: 16px;
  font-weight: bold;
}

.total-pl {
  font-size: 16px;
  font-weight: 600;
  font-family: monospace;
}

.total-pl.positive {
  color: #00c851;
}

.total-pl.negative {
  color: #ff4444;
}

.position-list {
  max-height: 400px;
  overflow-y: auto;
}

.enhanced-position-row {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
  transition: background-color 0.2s ease;
  gap: 12px;
}

.enhanced-position-row:hover {
  background-color: var(--bg-tertiary, #1a1d23);
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
  width: 20px;
  height: 20px;
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
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-secondary, #2a2d33);
  border-radius: 3px;
  background-color: var(--bg-primary, #0b0d10);
  cursor: pointer;
  transition: all 0.2s ease;
}

.custom-checkbox:checked + .checkbox-label {
  background-color: #007bff;
  border-color: #007bff;
}

.custom-checkbox:checked + .checkbox-label::after {
  content: "✓";
  position: absolute;
  top: -2px;
  left: 2px;
  color: white;
  font-size: 12px;
  font-weight: bold;
}

.position-qty {
  min-width: 30px;
  text-align: center;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
}

.position-description {
  flex: 1;
  min-width: 0;
}

.description-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.expiry-date {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  background-color: var(--bg-tertiary, #1a1d23);
  padding: 2px 6px;
  border-radius: 3px;
}

.day-indicator {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  background-color: var(--bg-secondary, #141519);
  padding: 1px 4px;
  border-radius: 2px;
}

.strike-price {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  font-weight: 600;
}

.option-type {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 4px;
  border-radius: 2px;
  min-width: 16px;
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
  padding: 1px 4px;
  border-radius: 2px;
  text-transform: uppercase;
}

.position-pl {
  min-width: 60px;
  text-align: right;
  font-family: monospace;
  font-size: 12px;
  font-weight: 600;
}

.position-pl.positive {
  color: #00c851;
}

.position-pl.negative {
  color: #ff4444;
}

.position-price {
  min-width: 60px;
  text-align: right;
  font-family: monospace;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
}
</style>
