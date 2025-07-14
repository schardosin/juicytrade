<template>
  <!-- Bottom Sheet Overlay -->
  <div v-if="visible" class="bottom-sheet-overlay" @click="handleCancel">
    <!-- Bottom Sheet Container -->
    <div class="bottom-sheet" :class="{ 'slide-up': visible }" @click.stop>
      <!-- Top Bar with Trade Info -->
      <div class="trade-header">
        <div class="trade-title">
          <span class="trade-icon">⚡</span>
          <span>Trade</span>
          <span class="quick-analysis">📊 Quick Analysis</span>
        </div>
        <div class="trade-actions">
          <button class="clear-trade-btn" @click="handleCancel">
            🗑️ Clear Trade
          </button>
        </div>
      </div>

      <!-- Trade Stats Bar -->
      <div class="trade-stats">
        <div class="stat-item">
          <span class="stat-label">POP</span>
          <span class="stat-value">59%</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">EXT</span>
          <span class="stat-value">{{
            formatCurrency(Math.abs(profitLossAnalysis.maxProfit), 0)
          }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">P50</span>
          <span class="stat-value">-</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">Delta</span>
          <span class="stat-value">{{ greeks.delta }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">Theta</span>
          <span class="stat-value negative">{{ greeks.theta }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">Max Profit</span>
          <span class="stat-value positive">{{
            formatCurrency(Math.abs(profitLossAnalysis.maxProfit), 2)
          }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">Max Loss</span>
          <span class="stat-value negative">{{
            formatCurrency(Math.abs(profitLossAnalysis.maxLoss), 2)
          }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">BP Eff.</span>
          <span class="stat-value"
            >{{ getBPEffect() }} <span class="stat-unit">db</span></span
          >
        </div>
      </div>

      <!-- Main Content -->
      <div class="sheet-content">
        <!-- Left Side - Account & Order Details -->
        <div class="left-section">
          <!-- Account Section -->
          <div class="account-section">
            <h3>Account</h3>
            <div class="account-name">
              {{
                orderData?.accountName ||
                "Joint Tenants with Rights of Survivorship"
              }}
            </div>
          </div>

          <!-- Order Details -->
          <div class="order-details-section">
            <h3>Order Details</h3>
            <div class="order-legs">
              <div
                v-for="(leg, index) in formattedLegs"
                :key="index"
                class="order-leg"
              >
                <div class="leg-quantity">
                  {{ leg.quantity > 0 ? leg.quantity : leg.quantity }}
                </div>
                <div class="leg-date">{{ leg.date }}</div>
                <div class="leg-details">{{ leg.strike }}</div>
                <div class="leg-type">{{ leg.type.charAt(0) }}</div>
                <div class="leg-action" :class="leg.action.toLowerCase()">
                  {{ leg.action.toUpperCase() }}
                </div>
                <div class="leg-price">${{ leg.priceValue }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- Right Side - Confirmation Details -->
        <div class="right-section">
          <!-- Confirm Title -->
          <div class="confirm-section">
            <h2 class="confirm-title">
              Confirm <span class="strategy-name">{{ getStrategyName() }}</span>
            </h2>

            <!-- Trade Details Grid -->
            <div class="trade-details-grid">
              <div class="detail-row">
                <span class="detail-label">Stock BP</span>
                <span class="detail-value"
                  >${{
                    orderData?.underlyingPrice?.toFixed(2) || "441.50"
                  }}</span
                >
                <span class="detail-label">Option BP</span>
                <span class="detail-value"
                  >${{
                    orderData?.underlyingPrice?.toFixed(2) || "441.50"
                  }}</span
                >
              </div>

              <div class="detail-row">
                <span class="detail-label">Type</span>
                <span class="detail-value">{{
                  orderData?.orderType || "Limit"
                }}</span>
                <span class="detail-label">{{ getCreditDebitLabel() }}</span>
                <span class="detail-value positive">{{ getNetCredit() }}</span>
              </div>

              <div class="detail-row">
                <span class="detail-label">Time in Force</span>
                <span class="detail-value">{{
                  orderData?.timeInForce || "Day"
                }}</span>
              </div>

              <div class="detail-row">
                <span class="detail-label">Estimated Trade Cost</span>
                <span class="detail-value">{{ getEstimatedCost() }}</span>
              </div>

              <div class="detail-row">
                <span class="detail-label">Comm. + Est. Fees</span>
                <span class="detail-value"
                  >2.00 + 1.56 <span class="detail-unit">db</span></span
                >
                <span class="detail-label">Estimated Total</span>
                <span class="detail-value">{{ getEstimatedTotal() }}</span>
              </div>

              <div class="detail-row">
                <span class="detail-label">Estimated BP Effect</span>
                <span class="detail-value"
                  >Reduced by
                  <span class="positive">${{ getBPEffect() }}</span></span
                >
              </div>
            </div>
          </div>

          <!-- Warning Message -->
          <div class="warning-message">
            ⚠️ Your order will begin working during next valid session.
          </div>
        </div>
      </div>

      <!-- Bottom Action Buttons -->
      <div class="action-buttons">
        <button class="edit-btn" @click="handleEdit">✏️ Edit</button>
        <button
          class="submit-btn"
          @click="handleConfirm"
          :disabled="!canConfirm || loading"
        >
          <span v-if="loading">⏳</span>
          <span v-else>📤</span>
          {{ loading ? "Submitting..." : "Submit" }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch } from "vue";
import {
  calculateMultiLegProfitLoss,
  calculateBuyingPowerEffect,
  formatCurrency,
  getCreditDebitInfo,
  calculateGreeks,
} from "../services/optionsCalculator.js";

export default {
  name: "OrderConfirmationDialog",
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
    orderData: {
      type: Object,
      default: null,
    },
    loading: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["hide", "confirm", "cancel", "edit", "clear-selections"],
  setup(props, { emit }) {
    const editableOrderPrice = ref(0);

    // Calculate comprehensive P&L analysis using centralized calculator
    const profitLossAnalysis = computed(() => {
      if (!props.orderData?.legs) {
        return {
          maxProfit: 0,
          maxLoss: 0,
          netPremium: 0,
          breakEvenPoints: [],
          currentPL: 0,
          positions: [],
        };
      }

      // Use the display limit price (always positive) for calculations
      const displayPrice =
        props.orderData?.displayLimitPrice ||
        Math.abs(
          props.orderData?.limitPrice || props.orderData?.netPremium || 0
        );

      // Use the original netPremium to determine if it's credit or debit
      const originalNetPremium = props.orderData?.netPremium || 0;
      const isCredit = originalNetPremium <= 0; // Negative netPremium = credit

      let maxProfit, maxLoss;

      if (isCredit) {
        // Credit spread: we receive premium
        maxProfit = displayPrice * 100; // Premium received × 100
        // For credit spread, max loss = (spread width - premium received)
        // Assuming $2 spread width based on typical iron condor/butterfly
        maxLoss = 200 - displayPrice * 100; // 200 - premium = max loss
      } else {
        // Debit spread: we pay premium
        maxLoss = displayPrice * 100; // Premium paid × 100
        maxProfit = 200 - displayPrice * 100; // Spread width - premium paid
      }

      const analysis = {
        maxProfit: maxProfit,
        maxLoss: maxLoss,
        netPremium: originalNetPremium,
        breakEvenPoints: [],
        currentPL: 0,
        positions: [],
      };

      return analysis;
    });

    // Calculate Greeks
    const greeks = computed(() => {
      return {
        delta: formatCurrency(props.orderData?.netDelta || 0, 2),
        theta: formatCurrency(props.orderData?.netTheta || 0, 2),
      };
    });

    // Format legs for display
    const formattedLegs = computed(() => {
      if (!props.orderData?.legs) return [];

      return props.orderData.legs.map((leg) => ({
        action: leg.side === "buy" ? "Buy" : "Sell",
        symbol: leg.displaySymbol || leg.symbol,
        date: leg.date || formatExpiry(props.orderData.expiry),
        type: leg.type || "Call",
        strike: leg.strike_price || leg.strike || "-",
        priceValue: (leg.price || 0).toFixed(2),
        quantity: leg.ratio_qty || leg.quantity || 1,
      }));
    });

    // Check if order can be confirmed
    const canConfirm = computed(() => {
      return props.orderData && !props.loading;
    });

    // Helper methods
    const getStrategyName = () => {
      if (!props.orderData?.legs) return "OPTION TRADE";

      const legs = props.orderData.legs;
      if (legs.length === 1) {
        return `1 ${
          props.orderData.symbol
        } ${legs[0].side?.toUpperCase()} ${legs[0].type?.toUpperCase()} VERTICAL`;
      }
      return `${legs.length} LEG STRATEGY`;
    };

    const getCreditDebitLabel = () => {
      // Use the display limit price for UI (always positive)
      const displayPrice =
        props.orderData?.displayLimitPrice ||
        Math.abs(
          props.orderData?.limitPrice || props.orderData?.netPremium || 0
        );
      const netPremium = props.orderData?.netPremium || 0;
      // Invert the logic: negative netPremium = credit, positive = debit
      return netPremium <= 0 ? "Net Credit @" : "Net Debit @";
    };

    const getNetCredit = () => {
      // Use the display limit price for UI (always positive)
      const displayPrice =
        props.orderData?.displayLimitPrice ||
        Math.abs(
          props.orderData?.limitPrice || props.orderData?.netPremium || 0
        );
      return formatCurrency(displayPrice);
    };

    const getEstimatedCost = () => {
      // Use the display limit price for UI (always positive)
      const displayPrice =
        props.orderData?.displayLimitPrice ||
        Math.abs(
          props.orderData?.limitPrice || props.orderData?.netPremium || 0
        );
      const netPremium = props.orderData?.netPremium || 0;
      // Invert the logic: negative netPremium = credit, positive = debit
      const label = netPremium <= 0 ? "cr" : "db";
      return `${formatCurrency(displayPrice)} ${label}`;
    };

    const getEstimatedTotal = () => {
      // Use the display limit price for UI (always positive)
      const displayPrice =
        props.orderData?.displayLimitPrice ||
        Math.abs(
          props.orderData?.limitPrice || props.orderData?.netPremium || 0
        );
      const netPremium = props.orderData?.netPremium || 0;
      // Invert the logic: negative netPremium = credit, positive = debit
      const label = netPremium <= 0 ? "cr" : "db";
      const fees = 3.56; // 2.00 + 1.56
      const total = displayPrice + fees;
      return `${formatCurrency(total)} ${label}`;
    };

    const getBPEffect = () => {
      // For BP effect, use the max loss from our analysis
      return formatCurrency(profitLossAnalysis.value.maxLoss);
    };

    // Format expiry date
    const formatExpiry = (expiry) => {
      if (!expiry) return "Jul 11";
      if (typeof expiry === "string") {
        const date = new Date(expiry);
        return date.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
        });
      }
      if (expiry instanceof Date) {
        return expiry.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
        });
      }
      return expiry.toString();
    };

    // Handle cancel
    const handleCancel = () => {
      emit("clear-selections"); // Clear all selected options
      emit("cancel");
      emit("hide");
    };

    // Handle edit
    const handleEdit = () => {
      emit("edit");
    };

    // Handle confirm
    const handleConfirm = () => {
      if (!canConfirm.value) return;

      const finalOrderData = {
        ...props.orderData,
        orderPrice: editableOrderPrice.value,
      };

      emit("confirm", finalOrderData);
    };

    return {
      editableOrderPrice,
      formattedLegs,
      canConfirm,
      profitLossAnalysis,
      greeks,
      getStrategyName,
      getCreditDebitLabel,
      getNetCredit,
      getEstimatedCost,
      getEstimatedTotal,
      getBPEffect,
      formatCurrency,
      formatExpiry,
      handleCancel,
      handleEdit,
      handleConfirm,
    };
  },
};
</script>

<style scoped>
/* Bottom Sheet Overlay */
.bottom-sheet-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.7);
  z-index: 9999;
  display: flex;
  align-items: flex-end;
}

