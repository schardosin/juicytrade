<template>
  <div class="providers-tab">
    <!-- Header -->
    <div class="tab-header">
      <h2>Provider Configuration</h2>
      <p>Configure which providers to use for different services</p>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-section">
      <div class="loading-spinner"></div>
      <span>Loading provider capabilities...</span>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-section">
      <i class="pi pi-exclamation-triangle"></i>
      <span>{{ error }}</span>
      <Button
        label="Retry"
        icon="pi pi-refresh"
        @click="loadProviderData"
        class="retry-button"
        size="small"
      />
    </div>

    <!-- Provider Configuration -->
    <div v-else class="provider-config">
      <!-- Service Categories -->
      <div v-for="category in serviceCategories" :key="category.name" class="service-category">
        <h3 class="category-title">
          <i :class="category.icon"></i>
          {{ category.title }}
        </h3>
        
        <!-- Service Items -->
        <div class="service-items">
          <div v-for="service in category.services" :key="service.key" class="service-item">
            <div class="service-info">
              <label class="service-label">{{ service.label }}</label>
              <span class="service-description">{{ service.description }}</span>
            </div>
            
            <!-- Provider Dropdown -->
            <div class="provider-selection">
              <Dropdown
                v-model="currentConfig[service.key]"
                :options="getAvailableProvidersForService(service.key)"
                option-label="label"
                option-value="value"
                :placeholder="getProviderPlaceholder(service.key)"
                @change="onProviderChange(service.key, $event.value)"
                class="provider-dropdown"
                :loading="updating"
                :disabled="updating"
              />
              
              <!-- Status Indicator -->
              <div class="provider-status">
                <i 
                  :class="getStatusIcon(service.key)" 
                  :title="getStatusTooltip(service.key)"
                ></i>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- Validation Warnings -->
      <div v-if="validationWarnings.length > 0" class="validation-warnings">
        <h4>Configuration Warnings</h4>
        <div v-for="warning in validationWarnings" :key="warning" class="warning-item">
          <i class="pi pi-exclamation-triangle"></i>
          <span>{{ warning }}</span>
        </div>
      </div>

      <!-- Save Status -->
      <div v-if="lastSaved" class="save-status">
        <i class="pi pi-check-circle"></i>
        <span>Last saved: {{ formatTime(lastSaved) }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, computed, watch } from "vue";
import Dropdown from "primevue/dropdown";
import Button from "primevue/button";
import api from "../../services/api";
import { useNotifications } from "../../composables/useNotifications";

