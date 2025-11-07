<template>
  <div 
    class="options-trading-container"
    :style="mobileTradingPanelStyles"
  >
    <!-- Top Bar -->
    <TopBar @toggle-mobile-nav="showMobileNav = true" />

    <!-- Mobile Navigation Drawer -->
    <MobileNavDrawer
      v-if="isMobile"
      :visible="showMobileNav"
      @close="showMobileNav = false"
      @navigate="onMobileNavigation"
    />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Desktop Left Navigation -->
      <SideNav v-if="!isMobile" />

      <!-- Content Area -->
      <div class="content-area" :class="{ 'mobile-layout': isMobile }">
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
          :selectedTradeMode="selectedTradeMode"
          :tradeModes="tradeModes"
          :showTradeMode="true"
          @trade-mode-changed="onTradeModeChanged"
          @open-mobile-search="onOpenMobileSearch"
        />

        <!-- Options Mode: Options Chain Section -->
        <div v-if="selectedTradeMode === 'options'" class="options-section">
          <!-- Collapsible Options Chain -->
          <div class="options-chain-wrapper">
            <CollapsibleOptionsChain
              :symbol="currentSymbol"
              :underlyingPrice="currentPrice"
              :expirationDates="optionsManager.expirationDates.value"
              :optionsDataByExpiration="optionsManager.dataByExpiration.value"
              :loading="shouldShowLoading"
              :error="optionsManager.error.value"
              :expandedExpirations="optionsManager.expandedExpirations.value"
              :currentStrikeCount="optionsManager.strikeCount.value"
              :ivxData="ivxData"
              @expiration-expanded="onExpirationExpanded"
              @expiration-collapsed="onExpirationCollapsed"
              @strike-count-changed="onStrikeCountChanged"
            />
          </div>
        </div>

        <!-- Shares Mode: Chart Section -->
        <div v-if="selectedTradeMode === 'shares'" class="shares-section">
          <LightweightChart
            :symbol="currentSymbol"
            :theme="'dark'"
            :height="chartHeight"
            :enableRealtime="true"
            :livePrice="livePrice"
          />
        </div>
      </div>

      <!-- Right Panel (visible in both Options and Shares modes) -->
      <RightPanel
        :currentSymbol="currentSymbol"
        :currentPrice="currentPrice"
        :priceChange="priceChange"
        :isLivePrice="isLivePrice"
        :chartData="chartData"
        :additionalQuoteData="additionalQuoteData"
        :optionsChainData="optionsManager.flattenedData.value"
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
      @clear-trade-reset="handleClearTradeReset"
    />

    <!-- Options Trading Panel -->
    <BottomTradingPanel
      v-if="selectedTradeMode === 'options'"
      :visible="showBottomPanel"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      :moveStrike="optionsManager.moveStrike"
      @review-send="onReviewAndSend"
      @price-adjusted="onPriceAdjusted"
    />

    <!-- Shares Trading Panel -->
    <SharesTradingPanel
      ref="sharesTradingPanelRef"
      v-if="selectedTradeMode === 'shares'"
      :visible="showSharesPanel"
      :symbol="currentSymbol"
      :underlyingPrice="currentPrice"
      :resetTrigger="resetSharesPanelTrigger"
      @review-send="onSharesReviewAndSend"
      @clear-trade="onSharesClearTrade"
    />

    <!-- Mobile Bottom Button Bar (only on mobile) -->
    <MobileBottomButtonBar
      v-if="isMobile"
      :activeSection="showMobileOverlay ? activeMobileSection : null"
      @section-selected="onMobileSectionSelected"
    />

    <!-- Mobile Full Screen Overlay (only on mobile) -->
    <MobileFullScreenOverlay
      v-if="isMobile"
      :visible="showMobileOverlay"
      :activeSection="activeMobileSection"
      :currentSymbol="currentSymbol"
      :currentPrice="currentPrice"
      :priceChange="priceChange"
      :isLivePrice="isLivePrice"
      :chartData="chartData"
      :additionalQuoteData="additionalQuoteData"
      :allPositions="allMobilePositions"
      :checkedPositions="mobileCheckedPositions"
      :isAllSelected="isMobileAllSelected"
      :isIndeterminate="isMobileIndeterminate"
      @close="closeMobileOverlay"
      @toggle-position-check="onMobileTogglePositionCheck"
      @toggle-select-all="onMobileToggleSelectAll"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue";
import { DateTime } from "luxon";
import TopBar from "../components/TopBar.vue";
import SideNav from "../components/SideNav.vue";
import MobileNavDrawer from "../components/MobileNavDrawer.vue";
import MobileBottomButtonBar from "../components/MobileBottomButtonBar.vue";
import MobileFullScreenOverlay from "../components/MobileFullScreenOverlay.vue";
import SymbolHeader from "../components/SymbolHeader.vue";
import CollapsibleOptionsChain from "../components/CollapsibleOptionsChain.vue";
import PayoffChart from "../components/PayoffChart.vue";
import BottomTradingPanel from "../components/BottomTradingPanel.vue";
import SharesTradingPanel from "../components/SharesTradingPanel.vue";
import LightweightChart from "../components/LightweightChart.vue";
import OrderConfirmationDialog from "../components/OrderConfirmationDialog.vue";
import RightPanel from "../components/RightPanel.vue";
import { useOrderManagement } from "../composables/useOrderManagement";
import { useGlobalSymbol } from "../composables/useGlobalSymbol";
import { useOptionsChainManager } from "../composables/useOptionsChainManager";
import { useMarketData } from "../composables/useMarketData.js";
import { useSelectedLegs } from "../composables/useSelectedLegs.js";
import { useTradeNavigation } from "../composables/useTradeNavigation.js";
import { useMobileDetection } from "../composables/useMobileDetection.js";
import api from "../services/api";
import authService from "../services/authService";
// webSocketClient no longer needed - global state is automatically updated by SmartMarketDataStore
// import webSocketClient from "../services/webSocketClient";
import { generateMultiLegPayoff } from "../utils/chartUtils";

