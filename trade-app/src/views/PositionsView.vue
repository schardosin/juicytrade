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
                :class="['tab-button', { active: showOptions }]"
                @click="toggleFilter('options')"
              >
                Options
              </button>
              <button
                :class="['tab-button', { active: showStocks }]"
                @click="toggleFilter('stocks')"
              >
                Stocks
              </button>
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
                            :class="{ selected: isLegSelected(leg.symbol) }"
                            @click="togglePositionLegSelection(leg, strategy, group)"
                          >
                            <td class="symbol-cell leg-symbol">
                              <div class="leg-description">
                                <span class="leg-qty">{{
                                  formatLegQty(leg.qty)
                                }}</span>
                                <div class="leg-details">
                                  <!-- Show "Shares" for equity positions, options details for options -->
                                  <template v-if="leg.asset_class === 'us_equity'">
                                    <span class="shares-label">Shares</span>
                                  </template>
                                  <template v-else>
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
                                  </template>
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
          <p>No positions found.</p>
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
        :adjustedNetCredit="adjustedNetCredit"
        :forceExpanded="isRightPanelExpanded"
        :forceSection="rightPanelSection"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>

    <!-- Order Confirmation Dialog -->
    <OrderConfirmationDialog
      :visible="showOrderConfirmation"
      :orderData="orderData"
      :loading="isPlacingOrder"
      @hide="handleOrderCancellation"
      @confirm="handleOrderConfirmation"
      @cancel="handleOrderCancellation"
      @edit="handleOrderEdit"
      @clear-selections="clearAllSelections"
    />

    <!-- Bottom Trading Panel -->
    <BottomTradingPanel
      :visible="hasSelectedLegs"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      @review-send="onReviewSend"
      @price-adjusted="onPriceAdjusted"
    />
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onUnmounted, reactive, nextTick } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import RightPanel from "../components/RightPanel.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import BottomTradingPanel from "../components/BottomTradingPanel.vue";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useMarketData } from "../composables/useMarketData.js";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import { useSelectedLegs } from "../composables/useSelectedLegs.js";
import { useOrderManagement } from "../composables/useOrderManagement";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";
import { mapToRootSymbol } from "../utils/symbolMapping.js";

