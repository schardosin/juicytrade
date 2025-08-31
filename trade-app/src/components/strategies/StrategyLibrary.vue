<template>
  <div class="strategy-library">
    <!-- Header Section -->
    <div class="library-header">
      <div class="header-content">
        <h1 class="library-title">
          <i class="pi pi-book"></i>
          Strategy Library
        </h1>
        <p class="library-subtitle">
          Browse and manage your trading strategy collection
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
          icon="pi pi-chart-line"
          label="Dashboard"
          class="p-button-outlined"
          @click="$router.push('/strategies')"
        />
      </div>
    </div>

    <!-- Filter and Search Section -->
    <div class="filter-section">
      <div class="search-controls">
        <div class="search-input-container">
          <i class="pi pi-search search-icon"></i>
          <InputText
            v-model="searchQuery"
            placeholder="Search strategies..."
            class="search-input"
          />
        </div>
        <Dropdown
          v-model="selectedStatus"
          :options="statusOptions"
          option-label="label"
          option-value="value"
          placeholder="All Statuses"
          class="status-filter"
        />
        <Dropdown
          v-model="selectedSort"
          :options="sortOptions"
          option-label="label"
          option-value="value"
          placeholder="Sort by"
          class="sort-filter"
        />
      </div>
      <div class="view-controls">
        <Button
          icon="pi pi-th-large"
          class="p-button-text"
          :class="{ 'p-button-secondary': viewMode === 'grid' }"
          @click="viewMode = 'grid'"
        />
        <Button
          icon="pi pi-list"
          class="p-button-text"
          :class="{ 'p-button-secondary': viewMode === 'list' }"
          @click="viewMode = 'list'"
        />
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="strategiesLoading" class="loading-state">
      <div class="loading-spinner"></div>
      <p>Loading strategy library...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="strategiesError" class="error-state">
      <i class="pi pi-exclamation-triangle"></i>
      <p>Failed to load strategy library</p>
      <Button
        label="Retry"
        class="p-button-sm"
        @click="refreshStrategies"
      />
    </div>

    <!-- Empty State -->
    <div v-else-if="!filteredStrategies.length" class="empty-state">
      <div class="empty-icon">
        <i class="pi pi-inbox"></i>
      </div>
      <h3>No Strategies Found</h3>
      <p v-if="searchQuery || selectedStatus !== 'all'">
        Try adjusting your search or filter criteria
      </p>
      <p v-else>
        Upload your first strategy to get started with automated trading
      </p>
      <Button
        icon="pi pi-upload"
        label="Upload Strategy"
        class="p-button-primary"
        @click="showUploadDialog = true"
      />
    </div>

    <!-- Strategy Grid/List -->
    <div v-else class="strategies-container">
      <!-- Grid View -->
      <div v-if="viewMode === 'grid'" class="strategy-grid">
        <div
          v-for="strategy in filteredStrategies"
          :key="strategy.strategy_id"
          class="strategy-card"
          :class="getStrategyStatusClass(strategy)"
        >
          <div class="strategy-header">
            <div class="strategy-info">
              <h3 class="strategy-name">{{ strategy.name }}</h3>
              <p class="strategy-description">{{ strategy.description || 'No description provided' }}</p>
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
              <span class="metric-label">Created</span>
              <span class="metric-value">{{ formatDate(strategy.created_at) }}</span>
            </div>
          </div>

          <div class="strategy-actions">
            <Button
              v-if="!strategy.is_running"
              icon="pi pi-play"
              class="p-button-text p-button-sm p-button-success"
              title="Start Strategy"
              @click="startStrategy(strategy.strategy_id)"
              :loading="isLoading(`start_strategy_${strategy.strategy_id}`)"
            />
            <Button
              v-else-if="strategy.is_running && !strategy.is_paused"
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
              icon="pi pi-eye"
              class="p-button-text p-button-sm"
              title="Monitor Strategy"
              @click="monitorStrategy(strategy.strategy_id)"
            />
            <Button
              icon="pi pi-chart-bar"
              class="p-button-text p-button-sm"
              title="Backtest Strategy"
              @click="backtestStrategy(strategy.strategy_id)"
            />
            <Button
              icon="pi pi-trash"
              class="p-button-text p-button-sm p-button-danger"
              title="Delete Strategy"
              @click="confirmDeleteStrategy(strategy)"
              :loading="isLoading(`delete_strategy_${strategy.strategy_id}`)"
            />
          </div>
        </div>
      </div>

      <!-- List View -->
      <div v-else class="strategy-list">
        <div class="list-header">
          <div class="col-name">Name</div>
          <div class="col-status">Status</div>
          <div class="col-pnl">P&L</div>
          <div class="col-trades">Trades</div>
          <div class="col-created">Created</div>
          <div class="col-actions">Actions</div>
        </div>
        <div
          v-for="strategy in filteredStrategies"
          :key="strategy.strategy_id"
          class="strategy-row"
          :class="getStrategyStatusClass(strategy)"
        >
          <div class="col-name">
            <div class="strategy-name-cell">
              <h4>{{ strategy.name }}</h4>
              <p>{{ strategy.description || 'No description' }}</p>
            </div>
          </div>
          <div class="col-status">
            <span class="status-indicator" :class="getStatusClass(strategy)">
              <i :class="getStatusIcon(strategy)"></i>
              {{ getStatusText(strategy) }}
            </span>
          </div>
          <div class="col-pnl">
            <span class="pnl-value" :class="getPnLClass(strategy.pnl)">
              {{ formatCurrency(strategy.pnl || 0) }}
            </span>
          </div>
          <div class="col-trades">
            <span class="trades-value">{{ strategy.trades_count || 0 }}</span>
          </div>
          <div class="col-created">
            <span class="created-value">{{ formatDate(strategy.created_at) }}</span>
          </div>
          <div class="col-actions">
            <div class="action-buttons">
              <Button
                v-if="!strategy.is_running"
                icon="pi pi-play"
                class="p-button-text p-button-sm p-button-success"
                title="Start Strategy"
                @click="startStrategy(strategy.strategy_id)"
                :loading="isLoading(`start_strategy_${strategy.strategy_id}`)"
              />
              <Button
                v-else-if="strategy.is_running && !strategy.is_paused"
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
                icon="pi pi-eye"
                class="p-button-text p-button-sm"
                title="Monitor Strategy"
                @click="monitorStrategy(strategy.strategy_id)"
              />
              <Button
                icon="pi pi-chart-bar"
                class="p-button-text p-button-sm"
                title="Backtest Strategy"
                @click="backtestStrategy(strategy.strategy_id)"
              />
              <Button
                icon="pi pi-trash"
                class="p-button-text p-button-sm p-button-danger"
                title="Delete Strategy"
                @click="confirmDeleteStrategy(strategy)"
                :loading="isLoading(`delete_strategy_${strategy.strategy_id}`)"
              />
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Strategy Upload Dialog -->
    <StrategyUploadDialog
      v-model:visible="showUploadDialog"
      @uploaded="handleStrategyUploaded"
    />

    <!-- Delete Confirmation Dialog -->
    <Dialog
      v-model:visible="showDeleteDialog"
      modal
      header="Delete Strategy"
      :style="{ width: '450px' }"
    >
      <div class="delete-confirmation">
        <i class="pi pi-exclamation-triangle delete-icon"></i>
        <div class="delete-message">
          <h3>Are you sure you want to delete this strategy?</h3>
          <p><strong>{{ strategyToDelete?.name }}</strong></p>
          <p class="warning-text">This action cannot be undone.</p>
        </div>
      </div>
      <template #footer>
        <Button
          label="Cancel"
          class="p-button-text"
          @click="showDeleteDialog = false"
        />
        <Button
          label="Delete"
          icon="pi pi-trash"
          class="p-button-danger"
          @click="deleteStrategy"
          :loading="isDeleting"
        />
      </template>
    </Dialog>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useStrategyData } from '../../composables/useStrategyData.js'
