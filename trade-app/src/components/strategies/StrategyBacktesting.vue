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
          :disabled="availableStrategies.length === 0"
        >
          <span class="icon">▶️</span>
          Run New Backtest
        </button>
      </div>
    </div>

    <!-- Run Backtest Dialog -->
    <div v-if="showRunDialog" class="dialog-overlay" @click="closeDialog">
      <div class="dialog-content" @click.stop>
        <div class="dialog-header">
          <h2>Run New Backtest</h2>
          <button class="btn-close" @click="closeDialog">×</button>
        </div>
        
        <div class="dialog-body">
          <!-- Strategy Selection -->
          <div class="form-section">
            <h3>Select Strategy</h3>
            <div class="strategy-selector">
              <div 
                v-for="strategy in availableStrategies" 
                :key="strategy.strategy_id"
                class="strategy-option"
                :class="{ active: selectedStrategy?.strategy_id === strategy.strategy_id }"
                @click="selectStrategy(strategy)"
              >
                <div class="strategy-info">
                  <h4>{{ strategy.name }}</h4>
                  <p>{{ strategy.description || 'No description' }}</p>
                </div>
                <div class="strategy-meta">
                  <span class="meta-badge">{{ strategy.risk_level || 'Medium' }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Parameters Configuration -->
          <div v-if="selectedStrategy && strategyParameters" class="form-section">
            <h3>Configure Parameters</h3>
            
            <!-- Framework Parameters -->
            <div class="parameter-group">
              <h4>Framework Settings</h4>
              <div class="form-row">
                <div class="form-group">
                  <label>Start Date</label>
                  <input 
                    type="date" 
                    v-model="backtestConfig.start_date"
                    class="form-input"
                  />
                </div>
                <div class="form-group">
                  <label>End Date</label>
                  <input 
                    type="date" 
                    v-model="backtestConfig.end_date"
                    class="form-input"
                  />
                </div>
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label>Data Timeframe</label>
                  <select v-model="backtestConfig.timeframe" class="form-input">
                    <option value="1m">1 Minute</option>
                    <option value="5m">5 Minutes</option>
                    <option value="15m">15 Minutes</option>
                    <option value="30m">30 Minutes</option>
                    <option value="1h">1 Hour</option>
                    <option value="4h">4 Hours</option>
                    <option value="D">Daily</option>
                  </select>
                </div>
                <div class="form-group">
                  <label>Initial Capital ($)</label>
                  <input 
                    type="number" 
                    v-model="backtestConfig.initial_capital"
                    min="1000"
                    step="1000"
                    class="form-input"
                  />
                </div>
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label>Speed Multiplier</label>
                  <select v-model="backtestConfig.speed_multiplier" class="form-input">
                    <option value="1">1x (Real-time)</option>
                    <option value="10">10x (Fast)</option>
                    <option value="100">100x (Very Fast)</option>
                    <option value="1000">1000x (Maximum)</option>
                  </select>
                </div>
                <div class="form-group">
                  <!-- Empty space for layout balance -->
                </div>
              </div>
            </div>

            <!-- Strategy Parameters -->
            <div class="parameter-group">
              <h4>Strategy Parameters</h4>
              <div class="parameters-grid">
                <div
                  v-for="(param, paramName) in strategyParameters.parameters"
                  :key="paramName"
                  class="form-group"
                >
                  <label>
                    {{ formatParameterName(paramName) }}
                    <span v-if="param.description" class="param-description">
                      {{ param.description }}
                    </span>
                  </label>

                  <!-- String Input -->
                  <input
                    v-if="param.type === 'string'"
                    type="text"
                    v-model="strategyConfig[paramName]"
                    :placeholder="param.default || 'Enter value'"
                    class="form-input"
                  />

                  <!-- Integer Input -->
                  <input
                    v-else-if="param.type === 'integer'"
                    type="number"
                    v-model="strategyConfig[paramName]"
                    :min="param.min"
                    :max="param.max"
                    step="1"
                    :placeholder="param.default?.toString() || '0'"
                    class="form-input"
                  />

                  <!-- Float Input -->
                  <input
                    v-else-if="param.type === 'float'"
                    type="number"
                    v-model="strategyConfig[paramName]"
                    :min="param.min"
                    :max="param.max"
                    step="0.1"
                    :placeholder="param.default?.toString() || '0.0'"
                    class="form-input"
                  />

                  <!-- Boolean Input -->
                  <div v-else-if="param.type === 'boolean'" class="checkbox-group">
                    <input
                      type="checkbox"
                      :id="paramName"
                      v-model="strategyConfig[paramName]"
                    />
                    <label :for="paramName">{{ param.default ? 'Enabled' : 'Disabled' }}</label>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="dialog-footer">
          <button class="btn btn-outline" @click="closeDialog">Cancel</button>
          <button 
            class="btn btn-primary" 
            @click="runBacktest"
            :disabled="!selectedStrategy || isRunningBacktest"
            :class="{ loading: isRunningBacktest }"
          >
            <span v-if="isRunningBacktest">Running...</span>
            <span v-else>Run Backtest</span>
          </button>
        </div>
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
        Run your first backtest to see how your strategies perform with historical data. Click "Run New Backtest" above to get started.
      </p>
      <div class="empty-actions">
        <button 
          class="btn btn-primary"
          @click="showRunDialog = true"
          :disabled="availableStrategies.length === 0"
        >
          <span class="icon">▶️</span>
          Run New Backtest
        </button>
        <button 
          class="btn btn-outline"
          @click="navigateToLibrary"
        >
          <span class="icon">📚</span>
          Strategy Library
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
    const availableStrategies = ref([])
    const loading = ref(true)
    const error = ref(null)
    
    // Dialog states
    const showRunDialog = ref(false)
    const selectedStrategy = ref(null)
    const strategyParameters = ref(null)
    const isRunningBacktest = ref(false)
    
    // Configuration state
    const backtestConfig = ref({
      start_date: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0], // 30 days ago
      end_date: new Date().toISOString().split('T')[0], // today
      initial_capital: 10000,
      speed_multiplier: 1000,
      timeframe: '1m' // Default to 1 minute timeframe
    })
    
    const strategyConfig = ref({})

    // Load data
    const loadBacktestResults = async () => {
      try {
        loading.value = true
        const response = await api.get('/api/strategies/backtest/runs')
        backtestResults.value = response.data || []
      } catch (err) {
        console.error('Error loading backtest results:', err)
        error.value = 'Failed to load backtest results'
      } finally {
        loading.value = false
      }
    }

    const loadAvailableStrategies = async () => {
      try {
        const strategies = await api.getStrategies()
        availableStrategies.value = strategies || []
      } catch (err) {
        console.error('Error loading strategies:', err)
      }
    }

    // Dialog functions
    const closeDialog = () => {
      showRunDialog.value = false
      selectedStrategy.value = null
      strategyParameters.value = null
      strategyConfig.value = {}
    }

    const selectStrategy = async (strategy) => {
      selectedStrategy.value = strategy
      
      try {
        // Load strategy parameters
        const response = await api.get(`/api/strategies/${strategy.strategy_id}/parameters`)
        if (response.data.success) {
          strategyParameters.value = response.data
          
          // Initialize strategy config with default values
          const config = {}
          Object.entries(response.data.parameters).forEach(([paramName, param]) => {
            config[paramName] = param.default
          })
          strategyConfig.value = config
        }
      } catch (err) {
        console.error('Error loading strategy parameters:', err)
      }
    }

    const runBacktest = async () => {
      if (!selectedStrategy.value) return
      
      try {
        isRunningBacktest.value = true
        
        // Prepare backtest request with template-based approach
        const backtestRequest = {
          parameters: strategyConfig.value,
          start_date: backtestConfig.value.start_date,
          end_date: backtestConfig.value.end_date,
          initial_capital: backtestConfig.value.initial_capital,
          speed_multiplier: backtestConfig.value.speed_multiplier,
          timeframe: backtestConfig.value.timeframe  // Include timeframe
        }
        
        const response = await api.post(`/api/strategies/${selectedStrategy.value.strategy_id}/backtest`, backtestRequest)
        
        if (response.data.success) {
          console.log('Backtest started successfully:', response.data)
          closeDialog()
          await loadBacktestResults() // Refresh results
        } else {
          console.error('Backtest failed:', response.data.error)
        }
      } catch (err) {
        console.error('Error running backtest:', err)
      } finally {
        isRunningBacktest.value = false
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
      if (number === null || number === undefined) return '0.00'
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
      // Navigate to detailed backtest view with strategy ID and run ID
      // Pass the run_id as a query parameter so we can load specific backtest results
      router.push({
        path: `/strategies/backtest/${result.strategy_id || result.run_id}`,
        query: { run_id: result.run_id }
      })
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

    // Parameter formatting utility
    const formatParameterName = (paramName) => {
      return paramName
        .replace(/_/g, ' ')
        .replace(/\b\w/g, l => l.toUpperCase())
    }

    // Lifecycle
    onMounted(async () => {
      await loadBacktestResults()
      await loadAvailableStrategies()
    })

    return {
      // State
      backtestResults,
      availableStrategies,
      loading,
      error,
      
      // Dialog states
      showRunDialog,
      selectedStrategy,
      strategyParameters,
      isRunningBacktest,
      backtestConfig,
      strategyConfig,
      
      // Dialog functions
      closeDialog,
      selectStrategy,
      runBacktest,
      
      // Helper functions
      formatDateRange,
      formatPercentage,
      formatNumber,
      formatDuration,
      getPnLClass,
      formatParameterName,
      
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

.workflow-steps {
  display: flex;
  justify-content: center;
  gap: var(--spacing-xl);
  margin: var(--spacing-xl) 0;
  flex-wrap: wrap;
}

.workflow-step {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  max-width: 200px;
  text-align: left;
}

.step-number {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: var(--color-brand);
  color: var(--text-primary);
  border-radius: 50%;
  font-weight: var(--font-weight-bold);
  font-size: var(--font-size-sm);
  flex-shrink: 0;
}

.step-content h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.step-content p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
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

.strategy-selector {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.strategy-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition-normal);
}

.strategy-option:hover {
  border-color: var(--color-brand);
  background: var(--bg-tertiary);
}

.strategy-option.active {
  border-color: var(--color-brand);
  background: rgba(255, 107, 53, 0.1);
}

.strategy-info h4 {
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.strategy-info p {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.meta-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  background: var(--bg-quaternary);
  color: var(--text-secondary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
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
}
</style>
