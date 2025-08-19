<template>
  <div class="activity-section">
    <!-- Filters Header -->
    <div class="activity-filters">
      <div class="filter-row">
        <div class="filter-button">
          <i class="pi pi-filter"></i>
          <span>Filter</span>
        </div>
        <div class="symbol-filters">
          <button
            class="symbol-filter-btn"
            :class="{ active: selectedSymbolFilter === currentSymbol }"
            @click="selectedSymbolFilter = currentSymbol"
          >
            {{ currentSymbol }}
          </button>
          <button
            class="symbol-filter-btn"
            :class="{ active: selectedSymbolFilter === 'All' }"
            @click="selectedSymbolFilter = 'All'"
          >
            All
          </button>
        </div>
      </div>

      <!-- Date Range -->
      <div class="date-range">
        <span class="date-text">{{ formatDateRange() }}</span>
        <i class="pi pi-calendar"></i>
      </div>

      <!-- Status Tabs -->
      <div class="status-tabs">
        <button
          class="status-tab"
          :class="{ active: selectedStatus === 'working' }"
          @click="selectedStatus = 'working'"
        >
          Working
        </button>
        <button
          class="status-tab"
          :class="{ active: selectedStatus === 'filled' }"
          @click="selectedStatus = 'filled'"
        >
          Filled
        </button>
        <button
          class="status-tab"
          :class="{ active: selectedStatus === 'canceled' }"
          @click="selectedStatus = 'canceled'"
        >
          Canceled
        </button>
        <button
          class="status-tab"
          :class="{ active: selectedStatus === 'all' }"
          @click="selectedStatus = 'all'"
        >
          All
        </button>
      </div>
    </div>

    <!-- Orders List -->
    <div class="orders-list">
      <div v-if="loading" class="loading-message">Loading orders...</div>
      <div v-else-if="filteredOrders.length === 0" class="no-orders">
        No orders to show
      </div>
      <div v-else>
        <div
          v-for="order in filteredOrders"
          :key="order.id"
          class="order-group"
          @contextmenu.prevent="showContextMenu($event, order)"
        >
          <!-- Order Header -->
          <div class="order-header">
            <div class="order-symbol">{{ getOrderSymbol(order) }}</div>
            <div class="order-info">
              <i class="pi pi-shield order-icon"></i>
              <span class="order-type">{{ formatOrderType(order) }}</span>
              <span class="order-id">#{{ order.id }}</span>
              <span class="order-time">{{
                formatOrderTime(order.submitted_at)
              }}</span>
            </div>
          </div>

          <!-- Order Summary -->
          <div class="order-summary">
            <div class="fill-info">
              <span class="fill-label">{{ order.status.toUpperCase() }}</span>
              <span class="fill-price">{{ formatFillPrice(order) }}</span>
              <span class="fill-type" :class="getFillTypeClass(order)">{{
                getFillType(order)
              }}</span>
            </div>
            <div class="order-value">
              <span class="value-label">LIMIT</span>
              <span class="value-amount">{{ formatOrderPrice(order) }}</span>
              <span class="value-type" :class="getFillTypeClass(order)">{{
                getFillType(order)
              }}</span>
            </div>
          </div>

          <!-- Order Legs -->
          <div class="order-legs">
            <div
              v-if="order.legs && order.legs.length > 0"
              v-for="leg in order.legs"
              :key="leg.symbol"
              class="order-leg"
            >
              <div class="leg-qty">{{ leg.qty > 0 ? leg.qty : leg.qty }}</div>
              <div class="leg-expiry">{{ formatLegExpiry(leg.symbol) }}</div>
              <div class="leg-days">{{ formatLegDays(leg.symbol) }}</div>
              <div class="leg-strike">{{ formatLegStrike(leg.symbol) }}</div>
              <div class="leg-type" :class="getLegTypeClass(leg.symbol)">
                {{ formatLegType(leg.symbol) }}
              </div>
              <div class="leg-side" :class="getLegSideClass(leg.side)">
                {{ formatLegSide(leg.side) }}
              </div>
            </div>
            <div v-else-if="isOptionOrder(order)" class="order-leg">
              <!-- Single leg option order -->
              <div class="leg-qty">{{ order.qty }}</div>
              <div class="leg-expiry">{{ formatLegExpiry(order.symbol) }}</div>
              <div class="leg-days">{{ formatLegDays(order.symbol) }}</div>
              <div class="leg-strike">{{ formatLegStrike(order.symbol) }}</div>
              <div class="leg-type" :class="getLegTypeClass(order.symbol)">
                {{ formatLegType(order.symbol) }}
              </div>
              <div class="leg-side" :class="getLegSideClass(order.side)">
                {{ formatLegSide(order.side) }}
              </div>
            </div>
            <div v-else class="order-leg stock-leg">
              <!-- Stock order -->
              <div class="leg-qty">{{ order.qty }}</div>
              <div class="leg-symbol">{{ order.symbol }}</div>
              <div class="leg-side" :class="getLegSideClass(order.side)">
                {{ formatLegSide(order.side) }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Context Menu -->
    <div
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{
        left: contextMenu.x + 'px',
        top: contextMenu.y + 'px',
      }"
      @click.stop
    >
      <div
        class="context-menu-item cancel-item"
        :class="{ disabled: !isOrderCancellable(contextMenu.order) }"
        @click="cancelOrder(contextMenu.order)"
      >
        <i class="pi pi-times"></i>
        <span>Cancel</span>
      </div>
      <div
        class="context-menu-item similar-item"
        :class="{ disabled: isOrderExpired(contextMenu.order) }"
        @click="createSimilarOrder(contextMenu.order)"
      >
        <i class="pi pi-arrow-down"></i>
        <span>Similar</span>
      </div>
      <div
        class="context-menu-item opposite-item"
        :class="{ disabled: isOrderExpired(contextMenu.order) }"
        @click="createOppositeOrder(contextMenu.order)"
      >
        <i class="pi pi-refresh"></i>
        <span>Opposite</span>
      </div>
    </div>

    <!-- Overlay to close context menu -->
    <div
      v-if="contextMenu.visible"
      class="context-menu-overlay"
      @click="hideContextMenu"
    ></div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch, reactive } from "vue";
