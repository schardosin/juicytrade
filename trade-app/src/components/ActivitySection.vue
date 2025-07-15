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
          :class="{ active: selectedStatus === 'other' }"
          @click="selectedStatus = 'other'"
        >
          Other
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
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from "vue";
import api from "../services/api";
import { detectStrategy } from "../utils/optionsStrategies";

export default {
  name: "ActivitySection",
  props: {
    currentSymbol: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const orders = ref([]);
    const loading = ref(false);
    const selectedSymbolFilter = ref(props.currentSymbol);
    const selectedStatus = ref("filled");

    // Computed property for filtered orders
    const filteredOrders = computed(() => {
      let filtered = orders.value;

      // Filter by symbol
      if (selectedSymbolFilter.value !== "All") {
        filtered = filtered.filter((order) => {
          const orderSymbol = getOrderSymbol(order);
          return orderSymbol === selectedSymbolFilter.value;
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
        loading.value = true;

        // Map UI status to API status
        let apiStatus = status || selectedStatus.value;
        switch (apiStatus) {
          case "working":
            apiStatus = "open";
            break;
          case "canceled":
            apiStatus = "canceled";
            break;
          case "filled":
            apiStatus = "filled";
            break;
          case "other":
            apiStatus = "all"; // Get all and filter on frontend for "other"
            break;
          default:
            apiStatus = "all";
        }

        console.log(
          `Fetching orders with status: ${apiStatus} (UI status: ${
            status || selectedStatus.value
          })`
        );
        const response = await api.getOrders(apiStatus);

        console.log("Orders API response:", response);

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

        console.log("Processed orders:", orders.value);
      } catch (error) {
        console.error("Error fetching orders:", error);
        orders.value = [];
      } finally {
        loading.value = false;
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
      // Show actual fill price (what you got) - consistent positive format since CR indicates credit
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
        const date = new Date(parsed.expiry);
        return date.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
        });
      }
      return "";
    };

    const formatLegDays = (symbol) => {
      if (!isOptionSymbol(symbol)) return "";

      const parsed = parseOptionSymbol(symbol);
      if (parsed && parsed.expiry) {
        const expiryDate = new Date(parsed.expiry);
        const today = new Date();
        const diffTime = expiryDate.getTime() - today.getTime();
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
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

    // Watch for symbol changes
    watch(
      () => props.currentSymbol,
      (newSymbol) => {
        selectedSymbolFilter.value = newSymbol;
      }
    );

    // Watch for status changes and refetch orders
    watch(
      () => selectedStatus.value,
      (newStatus) => {
        console.log(`Status changed to: ${newStatus}, refetching orders...`);
        fetchOrders(newStatus);
      }
    );

    // Fetch orders on mount
    onMounted(() => {
      fetchOrders();
    });

    return {
      orders,
      loading,
      selectedSymbolFilter,
      selectedStatus,
      filteredOrders,
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
  background-color: var(--color-info);
  color: var(--text-primary);
  border-color: var(--color-info);
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
  background-color: var(--color-info);
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
</style>
