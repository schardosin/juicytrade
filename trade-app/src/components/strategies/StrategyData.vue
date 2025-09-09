<template>
  <div class="strategy-data">
    <!-- Header -->
    <div class="data-header">
      <div class="header-content">
        <h1 class="page-title">💾 Data Management</h1>
        <p class="page-subtitle">Import and manage historical data for backtesting</p>
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
          @click="showImportDialog = true"
        >
          <span class="icon">📥</span>
          Import Data
        </button>
      </div>
    </div>

    <!-- Data Cards View -->
    <div v-if="importedData.length > 0 && viewMode === 'cards'" class="data-grid">
      <div 
        v-for="dataset in importedData" 
        :key="dataset.id"
        class="data-card"
      >
        <!-- Dataset Header -->
        <div class="dataset-header">
          <div class="dataset-info">
            <h3 class="dataset-name">{{ dataset.symbol }}</h3>
            <p class="dataset-description">{{ dataset.asset_type }} options data for {{ dataset.symbol }}</p>
            <div class="dataset-meta">
              <span class="meta-item">
                <span class="meta-label">Asset Type:</span>
                <span class="asset-badge" :class="`asset-${dataset.asset_type.toLowerCase()}`">
                  {{ dataset.asset_type }}
                </span>
              </span>
              <span class="meta-item">
                <span class="meta-label">Period:</span>
                {{ formatDateRange(dataset.start_date, dataset.end_date) }}
              </span>
              <span class="meta-item">
                <span class="meta-label">Imported:</span>
                {{ formatImportDate(dataset.imported_at) }}
              </span>
            </div>
          </div>
          <div class="dataset-actions-header">
            <button 
              class="btn-icon"
              @click="deleteDataset(dataset)"
              title="Delete Dataset"
            >
              🗑️
            </button>
          </div>
        </div>

        <!-- Dataset Summary -->
        <div class="dataset-summary">
          <div class="summary-stats">
            <div class="stat-item">
              <span class="stat-number">{{ formatNumber(dataset.partition_count) }}</span>
              <span class="stat-label">Partitions</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ formatNumber(dataset.record_count) }}</span>
              <span class="stat-label">Records</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ formatFileSize(dataset.file_size) }}</span>
              <span class="stat-label">Size</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">{{ formatDuration(dataset.start_date, dataset.end_date) }}</span>
              <span class="stat-label">Duration</span>
            </div>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="dataset-actions">
          <button 
            class="btn btn-outline"
            @click="viewDetails(dataset)"
          >
            <span class="icon">📊</span>
            View Details
          </button>
          <button 
            class="btn btn-secondary"
            @click="exportData(dataset)"
          >
            <span class="icon">📤</span>
            Export
          </button>
        </div>
      </div>
    </div>

    <!-- Data List View -->
    <div v-if="importedData.length > 0 && viewMode === 'list'" class="data-list">
      <div class="list-header">
        <div class="list-col col-dataset">Dataset</div>
        <div class="list-col col-asset">Asset Type</div>
        <div class="list-col col-period">Period</div>
        <div class="list-col col-symbols">Symbols</div>
        <div class="list-col col-records">Records</div>
        <div class="list-col col-size">Size</div>
        <div class="list-col col-imported">Imported</div>
        <div class="list-col col-actions">Actions</div>
      </div>
      
      <div 
        v-for="dataset in importedData" 
        :key="dataset.id"
        class="data-row"
      >
        <div class="list-col col-dataset">
          <div class="dataset-name-info">
            <h4 class="dataset-name">{{ dataset.symbol }}</h4>
            <p class="dataset-description">{{ dataset.asset_type }} options data for {{ dataset.symbol }}</p>
          </div>
        </div>
        <div class="list-col col-asset">
          <span class="asset-badge" :class="`asset-${dataset.asset_type.toLowerCase()}`">
            {{ dataset.asset_type }}
          </span>
        </div>
        <div class="list-col col-period">
          <span class="period-text">{{ formatDateRange(dataset.start_date, dataset.end_date) }}</span>
        </div>
        <div class="list-col col-symbols">
          <span class="stat-number">{{ formatNumber(dataset.symbol_count) }}</span>
        </div>
        <div class="list-col col-records">
          <span class="stat-number">{{ formatNumber(dataset.record_count) }}</span>
        </div>
        <div class="list-col col-size">
          <span class="size-text">{{ formatFileSize(dataset.file_size) }}</span>
        </div>
        <div class="list-col col-imported">
          <span class="imported-text">{{ formatImportDate(dataset.imported_at) }}</span>
        </div>
        <div class="list-col col-actions">
          <div class="list-actions">
            <button 
              class="btn btn-sm btn-outline"
              @click="viewDetails(dataset)"
              title="View Details"
            >
              📊
            </button>
            <button 
              class="btn btn-sm btn-secondary"
              @click="exportData(dataset)"
              title="Export Data"
            >
              📤
            </button>
            <button 
              class="btn btn-sm btn-danger"
              @click="deleteDataset(dataset)"
              title="Delete Dataset"
            >
              🗑️
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Loading imported data...</p>
    </div>

    <!-- Empty State -->
    <div v-else-if="!loading && importedData.length === 0" class="empty-state">
      <div class="empty-icon">💾</div>
      <h3 class="empty-title">No Data Imported Yet</h3>
      <p class="empty-description">
        Import your first dataset to start backtesting strategies with historical data. 
        Click "Import Data" above to get started.
      </p>
      <div class="empty-actions">
        <button 
          class="btn btn-primary"
          @click="showImportDialog = true"
        >
          <span class="icon">📥</span>
          Import Your First Dataset
        </button>
      </div>
    </div>

    <!-- Import Dialog -->
    <DataImportDialog 
      v-model:visible="showImportDialog"
      @imported="handleDataImported"
    />

    <!-- Delete Confirmation Dialog -->
    <div v-if="showDeleteDialog" class="dialog-overlay" @click="closeDeleteDialog">
      <div class="dialog-content" @click.stop>
        <div class="dialog-header">
          <h2>Delete Dataset</h2>
          <button class="btn-close" @click="closeDeleteDialog">×</button>
        </div>
        <div class="dialog-body">
          <p>Are you sure you want to delete the dataset <strong>{{ selectedDataset?.symbol }}</strong>?</p>
          <p class="warning-text">This action cannot be undone and will permanently remove all imported data.</p>
        </div>
        <div class="dialog-footer">
          <button class="btn btn-outline" @click="closeDeleteDialog">Cancel</button>
          <button class="btn btn-danger" @click="confirmDelete" :disabled="deleting">
            <span v-if="deleting">Deleting...</span>
            <span v-else>Delete Dataset</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNotifications } from '../../composables/useNotifications.js'