/* Bottom Sheet Container */
.bottom-sheet {
  width: 100%;
  max-height: 80vh;
  background-color: var(--bg-secondary);
  border-radius: 12px 12px 0 0;
  color: var(--text-primary);
  transform: translateY(100%);
  transition: transform 0.3s ease-out;
  overflow-y: auto;
}

.bottom-sheet.slide-up {
  transform: translateY(0);
}

/* Trade Header */
.trade-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-secondary);
}

.trade-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
}

.trade-icon {
  font-size: 14px;
}

.quick-analysis {
  color: var(--text-tertiary);
  font-size: 12px;
  font-weight: 400;
}

.clear-trade-btn {
  background: none;
  border: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
}

.clear-trade-btn:hover {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

/* Trade Stats Bar */
.trade-stats {
  display: flex;
  justify-content: space-between;
  padding: 6px 16px;
  background-color: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-secondary);
  overflow-x: auto;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 60px;
}

.stat-label {
  font-size: 9px;
  color: var(--text-tertiary);
  font-weight: 500;
  margin-bottom: 2px;
}

.stat-value {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-primary);
}

.stat-value.positive {
  color: var(--color-success);
}

.stat-value.negative {
  color: var(--color-danger);
}

.stat-unit {
  font-size: 8px;
  color: var(--text-tertiary);
}

