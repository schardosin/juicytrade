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

    // Backtest configuration
    const backtestConfig = ref({
      start_date: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000), // 30 days ago
      end_date: new Date(),
      initial_capital: 10000,
      speed_multiplier: 1
    })

    // Speed options
    const speedOptions = [
      { label: '1x (Real-time)', value: 1 },
      { label: '10x (Fast)', value: 10 },
      { label: '100x (Very Fast)', value: 100 },
      { label: '1000x (Maximum)', value: 1000 }
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

        const config = {
          start_date: backtestConfig.value.start_date.toISOString().split('T')[0],
          end_date: backtestConfig.value.end_date.toISOString().split('T')[0],
          initial_capital: backtestConfig.value.initial_capital,
          speed_multiplier: backtestConfig.value.speed_multiplier
        }

        const result = await getStrategyBacktest(strategyId.value, config)
        
        if (result.success) {
          backtestResults.value = result.data
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
      const value = Math.abs(amount || 0)
      const sign = amount >= 0 ? '+' : '-'
      return `${sign}$${value.toLocaleString('en-US', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
      })}`
    }

    const formatPercentage = (value) => {
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

    return {
      // Data
      strategyId,
      strategyName,
      backtestConfig,
      speedOptions,
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

      // Utility methods
      getPnLClass,
      formatCurrency,
      formatPercentage,
      formatDate
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