import { api } from '../../services/api.js'
import DataImportDialog from './DataImportDialog.vue'

export default {
  name: 'StrategyData',
  components: {
    DataImportDialog
  },
  setup() {
    const router = useRouter()
    const { showSuccess, showError } = useNotifications()
    
    // Reactive state
    const importedData = ref([])
    const loading = ref(true)
    const error = ref(null)
    
    // Dialog states
    const showImportDialog = ref(false)
    const showDeleteDialog = ref(false)
    const selectedDataset = ref(null)
    const deleting = ref(false)
    
    // View mode state
    const viewMode = ref('cards') // 'cards' or 'list'

    // Load imported data
    const loadImportedData = async () => {
      try {
        loading.value = true
        const response = await api.get('/api/data-import/imported-data')
        importedData.value = response.data.data || []
      } catch (err) {
        console.error('Error loading imported data:', err)
        error.value = 'Failed to load imported data'
        showError('Failed to load imported data', 'Data Error')
      } finally {
        loading.value = false
      }
    }

    // Helper functions
    const formatDateRange = (startDate, endDate) => {
      if (!startDate || !endDate) return 'N/A'
      // Parse as local date to avoid timezone conversion issues
      const start = new Date(startDate + 'T00:00:00').toLocaleDateString()
      const end = new Date(endDate + 'T00:00:00').toLocaleDateString()
      return `${start} - ${end}`
    }

    const formatImportDate = (dateString) => {
      if (!dateString) return 'N/A'
      const date = new Date(dateString)
      return date.toLocaleString([], { 
        month: 'short', 
        day: 'numeric', 
        year: 'numeric',
        hour: '2-digit', 
        minute: '2-digit'
      })
    }

    const formatNumber = (number) => {
      if (number === null || number === undefined) return '0'
      return number.toLocaleString()
    }

    const formatFileSize = (bytes) => {
      if (!bytes) return '0 B'
      const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
      const i = Math.floor(Math.log(bytes) / Math.log(1024))
      return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`
    }

    const formatDuration = (startDate, endDate) => {
      if (!startDate || !endDate) return 'N/A'
      
      const start = new Date(startDate)
      const end = new Date(endDate)
      const diffMs = end - start
      
      if (diffMs < 0) return 'Invalid'
      
      const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
      
      if (diffDays > 365) {
        const years = Math.floor(diffDays / 365)
        return `${years}y`
      } else if (diffDays > 30) {
        const months = Math.floor(diffDays / 30)
        return `${months}mo`
      } else if (diffDays > 0) {
        return `${diffDays}d`
      } else {
        const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
        return diffHours > 0 ? `${diffHours}h` : '<1h'
      }
    }

    // Actions
    const viewDetails = (dataset) => {
      // TODO: Implement dataset details view
      showSuccess(`Viewing details for ${dataset.symbol}`, 'Feature Preview')
    }

    const exportData = (dataset) => {
      // TODO: Implement data export functionality
      showSuccess(`Exporting ${dataset.symbol}`, 'Feature Preview')
    }

    const deleteDataset = (dataset) => {
      selectedDataset.value = dataset
      showDeleteDialog.value = true
    }

    const closeDeleteDialog = () => {
      showDeleteDialog.value = false
      selectedDataset.value = null
      deleting.value = false
    }

    const confirmDelete = async () => {
      if (!selectedDataset.value) return
      
      try {
        deleting.value = true
        
        // Pass the asset_type as a query parameter to ensure only the specific asset type is deleted
        const assetType = selectedDataset.value.asset_type
        const url = `/api/data-import/imported-data/${selectedDataset.value.symbol}?asset_type=${encodeURIComponent(assetType)}`
        const response = await api.delete(url)
        
        if (response.data.success) {
          showSuccess(`${assetType} dataset for "${selectedDataset.value.symbol}" deleted successfully!`, 'Dataset Deleted')
          await loadImportedData() // Reload data
          closeDeleteDialog()
        } else {
          showError(`Failed to delete dataset: ${response.data.error}`, 'Delete Error')
        }
      } catch (error) {
        console.error('Error deleting dataset:', error)
        showError('Failed to delete dataset. Please try again.', 'Delete Error')
      } finally {
        deleting.value = false
      }
    }

    // Event handlers
    const handleDataImported = () => {
      showImportDialog.value = false
      loadImportedData()
      showSuccess('Data import completed successfully!', 'Import Complete')
    }

    // Lifecycle
    onMounted(() => {
      loadImportedData()
    })

    return {
      // State
      importedData,
      loading,
      error,
      viewMode,
      
      // Dialog states
      showImportDialog,
      showDeleteDialog,
      selectedDataset,
      deleting,
      
      // Helper functions
      formatDateRange,
      formatImportDate,
      formatNumber,
      formatFileSize,
      formatDuration,
      
      // Actions
      viewDetails,
      exportData,
      deleteDataset,
      closeDeleteDialog,
      confirmDelete,
      
      // Event handlers
      handleDataImported
    }
  }
}
</script>

<style scoped>
.strategy-data {
  padding: var(--spacing-xl);
  height: 100%;
  overflow-y: auto;
  background-color: var(--bg-secondary);
}

.data-header {
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

.data-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: var(--spacing-xl);
}

.data-card {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  transition: var(--transition-normal);
}

.data-card:hover {
  border-color: var(--color-brand);
  box-shadow: 0 4px 12px rgba(255, 107, 53, 0.1);
}

.dataset-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.dataset-info {
  flex: 1;
}

.dataset-name {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
}

.dataset-description {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-md);
  line-height: 1.4;
}

.dataset-meta {
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

.asset-badge {
  padding: 2px var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
  font-size: var(--font-size-xs);
}

.asset-options { background: var(--color-brand); color: var(--text-primary); opacity: 0.8; }
.asset-stocks { background: var(--color-info); color: var(--text-primary); opacity: 0.8; }
.asset-futures { background: var(--color-warning); color: var(--text-primary); opacity: 0.8; }
.asset-forex { background: var(--color-success); color: var(--text-primary); opacity: 0.8; }

.dataset-actions-header {
  display: flex;
  gap: var(--spacing-xs);
}

.dataset-summary {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
}

.summary-stats {
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

.dataset-actions {
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

/* List View Styles */
.data-list {
  background: var(--bg-tertiary);
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--border-primary);
}

.list-header {
  display: grid;
  grid-template-columns: 2fr 1fr 1.5fr 0.8fr 1fr 0.8fr 1fr 1.2fr;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background: var(--bg-quaternary);
  border-bottom: 1px solid var(--border-primary);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  text-transform: uppercase;
}

.data-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1.5fr 0.8fr 1fr 0.8fr 1fr 1.2fr;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  transition: var(--transition-fast);
  align-items: center;
}

.data-row:hover {
  background: var(--bg-primary);
}

.data-row:last-child {
  border-bottom: none;
}

.list-col {
  display: flex;
  align-items: center;
}

.col-dataset {
  flex-direction: column;
  align-items: flex-start;
}

.dataset-name-info .dataset-name {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
}

.dataset-name-info .dataset-description {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.col-asset,
.col-period,
.col-symbols,
.col-records,
.col-size,
.col-imported {
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

.period-text,
.imported-text,
.size-text {
  font-size: var(--font-size-sm);
  color: var(--text-primary);
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
  max-width: 500px;
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

.dialog-body p {
  margin: 0 0 var(--spacing-md) 0;
  color: var(--text-primary);
}

.warning-text {
  color: var(--color-warning);
  font-size: var(--font-size-sm);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
}

/* Responsive List View */
@media (max-width: 1200px) {
  .list-header,
  .data-row {
    grid-template-columns: 2fr 1fr 1.2fr 0.8fr 1.2fr;
  }
  
  .col-period,
  .col-records,
  .col-imported {
    display: none;
  }
}

@media (max-width: 768px) {
  .list-header,
  .data-row {
    grid-template-columns: 2fr 1fr 1.2fr;
    gap: var(--spacing-sm);
  }
  
  .col-period,
  .col-symbols,
  .col-records,
  .col-size,
  .col-imported {
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
