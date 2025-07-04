<template>
  <Dialog
    :visible="visible"
    modal
    :header="orderResult?.success ? 'Order Success' : 'Order Failed'"
    :style="{ width: '40rem' }"
    @hide="$emit('hide')"
    @update:visible="$emit('hide')"
  >
    <div v-if="orderResult?.success" class="order-success">
      <Message severity="success" :closable="false">
        {{ orderResult.message || "Order Submitted Successfully!" }}
      </Message>

      <!-- Order Details -->
      <div v-if="orderResult.order" class="order-details">
        <h4>Order Details</h4>
        <div class="detail-grid">
          <div v-if="orderResult.order.id" class="detail-item">
            <strong>Order ID:</strong> {{ orderResult.order.id }}
          </div>
          <div v-if="orderResult.order.status" class="detail-item">
            <strong>Status:</strong>
            <Tag
              :value="orderResult.order.status"
              :severity="getStatusSeverity(orderResult.order.status)"
            />
          </div>
          <div v-if="orderResult.order.symbol" class="detail-item">
            <strong>Symbol:</strong> {{ orderResult.order.symbol }}
          </div>
          <div v-if="orderResult.order.qty" class="detail-item">
            <strong>Quantity:</strong> {{ orderResult.order.qty }}
          </div>
          <div v-if="orderResult.order.order_type" class="detail-item">
            <strong>Order Type:</strong> {{ orderResult.order.order_type }}
          </div>
          <div v-if="orderResult.order.time_in_force" class="detail-item">
            <strong>Time in Force:</strong>
            {{ orderResult.order.time_in_force }}
          </div>
          <div v-if="orderResult.order.limit_price" class="detail-item">
            <strong>Limit Price:</strong> ${{ orderResult.order.limit_price }}
          </div>
          <div v-if="orderResult.order.submitted_at" class="detail-item">
            <strong>Submitted:</strong>
            {{ formatDateTime(orderResult.order.submitted_at) }}
          </div>
        </div>
      </div>

      <!-- Raw JSON (collapsible) -->
      <div v-if="showRawData && orderResult.order" class="raw-data-section">
        <Button
          :label="showJson ? 'Hide Details' : 'Show Raw Details'"
          @click="showJson = !showJson"
          severity="secondary"
          size="small"
          class="toggle-json-btn"
        />
        <div v-if="showJson" class="order-result-json">
          <pre>{{ JSON.stringify(orderResult.order, null, 2) }}</pre>
        </div>
      </div>
    </div>

    <div v-else class="order-error">
      <Message severity="error" :closable="false">
        {{ orderResult?.message || "Order submission failed" }}
      </Message>

      <!-- Error Details -->
      <div v-if="orderResult?.error" class="error-details">
        <h4>Error Details</h4>
        <div class="error-content">
          <p><strong>Error:</strong> {{ orderResult.error }}</p>
          <div v-if="orderResult.details" class="error-extra">
            <p><strong>Additional Information:</strong></p>
            <pre>{{ JSON.stringify(orderResult.details, null, 2) }}</pre>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <Button label="Close" @click="handleClose" severity="secondary" />
      <Button
        v-if="orderResult?.success"
        label="View Positions"
        @click="handleViewPositions"
        severity="info"
      />
    </template>
  </Dialog>
</template>

<script>
import { ref, watch } from "vue";
import Tag from "primevue/tag";

export default {
  name: "OrderResultDialog",
  components: {
    Tag,
  },
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
    orderResult: {
      type: Object,
      default: null,
    },
    showRawData: {
      type: Boolean,
      default: true,
    },
  },
  emits: ["hide", "close", "viewPositions"],
  setup(props, { emit }) {
    const showJson = ref(false);

    // Get status severity for tag styling
    const getStatusSeverity = (status) => {
      if (!status) return "secondary";

      const statusLower = status.toLowerCase();
      switch (statusLower) {
        case "filled":
        case "accepted":
        case "new":
          return "success";
        case "pending":
        case "pending_new":
          return "warning";
        case "rejected":
        case "canceled":
        case "expired":
          return "danger";
        default:
          return "info";
      }
    };

    // Format date/time for display
    const formatDateTime = (dateString) => {
      if (!dateString) return "";

      try {
        const date = new Date(dateString);
        return date.toLocaleString();
      } catch (error) {
        return dateString;
      }
    };

    // Handle close
    const handleClose = () => {
      emit("close");
      emit("hide");
    };

    // Handle view positions
    const handleViewPositions = () => {
      emit("viewPositions");
      emit("hide");
    };

    // Reset JSON visibility when dialog opens/closes
    watch(
      () => props.visible,
      (newVisible) => {
        if (!newVisible) {
          showJson.value = false;
        }
      }
    );

    return {
      showJson,
      getStatusSeverity,
      formatDateTime,
      handleClose,
      handleViewPositions,
    };
  },
};
</script>

<style scoped>
.order-success,
.order-error {
  padding: 10px 0;
}

.order-details {
  margin-top: 20px;
}

.order-details h4 {
  margin: 0 0 15px 0;
  color: #2c3e50;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 10px;
  margin-bottom: 15px;
}

.detail-item {
  padding: 8px;
  background: #f8f9fa;
  border-radius: 4px;
  font-size: 0.9rem;
}

.detail-item strong {
  color: #495057;
}

.error-details {
  margin-top: 20px;
}

.error-details h4 {
  margin: 0 0 15px 0;
  color: #dc3545;
}

.error-content {
  background: #f8f9fa;
  padding: 15px;
  border-radius: 6px;
  border-left: 4px solid #dc3545;
}

.error-extra {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid #dee2e6;
}

.error-extra pre {
  background: #ffffff;
  padding: 10px;
  border-radius: 4px;
  border: 1px solid #dee2e6;
  font-size: 0.8rem;
  overflow-x: auto;
}

.raw-data-section {
  margin-top: 20px;
}

.toggle-json-btn {
  margin-bottom: 10px;
}

.order-result-json {
  background: #f8f9fa;
  padding: 15px;
  border-radius: 6px;
  border: 1px solid #dee2e6;
  max-height: 300px;
  overflow-y: auto;
}

.order-result-json pre {
  margin: 0;
  font-size: 0.875rem;
  line-height: 1.4;
  color: #495057;
}

/* Tag styling */
:deep(.p-tag) {
  font-size: 0.75rem;
  font-weight: 600;
}

/* Message styling */
:deep(.p-message) {
  margin-bottom: 0;
}

:deep(.p-message .p-message-wrapper) {
  padding: 12px 16px;
}

/* Success styling */
.order-success :deep(.p-message-success) {
  background-color: #d1edff;
  border: 1px solid #b3d7ff;
  color: #004085;
}

.order-success :deep(.p-message-success .p-message-icon) {
  color: #28a745;
}

/* Error styling */
.order-error :deep(.p-message-error) {
  background-color: #f8d7da;
  border: 1px solid #f5c6cb;
  color: #721c24;
}

.order-error :deep(.p-message-error .p-message-icon) {
  color: #dc3545;
}
</style>
