<template>
  <div class="order-ticket">
    <Card class="ticket-card">
      <template #title>
        <div class="ticket-header">
          <span>Order Ticket</span>
          <Button
            icon="pi pi-times"
            class="clear-button"
            text
            size="small"
            @click="clearSelections"
          />
        </div>
      </template>

      <template #content>
        <div class="ticket-content">
          <!-- Selected Options List -->
          <div class="selected-options">
            <div
              v-for="selection in selectedOptions"
              :key="selection.symbol"
              class="option-row"
              :class="getRowClass(selection)"
              @click="selectLeg(selection.symbol)"
            >
              <div class="option-info">
                <div class="option-details">
                  <span class="strike"
                    >${{ selection.strike?.toFixed(0) }}</span
                  >
                  <Tag
                    :value="selection.type?.toUpperCase()"
                    :severity="selection.type === 'call' ? 'success' : 'info'"
                    size="small"
                  />
                  <Tag
                    :value="selection.side?.toUpperCase()"
                    :severity="selection.side === 'buy' ? 'success' : 'danger'"
                    size="small"
                  />
                </div>
                <div class="option-price">
                  ${{ getOptionPrice(selection).toFixed(2) }}
                </div>
              </div>

              <div class="option-controls" @click.stop>
                <InputNumber
                  :model-value="selection.quantity"
                  :min="1"
                  :max="10"
                  size="small"
                  class="quantity-input"
                  @update:model-value="updateQuantity(selection.symbol, $event)"
                />
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  size="small"
                  text
                  @click="removeOption(selection.symbol)"
                />
              </div>
            </div>
          </div>

          <!-- Order Summary -->
          <div class="order-summary">
            <div class="summary-row">
              <span class="label">Net Premium:</span>
              <span class="value" :class="netPremiumClass">
                ${{ Math.abs(netPremium).toFixed(2) }}
                {{ netPremium >= 0 ? "CR" : "DB" }}
              </span>
            </div>

            <div class="summary-row">
              <span class="label">Max Risk:</span>
              <span class="value risk"> ${{ maxRisk.toFixed(2) }} </span>
            </div>

            <div class="summary-row">
              <span class="label">Max Reward:</span>
              <span class="value reward"> ${{ maxReward.toFixed(2) }} </span>
            </div>
          </div>

          <!-- Order Controls -->
          <div class="order-controls">
            <div class="control-row">
              <div class="control-group">
                <label>Order Type:</label>
                <Dropdown
                  v-model="orderType"
                  :options="orderTypeOptions"
                  optionLabel="label"
                  optionValue="value"
                  class="control-dropdown"
                />
              </div>

              <div class="control-group">
                <label>Time in Force:</label>
                <Dropdown
                  v-model="timeInForce"
                  :options="timeInForceOptions"
                  optionLabel="label"
                  optionValue="value"
                  class="control-dropdown"
                />
              </div>
            </div>

            <div class="price-row" v-if="orderType === 'limit'">
              <label>Limit Price:</label>
              <div class="price-input-group">
                <InputNumber
                  v-model="limitPrice"
                  :min="-100"
                  :max="100"
                  :step="0.01"
                  :minFractionDigits="2"
                  :maxFractionDigits="2"
                  size="small"
                  class="price-input"
                  showButtons
                  buttonLayout="horizontal"
                />
                <span class="price-type" :class="netPremiumClass">
                  {{ netPremium >= 0 ? "CR" : "DB" }}
                </span>
              </div>
            </div>

            <!-- Submit Button -->
            <Button
              :label="submitButtonLabel"
              :disabled="!canSubmitOrder"
              severity="warning"
              size="large"
              class="submit-button"
              @click="submitOrder"
            />
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>

<script>
import { ref, computed, watch, toRefs } from "vue";

