<template>
  <div class="mandatory-routes-step">
    <div class="step-header">
      <h2 class="step-title">Configure Mandatory Routes</h2>
      <p class="step-description">
        Map the required services to your provider instances. All mandatory services must be configured to continue.
      </p>
    </div>

    <div class="routes-content">
      <!-- Loading State -->
      <div v-if="routingLoading" class="loading-section">
        <div class="loading-spinner"></div>
        <span>Loading service configuration...</span>
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
            <div class="section-icon mandatory">
              <i class="pi pi-exclamation-circle"></i>
            </div>
            <div class="section-info">
              <h3 class="section-title">Required Services</h3>
              <p class="section-description">These services must be configured for the platform to work properly</p>
            </div>
          </div>
          
          <!-- Mandatory Service Items -->
          <div class="service-items">
            <div 
              v-for="service in mandatoryServices" 
              :key="service.key"
              class="service-item mandatory"
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

        <!-- Configuration Progress -->
        <div class="configuration-progress">
          <div class="progress-header">
            <h3>Configuration Progress</h3>
            <span class="progress-text">
              {{ configuredServicesCount }} of {{ mandatoryServices.length }} services configured
            </span>
          </div>
          
          <div class="progress-bar">
            <div 
              class="progress-fill" 
              :style="{ width: `${configurationProgress}%` }"
            ></div>
          </div>
          
          <div class="progress-services">
            <div 
              v-for="service in mandatoryServices"
              :key="service.key"
              class="progress-service"
              :class="{ 'configured': isServiceConfigured(service.key) }"
            >
              <div class="service-indicator">
                <i :class="isServiceConfigured(service.key) ? 'pi pi-check' : 'pi pi-circle'"></i>
              </div>
              <span class="service-name">{{ service.name }}</span>
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

        <!-- Smart Suggestions -->
        <div v-if="smartSuggestions.length > 0" class="smart-suggestions">
          <h4>Smart Suggestions</h4>
          <div v-for="suggestion in smartSuggestions" :key="suggestion.service" class="suggestion-item">
            <div class="suggestion-info">
              <i class="pi pi-lightbulb"></i>
              <span>{{ suggestion.message }}</span>
            </div>
            <Button
              :label="suggestion.action"
              @click="applySuggestion(suggestion)"
              class="p-button-secondary"
              size="small"
            />
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
  name: 'WizardStep3MandatoryRoutes',
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
  emits: ['update-data'],
  setup(props, { emit }) {
    const { showSuccess, showError, showWarning } = useNotifications();

    // Service routing data
    const availableProviders = ref({});
    const currentConfig = ref({});
    const routingLoading = ref(false);
    const routingError = ref(null);
    const updatingRouting = ref(false);
    const validationWarnings = ref([]);

    // Mandatory services that must be configured
    const mandatoryServices = [
      {
        key: 'trade_account',
        name: 'Trade Account',
        description: 'Execute orders and manage your trading account',
        icon: 'pi pi-wallet'
      },
      {
        key: 'options_chain',
        name: 'Options Chain',
        description: 'Get options contract data and pricing',
        icon: 'pi pi-link'
      },
      {
        key: 'historical_data',
        name: 'Historical Data',
        description: 'Access historical price and volume data',
        icon: 'pi pi-chart-bar'
      },
      {
        key: 'symbol_lookup',
        name: 'Symbol Lookup',
        description: 'Search for stocks and get company information',
        icon: 'pi pi-search'
      },
      {
        key: 'streaming_quotes',
        name: 'Streaming Quotes',
        description: 'Real-time price updates and market data',
        icon: 'pi pi-bolt'
      }
    ];

    // Computed properties
    const configuredServicesCount = computed(() => {
      return mandatoryServices.filter(service => isServiceConfigured(service.key)).length;
    });

    const configurationProgress = computed(() => {
      return (configuredServicesCount.value / mandatoryServices.length) * 100;
    });

    const allMandatoryServicesConfigured = computed(() => {
      return mandatoryServices.every(service => isServiceConfigured(service.key));
    });

    const smartSuggestions = computed(() => {
      const suggestions = [];
      
      // Find providers that can cover multiple unconfigured services
      const unconfiguredServices = mandatoryServices.filter(service => !isServiceConfigured(service.key));
      
      if (unconfiguredServices.length > 1) {
        // Check which providers can cover multiple services
        Object.entries(availableProviders.value).forEach(([providerName, providerData]) => {
          const capabilities = providerData.capabilities || {};
          const allCapabilities = [
            ...(capabilities.rest || []),
            ...(capabilities.streaming || [])
          ];
          
          const coverableServices = unconfiguredServices.filter(service => 
            allCapabilities.includes(service.key)
          );
          
          if (coverableServices.length > 1) {
            suggestions.push({
              service: 'multiple',
              message: `${formatProviderName(providerName, providerData)} can provide ${coverableServices.length} services: ${coverableServices.map(s => s.name).join(', ')}`,
              action: 'Auto-configure',
              providerName,
              services: coverableServices.map(s => s.key)
            });
          }
        });
      }
      
      return suggestions.slice(0, 3); // Limit to 3 suggestions
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
        
        // Use the parent's mandatory routes configuration (which starts with null values)
        currentConfig.value = { ...props.wizardData.mandatoryRoutes };
        
        // Update wizard data
        emit('update-data', {
          mandatoryRoutes: { ...currentConfig.value },
          availableProviders: availableProvidersData
        });
        
        validateConfiguration();
        
      } catch (error) {
        console.error("Error loading routing data:", error);
        routingError.value = "Failed to load service routing configuration";
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
      return "Select provider...";
    };

    const onProviderChange = (serviceKey, newProvider) => {
      // Update the local configuration
      currentConfig.value[serviceKey] = newProvider;
      
      // Update wizard data
      emit('update-data', {
        mandatoryRoutes: { ...currentConfig.value }
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
          const service = mandatoryServices.find(s => s.key === serviceKey);
          warnings.push(`${service?.name || serviceKey} provider "${formatProviderName(providerName, availableProviders.value[providerName])}" is no longer available`);
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

    const getStatusIcon = (serviceKey) => {
      const provider = currentConfig.value[serviceKey];
      const availableProviders = getAvailableInstancesForService(serviceKey);
      
      if (!provider) {
        return "pi pi-exclamation-circle text-warning";
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
        return "No provider selected";
      }
      
      if (!availableProvidersForService.find(p => p.value === provider)) {
        return "Selected provider is not available";
      }
      
      const providerData = availableProviders.value[provider];
      return `Using ${formatProviderName(provider, providerData)}`;
    };

    const isServiceConfigured = (serviceKey) => {
      const value = currentConfig.value[serviceKey];
      return value && value !== null && value !== '' && value !== undefined;
    };

    const applySuggestion = (suggestion) => {
      if (suggestion.service === 'multiple') {
        // Auto-configure multiple services with the suggested provider
        suggestion.services.forEach(serviceKey => {
          currentConfig.value[serviceKey] = suggestion.providerName;
        });
        
        // Update wizard data
        emit('update-data', {
          mandatoryRoutes: { ...currentConfig.value }
        });
        
        validateConfiguration();
        
        showSuccess(
          `Auto-configured ${suggestion.services.length} services with ${suggestion.providerName}`,
          'Configuration Applied'
        );
      }
    };

    // Watch for changes in wizard data
    watch(() => props.wizardData.providers, () => {
      // Reload routing data when providers change
      loadRoutingData();
    }, { deep: true });

    // Watch for configuration changes and emit to parent
    watch(currentConfig, () => {
      emit('update-data', {
        mandatoryRoutes: { ...currentConfig.value }
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
      mandatoryServices,
      
      // Computed
      configuredServicesCount,
      configurationProgress,
      allMandatoryServicesConfigured,
      smartSuggestions,
      
      // Methods
      loadRoutingData,
      getAvailableInstancesForService,
      formatProviderName,
      getProviderPlaceholder,
      onProviderChange,
      validateConfiguration,
      getStatusIcon,
      getStatusTooltip,
      isServiceConfigured,
      applySuggestion
    };
  }
};
</script>

<style scoped>
.mandatory-routes-step {
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

.section-icon.mandatory {
  background-color: rgba(255, 107, 53, 0.1);
  color: var(--color-brand);
  border: 1px solid var(--color-brand);
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
  border-color: var(--color-success);
  background-color: rgba(0, 200, 81, 0.05);
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
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
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

.text-warning {
  color: var(--color-warning);
}

.text-danger {
  color: var(--color-danger);
}

/* Configuration Progress */
.configuration-progress {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
}

.progress-header h3 {
  margin: 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.progress-text {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.progress-bar {
  width: 100%;
  height: 8px;
  background-color: var(--border-secondary);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-lg);
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--color-brand) 0%, #ff8a50 100%);
  border-radius: var(--radius-sm);
  transition: width 0.3s ease;
}

.progress-services {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-md);
}

.progress-service {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.progress-service.configured {
  border-color: var(--color-success);
  background-color: rgba(0, 200, 81, 0.05);
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

.progress-service.configured .service-indicator {
  color: var(--color-success);
}

.progress-service:not(.configured) .service-indicator {
  color: var(--text-tertiary);
}

.service-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
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

/* Smart Suggestions */
.smart-suggestions {
  background-color: rgba(59, 130, 246, 0.1);
  border: 1px solid var(--color-info);
  border-radius: var(--radius-md);
  padding: var(--spacing-lg);
}

.smart-suggestions h4 {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--color-info);
}

.suggestion-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-sm);
}

.suggestion-item:last-child {
  margin-bottom: 0;
}

.suggestion-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex: 1;
}

.suggestion-info i {
  color: var(--color-info);
  flex-shrink: 0;
}

.suggestion-info span {
  font-size: var(--font-size-sm);
  color: var(--text-primary);
  line-height: 1.4;
}

/* Responsive Design */
@media (max-width: 768px) {
  .service-item {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-md);
  }
  
  .provider-selection {
    width: 100%;
    min-width: auto;
  }
  
  .progress-services {
    grid-template-columns: 1fr;
  }
  
  .suggestion-item {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
  
  .progress-header {
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
  
  .service-info {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
}
</style>
