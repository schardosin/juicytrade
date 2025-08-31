<template>
  <Dialog
    v-model:visible="dialogVisible"
    modal
    header="Upload Trading Strategy"
    :style="{ width: '600px' }"
    class="strategy-upload-dialog"
    @hide="resetForm"
  >
    <div class="upload-content">
      <!-- File Upload Section -->
      <div class="upload-section">
        <div class="upload-header">
          <h3>
            <i class="pi pi-upload"></i>
            Select Strategy File
          </h3>
          <p>Upload a Python (.py) file containing your trading strategy</p>
        </div>

        <!-- File Drop Zone -->
        <div
          class="file-drop-zone"
          :class="{ 
            'drag-over': isDragOver,
            'has-file': selectedFile,
            'error': uploadError
          }"
          @drop="handleDrop"
          @dragover="handleDragOver"
          @dragleave="handleDragLeave"
          @click="triggerFileInput"
        >
          <input
            ref="fileInput"
            type="file"
            accept=".py"
            @change="handleFileSelect"
            style="display: none"
          />

          <div v-if="!selectedFile" class="drop-zone-content">
            <i class="pi pi-cloud-upload drop-icon"></i>
            <h4>Drop your strategy file here</h4>
            <p>or click to browse files</p>
            <small>Accepts Python (.py) files up to 10MB</small>
          </div>

          <div v-else class="selected-file-content">
            <i class="pi pi-file file-icon"></i>
            <div class="file-info">
              <h4>{{ selectedFile.name }}</h4>
              <p>{{ formatFileSize(selectedFile.size) }}</p>
            </div>
            <Button
              icon="pi pi-times"
              class="p-button-text p-button-sm remove-file-btn"
              @click.stop="removeFile"
            />
          </div>
        </div>

        <!-- Upload Error -->
        <div v-if="uploadError" class="error-message">
          <i class="pi pi-exclamation-triangle"></i>
          {{ uploadError }}
        </div>
      </div>

      <!-- Strategy Details Section -->
      <div class="details-section">
        <div class="details-header">
          <h3>
            <i class="pi pi-info-circle"></i>
            Strategy Details
          </h3>
        </div>

        <div class="form-grid">
          <div class="form-field">
            <label for="strategy-name">Strategy Name *</label>
            <InputText
              id="strategy-name"
              v-model="strategyName"
              placeholder="Enter strategy name"
              :class="{ 'p-invalid': !strategyName && showValidation }"
            />
            <small v-if="!strategyName && showValidation" class="p-error">
              Strategy name is required
            </small>
          </div>

          <div class="form-field">
            <label for="strategy-description">Description</label>
            <textarea
              id="strategy-description"
              v-model="strategyDescription"
              placeholder="Describe what your strategy does..."
              rows="3"
              class="p-inputtextarea p-inputtext p-component"
              style="resize: vertical; min-height: 80px;"
            />
          </div>
        </div>
      </div>

      <!-- Validation Results -->
      <div v-if="validationResults" class="validation-section">
        <div class="validation-header">
          <h3>
            <i class="pi pi-shield" :class="validationResults.success ? 'success' : 'danger'"></i>
            Validation Results
          </h3>
        </div>

        <div class="validation-content">
          <div v-if="validationResults.success" class="validation-success">
            <i class="pi pi-check-circle"></i>
            <span>Strategy validation passed successfully</span>
          </div>

          <div v-else class="validation-errors">
            <div v-for="error in validationResults.details" :key="error" class="validation-error">
              <i class="pi pi-times-circle"></i>
              <span>{{ error }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <Button
          label="Cancel"
          class="p-button-text"
          @click="closeDialog"
          :disabled="isUploading"
        />
        <Button
          label="Upload Strategy"
          icon="pi pi-upload"
          class="p-button-primary"
          @click="uploadStrategy"
          :loading="isUploading"
          :disabled="!canUpload"
        />
      </div>
    </template>
  </Dialog>
</template>

<script>
import { ref, computed, watch } from 'vue'
import { useStrategyData } from '../../composables/useStrategyData.js'
import { useNotifications } from '../../composables/useNotifications.js'

export default {
  name: 'StrategyUploadDialog',
  props: {
    visible: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:visible', 'uploaded'],
  setup(props, { emit }) {
    const { uploadStrategy: uploadStrategyAction } = useStrategyData()
    const { showSuccess, showError } = useNotifications()

    // Dialog state
    const dialogVisible = computed({
      get: () => props.visible,
      set: (value) => emit('update:visible', value)
    })

    // Form state
    const fileInput = ref(null)
    const selectedFile = ref(null)
    const strategyName = ref('')
    const strategyDescription = ref('')
    const isDragOver = ref(false)
    const isUploading = ref(false)
    const uploadError = ref('')
    const validationResults = ref(null)
    const showValidation = ref(false)

    // Computed properties
    const canUpload = computed(() => {
      return selectedFile.value && 
             strategyName.value.trim() && 
             !isUploading.value &&
             (!validationResults.value || validationResults.value.success)
    })

    // File handling methods
    const triggerFileInput = () => {
      fileInput.value?.click()
    }

    const handleFileSelect = (event) => {
      const file = event.target.files[0]
      if (file) {
        validateAndSetFile(file)
      }
    }

    const handleDrop = (event) => {
      event.preventDefault()
      isDragOver.value = false
      
      const files = event.dataTransfer.files
      if (files.length > 0) {
        validateAndSetFile(files[0])
      }
    }

    const handleDragOver = (event) => {
      event.preventDefault()
      isDragOver.value = true
    }

    const handleDragLeave = (event) => {
      event.preventDefault()
      isDragOver.value = false
    }

    const validateAndSetFile = (file) => {
      uploadError.value = ''
      validationResults.value = null

      // File type validation
      if (!file.name.endsWith('.py')) {
        uploadError.value = 'Please select a Python (.py) file'
        return
      }

      // File size validation (10MB limit)
      const maxSize = 10 * 1024 * 1024 // 10MB
      if (file.size > maxSize) {
        uploadError.value = 'File size must be less than 10MB'
        return
      }

      selectedFile.value = file
      
      // Auto-populate strategy name from filename if empty
      if (!strategyName.value) {
        const baseName = file.name.replace('.py', '')
        strategyName.value = baseName
          .replace(/[_-]/g, ' ')
          .replace(/\b\w/g, l => l.toUpperCase())
      }
    }

    const removeFile = () => {
      selectedFile.value = null
      uploadError.value = ''
      validationResults.value = null
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }

    const uploadStrategy = async () => {
      showValidation.value = true

      if (!canUpload.value) {
        return
      }

      isUploading.value = true
      uploadError.value = ''

      try {
        const config = {
          name: strategyName.value.trim(),
          description: strategyDescription.value.trim()
        }

        const result = await uploadStrategyAction(selectedFile.value, config)

        if (result.success) {
          showSuccess(
            `Strategy "${config.name}" uploaded successfully`,
            'Upload Success'
          )

          emit('uploaded', result)
          closeDialog()
        } else {
          // Handle validation errors
          validationResults.value = {
            success: false,
            details: result.validation_details || [result.error || 'Upload failed']
          }
          
          uploadError.value = result.error || 'Upload failed'
        }
      } catch (error) {
        console.error('Strategy upload error:', error)
        uploadError.value = error.message || 'Upload failed due to network error'
        
        showError(
          'Failed to upload strategy',
          'Upload Error'
        )
      } finally {
        isUploading.value = false
      }
    }

    const closeDialog = () => {
      dialogVisible.value = false
    }

    const resetForm = () => {
      selectedFile.value = null
      strategyName.value = ''
      strategyDescription.value = ''
      uploadError.value = ''
      validationResults.value = null
      showValidation.value = false
      isDragOver.value = false
      isUploading.value = false
      
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }

    const formatFileSize = (bytes) => {
      if (bytes === 0) return '0 Bytes'
      
      const k = 1024
      const sizes = ['Bytes', 'KB', 'MB', 'GB']
      const i = Math.floor(Math.log(bytes) / Math.log(k))
      
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    }

    // Watch for dialog visibility changes
    watch(() => props.visible, (newValue) => {
      if (!newValue) {
        resetForm()
      }
    })

    return {
      // Refs
      fileInput,
      dialogVisible,
      selectedFile,
      strategyName,
      strategyDescription,
      isDragOver,
      isUploading,
      uploadError,
      validationResults,
      showValidation,

      // Computed
      canUpload,

      // Methods
      triggerFileInput,
      handleFileSelect,
      handleDrop,
      handleDragOver,
      handleDragLeave,
      removeFile,
      uploadStrategy,
      closeDialog,
      resetForm,
      formatFileSize
    }
  }
}
</script>

<style scoped>
.strategy-upload-dialog {
  font-family: var(--font-family);
}

.upload-content {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xl);
}

/* Upload Section */
.upload-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.upload-header h3 {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.upload-header h3 i {
  color: var(--color-brand);
}

.upload-header p {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

/* File Drop Zone */
.file-drop-zone {
  border: 2px dashed var(--border-secondary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-xl);
  text-align: center;
  cursor: pointer;
  transition: var(--transition-normal);
  background: var(--bg-secondary);
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.file-drop-zone:hover {
  border-color: var(--color-brand);
  background: var(--bg-tertiary);
}

.file-drop-zone.drag-over {
  border-color: var(--color-brand);
  background: rgba(var(--color-brand-rgb), 0.05);
}

.file-drop-zone.has-file {
  border-color: var(--color-success);
  background: rgba(34, 197, 94, 0.05);
}

.file-drop-zone.error {
  border-color: var(--color-danger);
  background: rgba(239, 68, 68, 0.05);
}

.drop-zone-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-sm);
}

.drop-icon {
  font-size: var(--font-size-3xl);
  color: var(--color-brand);
  margin-bottom: var(--spacing-sm);
}

.drop-zone-content h4 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0;
}

.drop-zone-content p {
  font-size: var(--font-size-md);
  color: var(--text-secondary);
  margin: 0;
}

.drop-zone-content small {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
}

.selected-file-content {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  width: 100%;
}

.file-icon {
  font-size: var(--font-size-2xl);
  color: var(--color-success);
}

.file-info {
  flex: 1;
  text-align: left;
}

.file-info h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 2px 0;
}

