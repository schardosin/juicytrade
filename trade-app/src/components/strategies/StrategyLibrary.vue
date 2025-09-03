<template>
  <div class="strategy-library">
    <!-- Header -->
    <div class="library-header">
      <div class="header-content">
        <h1 class="library-title">📚 Strategy Library</h1>
        <p class="library-subtitle">Manage your trading strategies and configurations</p>
      </div>
      <div class="header-actions">
        <!-- View Toggle -->
        <div class="view-toggle">
          <button 
            class="toggle-btn"
            :class="{ active: viewMode === 'cards' }"
            @click="viewMode = 'cards'"
            title="Card View"
          >
            <span class="icon">⊞</span>
          </button>
          <button 
            class="toggle-btn"
            :class="{ active: viewMode === 'list' }"
            @click="viewMode = 'list'"
            title="List View"
          >
            <span class="icon">☰</span>
          </button>
        </div>
        <button 
          class="btn btn-primary"
          @click="showUploadDialog = true"
        >
          <span class="icon">➕</span>
          Upload Strategy
        </button>
      </div>
    </div>

    <!-- Strategy Cards View -->
    <div v-if="strategies.length > 0 && viewMode === 'cards'" class="strategies-grid">
      <div 
        v-for="strategy in strategies" 
        :key="strategy.strategy_id"
        class="strategy-card"
      >
        <!-- Strategy Header -->
        <div class="strategy-header">
          <div class="strategy-info">
            <h3 class="strategy-name">{{ strategy.name }}</h3>
            <p class="strategy-description">{{ strategy.description || 'No description provided' }}</p>
            <div class="strategy-meta">
              <span class="meta-item">
                <span class="meta-label">Risk:</span>
                <span class="risk-badge" :class="`risk-${strategy.risk_level}`">
                  {{ strategy.risk_level }}
                </span>
              </span>
              <span class="meta-item">
                <span class="meta-label">Author:</span>
                {{ strategy.author }}
              </span>
              <span class="meta-item">
                <span class="meta-label">Version:</span>
                {{ strategy.version }}
              </span>
            </div>
          </div>
          <div class="strategy-status">
            <div class="strategy-actions-header">
              <button 
                class="btn-icon"
                @click="deleteStrategy(strategy)"
                title="Delete Strategy"
              >
                🗑️
              </button>
            </div>
          </div>
        </div>

        <!-- Instance Summary -->
        <div class="instance-summary">
          <div class="instance-stats">
            <div class="stat-item">
              <span class="stat-number">{{ getBacktestCount(strategy.strategy_id) }}</span>
              <span class="stat-label">Backtests</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ getLiveCount(strategy.strategy_id) }}</span>
              <span class="stat-label">Live Instances</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ getTotalPnL(strategy.strategy_id) }}</span>
              <span class="stat-label">Total P&L</span>
            </div>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="strategy-actions">
          <button 
            class="btn btn-outline"
            @click="createBacktest(strategy)"
          >
            <span class="icon">🚀</span>
            Create Backtest
          </button>
          <button 
            class="btn btn-success"
            @click="deployLive(strategy)"
          >
            <span class="icon">⚡</span>
            Deploy Live
          </button>
        </div>
      </div>
    </div>

    <!-- Strategy List View -->
    <div v-if="strategies.length > 0 && viewMode === 'list'" class="strategies-list">
      <div class="list-header">
        <div class="list-col col-name">Strategy</div>
        <div class="list-col col-risk">Risk</div>
        <div class="list-col col-author">Author</div>
        <div class="list-col col-version">Version</div>
        <div class="list-col col-backtests">Backtests</div>
        <div class="list-col col-live">Live</div>
        <div class="list-col col-pnl">P&L</div>
        <div class="list-col col-actions">Actions</div>
      </div>
      
      <div 
        v-for="strategy in strategies" 
        :key="strategy.strategy_id"
        class="strategy-row"
      >
        <div class="list-col col-name">
          <div class="strategy-name-info">
            <h4 class="strategy-name">{{ strategy.name }}</h4>
            <p class="strategy-description">{{ strategy.description || 'No description provided' }}</p>
          </div>
        </div>
        <div class="list-col col-risk">
          <span class="risk-badge" :class="`risk-${strategy.risk_level}`">
            {{ strategy.risk_level }}
          </span>
        </div>
        <div class="list-col col-author">{{ strategy.author }}</div>
        <div class="list-col col-version">{{ strategy.version }}</div>
        <div class="list-col col-backtests">
          <span class="stat-number">{{ getBacktestCount(strategy.strategy_id) }}</span>
        </div>
        <div class="list-col col-live">
          <span class="stat-number">{{ getLiveCount(strategy.strategy_id) }}</span>
        </div>
        <div class="list-col col-pnl">
          <span class="stat-number" :class="{ 'positive': getTotalPnL(strategy.strategy_id).startsWith('+') }">
            {{ getTotalPnL(strategy.strategy_id) }}
          </span>
        </div>
        <div class="list-col col-actions">
          <div class="list-actions">
            <button 
              class="btn btn-sm btn-outline"
              @click="createBacktest(strategy)"
              title="Create Backtest"
            >
              🚀
            </button>
            <button 
              class="btn btn-sm btn-success"
              @click="deployLive(strategy)"
              title="Deploy Live"
            >
              ⚡
            </button>
            <button 
              class="btn btn-sm btn-danger"
              @click="deleteStrategy(strategy)"
              title="Delete Strategy"
            >
              🗑️
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty-state">
      <div class="empty-icon">📄</div>
      <h3 class="empty-title">No Strategies Yet</h3>
      <p class="empty-description">
        Upload your first trading strategy to get started with automated trading.
      </p>
      <button 
        class="btn btn-primary"
        @click="showUploadDialog = true"
      >
        <span class="icon">➕</span>
        Upload Your First Strategy
      </button>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Loading strategies...</p>
    </div>

    <!-- Upload Dialog -->
    <StrategyUploadDialog 
      v-model:visible="showUploadDialog"
      @uploaded="handleStrategyUploaded"
    />


    <!-- Configuration Dialog - TODO: Implement -->
    <!-- <StrategyConfigurationDialog
      v-if="showConfigDialog"
      :strategy="selectedStrategy"
      :configuration="selectedConfiguration"
      @close="showConfigDialog = false"
      @saved="handleConfigurationSaved"
    /> -->

    <!-- Backtest Results Dialog - TODO: Implement -->
    <!-- <BacktestResultsDialog
      v-if="showBacktestDialog"
      :strategy="selectedStrategy"
      @close="showBacktestDialog = false"
    /> -->
  </div>