import { useRouter } from "vue-router";
import { useMarketData } from "../composables/useMarketData.js";
import { useTradeNavigation } from "../composables/useTradeNavigation.js";
import { useSmartMarketData } from "../composables/useSmartMarketData.js";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";
import { detectStrategy } from "../utils/optionsStrategies";
import notificationService from "../services/notificationService";
import api from "../services/api"; // Keep for cancelOrder method

export default {
  name: "ActivitySection",
  props: {
    currentSymbol: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const router = useRouter();
    const { setPendingOrder } = useTradeNavigation();
    // Use unified market data composable
    const { getOrdersByStatus, isLoading } = useMarketData();

    // Component registration system for live pricing
    const componentId = `ActivitySection-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // Smart Market Data integration - same pattern as BottomTradingPanel
    const { getOptionPrice: getSmartOptionPrice } = useSmartMarketData();
    const liveOptionPrices = reactive(new Map());

    const orders = ref([]);
    const selectedSymbolFilter = ref(props.currentSymbol);
    const selectedStatus = ref("filled");

    // Get loading state from unified system
    const loading = computed(() => {
      const status =
        selectedStatus.value === "working" ? "pending" : selectedStatus.value;
      return isLoading(`orders.${status}`).value;
    });

    // Context menu state
    const contextMenu = ref({
      visible: false,
      x: 0,
      y: 0,
      order: null,
    });

    // Single registration method per component to prevent double registration
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    // Live price function - same pattern as BottomTradingPanel
    const getLivePrice = (symbol) => {
      if (!symbol) return null;

      if (!liveOptionPrices.has(symbol)) {
        // Ensure symbol is registered (only once per component)
        ensureSymbolRegistration(symbol);
        
        // Call getSmartOptionPrice only once to set up the subscription
        liveOptionPrices.set(symbol, getSmartOptionPrice(symbol));
      }
      return liveOptionPrices.get(symbol)?.value;
    };

    // Check if order is a working order (needs live pricing)
    const isWorkingOrder = (order) => {
      const status = order.status.toLowerCase();
      return [
        "new",
        "accepted",
        "pending_new",
        "partially_filled",
        "held",
        "pending",
        "unknown",
        "open",
        "submitted"
      ].includes(status);
    };

    // Calculate live combined order price (following BottomTradingPanel pattern)
    const calculateLiveOrderPrice = (order) => {
      if (!isWorkingOrder(order)) {
        return null; // Only calculate for working orders
      }

      // Get legs array (handle both multi-leg and single orders)
      const legs = order.legs && order.legs.length > 0 ? order.legs : [order];
      
      // Skip stock orders - only handle option orders
      if (!legs.some(leg => isOptionSymbol(leg.symbol))) {
        return null;
      }

      const total = legs.reduce((acc, leg) => {
        const livePrice = getLivePrice(leg.symbol);
        let midPrice = 0;
        
        if (livePrice && livePrice.bid > 0 && livePrice.ask > 0) {
          midPrice = (livePrice.bid + livePrice.ask) / 2;
        } else {
          // Fallback to static price if live price not available
          return acc;
        }
        
        // Apply side logic: buy = negative (you pay), sell = positive (you receive)
        const signedPrice = leg.side && leg.side.includes('buy') ? -midPrice : midPrice;
        return acc + (signedPrice * Math.abs(leg.qty || 0));
      }, 0);

      // Calculate per-contract price (same as BottomTradingPanel)
      const minQty = Math.min(...legs.map(leg => Math.abs(leg.qty || 1)));
      return total / minQty;
    };

    // Computed property for filtered orders
    const filteredOrders = computed(() => {
      let filtered = orders.value;

      // Filter by symbol
      if (selectedSymbolFilter.value !== "All") {
        const symbolGroup = getSymbolGroup(selectedSymbolFilter.value);
        filtered = filtered.filter((order) => {
          const orderSymbol = getOrderSymbol(order);
          return symbolGroup.includes(orderSymbol);
        });
      }

      // Filter by status
      filtered = filtered.filter((order) => {
        const status = order.status.toLowerCase();
        switch (selectedStatus.value) {
          case "working":
            return [
              "new",
              "accepted",
              "pending_new",
              "partially_filled",
              "held",
              "pending",
              "unknown",
              "open",
              "submitted"
            ].includes(status);
          case "filled":
            return status === "filled";
          case "canceled":
            return ["canceled", "cancelled", "expired", "rejected"].includes(
              status
            );
          case "other":
            return ![
              "new",
              "accepted",
              "pending_new",
              "partially_filled",
              "held",
              "filled",
              "canceled",
              "cancelled",
              "expired",
              "rejected",
            ].includes(status);
          default:
            return true;
        }
      });

      return filtered;
    });

    // Methods
    const fetchOrders = async (status = null) => {
      try {
        // Map UI status to API status
        let apiStatus = status || selectedStatus.value;
        switch (apiStatus) {
          case "working":
            apiStatus = "pending";
            break;
          case "canceled":
            apiStatus = "canceled";
            break;
          case "filled":
            apiStatus = "filled";
            break;
          case "all":
            apiStatus = "all";
            break;
          default:
            apiStatus = "all";
        }

        // Use unified system - always gets fresh data
        const response = await getOrdersByStatus(apiStatus);

        // Handle different response structures
        if (response && response.data && response.data.orders) {
          // Backend returns: { success: true, data: { orders: [...], total_orders: N } }
          orders.value = response.data.orders;
        } else if (response && response.orders) {
          // Direct orders array in response
          orders.value = response.orders;
        } else if (response && Array.isArray(response)) {
          // Response is directly an array
          orders.value = response;
        } else {
          console.warn("Unexpected orders response format:", response);
          orders.value = [];
        }

      } catch (error) {
        console.error("❌ Error fetching orders via unified system:", error);
        orders.value = [];
      }
    };

    const getOrderSymbol = (order) => {
      // For multi-leg orders, extract underlying symbol
      if (order.legs && order.legs.length > 0) {
        const firstLeg = order.legs[0];
        return (
          extractUnderlyingFromOptionSymbol(firstLeg.symbol) || firstLeg.symbol
        );
      }

      // For single orders, extract underlying if it's an option
      if (isOptionSymbol(order.symbol)) {
        return extractUnderlyingFromOptionSymbol(order.symbol) || order.symbol;
      }

      return order.symbol;
    };

    const formatOrderType = (order) => {
      if (order.legs && order.legs.length > 0) {
        // Use centralized strategy detection
        return detectStrategy(order.legs);
      }
      return order.order_type?.toUpperCase() || "MARKET";
    };

    const formatOrderTime = (timestamp) => {
      if (!timestamp) return "";
      const date = new Date(timestamp);
      return (
        date.toLocaleTimeString("en-US", {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
          hour12: false,
        }) + "a"
      );
    };

    const formatFillPrice = (order) => {
      // For Working orders, show live combined mid-price
      if (isWorkingOrder(order)) {
        const livePrice = calculateLiveOrderPrice(order);
        if (livePrice !== null) {
          return Math.abs(livePrice).toFixed(2);
        }
      }
      
      // For non-working orders, show static price (existing logic)
      if (order.avg_fill_price) {
        return Math.abs(order.avg_fill_price).toFixed(2);
      } else if (order.limit_price) {
        return Math.abs(order.limit_price).toFixed(2);
      }
      return "0.00";
    };

    const formatOrderPrice = (order) => {
      // Show original order price (what you asked for) - consistent positive format since CR indicates credit
      if (order.limit_price) {
        return Math.abs(order.limit_price).toFixed(2);
      } else if (order.avg_fill_price) {
        return Math.abs(order.avg_fill_price).toFixed(2);
      }
      return "0.00";
    };

    const formatOrderValue = (order) => {
      // Calculate total order value (price * quantity * multiplier)
      const price = order.avg_fill_price || order.limit_price || 0;
      const qty = Math.abs(order.qty || 0);

      if (order.legs && order.legs.length > 0) {
        // Multi-leg order: sum up all leg values
        let totalValue = 0;
        order.legs.forEach((leg) => {
          const legQty = Math.abs(leg.qty || 0);
          totalValue += price * legQty * 100; // Options multiplier is 100
        });
        return totalValue.toFixed(2);
      } else if (isOptionOrder(order)) {
        // Single option: price * qty * 100
        return (price * qty * 100).toFixed(2);
      } else {
        // Stock: price * qty
        return (price * qty).toFixed(2);
      }
    };

    const getOrderQuantityLabel = (order) => {
      // Return a meaningful label for the right side
      if (order.legs && order.legs.length > 0) {
        return "QTY"; // Multi-leg quantity
      }
      return "QTY"; // Single leg quantity
    };

    const getOrderQuantity = (order) => {
      // Return the total quantity for the order
      if (order.legs && order.legs.length > 0) {
        // For multi-leg orders, show the quantity of the first leg (they should be the same)
        return Math.abs(order.legs[0]?.qty || 0);
      }
      return Math.abs(order.qty || 0);
    };

    const getFillType = (order) => {
      // Simple and reliable: check the price sign
      const price = order.avg_fill_price || order.limit_price || 0;

      // Negative price = you receive money = Credit
      // Positive price = you pay money = Debit
      return price < 0 ? "CR" : "DB";
    };

    const getFillTypeClass = (order) => {
      const fillType = getFillType(order);
      return {
        "credit-type": fillType === "CR",
        "debit-type": fillType === "DB",
      };
    };

    const formatLegExpiry = (symbol) => {
      if (!isOptionSymbol(symbol)) return "";

      const parsed = parseOptionSymbol(symbol);
      if (parsed && parsed.expiry) {
        // Parse as UTC to avoid timezone issues (same fix as RightPanel.vue)
        const [year, month, day] = parsed.expiry.split("-").map(Number);
        const date = new Date(Date.UTC(year, month - 1, day));
        return date.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
          timeZone: "UTC",
        });
      }
      return "";
    };

    const formatLegDays = (symbol) => {
      if (!isOptionSymbol(symbol)) return "";

      const parsed = parseOptionSymbol(symbol);
      if (parsed && parsed.expiry) {
        // Parse as UTC to avoid timezone issues (same fix as RightPanel.vue)
        const [year, month, day] = parsed.expiry.split("-").map(Number);
        const expiryDate = new Date(Date.UTC(year, month - 1, day));
        const currentDate = new Date();
        currentDate.setHours(0, 0, 0, 0);
        const timeDiff = expiryDate.getTime() - currentDate.getTime();
        const diffDays = Math.ceil(timeDiff / (1000 * 60 * 60 * 24));
        return `${Math.max(0, diffDays)}d`;
      }
      return "";
    };

    const formatLegStrike = (symbol) => {
      if (!isOptionSymbol(symbol)) return "";

      const parsed = parseOptionSymbol(symbol);
      return parsed ? parsed.strike.toString() : "";
    };

    const formatLegType = (symbol) => {
      if (!isOptionSymbol(symbol)) return "";

      const parsed = parseOptionSymbol(symbol);
      return parsed ? parsed.type.toUpperCase() : "";
    };

    const formatLegSide = (side) => {
      if (side.includes("buy")) {
        return "BTO";
      } else if (side.includes("sell")) {
        return "STO";
      }
      return side.toUpperCase();
    };

    const getLegTypeClass = (symbol) => {
      const type = formatLegType(symbol);
      return {
        "call-type": type === "C",
        "put-type": type === "P",
      };
    };

    const getLegSideClass = (side) => {
      const formattedSide = formatLegSide(side);
      return {
        "buy-side": formattedSide === "BTO",
        "sell-side": formattedSide === "STO",
      };
    };

    const isOptionOrder = (order) => {
      return isOptionSymbol(order.symbol);
    };

    const isOptionSymbol = (symbol) => {
      return symbol && symbol.length > 10 && /[CP]\d{8}$/.test(symbol);
    };

    // Helper function to get symbol group (handles SPX/SPXW grouping)
    const getSymbolGroup = (symbol) => {
      if (symbol === "SPX" || symbol === "SPXW") {
        return ["SPX", "SPXW"];
      }
      return [symbol];
    };

    // Map weekly symbols back to their root symbols
    const mapToRootSymbol = (symbol) => {
      const weeklyMap = {
        'SPXW': 'SPX',
        'NDXP': 'NDX',
        'RUTW': 'RUT',
        'VIXW': 'VIX',
      };
      return weeklyMap[symbol] || symbol;
    };

    const extractUnderlyingFromOptionSymbol = (symbol) => {
      if (!isOptionSymbol(symbol)) return null;
      const match = symbol.match(/^([A-Z]+)/);
      return match ? match[1] : null;
    };

    const parseOptionSymbol = (symbol) => {
      if (!isOptionSymbol(symbol)) return null;

      try {
        const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
        if (!match) return null;

        const [, underlying, dateStr, type, strikeStr] = match;

        // Parse date: YYMMDD -> YYYY-MM-DD
        const year = 2000 + parseInt(dateStr.substring(0, 2));
        const month = dateStr.substring(2, 4);
        const day = dateStr.substring(4, 6);
        const expiry = `${year}-${month}-${day}`;

        // Parse strike: 8 digits with 3 decimal places
        const strike = parseInt(strikeStr) / 1000;

        return {
          underlying,
          expiry,
          type: type.toLowerCase(),
          strike,
        };
      } catch (error) {
        console.error("Error parsing option symbol:", symbol, error);
        return null;
      }
    };

    const formatDateRange = () => {
      const today = new Date();
      const tomorrow = new Date(today);
      tomorrow.setDate(tomorrow.getDate() + 1);

      const formatDate = (date) => {
        return date.toLocaleDateString("en-US", {
          month: "numeric",
          day: "numeric",
          year: "numeric",
        });
      };

      return `${formatDate(today)} - ${formatDate(tomorrow)}`;
    };

    // Global context menu handler to prevent browser context menu
    const handleGlobalContextMenu = (event) => {
      if (contextMenu.value.visible) {
        event.preventDefault();
        return false;
      }
    };

    // Context menu methods
    const showContextMenu = (event, order) => {
      // Always prevent the default browser context menu
      event.preventDefault();

      const orderSymbol = getOrderSymbol(order);
      if (orderSymbol && orderSymbol !== props.currentSymbol) {
        // Map weekly symbols back to their root symbols for navigation
        const rootSymbol = mapToRootSymbol(orderSymbol);
        const symbolData = {
          symbol: rootSymbol,
          description: "", // Will be fetched by SymbolHeader
          exchange: "", // Will be fetched by SymbolHeader
        };
        window.dispatchEvent(
          new CustomEvent("symbol-selected", {
            detail: symbolData,
          })
        );
      }

      // If menu is already visible, hide it first
      if (contextMenu.value.visible) {
        contextMenu.value.visible = false;
        // Use setTimeout to ensure the menu is hidden before showing the new one
        setTimeout(() => {
          contextMenu.value = {
            visible: true,
            x: event.clientX,
            y: event.clientY,
            order: order,
          };
        }, 0);
      } else {
        contextMenu.value = {
          visible: true,
          x: event.clientX,
          y: event.clientY,
          order: order,
        };
      }
    };

    const hideContextMenu = () => {
      contextMenu.value.visible = false;
    };

    const isOrderExpired = (order) => {
      if (!order) {
        return true;
      }
      
      if (!order.legs || order.legs.length === 0) {
        if (isOptionOrder(order)) {
          const parsed = parseOptionSymbol(order.symbol);
          if (parsed) {
            // Parse as UTC to avoid timezone issues (same fix as other date functions)
            const [year, month, day] = parsed.expiry.split("-").map(Number);
            const expiryDate = new Date(Date.UTC(year, month - 1, day));
            // Use UTC for current date comparison to match expiry date
            const currentDate = new Date();
            const currentDateUTC = new Date(Date.UTC(currentDate.getFullYear(), currentDate.getMonth(), currentDate.getDate()));
            
            // Only consider expired if expiry date is BEFORE current date (not same day)
            if (expiryDate.getTime() < currentDateUTC.getTime()) {
              return true;
            }
          }
        }
        return false;
      }
      
      for (const leg of order.legs) {
        const parsed = parseOptionSymbol(leg.symbol);
        if (parsed) {
          // Parse as UTC to avoid timezone issues (same fix as other date functions)
          const [year, month, day] = parsed.expiry.split("-").map(Number);
          const expiryDate = new Date(Date.UTC(year, month - 1, day));
          // Use UTC for current date comparison to match expiry date
          const currentDate = new Date();
          const currentDateUTC = new Date(Date.UTC(currentDate.getFullYear(), currentDate.getMonth(), currentDate.getDate()));
          
          // Only consider expired if expiry date is BEFORE current date (not same day)
          if (expiryDate.getTime() < currentDateUTC.getTime()) {
            return true;
          }
        }
      }
      return false;
    };

    const isOrderCancellable = (order) => {
      if (!order) return false;
      const status = order.status.toLowerCase();
      return [
        "new",
        "accepted",
        "pending_new",
        "partially_filled",
        "held",
        "pending",
        "unknown",
        "open",
        "submitted",
        "pending_cancel",
      ].includes(status);
    };

    const cancelOrder = async (order) => {
      // Check if order is cancellable before proceeding
      if (!isOrderCancellable(order)) {
        hideContextMenu();
        return;
      }

      try {
        console.log("Cancelling order:", order.id);
        hideContextMenu();

        // Call the API to cancel the order
        const response = await api.cancelOrder(order.id);

        if (response.success) {
          console.log("Order cancelled successfully:", response);

          // Show success notification
          notificationService.showSuccess(
            `Order #${order.id} has been cancelled successfully`,
            "Order Cancelled"
          );

          // Refresh orders to show updated status
          await fetchOrders();
        } else {
          console.error("Order cancellation failed:", response);

          // Show error notification
          notificationService.showError(
            response.message || "Unknown error occurred",
            "Cancellation Failed"
          );
        }
      } catch (error) {
        console.error("Error cancelling order:", error);

        // Show user-friendly error notification
        let errorMessage = "Failed to cancel order";
        if (
          error.response &&
          error.response.data &&
          error.response.data.message
        ) {
          errorMessage = error.response.data.message;
        } else if (error.message) {
          errorMessage = error.message;
        }

        notificationService.showError(errorMessage, "Cancellation Error");
      } finally {
        hideContextMenu();
      }
    };

    const createSimilarOrder = (order) => {
      if (isOrderExpired(order)) return;
      setPendingOrder({ ...order, isOpposite: false });
      router.push("/");
      hideContextMenu();
    };

    const createOppositeOrder = (order) => {
      if (isOrderExpired(order)) return;
      setPendingOrder({ ...order, isOpposite: true });
      router.push("/");
      hideContextMenu();
    };

    const convertOrderToSelections = (order, isOpposite = false) => {
      const selections = [];

      if (order.legs && order.legs.length > 0) {
        // Multi-leg order
        order.legs.forEach((leg) => {
          const parsed = parseOptionSymbol(leg.symbol);
          if (parsed) {
            const originalSide = leg.side;
            let newSide = originalSide;

            if (isOpposite) {
              // Flip the side for opposite order
              if (originalSide.includes("buy")) {
                newSide = originalSide.replace("buy", "sell");
              } else if (originalSide.includes("sell")) {
                newSide = originalSide.replace("sell", "buy");
              }
            }

            selections.push({
              symbol: leg.symbol,
              strike_price: parsed.strike,
              type: parsed.type,
              expiry: parsed.expiry,
              side: newSide.includes("buy") ? "buy" : "sell",
              quantity: Math.abs(leg.qty),
            });
          }
        });
      } else if (isOptionOrder(order)) {
        // Single leg option order
        const parsed = parseOptionSymbol(order.symbol);
        if (parsed) {
          const originalSide = order.side;
          let newSide = originalSide;

          if (isOpposite) {
            // Flip the side for opposite order
            if (originalSide.includes("buy")) {
              newSide = originalSide.replace("buy", "sell");
            } else if (originalSide.includes("sell")) {
              newSide = originalSide.replace("sell", "buy");
            }
          }

          selections.push({
            symbol: order.symbol,
            strike_price: parsed.strike,
            type: parsed.type,
            expiry: parsed.expiry,
            side: newSide.includes("buy") ? "buy" : "sell",
            quantity: Math.abs(order.qty),
          });
        }
      }

      return selections;
    };


    // Watch for symbol changes
    watch(
      () => props.currentSymbol,
      (newSymbol) => {
        selectedSymbolFilter.value = newSymbol;
      }
    );

    // Component cleanup system
    const cleanupComponentRegistrations = () => {
      // Unregister all symbols this component was using
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      
      // Clear local tracking
      registeredSymbols.clear();
      liveOptionPrices.clear();
    };

    // Watch for symbol changes to clean up old registrations
    watch(
      () => props.currentSymbol,
      (newSymbol, oldSymbol) => {
        if (newSymbol !== oldSymbol) {
          // Clean up old registrations when symbol changes
          cleanupComponentRegistrations();
        }
        selectedSymbolFilter.value = newSymbol;
      }
    );

    // Watch for status changes and manage subscriptions
    watch(
      () => selectedStatus.value,
      (newStatus, oldStatus) => {
        // Clean up subscriptions when switching away from working orders
        if (oldStatus === 'working' && newStatus !== 'working') {
          cleanupComponentRegistrations();
        }
        
        fetchOrders(newStatus);
      }
    );

    // Watch filtered orders to manage subscriptions for working orders
    watch(
      filteredOrders,
      (newOrders) => {
        // Only manage subscriptions for working orders
        if (selectedStatus.value === 'working') {
          // Extract all option symbols from working orders
          const workingSymbols = new Set();
          
          newOrders.forEach(order => {
            if (isWorkingOrder(order)) {
              // Get legs array (handle both multi-leg and single orders)
              const legs = order.legs && order.legs.length > 0 ? order.legs : [order];
              
              legs.forEach(leg => {
                if (isOptionSymbol(leg.symbol)) {
                  workingSymbols.add(leg.symbol);
                }
              });
            }
          });
          
          // Register new symbols that aren't already registered
          workingSymbols.forEach(symbol => {
            if (!registeredSymbols.has(symbol)) {
              ensureSymbolRegistration(symbol);
            }
          });
          
          // Unregister symbols that are no longer needed
          const symbolsToRemove = [];
          registeredSymbols.forEach(symbol => {
            if (!workingSymbols.has(symbol)) {
              symbolsToRemove.push(symbol);
            }
          });
          
          symbolsToRemove.forEach(symbol => {
            smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
            registeredSymbols.delete(symbol);
            liveOptionPrices.delete(symbol);
          });
        }
      },
      { deep: true }
    );

    // Fetch orders on mount
    onMounted(() => {
      fetchOrders();
      // Add global context menu handler
      document.addEventListener("contextmenu", handleGlobalContextMenu);
    });

    // Clean up event listeners and subscriptions
    onUnmounted(() => {
      document.removeEventListener("contextmenu", handleGlobalContextMenu);
      // Clean up all component registrations
      cleanupComponentRegistrations();
    });

    return {
      orders,
      loading,
      selectedSymbolFilter,
      selectedStatus,
      filteredOrders,
      contextMenu,
      getOrderSymbol,
      formatOrderType,
      formatOrderTime,
      formatFillPrice,
      formatOrderPrice,
      formatOrderValue,
      getOrderQuantityLabel,
      getOrderQuantity,
      getFillType,
      getFillTypeClass,
      formatLegExpiry,
      formatLegDays,
      formatLegStrike,
      formatLegType,
      formatLegSide,
      getLegTypeClass,
      getLegSideClass,
      isOptionOrder,
      formatDateRange,
      showContextMenu,
      hideContextMenu,
      isOrderCancellable,
      cancelOrder,
      createSimilarOrder,
      createOppositeOrder,
      isOrderExpired,
    };
  },
};
</script>

