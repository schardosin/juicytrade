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
    <div v-if="!backtestResults && !route.query.run_id" class="backtest-config">
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

    <!-- Backtest Results with Tabbed Interface -->
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

      <!-- Custom Tabbed Interface -->
      <div class="custom-tabs">
        <!-- Tab Headers -->
        <div class="tab-headers">
          <button 
            class="tab-header"
            :class="{ active: activeTabIndex === 0 }"
            @click="activeTabIndex = 0"
          >
            <i class="pi pi-chart-bar"></i>
            <span>Trade Results</span>
          </button>
          <button 
            v-if="backtestResults.decision_timeline && backtestResults.decision_timeline.length"
            class="tab-header"
            :class="{ active: activeTabIndex === 1 }"
            @click="activeTabIndex = 1"
          >
            <i class="pi pi-search"></i>
            <span>Decision Analysis</span>
            <Badge 
              :value="backtestResults.decision_timeline.length" 
              class="ml-2"
              severity="info"
            />
          </button>
        </div>

        <!-- Tab Content -->
        <div class="tab-content">
          <!-- Tab 1: Trade Results (Traditional View) -->
          <div v-show="activeTabIndex === 0" class="tab-panel">
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

          <!-- Additional Performance Metrics -->
          <div class="additional-metrics">
            <div class="metrics-row">
              <div class="metric-item">
                <span class="metric-label">Sharpe Ratio</span>
                <span class="metric-value">{{ formatNumber(backtestResults.sharpe_ratio || 0) }}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">Max Drawdown</span>
                <span class="metric-value negative">{{ formatPercentage(backtestResults.max_drawdown || 0) }}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">Max Profit</span>
                <span class="metric-value positive">{{ formatCurrency(backtestResults.max_profit || 0) }}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">Max Loss</span>
                <span class="metric-value negative">{{ formatCurrency(backtestResults.max_loss || 0) }}</span>
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

          <!-- Tab 2: Decision Analysis (NEW) -->
          <div v-show="activeTabIndex === 1" class="tab-panel">
            <!-- Decision Analysis Overview -->
          <div class="decision-overview">
            <div class="overview-card">
              <div class="overview-content">
                <div class="overview-icon">
                  <i class="pi pi-search"></i>
                </div>
                <div class="overview-text">
                  <h4>Strategy Decision Analysis</h4>
                  <p>Analyze why your strategy made or didn't make trades at each data point. This detailed view shows the decision-making process behind every action.</p>
                </div>
              </div>
              
              <!-- Decision Statistics -->
              <div class="decision-stats">
                <div class="stat-item success">
                  <div class="stat-value">{{ backtestResults.decision_timeline.filter(d => d.result).length }}</div>
                  <div class="stat-label">Successful Decisions</div>
                </div>
                <div class="stat-item failed">
                  <div class="stat-value">{{ backtestResults.decision_timeline.filter(d => !d.result && d.error).length }}</div>
                  <div class="stat-label">Failed Decisions</div>
                </div>
                <div class="stat-item total">
                  <div class="stat-value">{{ backtestResults.decision_timeline.length }}</div>
                  <div class="stat-label">Total Decisions</div>
                </div>
                <div class="stat-item rate">
                  <div class="stat-value">{{ formatPercentage(getDecisionSuccessRate()) }}</div>
                  <div class="stat-label">Success Rate</div>
                </div>
              </div>
            </div>
          </div>

          <!-- Action Execution Metrics -->
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

          <!-- Enhanced Granular Decision Analysis -->
          <div class="enhanced-decision-analysis">
            <div class="analysis-header">
              <h3 class="analysis-title">
                <i class="pi pi-microscope"></i>
                Granular Decision Analysis
              </h3>
              <p class="analysis-description">
                Navigate through every decision point to understand exactly why your strategy made or didn't make trades.
              </p>
            </div>

            <!-- Decision Point Navigator -->
            <DataPointTimelineNavigator 
              :decision-timeline="backtestResults.decision_timeline"
              @datapoint-selected="handleDatapointSelected"
            />

            <!-- Decision Breakdown Panel -->
            <GranularDecisionBreakdownPanel 
              :selected-data-point="selectedDecisionPoint"
            />
          </div>

          <!-- Strategy Checkpoints -->
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
          </div>
        </div>
      </div>
    </div>

    <!-- Error State for API failures when trying to load specific backtest -->
    <div v-if="route.query.run_id && !backtestResults && !backtestError" class="error-state">
      <i class="pi pi-exclamation-triangle"></i>
      <h3>Backtest Result Not Found</h3>
      <p>The requested backtest result could not be loaded. It may have been deleted or the ID is invalid.</p>
      <div class="error-actions">
        <Button
          icon="pi pi-arrow-left"
          label="Back to Backtesting"
          class="p-button-primary"
          @click="$router.push('/strategies/backtesting')"
        />
        <Button
          icon="pi pi-refresh"
          label="Run New Backtest"
          class="p-button-outlined"
          @click="$router.push(`/strategies/backtest/${strategyId}`)"
        />
      </div>
    </div>

    <!-- Error State for backtest execution failures -->
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
import DataPointTimelineNavigator from './DataPointTimelineNavigator.vue'
import GranularDecisionBreakdownPanel from './GranularDecisionBreakdownPanel.vue'