</template>

<script>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useNotifications } from '../../composables/useNotifications.js'
import { api } from '../../services/api.js'
import StrategyUploadDialog from './StrategyUploadDialog.vue'
import BacktestConfigDialog from './BacktestConfigDialog.vue'

export default {
  name: 'StrategyLibrary',
  components: {
    StrategyUploadDialog,
    BacktestConfigDialog,
    // These will be created next
    // StrategyConfigurationDialog,
    // BacktestResultsDialog
  },
  setup() {
    const router = useRouter()
    const { showSuccess, showError } = useNotifications()
    
    // Reactive state
    const strategies = ref([])
    const configurations = ref([])
    const backtestRuns = ref([])
    const loading = ref(true)
    const error = ref(null)
    
    // Dialog states
    const showUploadDialog = ref(false)
    const showConfigDialog = ref(false)
    const showBacktestDialog = ref(false)
    const selectedStrategy = ref(null)
    const selectedConfiguration = ref(null)
    
    // Backtest dialog states
    const selectedStrategyForBacktest = ref(null)
    
    // View mode state
    const viewMode = ref('cards') // 'cards' or 'list'

    // Load data
    const loadStrategies = async () => {
      try {
        loading.value = true
        const response = await api.getStrategies()
        strategies.value = response || []
      } catch (err) {
        console.error('Error loading strategies:', err)
        error.value = 'Failed to load strategies'
      } finally {
        loading.value = false
      }
    }

    const loadConfigurations = async () => {
      try {
        // Load configurations for all strategies
        const allConfigs = []
        for (const strategy of strategies.value) {
          try {
            const configs = await api.getStrategyConfigurations(strategy.strategy_id)
            if (configs && Array.isArray(configs)) {
              allConfigs.push(...configs)
            }
          } catch (err) {
            // Strategy might not have configurations yet, that's okay
            console.log(`No configurations found for strategy ${strategy.strategy_id}`)
          }
        }
        configurations.value = allConfigs
      } catch (err) {
        console.error('Error loading configurations:', err)
      }
    }

    const loadBacktestRuns = async () => {
      try {
        const runs = await api.getBacktestRuns()
        backtestRuns.value = runs || []
      } catch (err) {
        console.error('Error loading backtest runs:', err)
        // Set empty array if API call fails
        backtestRuns.value = []
      }
    }

    // Helper functions
    const getBacktestCount = (strategyId) => {
      return backtestRuns.value.filter(r => r.strategy_id === strategyId).length
    }

    const getLiveCount = (strategyId) => {
      // TODO: Implement live deployment tracking
      return 0
    }

    const getTotalPnL = (strategyId) => {
      // TODO: Calculate total P&L from all instances of this strategy
      const backtests = backtestRuns.value.filter(r => r.strategy_id === strategyId)
      const totalPnL = backtests.reduce((sum, run) => sum + (run.total_pnl || 0), 0)
      return totalPnL > 0 ? `+$${totalPnL.toFixed(2)}` : `$${totalPnL.toFixed(2)}`
    }

    const formatDate = (dateString) => {
      return new Date(dateString).toLocaleDateString()
    }

    // Actions
    const configureStrategy = (strategy, config = null) => {
      // For now, create a simple configuration using prompt
      if (config) {
        // Edit existing configuration
        showSuccess('Configuration editing coming soon!', 'Feature Preview')
      } else {
        // Create new configuration
        const configName = prompt(`Enter a name for the new configuration for "${strategy.name}":`)
        if (configName) {
          createNewConfiguration(strategy, configName)
        }
      }
    }

    const createNewConfiguration = async (strategy, configName) => {
      try {
        const defaultConfig = {
          name: configName,
          parameters: {
            // Add some default parameters based on strategy metadata
            symbol: 'AAPL',
            risk_per_trade: 100.0,
            position_size_pct: 1.0
          }
        }

        const result = await api.createConfiguration(strategy.strategy_id, defaultConfig)
        if (result.success) {
          showSuccess(`Configuration "${configName}" created successfully!`, 'Configuration Created')
          await loadConfigurations() // Reload configurations
        } else {
          showError(`Failed to create configuration: ${result.error}`, 'Configuration Error')
        }
      } catch (error) {
        console.error('Error creating configuration:', error)
        showError('Failed to create configuration. Please try again.', 'Configuration Error')
      }
    }

    // Modal dialog functions
    const closeBacktestDialog = () => {
      showBacktestDialog.value = false
      selectedStrategyForBacktest.value = null
    }

    const createBacktest = (strategy) => {
      // Navigate to backtesting page and trigger modal there
      router.push({
        path: '/strategies/backtest',
        query: { 
          strategy_id: strategy.strategy_id,
          auto_open: 'true'
        }
      })
    }

    const handleBacktestStarted = (result) => {
      if (result.success) {
        showSuccess(result.message, 'Backtest Started')
        loadBacktestRuns() // Refresh backtest runs
      } else {
        showError(result.message, 'Backtest Error')
      }
    }

    const viewBacktests = (strategy) => {
      // Navigate to backtest view for this strategy
      router.push({
        path: '/strategies/backtest',
        query: { strategy_id: strategy.strategy_id }
      })
    }

    const deployLive = (strategy) => {
      // Navigate to live trading with the strategy
      router.push({
        path: '/strategies/live',
        query: { strategy_id: strategy.strategy_id }
      })
    }

    const deleteStrategy = async (strategy) => {
      // Confirm deletion
      const confirmed = confirm(`Are you sure you want to delete the strategy "${strategy.name}"? This action cannot be undone.`)
      if (!confirmed) return

      try {
        const result = await api.deleteStrategy(strategy.strategy_id)
        if (result.success) {
          showSuccess(`Strategy "${strategy.name}" deleted successfully!`, 'Strategy Deleted')
          await loadStrategies() // Reload strategies list
        } else {
          showError(`Failed to delete strategy: ${result.error}`, 'Delete Error')
        }
      } catch (error) {
        console.error('Error deleting strategy:', error)
        showError('Failed to delete strategy. Please try again.', 'Delete Error')
      }
    }

    // Event handlers
    const handleStrategyUploaded = () => {
      showUploadDialog.value = false
      loadStrategies()
    }

    const handleConfigurationSaved = () => {
      showConfigDialog.value = false
      loadConfigurations()
    }

    // Lifecycle
    onMounted(async () => {
      await loadStrategies()
      await loadConfigurations()
      await loadBacktestRuns()
    })

    return {
      // State
      strategies,
      configurations,
      backtestRuns,
      loading,
      error,
      viewMode,
      
      // Dialog states
      showUploadDialog,
      showConfigDialog,
      showBacktestDialog,
      selectedStrategy,
      selectedConfiguration,
      
      // Backtest dialog states
      selectedStrategyForBacktest,
      
      // Helper functions
      getBacktestCount,
      getLiveCount,
      getTotalPnL,
      formatDate,
      
      // Actions
      configureStrategy,
      createNewConfiguration,
      createBacktest,
      viewBacktests,
      deployLive,
      deleteStrategy,
      
      // Modal dialog functions
      closeBacktestDialog,
      
      // Event handlers
      handleStrategyUploaded,
      handleConfigurationSaved,
      handleBacktestStarted
    }
  }
}
</script>

