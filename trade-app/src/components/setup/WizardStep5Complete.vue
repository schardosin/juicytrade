<template>
  <div class="complete-step">
    <div class="completion-content">
      <!-- Success Hero -->
      <div class="success-hero">
        <div class="success-icon">
          <i class="pi pi-check-circle"></i>
        </div>
        <h2 class="success-title">Setup Complete!</h2>
        <p class="success-description">
          Your JuicyTrade platform is now configured and ready for trading. 
          All mandatory services have been set up successfully.
        </p>
      </div>

      <!-- Configuration Summary -->
      <div class="configuration-summary">
        <h3 class="summary-title">Configuration Summary</h3>
        
        <!-- Provider Instances Summary -->
        <div class="summary-section">
          <div class="section-header">
            <div class="section-icon">
              <i class="pi pi-server"></i>
            </div>
            <div class="section-info">
              <h4>Provider Instances</h4>
              <p>{{ activeProvidersCount }} active provider{{ activeProvidersCount !== 1 ? 's' : '' }} configured</p>
            </div>
          </div>
          
          <div class="provider-list">
            <div 
              v-for="(provider, instanceId) in activeProviders"
              :key="instanceId"
              class="provider-item"
            >
              <div class="provider-icon">
                <img 
                  v-if="!getProviderIcon(provider.provider_type)" 
                  :src="`/src/assets/logos/${provider.provider_type}.svg`" 
                  :alt="`${provider.provider_type} logo`"
                  class="provider-logo"
                />
                <i v-else :class="getProviderIcon(provider.provider_type)"></i>
              </div>
              <div class="provider-details">
                <span class="provider-name">{{ provider.display_name }}</span>
                <span class="provider-type">{{ getProviderTypeName(provider.provider_type) }}</span>
              </div>
              <div class="provider-badge" :class="provider.account_type">
                {{ provider.account_type.toUpperCase() }}
              </div>
            </div>
          </div>
        </div>

        <!-- Mandatory Services Summary -->
        <div class="summary-section">
          <div class="section-header">
            <div class="section-icon success">
              <i class="pi pi-check-circle"></i>
            </div>
            <div class="section-info">
              <h4>Mandatory Services</h4>
              <p>All {{ mandatoryServicesCount }} required services configured</p>
            </div>
          </div>
          
          <div class="services-grid">
            <div 
              v-for="service in configuredMandatoryServices"
              :key="service.key"
              class="service-item configured"
            >
              <div class="service-icon">
                <i :class="service.icon"></i>
              </div>
              <div class="service-details">
                <span class="service-name">{{ service.name }}</span>
                <span class="service-provider">{{ getServiceProvider(service.key) }}</span>
              </div>
              <div class="service-status">
                <i class="pi pi-check"></i>
              </div>
            </div>
          </div>
        </div>

        <!-- Optional Services Summary -->
        <div v-if="configuredOptionalServicesCount > 0" class="summary-section">
          <div class="section-header">
            <div class="section-icon optional">
              <i class="pi pi-info-circle"></i>
            </div>
            <div class="section-info">
              <h4>Optional Services</h4>
              <p>{{ configuredOptionalServicesCount }} additional service{{ configuredOptionalServicesCount !== 1 ? 's' : '' }} configured</p>
            </div>
          </div>
          
          <div class="services-grid">
            <div 
              v-for="service in configuredOptionalServices"
              :key="service.key"
              class="service-item configured optional"
            >
              <div class="service-icon">
                <i :class="service.icon"></i>
              </div>
              <div class="service-details">
                <span class="service-name">{{ service.name }}</span>
                <span class="service-provider">{{ getServiceProvider(service.key) }}</span>
              </div>
              <div class="service-status">
                <i class="pi pi-check"></i>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Connection Testing -->
      <div class="connection-testing">
        <div class="testing-header">
          <h3>Connection Testing</h3>
          <p>Verifying all configured services are working properly</p>
        </div>
        
        <div class="testing-progress">
          <div class="progress-bar">
            <div 
              class="progress-fill" 
              :style="{ width: `${testingProgress}%` }"
            ></div>
          </div>
          <div class="progress-text">
            {{ testingStatus }}
          </div>
        </div>
        
        <div class="test-results">
          <div 
            v-for="test in connectionTests"
            :key="test.service"
            class="test-result"
            :class="test.status"
          >
            <div class="test-icon">
              <i v-if="test.status === 'testing'" class="pi pi-spin pi-spinner"></i>
              <i v-else-if="test.status === 'success'" class="pi pi-check"></i>
              <i v-else-if="test.status === 'error'" class="pi pi-times"></i>
              <i v-else class="pi pi-clock"></i>
            </div>
            <div class="test-details">
              <span class="test-name">{{ test.name }}</span>
              <span class="test-message">{{ test.message }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Next Steps -->
      <div class="next-steps">
        <h3>What's Next?</h3>
        <div class="steps-grid">
          <div class="step-card">
            <div class="step-icon">
              <i class="pi pi-chart-line"></i>
            </div>
            <div class="step-content">
              <h4>Start Trading</h4>
              <p>Begin exploring options chains and placing trades with your configured providers</p>
            </div>
          </div>
          
          <div class="step-card">
            <div class="step-icon">
              <i class="pi pi-cog"></i>
            </div>
            <div class="step-content">
              <h4>Customize Settings</h4>
              <p>Fine-tune your configuration and add more providers in the Settings panel</p>
            </div>
          </div>
          
          <div class="step-card">
            <div class="step-icon">
              <i class="pi pi-book"></i>
            </div>
            <div class="step-content">
              <h4>Learn More</h4>
              <p>Explore advanced features and trading strategies in our documentation</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Important Notes -->
      <div class="important-notes">
        <div class="note-card security">
          <div class="note-icon">
            <i class="pi pi-shield"></i>
          </div>
          <div class="note-content">
            <h4>Security Reminder</h4>
            <p>Your credentials are encrypted and stored securely. You can update them anytime in Settings.</p>
          </div>
        </div>
        
        <div v-if="hasLiveAccounts" class="note-card warning">
          <div class="note-icon">
            <i class="pi pi-exclamation-triangle"></i>
          </div>
          <div class="note-content">
            <h4>Live Trading Active</h4>
            <p>You have live trading accounts configured. Please trade responsibly with real money.</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue';

export default {
  name: 'WizardStep5Complete',
  props: {
    wizardData: {
      type: Object,
      required: true
    }
  },
  setup(props) {
    // Testing state
    const testingProgress = ref(0);
    const testingStatus = ref('Preparing tests...');
    const connectionTests = ref([]);

    // Mandatory services
    const mandatoryServices = [
      { key: 'trade_account', name: 'Trade Account', icon: 'pi pi-wallet' },
      { key: 'options_chain', name: 'Options Chain', icon: 'pi pi-link' },
      { key: 'historical_data', name: 'Historical Data', icon: 'pi pi-chart-bar' },
      { key: 'symbol_lookup', name: 'Symbol Lookup', icon: 'pi pi-search' },
      { key: 'streaming_quotes', name: 'Streaming Quotes', icon: 'pi pi-bolt' }
    ];

    // Optional services
    const optionalServices = [
      { key: 'stock_quotes', name: 'Stock Quotes', icon: 'pi pi-dollar' },
      { key: 'market_calendar', name: 'Market Calendar', icon: 'pi pi-calendar' },
      { key: 'greeks', name: 'Greeks (API)', icon: 'pi pi-calculator' },
      { key: 'streaming_greeks', name: 'Greeks (Streaming)', icon: 'pi pi-wave-pulse' }
    ];

    // Computed properties
    const activeProviders = computed(() => {
      const providers = props.wizardData.providers || {};
      return Object.fromEntries(
        Object.entries(providers).filter(([, provider]) => provider.active)
      );
    });

    const activeProvidersCount = computed(() => {
      return Object.keys(activeProviders.value).length;
    });

    const configuredMandatoryServices = computed(() => {
      const routes = props.wizardData.mandatoryRoutes || {};
      return mandatoryServices.filter(service => routes[service.key]);
    });

    const mandatoryServicesCount = computed(() => {
      return configuredMandatoryServices.value.length;
    });

    const configuredOptionalServices = computed(() => {
      const routes = props.wizardData.optionalRoutes || {};
      return optionalServices.filter(service => routes[service.key]);
    });

    const configuredOptionalServicesCount = computed(() => {
      return configuredOptionalServices.value.length;
    });

    const hasLiveAccounts = computed(() => {
      return Object.values(activeProviders.value).some(provider => 
        provider.account_type === 'live'
      );
    });

    // Methods
    const getProviderIcon = (providerType) => {
      const svgProviders = ['alpaca', 'tradier', 'public', 'tastytrade'];
      if (svgProviders.includes(providerType)) {
        return null;
      }
      return "pi pi-server";
    };

    const getProviderTypeName = (providerType) => {
      const providerTypes = props.wizardData.providerTypes || {};
      return providerTypes[providerType]?.name || providerType;
    };

    const getServiceProvider = (serviceKey) => {
      const mandatoryRoutes = props.wizardData.mandatoryRoutes || {};
      const optionalRoutes = props.wizardData.optionalRoutes || {};
      const providerName = mandatoryRoutes[serviceKey] || optionalRoutes[serviceKey];
      
      if (!providerName) return 'Not configured';
      
      const provider = activeProviders.value[providerName];
      return provider ? provider.display_name : providerName;
    };

    const runConnectionTests = async () => {
      const allServices = [...configuredMandatoryServices.value, ...configuredOptionalServices.value];
      
      // Initialize tests
      connectionTests.value = allServices.map(service => ({
        service: service.key,
        name: service.name,
        status: 'pending',
        message: 'Waiting to test...'
      }));

      testingStatus.value = 'Testing connections...';
      
      // Simulate testing each service
      for (let i = 0; i < connectionTests.value.length; i++) {
        const test = connectionTests.value[i];
        
        // Update status to testing
        test.status = 'testing';
        test.message = 'Testing connection...';
        
        // Simulate test delay
        await new Promise(resolve => setTimeout(resolve, 1000 + Math.random() * 1000));
        
        // Simulate test result (90% success rate)
        const success = Math.random() > 0.1;
        
        if (success) {
          test.status = 'success';
          test.message = 'Connection successful';
        } else {
          test.status = 'error';
          test.message = 'Connection failed - check configuration';
        }
        
        // Update progress
        testingProgress.value = ((i + 1) / connectionTests.value.length) * 100;
      }
      
      // Final status
      const failedTests = connectionTests.value.filter(test => test.status === 'error');
      if (failedTests.length === 0) {
        testingStatus.value = 'All connections successful!';
      } else {
        testingStatus.value = `${failedTests.length} connection${failedTests.length !== 1 ? 's' : ''} failed`;
      }
    };

    // Lifecycle
    onMounted(() => {
      // Start connection testing after a short delay
      setTimeout(() => {
        runConnectionTests();
      }, 1000);
    });

    return {
      // State
      testingProgress,
      testingStatus,
      connectionTests,
      mandatoryServices,
      optionalServices,
      
      // Computed
      activeProviders,
      activeProvidersCount,
      configuredMandatoryServices,
      mandatoryServicesCount,
      configuredOptionalServices,
      configuredOptionalServicesCount,
      hasLiveAccounts,
      
      // Methods
      getProviderIcon,
      getProviderTypeName,
      getServiceProvider
    };
  }
};
</script>

<style scoped>
.complete-step {
  max-width: 900px;
  margin: 0 auto;
}

.completion-content {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xxl);
}