export default {
  name: "OptionsTrading",
  components: {
    TopBar,
    SideNav,
    MobileNavDrawer,
    MobileBottomButtonBar,
    MobileFullScreenOverlay,
    SymbolHeader,
    CollapsibleOptionsChain,
    PayoffChart,
    BottomTradingPanel,
    SharesTradingPanel,
    LightweightChart,
    OrderConfirmationDialog,
    RightPanel,
  },
  setup() {
    // Component refs
    const sharesTradingPanelRef = ref(null);
    
    // Mobile detection
    const { isMobile, isTablet, isDesktop } = useMobileDetection();
    
    // Mobile navigation state
    const showMobileNav = ref(false);
    
    // Mobile overlay state
    const showMobileOverlay = ref(false);
    const activeMobileSection = ref('overview');
    const mobileCheckedPositions = ref(new Set());
    
    const { pendingOrder, clearPendingOrder } = useTradeNavigation();
    // Use centralized order management with cleanup callback
    const { getIvxData } = useMarketData();    const {
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

    // Use global symbol state with centralized symbol selection
    const { globalSymbolState, updateSymbol, updatePrice, updateMarketStatus, setupSymbolSelectionListener } =
      useGlobalSymbol({
        onSymbolChange: async (symbolData) => {
          // Clear existing data
          clearAllSelections();
          optionsChainData.value = [];
          optionsDataByExpiration.value = {}; // Clear CollapsibleOptionsChain data (legacy)
          expirationDates.value = [];
          selectedExpiry.value = null;

          // CRITICAL: Manually clear the options manager state completely
          optionsManager.clearAllData(); // This should clear everything including expirationDates
          
          // CRITICAL: Update the symbol ref to trigger the watcher
          symbolRef.value = symbolData.symbol;
          
          // The symbol watcher should now trigger automatically, but also trigger manually as backup
          nextTick(() => {
            if (optionsManager.loadExpirationDates) {
              optionsManager.loadExpirationDates().catch(error => {
                console.error(`Failed to load expiration dates for ${symbolData.symbol}:`, error);
              });
            }
          });
        }
      });

    // Use centralized selected legs (positions loaded globally by useGlobalSymbol)
    const { selectedLegs, addLeg, clearAll: clearSelectedLegs } = useSelectedLegs();

    const activePositions = ref([]);
    const hasExplicitPositionSelection = ref(false); // Track if user has made checkbox selections

    // Use centralized options chain manager
    // Create symbol and price refs that are properly reactive to globalSymbolState
    const symbolRef = ref(globalSymbolState.currentSymbol);
    const priceRef = ref(globalSymbolState.currentPrice);
    
    // Watch for global symbol changes and update the refs
    watch(() => globalSymbolState.currentSymbol, (newSymbol) => {
      symbolRef.value = newSymbol;
    });
    
    watch(() => globalSymbolState.currentPrice, (newPrice) => {
      priceRef.value = newPrice;
    });
    
    const optionsManager = useOptionsChainManager(
      symbolRef,  // Use reactive ref
      priceRef,   // Use reactive ref
      20 // default strike count
    );

    // Get IVx data for current symbol (after globalSymbolState is initialized)
    const rawIvxData = getIvxData(
      computed(() => {
        return globalSymbolState.currentSymbol;
      })
    );

    // Create defensive IVx data with fallbacks
    const ivxData = computed(() => {
      const data = rawIvxData.value;
      if (!data) {
        return {
          isLoading: true,
          status: 'loading',
          expirations: [],
          progress: { completed: 0, total: 0 },
          symbol: globalSymbolState.currentSymbol,
          error: null
        };
      }
      return data;
    });

    // Local reactive data (non-symbol related)
    const selectedExpiry = ref(null);
    const chartData = ref(null);
    const selectedTradeMode = ref("options");
    const isRightPanelExpanded = ref(false);
    const showBottomPanel = ref(false);
    const showSharesPanel = ref(false);
    const currentStrikeCount = ref(20);
    const adjustedNetCredit = ref(null);
    const rightPanelControlsChart = ref(false); // Flag to indicate RightPanel is controlling chart

    // Legacy data for backward compatibility (will be removed gradually)
    const optionsChainData = ref([]);
    const optionsDataByExpiration = ref({});
    const expirationDates = ref([]);
    const optionsChainLoading = ref(false);
    const optionsChainError = ref(null);

    // Trade modes
    const tradeModes = [
      { label: "Options", value: "options" },
      { label: "Shares", value: "shares" },
      { label: "Futures", value: "futures" },
    ];

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

    const priceChangeClass = computed(() => ({
      positive: priceChange.value > 0,
      negative: priceChange.value < 0,
      neutral: priceChange.value === 0,
    }));

    // Smart loading state - only show loading when we have no expiration dates to display
    // This prevents blocking the UI while expiration dates are loading
    const shouldShowLoading = computed(() => {
      // Show loading only if we're loading expiration dates AND we have no expirations to show
      return optionsManager?.expirationDatesLoading?.value && 
             optionsManager?.expirationDates?.value?.length === 0;
    });

    const marketStatusClass = computed(() => ({
      open: marketStatus.value === "Market Open",
      closed: marketStatus.value === "Market Closed",
      "pre-market": marketStatus.value === "Pre-Market",
      "after-hours": marketStatus.value === "After Hours",
    }));

    const estimatedCost = computed(() => {
      if (selectedLegs.value.length === 0) return null;

      let totalCost = 0;
      selectedLegs.value.forEach((leg) => {
        const price = leg.side === "buy" ? leg.ask : leg.bid;
        const cost = leg.side === "buy" ? -price : price;
        totalCost += cost * leg.quantity * 100; // Options are in contracts of 100
      });

      return totalCost;
    });

    // Computed property to determine which section to show in right panel
    const rightPanelSection = computed(() => {
      // If options are selected, show analysis section, otherwise null (let user choose)
      return selectedLegs.value.length > 0 || activePositions.value.length > 0 ? "analysis" : null;
    });

    // Get positions data using the same method as desktop RightPanel
    const { getPositionsForSymbol } = useMarketData();
    
    // Mobile computed properties - exact same logic as desktop RightPanel
    const allMobilePositions = computed(() => {
      const positions = [];
      const existingSymbols = new Set();
      const symbolGroup = getSymbolGroup(currentSymbol.value);

      // Get existing positions using the same method as desktop RightPanel
      const positionsComputed = getPositionsForSymbol(currentSymbol.value);
      const positionsData = positionsComputed.value;
      
      // Add existing positions for the current symbol group
      if (positionsData?.positions && Array.isArray(positionsData.positions)) {
        positionsData.positions.forEach((position) => {
          if (symbolGroup.includes(position.underlying_symbol)) {
            // CRITICAL FIX: Parse option symbol to get missing details for existing positions
            const parsedOption = parseOptionSymbol(position.symbol);
            
            // Look up current market price from options chain data for existing positions
            const chainOption = optionsManager.flattenedData.value.find(
              (opt) => opt.symbol === position.symbol
            );

            // Use entry price as fallback if current price is 0
            let currentPrice = position.current_price || position.avg_entry_price || 0;
            let unrealizedPL = position.unrealized_pl || 0;

            // If we don't have current P&L but have entry price, estimate it
            if (unrealizedPL === 0 && position.avg_entry_price && position.avg_entry_price > 0) {
              // For now, assume current price equals entry price (no P&L change)
              currentPrice = position.avg_entry_price;
              unrealizedPL = 0; // No change assumed
            }

            // Update current price and P&L if we have chain data
            if (chainOption && position.avg_entry_price && chainOption.bid && chainOption.ask) {
              // Use mid price for current market value
              currentPrice = (chainOption.bid + chainOption.ask) / 2;

              // Recalculate P&L with current market price
              const qty = position.qty;
              if (qty > 0) {
                // Long position: (current_price - entry_price) * qty * 100
                unrealizedPL = (currentPrice - position.avg_entry_price) * qty * 100;
              } else {
                // Short position: (entry_price - current_price) * |qty| * 100
                unrealizedPL = (position.avg_entry_price - currentPrice) * Math.abs(qty) * 100;
              }
            }

            const positionData = {
              ...position,
              id: `existing:${position.symbol || position.id}`,
              symbol: position.symbol,
              asset_class: position.asset_class || "us_option",
              underlying_symbol: position.underlying_symbol,
              qty: position.qty,
              // CRITICAL FIX: Use parsed option data to fill missing fields
              strike_price: parsedOption?.strike_price || position.strike_price || 0,
              option_type: parsedOption?.option_type || position.option_type || "call",
              expiry_date: parsedOption?.expiry_date || position.expiry_date || "",
              current_price: currentPrice,
              avg_entry_price: position.avg_entry_price || 
                (position.cost_basis ? Math.abs(position.cost_basis / (position.qty * 100)) : 0),
              unrealized_pl: unrealizedPL,
              isExisting: true,
              isSelected: false,
            };

            positions.push(positionData);
            existingSymbols.add(position.symbol);
          }
        });
      }

      // Add selected legs from centralized store as new positions (same as desktop RightPanel)
      selectedLegs.value.forEach((leg, index) => {
        positions.push({
          id: `selected:${leg.symbol}:${index}`,
          symbol: leg.symbol,
          asset_class: "us_option",
          qty: leg.side === "buy" ? leg.quantity : -leg.quantity,
          strike_price: leg.strike_price,
          option_type: leg.type,
          expiry_date: leg.expiry,
          current_price: leg.current_price || ((leg.bid + leg.ask) / 2) || 0,
          avg_entry_price: leg.avg_entry_price,
          unrealized_pl: 0,
          isExisting: false,
          isSelected: true, // This is key - newly selected legs should be marked as selected
        });
      });

      return positions;
    });

    const isMobileAllSelected = computed(() => {
      if (allMobilePositions.value.length === 0) return false;
      return allMobilePositions.value.every((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      );
    });

    const isMobileIndeterminate = computed(() => {
      if (allMobilePositions.value.length === 0) return false;
      const checkedCount = allMobilePositions.value.filter((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      ).length;
      return checkedCount > 0 && checkedCount < allMobilePositions.value.length;
    });

    // Helper function to get symbol group (handles SPX/SPXW grouping)
    const getSymbolGroup = (symbol) => {
      if (symbol === "SPX" || symbol === "SPXW") {
        return ["SPX", "SPXW"];
      }
      return [symbol];
    };

    // Helper function to parse option symbol for details (same as desktop RightPanel)
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

    // Methods
    const fetchSymbolData = async (symbol) => {
      try {
        // Don't fetch data if user is not authenticated
        if (!authService.isAuthenticated()) {
          console.log('🔒 Skipping symbol data fetch - user not authenticated');
          return;
        }
        
        // Only fetch price via API if we don't have live data already
        if (!isLivePrice.value || currentPrice.value === 0) {
          const price = await api.getUnderlyingPrice(symbol);
          if (price !== null) {
            updatePrice({ price, isLive: false });
          }
        }
        // Subscriptions are now handled automatically by the SmartMarketDataStore.
        // No need to manually call webSocketClient here. The SymbolHeader
        // component will trigger the necessary stock subscription.
      } catch (error) {
        console.error("Error fetching symbol data:", error);
      }
    };

    const fetchExpirationDates = async (symbol) => {
      try {
        // Don't fetch data if user is not authenticated
        if (!authService.isAuthenticated()) {
          console.log('🔒 Skipping expiration dates fetch - user not authenticated');
          return;
        }
        
        optionsChainLoading.value = true;
        optionsChainError.value = null;

        const response = await api.getAvailableExpirations(symbol);

        if (response && response.length > 0) {
          // Store raw expiration dates for CollapsibleOptionsChain
          expirationDates.value = response;

          // Also create formatted dates for backward compatibility
          const dates = response.map((expirationEntry) => {
            // Handle both old string format and new object format
            const dateStr = typeof expirationEntry === 'string' ? expirationEntry : expirationEntry.date;
            const [year, month, day] = dateStr.split("-").map(Number);
            const date = new Date(Date.UTC(year, month - 1, day));
            const today = new Date();
            today.setHours(0, 0, 0, 0);

            return {
              label: date.toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
                year:
                  date.getUTCFullYear() !== today.getFullYear()
                    ? "numeric"
                    : undefined,
                timeZone: "UTC",
              }),
              value: dateStr,
            };
          });

          if (dates.length > 0) {
            selectedExpiry.value = dates[0].value;
          }
        } else {
          console.warn("No expiration dates returned from API");
          expirationDates.value = [];
          optionsChainError.value = "No expiration dates available";
        }
      } catch (error) {
        console.error("Error fetching expiration dates from API:", error);
        expirationDates.value = [];
        optionsChainError.value = "Failed to load expiration dates";
      } finally {
        optionsChainLoading.value = false;
      }
    };

    // The startStreaming logic is now handled globally by the SmartMarketDataStore
    // when it is initialized in main.js. We can remove this local function.
    // The onPriceUpdate logic is also handled inside the store.

    const onExpiryChange = async () => {
      if (selectedExpiry.value) {
        // The subscription logic is now handled automatically by the
        // CollapsibleOptionsChain component via the SmartMarketDataStore.
        // No need to manually subscribe here.
      }
    };

    const onOptionSelected = (optionSelection) => {
      const existingIndex = selectedOptions.value.findIndex(
        (sel) => sel.symbol === optionSelection.symbol
      );

      // The optionSelection already contains the correct expiry from CollapsibleOptionsChain
      // Don't override it with selectedExpiry.value which is from the old single-expiration system
      const selectionWithExpiry = {
        ...optionSelection,
        // Keep the expiry that came from the selection (from the correct expiration group)
        expiry: optionSelection.expiry || optionSelection.expiration_date,
      };

      if (existingIndex >= 0) {
        // Update existing selection
        selectedOptions.value[existingIndex] = selectionWithExpiry;
      } else {
        // Add new selection
        selectedOptions.value.push(selectionWithExpiry);
      }

      // Force show bottom panel and expand right panel
      showBottomPanel.value = true;
      isRightPanelExpanded.value = true;
    };

    const onOptionDeselected = (symbol) => {
      const index = selectedOptions.value.findIndex(
        (sel) => sel.symbol === symbol
      );
      if (index >= 0) {
        selectedOptions.value.splice(index, 1);

        // Auto-hide panel if no options left
        if (selectedOptions.value.length === 0) {
          showBottomPanel.value = false;
          isRightPanelExpanded.value = false;
        }
      }
    };

    const clearAllSelections = () => {
      clearSelectedLegs();
      activePositions.value = [];
      chartData.value = null;
      showBottomPanel.value = false;
      isRightPanelExpanded.value = false;
    };

    const updateChartData = async (positions = null) => {
      // Chart ONLY uses positions from RightPanel checkboxes
      // No fallback to selectedLegs - if no positions are checked, show no chart
      
      if (!positions || positions.length === 0) {
        chartData.value = null;
        return;
      }

      try {
        const payoffData = generateMultiLegPayoff(
          positions,
          currentPrice.value,
          adjustedNetCredit.value
        );

        // Force reactivity by creating a new object reference
        chartData.value = {
          ...payoffData,
          timestamp: Date.now(), // Add timestamp to ensure object reference changes
        };
      } catch (error) {
        console.error("Error generating payoff chart:", error);
        chartData.value = null;
      }
    };

    const onOrderPlaced = (orderData) => {
      initializeOrder(orderData);
    };

    const toggleRightPanel = () => {
      isRightPanelExpanded.value = !isRightPanelExpanded.value;
    };

    const onReviewAndSend = (orderData) => {
      // Hide bottom panel and show confirmation dialog
      showBottomPanel.value = false;
      initializeOrder(orderData);
    };

    // onUpdateLegQuantity is no longer needed - quantity updates are handled by the centralized store

    const handleOrderEdit = () => {
      // Hide confirmation dialog and show appropriate trading panel based on mode
      if (selectedTradeMode.value === 'shares') {
        showSharesPanel.value = true;
      } else if (selectedTradeMode.value === 'options') {
        showBottomPanel.value = true;
      }
      // Also need to hide the confirmation dialog
      handleOrderCancellation();
    };

    const onSymbolSelected = async (symbol) => {
      // Clear existing data
      clearAllSelections();
      optionsChainData.value = [];
      optionsDataByExpiration.value = {}; // Clear CollapsibleOptionsChain data
      expirationDates.value = [];
      selectedExpiry.value = null;

      // Update global symbol state (positions will be loaded automatically by useGlobalSymbol)
      updateSymbol(symbol);

      // Fetch new data for the selected symbol
      try {
        await fetchSymbolData(symbol.symbol);
        await fetchExpirationDates(symbol.symbol);

        if (selectedExpiry.value) {
          onExpiryChange();
        }
      } catch (error) {
        console.error("Error loading data for new symbol:", error);
      }
    };

    const onTradeModeChanged = (mode) => {
      selectedTradeMode.value = mode;
      
      // Clear selections when switching modes
      clearAllSelections();
      
      // Show appropriate panels based on mode
      if (mode === "shares") {
        showBottomPanel.value = false;
        showSharesPanel.value = true;
        isRightPanelExpanded.value = false;
      } else if (mode === "options") {
        showSharesPanel.value = false;
        // showBottomPanel will be controlled by options selection logic
      }
    };

    // Shares-specific methods
    const onSharesReviewAndSend = (orderData) => {
      // Hide shares panel and show confirmation dialog
      showSharesPanel.value = false;
      initializeOrder(orderData);
    };

    const onSharesClearTrade = () => {
      // Clear any shares-specific state if needed
      console.log("Shares trade cleared");
    };

    // Create a reactive trigger for reset
    const resetSharesPanelTrigger = ref(0);
    
    const handleClearTradeReset = async () => {
      // Reset SharesTradingPanel to default values while keeping it visible
      // This is called when the Clear Trade button is clicked for equity orders
      
      // First ensure we're in shares mode
      selectedTradeMode.value = "shares";
      showSharesPanel.value = true;
      
      // Wait for component to be rendered
      await nextTick();
      
      // Trigger reset via reactive property instead of ref
      resetSharesPanelTrigger.value++;
    };

    // Window height tracking for responsive chart
    const windowHeight = ref(window.innerHeight);
    
    // Dynamic chart height calculation
    const chartHeight = computed(() => {
      // Calculate available space dynamically
      // TopBar: ~60px, SymbolHeader: ~80px
      const fixedElements = 140;
      
      // Trading panel height varies based on mode and visibility
      let tradingPanelHeight = 0;
      if (selectedTradeMode.value === 'shares' && showSharesPanel.value) {
        // SharesTradingPanel without stats header is smaller (~160px)
        tradingPanelHeight = 160;
      } else if (selectedTradeMode.value === 'options' && showBottomPanel.value) {
        // BottomTradingPanel with stats is larger (~200px)
        tradingPanelHeight = 200;
      }
      
      // Chart controls at bottom: ~60px
      const chartControls = 60;
      
      // Calculate remaining space for chart using reactive window height
      const availableHeight = windowHeight.value - fixedElements - tradingPanelHeight - chartControls;
      
      // Ensure minimum height
      return Math.max(availableHeight, 300);
    });

    // Live price data for chart
    const livePrice = computed(() => {
      // This would come from the smart market data store
      return {
        price: currentPrice.value,
        bid: currentPrice.value - 0.01,
        ask: currentPrice.value + 0.01,
      };
    });

    const onRightPanelCollapsed = () => {
      // Right panel collapsed
    };

    const onPositionsChanged = (checkedPositions) => {
      activePositions.value = checkedPositions;
      // The watcher on activePositions will handle the chart update
    };

    // CollapsibleOptionsChain event handlers - now using centralized manager
    const onExpirationExpanded = async (expirationKey, expirationData) => {
      await optionsManager.expandExpiration(expirationKey, expirationData);
    };

    const onExpirationCollapsed = (expirationDate) => {
      optionsManager.collapseExpiration(expirationDate);
    };

    const onStrikeCountChanged = (newStrikeCount) => {
      currentStrikeCount.value = newStrikeCount;
      optionsManager.updateStrikeCount(newStrikeCount);
    };

    const onPriceAdjusted = (data) => {
      adjustedNetCredit.value = data.adjustedNetCredit;
    };

    // Helper function to check if we have existing positions for current symbol
    const hasExistingPositionsForSymbol = () => {
      // This would need to check if there are existing positions
      // For now, return false to let selectedOptions watcher work
      // In a real implementation, this would check the positions API
      return false;
    };

    // Additional quote data for the right panel
    const additionalQuoteData = computed(() => ({
      // Mock data - in real implementation, this would come from API
      open: 623.16,
      close: 623.62,
      high: 625.16,
      low: 621.8,
      yearHigh: 626.87,
      yearLow: 481.8,
      bid: currentPrice.value - 0.05,
      ask: currentPrice.value + 0.05,
      bidSize: 400,
      askSize: 300,
      ivRank: 13.2,
      ivIndex: 17.5,
      volume: 51500000,
      marketCap: null,
      peRatio: null,
      eps: null,
      earnings: null,
      dividend: 1.76,
      dividendYield: 1.13,
      correlation: 1.0,
      liquidity: 4,
    }));

    const processPendingOrder = async (order) => {
      if (!order) return;

      clearAllSelections();

      const { legs, isOpposite } = order;
      if (!legs || legs.length === 0) return;

      // 1. Expand Expiration Dates
      const expirationDatesToExpand = new Set(
        legs.map((leg) => {
          const parsed = parseOptionSymbol(leg.symbol);
          return parsed && parsed.expiry_date ? parsed.expiry_date.substring(0, 10) : null;
        }).filter(Boolean) // Remove null values
      );

      expirationDatesToExpand.forEach((expiryDate) => {
        if (expiryDate) {
          const expirationEntry = optionsManager.expirationDates.value.find(
            (e) => e.date === expiryDate
          );
          if (expirationEntry) {
            const uniqueKey = `${expirationEntry.date}-${expirationEntry.type}-${expirationEntry.symbol}`;
            optionsManager.expandExpiration(uniqueKey, expirationEntry);
          }
        }
      });

      // 2. Select the Legs
      legs.forEach((leg) => {
        const parsed = parseOptionSymbol(leg.symbol);
        if (parsed) {
          let side = leg.side.includes("buy") ? "buy" : "sell";
          if (isOpposite) {
            side = side === "buy" ? "sell" : "buy";
          }

          const selection = {
            symbol: leg.symbol,
            strike_price: parsed.strike_price,
            type: parsed.option_type,
            expiry: parsed.expiry_date,
            side: side,
            quantity: Math.abs(leg.qty),
          };
          addLeg(selection);
        }
      });
    };


    // Mobile navigation handler
    const onMobileNavigation = (navItem) => {
      console.log("Mobile navigation:", navItem);
      // Navigation is handled by the MobileNavDrawer component
      // The drawer will close automatically after navigation
    };

    // Mobile search handler
    const onOpenMobileSearch = () => {
      // This method is called when the search icon in SymbolHeader is clicked on mobile
      // We need to trigger the same mobile search overlay that TopBar uses
      // Find the TopBar component's mobile search method and call it
      // For now, we'll dispatch a custom event that TopBar can listen to
      window.dispatchEvent(new CustomEvent('open-mobile-search'));
    };

    // Mobile overlay methods
    const onMobileSectionSelected = (sectionKey) => {
      activeMobileSection.value = sectionKey;
      showMobileOverlay.value = true;
    };

    const closeMobileOverlay = () => {
      showMobileOverlay.value = false;
    };

    const onMobileTogglePositionCheck = (positionId) => {
      if (mobileCheckedPositions.value.has(positionId)) {
        mobileCheckedPositions.value.delete(positionId);
      } else {
        mobileCheckedPositions.value.add(positionId);
      }
      // CRITICAL FIX: Use the same event-driven pattern as desktop RightPanel
      // This ensures mobile and desktop use identical chart update logic
      const checkedPositionsList = allMobilePositions.value.filter((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      );
      onPositionsChanged(checkedPositionsList);
    };

    const onMobileToggleSelectAll = () => {
      if (isMobileAllSelected.value) {
        // Uncheck all
        mobileCheckedPositions.value.clear();
      } else {
        // Check all
        allMobilePositions.value.forEach((pos) => {
          mobileCheckedPositions.value.add(pos.id);
        });
      }
      // CRITICAL FIX: Use the same event-driven pattern as desktop RightPanel
      // This ensures mobile and desktop use identical chart update logic
      const checkedPositionsList = allMobilePositions.value.filter((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      );
      onPositionsChanged(checkedPositionsList);
    };

    // Window resize handler for responsive chart
    const handleWindowResize = () => {
      windowHeight.value = window.innerHeight;
    };

    // Mobile trading panel height management
    const mobileTradingPanelStyles = computed(() => {
      if (!isMobile.value) return {};
      
      let panelHeight = 0;
      if (selectedTradeMode.value === 'options' && showBottomPanel.value) {
        // BottomTradingPanel height: ~280px (stats + controls + content)
        panelHeight = 280;
      } else if (selectedTradeMode.value === 'shares' && showSharesPanel.value) {
        // SharesTradingPanel height: ~160px (simpler layout)
        panelHeight = 160;
      }
      
      return {
        '--mobile-trading-panel-height': `${panelHeight}px`
      };
    });

    // Listen for system recovery events to refresh data
    const handleSystemRecovery = async (event) => {
      try {
        // Only refresh data if user is authenticated
        if (!authService.isAuthenticated()) {
          console.log('🔒 Skipping system recovery data refresh - user not authenticated');
          return;
        }
        
        console.log('🎉 System recovery detected, refreshing options data');
        
        // Refresh all critical data after recovery
        await fetchSymbolData(currentSymbol.value);
        await fetchExpirationDates(currentSymbol.value);
        
        // If we had a selected expiry, refresh its options chain
        if (selectedExpiry.value) {
        }
        
        // Refresh options manager data
        await optionsManager.refreshAllData();
        
        // With API approach, clear cache to force fresh data on next access
        if (smartMarketDataStore.isServicesRunning() && currentSymbol.value) {
          console.log(`🔄 Clearing IVx cache for ${currentSymbol.value} after recovery`);
          smartMarketDataStore.clearCacheData(`ivxData.${currentSymbol.value}`);
        }
      } catch (error) {
        console.error('❌ Error refreshing OptionsTrading data after recovery:', error);
      }
    };

    // Lifecycle hooks
    onMounted(async () => {
      // Check if user is authenticated before making any API calls
      if (!authService.isAuthenticated()) {
        console.log('🔒 Skipping OptionsTrading data fetch - user not authenticated');
        return;
      }
      
      // The SmartMarketDataStore is already connected via main.js.
      // We just need to fetch the initial data for the default symbol.
      await fetchSymbolData(currentSymbol.value);
      await fetchExpirationDates(currentSymbol.value);
      if (selectedExpiry.value) {
        onExpiryChange();
      }

      // Set up centralized symbol selection listener
      const cleanup = setupSymbolSelectionListener();

      window.addEventListener("websocket-recovered", handleSystemRecovery);
      
      // CRITICAL: Listen for when services become available and force trigger IVx
      // With API approach, IVx data is automatically fetched when needed
      // No manual triggering required
      const checkServicesAndTriggerIvx = () => {
        return true; // Always return true as API handles fetching automatically
      };

      // Check immediately in case services are already running
      checkServicesAndTriggerIvx();

      // Also check periodically for the first few seconds after mount, but stop when successful
      let checkCount = 0;
      const maxChecks = 10; // Stop after 10 seconds
      const serviceCheckInterval = setInterval(() => {
        checkCount++;
        const triggered = checkServicesAndTriggerIvx();
        
        // Stop checking if we successfully triggered or exceeded max checks
        if (triggered || checkCount >= maxChecks) {
          clearInterval(serviceCheckInterval);
          if (checkCount >= maxChecks) {
            console.log(`⏰ Stopped IVx checking after ${maxChecks} attempts`);
          }
        }
      }, 1000);
      
      // Add window resize listener for responsive chart
      window.addEventListener('resize', handleWindowResize);
    });

    onUnmounted(() => {
      const cleanup = setupSymbolSelectionListener();
      cleanup();
      
      // Remove window resize listener
      window.removeEventListener('resize', handleWindowResize);
      // No need to remove the websocket-recovered event listener here as it's tied to the component's lifecycle
    });

    // Watchers

    // This single watcher now handles all chart updates.
    // It reacts to changes in selected legs, checked positions, price, and adjusted credit.
    watch(
      pendingOrder,
      async (newOrder) => {
        if (newOrder) {
          // Ensure expiration dates are loaded
          if (optionsManager.expirationDates.value.length === 0) {
            await optionsManager.loadExpirationDates();
          }
          await processPendingOrder(newOrder);
          clearPendingOrder();
        }
      },
      { immediate: true }
    );

    // Track previous leg count to detect changes
    const previousLegCount = ref(0);

    watch(
      [selectedLegs, activePositions, currentPrice, adjustedNetCredit],
      () => {
        // Reset adjustedNetCredit when number of legs changes
        const currentLegCount = selectedLegs.value.length;
        if (currentLegCount !== previousLegCount.value) {
          adjustedNetCredit.value = null; // Reset to use natural mid price
          previousLegCount.value = currentLegCount;
        }

        // PRIORITY SYSTEM for chart data:
        // 1. If user has explicitly checked positions in RightPanel, use those (activePositions)
        // 2. If no explicit position selections but we have selectedLegs, use those for chart
        // This ensures chart updates immediately when quantities change in BottomTradingPanel
        
        let positionsForChart = [];
        
        if (activePositions.value.length > 0) {
          // User has made explicit checkbox selections in RightPanel
          // Sync quantities for selected (non-existing) legs with the latest store values
          const legsMap = new Map(selectedLegs.value.map(leg => [leg.symbol, leg]));
          positionsForChart = activePositions.value.map((pos) => {
            if (!pos.isExisting && pos.asset_class === "us_option" && legsMap.has(pos.symbol)) {
              const leg = legsMap.get(pos.symbol);
              return {
                ...pos,
                // Ensure qty reflects current leg quantity and side
                qty: leg.side === "buy" ? leg.quantity : -leg.quantity,
                // Keep pricing in sync where possible (fallback to existing pos)
                current_price: leg.current_price || ((leg.bid + leg.ask) / 2) || pos.current_price || 0,
                avg_entry_price: leg.avg_entry_price ?? pos.avg_entry_price,
              };
            }
            return pos;
          });
        } else if (selectedLegs.value.length > 0) {
          // No explicit selections, but we have legs from options chain - convert for chart
          positionsForChart = selectedLegs.value.map((leg, index) => ({
            id: `selected:${leg.symbol}:${index}`,
            symbol: leg.symbol,
            asset_class: "us_option",
            qty: leg.side === "buy" ? leg.quantity : -leg.quantity,
            strike_price: leg.strike_price,
            option_type: leg.type,
            expiry_date: leg.expiry,
            current_price: leg.current_price || ((leg.bid + leg.ask) / 2) || 0,
            avg_entry_price: leg.avg_entry_price,
            unrealized_pl: 0,
            isExisting: false,
            isSelected: true,
          }));
        }
        
        updateChartData(positionsForChart);
        
        const hasLegsForChart = positionsForChart.length > 0;
        const hasLegsForTicket = selectedLegs.value.length > 0;

        showBottomPanel.value = hasLegsForTicket;
        
        if (hasLegsForChart) {
          isRightPanelExpanded.value = true;
        } else if (selectedLegs.value.length > 0) {
          // Expand panel when options are selected (so user can see position details)
          isRightPanelExpanded.value = true;
        } else {
          isRightPanelExpanded.value = false;
        }
      },
      { deep: true, immediate: true }
    );

    watch(selectedExpiry, (newExpiry, oldExpiry) => {
      if (newExpiry && newExpiry !== oldExpiry) {
        onExpiryChange();
      }
    });

    // Watch for changes in allMobilePositions and auto-check newly selected positions
    // This mimics the desktop RightPanel behavior
    watch(
      allMobilePositions,
      (newPositions, oldPositions) => {
        if (!newPositions || newPositions.length === 0) {
          mobileCheckedPositions.value.clear();
          return;
        }

        // Create sets for efficient comparison
        const oldIds = new Set((oldPositions || []).map(pos => pos.id));
        const newIds = new Set(newPositions.map(pos => pos.id));

        let hasNewPositions = false;
        let hasRemovedPositions = false;

        // Check for new positions and auto-select them if they're marked as selected
        newPositions.forEach((pos) => {
          if (!oldIds.has(pos.id) && pos.isSelected && !pos.isExisting) {
            mobileCheckedPositions.value.add(pos.id);
            hasNewPositions = true;
          }
        });

        // Remove positions that no longer exist
        (oldPositions || []).forEach((pos) => {
          if (!newIds.has(pos.id) && mobileCheckedPositions.value.has(pos.id)) {
            mobileCheckedPositions.value.delete(pos.id);
            hasRemovedPositions = true;
          }
        });

        // Also ensure all selected positions are checked (for consistency)
        newPositions.forEach((pos) => {
          if (pos.isSelected && !pos.isExisting) {
            mobileCheckedPositions.value.add(pos.id);
            hasNewPositions = true;
          }
        });

        // Update chart if there were changes
        if (hasNewPositions || hasRemovedPositions) {
          const checkedPositionsList = newPositions.filter((pos) =>
            mobileCheckedPositions.value.has(pos.id)
          );
          updateChartData(checkedPositionsList);
        }
      },
      { deep: true, immediate: true }
    );


    // Watch for time and update marketStatus accordingly (use US/Eastern time)
    function updateMarketStatusNow() {
      const now = DateTime.now().setZone("America/New_York");
      const day = now.weekday; // 1=Monday, 7=Sunday
      const hour = now.hour;
      const minute = now.minute;
      // US market open: Mon-Fri, 9:30am-4:00pm ET
      const isWeekday = day >= 1 && day <= 5;
      const isOpen = isWeekday && (
        (hour > 9 || (hour === 9 && minute >= 30)) &&
        (hour < 16)
      );
      updateMarketStatus(isOpen ? "Market Open" : "Market Closed");
    }
    updateMarketStatusNow(); // run immediately on load
    const marketStatusInterval = setInterval(updateMarketStatusNow, 60000); // check every minute

    // Store cleanup for unmount
    const cleanupOnUnmounted = () => {
      clearInterval(marketStatusInterval);
      if (optionsManager) {
        optionsManager.clearAllData();
      }
    };

    onUnmounted(() => {
      cleanupOnUnmounted();
    });

    return {
      // Reactive data
      currentSymbol,
      companyName,
      exchange,
      currentPrice,
      priceChange,
      priceChangePercent,
      isLivePrice,
      marketStatus,
      selectedExpiry,
      expirationDates,
      optionsChainData,
      selectedLegs,
      chartData,
      selectedTradeMode,
      tradeModes,
      isRightPanelExpanded,
      showBottomPanel,
      showSharesPanel,

      // Mobile detection
      isMobile,
      isTablet,
      isDesktop,
      showMobileNav,

      // Mobile overlay state
      showMobileOverlay,
      activeMobileSection,
      mobileCheckedPositions,
      allMobilePositions,
      isMobileAllSelected,
      isMobileIndeterminate,

      // Mobile methods
      onMobileSectionSelected,
      closeMobileOverlay,
      onMobileTogglePositionCheck,
      onMobileToggleSelectAll,

      // Options Manager
      optionsManager,

      // Computed
      priceChangeClass,
      marketStatusClass,
      shouldShowLoading,
      estimatedCost,
      rightPanelSection,
      livePrice,
      chartHeight,

      // Methods
      onExpiryChange,
      onOptionSelected,
      onOptionDeselected,
      clearAllSelections,
      onOrderPlaced,
      toggleRightPanel,
      onReviewAndSend,
      handleOrderEdit,
      onSymbolSelected,
      onTradeModeChanged,
      onSharesReviewAndSend,
      onSharesClearTrade,
      handleClearTradeReset,
      resetSharesPanelTrigger,
      onRightPanelCollapsed,
      onPositionsChanged,
      onExpirationExpanded,
      onExpirationCollapsed,
      onStrikeCountChanged,
      onPriceAdjusted,
      adjustedNetCredit,
      additionalQuoteData,
      onMobileNavigation,
      onOpenMobileSearch,

      // Mobile trading panel height management
      mobileTradingPanelStyles,

      // New props for CollapsibleOptionsChain
      optionsDataByExpiration,
      optionsChainLoading,
      optionsChainError,

      ivxData,

      // Order management
      showOrderConfirmation,
      showOrderResult,
      orderData,
      orderResult,
      isPlacingOrder,
      handleOrderConfirmation,
      handleOrderCancellation,
      handleOrderResultClose,
    };
  },
};
</script>

<style scoped>
.options-trading-container {
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
}

.symbol-search-section {
  min-width: 300px;
  margin-right: 24px;
}

.symbol-info {
  display: flex;
  align-items: center;
  gap: 24px;
}

.symbol-details h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #ffffff;
}

.symbol-meta {
  display: flex;
  gap: 12px;
  margin-top: 4px;
}

.company-name {
  color: #cccccc;
  font-size: 14px;
}

.exchange {
  color: #888888;
  font-size: 14px;
}

.price-info {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.current-price {
  display: flex;
  align-items: center;
  gap: 8px;
}

.price {
  font-size: 20px;
  font-weight: 600;
  color: #ffffff;
}

.change,
.change-percent {
  font-size: 14px;
  font-weight: 500;
}

.change.positive,
.change-percent.positive {
  color: #00c851;
}

.change.negative,
.change-percent.negative {
  color: #ff4444;
}

.change.neutral,
.change-percent.neutral {
  color: #cccccc;
}

.market-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-indicator.open {
  background-color: #00c851;
}

.status-indicator.closed {
  background-color: #ff4444;
}

.status-indicator.pre-market,
.status-indicator.after-hours {
  background-color: #ffbb33;
}

.status-text {
  font-size: 12px;
  color: #cccccc;
}

.trade-mode-selector {
  display: flex;
  align-items: center;
}

.mode-tabs {
  display: flex;
  background-color: #444444;
  border-radius: 6px;
  overflow: hidden;
}

.mode-tab {
  padding: 8px 16px;
  background: none;
  border: none;
  color: #cccccc;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mode-tab:hover {
  background-color: #555555;
  color: #ffffff;
}

.mode-tab.active {
  background-color: #007bff;
  color: #ffffff;
}

.options-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.shares-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chart-wrapper {
  flex: 1;
  padding: 16px 24px;
  overflow: hidden;
}

.expiration-selector {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  background-color: #141519;
  margin-bottom: 0;
}

.expiration-selector label {
  font-size: 14px;
  font-weight: 500;
  color: #cccccc;
}

.expiry-dropdown {
  min-width: 150px;
}

.options-chain-wrapper {
  flex: 1;
  overflow: hidden;
  padding: 16px 24px;
}

.collapsed-menu {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 12px;
  gap: 16px;
}

.menu-item {
  font-size: 18px;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: background-color 0.2s ease;
  color: #cccccc;
}

.menu-item:hover {
  background-color: #1a1d23;
}

.expanded-content {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #1a1d23;
  border-bottom: 1px solid #2a2d33;
  flex-shrink: 0;
}

.panel-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #ffffff;
}

.expand-toggle-btn {
  background: #444444;
  border: 1px solid #555555;
  color: #ffffff;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  font-size: 16px;
  font-weight: bold;
}

.expand-toggle-btn:hover {
  background-color: #007bff;
  border-color: #007bff;
  color: #ffffff;
  transform: scale(1.05);
}

.expand-toggle-btn:active {
  transform: scale(0.95);
}

.panel-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

.panel-content::-webkit-scrollbar {
  width: 8px;
}

.panel-content::-webkit-scrollbar-track {
  background: #2a2a2a;
  border-radius: 4px;
}

.panel-content::-webkit-scrollbar-thumb {
  background: #555555;
  border-radius: 4px;
}

.panel-content::-webkit-scrollbar-thumb:hover {
  background: #666666;
}

.chart-section {
  padding: 16px;
  border-bottom: 1px solid #444444;
}

.order-ticket-section {
  padding: 16px;
  border-bottom: 1px solid #444444;
}

.position-details-section {
  padding: 16px;
  flex: 1;
}

.position-card {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

.position-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.summary-item .label {
  color: #cccccc;
  font-size: 14px;
}

.summary-item .value {
  font-weight: 600;
  font-size: 14px;
}

.summary-item .value.credit {
  color: #00c851;
}

.summary-item .value.debit {
  color: #ff4444;
}


/* Mobile layout adjustments */
.content-area.mobile-layout {
  padding-left: 0;
  margin-left: 0;
}

.content-area.mobile-layout .options-chain-wrapper {
  padding: 8px 12px;
}

.content-area.mobile-layout .shares-section {
  padding: 8px 12px;
}

/* Mobile: Adjust content area height when trading panels are visible */
@media (max-width: 768px) {
  .content-area {
    /* Reserve space for mobile bottom button bar (80px) + trading panel when visible */
    padding-bottom: calc(80px + var(--mobile-trading-panel-height, 0px));
  }
  
  .options-chain-wrapper {
    /* Ensure options chain accounts for trading panel space */
    max-height: calc(100vh - 140px - var(--mobile-trading-panel-height, 0px)); /* 140px = TopBar + SymbolHeader */
    /* Add bottom padding to prevent content from being hidden behind trading panel */
    padding-bottom: calc(80px + var(--mobile-trading-panel-height, 0px));
  }
  
  .shares-section {
    /* Ensure shares section accounts for trading panel space */
    max-height: calc(100vh - 140px - var(--mobile-trading-panel-height, 0px)); /* 140px = TopBar + SymbolHeader */
    /* Add bottom padding to prevent content from being hidden behind trading panel */
    padding-bottom: calc(80px + var(--mobile-trading-panel-height, 0px));
  }
}

/* Hide right panel on mobile - will be replaced with tabs */
@media (max-width: 768px) {
  .main-layout {
    flex-direction: column;
  }
  
  .content-area {
    margin-right: 0 !important;
  }
  
  /* Hide desktop right panel on mobile */
  .right-panel {
    display: none;
  }
  
  /* Adjust options chain for mobile */
  .options-chain-wrapper {
    padding: 8px !important;
  }
  
  /* Mobile-optimized trading panels */
  .options-section,
  .shares-section {
    padding: 8px;
  }
  
  /* Ensure mobile nav toggle is always visible */
  .mobile-nav-toggle {
    display: flex !important;
  }
}

/* Tablet adjustments */
@media (max-width: 1024px) and (min-width: 769px) {
  .content-area {
    margin-right: 0;
  }
  
  .right-panel {
    width: 400px;
  }
  
  .options-chain-wrapper {
    padding: 12px 16px;
  }
}

/* Touch-friendly adjustments */
@media (hover: none) and (pointer: coarse) {
  .mobile-nav-toggle {
    min-height: 48px;
    min-width: 48px;
  }
  
  /* Increase touch targets for mobile */
  .nav-item,
  .tab-item,
  .option-data,
  .price-cell {
    min-height: 44px;
  }
}

/* Dark theme overrides for PrimeVue components */
:deep(.p-dropdown) {
  background-color: #1a1d23;
  border: 1px solid #2a2d33;
  color: #ffffff;
}

:deep(.p-dropdown:not(.p-disabled):hover) {
  border-color: #3a3d43;
}

:deep(.p-dropdown-panel) {
  background-color: #1a1d23;
  border: 1px solid #2a2d33;
}

:deep(.p-dropdown-item) {
  color: #ffffff;
}

:deep(.p-dropdown-item:not(.p-highlight):not(.p-disabled):hover) {
  background-color: #2a2d33;
}

:deep(.p-card) {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

:deep(.p-card .p-card-title) {
  color: #ffffff;
}

:deep(.p-card .p-card-content) {
  color: #cccccc;
}
</style>