export default {
  name: 'StrategyBacktest',
  components: {
    DataPointTimelineNavigator,
    GranularDecisionBreakdownPanel
  },
  setup() {
    const route = useRoute()
    const router = useRouter()
    const { showSuccess, showError } = useNotifications()
    const { getMyStrategies, getStrategyBacktest, getBacktestRun, isLoading } = useStrategyData()

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
    const showDecisionInspector = ref(false)
    const activeTabIndex = ref(0) // NEW: Tab control
      const selectedDecisionPoint = ref(null) // NEW: Selected decision point for granular analysis
      
      // NEW: Method to handle datapoint selection
      const handleDatapointSelected = (dataPoint) => {
        selectedDecisionPoint.value = dataPoint
      }
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
        // For now, we'll use a direct import since this functionality 
        // isn't yet implemented in the smart data system
        const api = await import('../../services/api.js')
        const response = await api.default.get(`/api/strategies/${strategyId.value}/parameters`)
        if (response.data.success) {
          strategyParameters.value = response.data
          
          // Initialize strategy config with default values
          const config = {}
          Object.entries(response.data.parameters).forEach(([paramName, param]) => {
            config[paramName] = param.default
          })
          strategyConfig.value = config          
        }
      } catch (error) {
        console.error('Failed to load strategy parameters:', error)
        showError(
          'Failed to load strategy configuration',
          'Configuration Error'
        )
      }
    }

    // Load specific backtest result
    const loadBacktestResult = async (runId) => {
      try {
        // Use the proper API method from useStrategyData composable
        const response = await getBacktestRun(runId)
        
        if (response.success && response.data) {
          const result = response.data
          
          // Extract metrics from the nested structure
          const pnlMetrics = result.results?.metrics?.pnl
          const tradingMetrics = result.results?.metrics?.trading
          const riskMetrics = result.results?.metrics?.risk
          
          // Transform the result to match our UI expectations
          // Data comes from nested metrics structure: result.results.metrics.{pnl|trading|risk|actions}
          const transformedResults = {
            // Extract P&L metrics from result.results.metrics.pnl
            total_pnl: pnlMetrics?.total_pnl,
            total_return: pnlMetrics?.total_return,
            max_profit: pnlMetrics?.max_profit,
            max_loss: pnlMetrics?.max_loss,
            
            // Extract trading metrics from result.results.metrics.trading
            total_trades: tradingMetrics?.total_trades,
            win_rate: tradingMetrics?.win_rate,
            
            // Extract risk metrics from result.results.metrics.risk
            sharpe_ratio: riskMetrics?.sharpe_ratio,
            max_drawdown: riskMetrics?.max_drawdown,
            
            // Use the trades array directly
            trades: result.results?.trades,
            
            // Add additional data from new system
            action_metrics: result.results?.metrics?.actions,
            equity_curve: result.results?.equity_curve,
            checkpoints: result.results?.checkpoints,
            action_log: result.results?.action_log,
            decision_timeline: result.results?.decision_timeline,
            
            // Keep essential original data without overwriting our extracted metrics
            run_id: result.run_id,
            strategy_id: result.strategy_id,
            status: result.status,
            created_at: result.created_at,
            results: result.results
          }
          
          backtestResults.value = transformedResults
          
          // Auto-show the decision inspector if we have decision data
          if (transformedResults.decision_timeline && transformedResults.decision_timeline.length > 0) {
            showDecisionInspector.value = true
          }
          
          showSuccess(
            'Backtest results loaded successfully',
            'Results Loaded'
          )
        } else {
          throw new Error('Backtest result not found')
        }
      } catch (error) {
        console.error('Failed to load backtest result:', error)
        showError(
          `Failed to load backtest result: ${error.message}`,
          'Load Error'
        )
      }
    }

    // Initialize on mount
    onMounted(async () => {
      if (strategyId.value) {
        await loadStrategyParameters()
        
        // Check if we have a run_id query parameter (coming from backtest results)
        const runId = route.query.run_id
        if (runId) {
          await loadBacktestResult(runId)
        }
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
          // Transform the comprehensive results to match UI expectations
          // Data comes from nested metrics structure: result.data.metrics.{pnl|trading|risk|actions}
          const transformedResults = {
            // Extract P&L metrics from result.data.metrics.pnl
            total_pnl: result.data.metrics.pnl.total_pnl,
            total_return: result.data.metrics.pnl.total_return,
            max_profit: result.data.metrics.pnl.max_profit,
            max_loss: result.data.metrics.pnl.max_loss,
            
            // Extract trading metrics from result.data.metrics.trading
            total_trades: result.data.metrics.trading.total_trades,
            win_rate: result.data.metrics.trading.win_rate,
            
            // Extract risk metrics from result.data.metrics.risk
            sharpe_ratio: result.data.metrics.risk.sharpe_ratio,
            max_drawdown: result.data.metrics.risk.max_drawdown,
            
            // Use the trades array directly
            trades: result.data.trades,
            
            // Add additional data from new system
            action_metrics: result.data.metrics.actions,
            equity_curve: result.data.equity_curve,
            checkpoints: result.data.checkpoints,
            action_log: result.data.action_log,
            decision_timeline: result.data.decision_timeline,
            
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

    // NEW: Additional utility methods for Decision Analysis tab
    const formatNumber = (value) => {
      if (value === null || value === undefined) return '0.00'
      return Number(value).toFixed(2)
    }

    const getDecisionSuccessRate = () => {
      if (!backtestResults.value?.decision_timeline?.length) return 0
      const successful = backtestResults.value.decision_timeline.filter(d => d.result).length
      return successful / backtestResults.value.decision_timeline.length
    }

    const refreshDecisionData = () => {
      // Placeholder for refresh functionality
      console.log('Refreshing decision data...')
    }

    // Auto-switch to Decision Analysis tab if decision data exists
    const switchToDecisionTab = () => {
      if (backtestResults.value?.decision_timeline?.length > 0) {
        activeTabIndex.value = 1 // Switch to Decision Analysis tab
      }
    }

    return {
      // Router
      route,
      
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
      showDecisionInspector,
      activeTabIndex, // NEW: Tab control

      // Computed
      maxStartDate,
      isConfigValid,
      isRunningBacktest,

      // Methods
      runBacktest,
      resetConfig,
      exportResults,
      loadStrategyParameters,
      refreshDecisionData, // NEW: Decision data refresh
      switchToDecisionTab, // NEW: Tab switching

      // Utility methods
      getPnLClass,
      formatCurrency,
      formatPercentage,
      formatDate,
      formatParameterName,
      formatNumber, // NEW: Number formatting
      getDecisionSuccessRate, // NEW: Decision success rate calculation
      handleDatapointSelected, // NEW: Event handler for datapoint selection
      selectedDecisionPoint // NEW: Selected decision point reactive ref
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

/* Custom Tabbed Interface Styles */
.custom-tabs {
  margin-top: var(--spacing-lg);
}

.tab-headers {
  display: flex;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  overflow: hidden;
}

.tab-header {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
  padding: var(--spacing-md) var(--spacing-lg);
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  cursor: pointer;
  transition: var(--transition-normal);
  font-size: var(--font-size-md);
  border-bottom: 3px solid transparent;
  position: relative;
}

.tab-header:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.tab-header.active {
  background: var(--color-brand);
  color: white;
  font-weight: var(--font-weight-semibold);
  border-bottom-color: var(--color-brand);
}

.tab-header i {
  font-size: var(--font-size-sm);
}

.tab-content {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-top: none;
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
  padding: var(--spacing-lg);
  min-height: 400px;
}

.tab-panel {
  width: 100%;
}

/* Decision Analysis Tab Styles */
.decision-analysis-tab {
  min-height: 600px;
}

.decision-overview {
  margin-bottom: var(--spacing-xl);
}

.overview-card {
  background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-tertiary) 100%);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  box-shadow: var(--shadow-sm);
}

.overview-content {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
}

.overview-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: var(--color-brand);
  border-radius: var(--radius-lg);
  font-size: var(--font-size-xl);
  color: white;
  flex-shrink: 0;
}

.overview-text {
  flex: 1;
}

.overview-text h4 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
}

.overview-text p {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0;
}

.decision-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: var(--spacing-md);
}

