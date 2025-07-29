<template>
  <div class="positions-view-container">
    <!-- Top Bar -->
    <TopBar />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Left Navigation -->
      <SideNav />

      <!-- Content Area -->
      <div class="content-area">
        <!-- Symbol Header -->
        <SymbolHeader
          :currentSymbol="currentSymbol"
          :companyName="companyName"
          :exchange="exchange"
          :currentPrice="currentPrice"
          :priceChange="priceChange"
          :priceChangePercent="priceChangePercent"
          :isLivePrice="isLivePrice"
          :marketStatus="marketStatus"
          :showTradeMode="false"
        />

        <!-- Header Section -->
        <div class="positions-header">
          <div class="header-left">
            <div class="position-type-tabs">
              <button
                :class="['tab-button', { active: selectedType === 'options' }]"
                @click="selectedType = 'options'"
              >
                Options
              </button>
              <button
                :class="['tab-button', { active: selectedType === 'stocks' }]"
                @click="selectedType = 'stocks'"
              >
                Stocks
              </button>
            </div>
          </div>

          <div class="header-right">
            <div class="group-by-section">
              <label for="groupBy">Group By:</label>
              <select id="groupBy" v-model="groupBy" class="group-by-select">
                <option value="order_chains">Order Chains</option>
                <option value="symbol">Symbol</option>
                <option value="strategy">Strategy</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Loading State -->
        <div v-if="loading" class="loading-state">
          <div class="loading-spinner"></div>
          <p>Loading positions...</p>
        </div>

        <!-- Error State -->
        <div v-else-if="error" class="error-state">
          <p>{{ error }}</p>
          <button @click="fetchPositions" class="retry-button">Retry</button>
        </div>

        <!-- Content when positions exist -->
        <template v-else-if="filteredPositionGroups.length > 0">
          <!-- Totals Row -->
          <div class="totals-section">
            <div class="totals-row">
              <div class="totals-label">TOTALS</div>
              <div class="totals-values">
                <div
                  class="total-pl-day"
                  :class="totalPlDay >= 0 ? 'profit' : 'loss'"
                >
                  {{ formatCurrency(totalPlDay) }}
                </div>
                <div
                  class="total-pl-open"
                  :class="totalPlOpen >= 0 ? 'profit' : 'loss'"
                >
                  {{ formatCurrency(totalPlOpen) }}
                </div>
              </div>
            </div>
          </div>

          <!-- Positions Table -->
          <div class="positions-table-container">
            <div class="positions-table-scroll">
              <table class="positions-table">
                <thead>
                  <tr>
                    <th class="symbol-header">Symbol</th>
                    <th class="pl-day-header">P/L Day</th>
                    <th class="pl-open-header">P/L Open</th>
                    <th class="bid-header">Bid (Sell)</th>
                    <th class="ask-header">Ask (Buy)</th>
                    <th class="trd-prc-header">Trd Prc</th>
                    <th class="dsopn-header">D'sOpn</th>
                    <th class="beta-delta-header">βΔ</th>
                    <th class="dte-header">DTE</th>
                    <th class="pop-header">PoP</th>
                    <th class="indctr-header">Indctr</th>
                  </tr>
                </thead>
                <tbody>
                  <template
                    v-for="group in filteredPositionGroups"
                    :key="group.id"
                  >
                    <!-- Collapsed Group Row -->
                    <tr
                      class="position-group-row"
                      @click="toggleExpand(group.id)"
                      :class="{ expanded: expandedGroups.includes(group.id) }"
                    >
                      <td class="symbol-cell">
                        <div class="symbol-content">
                          <span class="expand-icon">
                            {{ expandedGroups.includes(group.id) ? "▼" : "▶" }}
                          </span>
                          <span class="symbol-name">{{ group.symbol }}</span>
                          <span class="strategy-name">{{
                            group.strategy
                          }}</span>
                        </div>
                      </td>
                      <td
                        class="pl-day-cell"
                        :class="getSymbolPlDay(group) >= 0 ? 'profit' : 'loss'"
                      >
                        {{ formatSymbolPlDay(group) }}
                      </td>
                      <td
                        class="pl-open-cell"
                        :class="getSymbolPlOpen(group) >= 0 ? 'profit' : 'loss'"
                      >
                        {{ formatSymbolPL(group) }}
                      </td>
                      <td class="bid-cell">--</td>
                      <td class="ask-cell">--</td>
                      <td class="trd-prc-cell">{{ formatSymbolTrdPrc(group) }}</td>
                      <td class="dsopn-cell">{{ formatDaysOpen(getSymbolDaysOpen(group)) }}</td>
                      <td class="beta-delta-cell">--</td>
                      <td class="dte-cell">
                        {{ formatDTE(getSymbolDTE(group)) }}
                      </td>
                      <td class="pop-cell">--</td>
                      <td class="indctr-cell">--</td>
                    </tr>

                    <!-- Expanded Strategy and Leg Rows -->
                    <template v-if="expandedGroups.includes(group.id)">
                      <!-- Show strategies if this is a symbol group -->
                      <template v-if="group.strategies">
                        <template
                          v-for="strategy in group.strategies"
                          :key="`${group.id}-strategy-${strategy.name}`"
                        >
                          <!-- Strategy Header Row -->
                          <tr class="position-strategy-row">
                            <td class="symbol-cell strategy-symbol">
                              <div class="strategy-details">
                                <span class="strategy-indent"> </span>
                                <span class="strategy-name-text">{{
                                  strategy.name
                                }}</span>
                              </div>
                            </td>
                            <td
                              class="pl-day-cell"
                              :class="getStrategyPlDay(strategy) >= 0 ? 'profit' : 'loss'"
                            >
                              {{ formatStrategyPlDay(strategy) }}
                            </td>
                            <td
                              class="pl-open-cell"
                              :class="getStrategyPlOpen(strategy) >= 0 ? 'profit' : 'loss'"
                            >
                              {{ formatStrategyPL(strategy) }}
                            </td>
                            <td class="bid-cell">--</td>
                            <td class="ask-cell">--</td>
                            <td class="trd-prc-cell">{{ formatStrategyTrdPrc(strategy) }}</td>
                            <td class="dsopn-cell">
                              {{ formatDaysOpen(getStrategyDaysOpen(strategy)) }}
                            </td>
                            <td class="beta-delta-cell">--</td>
                            <td class="dte-cell">
                              {{ formatDTE(getStrategyDTE(strategy)) }}
                            </td>
                            <td class="pop-cell">--</td>
                            <td class="indctr-cell">--</td>
                          </tr>

                          <!-- Individual Leg Rows for this Strategy -->
                          <tr
                            v-for="(leg, legIndex) in strategy.legs"
                            :key="`${group.id}-strategy-${strategy.name}-leg-${legIndex}`"
                            class="position-leg-row"
                          >
                            <td class="symbol-cell leg-symbol">
                              <div class="leg-description">
                                <span class="leg-qty">{{
                                  formatLegQty(leg.qty)
                                }}</span>
                                <div class="leg-details">
                                  <span class="expiry-date">{{
                                    formatLegDate(leg)
                                  }}</span>
                                  <span class="day-indicator">{{
                                    formatLegDays(leg)
                                  }}</span>
                                  <span class="strike-price">{{
                                    formatLegStrike(leg)
                                  }}</span>
                                  <span
                                    class="option-type"
                                    :class="getLegTypeClass(leg)"
                                  >
                                    {{ formatLegType(leg) }}
                                  </span>
                                  <span
                                    v-if="isLegITM(leg)"
                                    class="itm-indicator"
                                  >
                                    ITM
                                  </span>
                                </div>
                              </div>
                            </td>
                            <td
                              class="pl-day-cell"
                              :class="getLegPlDay(leg) >= 0 ? 'profit' : 'loss'"
                            >
                              {{ formatLegPlDay(leg) }}
                            </td>
                            <td
                              class="pl-open-cell"
                              :class="getLegPL(leg) >= 0 ? 'profit' : 'loss'"
                            >
                              {{ formatLegPL(leg) }}
                            </td>
                            <td class="bid-cell">{{ getLegBid(leg) }}</td>
                            <td class="ask-cell">{{ getLegAsk(leg) }}</td>
                            <td class="trd-prc-cell">
                              {{ formatCurrency(leg.avg_entry_price) }}
                            </td>
                            <td class="dsopn-cell">
                              {{ formatDaysOpen(getLegDaysOpen(leg)) }}
                            </td>
                            <td class="beta-delta-cell">--</td>
                            <td class="dte-cell">
                              {{ formatDTE(getStrategyDTE(strategy)) }}
                            </td>
                            <td class="pop-cell">--</td>
                            <td class="indctr-cell">--</td>
                          </tr>
                        </template>
                      </template>

                      <!-- Fallback: Show legs directly if no strategies (old format) -->
                      <template v-else>
                        <tr
                          v-for="(leg, index) in group.legs"
                          :key="`${group.id}-leg-${index}`"
                          class="position-leg-row"
                        >
                          <td class="symbol-cell leg-symbol">
                            <div class="leg-details">
                              <span class="leg-qty">{{
                                formatLegQty(leg.qty)
                              }}</span>
                              <span class="leg-info">{{
                                formatLegInfo(leg)
                              }}</span>
                            </div>
                          </td>
                          <td class="pl-day-cell">--</td>
                          <td
                            class="pl-open-cell"
                            :class="getLegPL(leg) >= 0 ? 'profit' : 'loss'"
                          >
                            {{ formatLegPL(leg) }}
                          </td>
                          <td class="bid-cell">{{ getLegBid(leg) }}</td>
                          <td class="ask-cell">{{ getLegAsk(leg) }}</td>
                          <td class="trd-prc-cell">
                            {{ formatCurrency(leg.avg_entry_price) }}
                          </td>
                          <td class="dsopn-cell">{{ group.dte || "--" }}</td>
                          <td class="beta-delta-cell">--</td>
                          <td class="dte-cell">
                            {{ group.dte ? group.dte + "d" : "--" }}
                          </td>
                          <td class="pop-cell">--</td>
                          <td class="indctr-cell">--</td>
                        </tr>
                      </template>
                    </template>
                  </template>
                </tbody>
              </table>
            </div>
          </div>
        </template>

        <!-- Empty State -->
        <div v-else class="empty-state">
          <p>No {{ selectedType }} positions found.</p>
        </div>
      </div>

      <!-- Right Panel -->
      <RightPanel
        :currentSymbol="currentSymbol"
        :currentPrice="currentPrice"
        :priceChange="priceChange"
        :isLivePrice="isLivePrice"
        :selectedOptions="selectedOptions"
        :chartData="chartData"
        :additionalQuoteData="additionalQuoteData"
        :ChainData="optionsManager.flattenedData.value"
        :adjustedNetCredit="null"
        :forceExpanded="isRightPanelExpanded"
        :forceSection="rightPanelSection"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onUnmounted, reactive } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import RightPanel from "../components/RightPanel.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useMarketData } from "../composables/useMarketData.js";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";