<style scoped>
.strategy-library {
  padding: var(--spacing-xl);
  height: 100%;
  overflow-y: auto;
  background-color: var(--bg-secondary);
}

.library-header {
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

.library-title {
  font-size: var(--font-size-xxl);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
}

.library-subtitle {
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-md);
  align-items: center;
}

/* View Toggle Styles */
.view-toggle {
  display: flex;
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  padding: 2px;
  border: 1px solid var(--border-primary);
}

.toggle-btn {
  background: none;
  border: none;
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition-fast);
  color: var(--text-secondary);
  font-size: var(--font-size-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 40px;
}

.toggle-btn:hover {
  color: var(--text-primary);
  background: var(--bg-quaternary);
}

.toggle-btn.active {
  background: var(--color-brand);
  color: var(--text-primary);
  box-shadow: 0 2px 4px rgba(255, 107, 53, 0.2);
}

.strategies-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: var(--spacing-xl);
}

.strategy-card {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  transition: var(--transition-normal);
}

.strategy-card:hover {
  border-color: var(--color-brand);
  box-shadow: 0 4px 12px rgba(255, 107, 53, 0.1);
}

.strategy-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.strategy-info {
  flex: 1;
}

.strategy-name {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
}

.strategy-description {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-md);
  line-height: 1.4;
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

