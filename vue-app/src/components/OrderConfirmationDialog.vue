<template>
  <Dialog
    :visible="visible"
    modal
    header="Order Confirmation"
    :style="{ width: '50rem' }"
    @show="initializeOrderPrice"
    @hide="$emit('hide')"
    @update:visible="$emit('hide')"
  >
    <div v-if="orderData" class="order-confirmation">
      <!-- Order Summary -->
      <div class="order-summary">
        <!-- Basic Information - Two Column Layout -->
        <div class="summary-grid">
          <div class="summary-item">
            <strong>Strategy:</strong> {{ orderData.strategyType }}
          </div>
          <div class="summary-item">
            <strong>Symbol:</strong> {{ orderData.symbol }}
          </div>
          <div class="summary-item">
            <strong>Expiry:</strong> {{ formatExpiry(orderData.expiry) }}
          </div>
          <div v-if="orderData.underlyingPrice" class="summary-item">
            <strong>Underlying Price:</strong>
            ${{ orderData.underlyingPrice.toFixed(2) }}
          </div>
          <div
            v-if="orderData.currentOptionsPrice !== undefined"
            class="summary-item"
          >
            <strong>Options Price:</strong>
            ${{ orderData.currentOptionsPrice.toFixed(2) }}
          </div>
          <div
            v-if="orderData.calculatedOrderPrice !== undefined"
            class="summary-item"
          >
            <strong>Calculated Order Price:</strong>
            ${{ orderData.calculatedOrderPrice.toFixed(2) }}
          </div>
        </div>

        <!-- Trade Management specific information -->
        <div
          v-if="
            orderData.currentPositionPL !== undefined ||
            orderData.adjustmentCost !== undefined ||
            orderData.isClosingOrder
          "
          class="summary-section"
        >
          <div class="summary-grid">
            <div
              v-if="orderData.currentPositionPL !== undefined"
              class="summary-item"
            >
              <strong>Current Position P&L:</strong>
              <span
                :class="orderData.currentPositionPL >= 0 ? 'profit' : 'loss'"
              >
                ${{ orderData.currentPositionPL.toFixed(2) }}
              </span>
            </div>
            <div
              v-if="orderData.adjustmentCost !== undefined"
              class="summary-item"
            >
              <strong>Adjustment Cost:</strong>
              <span
                :class="
                  orderData.adjustmentType === 'Credit' ? 'profit' : 'loss'
                "
              >
                ${{ Math.abs(orderData.adjustmentCost).toFixed(2) }}
                {{ orderData.adjustmentType }}
              </span>
            </div>
            <div
              v-if="orderData.estimatedNewPL !== undefined"
              class="summary-item"
            >
              <strong>Estimated New P&L:</strong>
              <span :class="orderData.estimatedNewPL >= 0 ? 'profit' : 'loss'">
                ${{ orderData.estimatedNewPL.toFixed(2) }}
              </span>
            </div>
            <div
              v-if="orderData.totalContracts !== undefined"
              class="summary-item"
            >
              <strong>Total Contracts:</strong> {{ orderData.totalContracts }}
            </div>
            <div
              v-if="orderData.positionCount !== undefined"
              class="summary-item"
            >
              <strong>Positions Affected:</strong> {{ orderData.positionCount }}
            </div>

            <!-- Position Close specific information -->
            <div
              v-if="
                orderData.isClosingOrder &&
                orderData.estimatedProceeds !== undefined
              "
              class="summary-item"
            >
              <strong>Estimated Proceeds:</strong>
              <span
                :class="orderData.closeType === 'Credit' ? 'profit' : 'loss'"
              >
                ${{ orderData.estimatedProceeds.toFixed(2) }}
                {{ orderData.closeType }}
              </span>
            </div>
            <div
              v-if="
                orderData.isClosingOrder &&
                orderData.totalPLImpact !== undefined
              "
              class="summary-item"
            >
              <strong>Total P&L Impact:</strong>
              <span :class="orderData.totalPLImpact >= 0 ? 'profit' : 'loss'">
                ${{ orderData.totalPLImpact.toFixed(2) }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Editable Order Price -->
      <div class="order-price-section">
        <h4>Order Price</h4>
        <div class="field">
          <label for="orderPrice">Your Order Price:</label>
          <InputNumber
            id="orderPrice"
            v-model="editableOrderPrice"
            :min="0"
            :max="10"
            :step="0.01"
            :minFractionDigits="2"
            :maxFractionDigits="2"
            showButtons
            @input="updateCalculations"
          />
          <div v-if="calculatedSummary" class="net-type-info">
            <strong>Net Type:</strong>
            <span :class="calculatedSummary.netCredit ? 'credit' : 'debit'">
              {{ calculatedSummary.netCredit ? "Credit" : "Debit" }}
            </span>
          </div>
        </div>
      </div>

      <!-- Updated calculations based on manual price -->
      <div v-if="calculatedSummary" class="order-summary">
        <div class="summary-item">
          <strong>Max Profit:</strong> ${{
            calculatedSummary.maxProfit.toFixed(2)
          }}
        </div>
        <div class="summary-item">
          <strong>Max Loss:</strong> ${{ calculatedSummary.maxLoss.toFixed(2) }}
        </div>
      </div>

      <!-- Legs Table -->
      <h4>Legs to be Traded:</h4>
      <DataTable :value="formattedLegs" class="p-datatable-sm">
        <Column field="action" header="Action">
          <template #body="slotProps">
            <Tag
              :value="slotProps.data.action"
              :severity="slotProps.data.action === 'Buy' ? 'success' : 'danger'"
            />
          </template>
        </Column>
        <Column field="symbol" header="Symbol">
          <template #body="slotProps">
            <span class="symbol-cell">{{ slotProps.data.symbol }}</span>
          </template>
        </Column>
        <Column field="date" header="Date"></Column>
        <Column field="type" header="Type">
          <template #body="slotProps">
            <Tag
              :value="slotProps.data.type"
              :severity="slotProps.data.type === 'Call' ? 'success' : 'info'"
            />
          </template>
        </Column>
        <Column field="strike" header="Strike"></Column>
        <Column field="price" header="Price"></Column>
        <Column field="quantity" header="Qty"></Column>
      </DataTable>

      <!-- Validation Errors -->
      <div v-if="validationErrors.length > 0" class="validation-errors">
        <Message severity="error" :closable="false">
          <div>
            <strong>Please fix the following errors:</strong>
            <ul>
              <li v-for="error in validationErrors" :key="error">
                {{ error }}
              </li>
            </ul>
          </div>
        </Message>
      </div>
    </div>

    <template #footer>
      <Button label="Cancel" @click="handleCancel" severity="secondary" />
      <Button
        label="Confirm Order"
        @click="handleConfirm"
        :loading="loading"
        :disabled="!canConfirm"
        severity="success"
      />
    </template>
  </Dialog>
</template>

<script>
import { ref, computed, watch } from "vue";
import Tag from "primevue/tag";
import orderService from "../services/orderService";

export default {
  name: "OrderConfirmationDialog",
  components: {
    Tag,
  },
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
  emits: ["hide", "confirm", "cancel"],
  setup(props, { emit }) {
    const editableOrderPrice = ref(0);
    const calculatedSummary = ref(null);
    const validationErrors = ref([]);

    // Initialize order price when dialog opens
    const initializeOrderPrice = () => {
      if (props.orderData) {
        editableOrderPrice.value =
          props.orderData.orderPrice ||
          props.orderData.calculatedOrderPrice ||
          0;
        updateCalculations();
      }
    };

    // Update calculations when price changes
    const updateCalculations = () => {
      if (!props.orderData) return;

      const orderDataWithPrice = {
        ...props.orderData,
        orderPrice: editableOrderPrice.value,
      };

      calculatedSummary.value =
        orderService.calculateOrderSummary(orderDataWithPrice);

      // Validate order
      const validation = orderService.validateOrder(orderDataWithPrice);
      validationErrors.value = validation.errors;
    };

    // Format legs for display
    const formattedLegs = computed(() => {
      if (!props.orderData?.legs) return [];

      return props.orderData.legs.map((leg) => ({
        action: leg.side === "buy" ? "Buy" : "Sell",
        symbol: leg.displaySymbol || leg.symbol,
        date: leg.date || formatExpiry(props.orderData.expiry),
        type: leg.type || "Option",
        strike: leg.strike || "-",
        price: leg.price ? `$${leg.price.toFixed(2)}` : "-",
        quantity: leg.ratio_qty || leg.quantity || 1,
      }));
    });

    // Check if order can be confirmed
    const canConfirm = computed(() => {
      return (
        validationErrors.value.length === 0 &&
        editableOrderPrice.value !== null &&
        editableOrderPrice.value !== undefined
      );
    });

    // Format expiry date
    const formatExpiry = (expiry) => {
      if (!expiry) return "";
      if (typeof expiry === "string") return expiry;
      if (expiry instanceof Date) {
        return expiry.toLocaleDateString();
      }
      return expiry.toString();
    };

    // Handle cancel
    const handleCancel = () => {
      emit("cancel");
      emit("hide");
    };

    // Handle confirm
    const handleConfirm = () => {
      if (!canConfirm.value) return;

      const isCredit = calculatedSummary.value.netCredit;

      const finalOrderData = {
        ...props.orderData,
        orderPrice: isCredit
          ? -editableOrderPrice.value
          : editableOrderPrice.value,
      };

      emit("confirm", finalOrderData);
    };

    // Watch for orderData changes
    watch(
      () => props.orderData,
      () => {
        if (props.orderData) {
          initializeOrderPrice();
        }
      },
      { deep: true }
    );

    // Watch for visible changes
    watch(
      () => props.visible,
      (newVisible) => {
        if (newVisible && props.orderData) {
          initializeOrderPrice();
        }
      }
    );

    return {
      editableOrderPrice,
      calculatedSummary,
      validationErrors,
      formattedLegs,
      canConfirm,
      initializeOrderPrice,
      updateCalculations,
      formatExpiry,
      handleCancel,
      handleConfirm,
    };
  },
};
</script>

<style scoped>
.order-confirmation {
  padding: 20px 0;
}

.order-summary {
  margin-bottom: 20px;
}

.summary-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-bottom: 15px;
}

