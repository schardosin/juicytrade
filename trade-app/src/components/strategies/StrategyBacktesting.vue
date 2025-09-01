<template>
  <div class="backtesting">
    <!-- Header -->
    <div class="backtesting-header">
      <div class="header-content">
        <h1 class="page-title">📈 Backtesting</h1>
        <p class="page-subtitle">Test your strategies with historical data</p>
      </div>
      <div class="header-actions">
        <button 
          class="btn btn-primary"
          @click="showRunDialog = true"
          :disabled="availableConfigurations.length === 0"
        >
          <span class="icon">▶️</span>
          Run Backtest
        </button>
      </div>
    </div>

    <!-- Backtest Results Grid -->
    <div class="backtest-results-grid" v-if="backtestResults.length > 0">
      <div 
        v-for="result in backtestResults" 
        :key="result.run_id"
        class="backtest-result-card"
      >
        <!-- Result Header -->
        <div class="result-header">
          <div class="result-info">
            <h3 class="strategy-name">{{ result.strategy_name }}</h3>
            <p class="configuration-name">{{ result.configuration_name }}</p>
            <div class="result-meta">
              <span class="meta-item">
                <span class="meta-label">Period:</span>
                {{ formatDateRange(result.start_date, result.end_date) }}
              </span>
              <span class="meta-item">
                <span class="meta-label">Status:</span>
                <span class="status-badge" :class="`status-${result.status}`">
                  {{ result.status }}
                </span>
              </span>
            </div>
          </div>
          <div class="result-actions">
            <button 
              class="btn-icon"
              @click="viewDetails(result)"
              title="View Details"
            >
              📊
            </button>
            <button 
              class="btn-icon"
              @click="deleteResult(result)"
              title="Delete Result"
            >
              🗑️
            </button>
          </div>
        </div>

        <!-- Performance Summary -->
        <div class="performance-summary">
          <div class="performance-stats">
            <div class="stat-item">
              <span class="stat-number" :class="getPnLClass(result.total_return)">
                {{ formatPercentage(result.total_return) }}
              </span>
              <span class="stat-label">Total Return</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ result.total_trades }}</span>
              <span class="stat-label">Total Trades</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ formatPercentage(result.win_rate) }}</span>
              <span class="stat-label">Win Rate</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ formatNumber(result.sharpe_ratio) }}</span>
              <span class="stat-label">Sharpe Ratio</span>
            </div>
          </div>
        </div>

        <!-- Key Metrics -->
        <div class="key-metrics">
          <div class="metrics-grid">
            <div class="metric-item">
              <span class="metric-label">Max Drawdown</span>
              <span class="metric-value negative">{{ formatPercentage(result.max_drawdown) }}</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">Avg Trade</span>
              <span class="metric-value" :class="getPnLClass(result.avg_trade_return)">
                {{ formatPercentage(result.avg_trade_return) }}
              </span>
            </div>
            <div class="metric-item">
              <span class="metric-label">Volatility</span>
              <span class="metric-value">{{ formatPercentage(result.volatility) }}</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">Duration</span>
              <span class="metric-value">{{ formatDuration(result.created_at, result.completed_at) }}</span>
            </div>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="result-actions-footer">
          <button 
            class="btn btn-outline"
            @click="viewDetails(result)"
          >
            <span class="icon">📊</span>
            View Details
          </button>
          <button 
            class="btn btn-secondary"
            @click="runSimilar(result)"
          >
            <span class="icon">🔄</span>
            Run Similar
          </button>
          <button 
            class="btn btn-success"
            @click="deployLive(result)"
            :disabled="result.status !== 'completed' || result.total_return <= 0"
          >
            <span class="icon">⚡</span>
            Deploy Live
          </button>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty-state">
      <div class="empty-icon">📈</div>
      <h3 class="empty-title">No Backtest Results</h3>
      <p class="empty-description">
        Run your first backtest to see how your strategies perform with historical data.
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
          @click="showRunDialog = true"
          :disabled="availableConfigurations.length === 0"
        >
          <span class="icon">▶️</span>
          Run Backtest
        </button>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Loading backtest results...</p>
    </div>

    <!-- Run Backtest Dialog - TODO: Implement -->
    <!-- <BacktestRunDialog 
      v-if="showRunDialog"
      :configurations="availableConfigurations"
      @close="showRunDialog = false"
      @started="handleBacktestStarted"
    /> -->
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../services/api.js'