<style scoped>
.activity-section {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.activity-filters {
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
  padding: var(--spacing-lg);
  flex-shrink: 0;
}

.filter-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
}

.filter-button {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
}

.symbol-filters {
  display: flex;
  gap: var(--spacing-sm);
}

.symbol-filter-btn {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: 1px solid var(--border-secondary);
  background-color: var(--bg-tertiary);
  color: var(--text-secondary);
  border-radius: 20px;
  cursor: pointer;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  transition: var(--transition-normal);
}

.symbol-filter-btn:hover {
  background-color: var(--bg-quaternary);
}

.symbol-filter-btn.active {
  background-color: var(--color-brand);
  color: var(--text-primary);
  border-color: var(--color-brand);
}

.date-range {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin-bottom: var(--spacing-md);
}

.status-tabs {
  display: flex;
  gap: var(--spacing-xs);
}

.status-tab {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  background-color: var(--bg-tertiary);
  color: var(--text-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  transition: var(--transition-normal);
}

.status-tab:hover {
  background-color: var(--bg-quaternary);
}

.status-tab.active {
  background-color: var(--color-brand);
  color: var(--text-primary);
}

.orders-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-lg);
}

.loading-message,
.no-orders {
  text-align: center;
  padding: var(--spacing-xxl) var(--spacing-lg);
  color: var(--text-tertiary);
  font-size: var(--font-size-md);
}

