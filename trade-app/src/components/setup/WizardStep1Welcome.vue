<template>
  <div class="welcome-step">
    <div class="welcome-content">
      <!-- Hero Section -->
      <div class="hero-section">
        <div class="hero-icon">
          <i class="pi pi-chart-line"></i>
        </div>
        <h2 class="hero-title">Welcome to JuicyTrade</h2>
        <p class="hero-description">
          Let's get your trading platform configured with the essential data providers 
          you need to start trading options successfully.
        </p>
      </div>

      <!-- What We'll Configure -->
      <div class="configuration-overview">
        <h3 class="section-title">What we'll configure:</h3>
        
        <div class="config-categories">
          <!-- Mandatory Services -->
          <div class="config-category">
            <div class="category-header">
              <div class="category-icon mandatory">
                <i class="pi pi-exclamation-circle"></i>
              </div>
              <div class="category-info">
                <h4 class="category-title">Required Services</h4>
                <p class="category-description">Essential services needed for trading</p>
              </div>
            </div>
            
            <div class="service-list">
              <div 
                v-for="service in mandatoryServices" 
                :key="service.key"
                class="service-item mandatory"
              >
                <div class="service-icon">
                  <i :class="service.icon"></i>
                </div>
                <div class="service-details">
                  <span class="service-name">{{ service.name }}</span>
                  <span class="service-description">{{ service.description }}</span>
                </div>
                <div class="service-badge required">
                  Required
                </div>
              </div>
            </div>
          </div>

          <!-- Optional Services -->
          <div class="config-category">
            <div class="category-header">
              <div class="category-icon optional">
                <i class="pi pi-info-circle"></i>
              </div>
              <div class="category-info">
                <h4 class="category-title">Optional Services</h4>
                <p class="category-description">Additional features you can configure later</p>
              </div>
            </div>
            
            <div class="service-list">
              <div 
                v-for="service in optionalServices" 
                :key="service.key"
                class="service-item optional"
              >
                <div class="service-icon">
                  <i :class="service.icon"></i>
                </div>
                <div class="service-details">
                  <span class="service-name">{{ service.name }}</span>
                  <span class="service-description">{{ service.description }}</span>
                </div>
                <div class="service-badge optional">
                  Optional
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Setup Process -->
      <div class="process-overview">
        <h3 class="section-title">Setup Process:</h3>
        
        <div class="process-steps">
          <div class="process-step">
            <div class="step-number">1</div>
            <div class="step-content">
              <h4>Add Providers</h4>
              <p>Connect your trading accounts and data providers</p>
            </div>
          </div>
          
          <div class="step-connector"></div>
          
          <div class="process-step">
            <div class="step-number">2</div>
            <div class="step-content">
              <h4>Configure Routes</h4>
              <p>Map services to your preferred providers</p>
            </div>
          </div>
          
          <div class="step-connector"></div>
          
          <div class="process-step">
            <div class="step-number">3</div>
            <div class="step-content">
              <h4>Test & Complete</h4>
              <p>Verify connections and start trading</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Important Notes -->
      <div class="important-notes">
        <div class="note-card">
          <div class="note-icon">
            <i class="pi pi-cog"></i>
          </div>
          <div class="note-content">
            <h4>Local Configuration</h4>
            <p>Your provider settings are stored locally in your trading environment. You have full control over your configuration data.</p>
          </div>
        </div>
        
        <div class="note-card">
          <div class="note-icon">
            <i class="pi pi-clock"></i>
          </div>
          <div class="note-content">
            <h4>Quick Setup</h4>
            <p>This setup typically takes 5-10 minutes. You can always modify your configuration later in Settings.</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'WizardStep1Welcome',
  props: {
    wizardData: {
      type: Object,
      required: true
    }
  },
  setup() {
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
        name: 'Greeks (API)',
        description: 'Option Greeks via API calls',
        icon: 'pi pi-calculator'
      },
      {
        key: 'streaming_greeks',
        name: 'Greeks (Streaming)',
        description: 'Real-time option Greeks streaming',
        icon: 'pi pi-wave-pulse'
      }
    ];

    return {
      mandatoryServices,
      optionalServices
    };
  }
};
</script>