export default {
  name: "PositionsView",
  components: {
    TopBar,
    SideNav,
    RightPanel,
    SymbolHeader,
    BottomTradingPanel,
    OrderConfirmationDialog,
  },
  setup() {
    // Use global symbol state with centralized symbol selection
    const { globalSymbolState, setupSymbolSelectionListener, updateSymbol } = useGlobalSymbol({
      onSymbolChange: async (symbolData) => {
        // Positions are already refreshed by updateSymbol in useGlobalSymbol
        // Any additional positions-specific logic can go here if needed
      }
    });

    // Use unified market data composable
    const { getPositions, refreshPositions } = useMarketData();

    // Component registration system
    const componentId = `PositionsView-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // Smart Market Data integration - same pattern as other components
    const { getStockPrice: getSmartStockPrice, getOptionPrice: getSmartOptionPrice } = useSmartMarketData();
    const livePrices = reactive(new Map());

    // Single registration method per component to prevent double registration
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    // Selected legs integration - same pattern as OptionsTrading
    const { selectedLegs, hasSelectedLegs, addLeg, removeLeg, clearAll } = useSelectedLegs();

    // Use centralized order management with cleanup callback
    const {
      showOrderConfirmation,
      showOrderResult,
      orderData,
      orderResult,
      isPlacingOrder,
      initializeOrder,
      handleOrderConfirmation,
      handleOrderCancellation,
      handleOrderResultClose,
    } = useOrderManagement({
      onOrderSuccess: () => {
        // Clear all selections when order is successful
        clearAllSelections();
      }
    });

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
    // Filter state - both can be active at the same time
    const showOptions = ref(true);
    const showStocks = ref(true);
    const expandedGroups = ref([]);
    const isRightPanelExpanded = ref(false);
    const adjustedNetCredit = ref(null); // Add adjustedNetCredit for limit price functionality
    const isSymbolTransitioning = ref(false); // Flag to track symbol change in progress

    // Computed property to determine which section to show in right panel
    const rightPanelSection = computed(() => {
      // If legs are selected, show analysis section, otherwise null (let user choose)
      return selectedLegs.value.length > 0 ? "analysis" : null;
    });

    const onRightPanelCollapsed = () => {
      isRightPanelExpanded.value = false;
    };

    const onPositionsChanged = (positions) => {
      if (positions && positions.length > 0) {
        // 🔑 KEY FIX: Store positions for chart regeneration when adjustedNetCredit changes
        lastChartPositions.value = positions;
        
        // Import the chart generation function
        import("../utils/chartUtils").then(({ generateMultiLegPayoff }) => {
          try {
            // 🔑 KEY FIX: Pass adjustedNetCredit.value instead of null
            const chartResult = generateMultiLegPayoff(positions, currentPrice.value, adjustedNetCredit.value);
            
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
        chartData.value = null;
        lastChartPositions.value = null; // Clear stored positions
      }
    };

    // Real-time update interval
    let updateInterval = null;

    // Toggle filter method
    const toggleFilter = (type) => {
      if (type === 'options') {
        showOptions.value = !showOptions.value;
      } else if (type === 'stocks') {
        showStocks.value = !showStocks.value;
      }
    };

    // Computed properties
    const filteredPositionGroups = computed(() => {
      return positionGroups.value.filter((group) => {
        // Handle both old format ("options"/"stocks") and new format ("us_option"/"us_equity")
        if ((group.asset_class === "options" || group.asset_class === "us_option") && showOptions.value) {
          return true;
        }
        if ((group.asset_class === "stocks" || group.asset_class === "us_equity") && showStocks.value) {
          return true;
        }
        return false;
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

    // Live price function - handles both stocks and options
    const getLivePrice = (symbol) => {
      if (!symbol) return null;

      if (!livePrices.has(symbol)) {
        // Ensure symbol is registered (only once per component)
        ensureSymbolRegistration(symbol);
        
        // Use appropriate smart market data method based on symbol type
        if (smartMarketDataStore.isOptionSymbol(symbol)) {
          livePrices.set(symbol, getSmartOptionPrice(symbol));
        } else {
          livePrices.set(symbol, getSmartStockPrice(symbol));
        }
      }
      return livePrices.get(symbol)?.value;
    };


    // Get current market price (mid of bid/ask) for P/L calculation
    const getLegCurrentPrice = (leg) => {
      const livePrice = getLivePrice(leg.symbol);

      if (leg.asset_class === "us_option") {
        // Option logic - use bid/ask mid-price
        const bid = livePrice?.bid ?? leg.bid;
        const ask = livePrice?.ask ?? leg.ask;

        // If both bid and ask are present and positive, use mid-price
        if (bid > 0 && ask > 0) {
          return (bid + ask) / 2;
        }

        // If only one side is positive, use that side
        if (bid > 0) return bid;
        if (ask > 0) return ask;
        
        // If both are explicitly zero, the option is worthless
        if (bid === 0 && ask === 0) {
          return 0;
        }

        // Fallback to other price fields, but NOT entry price
        return leg.current_price || leg.last_price || 0;
      } else {
        // Equity logic - use live stock price
        if (livePrice?.price) {
          return livePrice.price;
        }
        
        // Fallback to static data for equity positions
        return leg.current_price || leg.last_price || leg.avg_entry_price || 0;
      }
    };

    // Calculate live P/L for individual leg
    const getLegPL = (leg) => {
      const currentPrice = getLegCurrentPrice(leg);
      const entryPrice = leg.avg_entry_price || 0;

      // If we don't have entry price, we can't calculate P/L
      if (entryPrice === 0 && leg.asset_class === 'us_option') return 0;

      // For options: (current_price - entry_price) * quantity * 100 (contract multiplier)
      // For stocks: (current_price - entry_price) * quantity
      const multiplier = leg.asset_class === "us_option" ? 100 : 1;
      
      // For short positions (negative quantity), P/L is inverted
      // The formula is: (current - entry) * qty
      // Example short: (0 - 0.50) * -1 = 0.50 (profit)
      // Example long: (1.0 - 0.50) * 1 = 0.50 (profit)
      const priceDiff = currentPrice - entryPrice;

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

        // Parse date: YYMMDD -> full year extraction
        const year = parseInt(dateStr.substring(0, 2));
        const month = dateStr.substring(2, 4);
        const day = dateStr.substring(4, 6);
        
        // Convert 2-digit year to 4-digit year
        // Years 00-30 are assumed to be 2000-2030, 31-99 are 1931-1999
        const fullYear = year <= 30 ? 2000 + year : 1900 + year;
        
        const expiry = `${month}/${day}`;
        const expiryWithYear = `${month}/${day}/${fullYear}`;

        // Parse strike: 8 digits with 3 decimal places
        const strike = parseInt(strikeStr) / 1000;

        return {
          underlying,
          expiry, // Keep for backward compatibility (MM/DD format)
          expiryWithYear, // New field with full date (MM/DD/YYYY format)
          year: fullYear,
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
          // Convert MM/DD format to "Aug 1" format using actual year
          const [month, day] = parts.expiry.split("/");
          const date = new Date(
            parts.year, // Use the actual year from the option symbol
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
          // Use the actual year from the parsed option symbol
          const [month, day] = parts.expiry.split("/");
          const expiryDate = new Date(
            parts.year, // Use the actual year from the option symbol
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

    const getLegTypeClass = (leg) => {
      // Color based on buy/sell (positive qty = green, negative qty = red)
      return {
        "buy-type": leg.qty > 0,
        "sell-type": leg.qty < 0,
      };
    };

    const getLegActionClass = (leg) => {
      return {
        "bto-action": leg.qty > 0,
        "sto-action": leg.qty < 0,
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
      const livePrice = getLivePrice(leg.symbol);
      
      if (leg.asset_class === "us_option") {
        // Option logic - use live bid or fallback to static bid
        const bidPrice = livePrice?.bid ?? leg.bid ?? 0;
        return formatCurrency(bidPrice);
      } else {
        // Equity logic - for stocks, bid/ask is usually very close to current price
        // Use live price or fallback to static data
        const bidPrice = livePrice?.bid ?? livePrice?.price ?? leg.current_price ?? leg.last_price ?? 0;
        return formatCurrency(bidPrice);
      }
    };

    // Get live ask price for a leg
    const getLegAsk = (leg) => {
      const livePrice = getLivePrice(leg.symbol);
      
      if (leg.asset_class === "us_option") {
        // Option logic - use live ask or fallback to static ask
        const askPrice = livePrice?.ask ?? leg.ask ?? 0;
        return formatCurrency(askPrice);
      } else {
        // Equity logic - for stocks, bid/ask is usually very close to current price
        // Use live price or fallback to static data
        const askPrice = livePrice?.ask ?? livePrice?.price ?? leg.current_price ?? leg.last_price ?? 0;
        return formatCurrency(askPrice);
      }
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

    // Calculate net trade price for strategy (total credit/debit for symbol totals)
    const getStrategyTrdPrc = (strategy) => {
      if (!strategy.legs || strategy.legs.length === 0) return 0;
      
      let totalValue = 0;
      
      strategy.legs.forEach((leg) => {
        const entryPrice = leg.avg_entry_price || 0;
        const quantity = leg.qty; // Keep sign for credit/debit calculation
        const legValue = entryPrice * quantity;
        
        totalValue += legValue;
      });
      
      // Return the net credit/debit for the strategy (used for symbol totals)
      return totalValue;
    };

    // Calculate average trade price per leg for display (new function)
    const getStrategyDisplayTrdPrc = (strategy) => {
      if (!strategy.legs || strategy.legs.length === 0) return 0;
      
      let totalValue = 0;
      let legCount = 0;
      
      strategy.legs.forEach((leg) => {
        const entryPrice = leg.avg_entry_price || 0;
        const quantity = leg.qty; // Keep sign for credit/debit
        const legValue = entryPrice * quantity; // Total value with sign
        
        totalValue += legValue;
        legCount += 1; // Count legs, not contracts
      });
      
      // Return the net credit/debit for the strategy (total, not average)
      return totalValue;
    };

    const formatStrategyTrdPrc = (strategy) => {
      const trdPrc = getStrategyDisplayTrdPrc(strategy); // Use new display function
      return formatCurrency(trdPrc);
    };

    // Calculate average trade price for symbol (average of all strategy display prices)
    const getSymbolTrdPrc = (group) => {
      if (!group.strategies || group.strategies.length === 0) return 0;
      
      let totalValue = 0;
      let strategyCount = 0;
      
      group.strategies.forEach((strategy) => {
        // Sum up the display trade price from each strategy
        totalValue += getStrategyDisplayTrdPrc(strategy);
        strategyCount += 1;
      });
      
      // Return the average trade price across all strategies
      return strategyCount > 0 ? totalValue / strategyCount : 0;
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
            // Use the actual year from the parsed option symbol
            const expiryDate = new Date(parts.year, parseInt(month) - 1, parseInt(day));
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

    // Bottom Trading Panel event handlers
    const onReviewSend = (orderData) => {
      // Initialize the order confirmation dialog
      initializeOrder(orderData);
    };

    // Additional order management functions
    const clearAllSelections = () => {
      clearAll();
      chartData.value = null;
      isRightPanelExpanded.value = false;
    };

    const handleOrderEdit = () => {
      // Hide confirmation dialog and show bottom trading panel
      // The bottom trading panel visibility is controlled by hasSelectedLegs
      // So we just need to hide the confirmation dialog
      handleOrderCancellation();
    };

    const onPriceAdjusted = (priceData) => {
      // Update the adjustedNetCredit to affect chart calculation
      adjustedNetCredit.value = priceData.adjustedNetCredit;
    };

    // Position leg selection functions
    const isLegSelected = (legSymbol) => {
      return selectedLegs.value.some(leg => leg.symbol === legSymbol);
    };

    const togglePositionLegSelection = async (leg, strategy, group) => {
      const legSymbol = leg.symbol;
      const underlyingSymbol = group.symbol;

      // Map weekly symbols to their root symbols (e.g., SPXW -> SPX, NDXP -> NDX)
      const rootSymbol = mapToRootSymbol(underlyingSymbol);
      const needsSymbolChange = rootSymbol && rootSymbol !== currentSymbol.value;

      console.log(`🎯 PositionsView: Leg selection for ${legSymbol}, underlying: ${underlyingSymbol}, needsSymbolChange: ${needsSymbolChange}`);

      // 🔑 KEY FIX: Change symbol FIRST if needed, then handle leg selection
      if (needsSymbolChange) {
        try {
          console.log(`🔄 PositionsView: Changing symbol from ${currentSymbol.value} to ${rootSymbol}`);
          
          // Set flag to prevent chart updates during transition
          isSymbolTransitioning.value = true;
          
          // Temporarily disable chart updates during symbol transition
          chartData.value = null;
          
          // Change symbol synchronously and wait for positions to load
          const symbolData = {
            symbol: rootSymbol,
            description: "", // Will be fetched by SymbolHeader
            exchange: "", // Will be fetched by SymbolHeader
          };
          
          // Use the global symbol update function directly
          await updateSymbol(symbolData);
          
          // Wait for all reactive updates to complete
          await nextTick();
          
          // Wait for underlying price to be available (critical for chart generation)
          let retryCount = 0;
          const maxRetries = 20; // Max 2 seconds
          while ((!currentPrice.value || currentPrice.value === 0) && retryCount < maxRetries) {
            await new Promise(resolve => setTimeout(resolve, 100));
            retryCount++;
          }
          
          if (retryCount >= maxRetries) {
            console.warn(`⚠️ PositionsView: Timeout waiting for price data for ${rootSymbol}`);
          } else {
            console.log(`✅ PositionsView: Symbol change completed, current symbol: ${currentSymbol.value}, price: ${currentPrice.value}`);
          }
          
          // Clear the transition flag
          isSymbolTransitioning.value = false;
          
        } catch (error) {
          console.error('❌ Error changing symbol before leg selection:', error);
          // Clear the transition flag even on error
          isSymbolTransitioning.value = false;
        }
      }

      // Now handle leg selection after symbol change is complete
      if (isLegSelected(legSymbol)) {
        // Remove from selection
        removeLeg(legSymbol);
      } else {
        // Parse option symbol to get missing details
        const parsedOption = parseOptionSymbol(leg.symbol);
        
        // Format expiry date for centralized store (YYYY-MM-DD format)
        let formattedExpiry = null;
        if (parsedOption && parsedOption.expiry) {
          // parsedOption.expiry is in MM/DD format, convert to YYYY-MM-DD using actual year
          const [month, day] = parsedOption.expiry.split("/");
          formattedExpiry = `${parsedOption.year}-${month.padStart(2, '0')}-${day.padStart(2, '0')}`;
        }
        
        // Get current market price for the closing trade
        const currentMarketPrice = getLegCurrentPrice(leg);
        
        // Get live bid/ask prices for proper chart calculation
        const livePrice = getLivePrice(leg.symbol);
        const bidPrice = livePrice?.bid || leg.bid || currentMarketPrice;
        const askPrice = livePrice?.ask || leg.ask || currentMarketPrice;
        
        // Add to selection with opposite side for closing
        const closingLeg = {
          symbol: leg.symbol,
          side: leg.qty > 0 ? 'sell' : 'buy', // Opposite side for closing
          quantity: Math.abs(leg.qty), // Use position quantity
          strike_price: parsedOption?.strike || 0,
          type: parsedOption?.type || 'call',
          expiry: formattedExpiry,
          current_price: currentMarketPrice,
          // 🔑 KEY FIX: Provide proper bid/ask for RightPanel chart calculation
          bid: bidPrice,
          ask: askPrice,
          // For closing trades, the "entry price" should be the price we'd get for the closing trade
          // For sell orders (closing long positions), use bid price
          // For buy orders (closing short positions), use ask price  
          avg_entry_price: leg.qty > 0 ? bidPrice : askPrice,
          // Position-specific data (for reference only)
          original_quantity: Math.abs(leg.qty),
          original_avg_entry_price: leg.avg_entry_price,
          unrealized_pl: leg.unrealized_pl
        };
        
        addLeg(closingLeg, 'positions');
        
        // � KEY FIX: Immediately update chart with the new selection
        await nextTick();
        updateChartWithSelectedLegs();
        
        // Force right panel to expand to analysis tab after a short delay
        setTimeout(() => {
          isRightPanelExpanded.value = true;
        }, 50);
      }
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
                    // Subscribe to ALL position symbols for live price updates, not just options
                    if (leg.symbol) {
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

    // Store the last positions used for chart generation so we can regenerate when adjustedNetCredit changes
    const lastChartPositions = ref(null);

    // Watch for adjusted net credit changes to update chart
    watch(adjustedNetCredit, (newCredit, oldCredit) => {
      if (newCredit !== oldCredit && lastChartPositions.value) {
        // 🔑 KEY FIX: Regenerate chart data with new adjusted net credit
        // Import the chart generation function and regenerate with stored positions
        import("../utils/chartUtils").then(({ generateMultiLegPayoff }) => {
          try {
            const chartResult = generateMultiLegPayoff(
              lastChartPositions.value, 
              currentPrice.value, 
              adjustedNetCredit.value
            );
            
            if (chartResult) {
              chartData.value = chartResult;
            } else {
              console.warn("⚠️ PositionsView: Chart regeneration returned null");
            }
          } catch (error) {
            console.error("❌ PositionsView: Error regenerating chart:", error);
          }
        });
      }
    });

    // Handle symbol selection from TopBar
    const onSymbolSelected = async (symbol) => {
      // The global symbol state will be updated automatically by TopBar
      // No need to manually update it here
    };

    // Show/hide right panel based on selected legs (same as OptionsTrading)
    watch(
      selectedLegs,
      (newLegs) => {
        const hasLegs = newLegs.length > 0;
        
        // Auto-expand right panel to Analysis tab when legs are selected
        if (hasLegs) {
          isRightPanelExpanded.value = true;
        } else {
          // Auto-collapse when no legs selected
          isRightPanelExpanded.value = false;
        }
      },
      { immediate: true }
    );

    // 🔑 KEY FIX: Watch for selectedLegs changes to trigger chart generation
    // This ensures chart updates when legs are selected, especially after symbol changes
    // TEMPORARILY DISABLED TO DEBUG RACE CONDITION
    /*
    watch(
      selectedLegs,
      async (newLegs) => {
        // Wait for any pending operations to complete
        await nextTick();
        
        // Regenerate chart with current selectedLegs
        updateChartWithSelectedLegs();
      },
      { deep: true }
    );
    */

    // 🔑 NEW: Function to update chart based on selected legs
    const updateChartWithSelectedLegs = async () => {
      // Don't update chart during symbol transitions
      if (isSymbolTransitioning.value) {
        console.log("⏸️ PositionsView: Skipping chart update during symbol transition");
        return;
      }
      
      try {
        if (selectedLegs.value.length > 0) {
          // Import chart generation function
          const { generateMultiLegPayoff } = await import("../utils/chartUtils");
          
          // Convert selected legs to positions format for chart generation
          const legPositions = selectedLegs.value.map(leg => ({
            symbol: leg.symbol,
            qty: leg.side === 'buy' ? leg.quantity : -leg.quantity,
            strike_price: leg.strike_price,
            option_type: leg.type,
            expiry_date: leg.expiry,
            current_price: leg.current_price,
            bid: leg.bid,
            ask: leg.ask,
            avg_entry_price: leg.avg_entry_price,
            asset_class: "us_option",
            isSelected: true,
            isExisting: leg.source === 'positions'
          }));
          
          // Ensure we have a valid current price for chart generation
          const chartCurrentPrice = currentPrice.value || 0;
          if (chartCurrentPrice === 0) {
            console.warn("⚠️ PositionsView: Current price is 0, chart may not render correctly");
            return; // Don't generate chart without valid price
          }
          
          // Generate chart with selected legs
          const chartResult = generateMultiLegPayoff(
            legPositions, 
            chartCurrentPrice, 
            adjustedNetCredit.value
          );
          
          if (chartResult) {
            chartData.value = chartResult;
            console.log(`✅ PositionsView: Chart updated with ${selectedLegs.value.length} selected legs`);
          } else {
            console.warn("⚠️ PositionsView: Chart generation returned null for selected legs");
            chartData.value = null;
          }
        } else {
          // No legs selected, clear chart
          chartData.value = null;
        }
      } catch (error) {
        console.error("❌ PositionsView: Error updating chart with selected legs:", error);
        chartData.value = null;
      }
    };

    // Component cleanup system
    const cleanupComponentRegistrations = () => {
      // Unregister all symbols this component was using
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      
      // Clear local tracking
      registeredSymbols.clear();
      livePrices.clear();
    };

    // Watch for symbol changes to clean up old registrations
    watch(
      () => currentSymbol.value,
      (newSymbol, oldSymbol) => {
        if (newSymbol !== oldSymbol) {
          // Unregister all current symbols
          cleanupComponentRegistrations();
          
          // 🔑 KEY FIX: Clear chart data when symbol changes to prevent stale data
          chartData.value = null;
          lastChartPositions.value = null;
        }
      },
      { immediate: true }
    );

    // Lifecycle
    onMounted(async () => {
      // Set up centralized symbol selection listener
      const cleanup = setupSymbolSelectionListener();
      
      // Store cleanup for unmount
      onUnmounted(cleanup);
    });

    // Clean up when the component is unmounted
    onUnmounted(() => {
      cleanupComponentRegistrations();
    });

    return {
      // State
      loading,
      error,
      positionGroups,
      showOptions,
      showStocks,
      expandedGroups,

      // Computed
      filteredPositionGroups,
      totalPlDay,
      totalPlOpen,

      // Methods
      fetchPositions,
      toggleFilter,
      toggleExpand,
      formatCurrency,
      formatNumber,
      formatLegQty,
      formatLegInfo,
      formatLegDate,
      formatLegStrike,
      formatLegType,
      formatLegDays,
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
      getStrategyDisplayTrdPrc,
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
      // Position selection functions
      isLegSelected,
      togglePositionLegSelection,
      updateChartWithSelectedLegs,
      isSymbolTransitioning,
      // Bottom Trading Panel integration
      hasSelectedLegs,
      onReviewSend,
      onPriceAdjusted,
      adjustedNetCredit,
      
      // Order management
      showOrderConfirmation,
      showOrderResult,
      orderData,
      orderResult,
      isPlacingOrder,
      handleOrderConfirmation,
      handleOrderCancellation,
      handleOrderResultClose,
      clearAllSelections,
      handleOrderEdit,
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
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.position-leg-row:hover {
  background-color: var(--bg-tertiary);
  border-color: #444444;
}

.position-leg-row.selected {
  background-color: rgba(255, 215, 0, 0.1);
  border-color: #ffd700;
  box-shadow: 0 0 0 1px rgba(255, 215, 0, 0.3);
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

.leg-description .shares-label {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  background-color: var(--bg-tertiary, #1a1d23);
  padding: 2px 6px;
  border-radius: 3px;
  font-weight: 500;
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
