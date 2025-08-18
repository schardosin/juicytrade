<template>
  <div class="optional-routes-step">
    <div class="step-header">
      <h2 class="step-title">Configure Optional Routes</h2>
      <p class="step-description">
        Configure additional services to enhance your trading experience. These are optional and can be configured later.
      </p>
    </div>

    <div class="routes-content">
      <!-- Skip Option -->
      <div class="skip-option">
        <div class="skip-card">
          <div class="skip-icon">
            <i class="pi pi-forward"></i>
          </div>
          <div class="skip-content">
            <h3>Skip Optional Configuration</h3>
            <p>You can configure these services later in Settings. Your mandatory services are already set up and ready to go.</p>
          </div>
          <Button
            label="Skip This Step"
            icon="pi pi-arrow-right"
            @click="$emit('skip')"
            class="p-button-secondary"
          />
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="routingLoading" class="loading-section">
        <div class="loading-spinner"></div>
        <span>Loading optional services...</span>
      </div>

      <!-- Error State -->
      <div v-else-if="routingError" class="error-section">
        <i class="pi pi-exclamation-triangle"></i>
        <span>{{ routingError }}</span>
        <Button
          label="Retry"
          icon="pi pi-refresh"
          @click="loadRoutingData"
          class="retry-button"
          size="small"
        />
      </div>

      <!-- Service Configuration -->
      <div v-else class="service-config">
        <div class="config-section">
          <div class="section-header">
            <div class="section-icon optional">
              <i class="pi pi-info-circle"></i>
            </div>
            <div class="section-info">
              <h3 class="section-title">Optional Services</h3>
              <p class="section-description">Additional features to enhance your trading experience</p>
            </div>
          </div>
          
          <!-- Optional Service Items -->
          <div class="service-items">
            <div 
              v-for="service in optionalServices" 
              :key="service.key"
              class="service-item optional"
              :class="{ 'configured': isServiceConfigured(service.key) }"
            >
              <div class="service-info">
                <div class="service-icon">
                  <i :class="service.icon"></i>
                </div>
                <div class="service-details">
                  <label class="service-label">{{ service.name }}</label>
                  <span class="service-description">{{ service.description }}</span>
                </div>
              </div>
              
              <!-- Provider Dropdown -->
              <div class="provider-selection">
                <Dropdown
                  v-model="currentConfig[service.key]"
                  :options="getAvailableInstancesForService(service.key)"
                  option-label="label"
                  option-value="value"
                  :placeholder="getProviderPlaceholder(service.key)"
                  @change="onProviderChange(service.key, $event.value)"
                  class="provider-dropdown"
                  :loading="updatingRouting"
                  :disabled="updatingRouting"
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

        <!-- Configuration Summary -->
        <div class="configuration-summary">
          <div class="summary-header">
            <h3>Optional Services Summary</h3>
            <span class="summary-text">
              {{ configuredOptionalServicesCount }} of {{ optionalServices.length }} optional services configured
            </span>
          </div>
          
          <div class="summary-grid">
            <div 
              v-for="service in optionalServices"
              :key="service.key"
              class="summary-service"
              :class="{ 'configured': isServiceConfigured(service.key) }"
            >
              <div class="service-indicator">
                <i :class="isServiceConfigured(service.key) ? 'pi pi-check' : 'pi pi-minus'"></i>
              </div>
              <div class="service-info">
                <span class="service-name">{{ service.name }}</span>
                <span class="service-status">
                  {{ isServiceConfigured(service.key) ? 'Configured' : 'Not Configured' }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- Validation Warnings -->
        <div v-if="validationWarnings.length > 0" class="validation-warnings">
          <h4>Configuration Issues</h4>
          <div v-for="warning in validationWarnings" :key="warning" class="warning-item">
            <i class="pi pi-exclamation-triangle"></i>
            <span>{{ warning }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue';
import Dropdown from 'primevue/dropdown';
import Button from 'primevue/button';
import { useNotifications } from '../../composables/useNotifications';
import api from '../../services/api';

export default {
  name: 'WizardStep4OptionalRoutes',
  components: {
    Dropdown,
    Button
  },
  props: {
    wizardData: {
      type: Object,
      required: true
    }
  },
  emits: ['update-data', 'skip'],
  setup(props, { emit }) {
    const { showSuccess, showError } = useNotifications();

    // Service routing data
    const availableProviders = ref({});
    const currentConfig = ref({});
    const routingLoading = ref(false);
    const routingError = ref(null);
    const updatingRouting = ref(false);
    const validationWarnings = ref([]);

    // Optional services
    const optionalServices = [
      {
        key: 'stock_quotes',
        name: 'Stock Quotes',
        description: 'Real-time stock price data',
        icon: 'pi pi-dollar'
      },
      {
        key: 'market_calendar',
        name: 'Market Calendar',
        description: 'Trading holidays and market hours',
        icon: 'pi pi-calendar'
      },
      {
        key: 'greeks',
        name: 'Option Greeks',
        description: 'Option Greeks via API calls',
        icon: 'pi pi-calculator'
      },
      {
        key: 'streaming_greeks',
        name: 'Streaming Greeks',
        description: 'Real-time option Greeks streaming',
        icon: 'pi pi-calculator'
      }
    ];

    // Computed properties
    const configuredOptionalServicesCount = computed(() => {
      return optionalServices.filter(service => isServiceConfigured(service.key)).length;
    });

    // Methods
    const loadRoutingData = async () => {
      try {
        routingLoading.value = true;
        routingError.value = null;
        
        const [availableProvidersData, existingConfig] = await Promise.all([
          api.getAvailableProviders(),
          api.getProviderConfig().catch(() => ({}))
        ]);
        
        availableProviders.value = availableProvidersData;
        
        // Initialize current config with existing optional routes
        const optionalConfig = {};
        optionalServices.forEach(service => {
          optionalConfig[service.key] = existingConfig[service.key] || null;
        });
        
        currentConfig.value = optionalConfig;
        
        // Update wizard data
        emit('update-data', {
          optionalRoutes: optionalConfig,
          availableProviders: availableProvidersData
        });
        
        validateConfiguration();
        
      } catch (error) {
        console.error("Error loading routing data:", error);
        routingError.value = "Failed to load optional services configuration";
      } finally {
        routingLoading.value = false;
      }
    };

    const getAvailableInstancesForService = (serviceKey) => {
      const providers = [];
      
      Object.entries(availableProviders.value).forEach(([providerName, providerData]) => {
        const capabilities = providerData.capabilities || {};
        
        // Check if provider supports this service (in rest or streaming)
        const supportsService = 
          capabilities.rest?.includes(serviceKey) || 
          capabilities.streaming?.includes(serviceKey);
          
        if (supportsService) {
          providers.push({
            label: formatProviderName(providerName, providerData),
            value: providerName,
            capabilities: capabilities
          });
        }
      });
      
      // Sort providers alphabetically
      return providers.sort((a, b) => a.label.localeCompare(b.label));
    };

    const formatProviderName = (providerName, providerData) => {
      if (!providerName) {
        return "None";
      }
      
      if (providerData) {
        const displayName = providerData.display_name || providerName;
        const accountType = providerData.paper ? "(Paper)" : "(Live)";
        return `${displayName} ${accountType}`;
      }
      
      return providerName.charAt(0).toUpperCase() + providerName.slice(1);
    };

    const getProviderPlaceholder = (serviceKey) => {
      const availableCount = getAvailableInstancesForService(serviceKey).length;
      if (availableCount === 0) {
        return "No providers available";
      }
      return "Select provider (optional)...";
    };

    const onProviderChange = (serviceKey, newProvider) => {
      // Update the local configuration
      currentConfig.value[serviceKey] = newProvider;
      
      // Update wizard data
      emit('update-data', {
        optionalRoutes: { ...currentConfig.value }
      });
      
      // Validate new configuration
      validateConfiguration();
    };

    const validateConfiguration = () => {
      const warnings = [];
      
      // Only validate if we have provider data loaded
      if (!availableProviders.value || Object.keys(availableProviders.value).length === 0) {
        return;
      }
      
      Object.entries(currentConfig.value).forEach(([serviceKey, providerName]) => {
        if (!providerName) return;
        
        const availableProvidersForService = getAvailableInstancesForService(serviceKey);
        
        // Check if selected provider is still available
        if (!availableProvidersForService.find(p => p.value === providerName)) {
          const service = optionalServices.find(s => s.key === serviceKey);
          warnings.push(`${service?.name || serviceKey} provider "${formatProviderName(providerName, availableProviders.value[providerName])}" is no longer available`);
        }
      });
      
      validationWarnings.value = warnings;
    };

    const getStatusIcon = (serviceKey) => {
      const provider = currentConfig.value[serviceKey];
      const availableProviders = getAvailableInstancesForService(serviceKey);
      
      if (!provider) {
        return "pi pi-minus text-secondary";
      }
      
      if (!availableProviders.find(p => p.value === provider)) {
        return "pi pi-times-circle text-danger";
      }
      
      return "pi pi-check-circle text-success";
    };

    const getStatusTooltip = (serviceKey) => {
      const provider = currentConfig.value[serviceKey];
      const availableProvidersForService = getAvailableInstancesForService(serviceKey);
      
      if (!provider) {
        return "Optional service - not configured";
      }
      
      if (!availableProvidersForService.find(p => p.value === provider)) {
        return "Selected provider is not available";
      }
      
      const providerData = availableProviders.value[provider];
      return `Using ${formatProviderName(provider, providerData)}`;
    };

    const isServiceConfigured = (serviceKey) => {
      return currentConfig.value[serviceKey] && currentConfig.value[serviceKey] !== null;
    };

    // Watch for changes in wizard data
    watch(() => props.wizardData.providers, () => {
      // Reload routing data when providers change
      loadRoutingData();
    }, { deep: true });

    // Watch for configuration changes and emit to parent
    watch(currentConfig, () => {
      emit('update-data', {
        optionalRoutes: { ...currentConfig.value }
      });
    }, { deep: true });

    // Load data on mount
    onMounted(() => {
      loadRoutingData();
    });

    return {
      // State
      availableProviders,
      currentConfig,
      routingLoading,
      routingError,
      updatingRouting,
      validationWarnings,
      optionalServices,
      
      // Computed
      configuredOptionalServicesCount,
      
      // Methods
      loadRoutingData,
      getAvailableInstancesForService,
      formatProviderName,
      getProviderPlaceholder,
      onProviderChange,
      validateConfiguration,
      getStatusIcon,
      getStatusTooltip,
      isServiceConfigured
    };
  }
};
</script>

<style scoped>
.optional-routes-step {
  max-width: 800px;
  margin: 0 auto;
}
.step-header {
  text-align: center;
  margin-bottom: var(--spacing-xl);
}
.step-title {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}
.step-description {
  margin: 0;
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
  line-height: 1.6;
}
.routes-content {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xl);
}
/* Skip Option */
.skip-option {
  margin-bottom: var(--spacing-lg);
}
.skip-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-lg);
  padding: var(--spacing-xl);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1) 0%, rgba(147, 197, 253, 0.05) 100%);
  border: 1px solid var(--color-info);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
}
.skip-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(59, 130, 246, 0.15);
}
.skip-icon {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-lg);
  flex-shrink: 0;
}
.skip-content {
  flex: 1;
}
.skip-content h3 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}
.skip-content p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  line-height: 1.5;
}
/* Loading and Error States */
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
/* Service Configuration */
.service-config {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xl);
}
.config-section {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
}
.section-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-lg);
}
.section-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-lg);
}
.section-icon.optional {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  border: 1px solid var(--color-info);
}
.section-info {
  flex: 1;
}
.section-title {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}
.section-description {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}
/* Service Items */
.service-items {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}
.service-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}
.service-item:hover {
  border-color: var(--border-tertiary);
}
.service-item.configured {
  border-color: var(--color-info);
  background-color: rgba(59, 130, 246, 0.05);
}
.service-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  flex: 1;
  min-width: 0;
}
.service-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-secondary);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  flex-shrink: 0;
}
.service-item.configured .service-icon {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
}
.service-details {
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
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  line-height: 1.4;
}
.provider-selection {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  min-width: 250px;
  flex-shrink: 0;
}
.provider-dropdown {
  flex: 1;
  min-width: 200px;
}
.provider-status i {
  font-size: var(--font-size-md);
}
.text-success {
  color: var(--color-success);
}
.text-secondary {
  color: var(--text-secondary);
}
.text-danger {
  color: var(--color-danger);
}
/* Configuration Summary */
.configuration-summary {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
}
.summary-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
}
.summary-header h3 {
  margin: 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}
.summary-text {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-md);
}
.summary-service {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}
.summary-service.configured {
  border-color: var(--color-info);
  background-color: rgba(59, 130, 246, 0.05);
}
.service-indicator {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-sm);
  flex-shrink: 0;
}
.summary-service.configured .service-indicator {
  color: var(--color-info);
}
.summary-service:not(.configured) .service-indicator {
  color: var(--text-tertiary);
}
.summary-service .service-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}
.service-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}
.service-status {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}
.summary-service.configured .service-status {
  color: var(--color-info);
}
/* Validation Warnings */
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
  font-size: var(--font-size-sm);
  color: var(--text-primary);
}
.warning-item:last-child {
  margin-bottom: 0;
}
.warning-item i {
  color: var(--color-warning);
  flex-shrink: 0;
}
/* Responsive Design */
@media (max-width: 768px) {
  .skip-card {
    flex-direction: column;
    text-align: center;
    gap: var(--spacing-md);
  }
  .service-item {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-md);
  }
  .provider-selection {
    width: 100%;
    min-width: auto;
  }
  .summary-grid {
    grid-template-columns: 1fr;
  }
  .summary-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
}
@media (max-width: 480px) {
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
}
</style>