<style scoped>
.welcome-step {
  max-width: 800px;
  margin: 0 auto;
}

.welcome-content {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xxl);
}

/* Hero Section */
.hero-section {
  text-align: center;
  padding: var(--spacing-xl) 0;
}

.hero-icon {
  width: 80px;
  height: 80px;
  margin: 0 auto var(--spacing-lg);
  background: linear-gradient(135deg, var(--color-brand) 0%, #ff8a50 100%);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 32px rgba(255, 107, 53, 0.3);
}

.hero-icon i {
  font-size: 2.5rem;
  color: white;
}

.hero-title {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-xxl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
}

.hero-description {
  margin: 0;
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
  line-height: 1.6;
  max-width: 600px;
  margin: 0 auto;
}

/* Configuration Overview */
.configuration-overview {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
}

.section-title {
  margin: 0 0 var(--spacing-lg) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.config-categories {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xl);
}

.config-category {
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  padding: var(--spacing-lg);
}

.category-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-lg);
}

.category-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-lg);
}

.category-icon.mandatory {
  background-color: rgba(255, 107, 53, 0.1);
  color: var(--color-brand);
  border: 1px solid var(--color-brand);
}

.category-icon.optional {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  border: 1px solid var(--color-info);
}

.category-info {
  flex: 1;
}

.category-title {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.category-description {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-secondary);
}

/* Service List */
.service-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.service-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background-color: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.service-item:hover {
  border-color: var(--border-tertiary);
}

.service-icon {
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

.service-details {
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

.service-description {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.service-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
}

.service-badge.required {
  background-color: rgba(255, 107, 53, 0.1);
  color: var(--color-brand);
  border: 1px solid var(--color-brand);
}

.service-badge.optional {
  background-color: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
  border: 1px solid var(--color-info);
}

/* Process Overview */
.process-overview {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
}

.process-steps {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-lg);
}

.process-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  flex: 1;
  max-width: 200px;
}

.step-number {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--color-brand) 0%, #ff8a50 100%);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  margin-bottom: var(--spacing-md);
  box-shadow: 0 4px 16px rgba(255, 107, 53, 0.3);
}

.step-content h4 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.step-content p {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  line-height: 1.4;
}

.step-connector {
  width: 60px;
  height: 2px;
  background: linear-gradient(90deg, var(--color-brand) 0%, #ff8a50 100%);
  margin: 0 var(--spacing-md);
  flex-shrink: 0;
}

/* Important Notes */
.important-notes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: var(--spacing-lg);
}

.note-card {
  display: flex;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
}

.note-card:hover {
  border-color: var(--border-tertiary);
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.note-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-lg);
  flex-shrink: 0;
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
  .config-categories {
    gap: var(--spacing-lg);
  }
  
  .process-steps {
    flex-direction: column;
    gap: var(--spacing-lg);
  }
  
  .step-connector {
    width: 2px;
    height: 40px;
    margin: var(--spacing-md) 0;
  }
  
  .important-notes {
    grid-template-columns: 1fr;
  }
  
  .hero-icon {
    width: 64px;
    height: 64px;
  }
  
  .hero-icon i {
    font-size: 2rem;
  }
  
  .hero-title {
    font-size: var(--font-size-xl);
  }
  
  .hero-description {
    font-size: var(--font-size-md);
  }
}

@media (max-width: 480px) {
  .welcome-content {
    gap: var(--spacing-xl);
  }
  
  .configuration-overview,
  .process-overview {
    padding: var(--spacing-lg);
  }
  
  .service-item {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
  
  .service-badge {
    align-self: flex-end;
  }
}
</style>