.risk-badge {
  padding: 2px var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
  font-size: var(--font-size-xs);
}

.risk-low { background: var(--color-success); color: var(--text-primary); opacity: 0.8; }
.risk-medium { background: var(--color-warning); color: var(--text-primary); opacity: 0.8; }
.risk-high { background: var(--color-danger); color: var(--text-primary); opacity: 0.8; }

.validation-status {
  padding: var(--spacing-xs) var(--spacing-md);
  border-radius: var(--radius-md);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
}

.status-passed { background: var(--color-success); color: var(--text-primary); opacity: 0.8; }
.status-failed { background: var(--color-danger); color: var(--text-primary); opacity: 0.8; }
.status-pending { background: var(--color-warning); color: var(--text-primary); opacity: 0.8; }

.strategy-status {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--spacing-sm);
}

.strategy-actions-header {
  display: flex;
  gap: var(--spacing-xs);
}

.instance-summary {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
}

.instance-stats {
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
  color: var(--color-brand);
}

.stat-label {
  display: block;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  text-transform: uppercase;
}

.recent-configs {
  margin-bottom: var(--spacing-lg);
}

.configs-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.config-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.config-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-primary);
  border-radius: var(--radius-md);
  font-size: var(--font-size-md);
}

.config-info {
  flex: 1;
}