/* Main Content */
.sheet-content {
  display: flex;
  padding: 12px 16px;
  gap: 24px;
}

.left-section {
  flex: 1;
  max-width: 400px;
}

.right-section {
  flex: 2;
}

/* Account Section */
.account-section {
  margin-bottom: 16px;
}

.account-section h3 {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 4px 0;
  font-weight: 500;
}

.account-name {
  font-size: 14px;
  color: var(--text-primary);
  font-weight: 400;
}

/* Order Details */
.order-details-section h3 {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
  font-weight: 500;
}

.order-legs {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.order-leg {
  display: grid;
  grid-template-columns: 30px 50px 50px 20px 40px 60px;
  gap: 8px;
  align-items: center;
  padding: 6px 8px;
  background-color: var(--bg-tertiary);
  border-radius: 4px;
  font-size: 11px;
}

.leg-quantity {
  font-weight: 600;
  color: var(--text-primary);
}

.leg-date {
  color: var(--text-secondary);
  font-size: 10px;
}

.leg-details {
  color: var(--text-primary);
  font-weight: 500;
}

.leg-type {
  color: var(--text-tertiary);
  font-weight: 600;
  text-align: center;
}

.leg-action {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 3px;
  text-align: center;
}

.leg-action.buy {
  background-color: var(--options-selected-buy);
  color: var(--color-success);
}

.leg-action.sell {
  background-color: var(--options-selected-sell);
  color: var(--color-danger);
}

.leg-price {
  color: var(--text-primary);
  font-weight: 500;
  text-align: right;
}

/* Confirm Section */
.confirm-section {
  margin-bottom: 12px;
}

.confirm-title {
  font-size: 18px;
  color: var(--text-primary);
  margin: 0 0 12px 0;
  font-weight: 600;
}

.strategy-name {
  color: var(--color-success);
}

/* Trade Details Grid */
.trade-details-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: 12px;
  align-items: center;
}

