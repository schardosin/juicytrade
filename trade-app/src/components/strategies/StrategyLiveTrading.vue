<template>
  <div class="live-trading">
    <!-- Header -->
    <div class="live-trading-header">
      <div class="header-content">
        <h1 class="page-title">⚡ Live Trading</h1>
        <p class="page-subtitle">Deploy and monitor your live trading strategies</p>
      </div>
      <div class="header-actions">
        <button 
          class="btn btn-primary"
          @click="showDeployDialog = true"
          :disabled="availableStrategies.length === 0"
        >
          <span class="icon">⚡</span>
          Deploy Strategy
        </button>
      </div>
    </div>

    <!-- Live Strategies Grid -->
    <div class="live-strategies-grid" v-if="liveStrategies.length > 0">
      <div 
        v-for="liveStrategy in liveStrategies" 
        :key="liveStrategy.deployment_id"
        class="live-strategy-card"
      >
        <!-- Strategy Header -->
        <div class="strategy-header">
          <div class="strategy-info">
            <h3 class="strategy-name">{{ liveStrategy.strategy_name }}</h3>
            <p class="configuration-name">{{ liveStrategy.configuration_name }}</p>
            <div class="strategy-meta">
              <span class="meta-item">
                <span class="meta-label">Status:</span>
                <span class="status-badge" :class="`status-${liveStrategy.status}`">
                  {{ liveStrategy.status }}
                </span>
              </span>
              <span class="meta-item">
                <span class="meta-label">Started:</span>
                {{ formatDate(liveStrategy.deployed_at) }}
              </span>
            </div>
          </div>
          <div class="strategy-controls">
            <button 
              class="btn-icon"
              @click="pauseStrategy(liveStrategy)"
              :disabled="liveStrategy.status !== 'running'"
              title="Pause Strategy"
            >
              ⏸️
            </button>
            <button 
              class="btn-icon"
              @click="stopStrategy(liveStrategy)"
              title="Stop Strategy"
            >
              ⏹️
            </button>
          </div>
        </div>

        <!-- Performance Summary -->
        <div class="performance-summary">
          <div class="performance-stats">
            <div class="stat-item">
              <span class="stat-number" :class="getPnLClass(liveStrategy.total_pnl)">
                ${{ formatNumber(liveStrategy.total_pnl) }}
              </span>
              <span class="stat-label">Total P&L</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ liveStrategy.total_trades }}</span>
              <span class="stat-label">Total Trades</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ liveStrategy.win_rate }}%</span>
              <span class="stat-label">Win Rate</span>
            </div>
          </div>
        </div>

        <!-- Recent Activity -->
        <div class="recent-activity" v-if="liveStrategy.recent_trades && liveStrategy.recent_trades.length > 0">
          <h4 class="activity-title">Recent Trades</h4>
          <div class="trade-list">
            <div 
              v-for="trade in liveStrategy.recent_trades.slice(0, 3)"
              :key="trade.trade_id"
              class="trade-item"
            >
              <div class="trade-info">
                <span class="trade-symbol">{{ trade.symbol }}</span>
                <span class="trade-action" :class="`action-${trade.action}`">{{ trade.action }}</span>
                <span class="trade-pnl" :class="getPnLClass(trade.pnl)">${{ formatNumber(trade.pnl) }}</span>
              </div>
              <div class="trade-time">{{ formatTime(trade.executed_at) }}</div>
            </div>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="strategy-actions">
          <button 
            class="btn btn-outline"
            @click="viewDetails(liveStrategy)"
          >
            <span class="icon">📊</span>
            View Details
          </button>
          <button 
            class="btn btn-secondary"
            @click="viewConfiguration(liveStrategy)"
          >
            <span class="icon">⚙️</span>
            Configuration
          </button>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty-state">
      <div class="empty-icon">⚡</div>
      <h3 class="empty-title">No Live Strategies</h3>
      <p class="empty-description">
        Deploy your first strategy configuration to start live trading.
      </p>
      <div class="empty-actions">
        <button 
          class="btn btn-primary"
          @click="navigateToLibrary"
        >
          <span class="icon">📚</span>
          Go to Library
        </button>
        <button 
          class="btn btn-outline"
          @click="navigateToBacktesting"
        >
          <span class="icon">📈</span>
          Test Strategies
        </button>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Loading live strategies...</p>
    </div>

    <!-- Deploy Dialog - TODO: Implement -->
    <!-- <StrategyDeployDialog 
      v-if="showDeployDialog"
      :configurations="availableConfigurations"
      @close="showDeployDialog = false"
      @deployed="handleStrategyDeployed"
    /> -->
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../services/api.js'

