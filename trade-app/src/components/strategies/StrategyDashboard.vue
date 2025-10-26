<template>
  <div class="strategy-dashboard">
    <!-- Header Section -->
    <div class="dashboard-header">
      <div class="header-content">
        <h1 class="dashboard-title">
          <i class="pi pi-chart-line"></i>
          Strategy Dashboard
        </h1>
        <p class="dashboard-subtitle">
          Monitor and manage your automated trading strategies
        </p>
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-upload"
          label="Upload Strategy"
          class="p-button-primary"
          @click="showUploadDialog = true"
        />
        <Button
          icon="pi pi-book"
          label="Strategy Library"
          class="p-button-outlined"
          @click="$router.push('/strategies/library')"
        />
      </div>
    </div>

    <!-- Quick Stats Section -->
    <div class="stats-section">
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon success">
            <i class="pi pi-play"></i>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.running_strategies || 0 }}</div>
            <div class="stat-label">Active Strategies</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-icon warning">
            <i class="pi pi-pause"></i>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.paused_strategies || 0 }}</div>
            <div class="stat-label">Paused</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-icon" :class="pnlClass">
            <i class="pi pi-dollar"></i>
          </div>
          <div class="stat-content">
            <div class="stat-value" :class="pnlClass">
              {{ formatCurrency(stats.total_pnl || 0) }}
            </div>
            <div class="stat-label">Total P&L</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-icon info">
            <i class="pi pi-chart-bar"></i>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.total_trades || 0 }}</div>
            <div class="stat-label">Total Trades</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Running Strategies Section -->
    <div class="strategies-section">
      <div class="section-header">
        <h2 class="section-title">
          <i class="pi pi-cog"></i>
          Running Strategies
        </h2>
        <div class="section-actions">
          <Button
            icon="pi pi-refresh"
            class="p-button-text p-button-sm"
            @click="refreshStrategies"
            :loading="isRefreshing"
          />
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="strategiesLoading" class="loading-state">
        <div class="loading-spinner"></div>
        <p>Loading strategies...</p>
      </div>

      <!-- Error State -->
      <div v-else-if="strategiesError" class="error-state">
        <i class="pi pi-exclamation-triangle"></i>
        <p>Failed to load strategies</p>
        <Button
          label="Retry"
          class="p-button-sm"
          @click="refreshStrategies"
        />
      </div>

      <!-- Empty State -->
      <div v-else-if="!runningStrategies.length" class="empty-state">
        <div class="empty-icon">
          <i class="pi pi-inbox"></i>
        </div>
        <h3>No Running Strategies</h3>
        <p>Upload your first strategy to get started with automated trading</p>
        <Button
          icon="pi pi-upload"
          label="Upload Strategy"
          class="p-button-primary"
          @click="showUploadDialog = true"
        />
      </div>

      <!-- Strategy Cards -->
      <div v-else class="strategy-grid">
        <div
          v-for="strategy in runningStrategies"
          :key="strategy.strategy_id"
          class="strategy-card"
          :class="getStrategyStatusClass(strategy)"
        >
          <div class="strategy-header">
            <div class="strategy-info">
              <h3 class="strategy-name">{{ strategy.name }}</h3>
              <p class="strategy-description">{{ strategy.description }}</p>
            </div>
            <div class="strategy-status">
              <span class="status-indicator" :class="getStatusClass(strategy)">
                <i :class="getStatusIcon(strategy)"></i>
                {{ getStatusText(strategy) }}
              </span>
            </div>
          </div>

          <div class="strategy-metrics">
            <div class="metric">
              <span class="metric-label">P&L</span>
              <span class="metric-value" :class="getPnLClass(strategy.pnl)">
                {{ formatCurrency(strategy.pnl || 0) }}
              </span>
            </div>
            <div class="metric">
              <span class="metric-label">Trades</span>
              <span class="metric-value">{{ strategy.trades_count || 0 }}</span>
            </div>
            <div class="metric">
              <span class="metric-label">Uptime</span>
              <span class="metric-value">{{ formatUptime(strategy.created_at) }}</span>
            </div>
          </div>

          <div class="strategy-actions">
            <Button
              icon="pi pi-eye"
              class="p-button-text p-button-sm"
              title="Monitor Strategy"
              @click="monitorStrategy(strategy.strategy_id)"
            />
            <Button
              v-if="strategy.is_running && !strategy.is_paused"
              icon="pi pi-pause"
              class="p-button-text p-button-sm p-button-warning"
              title="Pause Strategy"
              @click="pauseStrategy(strategy.strategy_id)"
              :loading="isLoading(`pause_strategy_${strategy.strategy_id}`)"
            />
            <Button
              v-else-if="strategy.is_paused"
              icon="pi pi-play"
              class="p-button-text p-button-sm p-button-success"
              title="Resume Strategy"
              @click="resumeStrategy(strategy.strategy_id)"
              :loading="isLoading(`resume_strategy_${strategy.strategy_id}`)"
            />
            <Button
              icon="pi pi-stop"
              class="p-button-text p-button-sm p-button-danger"
              title="Stop Strategy"
              @click="stopStrategy(strategy.strategy_id)"
              :loading="isLoading(`stop_strategy_${strategy.strategy_id}`)"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Strategy Upload Dialog -->
    <StrategyUploadDialog
      v-model:visible="showUploadDialog"
      @uploaded="handleStrategyUploaded"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useStrategyData } from '../../composables/useStrategyData.js'
