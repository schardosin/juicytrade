<template>
  <div class="setup-wizard">
    <!-- Header -->
    <div class="wizard-header">
      <div class="header-content">
        <h1 class="wizard-title">JuicyTrade Setup</h1>
        <p class="wizard-subtitle">Configure your trading providers to get started</p>
      </div>
      
      <!-- Progress Indicator -->
      <div class="progress-indicator">
        <div class="progress-bar">
          <div 
            class="progress-fill" 
            :style="{ width: `${progressPercentage}%` }"
          ></div>
        </div>
        <div class="step-indicators">
          <div 
            v-for="(step, index) in steps" 
            :key="index"
            class="step-indicator"
            :class="{
              'active': index === currentStep,
              'completed': index < currentStep,
              'disabled': index > currentStep
            }"
          >
            <div class="step-circle">
              <i v-if="index < currentStep" class="pi pi-check"></i>
              <span v-else>{{ index + 1 }}</span>
            </div>
            <span class="step-label">{{ step.label }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Step Content -->
    <div class="wizard-content">
      <component 
        :is="currentStepComponent" 
        :wizard-data="wizardData"
        @next="handleNext"
        @previous="handlePrevious"
        @update-data="handleDataUpdate"
        @skip="handleSkip"
      />
    </div>

    <!-- Footer -->
    <div class="wizard-footer">
      <div class="footer-actions">
        <Button
          v-if="currentStep > 0"
          label="Previous"
          icon="pi pi-arrow-left"
          @click="handlePrevious"
          class="p-button-secondary"
          :disabled="isProcessing"
        />
        
        <div class="spacer"></div>
        
        <Button
          v-if="canSkipStep"
          label="Skip"
          @click="handleSkip"
          class="p-button-text"
          :disabled="isProcessing"
        />
        
        <Button
          v-if="!isLastStep"
          :label="nextButtonLabel"
          icon="pi pi-arrow-right"
          icon-pos="right"
          @click="handleNext"
          :disabled="!canProceed || isProcessing"
          :loading="isProcessing"
        />
        
        <Button
          v-if="isLastStep"
          label="Complete Setup"
          icon="pi pi-check"
          @click="handleComplete"
          class="p-button-success"
          :disabled="!canComplete || isProcessing"
          :loading="isProcessing"
        />
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import Button from 'primevue/button';
import { useNotifications } from '../../composables/useNotifications';
import api from '../../services/api';

// Step Components
import WizardStep1Welcome from './WizardStep1Welcome.vue';
import WizardStep2Providers from './WizardStep2Providers.vue';
import WizardStep3MandatoryRoutes from './WizardStep3MandatoryRoutes.vue';
import WizardStep4OptionalRoutes from './WizardStep4OptionalRoutes.vue';
import WizardStep5Complete from './WizardStep5Complete.vue';

export default {
  name: 'SetupWizard',
  components: {
    Button,
    WizardStep1Welcome,
    WizardStep2Providers,
    WizardStep3MandatoryRoutes,
    WizardStep4OptionalRoutes,
    WizardStep5Complete
  },
  setup() {
    const router = useRouter();
    const { showSuccess, showError } = useNotifications();

    // Wizard state
    const currentStep = ref(0);
    const isProcessing = ref(false);
    
    // Wizard data - shared across all steps
    const wizardData = ref({
      providers: {},
      providerTypes: {},
      mandatoryRoutes: {},
      optionalRoutes: {},
      validationResults: {},
      allMandatoryServicesConfigured: false,
      isComplete: false
    });

    // Step definitions
    const steps = [
      { 
        label: 'Welcome', 
        component: 'WizardStep1Welcome',
        canSkip: false,
        required: true
      },
      { 
        label: 'Providers', 
        component: 'WizardStep2Providers',
        canSkip: false,
        required: true
      },
      { 
        label: 'Mandatory Routes', 
        component: 'WizardStep3MandatoryRoutes',
        canSkip: false,
        required: true
      },
      { 
        label: 'Optional Routes', 
        component: 'WizardStep4OptionalRoutes',
        canSkip: true,
        required: false
      },
      { 
        label: 'Complete', 
        component: 'WizardStep5Complete',
        canSkip: false,
        required: true
      }
    ];

    // Mandatory routes that must be configured
    const mandatoryRoutes = [
      'trade_account',
      'options_chain', 
      'historical_data',
      'symbol_lookup',
      'streaming_quotes'
    ];

    // Computed properties
    const currentStepComponent = computed(() => steps[currentStep.value].component);
    const isLastStep = computed(() => currentStep.value === steps.length - 1);
    const canSkipStep = computed(() => steps[currentStep.value].canSkip);
    const progressPercentage = computed(() => ((currentStep.value + 1) / steps.length) * 100);
    
    const nextButtonLabel = computed(() => {
      if (currentStep.value === 0) return 'Get Started';
      if (currentStep.value === steps.length - 2) return 'Finish';
      return 'Next';
    });

    const canProceed = computed(() => {
      if (isProcessing.value) return false;
      
      // Step-specific validation
      switch (currentStep.value) {
        case 0: // Welcome
          return true;
          
        case 1: // Providers
          // Check if at least one provider is configured
          return Object.keys(wizardData.value.providers).length > 0;
          
        case 2: // Mandatory Routes
          // Check if all mandatory routes are configured using the same logic as status icons
          return mandatoryRoutes.every(route => {
            const value = wizardData.value.mandatoryRoutes[route];
            return value && value !== null && value !== '' && value !== undefined;
          });
          
        case 3: // Optional Routes
          return true; // Always valid since it's optional
          
        case 4: // Complete
          return canComplete.value;
          
        default:
          return true;
      }
    });

    const canComplete = computed(() => {
      // Check if all mandatory routes are configured
      return mandatoryRoutes.every(route => 
        wizardData.value.mandatoryRoutes[route] && 
        wizardData.value.mandatoryRoutes[route] !== null
      );
    });

    // Methods
    const handleNext = async () => {
      if (currentStep.value < steps.length - 1) {
        isProcessing.value = true;
        
        try {
          // Validate current step before proceeding
          const isValid = await validateCurrentStep();
          
          if (isValid) {
            currentStep.value++;
          }
        } catch (error) {
          console.error('Error validating step:', error);
          showError('Failed to validate step. Please check your configuration.', 'Validation Error');
        } finally {
          isProcessing.value = false;
        }
      }
    };

    const handlePrevious = () => {
      if (currentStep.value > 0) {
        currentStep.value--;
      }
    };

    const handleSkip = () => {
      if (canSkipStep.value) {
        handleNext();
      }
    };

    const handleDataUpdate = (data) => {
      // Merge updated data into wizard state
      Object.assign(wizardData.value, data);
    };

    const handleComplete = async () => {
      isProcessing.value = true;
      
      try {
        // Final validation and save configuration
        await saveConfiguration();
        
        showSuccess('Setup completed successfully! Welcome to JuicyTrade.', 'Setup Complete');
        
        // Redirect to main application
        setTimeout(() => {
          router.push('/');
        }, 2000);
        
      } catch (error) {
        console.error('Error completing setup:', error);
        showError('Failed to complete setup. Please try again.', 'Setup Error');
      } finally {
        isProcessing.value = false;
      }
    };

    const validateCurrentStep = async () => {
      // Step-specific validation logic
      switch (currentStep.value) {
        case 0: // Welcome
          return true;
          
        case 1: // Providers
          // Check if at least one provider is configured
          return Object.keys(wizardData.value.providers).length > 0;
          
        case 2: // Mandatory Routes
          // Check if all mandatory routes are configured
          return mandatoryRoutes.every(route => 
            wizardData.value.mandatoryRoutes[route] && 
            wizardData.value.mandatoryRoutes[route] !== null
          );
          
        case 3: // Optional Routes
          return true; // Always valid since it's optional
          
        case 4: // Complete
          return canComplete.value;
          
        default:
          return true;
      }
    };

    const saveConfiguration = async () => {
      try {
        // Save provider configuration
        const config = {
          ...wizardData.value.mandatoryRoutes,
          ...wizardData.value.optionalRoutes
        };
        
        await api.updateProviderConfig(config);
        
        // Mark setup as complete
        wizardData.value.isComplete = true;
        
      } catch (error) {
        console.error('Error saving configuration:', error);
        throw error;
      }
    };

    const loadInitialData = async () => {
      try {
        // Load provider types and existing configuration
        const [providerTypes, existingConfig, providerInstances] = await Promise.all([
          api.getProviderTypes(),
          api.getProviderConfig().catch(() => ({})),
          api.getProviderInstances().catch(() => ({}))
        ]);

        wizardData.value.providerTypes = providerTypes;
        wizardData.value.providers = providerInstances;
        
        // Initialize mandatory routes with null values (user must actively select)
        const mandatory = {};
        mandatoryRoutes.forEach(route => {
          mandatory[route] = null;
        });
        
        // Only load existing config for optional routes
        const optional = {};
        Object.entries(existingConfig).forEach(([key, value]) => {
          if (!mandatoryRoutes.includes(key)) {
            optional[key] = value;
          }
        });
        
        wizardData.value.mandatoryRoutes = mandatory;
        wizardData.value.optionalRoutes = optional;
        
      } catch (error) {
        console.error('Error loading initial data:', error);
        showError('Failed to load configuration data', 'Loading Error');
      }
    };

    // Lifecycle
    onMounted(() => {
      loadInitialData();
    });

    return {
      // State
      currentStep,
      isProcessing,
      wizardData,
      steps,
      
      // Computed
      currentStepComponent,
      isLastStep,
      canSkipStep,
      progressPercentage,
      nextButtonLabel,
      canProceed,
      canComplete,
      
      // Methods
      handleNext,
      handlePrevious,
      handleSkip,
      handleDataUpdate,
      handleComplete
    };
  }
};
</script>

<style scoped>
.setup-wizard {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}

/* Header */
.wizard-header {
  flex-shrink: 0;
  padding: var(--spacing-xl) var(--spacing-xl) var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  background-color: var(--bg-secondary);
}

.header-content {
  text-align: center;
  margin-bottom: var(--spacing-xl);
}

.wizard-title {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-xxl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  background: linear-gradient(135deg, var(--color-brand) 0%, #ff8a50 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.wizard-subtitle {
  margin: 0;
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
}

/* Progress Indicator */
.progress-indicator {
  max-width: 800px;
  margin: 0 auto;
}

.progress-bar {
  width: 100%;
  height: 4px;
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

.step-indicators {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.step-indicator {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-sm);
  flex: 1;
  position: relative;
}

.step-indicator:not(:last-child)::after {
  content: '';
  position: absolute;
  top: 16px;
  left: 60%;
  right: -40%;
  height: 2px;
  background-color: var(--border-secondary);
  z-index: 0;
}

.step-indicator.completed:not(:last-child)::after {
  background-color: var(--color-brand);
}

.step-circle {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  position: relative;
  z-index: 1;
  transition: all 0.3s ease;
}

.step-indicator.disabled .step-circle {
  background-color: var(--bg-tertiary);
  border: 2px solid var(--border-secondary);
  color: var(--text-tertiary);
}

.step-indicator.active .step-circle {
  background-color: var(--color-brand);
  border: 2px solid var(--color-brand);
  color: white;
  box-shadow: 0 0 0 4px rgba(255, 107, 53, 0.2);
}

.step-indicator.completed .step-circle {
  background-color: var(--color-success);
  border: 2px solid var(--color-success);
  color: white;
}

.step-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  text-align: center;
  transition: color 0.3s ease;
}

.step-indicator.disabled .step-label {
  color: var(--text-tertiary);
}

.step-indicator.active .step-label {
  color: var(--color-brand);
  font-weight: var(--font-weight-semibold);
}

.step-indicator.completed .step-label {
  color: var(--color-success);
}

/* Content */
.wizard-content {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-xl);
  background-color: var(--bg-primary);
}

/* Footer */
.wizard-footer {
  flex-shrink: 0;
  padding: var(--spacing-lg) var(--spacing-xl);
  border-top: 1px solid var(--border-primary);
  background-color: var(--bg-secondary);
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.spacer {
  flex: 1;
}

/* Responsive Design */
@media (max-width: 768px) {
  .wizard-header {
    padding: var(--spacing-lg);
  }
  
  .wizard-content {
    padding: var(--spacing-lg);
  }
  
  .wizard-footer {
    padding: var(--spacing-md) var(--spacing-lg);
  }
  
  .step-indicators {
    flex-wrap: wrap;
    gap: var(--spacing-md);
  }
  
  .step-indicator {
    flex: 0 0 auto;
    min-width: 80px;
  }
  
  .step-indicator:not(:last-child)::after {
    display: none;
  }
  
  .wizard-title {
    font-size: var(--font-size-xl);
  }
  
  .wizard-subtitle {
    font-size: var(--font-size-md);
  }
}

/* Custom scrollbar */
.wizard-content::-webkit-scrollbar {
  width: 6px;
}

.wizard-content::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

.wizard-content::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.wizard-content::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}
</style>
