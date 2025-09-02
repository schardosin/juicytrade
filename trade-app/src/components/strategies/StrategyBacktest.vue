<template>
  <div class="strategy-backtest">
    <!-- Header Section -->
    <div class="backtest-header">
      <div class="header-content">
        <h1 class="backtest-title">
          <i class="pi pi-chart-bar"></i>
          Strategy Backtest
        </h1>
        <p class="backtest-subtitle">
          Historical performance analysis for strategy: <strong>{{ strategyName }}</strong>
        </p>
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-arrow-left"
          label="Back to Library"
          class="p-button-outlined"
          @click="$router.push('/strategies/library')"
        />
        <Button
          icon="pi pi-chart-line"
          label="Dashboard"
          class="p-button-outlined"
          @click="$router.push('/strategies')"
        />
      </div>
    </div>

    <!-- Backtest Configuration -->
    <div class="backtest-config">
      <div class="config-card">
        <h2 class="config-title">
          <i class="pi pi-cog"></i>
          Backtest Configuration
        </h2>
        
        <div class="config-form">
          <!-- Framework Parameters -->
          <div class="config-section">
            <h3 class="section-title">Framework Settings</h3>
            <div class="form-row">
              <div class="form-group">
                <label for="start-date">Start Date</label>
                <Calendar
                  id="start-date"
                  v-model="backtestConfig.start_date"
                  date-format="yy-mm-dd"
                  :max-date="maxStartDate"
                  placeholder="Select start date"
                />
              </div>
              <div class="form-group">
                <label for="end-date">End Date</label>
                <Calendar
                  id="end-date"
                  v-model="backtestConfig.end_date"
                  date-format="yy-mm-dd"
                  :min-date="backtestConfig.start_date"
                  :max-date="new Date()"
                  placeholder="Select end date"
                />
              </div>
            </div>

            <div class="form-row">
              <div class="form-group">
                <label for="timeframe">Data Timeframe</label>
                <Dropdown
                  id="timeframe"
                  v-model="backtestConfig.timeframe"
                  :options="timeframeOptions"
                  option-label="label"
                  option-value="value"
                  placeholder="Select timeframe"
                />
              </div>
              <div class="form-group">
                <label for="initial-capital">Initial Capital ($)</label>
                <InputNumber
                  id="initial-capital"
                  v-model="backtestConfig.initial_capital"
                  :min="1000"
                  :max="1000000"
                  :step="1000"
                  mode="currency"
                  currency="USD"
                  locale="en-US"
                />
              </div>
            </div>

            <div class="form-row">
              <div class="form-group">
                <label for="speed-multiplier">Speed Multiplier</label>
                <Dropdown
                  id="speed-multiplier"
                  v-model="backtestConfig.speed_multiplier"
                  :options="speedOptions"
                  option-label="label"
                  option-value="value"
                  placeholder="Select speed"
                />
              </div>
              <div class="form-group">
                <!-- Empty space for layout balance -->
              </div>
            </div>
          </div>

          <!-- Strategy Parameters -->
          <div v-if="strategyParameters" class="config-section">
            <h3 class="section-title">Strategy Parameters</h3>
            <div class="parameters-grid">
              <div
                v-for="(param, paramName) in strategyParameters.parameters"
                :key="paramName"
                class="form-group"
                :class="{ 'full-width': param.type === 'string' && paramName === 'symbol' }"
              >
                <label :for="paramName" class="parameter-label">
                  {{ formatParameterName(paramName) }}
                  <span v-if="param.description" class="parameter-description">
                    {{ param.description }}
                  </span>
                </label>

                <!-- String Input -->
                <InputText
                  v-if="param.type === 'string'"
                  :id="paramName"
                  v-model="strategyConfig[paramName]"
                  :placeholder="param.default || 'Enter value'"
                  class="parameter-input"
                />

                <!-- Integer Input -->
                <InputNumber
                  v-else-if="param.type === 'integer'"
                  :id="paramName"
                  v-model="strategyConfig[paramName]"
                  :min="param.min"
                  :max="param.max"
                  :step="1"
                  :placeholder="param.default?.toString() || '0'"
                  class="parameter-input"
                />

                <!-- Float Input -->
                <InputNumber
                  v-else-if="param.type === 'float'"
                  :id="paramName"
                  v-model="strategyConfig[paramName]"
                  :min="param.min"
                  :max="param.max"
                  :step="0.1"
                  :min-fraction-digits="1"
                  :max-fraction-digits="2"
                  :placeholder="param.default?.toString() || '0.0'"
                  class="parameter-input"
                />

                <!-- Boolean Input -->
                <div v-else-if="param.type === 'boolean'" class="boolean-input">
                  <Checkbox
                    :id="paramName"
                    v-model="strategyConfig[paramName]"
                    :binary="true"
                  />
                  <label :for="paramName" class="checkbox-label">
                    {{ param.default ? 'Enabled' : 'Disabled' }}
                  </label>
                </div>
              </div>
            </div>
          </div>

          <div class="form-actions">
            <Button
              icon="pi pi-play"
              label="Run Backtest"
              class="p-button-primary"
              @click="runBacktest"
              :loading="isRunningBacktest"
              :disabled="!isConfigValid"
            />
            <Button
              icon="pi pi-refresh"
              label="Reset"
              class="p-button-outlined"
              @click="resetConfig"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Backtest Results -->
    <div v-if="backtestResults" class="backtest-results">
      <div class="results-header">
        <h2 class="results-title">
          <i class="pi pi-chart-line"></i>
          Backtest Results
        </h2>
        <div class="results-actions">
          <Button
            icon="pi pi-download"
            label="Export"
            class="p-button-text p-button-sm"
            @click="exportResults"
          />
        </div>
      </div>

      <!-- Performance Metrics -->
      <div class="metrics-grid">
        <div class="metric-card">
          <div class="metric-icon success">
            <i class="pi pi-dollar"></i>
          </div>
          <div class="metric-content">
            <div class="metric-value" :class="getPnLClass(backtestResults.total_pnl)">
              {{ formatCurrency(backtestResults.total_pnl || 0) }}
            </div>
            <div class="metric-label">Total P&L</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-icon info">
            <i class="pi pi-percentage"></i>
          </div>
          <div class="metric-content">
            <div class="metric-value" :class="getPnLClass(backtestResults.total_return)">
              {{ formatPercentage(backtestResults.total_return || 0) }}
            </div>
            <div class="metric-label">Total Return</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-icon warning">
            <i class="pi pi-chart-bar"></i>
          </div>
          <div class="metric-content">
            <div class="metric-value">{{ backtestResults.total_trades || 0 }}</div>
            <div class="metric-label">Total Trades</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-icon success">
            <i class="pi pi-check"></i>
          </div>
          <div class="metric-content">
            <div class="metric-value">{{ formatPercentage(backtestResults.win_rate || 0) }}</div>
            <div class="metric-label">Win Rate</div>
          </div>
        </div>
      </div>

      <!-- Action Execution Metrics (New) -->
      <div v-if="backtestResults.action_metrics" class="action-metrics">
        <h3 class="metrics-title">
          <i class="pi pi-cog"></i>
          Action Execution Analysis
        </h3>
        <div class="action-metrics-grid">
          <div class="metric-card">
            <div class="metric-icon info">
              <i class="pi pi-play"></i>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ backtestResults.action_metrics.total_actions || 0 }}</div>
              <div class="metric-label">Total Actions</div>
            </div>
          </div>

          <div class="metric-card">
            <div class="metric-icon success">
              <i class="pi pi-check-circle"></i>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ backtestResults.action_metrics.successful_actions || 0 }}</div>
              <div class="metric-label">Successful Actions</div>
            </div>
          </div>

          <div class="metric-card">
            <div class="metric-icon danger">
              <i class="pi pi-times-circle"></i>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ backtestResults.action_metrics.failed_actions || 0 }}</div>
              <div class="metric-label">Failed Actions</div>
            </div>
          </div>

          <div class="metric-card">
            <div class="metric-icon warning">
              <i class="pi pi-percentage"></i>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ formatPercentage(backtestResults.action_metrics.action_success_rate || 0) }}</div>
              <div class="metric-label">Success Rate</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Strategy Checkpoints (New) -->
      <div v-if="backtestResults.checkpoints && backtestResults.checkpoints.length" class="checkpoints-section">
        <h3 class="checkpoints-title">
          <i class="pi pi-map-marker"></i>
          Strategy Checkpoints
        </h3>
        <div class="checkpoints-list">
          <div
            v-for="checkpoint in backtestResults.checkpoints.slice(0, 10)"
            :key="checkpoint.name"
            class="checkpoint-item"
          >
            <div class="checkpoint-icon">
              <i class="pi pi-circle-fill"></i>
            </div>
            <div class="checkpoint-content">
              <div class="checkpoint-name">{{ checkpoint.name }}</div>
              <div class="checkpoint-time">{{ formatDate(checkpoint.timestamp) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Trade History -->
      <div class="trade-history">
        <h3 class="history-title">Trade History</h3>
        <div class="history-table">
          <div class="table-header">
            <div class="col-date">Date</div>
            <div class="col-symbol">Symbol</div>
            <div class="col-action">Action</div>
            <div class="col-quantity">Quantity</div>
            <div class="col-price">Price</div>
            <div class="col-pnl">P&L</div>
          </div>
          <div
            v-for="trade in backtestResults.trades"
            :key="trade.id"
            class="table-row"
          >
            <div class="col-date">{{ formatDate(trade.timestamp) }}</div>
            <div class="col-symbol">{{ trade.symbol }}</div>
            <div class="col-action">
              <span class="action-badge" :class="trade.action.toLowerCase()">
                {{ trade.action }}
              </span>
            </div>
            <div class="col-quantity">{{ trade.quantity }}</div>
            <div class="col-price">{{ formatCurrency(trade.price) }}</div>
            <div class="col-pnl" :class="getPnLClass(trade.pnl)">
              {{ formatCurrency(trade.pnl) }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Error State -->
    <div v-if="backtestError" class="error-state">
      <i class="pi pi-exclamation-triangle"></i>
      <h3>Backtest Failed</h3>
      <p>{{ backtestError }}</p>
      <Button
        label="Try Again"
        class="p-button-primary"
        @click="backtestError = null"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useStrategyData } from '../../composables/useStrategyData.js'
import { useNotifications } from '../../composables/useNotifications.js'
import api from '../../services/api.js'

export default {
  name: 'StrategyBacktest',
  setup() {
    const route = useRoute()
    const router = useRouter()
    const { showSuccess, showError } = useNotifications()
    const { getMyStrategies, getStrategyBacktest, isLoading } = useStrategyData()

    const strategies = getMyStrategies()
    const strategyId = computed(() => route.params.id)

    const strategyName = computed(() => {
      const strategy = strategies.value.find(s => s.strategy_id === strategyId.value)
      return strategy?.name || 'Unknown Strategy'
    })

    // Strategy parameters
    const strategyParameters = ref(null)
    const strategyConfig = ref({})

    // Backtest configuration
    const backtestConfig = ref({
      start_date: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000), // 30 days ago
      end_date: new Date(),
      initial_capital: 10000,
      speed_multiplier: 1,
      timeframe: "1m"  // Use framework standard timeframe
    })

    // Speed options
    const speedOptions = [
      { label: '1x (Real-time)', value: 1 },
      { label: '10x (Fast)', value: 10 },
      { label: '100x (Very Fast)', value: 100 },
      { label: '1000x (Maximum)', value: 1000 }
    ]

    // Timeframe options (using framework standard)
    const timeframeOptions = [
      { label: '1 Minute', value: '1m' },
      { label: '5 Minutes', value: '5m' },
      { label: '15 Minutes', value: '15m' },
      { label: '30 Minutes', value: '30m' },
      { label: '1 Hour', value: '1h' },
      { label: '4 Hours', value: '4h' },
      { label: 'Daily', value: 'D' }
    ]

    // State
    const backtestResults = ref(null)
    const backtestError = ref(null)
    const isRunningBacktest = computed(() => isLoading(`backtest_${strategyId.value}`).value)

    // Computed properties
    const maxStartDate = computed(() => {
      const today = new Date()
      today.setDate(today.getDate() - 1) // Yesterday
      return today
    })

    const isConfigValid = computed(() => {
      return backtestConfig.value.start_date &&
             backtestConfig.value.end_date &&
             backtestConfig.value.initial_capital > 0 &&
             backtestConfig.value.start_date < backtestConfig.value.end_date
    })

    // Load strategy parameters
    const loadStrategyParameters = async () => {
      try {
        const response = await api.get(`/api/strategies/${strategyId.value}/parameters`)
        if (response.data.success) {
          strategyParameters.value = response.data
          
          // Initialize strategy config with default values
          const config = {}
          Object.entries(response.data.parameters).forEach(([paramName, param]) => {
            config[paramName] = param.default
          })
          strategyConfig.value = config
          
          console.log('Strategy parameters loaded:', response.data)
        }
      } catch (error) {
        console.error('Failed to load strategy parameters:', error)
        showError(
          'Failed to load strategy configuration',
          'Configuration Error'
        )
      }
    }

    // Initialize on mount
    onMounted(() => {
      if (strategyId.value) {
        loadStrategyParameters()
      }
    })

    // Methods
    const runBacktest = async () => {
      if (!isConfigValid.value) {
        showError(
          'Please check your backtest configuration',
          'Configuration Error'
        )
        return
      }

      try {
        backtestError.value = null
        backtestResults.value = null

        // NEW TEMPLATE-BASED APPROACH: Include strategy parameters directly
        const config = {
          parameters: strategyConfig.value, // Include strategy parameters
          start_date: backtestConfig.value.start_date.toISOString().split('T')[0],
          end_date: backtestConfig.value.end_date.toISOString().split('T')[0],
          initial_capital: backtestConfig.value.initial_capital,
          speed_multiplier: backtestConfig.value.speed_multiplier,
          timeframe: backtestConfig.value.timeframe  // Include timeframe
        }

        console.log('Running backtest with config:', config)
        const result = await getStrategyBacktest(strategyId.value, config)
        
        if (result.success) {
          // Transform the new comprehensive results to match UI expectations
          const transformedResults = {
            // Extract metrics from the new nested structure
            total_pnl: result.data.metrics.pnl.total_pnl,
            total_return: result.data.metrics.pnl.total_return,
            total_trades: result.data.metrics.trading.total_trades,
            win_rate: result.data.metrics.trading.win_rate,
            max_profit: result.data.metrics.pnl.max_profit,
            max_loss: result.data.metrics.pnl.max_loss,
            sharpe_ratio: result.data.metrics.risk.sharpe_ratio,
            max_drawdown: result.data.metrics.risk.max_drawdown,
            
            // Use the trades array directly
            trades: result.data.trades || [],
            
            // Add additional data from new system
            action_metrics: result.data.metrics.actions,
            equity_curve: result.data.equity_curve,
            checkpoints: result.data.checkpoints,
            action_log: result.data.action_log,
            
            // Keep original structure for backward compatibility
            ...result.data
          }
          
          backtestResults.value = transformedResults
          showSuccess(
            'Backtest completed successfully',
            'Backtest Complete'
          )
        } else {
          throw new Error(result.error || 'Backtest failed')
        }
      } catch (error) {
        console.error('Backtest failed:', error)
        backtestError.value = error.message
        showError(
          `Backtest failed: ${error.message}`,
          'Backtest Error'
        )
      }
    }

    const resetConfig = () => {
      backtestConfig.value = {
        start_date: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000),
        end_date: new Date(),
        initial_capital: 10000,
        speed_multiplier: 1
      }
      backtestResults.value = null
      backtestError.value = null
    }

    const exportResults = () => {
      if (!backtestResults.value) return

      const data = {
        strategy_name: strategyName.value,
        backtest_config: backtestConfig.value,
        results: backtestResults.value,
        exported_at: new Date().toISOString()
      }

      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${strategyName.value}_backtest_${Date.now()}.json`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)

      showSuccess(
        'Backtest results exported successfully',
        'Export Complete'
      )
    }

    // Utility methods
    const getPnLClass = (value) => {
      if (!value) return ''
      return value >= 0 ? 'positive' : 'negative'
    }

    const formatCurrency = (amount) => {
      if (amount === null || amount === undefined) return '$0.00'
      const value = Math.abs(amount || 0)
      const sign = amount >= 0 ? '+' : '-'
      return `${sign}$${value.toLocaleString('en-US', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
      })}`
    }

    const formatPercentage = (value) => {
      if (value === null || value === undefined) return '0.00%'
      const percentage = (value || 0) * 100
      const sign = percentage >= 0 ? '+' : ''
      return `${sign}${percentage.toFixed(2)}%`
    }

    const formatDate = (dateString) => {
      if (!dateString) return '--'
      
      const date = new Date(dateString)
      return date.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      })
    }

    // Parameter formatting utility
    const formatParameterName = (paramName) => {
      return paramName
        .replace(/_/g, ' ')
        .replace(/\b\w/g, l => l.toUpperCase())
    }

    return {
      // Data
      strategyId,
      strategyName,
      strategyParameters,
      strategyConfig,
      backtestConfig,
      speedOptions,
      timeframeOptions,  // Add timeframe options
      backtestResults,
      backtestError,

      // Computed
      maxStartDate,
      isConfigValid,
      isRunningBacktest,

      // Methods
      runBacktest,
      resetConfig,
      exportResults,
      loadStrategyParameters,

      // Utility methods
      getPnLClass,
      formatCurrency,
      formatPercentage,
      formatDate,
      formatParameterName
    }
  }
}
</script>

<style scoped>
.strategy-backtest {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header Section */
.backtest-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-xl);
  padding-bottom: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.header-content {
  flex: 1;
}

.backtest-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.backtest-title i {
  color: var(--color-brand);
}

.backtest-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Coming Soon Section */
.coming-soon-section {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}

.coming-soon-content {
  text-align: center;
  max-width: 600px;
  padding: var(--spacing-2xl);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
}

.coming-soon-icon {
  font-size: var(--font-size-4xl);
  color: var(--color-brand);
  margin-bottom: var(--spacing-lg);
}

.coming-soon-content h2 {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.coming-soon-content > p {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  line-height: 1.6;
  margin: 0 0 var(--spacing-xl) 0;
}

.planned-features {
  text-align: left;
  margin-bottom: var(--spacing-xl);
}

.planned-features h3 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.planned-features ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.planned-features li {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) 0;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.planned-features li::before {
  content: '✓';
  color: var(--color-success);
  font-weight: var(--font-weight-bold);
  font-size: var(--font-size-lg);
}

.back-actions {
  display: flex;
  gap: var(--spacing-md);
  justify-content: center;
}

/* Backtest Configuration */
.backtest-config {
  margin-bottom: var(--spacing-xl);
}

.config-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
}

.config-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-lg) 0;
}

.config-title i {
  color: var(--color-brand);
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-lg);
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

.form-actions {
  display: flex;
  gap: var(--spacing-md);
  justify-content: flex-start;
}

/* Configuration Sections */
.config-section {
  padding: var(--spacing-lg) 0;
  border-bottom: 1px solid var(--border-primary);
}

.config-section:last-child {
  border-bottom: none;
}

.section-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-lg) 0;
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.section-title::before {
  content: '';
  width: 4px;
  height: 20px;
  background: var(--color-brand);
  border-radius: var(--radius-sm);
}

/* Strategy Parameters */
.parameters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: var(--spacing-lg);
}

.parameter-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  margin-bottom: var(--spacing-xs);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.parameter-description {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-normal);
  font-style: italic;
}

.parameter-input {
  width: 100%;
}

.boolean-input {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) 0;
}

.checkbox-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
}

.form-group.full-width {
  grid-column: 1 / -1;
}

/* Parameter input focus states */
.parameter-input:focus {
  border-color: var(--color-brand);
  box-shadow: 0 0 0 2px rgba(var(--color-brand-rgb), 0.1);
}

/* Parameter validation states */
.parameter-input.p-invalid {
  border-color: var(--color-danger);
}

.parameter-input.p-invalid:focus {
  box-shadow: 0 0 0 2px rgba(var(--color-danger-rgb), 0.1);
}

/* Backtest Results */
.backtest-results {
  margin-bottom: var(--spacing-xl);
}

.results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
}

.results-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0;
}

.results-title i {
  color: var(--color-brand);
}

.results-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Performance Metrics */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-lg);
  margin-bottom: var(--spacing-xl);
}

.metric-card {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
}

.metric-card:hover {
  border-color: var(--border-secondary);
  box-shadow: var(--shadow-sm);
}

.metric-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  font-size: var(--font-size-xl);
}

.metric-icon.success {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.metric-icon.warning {
  background: rgba(245, 158, 11, 0.1);
  color: var(--color-warning);
}

.metric-icon.info {
  background: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
}

.metric-icon.danger {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.metric-content {
  flex: 1;
}

.metric-value {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin-bottom: 2px;
}

.metric-value.positive {
  color: var(--color-success);
}

.metric-value.negative {
  color: var(--color-danger);
}

.metric-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

/* Action Metrics Section */
.action-metrics {
  margin-bottom: var(--spacing-xl);
}

.metrics-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-lg) 0;
}

.metrics-title i {
  color: var(--color-brand);
}

.action-metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-md);
}

/* Checkpoints Section */
.checkpoints-section {
  margin-bottom: var(--spacing-xl);
}

.checkpoints-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-lg) 0;
}

.checkpoints-title i {
  color: var(--color-brand);
}

.checkpoints-list {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  max-height: 300px;
  overflow-y: auto;
}

.checkpoint-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-sm) 0;
  border-bottom: 1px solid var(--border-primary);
}

.checkpoint-item:last-child {
  border-bottom: none;
}

.checkpoint-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  color: var(--color-brand);
  font-size: var(--font-size-xs);
}

.checkpoint-content {
  flex: 1;
}

.checkpoint-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  margin-bottom: 2px;
}

.checkpoint-time {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}

/* Trade History */
.trade-history {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.history-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0;
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.history-table {
  max-height: 400px;
  overflow-y: auto;
}

.table-header {
  display: grid;
  grid-template-columns: 1.5fr 1fr 1fr 1fr 1fr 1fr;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.table-row {
  display: grid;
  grid-template-columns: 1.5fr 1fr 1fr 1fr 1fr 1fr;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  transition: var(--transition-normal);
  align-items: center;
}

.table-row:hover {
  background: var(--bg-tertiary);
}

.table-row:last-child {
  border-bottom: none;
}

.action-badge {
  display: inline-flex;
  align-items: center;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.action-badge.buy {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.action-badge.sell {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.col-pnl.positive {
  color: var(--color-success);
  font-weight: var(--font-weight-semibold);
}

.col-pnl.negative {
  color: var(--color-danger);
  font-weight: var(--font-weight-semibold);
}

/* Error State */
.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-2xl);
  text-align: center;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
}

.error-state i {
  font-size: var(--font-size-2xl);
  color: var(--color-danger);
  margin-bottom: var(--spacing-md);
}

.error-state h3 {
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
}

.error-state p {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-lg) 0;
}

/* Responsive Design */
@media (max-width: 1024px) {
  .form-row {
    grid-template-columns: 1fr;
  }

  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .table-header,
  .table-row {
    grid-template-columns: 1fr 1fr 1fr 1fr 1fr 1fr;
    font-size: var(--font-size-xs);
  }
}

@media (max-width: 768px) {
  .strategy-backtest {
    padding: var(--spacing-lg);
  }

  .backtest-header {
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .header-actions {
    width: 100%;
    justify-content: stretch;
  }

  .header-actions .p-button {
    flex: 1;
  }

  .form-actions {
    flex-direction: column;
  }

  .form-actions .p-button {
    width: 100%;
  }

  .metrics-grid {
    grid-template-columns: 1fr;
  }

  .results-header {
    flex-direction: column;
    gap: var(--spacing-md);
    align-items: flex-start;
  }

  .history-table {
    overflow-x: auto;
  }

  .table-header,
  .table-row {
    min-width: 600px;
  }

  .back-actions {
    flex-direction: column;
  }

  .back-actions .p-button {
    width: 100%;
  }
}
</style>