export default {
  name: 'StrategyLiveTrading',
  setup() {
    const router = useRouter()
    
    // Reactive state
    const liveStrategies = ref([])
    const availableStrategies = ref([])
    const loading = ref(true)
    const error = ref(null)
    
    // Dialog states
    const showDeployDialog = ref(false)

    // Load data
    const loadLiveStrategies = async () => {
      try {
        loading.value = true
        // TODO: Implement API endpoint for live strategies
        // const response = await api.getLiveStrategies()
        // liveStrategies.value = response || []
        
        // Mock data for now
        liveStrategies.value = []
      } catch (err) {
        console.error('Error loading live strategies:', err)
        error.value = 'Failed to load live strategies'
      } finally {
        loading.value = false
      }
    }

    const loadAvailableStrategies = async () => {
      try {
        // Load all strategy templates that could be deployed
        const strategies = await api.getStrategies()
        availableStrategies.value = strategies || []
      } catch (err) {
        console.error('Error loading strategies:', err)
      }
    }

    // Helper functions
    const formatDate = (dateString) => {
      return new Date(dateString).toLocaleDateString()
    }

    const formatTime = (dateString) => {
      return new Date(dateString).toLocaleTimeString()
    }

    const formatNumber = (number) => {
      return Math.abs(number).toFixed(2)
    }

    const getPnLClass = (pnl) => {
      if (pnl > 0) return 'positive'
      if (pnl < 0) return 'negative'
      return 'neutral'
    }

    // Actions
    const pauseStrategy = (liveStrategy) => {
      // TODO: Implement pause functionality
      console.log('Pausing strategy:', liveStrategy)
    }

    const stopStrategy = (liveStrategy) => {
      // TODO: Implement stop functionality
      console.log('Stopping strategy:', liveStrategy)
    }

    const viewDetails = (liveStrategy) => {
      // Navigate to detailed monitoring view
      router.push(`/strategies/monitor/${liveStrategy.deployment_id}`)
    }

    const viewConfiguration = (liveStrategy) => {
      // Navigate to configuration view
      router.push(`/strategies/library?config=${liveStrategy.config_id}`)
    }

    const navigateToLibrary = () => {
      router.push('/strategies/library')
    }

    const navigateToBacktesting = () => {
      router.push('/strategies/backtest')
    }

    // Event handlers
    const handleStrategyDeployed = () => {
      showDeployDialog.value = false
      loadLiveStrategies()
    }

    // Lifecycle
    onMounted(async () => {
      await loadLiveStrategies()
      await loadAvailableStrategies()
    })

    return {
      // State
      liveStrategies,
      availableStrategies,
      loading,
      error,
      
      // Dialog states
      showDeployDialog,
      
      // Helper functions
      formatDate,
      formatTime,
      formatNumber,
      getPnLClass,
      
      // Actions
      pauseStrategy,
      stopStrategy,
      viewDetails,
      viewConfiguration,
      navigateToLibrary,
      navigateToBacktesting,
      
      // Event handlers
      handleStrategyDeployed
    }
  }
}
</script>

<style scoped>
.live-trading {
  padding: var(--spacing-xl);
  height: 100%;
  overflow-y: auto;
  background-color: var(--bg-secondary);
}

