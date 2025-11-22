<template>
  <div class="providers-step">
    <div class="step-header">
      <h2 class="step-title">Add Provider Instances</h2>
      <p class="step-description">
        Connect your trading accounts and data providers. You'll need at least one provider 
        that supports the mandatory services to continue.
      </p>
    </div>

    <div class="providers-content">
      <!-- Provider Instances Management - Reusing ProvidersTab logic -->
      <div class="provider-instances-section">
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
          <div class="instances-header">
            <h3>Your Provider Instances</h3>
            <Button
              label="Add Provider"
              icon="pi pi-plus"
              @click="showAddProviderDialog = true"
              class="add-provider-btn p-button-brand"
            />
          </div>

          <div class="instances-list">
            <div 
              v-for="(instance, instanceId) in providerInstances" 
              :key="instanceId" 
              class="provider-instance"
              :class="{ 'inactive': !instance.active }"
            >
              <!-- Provider Info -->
              <div class="instance-main-info">
                <div class="instance-icon">
                  <img 
                    v-if="!getProviderIcon(instance.provider_type)" 
                    :src="`/logos/${instance.provider_type}.svg`"
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
                  <!-- Show supported mandatory services -->
                  <div class="supported-services">
                    <span 
                      v-for="service in getSupportedMandatoryServices(instance)"
                      :key="service"
                      class="service-tag"
                    >
                      {{ formatServiceName(service) }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- Status and Actions -->
              <div class="instance-actions">
                <div class="instance-status">
                  <span class="status-badge" :class="{ 'active': instance.active, 'inactive': !instance.active }">
                    {{ instance.active ? 'Active' : 'Inactive' }}
                  </span>
                </div>
                
                <div class="action-buttons">
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
          </div>
        </div>

        <!-- Empty State -->
        <div v-else class="empty-state">
          <i class="pi pi-server"></i>
          <h4>No Provider Instances</h4>
          <p>Add your first provider instance to continue with the setup</p>
          <Button
            label="Add Provider"
            icon="pi pi-plus"
            @click="showAddProviderDialog = true"
            class="p-button-primary"
          />
        </div>

        <!-- Mandatory Services Coverage -->
        <div v-if="Object.keys(providerInstances).length > 0" class="coverage-analysis">
          <h3>Mandatory Services Coverage</h3>
          <div class="coverage-grid">
            <div 
              v-for="service in mandatoryServices"
              :key="service.key"
              class="coverage-item"
              :class="{ 'covered': isMandatoryServiceCovered(service.key) }"
            >
              <div class="coverage-icon">
                <i :class="service.icon"></i>
              </div>
              <div class="coverage-details">
                <span class="service-name">{{ service.name }}</span>
                <span class="coverage-status">
                  {{ isMandatoryServiceCovered(service.key) ? 'Covered' : 'Not Covered' }}
                </span>
              </div>
              <div class="coverage-indicator">
                <i :class="isMandatoryServiceCovered(service.key) ? 'pi pi-check-circle' : 'pi pi-times-circle'"></i>
              </div>
            </div>
          </div>
          
          <div v-if="!allMandatoryServicesCovered" class="coverage-warning">
            <i class="pi pi-exclamation-triangle"></i>
            <span>You need to add providers that cover all mandatory services to continue.</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Add/Edit Provider Dialog - Reusing ProvidersTab dialog -->
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
                  :src="`/logos/${key}.svg`" 
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
                <!-- Show which mandatory services this provider supports -->
                <div class="mandatory-services-support">
                  <span class="support-label">Supports:</span>
                  <div class="supported-mandatory-services">
                    <span 
                      v-for="service in getMandatoryServicesForProvider(key)"
                      :key="service"
                      class="mandatory-service-tag"
                    >
                      {{ formatServiceName(service) }}
                    </span>
                  </div>
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
              <div class="account-icon" :class="accountType">
                <i :class="accountType === 'live' ? 'pi pi-dollar' : 'pi pi-file-edit'"></i>
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
              <MaskedCredentialInput
                v-model="newProvider.credentials[field.name]"
                :input-type="field.type"
                :placeholder="field.placeholder"
                :is-sensitive="isSensitiveField(field.name)"
                :has-existing-value="hasExistingValue(field.name)"
                :original-value="getOriginalValue(field.name)"
                :default-value="field.default || ''"
                @change="onCredentialFieldChange(field.name, $event)"
              />
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
import { ref, computed, onMounted, watch } from 'vue';
import Button from 'primevue/button';
import Dialog from 'primevue/dialog';
import InputText from 'primevue/inputtext';
import MaskedCredentialInput from '../settings/MaskedCredentialInput.vue';
import { useNotifications } from '../../composables/useNotifications';
import api from '../../services/api';

export default {
  name: 'WizardStep2Providers',
  components: {
    Button,
    Dialog,
    InputText,
    MaskedCredentialInput
  },
  props: {
    wizardData: {
      type: Object,
      required: true
    }
  },
  emits: ['update-data'],
  setup(props, { emit }) {
    const { showSuccess, showError } = useNotifications();

    // Provider instances data - reusing ProvidersTab logic
    const providerInstances = ref({});
    const providerTypes = ref({});
    const instancesLoading = ref(false);
    const instancesError = ref(null);
    const togglingInstances = ref(new Set());

    // Dialog management - reusing ProvidersTab logic
    const showAddProviderDialog = ref(false);
    const showDeleteDialog = ref(false);
    const dialogStep = ref(1);
    const editingInstance = ref(null);
    const instanceToDelete = ref(null);
    const savingProvider = ref(false);
    const deletingInstance = ref(false);
    const testingConnection = ref(false);
    const connectionTestResult = ref(null);

    // New provider form data - reusing ProvidersTab logic
    const newProvider = ref({
      provider_type: '',
      account_type: '',
      display_name: '',
      credentials: {}
    });

    // Mandatory services that must be covered
    const mandatoryServices = [
      { key: 'trade_account', name: 'Trade Account', icon: 'pi pi-wallet' },
      { key: 'options_chain', name: 'Options Chain', icon: 'pi pi-link' },
      { key: 'historical_data', name: 'Historical Data', icon: 'pi pi-chart-bar' },
      { key: 'symbol_lookup', name: 'Symbol Lookup', icon: 'pi pi-search' },
      { key: 'streaming_quotes', name: 'Streaming Quotes', icon: 'pi pi-bolt' }
    ];

    // Computed properties
    const allMandatoryServicesCovered = computed(() => {
      return mandatoryServices.every(service => isMandatoryServiceCovered(service.key));
    });

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
      if (editingInstance.value) {
        const fields = getCredentialFields();
        const hasRequiredFields = fields.every(field => {
          if (!field.required) return true;
          if (isSensitiveField(field.name)) {
            return true;
          }
          return newProvider.value.credentials[field.name];
        });
        return hasRequiredFields && newProvider.value.display_name;
      }
      return canTestConnection.value && newProvider.value.display_name;
    });

    // Methods - reusing ProvidersTab logic
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
        
        // Update wizard data
        emit('update-data', {
          providers: instancesData,
          providerTypes: typesData
        });
        
      } catch (error) {
        console.error("Error loading provider instances:", error);
        instancesError.value = "Failed to load provider instances";
      } finally {
        instancesLoading.value = false;
      }
    };

    const getProviderIcon = (providerType) => {
      const svgProviders = ['alpaca', 'tradier', 'public', 'tastytrade'];
      if (svgProviders.includes(providerType)) {
        return null;
      }
      return "pi pi-server";
    };

    const getProviderTypeName = (providerType) => {
      return providerTypes.value[providerType]?.name || providerType;
    };

    const getSupportedMandatoryServices = (instance) => {
      const providerType = providerTypes.value[instance.provider_type];
      if (!providerType) return [];
      
      const capabilities = providerType.capabilities || {};
      const allCapabilities = [
        ...(capabilities.rest || []),
        ...(capabilities.streaming || [])
      ];
      
      return mandatoryServices
        .filter(service => allCapabilities.includes(service.key))
        .map(service => service.key);
    };

    const getMandatoryServicesForProvider = (providerTypeKey) => {
      const providerType = providerTypes.value[providerTypeKey];
      if (!providerType) return [];
      
      const capabilities = providerType.capabilities || {};
      const allCapabilities = [
        ...(capabilities.rest || []),
        ...(capabilities.streaming || [])
      ];
      
      return mandatoryServices
        .filter(service => allCapabilities.includes(service.key))
        .map(service => service.key);
    };

    const isMandatoryServiceCovered = (serviceKey) => {
      return Object.values(providerInstances.value).some(instance => {
        if (!instance.active) return false;
        const supportedServices = getSupportedMandatoryServices(instance);
        return supportedServices.includes(serviceKey);
      });
    };

    const formatServiceName = (serviceKey) => {
      const service = mandatoryServices.find(s => s.key === serviceKey);
      return service ? service.name : serviceKey.replace('_', ' ');
    };

    const toggleProviderInstance = async (instanceId) => {
      try {
        togglingInstances.value.add(instanceId);
        
        const result = await api.toggleProviderInstance(instanceId);
        
        if (providerInstances.value[instanceId]) {
          providerInstances.value[instanceId].active = result.data.active;
        }
        
        const action = result.data.active ? "activated" : "deactivated";
        showSuccess(`Provider instance ${action} successfully`);
        
        // Update wizard data
        emit('update-data', { providers: providerInstances.value });
        
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
      
      const credentialFields = providerTypes.value[instance.provider_type]?.credential_fields[instance.account_type] || [];
      const credentials = {};
      
      credentialFields.forEach(field => {
        if (isSensitiveField(field.name)) {
          credentials[field.name] = '';
        } else {
          credentials[field.name] = instance.visible_credentials?.[field.name] || 
                                   instance.default_credentials?.[field.name] || 
                                   field.default || '';
        }
      });
      
      newProvider.value = {
        provider_type: instance.provider_type,
        account_type: instance.account_type,
        display_name: instance.display_name,
        credentials: credentials
      };
      
      dialogStep.value = 3;
      showAddProviderDialog.value = true;
    };

    const confirmDeleteInstance = (instanceId) => {
      instanceToDelete.value = {
        instance_id: instanceId,
        ...providerInstances.value[instanceId]
      };
      showDeleteDialog.value = true;
    };

    const deleteProviderInstance = async () => {
      if (!instanceToDelete.value) return;
      
      try {
        deletingInstance.value = true;
        
        await api.deleteProviderInstance(instanceToDelete.value.instance_id);
        
        delete providerInstances.value[instanceToDelete.value.instance_id];
        
        showSuccess("Provider instance deleted successfully");
        showDeleteDialog.value = false;
        instanceToDelete.value = null;
        
        // Update wizard data
        emit('update-data', { providers: providerInstances.value });
        
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
      
      if (!editingInstance.value) {
        const credentialFields = providerTypes.value[newProvider.value.provider_type]?.credential_fields[accountType] || [];
        const credentials = {};
        
        credentialFields.forEach(field => {
          if (field.default) {
            credentials[field.name] = field.default;
          } else {
            credentials[field.name] = '';
          }
        });
        
        newProvider.value.credentials = credentials;
      } else {
        newProvider.value.credentials = {};
      }
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
        
        await testConnection();
        
        if (!connectionTestResult.value?.success) {
          showError("Connection test failed. Please check your credentials.", "Save Error");
          return;
        }

        if (editingInstance.value) {
          await api.updateProviderInstance(editingInstance.value, {
            display_name: newProvider.value.display_name,
            credentials: newProvider.value.credentials
          });
          
          showSuccess("Provider instance updated successfully");
        } else {
          await api.createProviderInstance(newProvider.value);
          showSuccess("Provider instance created successfully");
        }
        
        await loadProviderInstances();
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

    const isSensitiveField = (fieldName) => {
      const sensitiveFields = ['password', 'api_key', 'api_secret'];
      return sensitiveFields.includes(fieldName);
    };

    const hasExistingValue = (fieldName) => {
      if (!editingInstance.value) return false;
      
      const instance = providerInstances.value[editingInstance.value];
      if (!instance) return false;
      
      if (isSensitiveField(fieldName)) {
        return instance.masked_credentials?.[fieldName] || false;
      } else {
        return !!(instance.visible_credentials?.[fieldName]);
      }
    };

    const getOriginalValue = (fieldName) => {
      if (!editingInstance.value) return '';
      
      const instance = providerInstances.value[editingInstance.value];
      if (!instance) return '';
      
      if (isSensitiveField(fieldName)) {
        return '';
      } else {
        return instance.visible_credentials?.[fieldName] || 
               instance.default_credentials?.[fieldName] || '';
      }
    };

    const onCredentialFieldChange = (fieldName, event) => {
      console.log(`Field ${fieldName} changed:`, event);
    };

    // Watch for changes and emit to parent
    watch([providerInstances, allMandatoryServicesCovered], () => {
      emit('update-data', {
        providers: providerInstances.value,
        allMandatoryServicesCovered: allMandatoryServicesCovered.value
      });
    }, { deep: true });

    // Load data on mount
    onMounted(() => {
      loadProviderInstances();
    });

    return {
      // State
      providerInstances,
      providerTypes,
      instancesLoading,
      instancesError,
      togglingInstances,
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
      mandatoryServices,
      
      // Computed
      allMandatoryServicesCovered,
      canProceedToNextStep,
      canTestConnection,
      canSaveProvider,
      
      // Methods
      loadProviderInstances,
      getProviderIcon,
      getProviderTypeName,
      getSupportedMandatoryServices,
      getMandatoryServicesForProvider,
      isMandatoryServiceCovered,
      formatServiceName,
      toggleProviderInstance,
      editProviderInstance,
      confirmDeleteInstance,
      deleteProviderInstance,
      selectProviderType,
      selectAccountType,
      getSelectedProviderAccountTypes,
      getCredentialFields,
      testConnection,
      saveProvider,
      closeDialog,
      isSensitiveField,
      hasExistingValue,
      getOriginalValue,
      onCredentialFieldChange
    };
  }
};
</script>

<style scoped>
.providers-step {
  max-width: 900px;
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

.providers-content {
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

/* Provider Instances */
.provider-instances-section {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
}

.instances-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
}

.instances-header h3 {
  margin: 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.instances-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.provider-instance {
  display: flex;
  align-items: center;
  gap: var(--spacing-lg);
  padding: var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
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
  flex: 1;
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
  flex: 1;
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

.supported-services {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
  margin-top: var(--spacing-xs);
}

.service-tag {
  padding: var(--spacing-xs) var(--spacing-sm);
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
  border: 1px solid var(--color-success);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
}

.instance-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--spacing-sm);
}

.instance-status {
  display: flex;
  justify-content: center;
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

.action-buttons {
  display: flex;
  gap: var(--spacing-xs);
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
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
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

/* Coverage Analysis */
.coverage-analysis {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  margin-top: var(--spacing-lg);
}

.coverage-analysis h3 {
  margin: 0 0 var(--spacing-lg) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.coverage-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-lg);
}

.coverage-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.coverage-item.covered {
  border-color: var(--color-success);
  background-color: rgba(0, 200, 81, 0.05);
}

.coverage-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-secondary);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.coverage-item.covered .coverage-icon {
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
}

.coverage-details {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.service-name {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.coverage-status {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.coverage-item.covered .coverage-status {
  color: var(--color-success);
}

.coverage-indicator {
  font-size: var(--font-size-lg);
}

.coverage-indicator .pi-check-circle {
  color: var(--color-success);
}

.coverage-indicator .pi-times-circle {
  color: var(--color-danger);
}

.coverage-warning {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  background-color: rgba(255, 187, 51, 0.1);
  border: 1px solid var(--color-warning);
  border-radius: var(--radius-md);
  color: var(--text-primary);
}

.coverage-warning i {
  color: var(--color-warning);
}

/* Dialog Styles - Reusing from ProvidersTab */
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
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
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

.provider-info h5 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.provider-info p {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.supported-accounts {
  display: flex;
  gap: var(--spacing-xs);
  margin-bottom: var(--spacing-md);
}

.account-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
}

.account-badge.live {
  background-color: rgba(255, 187, 51, 0.1);
  color: var(--color-warning);
  border: 1px solid var(--color-warning);
}

.account-badge.paper {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  border: 1px solid var(--color-info);
}

.mandatory-services-support {
  margin-top: var(--spacing-sm);
}

.support-label {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-medium);
  display: block;
  margin-bottom: var(--spacing-xs);
}

.supported-mandatory-services {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
}

.mandatory-service-tag {
  padding: var(--spacing-xs) var(--spacing-sm);
  background-color: rgba(255, 107, 53, 0.1);
  color: var(--color-brand);
  border: 1px solid var(--color-brand);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
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
  background-color: rgba(255, 107, 53, 0.1);
  border: 1px solid var(--color-brand);
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-brand);
}

.account-type-card .account-icon i {
  font-size: var(--font-size-lg);
}

.account-info h5 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.account-info p {
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
  background-color: rgba(255, 107, 53, 0.1);
  color: var(--color-brand);
  border: 1px solid var(--color-brand);
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

/* Responsive Design */
@media (max-width: 768px) {
  .provider-type-grid {
    grid-template-columns: 1fr;
  }
  
  .coverage-grid {
    grid-template-columns: 1fr;
  }
  
  .provider-instance {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-md);
  }
  
  .instance-actions {
    flex-direction: row;
    align-items: center;
    width: 100%;
    justify-content: space-between;
  }
  
  .action-buttons {
    flex-wrap: wrap;
  }
}
</style>