export default {
  name: "PositionsView",
  components: {
    TopBar,
    SideNav,
    RightPanel,
    SymbolHeader,
  },
  setup() {
    // Use global symbol state instead of local refs
    const { globalSymbolState } = useGlobalSymbol();

    // Use unified market data composable
    const { getPositions, refreshPositions } = useMarketData();

    // Smart Market Data integration - same pattern as other components
    const { getOptionPrice: getSmartOptionPrice } = useSmartMarketData();
    const liveOptionPrices = reactive(new Map());

    // Computed properties for global symbol state
    const currentSymbol = computed(() => globalSymbolState.currentSymbol);
    const companyName = computed(() => globalSymbolState.companyName);
    const exchange = computed(() => globalSymbolState.exchange);
    const currentPrice = computed(() => globalSymbolState.currentPrice);
    const priceChange = computed(() => globalSymbolState.priceChange);
    const priceChangePercent = computed(
      () => globalSymbolState.priceChangePercent
    );
    const isLivePrice = computed(() => globalSymbolState.isLivePrice);
    const marketStatus = computed(() => globalSymbolState.marketStatus);

    // Get reactive positions data (auto-updates every 30 seconds)
    const reactivePositions = getPositions();

    // Other reactive state
    const selectedOptions = ref([]);
    const chartData = ref(null);
    const additionalQuoteData = ref({});
    const optionsManager = ref({
      flattenedData: ref([]),
    });
    const loading = ref(false); // Start with false since data loads automatically
    const error = ref(null);
    const positionGroups = ref([]);
    const selectedType = ref("options");
    const groupBy = ref("order_chains");
    const expandedGroups = ref([]);
    const isRightPanelExpanded = ref(false);
    const rightPanelSection = ref(null);

    const onRightPanelCollapsed = () => {
      isRightPanelExpanded.value = false;
    };

    const onPositionsChanged = (positions) => {
      console.log("🔍 PositionsView: Received positions for chart:", positions);
      
      if (positions && positions.length > 0) {
        // Import the chart generation function
        import("../utils/chartUtils").then(({ generateMultiLegPayoff }) => {
          try {
            const chartResult = generateMultiLegPayoff(positions, currentPrice.value, null);
            console.log("🔍 PositionsView: Generated chart data:", chartResult);
            
            if (chartResult) {
              chartData.value = chartResult;
              
              // DON'T automatically expand the panel in PositionsView
              // Let the user manually expand it if they want to see the chart
              // isRightPanelExpanded.value = true;
              // rightPanelSection.value = "analysis";
            } else {
              console.warn("⚠️ PositionsView: Chart generation returned null");
              chartData.value = null;
            }
          } catch (error) {
            console.error("❌ PositionsView: Error generating chart:", error);
            chartData.value = null;
          }
        });
      } else {
        console.log("🔍 PositionsView: No positions provided, clearing chart");
        chartData.value = null;
      }
    };

    // Real-time update interval
    let updateInterval = null;

    // Computed properties
    const filteredPositionGroups = computed(() => {
      return positionGroups.value.filter((group) => {
        if (selectedType.value === "options") {
          return group.asset_class === "options";
        } else {
          return group.asset_class === "stocks";
        }
      });
    });

    const totalPlDay = computed(() => {
      return filteredPositionGroups.value.reduce(
        (sum, group) => sum + getSymbolPlDay(group),
        0
      );
    });

    const totalPlOpen = computed(() => {
      return filteredPositionGroups.value.reduce(
        (sum, group) => sum + getSymbolPlOpen(group),
        0
      );
    });

    // Live price function - same pattern as CollapsibleOptionsChain
    const getLivePrice = (symbol) => {
      if (!symbol) return null;

      if (!liveOptionPrices.has(symbol)) {
        // Call getSmartOptionPrice only once to set up the subscription and heartbeat
        liveOptionPrices.set(symbol, getSmartOptionPrice(symbol));
      }
      return liveOptionPrices.get(symbol)?.value;
    };


    // Get current market price (mid of bid/ask) for P/L calculation
    const getLegCurrentPrice = (leg) => {
      if (leg.asset_class !== "us_option") return leg.current_price || 0;
      
      const livePrice = getLivePrice(leg.symbol);

      // If we have live prices with both bid and ask, use them
      if (livePrice?.bid > 0 && livePrice?.ask > 0) {
        return (livePrice.bid + livePrice.ask) / 2;
      }

      // Fall back to original option data for mid price calculation
      if (leg.bid > 0 && leg.ask > 0) {
        return (leg.bid + leg.ask) / 2;
      }

      // If we have live price with only one side, use it
      if (livePrice?.bid > 0) return livePrice.bid;
      if (livePrice?.ask > 0) return livePrice.ask;

      // Fall back to original data with only one side
      if (leg.bid > 0) return leg.bid;
      if (leg.ask > 0) return leg.ask;

      // Final fallback to current_price, last_price, or entry price
      return leg.current_price || leg.last_price || leg.avg_entry_price || 0;
    };

    // Calculate live P/L for individual leg
    const getLegPL = (leg) => {
      const currentPrice = getLegCurrentPrice(leg);
      const entryPrice = leg.avg_entry_price || 0;

      // If we don't have entry price, we can't calculate P/L
      if (entryPrice === 0) return 0;

      // If current price is 0, use entry price (no P/L change)
      const priceToUse = currentPrice > 0 ? currentPrice : entryPrice;

      // For options: (current_price - entry_price) * quantity * 100 (contract multiplier)
      // For stocks: (current_price - entry_price) * quantity
      const multiplier = leg.asset_class === "us_option" ? 100 : 1;
      const priceDiff = priceToUse - entryPrice;

      // For short positions (negative quantity), P/L is inverted
      return priceDiff * leg.qty * multiplier;
    };

    // Calculate live daily P/L for individual leg (using lastday_price)
    const getLegPlDay = (leg) => {
      const lastdayPrice = leg.lastday_price;
      
      // Check if this is a same-day trade by comparing date_acquired with today
      const today = new Date().toISOString().split('T')[0];
      const dateAcquired = leg.date_acquired ? leg.date_acquired.split('T')[0] : null;
      
      // If lastday_price is null/undefined OR if position was acquired today, treat as same-day trade
      if (lastdayPrice === null || lastdayPrice === undefined || dateAcquired === today) {
        return getLegPL(leg);
      }

      const currentPrice = getLegCurrentPrice(leg);

      // If we don't have a valid current price, we can't calculate P/L Day.
      if (currentPrice === 0) {
        return 0;
      }

      // For options: (current_price - lastday_price) * quantity * 100 (contract multiplier)
      // For stocks: (current_price - lastday_price) * quantity
      const multiplier = leg.asset_class === "us_option" ? 100 : 1;
      const priceDiff = currentPrice - lastdayPrice;

      // For short positions (negative quantity), P/L is inverted
      return priceDiff * leg.qty * multiplier;
    };

    // Methods
    const fetchPositions = async () => {
      try {
        loading.value = true;
        error.value = null;

        // Use manual refresh for initial load and retry button
        await refreshPositions();

        // Get the current reactive data
        const response = reactivePositions.value;

        if (response && response.enhanced && response.symbol_groups) {
          // New hierarchical structure
          positionGroups.value = convertSymbolGroupsToDisplay(
            response.symbol_groups
          );
        } else if (response && response.enhanced && response.position_groups) {
          // Old enhanced structure
          positionGroups.value = response.position_groups;
        } else if (response && response.positions) {
          // Fallback: convert regular positions to simple groups
          positionGroups.value = convertPositionsToGroups(response.positions);
        } else {
          positionGroups.value = [];
        }
      } catch (err) {
        console.error("Error fetching positions:", err);
        error.value = "Failed to load positions. Please try again.";
      } finally {
        loading.value = false;
      }
    };

    const convertSymbolGroupsToDisplay = (symbolGroups) => {
      // Convert hierarchical symbol groups to flat display format for the table
      // NOTE: We no longer pre-calculate totals here - they will be computed reactively
      const displayGroups = [];

      symbolGroups.forEach((symbolGroup) => {
        // Create a symbol-level group with raw data (totals will be computed reactively)
        const symbolDisplayGroup = {
          id: `symbol_${symbolGroup.symbol}`,
          symbol: symbolGroup.symbol,
          strategy: `${symbolGroup.strategies?.length || 0} Strategies`,
          asset_class: symbolGroup.asset_class,
          total_qty: symbolGroup.total_qty || 0,
          total_cost_basis: symbolGroup.total_cost_basis || 0,
          total_market_value: symbolGroup.total_market_value || 0,
          total_unrealized_pl: 0, // Will be computed reactively
          total_unrealized_plpc: 0,
          pl_day: 0, // Will be computed reactively
          pl_open: 0, // Will be computed reactively
          legs: [], // Will be populated when expanded
          strategies: symbolGroup.strategies || [], // Store raw strategies
          order_date: null,
          expiration_date: null,
          dte: null,
          isSymbolGroup: true, // Flag to identify symbol-level groups
        };

        // Flatten all legs into the symbol group for display
        if (symbolGroup.strategies) {
          symbolGroup.strategies.forEach((strategy) => {
            if (strategy.legs) {
              strategy.legs.forEach((leg) => {
                symbolDisplayGroup.legs.push({
                  ...leg,
                  strategy_name: strategy.name,
                  strategy_pl_day: 0, // Will be calculated from live prices
                  strategy_pl_open: 0, // Will be calculated reactively
                });
              });
            }
          });
        }

        displayGroups.push(symbolDisplayGroup);
      });

      return displayGroups;
    };

    const convertPositionsToGroups = (positions) => {
      // Simple conversion for fallback - group by symbol
      const groups = {};

      positions.forEach((position) => {
        const key = position.symbol;
        if (!groups[key]) {
          groups[key] = {
            id: key,
            symbol: position.symbol,
            strategy:
              position.asset_class === "us_option"
                ? "Single Option"
                : "Stock Position",
            asset_class:
              position.asset_class === "us_option" ? "options" : "stocks",
            total_qty: 0,
            total_cost_basis: 0,
            total_market_value: 0,
            total_unrealized_pl: 0,
            total_unrealized_plpc: 0,
            pl_day: 0,
            pl_open: 0,
            legs: [],
            order_date: null,
            expiration_date: null,
            dte: null,
          };
        }

        groups[key].legs.push(position);
        groups[key].total_qty += position.qty;
        groups[key].total_cost_basis += position.cost_basis;
        groups[key].total_market_value += position.market_value;
        groups[key].total_unrealized_pl += position.unrealized_pl;
        groups[key].pl_open += position.unrealized_pl;
      });

      return Object.values(groups);
    };

    const toggleExpand = (groupId) => {
      const index = expandedGroups.value.indexOf(groupId);
      if (index > -1) {
        expandedGroups.value.splice(index, 1);
      } else {
        expandedGroups.value.push(groupId);
      }
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

    const formatNumber = (value) => {
      if (value === null || value === undefined) return "--";
      return new Intl.NumberFormat("en-US", {
        minimumFractionDigits: 0,
        maximumFractionDigits: 2,
      }).format(value);
    };

    const formatLegQty = (qty) => {
      return qty > 0 ? `+${qty}` : qty.toString();
    };

    const formatLegInfo = (leg) => {
      if (leg.asset_class === "us_option") {
        // Parse option symbol to get details
        const parts = parseOptionSymbol(leg.symbol);
        if (parts) {
          return `${parts.expiry} ${parts.strike} ${parts.type.toUpperCase()}`;
        }
      }
      return leg.symbol;
    };

    const parseOptionSymbol = (symbol) => {
      try {
        // Simple option symbol parsing - this should match the backend logic
        const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
        if (!match) return null;

        const [, underlying, dateStr, type, strikeStr] = match;

        // Parse date: YYMMDD -> MM/DD
        const month = dateStr.substring(2, 4);
        const day = dateStr.substring(4, 6);
        const expiry = `${month}/${day}`;

        // Parse strike: 8 digits with 3 decimal places
        const strike = parseInt(strikeStr) / 1000;

        return {
          underlying,
          expiry,
          type: type === "C" ? "call" : "put",
          strike,
        };
      } catch (error) {
        console.error("Error parsing option symbol:", symbol, error);
        return null;
      }
    };

    // New methods for leg card formatting
    const formatLegDate = (leg) => {
      if (leg.asset_class === "us_option") {
        const parts = parseOptionSymbol(leg.symbol);
        if (parts) {
          // Convert MM/DD format to "Aug 1" format
          const [month, day] = parts.expiry.split("/");
          const currentYear = new Date().getFullYear();
          const date = new Date(
            currentYear,
            parseInt(month) - 1,
            parseInt(day)
          );
          return date.toLocaleDateString("en-US", {
            month: "short",
            day: "numeric",
          });
        }
      }
      return "--";
    };

    const formatLegStrike = (leg) => {
      if (leg.asset_class === "us_option") {
        const parts = parseOptionSymbol(leg.symbol);
        return parts ? parts.strike.toString() : "--";
      }
      return "--";
    };

    const formatLegType = (leg) => {
      if (leg.asset_class === "us_option") {
        const parts = parseOptionSymbol(leg.symbol);
        return parts ? parts.type.charAt(0).toUpperCase() : "--";
      }
      return "--";
    };

    const formatLegDays = (leg) => {
      if (leg.asset_class === "us_option") {
        const parts = parseOptionSymbol(leg.symbol);
        if (parts) {
          // Parse MM/DD format to get proper date
          const [month, day] = parts.expiry.split("/");
          const currentYear = new Date().getFullYear();
          const expiryDate = new Date(
            currentYear,
            parseInt(month) - 1,
            parseInt(day)
          );
          const today = new Date();
          
          // Set both dates to midnight to get accurate day difference
          expiryDate.setHours(0, 0, 0, 0);
          today.setHours(0, 0, 0, 0);
          
          const diffTime = expiryDate.getTime() - today.getTime();
          const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
          return `${Math.max(0, diffDays)}d`;
        }
      }
      return "--";
    };

    const formatLegAction = (leg) => {
      return leg.qty > 0 ? "BTO" : "STO";
    };

    const getLegTypeClass = (leg) => {
      // Color based on buy/sell (positive qty = green, negative qty = red)
      return {
        "buy-type": leg.qty > 0,
        "sell-type": leg.qty < 0,
      };
    };

    const getLegActionClass = (leg) => {
      const action = formatLegAction(leg);
      return {
        "bto-action": action === "BTO",
        "sto-action": action === "STO",
      };
    };

    const isLegITM = (leg) => {
      if (leg.asset_class !== "us_option" || !currentPrice.value) return false;

      const parts = parseOptionSymbol(leg.symbol);
      if (!parts) return false;

      if (parts.type === "call") {
        return currentPrice.value > parts.strike;
      } else if (parts.type === "put") {
        return currentPrice.value < parts.strike;
      }
      return false;
    };

    // Get live bid price for a leg
    const getLegBid = (leg) => {
      if (leg.asset_class !== "us_option") return "$0.00";
      const livePrice = getLivePrice(leg.symbol);
      // Use nullish coalescing to fall back to original bid price
      const bidPrice = livePrice?.bid ?? leg.bid ?? 0;
      return formatCurrency(bidPrice);
    };

    // Get live ask price for a leg
    const getLegAsk = (leg) => {
      if (leg.asset_class !== "us_option") return "$0.00";
      const livePrice = getLivePrice(leg.symbol);
      // Use nullish coalescing to fall back to original ask price
      const askPrice = livePrice?.ask ?? leg.ask ?? 0;
      return formatCurrency(askPrice);
    };

    // Format P/L for display
    const formatLegPL = (leg) => {
      const pl = getLegPL(leg);
      return formatCurrency(pl);
    };

    // Reactive computed functions for strategy and symbol totals
    const getStrategyPlOpen = (strategy) => {
      if (!strategy.legs) return 0;
      return strategy.legs.reduce((sum, leg) => sum + getLegPL(leg), 0);
    };

    const getSymbolPlOpen = (group) => {
      if (!group.strategies) return 0;
      return group.strategies.reduce((sum, strategy) => sum + getStrategyPlOpen(strategy), 0);
    };

    const formatStrategyPL = (strategy) => {
      const pl = getStrategyPlOpen(strategy);
      return formatCurrency(pl);
    };

    const formatSymbolPL = (group) => {
      const pl = getSymbolPlOpen(group);
      return formatCurrency(pl);
    };

    // Reactive computed functions for daily P/L (strategy and symbol totals)
    const getStrategyPlDay = (strategy) => {
      if (!strategy.legs) return 0;
      return strategy.legs.reduce((sum, leg) => sum + getLegPlDay(leg), 0);
    };

    const getSymbolPlDay = (group) => {
      if (!group.strategies) return 0;
      return group.strategies.reduce((sum, strategy) => sum + getStrategyPlDay(strategy), 0);
    };

    const formatStrategyPlDay = (strategy) => {
      const pl = getStrategyPlDay(strategy);
      return formatCurrency(pl);
    };

    const formatSymbolPlDay = (group) => {
      const pl = getSymbolPlDay(group);
      return formatCurrency(pl);
    };

    // Format daily P/L for individual leg
    const formatLegPlDay = (leg) => {
      const pl = getLegPlDay(leg);
      return formatCurrency(pl);
    };

    // Calculate net trade price for strategy (total credit/debit, not weighted average)
    const getStrategyTrdPrc = (strategy) => {
      if (!strategy.legs || strategy.legs.length === 0) return 0;
      
      let totalValue = 0;
      
      strategy.legs.forEach((leg) => {
        const entryPrice = leg.avg_entry_price || 0;
        const quantity = leg.qty; // Use actual quantity (positive/negative) to determine credit/debit
        const legValue = entryPrice * quantity;
        
        totalValue += legValue;
      });
      
      // Return the net credit/debit for the strategy
      return totalValue;
    };

    const formatStrategyTrdPrc = (strategy) => {
      const trdPrc = getStrategyTrdPrc(strategy);
      return formatCurrency(trdPrc);
    };

    // Calculate net trade price for symbol (sum of all strategy credits/debits)
    const getSymbolTrdPrc = (group) => {
      if (!group.strategies || group.strategies.length === 0) return 0;
      
      let totalValue = 0;
      
      group.strategies.forEach((strategy) => {
        // Sum up the net credit/debit from each strategy
        totalValue += getStrategyTrdPrc(strategy);
      });
      
      return totalValue;
    };

    const formatSymbolTrdPrc = (group) => {
      const trdPrc = getSymbolTrdPrc(group);
      return formatCurrency(trdPrc);
    };

    // Calculate DTE for a strategy (use the minimum DTE from all legs)
    const getStrategyDTE = (strategy) => {
      if (!strategy.legs || strategy.legs.length === 0) return null;
      
      let minDTE = Infinity;
      
      strategy.legs.forEach((leg) => {
        if (leg.asset_class === "us_option") {
          const parts = parseOptionSymbol(leg.symbol);
          if (parts) {
            const [month, day] = parts.expiry.split("/");
            const currentYear = new Date().getFullYear();
            const expiryDate = new Date(currentYear, parseInt(month) - 1, parseInt(day));
            const today = new Date();
            
            // Set both dates to midnight to get accurate day difference
            expiryDate.setHours(0, 0, 0, 0);
            today.setHours(0, 0, 0, 0);
            
            const diffTime = expiryDate.getTime() - today.getTime();
            const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
            const dte = Math.max(0, diffDays);
            
            if (dte < minDTE) {
              minDTE = dte;
            }
          }
        }
      });
      
      return minDTE === Infinity ? null : minDTE;
    };

    // Calculate DTE for a symbol group (use the minimum DTE from all strategies)
    const getSymbolDTE = (group) => {
      if (!group.strategies || group.strategies.length === 0) return null;
      
      let minDTE = Infinity;
      
      group.strategies.forEach((strategy) => {
        const strategyDTE = getStrategyDTE(strategy);
        if (strategyDTE !== null && strategyDTE < minDTE) {
          minDTE = strategyDTE;
        }
      });
      
      return minDTE === Infinity ? null : minDTE;
    };

    // Format DTE for display
    const formatDTE = (dte) => {
      return dte !== null ? `${dte}d` : "--";
    };

    // Calculate Days Open for a strategy (use the maximum days open from all legs)
    const getStrategyDaysOpen = (strategy) => {
      if (!strategy.legs || strategy.legs.length === 0) return null;
      
      let maxDaysOpen = 0;
      
      strategy.legs.forEach((leg) => {
        const legDaysOpen = getLegDaysOpen(leg);
        if (legDaysOpen !== null && legDaysOpen > maxDaysOpen) {
          maxDaysOpen = legDaysOpen;
        }
      });
      
      return maxDaysOpen;
    };

    // Calculate Days Open for a symbol group (use the maximum days open from all strategies)
    const getSymbolDaysOpen = (group) => {
      if (!group.strategies || group.strategies.length === 0) return null;
      
      let maxDaysOpen = 0;
      
      group.strategies.forEach((strategy) => {
        const strategyDaysOpen = getStrategyDaysOpen(strategy);
        if (strategyDaysOpen !== null && strategyDaysOpen > maxDaysOpen) {
          maxDaysOpen = strategyDaysOpen;
        }
      });
      
      return maxDaysOpen;
    };

    // Calculate Days Open for individual leg
    const getLegDaysOpen = (leg) => {
      if (!leg.date_acquired) return null;
      
      try {
        // Parse the date_acquired (usually in ISO format like "2024-07-28T14:30:00Z")
        const acquiredDate = new Date(leg.date_acquired);
        const today = new Date();
        
        // Set both dates to midnight to get accurate day difference
        acquiredDate.setHours(0, 0, 0, 0);
        today.setHours(0, 0, 0, 0);
        
        const diffTime = today.getTime() - acquiredDate.getTime();
        const diffDays = Math.floor(diffTime / (1000 * 60 * 60 * 24));
        
        return Math.max(0, diffDays); // Ensure no negative values
      } catch (error) {
        console.error("Error calculating days open for leg:", leg.symbol, error);
        return null;
      }
    };

    // Format Days Open for display
    const formatDaysOpen = (daysOpen) => {
      return daysOpen !== null ? `${daysOpen}d` : "--";
    };

    const onTradeModeChanged = (mode) => {
      // Not used in positions view
    };

    // Subscribe to all position symbols for live price updates - DEFINED BEFORE WATCHER
    const subscribeToAllPositionSymbols = (positionsData) => {
      const symbols = new Set();

      try {
        // Extract data from the response structure
        const data = positionsData?.data || positionsData;

        if (data && data.enhanced && data.symbol_groups) {
          // New hierarchical structure - extract all leg symbols
          data.symbol_groups.forEach((symbolGroup) => {
            if (symbolGroup.strategies) {
              symbolGroup.strategies.forEach((strategy) => {
                if (strategy.legs) {
                  strategy.legs.forEach((leg) => {
                    if (leg.asset_class === "us_option" && leg.symbol) {
                      symbols.add(leg.symbol);
                    }
                  });
                }
              });
            }
          });
        } else if (data && data.enhanced && data.position_groups) {
          // Old enhanced structure - extract all leg symbols
          data.position_groups.forEach((group) => {
            if (group.legs) {
              group.legs.forEach((leg) => {
                if (leg.asset_class === "us_option" && leg.symbol) {
                  symbols.add(leg.symbol);
                }
              });
            }
          });
        } else if (data && data.positions) {
          // Fallback structure - extract all position symbols
          data.positions.forEach((position) => {
            if (position.asset_class === "us_option" && position.symbol) {
              symbols.add(position.symbol);
            }
          });
        }

        // Subscribe to all option symbols for live price updates
        if (symbols.size > 0) {
          symbols.forEach((symbol) => {
            // This will trigger subscription in SmartMarketDataStore
            getLivePrice(symbol);
          });
        }
      } catch (error) {
        console.error("❌ Error subscribing to position symbols:", error);
      }
    };

    // Helper function to process positions data - defined before watcher to avoid hoisting issues
    const processPositionsData = (response) => {
      try {
        // The API returns data nested under response.data
        const data = response?.data || response;

        if (data && data.enhanced && data.symbol_groups) {
          // New hierarchical structure
          positionGroups.value = convertSymbolGroupsToDisplay(
            data.symbol_groups
          );
        } else if (data && data.enhanced && data.position_groups) {
          // Old enhanced structure
          positionGroups.value = data.position_groups;
        } else if (data && data.positions) {
          // Fallback: convert regular positions to simple groups
          positionGroups.value = convertPositionsToGroups(data.positions);
        } else {
          positionGroups.value = [];
        }

        // Clear any previous errors since we got data
        error.value = null;
      } catch (err) {
        console.error("❌ Error processing positions data:", err);
        error.value = "Failed to process positions data.";
      }
    };

    // Watch for changes in reactive positions data - now after function is defined
    watch(
      reactivePositions,
      (newPositions) => {
        if (newPositions) {
          // Process the new data automatically
          processPositionsData(newPositions);

          // Subscribe to all position symbols for live price updates
          subscribeToAllPositionSymbols(newPositions);
        }
      },
      { immediate: true }
    );

    // Lifecycle
    onMounted(async () => {

    });

    onUnmounted(() => {
      // Cleanup if needed
    });

    return {
      // State
      loading,
      error,
      positionGroups,
      selectedType,
      groupBy,
      expandedGroups,

      // Computed
      filteredPositionGroups,
      totalPlDay,
      totalPlOpen,

      // Methods
      fetchPositions,
      toggleExpand,
      formatCurrency,
      formatNumber,
      formatLegQty,
      formatLegInfo,
      formatLegDate,
      formatLegStrike,
      formatLegType,
      formatLegDays,
      formatLegAction,
      getLegTypeClass,
      getLegActionClass,
      isLegITM,
      getLegBid,
      getLegAsk,
      getLegPL,
      formatLegPL,
      getStrategyPlOpen,
      getSymbolPlOpen,
      formatStrategyPL,
      formatSymbolPL,
      // Daily P/L functions
      getLegPlDay,
      getStrategyPlDay,
      getSymbolPlDay,
      formatLegPlDay,
      formatStrategyPlDay,
      formatSymbolPlDay,
      // Trade price functions
      getStrategyTrdPrc,
      formatStrategyTrdPrc,
      getSymbolTrdPrc,
      formatSymbolTrdPrc,
      // DTE functions
      getStrategyDTE,
      getSymbolDTE,
      formatDTE,
      // Days Open functions
      getStrategyDaysOpen,
      getSymbolDaysOpen,
      getLegDaysOpen,
      formatDaysOpen,
      currentSymbol,
      currentPrice,
      priceChange,
      isLivePrice,
      selectedOptions,
      chartData,
      additionalQuoteData,
      optionsManager,
      isRightPanelExpanded,
      rightPanelSection,
      onRightPanelCollapsed,
      onPositionsChanged,
      companyName,
      exchange,
      priceChangePercent,
      marketStatus,
      onTradeModeChanged,
    };
  },
};
</script>

<style scoped>
.positions-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: #141519;
  color: #ffffff;
}