.stat-item {
  text-align: center;
  padding: var(--spacing-md);
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.stat-item:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.stat-item.success {
  border-color: var(--color-success);
  background: rgba(34, 197, 94, 0.05);
}

.stat-item.failed {
  border-color: var(--color-danger);
  background: rgba(239, 68, 68, 0.05);
}

.stat-item.total {
  border-color: var(--color-info);
  background: rgba(59, 130, 246, 0.05);
}

.stat-item.rate {
  border-color: var(--color-warning);
  background: rgba(245, 158, 11, 0.05);
}

.stat-value {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin-bottom: var(--spacing-xs);
}

.stat-item.success .stat-value {
  color: var(--color-success);
}

.stat-item.failed .stat-value {
  color: var(--color-danger);
}

.stat-item.total .stat-value {
  color: var(--color-info);
}

.stat-item.rate .stat-value {
  color: var(--color-warning);
}

.stat-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

/* Additional Metrics Styles */
.additional-metrics {
  margin-bottom: var(--spacing-xl);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
}

.metrics-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-lg);
}

.metric-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md);
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
}

.metric-item .metric-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.metric-item .metric-value {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.metric-item .metric-value.positive {
  color: var(--color-success);
}

.metric-item .metric-value.negative {
  color: var(--color-danger);
}

/* Inspector Controls */
.inspector-controls {
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

/* Decision Inspector Section */
.decision-inspector-section {
  margin-bottom: var(--spacing-xl);
}

.inspector-section-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-lg) 0;
}