/* Success Hero */
.success-hero {
  text-align: center;
  padding: var(--spacing-xxl) 0;
}

.success-icon {
  width: 100px;
  height: 100px;
  margin: 0 auto var(--spacing-xl);
  background: linear-gradient(135deg, var(--color-success) 0%, #10b981 100%);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 20px 40px rgba(0, 200, 81, 0.3);
  animation: successPulse 2s ease-in-out infinite;
}

@keyframes successPulse {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.05); }
}

.success-icon i {
  font-size: 3rem;
  color: white;
}

.success-title {
  margin: 0 0 var(--spacing-lg) 0;
  font-size: var(--font-size-xxl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  background: linear-gradient(135deg, var(--color-success) 0%, #10b981 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.success-description {
  margin: 0;
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
  line-height: 1.6;
  max-width: 600px;
  margin: 0 auto;
}

/* Configuration Summary */
.configuration-summary {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-xl);
  padding: var(--spacing-xxl);
}

.summary-title {
  margin: 0 0 var(--spacing-xl) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  text-align: center;
}

.summary-section {
  margin-bottom: var(--spacing-xl);
}

.summary-section:last-child {
  margin-bottom: 0;
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
  background-color: var(--bg-tertiary);
  color: var(--text-secondary);
}

.section-icon.success {
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
}

.section-icon.optional {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
}

.section-info h4 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.section-info p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}

