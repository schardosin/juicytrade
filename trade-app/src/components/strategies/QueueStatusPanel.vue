<template>
  <div v-if="shouldShowPanel" class="queue-status-panel" :class="{ 'expanded': isExpanded }">
    <!-- Queue Header -->
    <div class="queue-header" @click="toggleExpanded">
      <div class="queue-title">
        <span class="queue-icon">📋</span>
        <h4>Import Queue</h4>
        <span class="queue-badge" :class="queueStatusClass">
          {{ queueStatusText }}
        </span>
      </div>
      <div class="queue-controls">
        <span class="queue-summary">{{ queueSummaryText }}</span>
        <button class="expand-btn" :class="{ 'expanded': isExpanded }">
          <span>{{ isExpanded ? '▼' : '▶' }}</span>
        </button>
      </div>
    </div>

    <!-- Queue Content (Expandable) -->
    <div v-if="isExpanded" class="queue-content">
      <!-- Overall Progress -->
      <div class="queue-progress">
        <div class="progress-header">
          <span class="progress-label">Overall Progress</span>
          <span class="progress-percentage">{{ queueProgress }}%</span>
        </div>
        <div class="progress-bar">
          <div 
            class="progress-fill" 
            :style="{ width: `${queueProgress}%` }"
          ></div>
        </div>
        <div class="progress-details">
          <span>{{ completedItems }} of {{ totalItems }} files completed</span>
          <span v-if="failedItems > 0" class="failed-count">{{ failedItems }} failed</span>
        </div>
      </div>

      <!-- Current Processing -->
      <div v-if="currentProcessing" class="current-processing">
        <div class="processing-header">
          <span class="processing-icon">⚡</span>
          <h5>Currently Processing</h5>
        </div>
        <div class="processing-details">
          <div class="processing-file">
            <span class="file-name">{{ currentProcessing.filename }}</span>
            <span class="file-index">File {{ currentFileIndex }} of {{ totalItems }}</span>
          </div>
          <div class="processing-progress">
            <div class="processing-spinner"></div>
            <span class="processing-status">{{ processingStatusText }}</span>
          </div>
        </div>
      </div>

      <!-- Queue Items -->
      <div v-if="queuedItems > 0" class="queued-items">
        <div class="queued-header">
          <span class="queued-icon">⏳</span>
          <h5>Queued Files</h5>
          <span class="queued-count">{{ queuedItems }} remaining</span>
        </div>
        <div class="queued-message">
          {{ queuedItems }} file{{ queuedItems > 1 ? 's' : '' }} waiting to be processed
        </div>
      </div>

      <!-- Queue Actions -->
      <div class="queue-actions">
        <button 
          class="btn btn-sm btn-outline"
          @click="refreshQueue"
          :disabled="refreshing"
        >
          <span v-if="refreshing">🔄</span>
          <span v-else>🔄</span>
          Refresh
        </button>
        <button 
          v-if="hasActiveItems"
          class="btn btn-sm btn-danger"
          @click="showClearQueueDialog = true"
        >
          🗑️ Clear Queue
        </button>
      </div>
    </div>

    <!-- Clear Queue Confirmation Dialog -->
    <div v-if="showClearQueueDialog" class="dialog-overlay" @click="showClearQueueDialog = false">
      <div class="dialog-content" @click.stop>
        <div class="dialog-header">
          <h3>Clear Import Queue</h3>
          <button class="btn-close" @click="showClearQueueDialog = false">×</button>
        </div>
        <div class="dialog-body">
          <p>Are you sure you want to clear the import queue?</p>
          <p class="warning-text">This will cancel all pending imports. Currently running imports will continue.</p>
        </div>
        <div class="dialog-footer">
          <button class="btn btn-outline" @click="showClearQueueDialog = false">Cancel</button>
          <button class="btn btn-danger" @click="clearQueue" :disabled="clearing">
            <span v-if="clearing">Clearing...</span>
            <span v-else>Clear Queue</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useGlobalBackgroundJobs } from '../../composables/useBackgroundJobs.js'
import { useNotifications } from '../../composables/useNotifications.js'
import { api } from '../../services/api.js'