import { useNotifications } from '../../composables/useNotifications.js'
import StrategyUploadDialog from './StrategyUploadDialog.vue'

export default {
  name: 'StrategyLibrary',
  components: {
    StrategyUploadDialog
  },
  setup() {
    const router = useRouter()
    const { addNotification } = useNotifications()
    
    // Smart Data System Integration
    const {
      getMyStrategies,
      startStrategy: startStrategyAction,
      pauseStrategy: pauseStrategyAction,
      resumeStrategy: resumeStrategyAction,
      deleteStrategy: deleteStrategyAction,
      isLoading,
      getError
    } = useStrategyData()

    // Reactive data from smart system
    const strategies = getMyStrategies()

    // Local state
    const showUploadDialog = ref(false)
    const searchQuery = ref('')
    const selectedStatus = ref('all')
    const selectedSort = ref('created_desc')
    const viewMode = ref('grid')
    const showDeleteDialog = ref(false)
    const strategyToDelete = ref(null)
    const isDeleting = ref(false)

    // Loading and error states
    const strategiesLoading = computed(() => isLoading('strategies').value)
    const strategiesError = computed(() => getError('strategies').value)

    // Filter and sort options
    const statusOptions = [
      { label: 'All Statuses', value: 'all' },
      { label: 'Running', value: 'running' },
      { label: 'Paused', value: 'paused' },
      { label: 'Stopped', value: 'stopped' }
    ]

    const sortOptions = [
      { label: 'Newest First', value: 'created_desc' },
      { label: 'Oldest First', value: 'created_asc' },
      { label: 'Name A-Z', value: 'name_asc' },
      { label: 'Name Z-A', value: 'name_desc' },
      { label: 'Best P&L', value: 'pnl_desc' },
      { label: 'Worst P&L', value: 'pnl_asc' }
    ]

    // Computed filtered and sorted strategies
    const filteredStrategies = computed(() => {
      let filtered = strategies.value

      // Apply search filter
      if (searchQuery.value) {
        const query = searchQuery.value.toLowerCase()
        filtered = filtered.filter(strategy =>
          strategy.name.toLowerCase().includes(query) ||
          (strategy.description && strategy.description.toLowerCase().includes(query))
        )
      }

      // Apply status filter
      if (selectedStatus.value !== 'all') {
        filtered = filtered.filter(strategy => {
          switch (selectedStatus.value) {
            case 'running':
              return strategy.is_running && !strategy.is_paused
            case 'paused':
              return strategy.is_paused
            case 'stopped':
              return !strategy.is_running
            default:
              return true
          }
        })
      }

      // Apply sorting
      filtered = [...filtered].sort((a, b) => {
        switch (selectedSort.value) {
          case 'created_desc':
            return new Date(b.created_at) - new Date(a.created_at)
          case 'created_asc':
            return new Date(a.created_at) - new Date(b.created_at)
          case 'name_asc':
            return a.name.localeCompare(b.name)
          case 'name_desc':
            return b.name.localeCompare(a.name)
          case 'pnl_desc':
            return (b.pnl || 0) - (a.pnl || 0)
          case 'pnl_asc':
            return (a.pnl || 0) - (b.pnl || 0)
          default:
            return 0
        }
      })

      return filtered
    })

    // Methods
    const refreshStrategies = () => {
      // Smart data system will handle refresh automatically
    }

    const startStrategy = async (strategyId) => {
      try {
        await startStrategyAction(strategyId)
        addNotification({
          type: 'success',
          message: 'Strategy started successfully',
          duration: 3000
        })
      } catch (error) {
        addNotification({
          type: 'error',
          message: `Failed to start strategy: ${error.message}`,
          duration: 5000
        })
      }
    }

    const pauseStrategy = async (strategyId) => {
      try {
        await pauseStrategyAction(strategyId)
        addNotification({
          type: 'success',
          message: 'Strategy paused successfully',
          duration: 3000
        })
      } catch (error) {
        addNotification({
          type: 'error',
          message: `Failed to pause strategy: ${error.message}`,
          duration: 5000
        })
      }
    }

    const resumeStrategy = async (strategyId) => {
      try {
        await resumeStrategyAction(strategyId)
        addNotification({
          type: 'success',
          message: 'Strategy resumed successfully',
          duration: 3000
        })
      } catch (error) {
        addNotification({
          type: 'error',
          message: `Failed to resume strategy: ${error.message}`,
          duration: 5000
        })
      }
    }

    const monitorStrategy = (strategyId) => {
      router.push(`/strategies/monitor/${strategyId}`)
    }

    const backtestStrategy = (strategyId) => {
      router.push(`/strategies/backtest/${strategyId}`)
    }

    const confirmDeleteStrategy = (strategy) => {
      strategyToDelete.value = strategy
      showDeleteDialog.value = true
    }

    const deleteStrategy = async () => {
      if (!strategyToDelete.value) return

      isDeleting.value = true
      try {
        await deleteStrategyAction(strategyToDelete.value.strategy_id)
        addNotification({
          type: 'success',
          message: `Strategy "${strategyToDelete.value.name}" deleted successfully`,
          duration: 3000
        })
        showDeleteDialog.value = false
        strategyToDelete.value = null
      } catch (error) {
        addNotification({
          type: 'error',
          message: `Failed to delete strategy: ${error.message}`,
          duration: 5000
        })
      } finally {
        isDeleting.value = false
      }
    }

    const handleStrategyUploaded = () => {
      showUploadDialog.value = false
      addNotification({
        type: 'success',
        message: 'Strategy uploaded successfully',
        duration: 3000
      })
    }

    // Utility methods (same as dashboard)
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

    const formatDate = (dateString) => {
      if (!dateString) return '--'
      
      const date = new Date(dateString)
      return date.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric'
      })
    }

    return {
      // Reactive data
      strategies,
      showUploadDialog,
      searchQuery,
      selectedStatus,
      selectedSort,
      viewMode,
      showDeleteDialog,
      strategyToDelete,
      isDeleting,

      // Computed
      strategiesLoading,
      strategiesError,
      filteredStrategies,

      // Static data
      statusOptions,
      sortOptions,

      // Methods
      refreshStrategies,
      startStrategy,
      pauseStrategy,
      resumeStrategy,
      monitorStrategy,
      backtestStrategy,
      confirmDeleteStrategy,
      deleteStrategy,
      handleStrategyUploaded,
      isLoading,

      // Utility methods
      getStrategyStatusClass,
      getStatusClass,
      getStatusIcon,
      getStatusText,
      getPnLClass,
      formatCurrency,
      formatDate
    }
  }
}
</script>