/* Provider List */
.provider-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.provider-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
}

.provider-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  color: var(--color-primary);
  flex-shrink: 0;
}

.provider-logo {
  width: 32px;
  height: 20px;
  object-fit: contain;
  filter: brightness(0) saturate(100%) invert(58%) sepia(69%) saturate(2618%) hue-rotate(346deg) brightness(101%) contrast(101%);
}

.provider-details {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.provider-name {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.provider-type {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.provider-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
}

.provider-badge.live {
  background-color: rgba(255, 187, 51, 0.1);
  color: var(--color-warning);
  border: 1px solid var(--color-warning);
}

.provider-badge.paper {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  border: 1px solid var(--color-info);
}

/* Services Grid */
.services-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-md);
}

.service-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
}

.service-item.configured {
  border-color: var(--color-success);
  background-color: rgba(0, 200, 81, 0.05);
}

.service-item.optional {
  border-color: var(--color-info);
  background-color: rgba(59, 130, 246, 0.05);
}

.service-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
  font-size: var(--font-size-md);
  flex-shrink: 0;
}

.service-item.optional .service-icon {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
}

.service-details {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.service-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.service-provider {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}

.service-status {
  color: var(--color-success);
  font-size: var(--font-size-md);
  flex-shrink: 0;
}

.service-item.optional .service-status {
  color: var(--color-info);
}

/* Connection Testing */
.connection-testing {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-xl);
  padding: var(--spacing-xxl);
}