.main-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
  position: relative;
}

.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: #141519;
  transition: margin-right 0.3s ease;
  min-height: 0;
}

.content-area > :not(.symbol-header) {
  padding-left: var(--spacing-lg);
  padding-right: var(--spacing-lg);
}

.content-area > .positions-header,
.content-area > .totals-section,
.content-area > .positions-table-container,
.content-area > .loading-state,
.content-area > .error-state,
.content-area > .empty-state {
  margin-top: var(--spacing-lg);
}

.content-area > .positions-header {
  margin-top: 0;
}

.positions-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
  padding: var(--spacing-sm) 0;
  border-bottom: 1px solid var(--border-primary);
}

.header-left {
  display: flex;
  align-items: center;
}

.header-right {
  display: flex;
  align-items: center;
}

.position-type-tabs {
  display: flex;
  gap: var(--spacing-xs);
}

.tab-button {
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition-normal);
  font-weight: var(--font-weight-medium);
}

.tab-button:hover {
  background: var(--bg-primary);
  color: var(--text-primary);
}

.tab-button.active {
  background: var(--color-brand);
  color: white;
  border-color: var(--color-brand);
}

.group-by-section {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.group-by-section label {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.group-by-select {
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-tertiary);
  color: var(--text-primary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.loading-state,
.error-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--text-secondary);
}

.loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-secondary);
  border-top: 3px solid var(--color-brand);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: var(--spacing-md);
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