<style scoped>
.strategy-library {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header Section */
.library-header {
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

.library-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.library-title i {
  color: var(--color-brand);
}

.library-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Filter Section */
.filter-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xl);
  padding: var(--spacing-lg);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
}

.search-controls {
  display: flex;
  gap: var(--spacing-md);
  align-items: center;
  flex: 1;
}

.search-input-container {
  position: relative;
  flex: 1;
  max-width: 300px;
}

.search-icon {
  position: absolute;
  left: var(--spacing-md);
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-tertiary);
  font-size: var(--font-size-md);
  z-index: 1;
}

.search-input {
  width: 100%;
  padding-left: 36px !important;
}

.status-filter,
.sort-filter {
  min-width: 150px;
}

.view-controls {
  display: flex;
  gap: var(--spacing-xs);
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
  flex-wrap: wrap;
}

/* Strategy List */
.strategy-list {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.list-header {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 2fr;
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

.strategy-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 2fr;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  transition: var(--transition-normal);
  align-items: center;
}

.strategy-row:hover {
  background: var(--bg-tertiary);
}

.strategy-row:last-child {
  border-bottom: none;
}

.strategy-row.status-running {
  border-left: 4px solid var(--color-success);
}

.strategy-row.status-paused {
  border-left: 4px solid var(--color-warning);
}

.strategy-row.status-stopped {
  border-left: 4px solid var(--color-danger);
}

.strategy-name-cell h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 2px 0;
}