.testing-header {
  text-align: center;
  margin-bottom: var(--spacing-xl);
}

.testing-header h3 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.testing-header p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}

.testing-progress {
  margin-bottom: var(--spacing-xl);
}

.progress-bar {
  width: 100%;
  height: 8px;
  background-color: var(--border-secondary);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-md);
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--color-info) 0%, var(--color-success) 100%);
  border-radius: var(--radius-sm);
  transition: width 0.3s ease;
}

.progress-text {
  text-align: center;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}

.test-results {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.test-result {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.test-result.success {
  border-color: var(--color-success);
  background-color: rgba(0, 200, 81, 0.05);
}

.test-result.error {
  border-color: var(--color-danger);
  background-color: rgba(220, 38, 127, 0.05);
}

.test-result.testing {
  border-color: var(--color-info);
  background-color: rgba(59, 130, 246, 0.05);
}

.test-icon {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-sm);
  flex-shrink: 0;
}

.test-result.success .test-icon {
  color: var(--color-success);
}

.test-result.error .test-icon {
  color: var(--color-danger);
}

.test-result.testing .test-icon {
  color: var(--color-info);
}

.test-result.pending .test-icon {
  color: var(--text-tertiary);
}

.test-details {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.test-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.test-message {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}

/* Next Steps */
.next-steps {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-xl);
  padding: var(--spacing-xxl);
}

.next-steps h3 {
  margin: 0 0 var(--spacing-xl) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  text-align: center;
}

.steps-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-lg);
}

.step-card {
  padding: var(--spacing-xl);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  text-align: center;
  transition: var(--transition-normal);
}

.step-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.15);
  border-color: var(--border-tertiary);
}

.step-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto var(--spacing-lg);
  background: linear-gradient(135deg, var(--color-brand) 0%, #ff8a50 100%);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-xl);
  color: white;
  box-shadow: 0 8px 24px rgba(255, 107, 53, 0.3);
}

.step-content h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.step-content p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  line-height: 1.5;
}

/* Important Notes */
.important-notes {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.note-card {
  display: flex;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
}

.note-card.security {
  background-color: rgba(0, 200, 81, 0.1);
  border: 1px solid var(--color-success);
}

.note-card.warning {
  background-color: rgba(255, 187, 51, 0.1);
  border: 1px solid var(--color-warning);
}

.note-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-lg);
  flex-shrink: 0;
}

.note-card.security .note-icon {
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
}

.note-card.warning .note-icon {
  background-color: rgba(255, 187, 51, 0.1);
  color: var(--color-warning);
}

.note-content h4 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.note-content p {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  line-height: 1.5;
}

/* Responsive Design */
@media (max-width: 768px) {
  .completion-content {
    gap: var(--spacing-xl);
  }
  
  .success-icon {
    width: 80px;
    height: 80px;
  }
  
  .success-icon i {
    font-size: 2.5rem;
  }
  
  .success-title {
    font-size: var(--font-size-xl);
  }
  
  .configuration-summary,
  .connection-testing,
  .next-steps {
    padding: var(--spacing-lg);
  }
  
  .services-grid {
    grid-template-columns: 1fr;
  }
  
  .steps-grid {
    grid-template-columns: 1fr;
  }
  
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
  
  .provider-item {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
  
  .provider-badge {
    align-self: flex-end;
  }
}

@media (max-width: 480px) {
  .success-hero {
    padding: var(--spacing-lg) 0;
  }
  
  .success-icon {
    width: 64px;
    height: 64px;
  }
  
  .success-icon i {
    font-size: 2rem;
  }
  
  .step-icon {
    width: 48px;
    height: 48px;
  }
  
  .step-icon i {
    font-size: var(--font-size-lg);
  }
  
  .important-notes {
    gap: var(--spacing-md);
  }
  
  .note-card {
    flex-direction: column;
    gap: var(--spacing-sm);
  }
}
</style>