.retry-button {
  margin-top: var(--spacing-md);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--color-brand);
  color: white;
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition-normal);
}

.retry-button:hover {
  background: var(--color-brand-dark);
}

.totals-section {
  margin-bottom: var(--spacing-md);
}

.totals-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md);
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-primary);
  font-weight: var(--font-weight-semibold);
}

.totals-label {
  color: var(--text-primary);
  font-size: var(--font-size-lg);
}

.totals-values {
  display: flex;
  gap: var(--spacing-lg);
}

.positions-table-container {
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-primary);
  overflow: hidden;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.positions-table-scroll {
  overflow-y: auto;
  flex: 1;
  max-height: calc(100vh - 300px);
}

.positions-table {
  width: 100%;
  border-collapse: collapse;
}

.positions-table th {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  padding: var(--spacing-sm) var(--spacing-md);
  text-align: left;
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-sm);
  border-bottom: 1px solid var(--border-secondary);
}

.positions-table td {
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--border-secondary);
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-normal);
}

.positions-table td:not(.symbol-cell) {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}

.pl-day-cell,
.pl-open-cell {
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-md);
}

.pl-day-cell.profit,
.pl-open-cell.profit {
  color: #00c851 !important;
}

.pl-day-cell.loss,
.pl-open-cell.loss {
  color: #ff4444 !important;
}