import { useMarketData } from '../../composables/useMarketData.js'
import { useNotifications } from '../../composables/useNotifications.js'
import StrategyUploadDialog from './StrategyUploadDialog.vue'

export default {
  name: 'StrategyDashboard',
  components: {
    StrategyUploadDialog
  },
  setup() {
    const router = useRouter()
    const { showSuccess, showError } = useNotifications()
    
    // Smart Data System Integration - "Dumb Component" Pattern
    const {
      getMyStrategies,
      getExecutionStats,
      pauseStrategy: pauseStrategyAction,
      resumeStrategy: resumeStrategyAction,
      stopStrategy: stopStrategyAction,
      isLoading,
      getError
    } = useStrategyData()

    // Cross-system integration - using existing market data
    const { getBalance } = useMarketData()

    // All reactive - no manual API calls
    const strategies = getMyStrategies()     // Auto-updates every 30s
    const rawStats = getExecutionStats()    // Real-time via WebSocket
    const balance = getBalance()            // Existing auto-refresh

    // Safe stats access with fallback
    const stats = computed(() => rawStats.value || {
      total_strategies: 0,
      running_strategies: 0,
      paused_strategies: 0,
      total_pnl: 0,
      total_trades: 0,
      uptime_seconds: 0
    })

    // Local state
    const showUploadDialog = ref(false)
    const isRefreshing = ref(false)

    // Loading and error states from centralized system
    const strategiesLoading = computed(() => isLoading('strategies').value)
    const strategiesError = computed(() => getError('strategies').value)

    // Computed properties
    const runningStrategies = computed(() => {
      return strategies.value.filter(strategy => 
        strategy.is_running || strategy.is_paused
      )
    })

    const pnlClass = computed(() => {
      const pnl = stats.value.total_pnl || 0
      return pnl >= 0 ? 'success' : 'danger'
    })

    // Methods
    const refreshStrategies = async () => {
      isRefreshing.value = true
      try {
        // The smart data system will handle the refresh
        // We just need to trigger a manual refresh
        await new Promise(resolve => setTimeout(resolve, 1000))
      } finally {
        isRefreshing.value = false
      }
    }

    const monitorStrategy = (strategyId) => {
      router.push(`/strategies/monitor/${strategyId}`)
    }

    const pauseStrategy = async (strategyId) => {
      try {
        await pauseStrategyAction(strategyId)
        showSuccess(
          'Strategy paused successfully',
          'Strategy Paused'
        )
      } catch (error) {
        showError(
          `Failed to pause strategy: ${error.message}`,
          'Pause Error'
        )
      }
    }

    const resumeStrategy = async (strategyId) => {
      try {
        await resumeStrategyAction(strategyId)
        showSuccess(
          'Strategy resumed successfully',
          'Strategy Resumed'
        )
      } catch (error) {
        showError(
          `Failed to resume strategy: ${error.message}`,
          'Resume Error'
        )
      }
    }

    const stopStrategy = async (strategyId) => {
      try {
        await stopStrategyAction(strategyId)
        showSuccess(
          'Strategy stopped successfully',
          'Strategy Stopped'
        )
      } catch (error) {
        showError(
          `Failed to stop strategy: ${error.message}`,
          'Stop Error'
        )
      }
    }

    const handleStrategyUploaded = () => {
      showUploadDialog.value = false
      showSuccess(
        'Strategy uploaded successfully',
        'Upload Success'
      )
    }

    // Utility methods
    const getStrategyStatusClass = (strategy) => {
      if (strategy.is_running && !strategy.is_paused) return 'status-running'
      if (strategy.is_paused) return 'status-paused'
      return 'status-stopped'
    }

    const getStatusClass = (strategy) => {
      if (strategy.is_running && !strategy.is_paused) return 'status-success'
      if (strategy.is_paused) return 'status-warning'
      return 'status-danger'
    }

    const getStatusIcon = (strategy) => {
      if (strategy.is_running && !strategy.is_paused) return 'pi pi-play'
      if (strategy.is_paused) return 'pi pi-pause'
      return 'pi pi-stop'
    }

    const getStatusText = (strategy) => {
      if (strategy.is_running && !strategy.is_paused) return 'Running'
      if (strategy.is_paused) return 'Paused'
      return 'Stopped'
    }

    const getPnLClass = (pnl) => {
      if (!pnl) return ''
      return pnl >= 0 ? 'positive' : 'negative'
    }

    const formatCurrency = (amount) => {
      const value = Math.abs(amount || 0)
      const sign = amount >= 0 ? '+' : '-'
      return `${sign}$${value.toLocaleString('en-US', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
      })}`
    }

    const formatUptime = (createdAt) => {
      if (!createdAt) return '--'
      
      const now = new Date()
      const created = new Date(createdAt)
      const diffMs = now - created
      const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
      const diffMinutes = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60))
      
      if (diffHours > 0) {
        return `${diffHours}h ${diffMinutes}m`
      } else {
        return `${diffMinutes}m`
      }
    }

    return {
      // Reactive data
      strategies,
      stats,
      balance,
      showUploadDialog,
      isRefreshing,

      // Computed
      runningStrategies,
      pnlClass,
      strategiesLoading,
      strategiesError,

      // Methods
      refreshStrategies,
      monitorStrategy,
      pauseStrategy,
      resumeStrategy,
      stopStrategy,
      handleStrategyUploaded,
      isLoading,

      // Utility methods
      getStrategyStatusClass,
      getStatusClass,
      getStatusIcon,
      getStatusText,
      getPnLClass,
      formatCurrency,
      formatUptime
    }
  }
}
</script>