.order-group {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  margin-bottom: var(--spacing-lg);
  overflow: hidden;
}

.order-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-secondary);
}

.order-symbol {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.order-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-base);
  color: var(--text-secondary);
}

.order-icon {
  color: var(--color-info);
}

.order-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-secondary);
}

.fill-info,
.order-value {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.fill-label,
.value-label {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
}

.fill-price,
.value-amount {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  font-family: monospace;
}

.fill-type,
.value-type {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
}

/* Credit/Debit color styling */
.credit-type {
  color: var(--color-danger) !important; /* Red for credits */
}

.debit-type {
  color: var(--color-success) !important; /* Green for debits */
}

.order-legs {
  padding: var(--spacing-sm) 0;
}

.order-leg {
  display: grid;
  grid-template-columns: 40px 60px 40px 60px 30px 40px;
  gap: var(--spacing-sm);
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-lg);
  font-size: var(--font-size-base);
  border-bottom: 1px solid var(--border-secondary);
}

.order-leg:last-child {
  border-bottom: none;
}

.stock-leg {
  grid-template-columns: 40px 1fr 60px;
}

.leg-qty {
  text-align: center;
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.leg-expiry,
.leg-days,
.leg-strike {
  text-align: center;
  color: var(--text-secondary);
}

.leg-symbol {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.leg-type {
  text-align: center;
  font-weight: var(--font-weight-semibold);
  padding: 2px 4px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
}

.leg-type.call-type {
  background-color: var(--color-success);
  color: var(--text-primary);
}

.leg-type.put-type {
  background-color: var(--color-danger);
  color: var(--text-primary);
}

.leg-side {
  text-align: center;
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-xs);
}

.leg-side.buy-side {
  color: var(--color-success);
}

.leg-side.sell-side {
  color: var(--color-danger);
}

/* Context Menu Styles */
.context-menu-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 999;
}

