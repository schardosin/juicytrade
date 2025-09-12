<template>
  <div v-if="visible" class="dialog-overlay" @click="closeDialog">
    <div class="dialog-content" @click.stop>
      <div class="dialog-header">
        <h2>📥 Import Data</h2>
        <button class="btn-close" @click="closeDialog">×</button>
      </div>
      
      <!-- Progress Steps -->
      <div class="steps-progress">
        <div 
          v-for="(step, index) in steps" 
          :key="index"
          class="step-item"
          :class="{ 
            active: currentStep === index, 
            completed: currentStep > index,
            disabled: currentStep < index 
          }"
        >
          <div class="step-number">
            <span v-if="currentStep > index">✓</span>
            <span v-else>{{ index + 1 }}</span>
          </div>
          <div class="step-label">{{ step.label }}</div>
        </div>
      </div>

      <div class="dialog-body">
        <!-- Step 1: File Selection -->
        <div v-if="currentStep === 0" class="step-content">
          <h3>Select Import Files</h3>
          <p class="step-description">Choose one or more files to import. Multiple files will be processed sequentially in a queue.</p>
          
          <!-- Selection Mode Toggle -->
          <div class="selection-mode-toggle">
            <button 
              class="mode-btn"
              :class="{ active: selectionMode === 'single' }"
              @click="selectionMode = 'single'"
            >
              Single File
            </button>
            <button 
              class="mode-btn"
              :class="{ active: selectionMode === 'multiple' }"
              @click="selectionMode = 'multiple'"
            >
              Multiple Files
            </button>
          </div>
          
          <div v-if="loadingFiles" class="loading-files">
            <div class="spinner-small"></div>
            <span>Loading available files...</span>
          </div>
          
          <div v-else-if="availableFiles.length === 0" class="no-files">
            <div class="no-files-icon">📁</div>
            <h4>No Import Files Found</h4>
            <p>No import files were found in the dbn_files directory. Please add DBN or CSV files to continue.</p>
          </div>
          
          <div v-else-if="availableFiles && availableFiles.length > 0" class="file-selector">
            <!-- Multi-select controls -->
            <div v-if="selectionMode === 'multiple'" class="multi-select-controls">
              <button 
                class="btn btn-sm btn-outline"
                @click="selectAllFiles"
                :disabled="selectedFiles.length === availableFiles.length"
              >
                Select All
              </button>
              <button 
                class="btn btn-sm btn-outline"
                @click="clearSelection"
                :disabled="selectedFiles.length === 0"
              >
                Clear All
              </button>
              <span class="selection-count">{{ selectedFiles.length }} of {{ availableFiles.length }} files selected</span>
            </div>
            
            <div class="file-list">
              <div 
                v-for="file in availableFiles" 
                :key="file?.filename || file?.file_path || Math.random()"
                class="file-option"
                :class="{ 
                  selected: isFileSelected(file),
                  'multi-select': selectionMode === 'multiple'
                }"
                @click="toggleFileSelection(file)"
              >
                <div class="file-info">
                  <h4 class="file-name">{{ file?.filename || 'Unknown File' }}</h4>
                  <div class="file-details">
                    <span class="file-size">{{ formatFileSize(file?.file_size) }}</span>
                    <span class="file-date">{{ formatFileDate(file?.modified_at) }}</span>
                    <span v-if="file?.file_type === 'csv'" class="file-type-badge csv">CSV</span>
                    <span v-else class="file-type-badge dbn">DBN</span>
                  </div>
                </div>
                <div class="file-status">
                  <div v-if="selectionMode === 'multiple'" class="checkbox-indicator">
                    <input 
                      type="checkbox" 
                      :checked="isFileSelected(file)"
                      @click.stop
                      @change="toggleFileSelection(file)"
                    />
                  </div>
                  <div v-else-if="isFileSelected(file)" class="selected-indicator">✓</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 2: Metadata Preview -->
        <div v-if="currentStep === 1" class="step-content">
          <h3>File Preview</h3>
          <p v-if="selectionMode === 'single'" class="step-description">Review the file information before importing all data.</p>
          <p v-else class="step-description">Review the selected files before importing. Multiple files will be processed sequentially.</p>
          
          <!-- Multi-file Summary -->
          <div v-if="selectionMode === 'multiple'" class="multi-file-summary">
            <div class="summary-header">
              <h4>Selected Files Summary</h4>
              <span class="file-count">{{ selectedFiles.length }} files selected</span>
            </div>
            
            <div class="files-list">
              <div 
                v-for="file in selectedFiles" 
                :key="file.filename"
                class="file-summary-item"
              >
                <div class="file-summary-info">
                  <h5 class="file-summary-name">{{ file.filename }}</h5>
                  <div class="file-summary-details">
                    <span class="file-size">{{ formatFileSize(file.file_size) }}</span>
                    <span v-if="file.file_type === 'csv'" class="file-type-badge csv">CSV</span>
                    <span v-else class="file-type-badge dbn">DBN</span>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- Multi-file Import Notice -->
            <div class="import-notice">
              <div class="notice-icon">📋</div>
              <div class="notice-content">
                <h4>Multi-File Import</h4>
                <p>All {{ selectedFiles.length }} selected files will be imported sequentially. Each file will be processed as a separate import job and added to the import queue.</p>
              </div>
            </div>
          </div>
          
          <!-- Single File Details (existing logic) -->
          <div v-else>
            <!-- CSV Symbol Input -->
            <div v-if="isCSVFile" class="csv-symbol-input">
              <div class="form-group">
                <label>Symbol (required for CSV files):</label>
                <input 
                  type="text" 
                  v-model="csvSymbol"
                  placeholder="e.g., SPX"
                  class="form-input"
                  required
                />
                <p class="form-help">Enter the single symbol that this CSV file contains data for.</p>
              </div>
              
              <div class="form-group">
                <label>Timestamp Convention:</label>
                <div class="timestamp-convention-selector">
                  <label class="radio-group">
                    <input 
                      type="radio" 
                      name="timestampConvention"
                      v-model="timestampConvention"
                      value="begin_of_minute"
                    />
                    <span>Beginning of Minute (9:30)</span>
                  </label>
                  <label class="radio-group">
                    <input 
                      type="radio" 
                      name="timestampConvention"
                      v-model="timestampConvention"
                      value="end_of_minute"
                    />
                    <span>End of Minute (9:31) - TradeStation</span>
                  </label>
                </div>
                <p class="form-help">
                  <strong>Beginning of Minute:</strong> 9:30 timestamp represents the 9:30-9:31 minute bar (standard)<br>
                  <strong>End of Minute:</strong> 9:31 timestamp represents the 9:30-9:31 minute bar (TradeStation format)
                </p>
              </div>
            </div>
            
            <div v-if="loadingMetadata" class="loading-metadata">
              <div class="spinner-small"></div>
              <span>Extracting metadata...</span>
            </div>
            
            <div v-else-if="metadata" class="metadata-preview">
              <!-- Dataset Info -->
              <div class="metadata-section">
                <h4>Dataset Information</h4>
                <div class="metadata-grid">
                  <div class="metadata-item">
                    <span class="metadata-label">File:</span>
                    <span class="metadata-value">{{ selectedFile?.filename }}</span>
                  </div>
                  <div class="metadata-item">
                    <span class="metadata-label">Dataset:</span>
                    <span class="metadata-value">{{ metadata.dataset }}</span>
                  </div>
                  <div class="metadata-item">
                    <span class="metadata-label">Asset Types:</span>
                    <div class="asset-types">
                      <span 
                        v-for="assetType in metadata.asset_types" 
                        :key="assetType"
                        class="asset-badge"
                        :class="`asset-${assetType.toLowerCase()}`"
                      >
                        {{ assetType }}
                      </span>
                    </div>
                  </div>
                  <div class="metadata-item">
                    <span class="metadata-label">Date Range:</span>
                    <span class="metadata-value">{{ formatDateRange(metadata?.overall_date_range?.start_date || metadata?.start_date, metadata?.overall_date_range?.end_date || metadata?.end_date) }}</span>
                  </div>
                  <div class="metadata-item">
                    <span class="metadata-label">Total Records:</span>
                    <span class="metadata-value">{{ formatNumber(metadata?.total_records) }}</span>
                  </div>
                  <div class="metadata-item">
                    <span class="metadata-label">Unique Symbols:</span>
                    <span class="metadata-value">{{ formatNumber(metadata?.symbols?.length) }}</span>
                  </div>
                </div>
              </div>

              <!-- Import Notice -->
              <div class="import-notice">
                <div class="notice-icon">ℹ️</div>
                <div class="notice-content">
                  <h4>Complete File Import</h4>
                  <p>This will import <strong>ALL data</strong> from the selected file without any filtering. All symbols and dates will be processed to ensure complete data coverage.</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 3: Import Configuration -->
        <div v-if="currentStep === 2" class="step-content">
          <h3>Import Configuration</h3>
          <p class="step-description">Configure the import job settings.</p>
          
          <div class="config-form">
            <div class="form-group">
              <label>Job Name:</label>
              <input 
                type="text" 
                v-model="importConfig.job_name"
                placeholder="Enter a name for this import job"
                class="form-input"
              />
            </div>
            
            <div class="form-group">
              <label class="checkbox-group">
                <input 
                  type="checkbox" 
                  v-model="importConfig.overwrite_existing"
                />
                <span>Overwrite existing data</span>
              </label>
              <p class="form-help">If checked, existing data for the same date range will be replaced.</p>
            </div>

            <!-- Import Summary -->
            <div class="import-summary">
              <h4>Import Summary</h4>
              <div class="summary-grid">
                <div class="summary-item">
                  <span class="summary-label">File:</span>
                  <span class="summary-value">{{ selectedFile?.filename }}</span>
                </div>
                <div class="summary-item">
                  <span class="summary-label">Dataset:</span>
                  <span class="summary-value">{{ metadata?.dataset }}</span>
                </div>
                <div class="summary-item">
                  <span class="summary-label">Date Range:</span>
                  <span class="summary-value">{{ formatDateRange(metadata?.overall_date_range?.start_date || metadata?.start_date, metadata?.overall_date_range?.end_date || metadata?.end_date) }}</span>
                </div>
                <div class="summary-item">
                  <span class="summary-label">Import Mode:</span>
                  <span class="summary-value">Complete File (No Filtering)</span>
                </div>
                <div class="summary-item">
                  <span class="summary-label">Job Name:</span>
                  <span class="summary-value">{{ importConfig.job_name || 'Unnamed Import' }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 4: Import Progress -->
        <div v-if="currentStep === 3" class="step-content">
          <h3>Import Progress</h3>
          <p class="step-description">Your data is being imported. This may take a few minutes.</p>
          
          <div class="import-progress">
            <div class="progress-info">
              <div class="progress-status">
                <span class="status-text">{{ importStatus.status }}</span>
                <span class="progress-percentage">{{ importStatus.progress }}%</span>
              </div>
              <div class="progress-bar">
                <div 
                  class="progress-fill" 
                  :style="{ width: `${importStatus.progress}%` }"
                ></div>
              </div>
            </div>
            
            <div v-if="importStatus.current_symbol" class="current-activity">
              <span class="activity-label">Status:</span>
              <span class="activity-value">{{ importStatus.current_symbol }}</span>
            </div>
            
            <div class="progress-stats">
              <div class="stat-item">
                <span class="stat-number">{{ formatNumber(importStatus.processed_records || 0) }}</span>
                <span class="stat-label">Records Processed</span>
              </div>
              <div class="stat-item">
                <span class="stat-number">{{ formatNumber(importStatus.symbols_processed || 0) }}</span>
                <span class="stat-label">Symbols Processed</span>
              </div>
              <div class="stat-item">
                <span class="stat-number">{{ formatDuration(importStatus.started_at) }}</span>
                <span class="stat-label">Elapsed Time</span>
              </div>
            </div>

            <div v-if="importStatus.error" class="import-error">
              <h4>Import Error</h4>
              <p>{{ importStatus.error }}</p>
            </div>

            <div v-if="importStatus.completed" class="import-complete">
              <div class="success-icon">✅</div>
              <h4>Import Complete!</h4>
              <p>Successfully imported {{ formatNumber(importStatus.processed_records) }} records.</p>
            </div>
          </div>
        </div>
      </div>

      <div class="dialog-footer">
        <button 
          v-if="currentStep > 0 && currentStep < 3" 
          class="btn btn-outline" 
          @click="previousStep"
        >
          Previous
        </button>
        
        <button 
          v-if="currentStep < 2" 
          class="btn btn-primary" 
          @click="nextStep"
          :disabled="!canProceed || loadingMetadata"
        >
          <div v-if="loadingMetadata" class="spinner-small"></div>
          <span v-if="loadingMetadata">Loading...</span>
          <span v-else>Next</span>
        </button>
        
        <button 
          v-if="currentStep === 2" 
          class="btn btn-primary" 
          @click="startImport"
          :disabled="importing || !canStartImport"
        >
          <span v-if="importing">Starting Import...</span>
          <span v-else>Start Import</span>
        </button>
        
        <button 
          v-if="currentStep === 3 && !importStatus.completed" 
          class="btn btn-danger" 
          @click="cancelImport"
          :disabled="!importJobId"
        >
          Cancel Import
        </button>
        
        <button 
          v-if="currentStep === 3 && importStatus.completed" 
          class="btn btn-primary" 
          @click="finishImport"
        >
          Finish
        </button>
        
        <button 
          v-if="currentStep === 3 && importing && !importStatus.completed && !importStatus.failed"
          class="btn btn-outline" 
          @click="closeInBackground"
        >
          Close (Continue in Background)
        </button>
        
        <button 
          v-else
          class="btn btn-outline" 
          @click="closeDialog"
        >
          {{ currentStep === 3 && importStatus.completed ? 'Close' : 'Cancel' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch } from 'vue'
import { api } from '../../services/api.js'
import { useNotifications } from '../../composables/useNotifications.js'

export default {
  name: 'DataImportDialog',
  props: {
    visible: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:visible', 'imported'],
  setup(props, { emit }) {
    const { showError, showSuccess } = useNotifications()
    
    // Dialog state
    const currentStep = ref(0)
    const steps = [
      { label: 'Select File' },
      { label: 'Configure' },
      { label: 'Import' }
    ]
    
    // File selection
    const availableFiles = ref([])
    const selectedFile = ref(null)
    const selectedFiles = ref([])
    const selectionMode = ref('single')
    const loadingFiles = ref(false)
    
    // CSV-specific
    const csvSymbol = ref('')
    const timestampConvention = ref('begin_of_minute') // Default to standard convention
    
    // Metadata
    const metadata = ref(null)
    const loadingMetadata = ref(false)
    
    // Import configuration
    const importConfig = ref({
      start_date: '',
      end_date: '',
      symbol_mode: 'all',
      symbol_list: '',
      job_name: '',
      overwrite_existing: false
    })
    
    // Import progress
    const importing = ref(false)
    const importJobId = ref(null)
    const importStatus = ref({
      status: 'Initializing...',
      progress: 0,
      current_symbol: '',
      processed_records: 0,
      symbols_processed: 0,
      started_at: null,
      completed: false,
      error: null
    })
    
    let progressInterval = null

    // Computed properties
    const isCSVFile = computed(() => {
      return selectedFile.value?.filename?.toLowerCase().endsWith('.csv') || 
             selectedFile.value?.file_type === 'csv'
    })
    
    const canProceed = computed(() => {
      switch (currentStep.value) {
        case 0:
          if (selectionMode.value === 'multiple') {
            return selectedFiles.value.length > 0
          } else {
            return selectedFile.value !== null
          }
        case 1:
          // For CSV files, require symbol input
          if (isCSVFile.value) {
            return metadata.value !== null && csvSymbol.value.trim().length > 0
          }
          return metadata.value !== null
        case 2:
          return true
        default:
          return false
      }
    })
    
    const canStartImport = computed(() => {
      const hasJobName = importConfig.value.job_name.trim().length > 0
      // For CSV files, also require symbol
      if (isCSVFile.value) {
        return hasJobName && csvSymbol.value.trim().length > 0
      }
      return hasJobName
    })

    // Methods
    const loadAvailableFiles = async () => {
      try {
        loadingFiles.value = true
        const response = await api.get('/api/data-import/files')
        // The API returns { success: true, data: [...] }
        const files = response.data?.data || []
        // Sort files alphabetically by filename
        availableFiles.value = files.sort((a, b) => {
          const filenameA = (a.filename || '').toLowerCase()
          const filenameB = (b.filename || '').toLowerCase()
          return filenameA.localeCompare(filenameB)
        })
      } catch (error) {
        console.error('Error loading files:', error)
        showError('Failed to load available files', 'File Error')
      } finally {
        loadingFiles.value = false
      }
    }

    const selectFile = (file) => {
      selectedFile.value = file
    }

    const loadMetadata = async () => {
      if (!selectedFile.value) return
      
      try {
        loadingMetadata.value = true
        
        // Use metadata from the selected file (already loaded from the files API)
        if (selectedFile.value.metadata) {
          metadata.value = selectedFile.value.metadata
        } else {
          // Fallback: make separate API call for metadata
          const response = await api.get(`/api/data-import/metadata/${selectedFile.value.filename}`)
          metadata.value = response.data?.data || response.data
        }
        
        // Set default date range from metadata
        if (metadata.value) {
          // Handle different date field names
          const startDate = metadata.value.start_date || metadata.value.overall_date_range?.start_date
          const endDate = metadata.value.end_date || metadata.value.overall_date_range?.end_date
          
          importConfig.value.start_date = startDate
          importConfig.value.end_date = endDate
          importConfig.value.job_name = `Import ${metadata.value.dataset} - ${new Date().toLocaleDateString()}`
        }
      } catch (error) {
        console.error('Error loading metadata:', error)
        showError('Failed to load file metadata', 'Metadata Error')
      } finally {
        loadingMetadata.value = false
      }
    }

    const nextStep = async () => {
      if (currentStep.value === 0 && selectedFile.value) {
        // Show loading state while processing metadata
        loadingMetadata.value = true
        try {
          await loadMetadata()
          currentStep.value++
        } catch (error) {
          // Error handling is already in loadMetadata, but ensure loading state is cleared
          console.error('Error in nextStep:', error)
        } finally {
          loadingMetadata.value = false
        }
      } else {
        currentStep.value++
      }
    }

    const previousStep = () => {
      currentStep.value--
    }

    const startImport = async () => {
      try {
        importing.value = true
        importStatus.value = {
          status: 'Starting import...',
          progress: 0,
          current_symbol: '',
          processed_records: 0,
          symbols_processed: 0,
          started_at: new Date(),
          completed: false,
          error: null
        }
        
        // Handle multiple files or single file
        const filesToImport = selectionMode.value === 'multiple' ? selectedFiles.value : [selectedFile.value]
        
        if (filesToImport.length === 1) {
          // Single file import - use existing logic
          const file = filesToImport[0]
          const importRequest = {
            filename: file.filename,
            job_name: importConfig.value.job_name,
            overwrite_existing: importConfig.value.overwrite_existing
          }
          
          // Add CSV-specific fields if it's a CSV file
          if (file?.filename?.toLowerCase().endsWith('.csv') || file?.file_type === 'csv') {
            importRequest.file_type = 'csv'
            importRequest.csv_symbol = csvSymbol.value.trim()
            importRequest.csv_format = file.csv_format || 'tradestation'
            importRequest.timestamp_convention = timestampConvention.value
          }
          
          const response = await api.post('/api/data-import/jobs', importRequest)
          
          if (response.data.success) {
            importJobId.value = response.data.data?.job_id || response.data.data
            currentStep.value = 3
            startProgressMonitoring()
          } else {
            throw new Error(response.data.error || 'Failed to start import')
          }
        } else {
          // Multiple files - use queue system
          const queueRequests = filesToImport.map((file, index) => {
            const request = {
              filename: file.filename,
              job_name: `${importConfig.value.job_name} - ${file.filename}`,
              overwrite_existing: importConfig.value.overwrite_existing,
              priority: index + 1 // Higher priority for files selected first
            }
            
            // Add CSV-specific fields if it's a CSV file
            if (file?.filename?.toLowerCase().endsWith('.csv') || file?.file_type === 'csv') {
              request.file_type = 'csv'
              request.csv_symbol = csvSymbol.value.trim()
              request.csv_format = file.csv_format || 'tradestation'
              request.timestamp_convention = timestampConvention.value
            }
            
            return request
          })
          
          // Submit all files to the queue
          const response = await api.post('/api/data-import/queue/batch', {
            jobs: queueRequests
          })
          
          if (response.data.success) {
            // For multiple files, we'll monitor the queue status instead of individual jobs
            importJobId.value = 'QUEUE_BATCH'
            currentStep.value = 3
            startQueueMonitoring()
            showSuccess(`${filesToImport.length} files added to import queue`, 'Queue Updated')
          } else {
            throw new Error(response.data.error || 'Failed to add files to queue')
          }
        }
      } catch (error) {
        console.error('Error starting import:', error)
        console.error('Error response:', error.response?.data)
        showError('Failed to start import: ' + error.message, 'Import Error')
        importing.value = false
      }
    }

    const startProgressMonitoring = () => {
      progressInterval = setInterval(async () => {
        if (!importJobId.value) {
          return
        }
        
        try {
          const response = await api.get(`/api/data-import/jobs/${importJobId.value}/status`)
          const status = response.data
          
          // The status is nested in status.data, not directly in status
          const jobData = status.data || status
          
          // Calculate progress percentage using day-based or record-based progress
          let progressPercentage = 0
          if (jobData.progress?.progress_percentage !== undefined) {
            // Use day-based progress if available
            progressPercentage = jobData.progress.progress_percentage
          } else if (jobData.progress?.processed_records && jobData.progress?.total_records) {
            // Fallback to record-based progress
            progressPercentage = Math.round((jobData.progress.processed_records / jobData.progress.total_records) * 100)
          }
          
          // Determine status message
          let statusMessage = 'Processing...'
          if (jobData.status === 'running') {
            if (jobData.progress?.current_symbol) {
              statusMessage = `Processing ${jobData.progress.current_symbol}`
            } else if (jobData.progress?.processed_records) {
              statusMessage = `Processing records (${jobData.progress.processed_records.toLocaleString()} processed)`
            } else {
              statusMessage = 'Importing data...'
            }
          } else if (jobData.status === 'completed') {
            statusMessage = 'Import completed successfully!'
          } else if (jobData.status === 'failed') {
            statusMessage = 'Import failed'
          } else {
            statusMessage = jobData.status || 'Processing...'
          }
          
          const newStatus = {
            status: statusMessage,
            progress: jobData.status === 'completed' ? 100 : Math.min(100, Math.max(0, progressPercentage)),
            current_symbol: jobData.progress?.current_symbol || '',
            processed_records: jobData.progress?.processed_records || 0,
            total_records: jobData.progress?.total_records || 0,
            symbols_processed: jobData.symbols_processed?.length || 0,
            started_at: importStatus.value.started_at,
            completed: jobData.status === 'completed',
            failed: jobData.status === 'failed',
            error: jobData.error_message
          }
          
          importStatus.value = newStatus
          
          if (jobData.status === 'completed' || jobData.status === 'failed') {
            clearInterval(progressInterval)
            progressInterval = null
            importing.value = false
            
            if (jobData.status === 'completed') {
              showSuccess(`Data import completed successfully! Processed ${jobData.progress?.processed_records?.toLocaleString() || 0} records.`, 'Import Complete')
            } else {
              showError('Import failed: ' + (jobData.error_message || 'Unknown error'), 'Import Failed')
            }
          }
        } catch (error) {
          console.error('Error checking import status:', error)
          
          // If we get a 404, the job might not exist
          if (error.response?.status === 404) {
            clearInterval(progressInterval)
            progressInterval = null
            importing.value = false
            importStatus.value.error = 'Import job not found'
            importStatus.value.failed = true
            showError('Import job not found. The import may have failed to start.', 'Import Error')
          }
        }
      }, 1000) // Check every 1 second for more responsive updates
    }

    const startQueueMonitoring = () => {
      progressInterval = setInterval(async () => {
        try {
          const response = await api.get('/api/data-import/queue/status')
          const queueData = response.data.data || response.data
          
          // Calculate overall progress based on queue status
          const totalJobs = queueData.total_items || 0
          const completedJobs = queueData.completed_items || 0
          const failedJobs = queueData.failed_items || 0
          const queuedJobs = queueData.queued_items || 0
          const currentProcessing = queueData.current_processing
          
          let progressPercentage = 0
          if (totalJobs > 0) {
            progressPercentage = Math.round((completedJobs / totalJobs) * 100)
          }
          
          // Determine status message with better queue information
          let statusMessage = 'Processing queue...'
          let currentSymbol = ''
          
          if (currentProcessing) {
            // Show which file is currently being processed
            const currentIndex = completedJobs + 1 // Current file index (1-based)
            statusMessage = `Processing file ${currentIndex} of ${totalJobs}: ${currentProcessing.filename}`
            currentSymbol = `File ${currentIndex}/${totalJobs}`
          } else if (queuedJobs > 0) {
            statusMessage = `${queuedJobs} files queued for processing`
            currentSymbol = `${completedJobs}/${totalJobs} completed`
          } else if (completedJobs === totalJobs && totalJobs > 0) {
            statusMessage = 'All files processed successfully!'
            currentSymbol = `${totalJobs}/${totalJobs} completed`
          } else if (failedJobs > 0 && completedJobs + failedJobs === totalJobs) {
            statusMessage = `Processing complete (${failedJobs} failed)`
            currentSymbol = `${completedJobs}/${totalJobs} completed`
          }
          
          const newStatus = {
            status: statusMessage,
            progress: progressPercentage,
            current_symbol: currentSymbol,
            processed_records: queueData.total_processed_records || 0,
            symbols_processed: completedJobs,
            started_at: importStatus.value.started_at,
            completed: completedJobs === totalJobs && totalJobs > 0 && !currentProcessing,
            failed: failedJobs > 0 && completedJobs + failedJobs === totalJobs && !currentProcessing,
            error: failedJobs > 0 ? `${failedJobs} file${failedJobs > 1 ? 's' : ''} failed to import` : null,
            
            // Additional queue information
            queue_info: {
              total_files: totalJobs,
              completed_files: completedJobs,
              failed_files: failedJobs,
              queued_files: queuedJobs,
              current_file: currentProcessing ? currentProcessing.filename : null,
              current_file_index: currentProcessing ? completedJobs + 1 : null
            }
          }
          
          importStatus.value = newStatus
          
          // Stop monitoring when all jobs are complete
          if (newStatus.completed || newStatus.failed) {
            clearInterval(progressInterval)
            progressInterval = null
            importing.value = false
            
            if (newStatus.completed && failedJobs === 0) {
              showSuccess(`All ${totalJobs} files imported successfully! Processed ${queueData.total_processed_records?.toLocaleString() || 0} records.`, 'Queue Complete')
            } else if (failedJobs > 0) {
              showError(`Queue processing complete with ${failedJobs} failed imports out of ${totalJobs} total files.`, 'Queue Complete with Errors')
            }
          }
        } catch (error) {
          console.error('Error checking queue status:', error)
          
          // If we can't get queue status, stop monitoring
          clearInterval(progressInterval)
          progressInterval = null
          importing.value = false
          importStatus.value.error = 'Unable to monitor queue status'
          importStatus.value.failed = true
          showError('Unable to monitor import queue status.', 'Queue Error')
        }
      }, 2000) // Check every 2 seconds for queue monitoring
    }

    const cancelImport = async () => {
      if (!importJobId.value) return
      
      try {
        await api.delete(`/api/data-import/jobs/${importJobId.value}`)
        clearInterval(progressInterval)
        importing.value = false
        showSuccess('Import cancelled successfully', 'Import Cancelled')
        closeDialog()
      } catch (error) {
        console.error('Error cancelling import:', error)
        showError('Failed to cancel import', 'Cancel Error')
      }
    }

    const finishImport = () => {
      emit('imported')
      closeDialog()
    }

    const closeDialog = () => {
      if (importing.value && !importStatus.value.completed && !importStatus.value.failed) {
        return // Prevent closing during active import
      }
      
      // Reset state
      currentStep.value = 0
      selectedFile.value = null
      csvSymbol.value = ''
      timestampConvention.value = 'begin_of_minute' // Reset to default
      metadata.value = null
      importConfig.value = {
        start_date: '',
        end_date: '',
        symbol_mode: 'all',
        symbol_list: '',
        job_name: '',
        overwrite_existing: false
      }
      importing.value = false
      importJobId.value = null
      
      if (progressInterval) {
        clearInterval(progressInterval)
        progressInterval = null
      }
      
      emit('update:visible', false)
    }

    const closeInBackground = () => {
      // Allow closing while import continues in background
      showSuccess('Import continues in background. You can check progress later.', 'Import Running')
      
      // Don't reset import state, just close the dialog
      currentStep.value = 0
      selectedFile.value = null
      metadata.value = null
      importConfig.value = {
        start_date: '',
        end_date: '',
        symbol_mode: 'all',
        symbol_list: '',
        job_name: '',
        overwrite_existing: false
      }
      
      // Keep the progress monitoring running
      // Don't clear progressInterval or reset importing/importJobId
      
      emit('update:visible', false)
    }

    // Helper functions
    const formatFileSize = (bytes) => {
      if (!bytes) return '0 B'
      const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
      const i = Math.floor(Math.log(bytes) / Math.log(1024))
      return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`
    }

    const formatFileDate = (dateString) => {
      if (!dateString) return 'N/A'
      return new Date(dateString).toLocaleString()
    }

    const formatDateRange = (startDate, endDate) => {
      if (!startDate || !endDate) return 'N/A'
      
      try {
        // Simple date formatting without timezone conversion
        const formatDate = (dateStr) => {
          if (typeof dateStr === 'string' && dateStr.match(/^\d{4}-\d{2}-\d{2}$/)) {
            // For YYYY-MM-DD format, just reformat without parsing through Date object
            const [year, month, day] = dateStr.split('-')
            return new Date(parseInt(year), parseInt(month) - 1, parseInt(day)).toLocaleDateString()
          }
          // For other formats, try direct parsing
          return new Date(dateStr).toLocaleDateString()
        }
        
        const start = formatDate(startDate)
        const end = formatDate(endDate)
        return `${start} - ${end}`
      } catch (error) {
        console.error('Error formatting date range:', error, { startDate, endDate })
        return `${startDate} - ${endDate}`
      }
    }

    const formatNumber = (number) => {
      if (number === null || number === undefined) return '0'
      return number.toLocaleString()
    }

    const formatDuration = (startTime) => {
      if (!startTime) return '0s'
      const now = new Date()
      const start = new Date(startTime)
      const diffMs = now - start
      const diffSecs = Math.floor(diffMs / 1000)
      const diffMins = Math.floor(diffSecs / 60)
      
      if (diffMins > 0) {
        return `${diffMins}m ${diffSecs % 60}s`
      } else {
        return `${diffSecs}s`
      }
    }

    // Watch for dialog visibility
    watch(() => props.visible, (newVisible) => {
      if (newVisible) {
        loadAvailableFiles()
      }
    })

    // Multi-file selection methods
    const isFileSelected = (file) => {
      if (selectionMode.value === 'multiple') {
        return selectedFiles.value.some(f => f.filename === file.filename)
      } else {
        return selectedFile.value && selectedFile.value.filename === file.filename
      }
    }

    const toggleFileSelection = (file) => {
      if (selectionMode.value === 'multiple') {
        const index = selectedFiles.value.findIndex(f => f.filename === file.filename)
        if (index >= 0) {
          selectedFiles.value.splice(index, 1)
        } else {
          selectedFiles.value.push(file)
        }
        // Set the first selected file as the primary file for metadata
        if (selectedFiles.value.length > 0) {
          selectedFile.value = selectedFiles.value[0]
        } else {
          selectedFile.value = null
        }
      } else {
        selectedFile.value = file
        selectedFiles.value = [file]
      }
    }

    const selectAllFiles = () => {
      selectedFiles.value = [...availableFiles.value]
      if (selectedFiles.value.length > 0) {
        selectedFile.value = selectedFiles.value[0]
      }
    }

    const clearSelection = () => {
      selectedFiles.value = []
      selectedFile.value = null
    }

    // Watch for selection mode changes
    watch(selectionMode, (newMode) => {
      if (newMode === 'single') {
        // Keep only the first selected file
        if (selectedFiles.value.length > 0) {
          selectedFile.value = selectedFiles.value[0]
          selectedFiles.value = [selectedFiles.value[0]]
        }
      } else {
        // Multiple mode - ensure selectedFiles includes selectedFile
        if (selectedFile.value && !selectedFiles.value.some(f => f.filename === selectedFile.value.filename)) {
          selectedFiles.value = [selectedFile.value]
        }
      }
    })

    return {
      // State
      currentStep,
      steps,
      availableFiles,
      selectedFile,
      selectedFiles,
      selectionMode,
      loadingFiles,
      csvSymbol,
      timestampConvention,
      metadata,
      loadingMetadata,
      importConfig,
      importing,
      importJobId,
      importStatus,
      
      // Computed
      isCSVFile,
      canProceed,
      canStartImport,
      
      // Methods
      selectFile,
      nextStep,
      previousStep,
      startImport,
      cancelImport,
      finishImport,
      closeDialog,
      closeInBackground,
      
      // Multi-file selection methods
      isFileSelected,
      toggleFileSelection,
      selectAllFiles,
      clearSelection,
      
      // Helpers
      formatFileSize,
      formatFileDate,
      formatDateRange,
      formatNumber,
      formatDuration
    }
  }
}
</script>

<style scoped>
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

/* Steps Progress */
.steps-progress {
  display: flex;
  justify-content: space-between;
  padding: var(--spacing-lg);
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-primary);
}

.step-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  position: relative;
}

.step-item:not(:last-child)::after {
  content: '';
  position: absolute;
  top: 16px;
  left: 60%;
  right: -40%;
  height: 2px;
  background: var(--border-secondary);
  z-index: 1;
}

.step-item.completed:not(:last-child)::after {
  background: var(--color-success);
}

.step-number {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-sm);
  margin-bottom: var(--spacing-xs);
  position: relative;
  z-index: 2;
  background: var(--bg-secondary);
  border: 2px solid var(--border-secondary);
  color: var(--text-secondary);
}

.step-item.active .step-number {
  background: var(--color-brand);
  border-color: var(--color-brand);
  color: var(--text-primary);
}

.step-item.completed .step-number {
  background: var(--color-success);
  border-color: var(--color-success);
  color: var(--text-primary);
}

.step-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  text-align: center;
}

.step-item.active .step-label {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.dialog-body {
  padding: var(--spacing-lg);
  min-height: 400px;
}

.step-content h3 {
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
}

.step-description {
  margin: 0 0 var(--spacing-xl) 0;
  color: var(--text-secondary);
}

/* Loading States */
.loading-files,
.loading-metadata {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-xl);
  justify-content: center;
  color: var(--text-secondary);
}

.spinner-small {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-brand);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* No Files State */
.no-files {
  text-align: center;
  padding: var(--spacing-xl);
}

.no-files-icon {
  font-size: 3rem;
  margin-bottom: var(--spacing-md);
}

.no-files h4 {
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--text-primary);
}

.no-files p {
  margin: 0;
  color: var(--text-secondary);
}

/* Selection Mode Toggle */
.selection-mode-toggle {
  display: flex;
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  padding: 2px;
  border: 1px solid var(--border-primary);
  margin-bottom: var(--spacing-lg);
  width: fit-content;
}

.mode-btn {
  background: none;
  border: none;
  padding: var(--spacing-sm) var(--spacing-lg);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition-fast);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
}

.mode-btn:hover {
  color: var(--text-primary);
  background: var(--bg-quaternary);
}

.mode-btn.active {
  background: var(--color-brand);
  color: var(--text-primary);
  box-shadow: 0 2px 4px rgba(255, 107, 53, 0.2);
}

/* Multi-select Controls */
.multi-select-controls {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-md);
}

.selection-count {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-left: auto;
}

.btn-sm {
  padding: var(--spacing-xs) var(--spacing-md);
  font-size: var(--font-size-sm);
}

/* File Selector */
.file-selector {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.file-list {
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-primary);
}

.file-list::-webkit-scrollbar {
  width: 8px;
}

.file-list::-webkit-scrollbar-track {
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
}

.file-list::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.file-list::-webkit-scrollbar-thumb:hover {
  background: var(--text-tertiary);
}

.file-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  cursor: pointer;
  transition: var(--transition-normal);
}

.file-option:last-child {
  border-bottom: none;
}

.file-option:hover {
  border-color: var(--color-brand);
  background: var(--bg-tertiary);
}

.file-option.selected {
  border-color: var(--color-brand);
  background: rgba(255, 107, 53, 0.1);
}

.file-info {
  flex: 1;
}

.file-name {
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.file-details {
  display: flex;
  gap: var(--spacing-lg);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  align-items: center;
}

.file-type-badge {
  padding: 2px var(--spacing-xs);
  border-radius: var(--radius-xs);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
  font-size: var(--font-size-xs);
}

.file-type-badge.csv {
  background: var(--color-info);
  color: var(--text-primary);
}

.file-type-badge.dbn {
  background: var(--color-brand);
  color: var(--text-primary);
}

.file-status {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
}

.selected-indicator {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--color-success);
  color: var(--text-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: var(--font-weight-bold);
}

.checkbox-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
}

.checkbox-indicator input[type="checkbox"] {
  width: 18px;
  height: 18px;
  cursor: pointer;
}

.file-option.multi-select {
  padding-left: var(--spacing-md);
}

/* CSV Symbol Input */
.csv-symbol-input {
  margin-bottom: var(--spacing-xl);
  padding: var(--spacing-lg);
  background: rgba(255, 107, 53, 0.05);
  border: 1px solid rgba(255, 107, 53, 0.2);
  border-radius: var(--radius-md);
}

.timestamp-convention-selector {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  margin: var(--spacing-sm) 0;
}

.timestamp-convention-selector .radio-group {
  padding: var(--spacing-sm);
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
}

.timestamp-convention-selector .radio-group:hover {
  background: rgba(255, 107, 53, 0.05);
}

/* Metadata Preview */
.metadata-preview {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xl);
}

.metadata-section h4 {
  margin: 0 0 var(--spacing-md) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.metadata-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-md);
}

.metadata-item {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.metadata-label {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-medium);
}

.metadata-value {
  font-size: var(--font-size-md);
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.asset-types {
  display: flex;
  gap: var(--spacing-xs);
  flex-wrap: wrap;
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

/* Multi-file Summary */
.multi-file-summary {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.summary-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: var(--spacing-md);
  border-bottom: 1px solid var(--border-primary);
}

.summary-header h4 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.file-count {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
}

.files-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  max-height: 200px;
  overflow-y: auto;
  padding: var(--spacing-sm);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.files-list::-webkit-scrollbar {
  width: 6px;
}

.files-list::-webkit-scrollbar-track {
  background: var(--bg-quaternary);
  border-radius: var(--radius-sm);
}

.files-list::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.file-summary-item {
  display: flex;
  align-items: center;
  padding: var(--spacing-sm);
  background: var(--bg-primary);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-primary);
}

.file-summary-info {
  flex: 1;
}

.file-summary-name {
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}

.file-summary-details {
  display: flex;
  gap: var(--spacing-md);
  align-items: center;
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
}

/* Import Notice */
.import-notice {
  display: flex;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.3);
  border-radius: var(--radius-md);
  align-items: flex-start;
}

.notice-icon {
  font-size: var(--font-size-xl);
  flex-shrink: 0;
  margin-top: 2px;
}

.notice-content {
  flex: 1;
}

.notice-content h4 {
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.notice-content p {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  line-height: 1.5;
}

/* Date Range Selector */
.date-range-selector {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-lg);
}

/* Symbol Selection */
.symbol-selection {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.selection-mode {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.radio-group {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

.radio-group input[type="radio"] {
  width: 16px;
  height: 16px;
}

.symbol-subset {
  margin-top: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.symbol-input label {
  display: block;
  margin-bottom: var(--spacing-xs);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

/* Form Elements */
.config-form {
  display: flex;
  flex-direction: column;
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

.form-input,
.form-textarea {
  padding: var(--spacing-sm);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.form-input:focus,
.form-textarea:focus {
  outline: none;
  border-color: var(--color-brand);
  box-shadow: 0 0 0 2px rgba(255, 107, 53, 0.1);
}

.form-textarea {
  resize: vertical;
  min-height: 80px;
}

.checkbox-group {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  cursor: pointer;
}

.checkbox-group input[type="checkbox"] {
  width: 16px;
  height: 16px;
}

.checkbox-group span {
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

.form-help {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  margin: var(--spacing-xs) 0 0 0;
  font-style: italic;
}

/* Import Summary */
.import-summary {
  padding: var(--spacing-lg);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-primary);
}

.import-summary h4 {
  margin: 0 0 var(--spacing-md) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-md);
}

.summary-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm);
  background: var(--bg-primary);
  border-radius: var(--radius-sm);
}

.summary-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.summary-value {
  font-size: var(--font-size-sm);
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
  text-align: right;
}

/* Import Progress */
.import-progress {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.progress-info {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.progress-status {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status-text {
  font-size: var(--font-size-md);
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.progress-percentage {
  font-size: var(--font-size-lg);
  color: var(--color-brand);
  font-weight: var(--font-weight-semibold);
}

.progress-bar {
  width: 100%;
  height: 8px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: var(--color-brand);
  transition: width 0.3s ease;
  border-radius: var(--radius-sm);
}

.current-activity {
  display: flex;
  gap: var(--spacing-sm);
  align-items: center;
  padding: var(--spacing-sm);
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
}

.activity-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.activity-value {
  font-size: var(--font-size-sm);
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
  font-family: monospace;
}

.progress-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--spacing-md);
}

.stat-item {
  text-align: center;
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.stat-number {
  display: block;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--color-brand);
}

.stat-label {
  display: block;
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
  text-transform: uppercase;
  margin-top: var(--spacing-xs);
}

.import-error {
  padding: var(--spacing-lg);
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-md);
}

.import-error h4 {
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--color-danger);
  font-size: var(--font-size-md);
}

.import-error p {
  margin: 0;
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.import-complete {
  text-align: center;
  padding: var(--spacing-xl);
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid var(--color-success);
  border-radius: var(--radius-md);
}

.success-icon {
  font-size: 3rem;
  margin-bottom: var(--spacing-md);
}

.import-complete h4 {
  margin: 0 0 var(--spacing-sm) 0;
  color: var(--color-success);
  font-size: var(--font-size-lg);
}

.import-complete p {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

/* Dialog Footer */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
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

/* Responsive Design */
@media (max-width: 768px) {
  .dialog-content {
    width: 95vw;
    margin: var(--spacing-md);
  }
  
  .date-range-selector {
    grid-template-columns: 1fr;
  }
  
  .summary-grid {
    grid-template-columns: 1fr;
  }
  
  .progress-stats {
    grid-template-columns: 1fr;
  }
  
  .steps-progress {
    flex-wrap: wrap;
    gap: var(--spacing-sm);
  }
  
  .step-item {
    flex: 0 0 auto;
    min-width: 80px;
  }
  
  .step-item:not(:last-child)::after {
    display: none;
  }
}
</style>