.summary-section {
  margin-top: 15px;
  padding-top: 15px;
  border-top: 1px solid #dee2e6;
}

.summary-item {
  padding: 8px 12px;
  background: #f8f9fa;
  border-radius: 4px;
  font-size: 0.9rem;
}

/* Responsive design for smaller screens */
@media (max-width: 768px) {
  .summary-grid {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}

.order-price-section {
  margin-bottom: 20px;
  padding: 15px;
  background: #f8f9fa;
  border-radius: 6px;
}

.order-price-section h4 {
  margin: 0 0 15px 0;
  color: #2c3e50;
}

.field {
  display: flex;
  flex-direction: column;
}

.field label {
  font-weight: 600;
  margin-bottom: 8px;
  color: #2c3e50;
}

.field small {
  margin-top: 5px;
  color: #6c757d;
  font-size: 0.875rem;
}

.net-type-info {
  margin-top: 8px;
  padding: 6px 10px;
  background: #fff;
  border-radius: 4px;
  border: 1px solid #e9ecef;
  font-size: 0.9rem;
}

.validation-errors {
  margin-top: 15px;
}

.validation-errors ul {
  margin: 10px 0 0 0;
  padding-left: 20px;
}

.validation-errors li {
  margin-bottom: 5px;
}

.symbol-cell {
  font-family: monospace;
  font-weight: 600;
}

.credit {
  color: #28a745;
  font-weight: 600;
}

.debit {
  color: #dc3545;
  font-weight: 600;
}

.profit {
  color: #28a745;
  font-weight: 600;
}

.loss {
  color: #dc3545;
  font-weight: 600;
}

/* DataTable styling */
:deep(.p-datatable .p-datatable-thead > tr > th) {
  background-color: #f8f9fa;
  color: #495057;
  font-weight: 600;
  border-bottom: 2px solid #dee2e6;
}

:deep(.p-datatable .p-datatable-tbody > tr:hover) {
  background-color: #f8f9fa;
}

:deep(.p-tag) {
  font-size: 0.75rem;
  font-weight: 600;
}
</style>