export default {
  name: "ProvidersTab",
  components: {
    Dropdown,
    Button,
  },
  setup() {
    const { showSuccess, showError, showWarning } = useNotifications();

    // Reactive data
    const availableProviders = ref({});
    const currentConfig = ref({});
    const loading = ref(true);
    const updating = ref(false);
    const error = ref(null);
    const validationWarnings = ref([]);
    const lastSaved = ref(null);

    // Service categories configuration
    const serviceCategories = [
      {
        name: "trading",
        title: "Trading Services",
        icon: "pi pi-chart-line",
        services: [
          {
            key: "trade_account",
            label: "Trade Account",
            description: "Order execution and account management"
          },
        ]
      },
      {
        name: "market",
        title: "Market Data",
        icon: "pi pi-database",
        services: [
          {
            key: "stock_quotes",
            label: "Stock Quotes",
            description: "Real-time stock price data"
          },
          {
            key: "options_chain",
            label: "Options Chain",
            description: "Options contract data and pricing"
          },
          {
            key: "expiration_dates",
            label: "Expiration Dates",
            description: "Available option expiration dates"
          },
          {
            key: "historical_data",
            label: "Historical Data",
            description: "Historical price and volume data"
          },
          {
            key: "symbol_lookup",
            label: "Symbol Lookup",
            description: "Symbol search and company information"
          },
          {
            key: "next_market_date",
            label: "Next Market Date",
            description: "Next trading day calculation"
          },
          {
            key: "market_calendar",
            label: "Market Calendar",
            description: "Trading holidays and market hours"
          },
          {
            key: "streaming_quotes",
            label: "Streaming Quotes",
            description: "Real-time price streaming"
          },
        ]
      },
    ];

    // Load provider data
    const loadProviderData = async () => {
      try {
        loading.value = true;
        error.value = null;
        
        // Load capabilities and current config in parallel
        const [capabilities, config] = await Promise.all([
          api.getAvailableProviders(),
          api.getProviderConfig()
        ]);
        
        availableProviders.value = capabilities;
        currentConfig.value = { ...config };
        
        // Validate current configuration
        validateConfiguration();
        
        console.log("Provider data loaded:", { capabilities, config });
        
      } catch (err) {
        console.error("Error loading provider data:", err);
        error.value = "Failed to load provider configuration. Please try again.";
        showError("Failed to load provider configuration", "Configuration Error");
      } finally {
        loading.value = false;
      }
    };

    // Get available providers for a specific service
    const getAvailableProvidersForService = (serviceKey) => {
      const providers = [];
      
      Object.entries(availableProviders.value).forEach(([providerName, providerData]) => {
        const capabilities = providerData.capabilities;
        
        // Check if provider supports this service (in rest or streaming)
        const supportsService = 
          capabilities.rest?.includes(serviceKey) || 
          capabilities.streaming?.includes(serviceKey);
          
        if (supportsService) {
          providers.push({
            label: formatProviderName(providerName),
            value: providerName,
            capabilities: capabilities
          });
        }
      });
      
      // Sort providers alphabetically
      return providers.sort((a, b) => a.label.localeCompare(b.label));
    };

    // Format provider name for display using backend data
    const formatProviderName = (providerName) => {
      const providerData = availableProviders.value[providerName];
      if (providerData) {
        const displayName = providerData.display_name || providerName;
        const accountType = providerData.paper ? "(Paper)" : "(Live)";
        return `${displayName} ${accountType}`;
      }
      
      // Fallback for unknown providers
      return providerName.charAt(0).toUpperCase() + providerName.slice(1);
    };

    // Get placeholder text for provider dropdown
    const getProviderPlaceholder = (serviceKey) => {
      const availableCount = getAvailableProvidersForService(serviceKey).length;
      if (availableCount === 0) {
        return "No providers available";
      }
      return "Select provider...";
    };

    // Handle provider change
    const onProviderChange = async (serviceKey, newProvider) => {
      if (!newProvider || updating.value) return;
      
      try {
        updating.value = true;
        
        // Update the configuration
        const updatedConfig = { ...currentConfig.value };
        updatedConfig[serviceKey] = newProvider;
        
        // Send to backend
        await api.updateProviderConfig(updatedConfig);
        
        // Update local state
        currentConfig.value = updatedConfig;
        lastSaved.value = new Date();
        
        // Validate new configuration
        validateConfiguration();
        
        showSuccess(
          `${serviceKey.replace('_', ' ')} provider updated to ${formatProviderName(newProvider)}`,
          "Provider Updated"
        );
        
        console.log(`Provider updated: ${serviceKey} -> ${newProvider}`);
        
      } catch (err) {
        console.error("Error updating provider:", err);
        showError("Failed to update provider configuration", "Update Error");
        
        // Revert the change in UI
        await loadProviderData();
      } finally {
        updating.value = false;
      }
    };

    // Validate current configuration
    const validateConfiguration = () => {
      const warnings = [];
      
      // Only validate if we have provider data loaded
      if (!availableProviders.value || Object.keys(availableProviders.value).length === 0) {
        return;
      }
      
      Object.entries(currentConfig.value).forEach(([serviceKey, providerName]) => {
        const availableProvidersForService = getAvailableProvidersForService(serviceKey);
        
        // Check if selected provider is still available
        if (!availableProvidersForService.find(p => p.value === providerName)) {
          warnings.push(`${serviceKey.replace('_', ' ')} provider "${formatProviderName(providerName)}" is no longer available`);
        }
        
        // Check for trading account warnings using backend paper flag
        if (serviceKey === 'trade_account' && providerName) {
          const providerData = availableProviders.value[providerName];
          if (providerData && !providerData.paper) {
            warnings.push('Live trading account selected - ensure you want to use real money');
          }
        }
      });
      
      validationWarnings.value = warnings;
    };

    // Get status icon for service
    const getStatusIcon = (serviceKey) => {
      const provider = currentConfig.value[serviceKey];
      const availableProviders = getAvailableProvidersForService(serviceKey);
      
      if (!provider) {
        return "pi pi-exclamation-circle text-warning";
      }
      
      if (!availableProviders.find(p => p.value === provider)) {
        return "pi pi-times-circle text-danger";
      }
      
      return "pi pi-check-circle text-success";
    };

    // Get status tooltip
    const getStatusTooltip = (serviceKey) => {
      const provider = currentConfig.value[serviceKey];
      const availableProviders = getAvailableProvidersForService(serviceKey);
      
      if (!provider) {
        return "No provider selected";
      }
      
      if (!availableProviders.find(p => p.value === provider)) {
        return "Selected provider is not available";
      }
      
      return `Using ${formatProviderName(provider)}`;
    };

    // Format time for display
    const formatTime = (date) => {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };

    // Load data on mount
    onMounted(() => {
      loadProviderData();
    });

    return {
      // Reactive data
      availableProviders,
      currentConfig,
      loading,
      updating,
      error,
      validationWarnings,
      lastSaved,
      
      // Static data
      serviceCategories,
      
      // Methods
      loadProviderData,
      getAvailableProvidersForService,
      formatProviderName,
      getProviderPlaceholder,
      onProviderChange,
      validateConfiguration,
      getStatusIcon,
      getStatusTooltip,
      formatTime,
    };
  },
};
</script>

