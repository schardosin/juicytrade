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
                <span class="detail-label">Net Credit @</span>
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
  emits: ["hide", "confirm", "cancel", "edit"],
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

      // Use the existing orderData values directly instead of recalculating
      const analysis = {
        maxProfit: Math.abs(props.orderData.maxReward || 0),
        maxLoss: Math.abs(props.orderData.maxRisk || 0),
        netPremium: props.orderData.netPremium || 0,
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

    const getNetCredit = () => {
      // Use the limit price from the order ticket
      const limitPrice =
        props.orderData?.limitPrice || props.orderData?.netPremium || 0;
      const creditDebitInfo = getCreditDebitInfo(limitPrice);
      return formatCurrency(creditDebitInfo.amount);
    };

    const getEstimatedCost = () => {
      // Use the limit price from the order ticket
      const limitPrice =
        props.orderData?.limitPrice || props.orderData?.netPremium || 0;
      const creditDebitInfo = getCreditDebitInfo(limitPrice);
      return `${formatCurrency(creditDebitInfo.amount)} ${
        creditDebitInfo.label
      }`;
    };

    const getEstimatedTotal = () => {
      // Use the limit price from the order ticket
      const limitPrice =
        props.orderData?.limitPrice || props.orderData?.netPremium || 0;
      const creditDebitInfo = getCreditDebitInfo(limitPrice);
      const fees = 3.56; // 2.00 + 1.56
      const total = creditDebitInfo.amount + fees;
      return `${formatCurrency(total)} ${creditDebitInfo.label}`;
    };

    const getBPEffect = () => {
      // For BP effect, use the max loss calculation but consider the limit price impact
      const limitPrice =
        props.orderData?.limitPrice || props.orderData?.netPremium || 0;
      const maxLoss = Math.abs(
        props.orderData?.maxRisk || Math.abs(limitPrice)
      );
      return formatCurrency(maxLoss);
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
  background-color: #1a1a1a;
  border-radius: 12px 12px 0 0;
  color: #ffffff;
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
  border-bottom: 1px solid #333333;
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
  color: #888888;
  font-size: 12px;
  font-weight: 400;
}

.clear-trade-btn {
  background: none;
  border: 1px solid #444444;
  color: #cccccc;
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
}

.clear-trade-btn:hover {
  background-color: #333333;
  color: #ffffff;
}

/* Trade Stats Bar */
.trade-stats {
  display: flex;
  justify-content: space-between;
  padding: 6px 16px;
  background-color: #2a2a2a;
  border-bottom: 1px solid #333333;
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
  color: #888888;
  font-weight: 500;
  margin-bottom: 2px;
}

.stat-value {
  font-size: 11px;
  font-weight: 600;
  color: #ffffff;
}

.stat-value.positive {
  color: #00c851;
}

.stat-value.negative {
  color: #ff4444;
}

.stat-unit {
  font-size: 8px;
  color: #888888;
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
  color: #cccccc;
  margin: 0 0 4px 0;
  font-weight: 500;
}

.account-name {
  font-size: 14px;
  color: #ffffff;
  font-weight: 400;
}

/* Order Details */
.order-details-section h3 {
  font-size: 12px;
  color: #cccccc;
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
  background-color: #2a2a2a;
  border-radius: 4px;
  font-size: 11px;
}

.leg-quantity {
  font-weight: 600;
  color: #ffffff;
}

.leg-date {
  color: #cccccc;
  font-size: 10px;
}

.leg-details {
  color: #ffffff;
  font-weight: 500;
}

.leg-type {
  color: #888888;
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
  background-color: rgba(0, 200, 81, 0.2);
  color: #00c851;
}

.leg-action.sell {
  background-color: rgba(255, 68, 68, 0.2);
  color: #ff4444;
}

.leg-price {
  color: #ffffff;
  font-weight: 500;
  text-align: right;
}

/* Confirm Section */
.confirm-section {
  margin-bottom: 12px;
}

.confirm-title {
  font-size: 18px;
  color: #ffffff;
  margin: 0 0 12px 0;
  font-weight: 600;
}

.strategy-name {
  color: #00c851;
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
  color: #888888;
  font-weight: 500;
}

.detail-value {
  font-size: 12px;
  color: #ffffff;
  font-weight: 500;
}

.detail-value.positive {
  color: #00c851;
}

.detail-unit {
  font-size: 10px;
  color: #888888;
}

.positive {
  color: #00c851;
}

/* Warning Message */
.warning-message {
  background-color: rgba(255, 193, 7, 0.1);
  border: 1px solid rgba(255, 193, 7, 0.3);
  color: #ffc107;
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
  border-top: 1px solid #333333;
}

.edit-btn {
  flex: 1;
  background-color: #444444;
  border: 1px solid #555555;
  color: #ffffff;
  padding: 10px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.edit-btn:hover {
  background-color: #555555;
}

.submit-btn {
  flex: 2;
  background-color: #00c851;
  border: none;
  color: #ffffff;
  padding: 10px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.submit-btn:hover:not(:disabled) {
  background-color: #00a844;
}

.submit-btn:disabled {
  background-color: #555555;
  color: #888888;
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