export default {
  name: 'QueueStatusPanel',
  setup() {
    const { showSuccess, showError } = useNotifications()
    const { queueStatus, hasQueueItems, queueProgress, queueStatusMessage, refresh } = useGlobalBackgroundJobs()
    
    // Local state
    const isExpanded = ref(false)
    const showClearQueueDialog = ref(false)
    const refreshing = ref(false)
    const clearing = ref(false)

    // Computed properties
    const shouldShowPanel = computed(() => {
      return hasQueueItems.value || (queueStatus.value && queueStatus.value.total_items > 0)
    })

    const totalItems = computed(() => queueStatus.value?.total_items || 0)
    const completedItems = computed(() => queueStatus.value?.completed_items || 0)
    const failedItems = computed(() => queueStatus.value?.failed_items || 0)
    const queuedItems = computed(() => queueStatus.value?.queued_items || 0)
    const currentProcessing = computed(() => queueStatus.value?.current_processing || null)

    const currentFileIndex = computed(() => {
      if (!currentProcessing.value) return 0
      return completedItems.value + 1
    })

    const hasActiveItems = computed(() => {
      return queuedItems.value > 0 || currentProcessing.value !== null
    })

    const queueStatusClass = computed(() => {
      if (currentProcessing.value) return 'processing'
      if (queuedItems.value > 0) return 'queued'
      if (failedItems.value > 0) return 'failed'
      if (completedItems.value === totalItems.value && totalItems.value > 0) return 'completed'
      return 'idle'
    })

    const queueStatusText = computed(() => {
      if (currentProcessing.value) return 'Processing'
      if (queuedItems.value > 0) return 'Queued'
      if (failedItems.value > 0 && completedItems.value + failedItems.value === totalItems.value) return 'Complete with Errors'
      if (completedItems.value === totalItems.value && totalItems.value > 0) return 'Completed'
      return 'Idle'
    })

    const queueSummaryText = computed(() => {
      if (currentProcessing.value) {
        return `${currentFileIndex.value}/${totalItems.value} processing`
      }
      if (queuedItems.value > 0) {
        return `${queuedItems.value} queued`
      }
      if (totalItems.value > 0) {
        return `${completedItems.value}/${totalItems.value} done`
      }
      return 'No items'
    })

    const processingStatusText = computed(() => {
      if (!currentProcessing.value) return ''
      return `Processing file ${currentFileIndex.value} of ${totalItems.value}`
    })

    // Methods
    const toggleExpanded = () => {
      isExpanded.value = !isExpanded.value
    }

    const refreshQueue = async () => {
      try {
        refreshing.value = true
        await refresh()
        showSuccess('Queue status refreshed', 'Queue Updated')
      } catch (error) {
        console.error('Error refreshing queue:', error)
        showError('Failed to refresh queue status', 'Queue Error')
      } finally {
        refreshing.value = false
      }
    }

    const clearQueue = async () => {
      try {
        clearing.value = true
        const response = await api.delete('/api/data-import/queue/clear')
        
        if (response.data.success) {
          showSuccess('Import queue cleared successfully', 'Queue Cleared')
          showClearQueueDialog.value = false
          await refresh()
        } else {
          throw new Error(response.data.error || 'Failed to clear queue')
        }
      } catch (error) {
        console.error('Error clearing queue:', error)
        showError('Failed to clear queue: ' + error.message, 'Queue Error')
      } finally {
        clearing.value = false
      }
    }

    return {
      // State
      isExpanded,
      showClearQueueDialog,
      refreshing,
      clearing,
      
      // Computed
      shouldShowPanel,
      totalItems,
      completedItems,
      failedItems,
      queuedItems,
      currentProcessing,
      currentFileIndex,
      hasActiveItems,
      queueStatusClass,
      queueStatusText,
      queueSummaryText,
      processingStatusText,
      queueProgress,
      
      // Methods
      toggleExpanded,
      refreshQueue,
      clearQueue
    }
  }
}
</script>

<style scoped>
.queue-status-panel {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  margin-bottom: var(--spacing-lg);
  overflow: hidden;
  transition: var(--transition-normal);
}

.queue-status-panel:hover {
  border-color: var(--color-brand);
  box-shadow: 0 2px 8px rgba(255, 107, 53, 0.1);
}