.inspector-section-title i {
  color: var(--color-brand);
}

.inspector-container {
  height: 600px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  overflow: hidden;
  background: var(--bg-secondary);
}

.inspector-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
}

/* Decision Navigation Section */
.decision-navigation {
  margin-bottom: var(--spacing-xl);
}

.navigation-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-xl);
  background: linear-gradient(135deg, var(--color-brand) 0%, rgba(var(--color-brand-rgb), 0.8) 100%);
  border-radius: var(--radius-lg);
  color: white;
  box-shadow: var(--shadow-md);
  transition: var(--transition-normal);
}

.navigation-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
}

.navigation-content {
  display: flex;
  align-items: center;
  gap: var(--spacing-lg);
  flex: 1;
}

.navigation-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: var(--radius-lg);
  font-size: var(--font-size-xl);
  color: white;
  backdrop-filter: blur(10px);
}

.navigation-text h4 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  color: white;
  margin: 0 0 var(--spacing-xs) 0;
}

.navigation-text p {
  font-size: var(--font-size-md);
  color: rgba(255, 255, 255, 0.9);
  margin: 0 0 var(--spacing-md) 0;
  line-height: 1.4;
}

.navigation-stats {
  display: flex;
  gap: var(--spacing-md);
  flex-wrap: wrap;
}

.stat-badge {
  display: inline-flex;
  align-items: center;
  padding: var(--spacing-xs) var(--spacing-sm);
  background: rgba(255, 255, 255, 0.2);
  border-radius: var(--radius-md);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: white;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.3);
}

.stat-badge.success {
  background: rgba(34, 197, 94, 0.3);
  border-color: rgba(34, 197, 94, 0.5);
}

.stat-badge.failed {
  background: rgba(239, 68, 68, 0.3);
  border-color: rgba(239, 68, 68, 0.5);
}

.stat-badge.total {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
}

.navigation-actions {
  display: flex;
  gap: var(--spacing-sm);
}

.navigation-actions .p-button {
  background: white;
  color: var(--color-brand);
  border: none;
  font-weight: var(--font-weight-semibold);
  padding: var(--spacing-md) var(--spacing-lg);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.navigation-actions .p-button:hover {
  background: rgba(255, 255, 255, 0.9);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* Enhanced Decision Analysis Section */
.enhanced-decision-analysis {
  margin-bottom: var(--spacing-xl);
}

.analysis-header {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-tertiary) 100%);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
}

.analysis-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
}

.analysis-title i {
  color: var(--color-brand);
}

.analysis-description {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0;
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