export default {
  name: 'StrategyBacktesting',
  setup() {
    const router = useRouter()
    
    // Reactive state
    const backtestResults = ref([])
    const availableConfigurations = ref([])
    const loading = ref(true)
    const error = ref(null)
    
    // Dialog states
    const showRunDialog = ref(false)

    // Load data
    const loadBacktestResults = async () => {
      try {
        loading.value = true
        const response = await api.getBacktestRuns()
        backtestResults.value = response || []
      } catch (err) {
        console.error('Error loading backtest results:', err)
        error.value = 'Failed to load backtest results'
      } finally {
        loading.value = false
      }
    }

    const loadAvailableConfigurations = async () => {
      try {
        // Load all configurations that could be backtested
        const strategies = await api.getStrategies()
        const allConfigs = []
        for (const strategy of strategies) {
          const configs = await api.getStrategyConfigurations(strategy.strategy_id)
          allConfigs.push(...configs.map(config => ({
            ...config,
            strategy_name: strategy.name
          })))
        }
        availableConfigurations.value = allConfigs
      } catch (err) {
        console.error('Error loading configurations:', err)
      }
    }

    // Helper functions
    const formatDateRange = (startDate, endDate) => {
      const start = new Date(startDate).toLocaleDateString()
      const end = new Date(endDate).toLocaleDateString()
      return `${start} - ${end}`
    }

    const formatPercentage = (value) => {
      return `${(value * 100).toFixed(2)}%`
    }

    const formatNumber = (number) => {
      return number.toFixed(2)
    }

    const formatDuration = (startTime, endTime) => {
      const start = new Date(startTime)
      const end = new Date(endTime)
      const diffMs = end - start
      const diffMins = Math.floor(diffMs / 60000)
      
      if (diffMins < 60) {
        return `${diffMins}m`
      } else {
        const hours = Math.floor(diffMins / 60)
        const mins = diffMins % 60
        return `${hours}h ${mins}m`
      }
    }

    const getPnLClass = (value) => {
      if (value > 0) return 'positive'
      if (value < 0) return 'negative'
      return 'neutral'
    }

    // Actions
    const viewDetails = (result) => {
      // Navigate to detailed backtest view
      router.push(`/strategies/backtest/${result.run_id}`)
    }

    const deleteResult = async (result) => {
      if (confirm('Are you sure you want to delete this backtest result?')) {
        try {
          await api.deleteBacktestRun(result.run_id)
          await loadBacktestResults() // Refresh the list
        } catch (err) {
          console.error('Error deleting backtest result:', err)
        }
      }
    }

    const runSimilar = (result) => {
      // Pre-populate run dialog with similar settings
      // TODO: Implement with configuration pre-selection
      showRunDialog.value = true
    }

    const deployLive = (result) => {
      // Navigate to live trading with this configuration
      router.push(`/strategies/live?deploy=${result.config_id}`)
    }

    const navigateToLibrary = () => {
      router.push('/strategies/library')
    }

    // Event handlers
    const handleBacktestStarted = () => {
      showRunDialog.value = false
      loadBacktestResults()
    }

    // Lifecycle
    onMounted(async () => {
      await loadBacktestResults()
      await loadAvailableConfigurations()
    })

    return {
      // State
      backtestResults,
      availableConfigurations,
      loading,
      error,
      
      // Dialog states
      showRunDialog,
      
      // Helper functions
      formatDateRange,
      formatPercentage,
      formatNumber,
      formatDuration,
      getPnLClass,
      
      // Actions
      viewDetails,
      deleteResult,
      runSimilar,
      deployLive,
      navigateToLibrary,
      
      // Event handlers
      handleBacktestStarted
    }
  }
}
</script>

<style scoped>
.backtesting {
  padding: var(--spacing-xl);
  height: 100%;
  overflow-y: auto;
  background-color: var(--bg-secondary);
}

.backtesting-header {
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

.backtest-results-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(450px, 1fr));
  gap: var(--spacing-xl);
}

.backtest-result-card {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  transition: var(--transition-normal);
}

.backtest-result-card:hover {
  border-color: var(--color-brand);
  box-shadow: 0 4px 12px rgba(255, 107, 53, 0.1);
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-lg);
}

.result-info {
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

.result-meta {
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

.status-completed { background: var(--color-success); color: var(--text-primary); opacity: 0.8; }
.status-running { background: var(--color-info); color: var(--text-primary); opacity: 0.8; }
.status-failed { background: var(--color-danger); color: var(--text-primary); opacity: 0.8; }

.result-actions {
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
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--spacing-md);
}

.stat-item {
  text-align: center;
}

.stat-number {
  display: block;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.stat-number.positive { color: var(--color-success); }
.stat-number.negative { color: var(--color-danger); }
.stat-number.neutral { color: var(--text-secondary); }

.stat-label {
  display: block;
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
  text-transform: uppercase;
  margin-top: var(--spacing-xs);
}

.key-metrics {
  margin-bottom: var(--spacing-lg);
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-md);
}

.metric-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-primary);
  border-radius: var(--radius-md);
  font-size: var(--font-size-sm);
}

.metric-label {
  color: var(--text-secondary);
}

.metric-value {
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}

.metric-value.positive { color: var(--color-success); }
.metric-value.negative { color: var(--color-danger); }
.metric-value.neutral { color: var(--text-secondary); }

.result-actions-footer {
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