.context-menu {
  position: fixed;
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  z-index: 1000;
  min-width: 120px;
  overflow: hidden;
}

.context-menu-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  cursor: pointer;
  font-size: var(--font-size-base);
  color: var(--text-primary);
  transition: var(--transition-fast);
  border-bottom: 1px solid var(--border-secondary);
}

.context-menu-item:last-child {
  border-bottom: none;
}

.context-menu-item:hover {
  background-color: var(--bg-quaternary);
}

.context-menu-item i {
  font-size: var(--font-size-sm);
  width: 16px;
  text-align: center;
}

.context-menu-item.cancel-item {
  color: var(--color-danger);
}

.context-menu-item.cancel-item:hover {
  background-color: rgba(255, 68, 68, 0.1);
}

.context-menu-item.cancel-item.disabled,
.context-menu-item.similar-item.disabled,
.context-menu-item.opposite-item.disabled {
  color: var(--text-quaternary);
  cursor: not-allowed;
  opacity: 0.5;
}

.context-menu-item.cancel-item.disabled:hover,
.context-menu-item.similar-item.disabled:hover,
.context-menu-item.opposite-item.disabled:hover {
  background-color: transparent;
  color: var(--text-quaternary);
}

.context-menu-item.similar-item i {
  color: var(--color-info);
}

.context-menu-item.opposite-item i {
  color: var(--color-warning);
}
</style>