.config-name {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.config-date {
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
  margin-left: var(--spacing-sm);
}

.config-actions {
  display: flex;
  gap: var(--spacing-xs);
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

.btn-success {
  background: var(--color-success);
  color: var(--text-primary);
}

.btn-success:hover {
  background: var(--color-success-hover);
}

.btn-danger {
  background: var(--color-danger);
  color: var(--text-primary);
}

.btn-danger:hover {
  background: var(--color-danger-hover);
}

.btn:disabled {
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

/* Dialog Styles */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog-content {
  background: var(--bg-secondary);
  border-radius: var(--radius-lg);
  max-width: 800px;
  width: 90vw;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.dialog-header h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-xl);
}

.btn-close {
  background: none;
  border: none;
  font-size: var(--font-size-xl);
  color: var(--text-secondary);
  cursor: pointer;
  padding: var(--spacing-xs);
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
}

.btn-close:hover {
  background: var(--bg-quaternary);
  color: var(--text-primary);
}

.dialog-body {
  padding: var(--spacing-lg);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
}

.form-section {
  margin-bottom: var(--spacing-xl);
}

.form-section h3 {
  margin: 0 0 var(--spacing-lg) 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}

.parameter-group {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.parameter-group h4 {
  margin: 0 0 var(--spacing-lg) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.form-group label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
}

.param-description {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-normal);
  font-style: italic;
  margin-top: 2px;
}

.form-input {
  padding: var(--spacing-sm);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.form-input:focus {
  outline: none;
  border-color: var(--color-brand);
  box-shadow: 0 0 0 2px rgba(255, 107, 53, 0.1);
}

.parameters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-lg);
}

.checkbox-group {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.checkbox-group input[type="checkbox"] {
  width: 16px;
  height: 16px;
}

.checkbox-group label {
  cursor: pointer;
  user-select: none;
}

/* List View Styles */
.strategies-list {
  background: var(--bg-tertiary);
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--border-primary);
}

.list-header {
  display: grid;
  grid-template-columns: 2fr 0.8fr 1fr 0.8fr 0.8fr 0.8fr 1fr 1.2fr;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background: var(--bg-quaternary);
  border-bottom: 1px solid var(--border-primary);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  text-transform: uppercase;
}

.strategy-row {
  display: grid;
  grid-template-columns: 2fr 0.8fr 1fr 0.8fr 0.8fr 0.8fr 1fr 1.2fr;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  transition: var(--transition-fast);
  align-items: center;
}

.strategy-row:hover {
  background: var(--bg-primary);
}

.strategy-row:last-child {
  border-bottom: none;
}

.list-col {
  display: flex;
  align-items: center;
}

.col-name {
  flex-direction: column;
  align-items: flex-start;
}

.strategy-name-info .strategy-name {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
}

.strategy-name-info .strategy-description {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.col-risk,
.col-author,
.col-version,
.col-backtests,
.col-live,
.col-pnl {
  justify-content: center;
  text-align: center;
}

.col-actions {
  justify-content: center;
}

.list-actions {
  display: flex;
  gap: var(--spacing-xs);
}

.btn-sm {
  padding: var(--spacing-xs) var(--spacing-sm);
  font-size: var(--font-size-sm);
  min-width: 32px;
  height: 32px;
}

.stat-number.positive {
  color: var(--color-success);
}

/* Responsive List View */
@media (max-width: 1200px) {
  .list-header,
  .strategy-row {
    grid-template-columns: 2fr 0.8fr 1fr 0.8fr 1.2fr;
  }
  
  .col-author,
  .col-version,
  .col-live {
    display: none;
  }
}

@media (max-width: 768px) {
  .form-row {
    grid-template-columns: 1fr;
  }
  
  .parameters-grid {
    grid-template-columns: 1fr;
  }
  
  .dialog-content {
    width: 95vw;
    margin: var(--spacing-md);
  }
  
  .list-header,
  .strategy-row {
    grid-template-columns: 2fr 1fr 1.2fr;
    gap: var(--spacing-sm);
  }
  
  .col-risk,
  .col-author,
  .col-version,
  .col-backtests,
  .col-live {
    display: none;
  }
  
  .header-actions {
    flex-direction: column;
    gap: var(--spacing-sm);
  }
  
  .view-toggle {
    order: 2;
  }
}
</style>