export default {
  name: "OrderTicket",
  props: {
    selectedOptions: {
      type: Array,
      default: () => [],
    },
    optionsData: {
      type: Array,
      default: () => [],
    },
    underlyingSymbol: {
      type: String,
      default: "SPY",
    },
    underlyingPrice: {
      type: Number,
      default: null,
    },
  },
  emits: [
    "order-placed",
    "clear-selections",
    "option-deselected",
    "quantity-updated",
  ],
  setup(props, { emit }) {
    const { selectedOptions, optionsData, underlyingSymbol, underlyingPrice } =
      toRefs(props);

    // Reactive data
    const orderType = ref("limit");
    const timeInForce = ref("day");
    const limitPrice = ref(0);
    const selectedLeg = ref(null);

    // Options for dropdowns
    const orderTypeOptions = [
      { label: "Market", value: "market" },
      { label: "Limit", value: "limit" },
    ];

    const timeInForceOptions = [
      { label: "Day", value: "day" },
      { label: "GTC", value: "gtc" },
      { label: "IOC", value: "ioc" },
      { label: "FOK", value: "fok" },
    ];

    // Computed properties
    const netPremium = computed(() => {
      let total = 0;
      selectedOptions.value.forEach((selection) => {
        const price = getOptionPrice(selection);
        const premium = selection.side === "buy" ? -price : price;
        total += premium * selection.quantity * 100; // Options are in contracts of 100
      });
      return total;
    });

    const netPremiumClass = computed(() => ({
      credit: netPremium.value >= 0,
      debit: netPremium.value < 0,
    }));

    const maxRisk = computed(() => {
      // Simplified risk calculation - in reality this would be more complex
      if (netPremium.value < 0) {
        return Math.abs(netPremium.value);
      }

      // For credit spreads, max risk is typically the width minus credit received
      // This is a simplified calculation
      let maxLoss = 0;
      const strikes = selectedOptions.value
        .map((sel) => sel.strike)
        .sort((a, b) => a - b);
      if (strikes.length >= 2) {
        const width = Math.max(...strikes) - Math.min(...strikes);
        maxLoss = width * 100 - Math.abs(netPremium.value);
      }

      return Math.max(maxLoss, Math.abs(netPremium.value));
    });

    const maxReward = computed(() => {
      // Simplified reward calculation
      if (netPremium.value >= 0) {
        return netPremium.value;
      }

      // For debit spreads, max reward is typically the width minus debit paid
      const strikes = selectedOptions.value
        .map((sel) => sel.strike)
        .sort((a, b) => a - b);
      if (strikes.length >= 2) {
        const width = Math.max(...strikes) - Math.min(...strikes);
        return width * 100 - Math.abs(netPremium.value);
      }

      return 0;
    });

    const canSubmitOrder = computed(() => {
      return (
        selectedOptions.value.length > 0 &&
        selectedOptions.value.every((sel) => sel.quantity > 0)
      );
    });

    const submitButtonLabel = computed(() => {
      if (selectedOptions.value.length === 0) return "Select Options";
      if (orderType.value === "market") return "Place Market Order";
      return `Place ${netPremium.value >= 0 ? "Credit" : "Debit"} Order`;
    });

    // Methods
    const getOptionPrice = (selection) => {
      const option = optionsData.value.find(
        (opt) => opt.symbol === selection.symbol
      );
      if (!option) return 0;

      return selection.side === "buy" ? option.ask : option.bid;
    };

    const getRowClass = (selection) => ({
      "buy-row": selection.side === "buy",
      "sell-row": selection.side === "sell",
      "selected-leg": selectedLeg.value === selection.symbol,
    });

    const selectLeg = (symbol) => {
      selectedLeg.value = selectedLeg.value === symbol ? null : symbol;
    };

    const updateQuantity = (symbol, quantity) => {
      // Emit event to parent to update quantity instead of modifying prop directly
      emit("quantity-updated", { symbol, quantity: quantity || 1 });
    };

    const removeOption = (symbol) => {
      emit("option-deselected", symbol);
    };

    const clearSelections = () => {
      emit("clear-selections");
    };

    const submitOrder = () => {
      if (!canSubmitOrder.value) return;

      const orderData = {
        symbol: underlyingSymbol.value,
        legs: selectedOptions.value.map((selection) => ({
          symbol: selection.symbol,
          side: selection.side,
          quantity: selection.quantity,
          strike: selection.strike,
          type: selection.type,
          expiry: selection.expiry,
        })),
        orderType: orderType.value,
        timeInForce: timeInForce.value,
        limitPrice: orderType.value === "limit" ? limitPrice.value : null,
        netPremium: netPremium.value,
        maxRisk: maxRisk.value,
        maxReward: maxReward.value,
        underlyingPrice: underlyingPrice.value,
      };

      emit("order-placed", orderData);
    };

    // Watchers
    watch(
      netPremium,
      (newValue) => {
        // Auto-set limit price based on net premium
        if (orderType.value === "limit") {
          limitPrice.value = parseFloat((newValue / 100).toFixed(2));
        }
      },
      { immediate: true }
    );

    return {
      // Reactive data
      orderType,
      timeInForce,
      limitPrice,

      // Options
      orderTypeOptions,
      timeInForceOptions,

      // Computed
      netPremium,
      netPremiumClass,
      maxRisk,
      maxReward,
      canSubmitOrder,
      submitButtonLabel,

      // Methods
      getOptionPrice,
      getRowClass,
      selectLeg,
      updateQuantity,
      removeOption,
      clearSelections,
      submitOrder,
    };
  },
};
</script>

<style scoped>
.order-ticket {
  width: 100%;
}