.total-pl-day.profit,
.total-pl-open.profit {
  color: #00c851 !important;
}

.total-pl-day.loss,
.total-pl-open.loss {
  color: #ff4444 !important;
}

.etf-delta-cell,
.bid-cell,
.ask-cell,
.trd-prc-cell,
.dsopn-cell,
.beta-delta-cell,
.dte-cell,
.pop-cell,
.indctr-cell {
  font-size: var(--font-size-md);
  color: var(--text-tertiary);
}

.symbol-cell {
  width: 300px;
  max-width: 300px;
}

.position-group-row {
  cursor: pointer;
  transition: var(--transition-normal);
}

.position-group-row:hover {
  background: var(--bg-tertiary);
}

.position-group-row.expanded {
  background: var(--bg-tertiary);
}

.position-strategy-row {
  background: var(--bg-secondary);
  font-weight: var(--font-weight-medium);
}

.position-strategy-row:hover {
  background: var(--bg-tertiary);
}

.position-leg-row {
  background: var(--bg-primary);
}

.symbol-content {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.expand-icon {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  width: 12px;
}

.symbol-name {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.strategy-name {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  margin-left: var(--spacing-xs);
}

.strategy-details {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.strategy-indent {
  width: 20px;
  display: inline-block;
}

.strategy-name-text {
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  font-style: italic;
}

.leg-details {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-left: var(--spacing-lg);
}

.leg-indent {
  width: 40px;
  display: inline-block;
}

.leg-qty {
  font-weight: var(--font-weight-semibold);
  min-width: 30px;
}

.leg-info {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.profit {
  color: var(--color-success);
}

.loss {
  color: var(--color-danger);
}

/* Leg Card Styling - matching BottomTradingPanel */
.leg-card {
  display: grid;
  grid-template-columns: 30px 50px 30px 50px 20px;
  gap: var(--spacing-xs);
  align-items: center;
  padding: 4px 8px;
  background-color: #2a2a2a;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  margin: 0 var(--spacing-md);
  border: 1px solid transparent;
  width: fit-content;
}

.leg-card .leg-qty {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  text-align: center;
}

.leg-card .leg-date {
  color: var(--text-secondary);
  font-size: var(--font-size-xs);
  text-align: center;
}

.leg-card .leg-days {
  color: var(--text-secondary);
  font-size: var(--font-size-xs);
  text-align: center;
}

.leg-card .leg-strike {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
  text-align: center;
}

.leg-card .leg-type {
  font-weight: var(--font-weight-semibold);
  text-align: center;
  font-size: var(--font-size-xs);
}

.leg-card .leg-type.buy-type {
  color: var(--color-success);
}

.leg-card .leg-type.sell-type {
  color: var(--color-danger);
}

/* New Leg Description Styling - exactly matching RightPanel Analysis */
.leg-description {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: var(--spacing-lg);
}

.leg-description .leg-qty {
  font-weight: 600;
  min-width: 30px;
  text-align: center;
  color: var(--text-primary, #ffffff);
}

.leg-description .leg-details {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.leg-description .expiry-date {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  background-color: var(--bg-tertiary, #1a1d23);
  padding: 2px 6px;
  border-radius: 3px;
}

.leg-description .day-indicator {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  background-color: var(--bg-secondary, #141519);
  padding: 1px 4px;
  border-radius: 2px;
}

.leg-description .strike-price {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  font-weight: 600;
}

.leg-description .option-type {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 4px;
  border-radius: 2px;
  min-width: 16px;
  text-align: center;
}

.leg-description .option-type.call-type {
  background-color: #28a745;
  color: white;
}

.leg-description .option-type.put-type {
  background-color: #dc3545;
  color: white;
}

.leg-description .itm-indicator {
  font-size: 9px;
  font-weight: 600;
  background-color: #ffc107;
  color: #000;
  padding: 1px 4px;
  border-radius: 2px;
  text-transform: uppercase;
}

/* Update option type classes to work with new structure */
.option-type.buy-type {
  background-color: #28a745;
  color: white;
}

.option-type.sell-type {
  background-color: #dc3545;
  color: white;
}

/* Responsive design */
@media (max-width: 1200px) {
  .positions-table th:nth-child(n + 8),
  .positions-table td:nth-child(n + 8) {
    display: none;
  }
}

@media (max-width: 768px) {
  .positions-header {
    flex-direction: column;
    gap: var(--spacing-md);
    align-items: flex-start;
  }

  .header-left,
  .header-right {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