.detail-label {
  font-size: 11px;
  color: var(--text-tertiary);
  font-weight: 500;
}

.detail-value {
  font-size: 12px;
  color: var(--text-primary);
  font-weight: 500;
}

.detail-value.positive {
  color: var(--color-success);
}

.detail-unit {
  font-size: 10px;
  color: var(--text-tertiary);
}

.positive {
  color: var(--color-success);
}

/* Warning Message */
.warning-message {
  background-color: rgba(255, 193, 7, 0.1);
  border: 1px solid rgba(255, 193, 7, 0.3);
  color: var(--color-warning);
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 12px;
  margin-top: 12px;
}

/* Action Buttons */
.action-buttons {
  display: flex;
  gap: 12px;
  padding: 12px 16px;
  border-top: 1px solid var(--border-secondary);
}

.edit-btn {
  flex: 1;
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  color: var(--text-secondary);
  padding: 10px 16px;
  border-radius: var(--radius-md);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: var(--transition-normal);
}

.edit-btn:hover {
  background-color: var(--bg-quaternary);
  color: var(--text-primary);
  border-color: var(--border-tertiary);
}

.submit-btn {
  flex: 2;
  background-color: var(--color-primary);
  border: none;
  color: var(--text-primary);
  padding: 10px 16px;
  border-radius: var(--radius-md);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: var(--transition-normal);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.submit-btn:hover:not(:disabled) {
  background-color: var(--color-primary-hover);
}

.submit-btn:disabled {
  background-color: var(--bg-tertiary);
  color: var(--text-quaternary);
  cursor: not-allowed;
}

/* Responsive Design */
@media (max-width: 768px) {
  .sheet-content {
    flex-direction: column;
    gap: 24px;
  }

  .left-section {
    max-width: none;
  }

  .trade-stats {
    flex-wrap: wrap;
    gap: 8px;
  }

  .stat-item {
    min-width: 60px;
  }

  .detail-row {
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .order-leg {
    grid-template-columns: 30px 50px 50px 25px 40px 60px;
    gap: 8px;
    font-size: 12px;
  }
}
</style>
