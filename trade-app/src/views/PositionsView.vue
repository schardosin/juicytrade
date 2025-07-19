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
                    <th class="etf-delta-header">Etf Δ</th>
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
                        :class="group.pl_day >= 0 ? 'profit' : 'loss'"
                      >
                        {{ formatCurrency(group.pl_day) }}
                      </td>
                      <td
                        class="pl-open-cell"
                        :class="group.pl_open >= 0 ? 'profit' : 'loss'"
                      >
                        {{ formatCurrency(group.pl_open) }}
                      </td>
                      <td class="etf-delta-cell">
                        {{ formatNumber(group.total_qty) }}
                      </td>
                      <td class="bid-cell">--</td>
                      <td class="ask-cell">--</td>
                      <td class="trd-prc-cell">--</td>
                      <td class="dsopn-cell">{{ group.dte || "--" }}</td>
                      <td class="beta-delta-cell">--</td>
                      <td class="dte-cell">
                        {{ group.dte ? group.dte + "d" : "--" }}
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
                              :class="strategy.pl_day >= 0 ? 'profit' : 'loss'"
                            >
                              {{ formatCurrency(strategy.pl_day) }}
                            </td>
                            <td
                              class="pl-open-cell"
                              :class="strategy.pl_open >= 0 ? 'profit' : 'loss'"
                            >
                              {{ formatCurrency(strategy.pl_open) }}
                            </td>
                            <td class="etf-delta-cell">
                              {{ formatNumber(strategy.total_qty) }}
                            </td>
                            <td class="bid-cell">--</td>
                            <td class="ask-cell">--</td>
                            <td class="trd-prc-cell">--</td>
                            <td class="dsopn-cell">
                              {{ strategy.dte || "--" }}
                            </td>
                            <td class="beta-delta-cell">--</td>
                            <td class="dte-cell">
                              {{ strategy.dte ? strategy.dte + "d" : "--" }}
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
                              <div class="leg-card">
                                <span class="leg-qty">{{
                                  formatLegQty(leg.qty)
                                }}</span>
                                <span class="leg-date">{{
                                  formatLegDate(leg)
                                }}</span>
                                <span class="leg-days">{{
                                  formatLegDays(leg)
                                }}</span>
                                <span class="leg-strike">{{
                                  formatLegStrike(leg)
                                }}</span>
                                <span
                                  class="leg-type"
                                  :class="getLegTypeClass(leg)"
                                >
                                  {{ formatLegType(leg) }}
                                </span>
                              </div>
                            </td>
                            <td
                              class="pl-day-cell"
                              :class="
                                leg.unrealized_pl >= 0 ? 'profit' : 'loss'
                              "
                            >
                              {{ formatCurrency(leg.unrealized_pl) }}
                            </td>
                            <td
                              class="pl-open-cell"
                              :class="
                                leg.unrealized_pl >= 0 ? 'profit' : 'loss'
                              "
                            >
                              {{ formatCurrency(leg.unrealized_pl) }}
                            </td>
                            <td class="etf-delta-cell">
                              {{ formatNumber(leg.qty) }}
                            </td>
                            <td class="bid-cell">--</td>
                            <td class="ask-cell">--</td>
                            <td class="trd-prc-cell">
                              {{ formatCurrency(leg.current_price) }}
                            </td>
                            <td class="dsopn-cell">
                              {{ strategy.dte || "--" }}
                            </td>
                            <td class="beta-delta-cell">--</td>
                            <td class="dte-cell">
                              {{ strategy.dte ? strategy.dte + "d" : "--" }}
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
                          <td
                            class="pl-day-cell"
                            :class="leg.unrealized_pl >= 0 ? 'profit' : 'loss'"
                          >
                            {{ formatCurrency(leg.unrealized_pl) }}
                          </td>
                          <td
                            class="pl-open-cell"
                            :class="leg.unrealized_pl >= 0 ? 'profit' : 'loss'"
                          >
                            {{ formatCurrency(leg.unrealized_pl) }}
                          </td>
                          <td class="etf-delta-cell">
                            {{ formatNumber(leg.qty) }}
                          </td>
                          <td class="bid-cell">--</td>
                          <td class="ask-cell">--</td>
                          <td class="trd-prc-cell">
                            {{ formatCurrency(leg.current_price) }}
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
        :forceExpanded="isRightPanelExpanded"
        :forceSection="rightPanelSection"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from "vue";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import RightPanel from "../components/RightPanel.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { api } from "../services/api.js";

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

    // Other reactive state
    const selectedOptions = ref([]);
    const chartData = ref(null);
    const additionalQuoteData = ref({});
    const optionsManager = ref({
      flattenedData: ref([]),
    });
    const loading = ref(true);
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

    const onPositionsChanged = () => {
      // Handle positions changed logic here
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
        (sum, group) => sum + group.pl_day,
        0
      );
    });

    const totalPlOpen = computed(() => {
      return filteredPositionGroups.value.reduce(
        (sum, group) => sum + group.pl_open,
        0
      );
    });

    // Methods
    const fetchPositions = async () => {
      try {
        loading.value = true;
        error.value = null;

        const response = await api.getPositions();

        if (response.enhanced && response.symbol_groups) {
          // New hierarchical structure
          positionGroups.value = convertSymbolGroupsToDisplay(
            response.symbol_groups
          );
        } else if (response.enhanced && response.position_groups) {
          // Old enhanced structure
          positionGroups.value = response.position_groups;
        } else if (response.positions) {
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
      const displayGroups = [];

      symbolGroups.forEach((symbolGroup) => {
        // Create a symbol-level group (collapsed by default)
        const symbolDisplayGroup = {
          id: `symbol_${symbolGroup.symbol}`,
          symbol: symbolGroup.symbol,
          strategy: `${symbolGroup.strategies.length} Strategies`,
          asset_class: symbolGroup.asset_class,
          total_qty: symbolGroup.total_qty,
          total_cost_basis: symbolGroup.total_cost_basis,
          total_market_value: symbolGroup.total_market_value,
          total_unrealized_pl: symbolGroup.total_pl_open,
          total_unrealized_plpc: 0,
          pl_day: symbolGroup.total_pl_day,
          pl_open: symbolGroup.total_pl_open,
          legs: [], // Will be populated when expanded
          strategies: symbolGroup.strategies, // Store strategies for expansion
          order_date: null,
          expiration_date: null,
          dte: null,
          isSymbolGroup: true, // Flag to identify symbol-level groups
        };

        // When expanded, we'll show strategy rows and leg rows
        // For now, flatten all legs into the symbol group for display
        symbolGroup.strategies.forEach((strategy) => {
          strategy.legs.forEach((leg) => {
            symbolDisplayGroup.legs.push({
              ...leg,
              strategy_name: strategy.name,
              strategy_pl_day: strategy.pl_day,
              strategy_pl_open: strategy.pl_open,
            });
          });
        });

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
        return parts ? parts.expiry : "--";
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

    const onTradeModeChanged = (mode) => {
      // Not used in positions view
    };

    // Lifecycle
    onMounted(async () => {
      // Global state is automatically initialized - no manual setup needed
      await fetchPositions();
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
  color: var(--color-success);
}

.pl-day-cell.loss,
.pl-open-cell.loss {
  color: var(--color-danger);
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