.ticket-card {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

.ticket-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.clear-button {
  color: #888888 !important;
}

.clear-button:hover {
  color: #ffffff !important;
  background-color: #444444 !important;
}

.ticket-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.selected-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.option-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border-radius: 6px;
  border: 1px solid #444444;
  transition: all 0.2s ease;
  cursor: pointer;
}

.option-row:hover {
  border-color: #666666;
  background-color: rgba(255, 255, 255, 0.05);
}

.option-row.buy-row {
  background-color: rgba(0, 200, 81, 0.1);
  border-color: rgba(0, 200, 81, 0.3);
}

.option-row.sell-row {
  background-color: rgba(255, 68, 68, 0.1);
  border-color: rgba(255, 68, 68, 0.3);
}

.option-row.selected-leg {
  border-color: #ffd700;
  box-shadow: 0 0 0 2px rgba(255, 215, 0, 0.3);
  background-color: rgba(255, 215, 0, 0.1);
}

.option-row.selected-leg.buy-row {
  border-color: #ffd700;
  background-color: rgba(255, 215, 0, 0.15);
}

.option-row.selected-leg.sell-row {
  border-color: #ffd700;
  background-color: rgba(255, 215, 0, 0.15);
}

.option-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.option-details {
  display: flex;
  align-items: center;
  gap: 8px;
}

.strike {
  font-family: monospace;
  font-weight: 600;
  font-size: 14px;
  color: #ffffff;
}

.option-price {
  font-family: monospace;
  font-weight: 600;
  font-size: 12px;
  color: #cccccc;
}

.option-controls {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.quantity-input {
  width: 65px;
}

.order-summary {
  background-color: #333333;
  border-radius: 6px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.summary-row .label {
  font-size: 12px;
  color: #cccccc;
  font-weight: 500;
}

.summary-row .value {
  font-size: 12px;
  font-weight: 600;
}

.summary-row .value.credit {
  color: #00c851;
}

.summary-row .value.debit {
  color: #ff4444;
}

.summary-row .value.risk {
  color: #ff4444;
}

.summary-row .value.reward {
  color: #00c851;
}

.order-controls {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.control-row {
  display: flex;
  gap: 12px;
}

.control-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.control-group label {
  font-size: 12px;
  font-weight: 500;
  color: #cccccc;
}

.control-dropdown {
  width: 100%;
}

.price-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.price-row label {
  font-size: 12px;
  font-weight: 500;
  color: #cccccc;
}

.price-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.price-input {
  flex: 1;
}

.price-type {
  font-size: 11px;
  font-weight: 600;
  padding: 4px 8px;
  border-radius: 4px;
  min-width: 30px;
  text-align: center;
}

.price-type.credit {
  color: #00c851;
  background-color: rgba(0, 200, 81, 0.1);
  border: 1px solid #00c851;
}

.price-type.debit {
  color: #ff4444;
  background-color: rgba(255, 68, 68, 0.1);
  border: 1px solid #ff4444;
}

.submit-button {
  width: 100%;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.submit-button:disabled {
  background-color: #555555 !important;
  color: #888888 !important;
  border-color: #555555 !important;
}

/* Dark theme overrides for PrimeVue components */
:deep(.p-card) {
  background-color: #2a2a2a;
  border: 1px solid #444444;
}

:deep(.p-card .p-card-title) {
  color: #ffffff;
  font-size: 16px;
  font-weight: 600;
}

:deep(.p-card .p-card-content) {
  color: #cccccc;
  padding: 16px;
}

:deep(.p-dropdown) {
  background-color: #333333;
  border: 1px solid #444444;
  color: #ffffff;
}

:deep(.p-dropdown:not(.p-disabled):hover) {
  border-color: #555555;
}

:deep(.p-dropdown-panel) {
  background-color: #333333;
  border: 1px solid #444444;
}

:deep(.p-dropdown-item) {
  color: #ffffff;
}

:deep(.p-dropdown-item:not(.p-highlight):not(.p-disabled):hover) {
  background-color: #444444;
}

:deep(.p-inputnumber) {
  background-color: #333333;
  border: 1px solid #444444;
}

:deep(.p-inputnumber-input) {
  background-color: #333333;
  border: none;
  color: #ffffff;
  text-align: center;
  width: 65px;
}

:deep(.p-inputnumber-button) {
  background-color: #444444;
  border: 1px solid #555555;
  color: #cccccc;
}

:deep(.p-inputnumber-button:hover) {
  background-color: #555555;
  color: #ffffff;
}

:deep(.p-tag) {
  font-size: 10px;
  font-weight: 600;
}

:deep(.p-button.p-button-text) {
  color: #cccccc;
}

:deep(.p-button.p-button-text:hover) {
  background-color: #444444;
  color: #ffffff;
}
</style>