<style scoped>
.providers-tab {
  height: 100%;
  overflow-y: auto;
}

.tab-header {
  margin-bottom: var(--spacing-xl);
  padding-bottom: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.tab-header h2 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.tab-header p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}

.loading-section,
.error-section {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-md);
  padding: var(--spacing-xxl);
  text-align: center;
  color: var(--text-secondary);
}

.loading-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-info);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.error-section {
  flex-direction: column;
  color: var(--color-danger);
}

.error-section i {
  font-size: var(--font-size-xl);
  margin-bottom: var(--spacing-sm);
}

.retry-button {
  margin-top: var(--spacing-md);
}

.provider-config {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xl);
}

.service-category {
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
}

.category-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin: 0 0 var(--spacing-lg) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.category-title i {
  color: var(--color-primary);
}

.service-items {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.service-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-md);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.service-item:hover {
  border-color: var(--border-tertiary);
}

.service-info {
  flex: 1;
  min-width: 0;
}

.service-label {
  display: block;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: 2px;
}

.service-description {
  font-size: var(--font-size-base);
  color: var(--text-secondary);
}

.provider-selection {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  min-width: 200px;
}

.provider-dropdown {
  flex: 1;
  min-width: 180px;
}

.provider-status i {
  font-size: var(--font-size-md);
}

.text-success {
  color: var(--color-success);
}

.text-warning {
  color: var(--color-warning);
}

.text-danger {
  color: var(--color-danger);
}

.validation-warnings {
  background-color: rgba(255, 187, 51, 0.1);
  border: 1px solid var(--color-warning);
  border-radius: var(--radius-md);
  padding: var(--spacing-lg);
}

.validation-warnings h4 {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--color-warning);
}

.warning-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
  font-size: var(--font-size-base);
  color: var(--text-primary);
}

.warning-item:last-child {
  margin-bottom: 0;
}

.warning-item i {
  color: var(--color-warning);
}

.save-status {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  background-color: rgba(0, 200, 81, 0.1);
  border: 1px solid var(--color-success);
  border-radius: var(--radius-md);
  font-size: var(--font-size-base);
  color: var(--text-primary);
}

.save-status i {
  color: var(--color-success);
}

/* Custom scrollbar */
.providers-tab::-webkit-scrollbar {
  width: 6px;
}

.providers-tab::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

.providers-tab::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.providers-tab::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}
</style>
