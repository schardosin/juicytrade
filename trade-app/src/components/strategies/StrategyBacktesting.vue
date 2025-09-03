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
    <BacktestConfigDialog
      :visible="showRunDialog"
      :strategy="selectedStrategy"
      @close="closeDialog"
      @backtest-started="handleBacktestStarted"
    />

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
              <span class="metric-value" :class="getPnLClass(result.avg_trade_pnl)">
                {{ formatCurrency(result.avg_trade_pnl) }}
              </span>
            </div>
            <div class="metric-item">
              <span class="metric-label">Volatility</span>
              <span class="metric-value">{{ formatPercentage(result.volatility) }}</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">Duration</span>
              <span class="metric-value">{{ formatBacktestDuration(result.start_date, result.end_date) }}</span>
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
import { useRouter, useRoute } from 'vue-router'
import { api } from '../../services/api.js'
import BacktestConfigDialog from './BacktestConfigDialog.vue'

export default {
  name: 'StrategyBacktesting',
  components: {
    BacktestConfigDialog
  },
  setup() {
    const router = useRouter()
    const route = useRoute()
    
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
        const rawResults = response.data || []
        
        // Process each result to extract metrics from nested structure
        backtestResults.value = rawResults.map(result => {
          // Parse results JSON if it's a string
          let parsedResults = result.results
          if (typeof parsedResults === 'string') {
            try {
              parsedResults = JSON.parse(parsedResults)
            } catch (e) {
              console.error('Error parsing results JSON:', e)
              parsedResults = null
            }
          }
          
          // Extract metrics from nested structure (same as StrategyBacktest.vue)
          const extractedMetrics = {}
          if (parsedResults && parsedResults.metrics) {
            const metrics = parsedResults.metrics
            
            // Extract P&L metrics
            if (metrics.pnl) {
              extractedMetrics.total_pnl = metrics.pnl.total_pnl || 0
              extractedMetrics.total_return = metrics.pnl.total_return || 0
              extractedMetrics.max_profit = metrics.pnl.max_profit || 0
              extractedMetrics.max_loss = metrics.pnl.max_loss || 0
              extractedMetrics.largest_win = metrics.pnl.largest_win || 0
              extractedMetrics.largest_loss = metrics.pnl.largest_loss || 0
            }
            
            // Extract trading metrics
            if (metrics.trading) {
              extractedMetrics.total_trades = metrics.trading.total_trades || 0
              extractedMetrics.winning_trades = metrics.trading.winning_trades || 0
              extractedMetrics.losing_trades = metrics.trading.losing_trades || 0
              extractedMetrics.win_rate = metrics.trading.win_rate || 0
            }
            
            // Extract risk metrics
            if (metrics.risk) {
              extractedMetrics.max_drawdown = metrics.risk.max_drawdown || 0
              extractedMetrics.sharpe_ratio = metrics.risk.sharpe_ratio || 0
              extractedMetrics.sortino_ratio = metrics.risk.sortino_ratio || 0
              extractedMetrics.calmar_ratio = metrics.risk.calmar_ratio || 0
            }
            
            // Calculate derived metrics
            if (extractedMetrics.total_trades > 0) {
              // Avg trade P&L in dollars (not percentage)
              extractedMetrics.avg_trade_pnl = extractedMetrics.total_pnl / extractedMetrics.total_trades
              // Avg trade return as percentage per trade
              extractedMetrics.avg_trade_return = (extractedMetrics.total_pnl / result.initial_capital) / extractedMetrics.total_trades
            } else {
              extractedMetrics.avg_trade_pnl = 0
              extractedMetrics.avg_trade_return = 0
            }
            
            // Calculate volatility from equity curve if available
            if (parsedResults && parsedResults.equity_curve && parsedResults.equity_curve.length > 1) {
              const equityValues = parsedResults.equity_curve.map(point => point.equity)
              const returns = []
              for (let i = 1; i < equityValues.length; i++) {
                if (equityValues[i-1] > 0) {
                  returns.push((equityValues[i] - equityValues[i-1]) / equityValues[i-1])
                }
              }
              if (returns.length > 0) {
                const meanReturn = returns.reduce((sum, ret) => sum + ret, 0) / returns.length
                const variance = returns.reduce((sum, ret) => sum + Math.pow(ret - meanReturn, 2), 0) / returns.length
                extractedMetrics.volatility = Math.sqrt(variance) * Math.sqrt(252) // Annualized volatility
              } else {
                extractedMetrics.volatility = 0
              }
            } else {
              // Fallback: estimate from drawdown (simplified)
              extractedMetrics.volatility = extractedMetrics.max_drawdown * 1.5
            }
          }
          
          // Return result with extracted metrics merged at top level
          return {
            ...result,
            ...extractedMetrics,
            // Keep original nested structure for compatibility
            results: parsedResults
          }
        })
        
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
      // Handle case where endTime might be null/undefined (use current time)
      const start = new Date(startTime)
      const end = endTime ? new Date(endTime) : new Date()
      const diffMs = end - start
      
      // If negative duration, it means the backtest is still running or dates are wrong
      if (diffMs < 0) {
        return 'Running...'
      }
      
      const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
      const diffHours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
      const diffMins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60))
      
      // For backtests, show the time period covered, not execution time
      if (diffDays > 0) {
        return `${diffDays}d`
      } else if (diffHours > 0) {
        return `${diffHours}h`
      } else if (diffMins > 0) {
        return `${diffMins}m`
      } else {
        return '<1m'
      }
    }

    const formatCurrency = (value) => {
      if (value === null || value === undefined) return '$0.00'
      return `$${value.toFixed(2)}`
    }

    const formatBacktestDuration = (startDate, endDate) => {
      if (!startDate || !endDate) return 'N/A'
      
      const start = new Date(startDate)
      const end = new Date(endDate)
      const diffMs = end - start
      
      if (diffMs < 0) return 'Invalid'
      
      const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
      
      if (diffDays > 0) {
        return `${diffDays}d`
      } else {
        const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
        return diffHours > 0 ? `${diffHours}h` : '<1h'
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

    // Auto-open logic for query parameters
    const handleAutoOpen = async () => {
      const { strategy_id, auto_open } = route.query
      
      if (auto_open === 'true' && strategy_id) {
        // Wait for strategies to load first
        await loadAvailableStrategies()
        
        // Find the strategy by ID
        const strategy = availableStrategies.value.find(s => s.strategy_id === strategy_id)
        if (strategy) {
          selectedStrategy.value = strategy
          showRunDialog.value = true
        }
      }
    }

    // Lifecycle
    onMounted(async () => {
      await loadBacktestResults()
      await loadAvailableStrategies()
      
      // Handle auto-open after data is loaded
      await handleAutoOpen()
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
      formatCurrency,
      formatBacktestDuration,
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