.file-info p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
}

.remove-file-btn {
  color: var(--color-danger) !important;
}

/* Error Message */
.error-message {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-md);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

/* Details Section */
.details-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.details-header h3 {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0;
}

.details-header h3 i {
  color: var(--color-brand);
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.form-field label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.form-field .p-error {
  color: var(--color-danger);
  font-size: var(--font-size-xs);
}

/* Validation Section */
.validation-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.validation-header h3 {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0;
}

.validation-header h3 i.success {
  color: var(--color-success);
}

.validation-header h3 i.danger {
  color: var(--color-danger);
}

.validation-content {
  padding: var(--spacing-md);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-secondary);
}

.validation-success {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--color-success);
  font-size: var(--font-size-md);
}

.validation-success i {
  font-size: var(--font-size-lg);
}

.validation-errors {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.validation-error {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
  line-height: 1.4;
}

.validation-error i {
  font-size: var(--font-size-md);
  margin-top: 2px;
  flex-shrink: 0;
}

/* Dialog Footer */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-sm);
}

/* Responsive Design */
@media (max-width: 768px) {
  .strategy-upload-dialog {
    width: 95vw !important;
    max-width: 500px !important;
  }

  .file-drop-zone {
    padding: var(--spacing-lg);
    min-height: 100px;
  }

  .drop-icon {
    font-size: var(--font-size-2xl);
  }

  .dialog-footer {
    flex-direction: column;
  }

  .dialog-footer .p-button {
    width: 100%;
  }
}
</style>