<style scoped>
.strategy-dashboard {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header Section */
.dashboard-header {
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

.dashboard-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.dashboard-title i {
  color: var(--color-brand);
}

.dashboard-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Stats Section */
.stats-section {
  margin-bottom: var(--spacing-xl);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-lg);
}

.stat-card {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  transition: var(--transition-normal);
}

.stat-card:hover {
  border-color: var(--border-secondary);
  box-shadow: var(--shadow-sm);
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  font-size: var(--font-size-xl);
}

.stat-icon.success {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.stat-icon.warning {
  background: rgba(245, 158, 11, 0.1);
  color: var(--color-warning);
}

.stat-icon.info {
  background: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
}

.stat-icon.danger {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.stat-content {
  flex: 1;
}

.stat-value {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin-bottom: 2px;
}

.stat-value.success {
  color: var(--color-success);
}

.stat-value.danger {
  color: var(--color-danger);
}

.stat-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

/* Strategies Section */
.strategies-section {
  margin-bottom: var(--spacing-xl);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
}

.section-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0;
}

.section-title i {
  color: var(--color-brand);
}

.section-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Loading, Error, Empty States */
.loading-state,
.error-state,
.empty-state {
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

.loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-secondary);
  border-top: 3px solid var(--color-brand);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: var(--spacing-md);
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.error-state i {
  font-size: var(--font-size-2xl);
  color: var(--color-danger);
  margin-bottom: var(--spacing-md);
}

.empty-icon {
  font-size: var(--font-size-3xl);
  color: var(--text-tertiary);
  margin-bottom: var(--spacing-md);
}

.empty-state h3 {
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
}

.empty-state p {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-lg) 0;
}

/* Strategy Grid */
.strategy-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: var(--spacing-lg);
}

.strategy-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  transition: var(--transition-normal);
}

.strategy-card:hover {
  border-color: var(--border-secondary);
  box-shadow: var(--shadow-md);
}

.strategy-card.status-running {
  border-left: 4px solid var(--color-success);
}

.strategy-card.status-paused {
  border-left: 4px solid var(--color-warning);
}

.strategy-card.status-stopped {
  border-left: 4px solid var(--color-danger);
}

.strategy-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-md);
}

.strategy-info {
  flex: 1;
  min-width: 0;
}

.strategy-name {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.strategy-description {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.4;
}

.strategy-status {
  flex-shrink: 0;
  margin-left: var(--spacing-sm);
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.status-indicator.status-success {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.status-indicator.status-warning {
  background: rgba(245, 158, 11, 0.1);
  color: var(--color-warning);
}

.status-indicator.status-danger {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.strategy-metrics {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.metric {
  text-align: center;
}

.metric-label {
  display: block;
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 2px;
}

.metric-value {
  display: block;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.metric-value.positive {
  color: var(--color-success);
}

.metric-value.negative {
  color: var(--color-danger);
}

.strategy-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-xs);
}

/* Responsive Design */
@media (max-width: 768px) {
  .strategy-dashboard {
    padding: var(--spacing-lg);
  }

  .dashboard-header {
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

  .stats-grid {
    grid-template-columns: 1fr;
  }

  .strategy-grid {
    grid-template-columns: 1fr;
  }

  .strategy-metrics {
    grid-template-columns: 1fr;
    gap: var(--spacing-sm);
  }
}
</style>