.live-trading-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-xxl);
  padding-bottom: var(--spacing-xl);
  border-bottom: 1px solid var(--border-secondary);
}

.header-content {
  flex: 1;
}

.page-title {
  font-size: var(--font-size-xxl);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
}

.page-subtitle {
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-md);
}

.live-strategies-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: var(--spacing-xl);
}

.live-strategy-card {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  transition: var(--transition-normal);
}

.live-strategy-card:hover {
  border-color: var(--color-brand);
  box-shadow: 0 4px 12px rgba(255, 107, 53, 0.1);
}

.strategy-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-lg);
}

.strategy-info {
  flex: 1;
}

.strategy-name {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
}

.configuration-name {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-md);
  font-style: italic;
}

.strategy-meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-lg);
  font-size: var(--font-size-sm);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.meta-label {
  color: var(--text-tertiary);
}

.status-badge {
  padding: 2px var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
  font-size: var(--font-size-xs);
}

.status-running { background: var(--color-success); color: var(--text-primary); opacity: 0.8; }
.status-paused { background: var(--color-warning); color: var(--text-primary); opacity: 0.8; }
.status-stopped { background: var(--color-danger); color: var(--text-primary); opacity: 0.8; }

.strategy-controls {
  display: flex;
  gap: var(--spacing-xs);
}

.performance-summary {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
}

.performance-stats {
  display: flex;
  justify-content: space-around;
}

.stat-item {
  text-align: center;
}

.stat-number {
  display: block;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.stat-number.positive { color: var(--color-success); }
.stat-number.negative { color: var(--color-danger); }
.stat-number.neutral { color: var(--text-secondary); }

.stat-label {
  display: block;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  text-transform: uppercase;
}

.recent-activity {
  margin-bottom: var(--spacing-lg);
}

.activity-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.trade-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.trade-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-primary);
  border-radius: var(--radius-md);
  font-size: var(--font-size-sm);
}

.trade-info {
  display: flex;
  gap: var(--spacing-md);
  align-items: center;
}

.trade-symbol {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.trade-action {
  text-transform: uppercase;
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
}

.action-buy { color: var(--color-success); }
.action-sell { color: var(--color-danger); }

.trade-pnl {
  font-weight: var(--font-weight-medium);
}

.trade-time {
  color: var(--text-tertiary);
  font-size: var(--font-size-xs);
}

.strategy-actions {
  display: flex;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-lg);
  border-radius: var(--radius-md);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-normal);
  border: none;
  text-decoration: none;
}

.btn-primary {
  background: var(--color-brand);
  color: var(--text-primary);
}

.btn-primary:hover {
  background: var(--color-brand-hover);
}

.btn-secondary {
  background: var(--bg-quaternary);
  color: var(--text-primary);
}

.btn-secondary:hover {
  background: var(--border-tertiary);
}

.btn-outline {
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border-secondary);
}

.btn-outline:hover {
  background: var(--bg-quaternary);
  color: var(--text-primary);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-icon {
  background: none;
  border: none;
  padding: var(--spacing-xs);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition-fast);
}

.btn-icon:hover {
  background: var(--bg-quaternary);
}

.btn-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.empty-state {
  text-align: center;
  padding: 80px var(--spacing-lg);
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: var(--spacing-lg);
}

.empty-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
}

.empty-description {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-xl) 0;
  max-width: 400px;
  margin-left: auto;
  margin-right: auto;
}

.empty-actions {
  display: flex;
  gap: var(--spacing-md);
  justify-content: center;
  flex-wrap: wrap;
}

.loading-state {
  text-align: center;
  padding: 80px var(--spacing-lg);
  color: var(--text-secondary);
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--border-secondary);
  border-top: 3px solid var(--color-brand);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto var(--spacing-lg);
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.icon {
  font-size: var(--font-size-md);
}

.positive { color: var(--color-success); }
.negative { color: var(--color-danger); }
.neutral { color: var(--text-secondary); }
</style>
