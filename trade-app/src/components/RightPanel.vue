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
        :class="{ active: activeSection === 'options' }"
        @click="toggleSection('options')"
        title="Options Chain Helper"
      >
        <i class="pi pi-calendar"></i>
        <span class="icon-label">TTL</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'options-helper' }"
        @click="toggleSection('options-helper')"
        title="Options Helper"
      >
        <i class="pi pi-link"></i>
        <span class="icon-label">OCH</span>
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
import { ref, computed, watch, onMounted } from "vue";
import RightPanelSection from "./RightPanelSection.vue";
import QuoteDetailsSection from "./QuoteDetailsSection.vue";
import PayoffChart from "./PayoffChart.vue";
import ActivitySection from "./ActivitySection.vue";
import { useMarketData } from "../composables/useMarketData.js";
import { generateMultiLegPayoff } from "../utils/chartUtils";

export default {
  name: "RightPanel",
  components: {
    RightPanelSection,
    QuoteDetailsSection,
    PayoffChart,
    ActivitySection,
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
    selectedOptions: {
      type: Array,
      default: () => [],
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
    // Use unified market data composable
    const { getPositions } = useMarketData();

    const activeSection = ref("overview");
    const isExpanded = ref(false);
    const existingPositions = ref([]);
    const checkedPositions = ref(new Set());
    const internalChartData = ref(null); // Internal chart data state

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

    // Computed property to combine existing positions with selected options
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

            console.log(`Updated existing position ${position.symbol}:`, {
              newCurrentPrice: currentPrice,
              newUnrealizedPL: unrealizedPL,
              calculation: {
                qty,
                entryPrice: position.avg_entry_price,
                priceDiff:
                  qty > 0
                    ? currentPrice - position.avg_entry_price
                    : position.avg_entry_price - currentPrice,
                multiplier: 100,
              },
            });
          }

          const positionData = {
            ...position,
            id: position.symbol || position.id,
            current_price: currentPrice,
            unrealized_pl: unrealizedPL,
            isExisting: true,
            isSelected: false,
          };

          positions.push(positionData);
          existingSymbols.add(position.symbol);
        }
      });

      // Add selected options as new positions (only if they don't already exist)
      props.selectedOptions.forEach((option) => {
        // Skip if this option already exists as a position
        if (!existingSymbols.has(option.symbol)) {
          // Look up current market price from options chain data
          const chainOption = props.optionsChainData.find(
            (opt) => opt.symbol === option.symbol
          );

          let currentPrice = 0;
          let avgEntryPrice = 0;
          let unrealizedPL = 0;

          if (chainOption) {
            // Use ask price for long positions, bid price for short positions
            const entryPrice =
              option.side === "buy" ? chainOption.ask : chainOption.bid;
            const marketPrice =
              option.side === "buy" ? chainOption.bid : chainOption.ask; // Current market price for exit

            currentPrice = marketPrice || entryPrice || 0;
            avgEntryPrice = entryPrice || 0;

            // Calculate unrealized P&L
            if (avgEntryPrice > 0) {
              const qty =
                option.side === "buy" ? option.quantity : -option.quantity;
              // For long positions: (current_price - entry_price) * qty * 100
              // For short positions: (entry_price - current_price) * |qty| * 100
              if (qty > 0) {
                // Long position
                unrealizedPL = (currentPrice - avgEntryPrice) * qty * 100;
              } else {
                // Short position
                unrealizedPL =
                  (avgEntryPrice - currentPrice) * Math.abs(qty) * 100;
              }
            }
          }

          positions.push({
            id: option.symbol,
            symbol: option.symbol,
            asset_class: "us_option", // Required for chart
            qty: option.side === "buy" ? option.quantity : -option.quantity,
            strike_price: option.strike_price || option.strike,
            option_type: option.type,
            expiry_date: option.expiry,
            current_price: currentPrice,
            avg_entry_price: avgEntryPrice,
            unrealized_pl: unrealizedPL,
            isExisting: false,
            isSelected: true,
          });
        }
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
      const checkedPositionsList = allPositions.value.filter((pos) =>
        checkedPositions.value.has(pos.id)
      );

      console.log("🔍 RightPanel: Updating chart with checked positions:", checkedPositionsList.map(pos => ({
        id: pos.id,
        symbol: pos.symbol,
        qty: pos.qty,
        strike_price: pos.strike_price,
        option_type: pos.option_type,
        avg_entry_price: pos.avg_entry_price,
        current_price: pos.current_price,
        asset_class: pos.asset_class,
        isExisting: pos.isExisting,
        isSelected: pos.isSelected
      })));

      // Emit positions to parent for chart generation
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
        // Parse date components directly to avoid timezone issues
        const [year, month, day] = expiry.split("-").map(Number);

        // Create date objects in local timezone
        const expiryDate = new Date(year, month - 1, day); // month is 0-indexed
        const currentDate = new Date();

        // Set both dates to midnight to get accurate day difference
        expiryDate.setHours(0, 0, 0, 0);
        currentDate.setHours(0, 0, 0, 0);

        // Calculate days difference
        const timeDiff = expiryDate.getTime() - currentDate.getTime();
        daysToExpiry = Math.ceil(timeDiff / (1000 * 3600 * 24));

        // Format the date directly from components to avoid timezone conversion
        formattedExpiry = expiryDate.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
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

    // Helper function to get symbol group (handles SPX/SPXW grouping)
    const getSymbolGroup = (symbol) => {
      if (symbol === "SPX" || symbol === "SPXW") {
        return ["SPX", "SPXW"];
      }
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

    // Get reactive positions data (auto-updates every 30 seconds)
    const reactivePositions = getPositions();

    // Process positions data - defined before watcher to avoid hoisting issues
    const processPositionsData = (response) => {
      try {
        // Handle the new enhanced response structure
        let positions = [];
        if (response && response.enhanced && response.symbol_groups) {
          // New hierarchical structure - extract individual positions from symbol groups
          const symbolGroup = getSymbolGroup(props.currentSymbol);

          response.symbol_groups.forEach((symbolGroupData) => {
            // Check if this symbol group is for the current symbol
            if (
              symbolGroup.includes(symbolGroupData.symbol) &&
              symbolGroupData.asset_class === "options"
            ) {
              // Extract positions from all strategies within this symbol group
              symbolGroupData.strategies.forEach((strategy) => {
                strategy.legs.forEach((leg) => {
                  // Parse option symbol to get missing details
                  const parsedOption = parseOptionSymbol(leg.symbol);

                  positions.push({
                    id: leg.symbol,
                    symbol: leg.symbol,
                    asset_class: "us_option", // Required for chart
                    underlying_symbol: symbolGroupData.symbol,
                    qty: leg.qty,
                    strike_price: parsedOption?.strike_price,
                    option_type: parsedOption?.option_type,
                    expiry_date: parsedOption?.expiry_date,
                    current_price: leg.current_price,
                    avg_entry_price:
                      leg.avg_entry_price ||
                      (leg.cost_basis
                        ? Math.abs(leg.cost_basis / (leg.qty * 100))
                        : 0),
                    unrealized_pl: leg.unrealized_pl || 0,
                    isExisting: true,
                    isSelected: false,
                  });
                });
              });
            }
          });
        } else if (response && response.enhanced && response.position_groups) {
          // Old enhanced structure - extract individual positions from groups
          const symbolGroup = getSymbolGroup(props.currentSymbol);

          response.position_groups.forEach((group) => {
            // Check if this group is for the current symbol
            if (
              symbolGroup.includes(group.symbol) &&
              group.asset_class === "options"
            ) {
              // Add each leg as an individual position
              group.legs.forEach((leg) => {
                // Parse option symbol to get missing details
                const parsedOption = parseOptionSymbol(leg.symbol);

                positions.push({
                  id: leg.symbol,
                  symbol: leg.symbol,
                  asset_class: "us_option", // Required for chart
                  underlying_symbol: group.symbol,
                  qty: leg.qty,
                  strike_price: parsedOption?.strike_price,
                  option_type: parsedOption?.option_type,
                  expiry_date: parsedOption?.expiry_date,
                  current_price: leg.current_price,
                  avg_entry_price: leg.avg_entry_price,
                  unrealized_pl: leg.unrealized_pl || 0,
                  isExisting: true,
                  isSelected: false,
                });
              });
            }
          });
        } else if (
          response &&
          response.positions &&
          Array.isArray(response.positions)
        ) {
          // Fallback to old structure
          const symbolGroup = getSymbolGroup(props.currentSymbol);

          positions = response.positions
            .filter((pos) => {
              const isOption = pos.asset_class === "us_option";
              const underlyingFromSymbol = extractUnderlyingFromOptionSymbol(
                pos.symbol
              );
              const isCurrentSymbolGroup =
                symbolGroup.includes(underlyingFromSymbol);
              return isOption && isCurrentSymbolGroup;
            })
            .map((pos) => {
              const parsedOption = parseOptionSymbol(pos.symbol);
              return {
                id: pos.symbol,
                symbol: pos.symbol,
                asset_class: "us_option", // Required for chart
                underlying_symbol:
                  parsedOption?.underlying_symbol ||
                  extractUnderlyingFromOptionSymbol(pos.symbol),
                qty: pos.qty,
                strike_price: parsedOption?.strike_price,
                option_type: parsedOption?.option_type,
                expiry_date: parsedOption?.expiry_date,
                current_price: pos.current_price,
                avg_entry_price: pos.avg_entry_price,
                unrealized_pl: pos.unrealized_pl || 0,
                isExisting: true,
                isSelected: false,
              };
            });
        }

        existingPositions.value = positions;
      } catch (error) {
        console.error("Error fetching existing positions:", error);
        console.error("Error details:", error.response?.data || error.message);
        existingPositions.value = [];
      }
    };

    // Watch for changes in reactive positions data - now after function is defined
    watch(
      reactivePositions,
      (response) => {
        processPositionsData(response);
      },
      { immediate: true }
    );

    // Initialize checked positions when component mounts or positions change
    const initializeCheckedPositions = () => {
      // Auto-check all positions initially
      allPositions.value.forEach((pos) => {
        checkedPositions.value.add(pos.id);
      });
    };

    // Watch for changes in positions to update checked state
    watch(
      () => allPositions.value,
      (newPositions, oldPositions) => {
        let hasNewPositions = false;
        
        // Only auto-check NEW positions that weren't there before
        // Don't re-check positions that the user has manually unchecked
        if (oldPositions) {
          const oldIds = new Set(oldPositions.map(pos => pos.id));
          newPositions.forEach((pos) => {
            // Only auto-check if this is a completely new position
            if (!oldIds.has(pos.id) && (pos.isSelected || pos.isExisting)) {
              checkedPositions.value.add(pos.id);
              hasNewPositions = true;
            }
          });
        } else {
          // Initial load - check all positions
          newPositions.forEach((pos) => {
            if (pos.isSelected || pos.isExisting) {
              checkedPositions.value.add(pos.id);
              hasNewPositions = true;
            }
          });
        }
        
        // If we auto-checked new positions, update the chart to include all checked positions
        if (hasNewPositions && checkedPositions.value.size > 0) {
          // Use nextTick to ensure the DOM and reactive state are updated
          setTimeout(() => {
            updateChartWithCheckedPositions();
          }, 0);
        }
      },
      { deep: true }
    );

    // Watch for symbol changes - positions will be automatically updated via reactive data
    watch(
      () => props.currentSymbol,
      (newSymbol) => {
        console.log("🔄 RightPanel: Symbol changed to:", newSymbol);
        // Positions will be automatically updated via the reactive positions watcher
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

    // Computed property to use either internal chart data or prop
    const chartData = computed(() => {
      // Prioritize internal chart data (from selected positions)
      // Fall back to prop chart data (from parent component like OptionsTrading)
      return internalChartData.value || props.chartData;
    });

    return {
      activeSection,
      isExpanded,
      existingPositions,
      checkedPositions,
      allPositions,
      isAllSelected,
      isIndeterminate,
      chartData, // Use computed chartData instead of prop
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
      initializeCheckedPositions,
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
