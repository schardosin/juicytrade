<template>
  <div class="providers-tab">
    <!-- Sub-tab Navigation -->
    <div class="sub-tab-nav">
      <button
        v-for="tab in subTabs"
        :key="tab.key"
        :class="['sub-tab-btn', { active: activeSubTab === tab.key }]"
        @click="setActiveSubTab(tab.key)"
      >
        <i :class="tab.icon"></i>
        <span>{{ tab.label }}</span>
      </button>
    </div>

    <!-- Provider Instances Tab -->
    <div v-if="activeSubTab === 'instances'" class="sub-tab-content">
      <div class="tab-header">
        <div class="header-content">
          <h2>Provider Instances</h2>
          <p>Manage your trading provider connections</p>
        </div>
        <Button
          label="Add Provider"
          icon="pi pi-plus"
          @click="showAddProviderDialog = true"
          class="add-provider-btn p-button-brand"
        />
      </div>

      <!-- Loading State -->
      <div v-if="instancesLoading" class="loading-section">
        <div class="loading-spinner"></div>
        <span>Loading provider instances...</span>
      </div>

      <!-- Error State -->
      <div v-else-if="instancesError" class="error-section">
        <i class="pi pi-exclamation-triangle"></i>
        <span>{{ instancesError }}</span>
        <Button
          label="Retry"
          icon="pi pi-refresh"
          @click="loadProviderInstances"
          class="retry-button"
          size="small"
        />
      </div>

      <!-- Provider Instances List -->
      <div v-else-if="providerInstances && Object.keys(providerInstances).length > 0" class="provider-instances">
        <div 
          v-for="(instance, instanceId) in providerInstances" 
          :key="instanceId" 
          class="provider-instance"
          :class="{ 'inactive': !instance.active }"
        >
          <!-- Fixed width section for logo and name -->
          <div class="instance-main-info">
            <div class="instance-icon">
              <img 
                v-if="!getProviderIcon(instance.provider_type)" 
                :src="`/src/assets/logos/${instance.provider_type}.svg`" 
                :alt="`${instance.provider_type} logo`"
                class="provider-logo"
              />
              <i v-else :class="getProviderIcon(instance.provider_type)"></i>
            </div>
            <div class="instance-details">
              <h4 class="instance-name">{{ instance.display_name }}</h4>
              <div class="instance-meta">
                <span class="provider-type">{{ getProviderTypeName(instance.provider_type) }}</span>
                <span class="separator">•</span>
                <span class="account-type" :class="instance.account_type">
                  {{ instance.account_type.toUpperCase() }}
                </span>
              </div>
            </div>
          </div>

          <!-- Status badge column -->
          <div class="instance-status">
            <span class="status-badge" :class="{ 'active': instance.active, 'inactive': !instance.active }">
              {{ instance.active ? 'Active' : 'Inactive' }}
            </span>
          </div>
          
          <!-- Actions column - aligned -->
          <div class="instance-actions">
            <Button
              :icon="instance.active ? 'pi pi-pause' : 'pi pi-play'"
              :label="instance.active ? 'Deactivate' : 'Activate'"
              @click="toggleProviderInstance(instanceId)"
              :class="instance.active ? 'p-button-warning' : 'p-button-success'"
              size="small"
              text
              :loading="togglingInstances.has(instanceId)"
            />
            <Button
              icon="pi pi-cog"
              label="Configure"
              @click="editProviderInstance(instanceId)"
              class="p-button-secondary"
              size="small"
              text
            />
            <Button
              icon="pi pi-trash"
              label="Delete"
              @click="confirmDeleteInstance(instanceId)"
              class="p-button-danger"
              size="small"
              text
            />
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="empty-state">
        <i class="pi pi-server"></i>
        <h4>No Provider Instances</h4>
        <p>Add your first provider instance to start trading</p>
        <Button
          label="Add Provider"
          icon="pi pi-plus"
          @click="showAddProviderDialog = true"
          class="p-button-primary"
        />
      </div>
    </div>

    <!-- Service Routing Tab -->
    <div v-if="activeSubTab === 'routing'" class="sub-tab-content">
      <div class="tab-header">
        <div class="header-content">
          <h2>Service Routing</h2>
          <p>Configure which provider instances to use for different services</p>
        </div>
      </div>

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

    <!-- Add/Edit Provider Dialog -->
    <Dialog
      v-model:visible="showAddProviderDialog"
      :header="editingInstance ? 'Edit Provider Instance' : 'Add Provider Instance'"
      modal
      class="provider-dialog"
      :style="{ width: '800px' }"
    >
      <form class="dialog-content" @submit.prevent="saveProvider">
        <!-- Step 1: Provider Type Selection -->
        <div v-if="dialogStep === 1" class="dialog-step">
          <h4>Select Provider Type</h4>
          <div class="provider-type-grid">
            <div
              v-for="(providerType, key) in providerTypes"
              :key="key"
              class="provider-type-card"
              :class="{ selected: newProvider.provider_type === key }"
              @click="selectProviderType(key)"
            >
              <div class="provider-icon">
                <img 
                  v-if="!getProviderIcon(key)" 
                  :src="`/src/assets/logos/${key}.svg`" 
                  :alt="`${key} logo`"
                  class="provider-logo"
                />
                <i v-else :class="getProviderIcon(key)"></i>
              </div>
              <div class="provider-info">
                <h5>{{ providerType.name }}</h5>
                <p>{{ providerType.description }}</p>
                <div class="supported-accounts">
                  <span
                    v-for="accountType in providerType.supports_account_types"
                    :key="accountType"
                    class="account-badge"
                    :class="accountType"
                  >
                    {{ accountType.toUpperCase() }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 2: Account Type Selection -->
        <div v-if="dialogStep === 2" class="dialog-step">
          <h4>Select Account Type</h4>
          <div class="account-type-selection">
            <div
              v-for="accountType in getSelectedProviderAccountTypes()"
              :key="accountType"
              class="account-type-card"
              :class="{ selected: newProvider.account_type === accountType }"
              @click="selectAccountType(accountType)"
            >
              <div class="account-icon">
                <i :class="accountType === 'live' ? 'pi pi-dollar' : 'pi pi-flask'"></i>
              </div>
              <div class="account-info">
                <h5>{{ accountType === 'live' ? 'Live Trading' : 'Paper Trading' }}</h5>
                <p>{{ accountType === 'live' ? 'Real money trading' : 'Practice with virtual money' }}</p>
              </div>
              <div v-if="accountType === 'live'" class="warning-badge">
                <i class="pi pi-exclamation-triangle"></i>
                <span>Real Money</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 3: Credentials Form -->
        <div v-if="dialogStep === 3" class="dialog-step">
          <h4>Configure Credentials</h4>
          <div class="credentials-form">
            <!-- Display Name -->
            <div class="form-group">
              <label>Display Name</label>
              <InputText
                v-model="newProvider.display_name"
                placeholder="Enter a name for this provider instance"
                autocomplete="off"
                class="w-full"
              />
            </div>

            <!-- Dynamic Credential Fields -->
            <div
              v-for="field in getCredentialFields()"
              :key="field.name"
              class="form-group"
            >
              <label>{{ field.label }}</label>
              <InputText
                v-model="newProvider.credentials[field.name]"
                :type="field.type"
                :placeholder="field.placeholder"
                :required="field.required"
                :autocomplete="field.type === 'password' ? 'current-password' : 'off'"
                class="w-full"
              />
              <small v-if="field.default" class="field-hint">
                Default: {{ field.default }}
              </small>
            </div>

            <!-- Test Connection -->
            <div class="test-connection">
              <Button
                label="Test Connection"
                icon="pi pi-wifi"
                @click="testConnection"
                class="p-button-secondary"
                :loading="testingConnection"
                :disabled="!canTestConnection"
                type="button"
              />
              <div v-if="connectionTestResult" class="test-result" :class="connectionTestResult.success ? 'success' : 'error'">
                <i :class="connectionTestResult.success ? 'pi pi-check' : 'pi pi-times'"></i>
                <span>{{ connectionTestResult.message }}</span>
              </div>
            </div>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="dialog-footer">
          <Button
            v-if="dialogStep > 1"
            label="Back"
            icon="pi pi-arrow-left"
            @click="dialogStep--"
            class="p-button-secondary"
          />
          <Button
            v-if="dialogStep < 3"
            label="Next"
            icon="pi pi-arrow-right"
            @click="dialogStep++"
            :disabled="!canProceedToNextStep"
          />
          <Button
            v-if="dialogStep === 3"
            :label="editingInstance ? 'Update' : 'Create'"
            icon="pi pi-check"
            @click="saveProvider"
            :loading="savingProvider"
            :disabled="!canSaveProvider"
          />
          <Button
            label="Cancel"
            icon="pi pi-times"
            @click="closeDialog"
            class="p-button-text"
          />
        </div>
      </template>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <Dialog
      v-model:visible="showDeleteDialog"
      header="Delete Provider Instance"
      modal
      class="delete-dialog"
      :style="{ width: '400px' }"
    >
      <div class="delete-content">
        <i class="pi pi-exclamation-triangle warning-icon"></i>
        <p>Are you sure you want to delete this provider instance?</p>
        <p><strong>{{ instanceToDelete?.display_name }}</strong></p>
        <p class="warning-text">This action cannot be undone.</p>
      </div>
      <template #footer>
        <Button
          label="Cancel"
          icon="pi pi-times"
          @click="showDeleteDialog = false"
          class="p-button-text"
        />
        <Button
          label="Delete"
          icon="pi pi-trash"
          @click="deleteProviderInstance"
          class="p-button-danger"
          :loading="deletingInstance"
        />
      </template>
    </Dialog>
  </div>
</template>

<script>
import { ref, onMounted, computed, watch } from "vue";
import Dropdown from "primevue/dropdown";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import { useMarketData } from "../../composables/useMarketData";
import { useNotifications } from "../../composables/useNotifications";
import api from "../../services/api";

export default {
  name: "ProvidersTab",
  components: {
    Dropdown,
    Button,
    Dialog,
    InputText,
  },
  setup() {
    const { showSuccess, showError, showWarning } = useNotifications();
    const { 
      getAvailableProviders, 
      getProviderConfig, 
      refreshProviderData,
      updateProviderConfig,
      isLoading,
      getError
    } = useMarketData();

    // Sub-tab management
    const activeSubTab = ref("instances");
    const subTabs = [
      { key: "instances", label: "Provider Instances", icon: "pi pi-server" },
      { key: "routing", label: "Service Routing", icon: "pi pi-sitemap" }
    ];

    // Provider instances data
    const providerInstances = ref({});
    const providerTypes = ref({});
    const instancesLoading = ref(false);
    const instancesError = ref(null);
    const togglingInstances = ref(new Set());

    // Service routing data
    const reactiveAvailableProviders = getAvailableProviders();
    const reactiveProviderConfig = getProviderConfig();
    const routingLoading = computed(() => 
      isLoading("providers.available").value || isLoading("providers.config").value
    );
    const routingError = computed(() => 
      getError("providers.available").value || getError("providers.config").value
    );
    const availableProviders = computed(() => reactiveAvailableProviders.value || {});
    const currentConfig = computed(() => reactiveProviderConfig.value || {});
    const updatingRouting = ref(false);
    const validationWarnings = ref([]);
    const lastSaved = ref(null);

    // Dialog management
    const showAddProviderDialog = ref(false);
    const showDeleteDialog = ref(false);
    const dialogStep = ref(1);
    const editingInstance = ref(null);
    const instanceToDelete = ref(null);
    const savingProvider = ref(false);
    const deletingInstance = ref(false);
    const testingConnection = ref(false);
    const connectionTestResult = ref(null);

    // New provider form data
    const newProvider = ref({
      provider_type: '',
      account_type: '',
      display_name: '',
      credentials: {}
    });

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

    // Methods
    const setActiveSubTab = (tabKey) => {
      activeSubTab.value = tabKey;
    };

    const loadProviderInstances = async () => {
      try {
        instancesLoading.value = true;
        instancesError.value = null;
        
        const [instancesData, typesData] = await Promise.all([
          api.getProviderInstances(),
          api.getProviderTypes()
        ]);
        
        providerInstances.value = instancesData;
        providerTypes.value = typesData;
        
      } catch (error) {
        console.error("Error loading provider instances:", error);
        instancesError.value = "Failed to load provider instances";
      } finally {
        instancesLoading.value = false;
      }
    };

    const loadRoutingData = async () => {
      try {
        await refreshProviderData();
        validateConfiguration();
      } catch (error) {
        console.error("Error loading routing data:", error);
        showError("Failed to load service routing configuration", "Configuration Error");
      }
    };

    const getProviderIcon = (providerType) => {
      // Return null for providers with SVG logos, use fallback icons for others
      const svgProviders = ['alpaca', 'tradier', 'public'];
      if (svgProviders.includes(providerType)) {
        return null; // Will use SVG logo instead
      }
      
      const icons = {
        // Fallback icons for providers without SVG logos
      };
      return icons[providerType] || "pi pi-server";
    };

    const getProviderTypeName = (providerType) => {
      return providerTypes.value[providerType]?.name || providerType;
    };

    const toggleProviderInstance = async (instanceId) => {
      try {
        togglingInstances.value.add(instanceId);
        
        const result = await api.toggleProviderInstance(instanceId);
        
        // Update local state
        if (providerInstances.value[instanceId]) {
          providerInstances.value[instanceId].active = result.data.active;
        }
        
        const action = result.data.active ? "activated" : "deactivated";
        showSuccess(`Provider instance ${action} successfully`);
        
        // Refresh routing data to update available providers
        await refreshProviderData();
        
      } catch (error) {
        console.error("Error toggling provider instance:", error);
        showError("Failed to toggle provider instance", "Toggle Error");
      } finally {
        togglingInstances.value.delete(instanceId);
      }
    };

    const editProviderInstance = (instanceId) => {
      const instance = providerInstances.value[instanceId];
      if (!instance) return;
      
      editingInstance.value = instanceId;
      
      // Initialize credentials object with empty values for required fields and defaults for optional fields
      const credentialFields = providerTypes.value[instance.provider_type]?.credential_fields[instance.account_type] || [];
      const credentials = {};
      credentialFields.forEach(field => {
        if (field.required) {
          credentials[field.name] = ''; // Empty for security, user must re-enter required fields
        } else {
          credentials[field.name] = field.default || ''; // Use default for optional fields
        }
      });
      
      newProvider.value = {
        provider_type: instance.provider_type,
        account_type: instance.account_type,
        display_name: instance.display_name,
        credentials: credentials
      };
      
      dialogStep.value = 3; // Skip to credentials step for editing
      showAddProviderDialog.value = true;
    };

    const confirmDeleteInstance = (instanceId) => {
      instanceToDelete.value = providerInstances.value[instanceId];
      showDeleteDialog.value = true;
    };

    const deleteProviderInstance = async () => {
      if (!instanceToDelete.value) return;
      
      try {
        deletingInstance.value = true;
        
        await api.deleteProviderInstance(instanceToDelete.value.instance_id);
        
        // Remove from local state
        delete providerInstances.value[instanceToDelete.value.instance_id];
        
        showSuccess("Provider instance deleted successfully");
        showDeleteDialog.value = false;
        instanceToDelete.value = null;
        
        // Refresh routing data
        await refreshProviderData();
        
      } catch (error) {
        console.error("Error deleting provider instance:", error);
        showError("Failed to delete provider instance", "Delete Error");
      } finally {
        deletingInstance.value = false;
      }
    };

    const selectProviderType = (providerType) => {
      newProvider.value.provider_type = providerType;
      newProvider.value.account_type = '';
      newProvider.value.credentials = {};
    };

    const selectAccountType = (accountType) => {
      newProvider.value.account_type = accountType;
      newProvider.value.credentials = {};
    };

    const getSelectedProviderAccountTypes = () => {
      if (!newProvider.value.provider_type || !providerTypes.value[newProvider.value.provider_type]) {
        return [];
      }
      return providerTypes.value[newProvider.value.provider_type].supports_account_types;
    };

    const getCredentialFields = () => {
      if (!newProvider.value.provider_type || !newProvider.value.account_type || !providerTypes.value[newProvider.value.provider_type]) {
        return [];
      }
      
      const providerType = providerTypes.value[newProvider.value.provider_type];
      const fields = providerType.credential_fields[newProvider.value.account_type] || [];
      
      return fields;
    };

    const canProceedToNextStep = computed(() => {
      if (dialogStep.value === 1) {
        return !!newProvider.value.provider_type;
      }
      if (dialogStep.value === 2) {
        return !!newProvider.value.account_type;
      }
      return false;
    });

    const canTestConnection = computed(() => {
      const fields = getCredentialFields();
      return fields.every(field => !field.required || newProvider.value.credentials[field.name]);
    });

    const canSaveProvider = computed(() => {
      return canTestConnection.value && newProvider.value.display_name;
    });

    const testConnection = async () => {
      try {
        testingConnection.value = true;
        connectionTestResult.value = null;
        
        const result = await api.testProviderConnection({
          provider_type: newProvider.value.provider_type,
          account_type: newProvider.value.account_type,
          credentials: newProvider.value.credentials
        });
        
        connectionTestResult.value = result;
        
      } catch (error) {
        console.error("Error testing connection:", error);
        connectionTestResult.value = {
          success: false,
          message: "Connection test failed"
        };
      } finally {
        testingConnection.value = false;
      }
    };

    const saveProvider = async () => {
      try {
        savingProvider.value = true;
        
        if (editingInstance.value) {
          // Update existing instance
          await api.updateProviderInstance(editingInstance.value, {
            display_name: newProvider.value.display_name,
            credentials: newProvider.value.credentials
          });
          
          showSuccess("Provider instance updated successfully");
        } else {
          // Create new instance
          const result = await api.createProviderInstance(newProvider.value);
          
          showSuccess("Provider instance created successfully");
        }
        
        // Reload data
        await loadProviderInstances();
        await refreshProviderData();
        
        closeDialog();
        
      } catch (error) {
        console.error("Error saving provider:", error);
        showError("Failed to save provider instance", "Save Error");
      } finally {
        savingProvider.value = false;
      }
    };

    const closeDialog = () => {
      showAddProviderDialog.value = false;
      dialogStep.value = 1;
      editingInstance.value = null;
      connectionTestResult.value = null;
      newProvider.value = {
        provider_type: '',
        account_type: '',
        display_name: '',
        credentials: {}
      };
    };

    // Service routing methods (from original ProvidersTab)
    const getAvailableInstancesForService = (serviceKey) => {
      const providers = [];
      
      Object.entries(availableProviders.value).forEach(([providerName, providerData]) => {
        const capabilities = providerData.capabilities;
        
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

    const onProviderChange = async (serviceKey, newProvider) => {
      if (!newProvider || updatingRouting.value) return;
      
      try {
        updatingRouting.value = true;
        
        // Update the configuration
        const updatedConfig = { ...currentConfig.value };
        updatedConfig[serviceKey] = newProvider;
        
        // Send to backend through smart data system
        await updateProviderConfig(updatedConfig);
        
        // Set last saved timestamp
        lastSaved.value = new Date();
        
        // Validate new configuration
        validateConfiguration();
        
        showSuccess(
          `${serviceKey.replace('_', ' ')} provider updated to ${formatProviderName(newProvider, availableProviders.value[newProvider])}`,
          "Provider Updated"
        );
        
      } catch (err) {
        console.error("Error updating provider:", err);
        showError("Failed to update provider configuration", "Update Error");
        
        // Refresh data to revert any UI changes
        await loadRoutingData();
      } finally {
        updatingRouting.value = false;
      }
    };

    const validateConfiguration = () => {
      const warnings = [];
      
      // Only validate if we have provider data loaded
      if (!availableProviders.value || Object.keys(availableProviders.value).length === 0) {
        return;
      }
      
      Object.entries(currentConfig.value).forEach(([serviceKey, providerName]) => {
        const availableProvidersForService = getAvailableInstancesForService(serviceKey);
        
        // Check if selected provider is still available
        if (!availableProvidersForService.find(p => p.value === providerName)) {
          warnings.push(`${serviceKey.replace('_', ' ')} provider "${formatProviderName(providerName, availableProviders.value[providerName])}" is no longer available`);
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

    const formatTime = (date) => {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };

    // Load data on mount
    onMounted(async () => {
      await Promise.all([
        loadProviderInstances(),
        loadRoutingData()
      ]);
    });

    // Watch for changes in provider data and auto-validate
    watch([reactiveAvailableProviders, reactiveProviderConfig], () => {
      validateConfiguration();
    }, { deep: true });

    return {
      // Sub-tab management
      activeSubTab,
      subTabs,
      setActiveSubTab,
      
      // Provider instances data
      providerInstances,
      providerTypes,
      instancesLoading,
      instancesError,
      togglingInstances,
      
      // Service routing data
      availableProviders,
      currentConfig,
      routingLoading,
      routingError,
      updatingRouting,
      validationWarnings,
      lastSaved,
      
      // Dialog management
      showAddProviderDialog,
      showDeleteDialog,
      dialogStep,
      editingInstance,
      instanceToDelete,
      savingProvider,
      deletingInstance,
      testingConnection,
      connectionTestResult,
      newProvider,
      
      // Static data
      serviceCategories,
      
      // Methods
      loadProviderInstances,
      loadRoutingData,
      getProviderIcon,
      getProviderTypeName,
      toggleProviderInstance,
      editProviderInstance,
      confirmDeleteInstance,
      deleteProviderInstance,
      selectProviderType,
      selectAccountType,
      getSelectedProviderAccountTypes,
      getCredentialFields,
      canProceedToNextStep,
      canTestConnection,
      canSaveProvider,
      testConnection,
      saveProvider,
      closeDialog,
      getAvailableInstancesForService,
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

/* Sub-tab Navigation */
.sub-tab-nav {
  display: flex;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-xl);
  border-bottom: 1px solid var(--border-primary);
}

.sub-tab-btn {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md) var(--spacing-lg);
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-normal);
  border-bottom: 2px solid transparent;
}

.sub-tab-btn:hover {
  color: var(--text-primary);
  background-color: var(--bg-tertiary);
}

.sub-tab-btn.active {
  color: var(--color-brand);
  border-bottom-color: var(--color-brand);
}

.sub-tab-btn i {
  font-size: var(--font-size-md);
}

/* Tab Content */
.sub-tab-content {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.tab-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-xl);
  padding-bottom: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.header-content h2 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.header-content p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}

.add-provider-btn {
  flex-shrink: 0;
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

/* Provider Instances */
.provider-instances {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.provider-instance {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: var(--spacing-lg);
  padding: var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
  min-width: 0; /* Allow flex items to shrink */
  overflow: hidden; /* Prevent content overflow */
}

.provider-instance:hover {
  border-color: var(--border-tertiary);
}

.provider-instance.inactive {
  opacity: 0.6;
}

.instance-main-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  min-width: 0; /* Allow content to shrink */
}

.instance-status {
  display: flex;
  justify-content: center;
  min-width: 80px; /* Fixed width for alignment */
}

.instance-icon {
  width: 56px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  color: var(--color-primary);
}

.instance-icon i {
  font-size: var(--font-size-lg);
}

.provider-logo {
  width: 48px;
  height: 28px;
  object-fit: contain;
  filter: brightness(0) saturate(100%) invert(58%) sepia(69%) saturate(2618%) hue-rotate(346deg) brightness(101%) contrast(101%);
}

.instance-details {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.instance-name {
  margin: 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.instance-meta {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.separator {
  color: var(--text-tertiary);
}

.account-type.live {
  color: var(--color-danger);
  font-weight: var(--font-weight-semibold);
}

.account-type.paper {
  color: var(--color-info);
  font-weight: var(--font-weight-semibold);
}

.instance-badges {
  display: flex;
  gap: var(--spacing-sm);
}

.status-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
}

.status-badge.active {
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
  border: 1px solid var(--color-success);
}

.status-badge.inactive {
  background-color: rgba(255, 187, 51, 0.1);
  color: var(--color-warning);
  border: 1px solid var(--color-warning);
}

.instance-actions {
  display: flex;
  gap: var(--spacing-xs);
  flex-shrink: 0; /* Prevent actions from shrinking */
  min-width: fit-content; /* Ensure buttons don't get cut off */
}

/* Empty State */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-xxl);
  text-align: center;
  color: var(--text-secondary);
}

.empty-state i {
  font-size: 3rem;
  margin-bottom: var(--spacing-lg);
  color: var(--text-tertiary);
}

.empty-state h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.empty-state p {
  margin: 0 0 var(--spacing-lg) 0;
  font-size: var(--font-size-md);
  color: var(--text-tertiary);
}

/* Service Configuration (from original ProvidersTab) */
.service-config {
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

/* Dialog Styles */
.provider-dialog :deep(.p-dialog) {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
}

.dialog-content {
  padding: var(--spacing-lg) 0;
}

.dialog-step h4 {
  margin: 0 0 var(--spacing-lg) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

/* Provider Type Grid */
.provider-type-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-md);
}

.provider-type-card {
  padding: var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border: 2px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: var(--transition-normal);
}

.provider-type-card:hover {
  border-color: var(--border-tertiary);
}

.provider-type-card.selected {
  border-color: var(--color-brand);
  background-color: rgba(255, 107, 53, 0.05);
}

.provider-type-card .provider-icon {
  width: 48px;
  height: 48px;
  margin-bottom: var(--spacing-md);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-primary);
}

.provider-type-card .provider-icon i {
  font-size: var(--font-size-xl);
}

.provider-type-card .provider-logo {
  width: 32px;
  height: 32px;
  object-fit: contain;
  filter: brightness(0) saturate(100%) invert(58%) sepia(69%) saturate(2618%) hue-rotate(346deg) brightness(101%) contrast(101%);
}

.provider-type-card h5 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.provider-type-card p {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.supported-accounts {
  display: flex;
  gap: var(--spacing-xs);
}

.account-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
}

.account-badge.live {
  background-color: rgba(220, 38, 127, 0.1);
  color: var(--color-danger);
  border: 1px solid var(--color-danger);
}

.account-badge.paper {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  border: 1px solid var(--color-info);
}

/* Account Type Selection */
.account-type-selection {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-md);
}

.account-type-card {
  position: relative;
  padding: var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border: 2px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: var(--transition-normal);
}

.account-type-card:hover {
  border-color: var(--border-tertiary);
}

.account-type-card.selected {
  border-color: var(--color-brand);
  background-color: rgba(255, 107, 53, 0.05);
}

.account-type-card .account-icon {
  width: 40px;
  height: 40px;
  margin-bottom: var(--spacing-md);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-primary);
}

.account-type-card .account-icon i {
  font-size: var(--font-size-lg);
}

.account-type-card h5 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.account-type-card p {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.warning-badge {
  position: absolute;
  top: var(--spacing-sm);
  right: var(--spacing-sm);
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  background-color: rgba(220, 38, 127, 0.1);
  color: var(--color-danger);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
}

/* Credentials Form */
.credentials-form {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.form-group label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.field-hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-style: italic;
}

.test-connection {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-secondary);
}

.test-result {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-md);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}

.test-result.success {
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
  border: 1px solid var(--color-success);
}

.test-result.error {
  background-color: rgba(220, 38, 127, 0.1);
  color: var(--color-danger);
  border: 1px solid var(--color-danger);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-sm);
  padding-top: var(--spacing-lg);
  border-top: 1px solid var(--border-secondary);
}

/* Delete Dialog */
.delete-content {
  text-align: center;
  padding: var(--spacing-lg);
}

.warning-icon {
  font-size: 3rem;
  color: var(--color-danger);
  margin-bottom: var(--spacing-lg);
}

.delete-content p {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

.warning-text {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  font-style: italic;
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