.strategy-name-cell p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.3;
}

.pnl-value {
  font-weight: var(--font-weight-semibold);
}

.pnl-value.positive {
  color: var(--color-success);
}

.pnl-value.negative {
  color: var(--color-danger);
}

.trades-value,
.created-value {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.action-buttons {
  display: flex;
  gap: var(--spacing-xs);
  justify-content: flex-end;
}

/* Delete Confirmation Dialog */
.delete-confirmation {
  display: flex;
  gap: var(--spacing-md);
  align-items: flex-start;
}

.delete-icon {
  font-size: var(--font-size-2xl);
  color: var(--color-danger);
  flex-shrink: 0;
  margin-top: var(--spacing-xs);
}

.delete-message {
  flex: 1;
}

.delete-message h3 {
  font-size: var(--font-size-lg);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
}

.delete-message p {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-xs) 0;
}

.warning-text {
  color: var(--color-danger) !important;
  font-weight: var(--font-weight-medium);
}

/* Responsive Design */
@media (max-width: 1024px) {
  .filter-section {
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .search-controls {
    width: 100%;
  }

  .view-controls {
    align-self: flex-end;
  }

  .strategy-list {
    overflow-x: auto;
  }

  .list-header,
  .strategy-row {
    min-width: 800px;
  }
}

@media (max-width: 768px) {
  .strategy-library {
    padding: var(--spacing-lg);
  }

  .library-header {
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

  .search-controls {
    flex-direction: column;
    gap: var(--spacing-sm);
  }

  .search-input-container {
    max-width: none;
  }

  .status-filter,
  .sort-filter {
    min-width: auto;
    width: 100%;
  }

  .strategy-grid {
    grid-template-columns: 1fr;
  }

  .strategy-actions {
    justify-content: center;
  }

  .action-buttons {
    justify-content: center;
    flex-wrap: wrap;
  }
}
</style>
