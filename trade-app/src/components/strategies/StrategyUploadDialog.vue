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
              <i class="pi pi-shield" :class="getValidationIconClass()"></i>
              Validation Results
            </h3>
          </div>

          <div class="validation-content">
            <!-- Success State -->
            <div v-if="validationResults.success" class="validation-success">
              <div class="success-summary">
                <i class="pi pi-check-circle"></i>
                <span>Strategy validation passed successfully!</span>
              </div>
              
              <!-- Validation Steps -->
              <div v-if="validationResults.validation_details?.validation_steps" class="validation-steps">
                <h4>Validation Steps:</h4>
                <div class="steps-list">
                  <div 
                    v-for="step in validationResults.validation_details.validation_steps" 
                    :key="step.name"
                    class="validation-step"
                    :class="`step-${step.status}`"
                  >
                    <i :class="getStepIcon(step.status)"></i>
                    <span class="step-name">{{ formatStepName(step.name) }}</span>
                    <span class="step-message">{{ step.message }}</span>
                  </div>
                </div>
              </div>

              <!-- Warnings -->
              <div v-if="validationResults.validation_details?.warnings?.length" class="validation-warnings">
                <h4>⚠️ Warnings:</h4>
                <div class="warnings-list">
                  <div v-for="warning in validationResults.validation_details.warnings" :key="warning" class="warning-item">
                    <i class="pi pi-exclamation-triangle"></i>
                    <span>{{ warning }}</span>
                  </div>
                </div>
              </div>

              <!-- Suggestions -->
              <div v-if="validationResults.validation_details?.suggestions?.length" class="validation-suggestions">
                <h4>💡 Suggestions:</h4>
                <div class="suggestions-list">
                  <div v-for="suggestion in validationResults.validation_details.suggestions" :key="suggestion" class="suggestion-item">
                    <i class="pi pi-lightbulb"></i>
                    <span>{{ suggestion }}</span>
                  </div>
                </div>
              </div>

              <!-- File Info -->
              <div v-if="validationResults.validation_details?.file_info" class="file-info">
                <h4>📄 File Information:</h4>
                <div class="info-grid">
                  <div class="info-item">
                    <span class="info-label">Size:</span>
                    <span class="info-value">{{ formatFileSize(validationResults.validation_details.file_info.size_bytes) }}</span>
                  </div>
                  <div class="info-item">
                    <span class="info-label">Lines:</span>
                    <span class="info-value">{{ validationResults.validation_details.file_info.lines }}</span>
                  </div>
                  <div class="info-item">
                    <span class="info-label">Classes:</span>
                    <span class="info-value">{{ validationResults.validation_details.file_info.classes }}</span>
                  </div>
                  <div class="info-item">
                    <span class="info-label">Functions:</span>
                    <span class="info-value">{{ validationResults.validation_details.file_info.functions }}</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Error State -->
            <div v-else class="validation-errors">
              <div class="error-summary">
                <i class="pi pi-times-circle"></i>
                <span>{{ validationResults.message || 'Strategy validation failed' }}</span>
              </div>

              <!-- Validation Steps (show progress even on failure) -->
              <div v-if="validationResults.validation_details?.validation_steps" class="validation-steps">
                <h4>Validation Progress:</h4>
                <div class="steps-list">
                  <div 
                    v-for="step in validationResults.validation_details.validation_steps" 
                    :key="step.name"
                    class="validation-step"
                    :class="`step-${step.status}`"
                  >
                    <i :class="getStepIcon(step.status)"></i>
                    <span class="step-name">{{ formatStepName(step.name) }}</span>
                    <span class="step-message">{{ step.message }}</span>
                  </div>
                </div>
              </div>

              <!-- Specific Errors -->
              <div v-if="validationResults.validation_details?.errors?.length" class="error-details">
                <h4>❌ Errors:</h4>
                <div class="errors-list">
                  <div v-for="error in validationResults.validation_details.errors" :key="error" class="error-item">
                    <i class="pi pi-times-circle"></i>
                    <span>{{ error }}</span>
                  </div>
                </div>
              </div>

              <!-- Help Section -->
              <div class="validation-help">
                <h4>🔧 How to Fix:</h4>
                <div class="help-content">
                  <p>Your strategy file needs to meet these requirements:</p>
                  <ul>
                    <li>Must be a valid Python (.py) file</li>
                    <li>Must contain a class that inherits from BaseStrategy</li>
                    <li>Must implement required methods: initialize_strategy() and get_strategy_metadata()</li>
                    <li>Must include proper imports from the framework</li>
                    <li>Should not contain security risks or malicious code</li>
                  </ul>
                  <p>
                    <strong>Example:</strong> 
                    <a href="#" @click.prevent="showExampleStrategy">View example strategy structure</a>
                  </p>
                </div>
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
          // Handle validation errors - properly structure the validation results
          console.log('Validation failed, result:', result)
          validationResults.value = {
            success: false,
            message: result.message || result.error || 'Upload failed',
            validation_details: result.validation_details || {}
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

    // Validation UI helper methods
    const getValidationIconClass = () => {
      if (!validationResults.value) return ''
      return validationResults.value.success ? 'success' : 'danger'
    }

    const getStepIcon = (status) => {
      switch (status) {
        case 'passed': return 'pi pi-check-circle'
        case 'failed': return 'pi pi-times-circle'
        case 'warning': return 'pi pi-exclamation-triangle'
        case 'skipped': return 'pi pi-minus-circle'
        default: return 'pi pi-circle'
      }
    }

    const formatStepName = (stepName) => {
      return stepName
        .replace(/_/g, ' ')
        .replace(/\b\w/g, l => l.toUpperCase())
    }

    const showExampleStrategy = () => {
      // Show example strategy structure in a new dialog or modal
      showSuccess(
        'Example strategy structure will be shown in documentation',
        'Coming Soon'
      )
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
      formatFileSize,
      
      // Validation UI methods
      getValidationIconClass,
      getStepIcon,
      formatStepName,
      showExampleStrategy
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

/* Enhanced Validation Styles */
.success-summary, .error-summary {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-md);
  border-radius: var(--radius-md);
}

.success-summary {
  color: var(--color-success);
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.error-summary {
  color: var(--color-danger);
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.success-summary i, .error-summary i {
  font-size: var(--font-size-lg);
}

/* Validation Steps */
.validation-steps {
  margin-bottom: var(--spacing-lg);
}

.validation-steps h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.steps-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.validation-step {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  transition: var(--transition-normal);
}

.validation-step i {
  font-size: var(--font-size-md);
  flex-shrink: 0;
}

.step-name {
  font-weight: var(--font-weight-semibold);
  min-width: 120px;
}

.step-message {
  color: var(--text-secondary);
  flex: 1;
}

.step-passed {
  background: rgba(34, 197, 94, 0.05);
  border-left: 3px solid var(--color-success);
}

.step-passed i {
  color: var(--color-success);
}

.step-failed {
  background: rgba(239, 68, 68, 0.05);
  border-left: 3px solid var(--color-danger);
}

.step-failed i {
  color: var(--color-danger);
}

.step-warning {
  background: rgba(245, 158, 11, 0.05);
  border-left: 3px solid var(--color-warning);
}

.step-warning i {
  color: var(--color-warning);
}

.step-skipped {
  background: rgba(107, 114, 128, 0.05);
  border-left: 3px solid var(--text-tertiary);
}

.step-skipped i {
  color: var(--text-tertiary);
}

/* Warnings, Suggestions, and Errors */
.validation-warnings, .validation-suggestions, .error-details {
  margin-bottom: var(--spacing-lg);
}

.validation-warnings h4, .validation-suggestions h4, .error-details h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.warnings-list, .suggestions-list, .errors-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.warning-item, .suggestion-item, .error-item {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  line-height: 1.4;
}

.warning-item {
  background: rgba(245, 158, 11, 0.05);
  border-left: 3px solid var(--color-warning);
  color: var(--color-warning);
}

.suggestion-item {
  background: rgba(59, 130, 246, 0.05);
  border-left: 3px solid var(--color-info);
  color: var(--color-info);
}

.error-item {
  background: rgba(239, 68, 68, 0.05);
  border-left: 3px solid var(--color-danger);
  color: var(--color-danger);
}

.warning-item i, .suggestion-item i, .error-item i {
  font-size: var(--font-size-md);
  margin-top: 2px;
  flex-shrink: 0;
}

/* File Info */
.file-info {
  margin-bottom: var(--spacing-lg);
}

.file-info h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-md);
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.info-label {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.info-value {
  color: var(--text-primary);
  font-weight: var(--font-weight-semibold);
}

/* Help Section */
.validation-help {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-secondary);
}

.validation-help h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.help-content p {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  line-height: 1.5;
  margin: 0 0 var(--spacing-md) 0;
}

.help-content ul {
  list-style: none;
  padding: 0;
  margin: 0 0 var(--spacing-md) 0;
}

.help-content li {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) 0;
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  line-height: 1.4;
}

.help-content li::before {
  content: '•';
  color: var(--color-brand);
  font-weight: var(--font-weight-bold);
  font-size: var(--font-size-md);
  margin-top: 2px;
}

.help-content a {
  color: var(--color-brand);
  text-decoration: none;
  font-weight: var(--font-weight-medium);
}

.help-content a:hover {
  text-decoration: underline;
}

/* Legacy validation styles for backward compatibility */
.validation-success {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.validation-errors {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
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