.queue-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg);
  cursor: pointer;
  transition: var(--transition-fast);
}

.queue-header:hover {
  background: var(--bg-quaternary);
}

.queue-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.queue-icon {
  font-size: var(--font-size-lg);
}

.queue-title h4 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.queue-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
}

.queue-badge.processing {
  background: var(--color-brand);
  color: var(--text-primary);
  animation: pulse 2s infinite;
}

.queue-badge.queued {
  background: var(--color-warning);
  color: var(--text-primary);
}

.queue-badge.completed {
  background: var(--color-success);
  color: var(--text-primary);
}

.queue-badge.failed {
  background: var(--color-danger);
  color: var(--text-primary);
}

.queue-badge.idle {
  background: var(--bg-quaternary);
  color: var(--text-secondary);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

.queue-controls {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.queue-summary {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.expand-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: var(--spacing-xs);
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
  font-size: var(--font-size-sm);
}

.expand-btn:hover {
  background: var(--bg-quaternary);
  color: var(--text-primary);
}

.expand-btn.expanded {
  transform: rotate(0deg);
}

.queue-content {
  padding: 0 var(--spacing-lg) var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
  background: var(--bg-primary);
}

/* Queue Progress */
.queue-progress {
  margin-bottom: var(--spacing-lg);
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-sm);
}

.progress-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.progress-percentage {
  font-size: var(--font-size-sm);
  color: var(--color-brand);
  font-weight: var(--font-weight-semibold);
}

.progress-bar {
  width: 100%;
  height: 6px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  overflow: hidden;
  margin-bottom: var(--spacing-sm);
}

.progress-fill {
  height: 100%;
  background: var(--color-brand);
  transition: width 0.3s ease;
  border-radius: var(--radius-sm);
}

.progress-details {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}

.failed-count {
  color: var(--color-danger);
  font-weight: var(--font-weight-medium);
}

/* Current Processing */
.current-processing {
  background: rgba(255, 107, 53, 0.05);
  border: 1px solid rgba(255, 107, 53, 0.2);
  border-radius: var(--radius-md);
  padding: var(--spacing-md);
  margin-bottom: var(--spacing-lg);
}

.processing-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
}

.processing-icon {
  font-size: var(--font-size-md);
  animation: bounce 1s infinite;
}

@keyframes bounce {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-2px); }
}

.processing-header h5 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
}

.processing-details {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.processing-file {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.file-name {
  font-size: var(--font-size-sm);
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
  font-family: monospace;
}

.file-index {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: var(--spacing-xs);
  border-radius: var(--radius-xs);
}

.processing-progress {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.processing-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-brand);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.processing-status {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}

/* Queued Items */
.queued-items {
  background: rgba(255, 193, 7, 0.05);
  border: 1px solid rgba(255, 193, 7, 0.2);
  border-radius: var(--radius-md);
  padding: var(--spacing-md);
  margin-bottom: var(--spacing-lg);
}

.queued-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
}

.queued-icon {
  font-size: var(--font-size-md);
}

.queued-header h5 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  flex: 1;
}

.queued-count {
  font-size: var(--font-size-xs);
  color: var(--color-warning);
  background: rgba(255, 193, 7, 0.1);
  padding: var(--spacing-xs);
  border-radius: var(--radius-xs);
  font-weight: var(--font-weight-medium);
}

.queued-message {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

/* Queue Actions */
.queue-actions {
  display: flex;
  gap: var(--spacing-sm);
  justify-content: flex-end;
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-normal);
  border: none;
  text-decoration: none;
}

.btn-sm {
  padding: var(--spacing-xs) var(--spacing-sm);
  font-size: var(--font-size-xs);
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
  max-width: 400px;
  width: 90vw;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.dialog-header h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
}

.btn-close {
  background: none;
  border: none;
  font-size: var(--font-size-lg);
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
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
}

.warning-text {
  color: var(--color-warning);
  font-size: var(--font-size-sm);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-sm);
  padding: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
}

/* Responsive Design */
@media (max-width: 768px) {
  .queue-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
  }
  
  .queue-controls {
    align-self: flex-end;
  }
  
  .processing-file {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-xs);
  }
  
  .progress-details {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-xs);
  }
}
</style>
