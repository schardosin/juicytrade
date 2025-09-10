<template>
  <div class="import-progress-overlay" :class="{ 'compact': compact }">
    <div class="overlay-background"></div>
    <div class="progress-content">
      <!-- Circular Progress Indicator -->
      <div class="progress-circle" :class="{ 'small': compact }">
        <svg class="progress-ring" :width="circleSize" :height="circleSize">
          <circle
            class="progress-ring-background"
            :cx="circleSize / 2"
            :cy="circleSize / 2"
            :r="radius"
            fill="transparent"
            :stroke-width="strokeWidth"
          />
          <circle
            class="progress-ring-progress"
            :cx="circleSize / 2"
            :cy="circleSize / 2"
            :r="radius"
            fill="transparent"
            :stroke-width="strokeWidth"
            :stroke-dasharray="circumference"
            :stroke-dashoffset="strokeDashoffset"
          />
        </svg>
        <div class="progress-text">
          <span class="percentage">{{ Math.round(progress) }}%</span>
        </div>
        <!-- Pulsing Animation Indicator - positioned relative to progress circle -->
        <div class="pulse-indicator" :class="{ 'small': compact }"></div>
      </div>
      
      <!-- Status Message -->
      <div v-if="!compact" class="status-message">
        <div class="status-text">{{ statusMessage }}</div>
        <div v-if="showDetails" class="status-details">
          <span v-if="processedRecords > 0">
            {{ formatNumber(processedRecords) }} records processed
          </span>
          <span v-if="filename" class="filename">{{ filename }}</span>
        </div>
      </div>
      
      <!-- Compact Status (for list view) -->
      <div v-if="compact" class="compact-status">
        <span class="compact-text">{{ compactStatusText }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from 'vue'

export default {
  name: 'ImportProgressOverlay',
  props: {
    progress: {
      type: Number,
      default: 0,
      validator: (value) => value >= 0 && value <= 100
    },
    statusMessage: {
      type: String,
      default: 'Importing...'
    },
    filename: {
      type: String,
      default: ''
    },
    processedRecords: {
      type: Number,
      default: 0
    },
    compact: {
      type: Boolean,
      default: false
    },
    showDetails: {
      type: Boolean,
      default: true
    }
  },
  setup(props) {
    // Circle dimensions based on compact mode
    const circleSize = computed(() => props.compact ? 32 : 80)
    const strokeWidth = computed(() => props.compact ? 3 : 4)
    const radius = computed(() => (circleSize.value - strokeWidth.value) / 2)
    const circumference = computed(() => 2 * Math.PI * radius.value)
    
    // Calculate stroke dash offset for progress
    const strokeDashoffset = computed(() => {
      const progressDecimal = Math.min(100, Math.max(0, props.progress)) / 100
      return circumference.value - (progressDecimal * circumference.value)
    })
    
    // Compact status text for list view
    const compactStatusText = computed(() => {
      if (props.progress > 0) {
        return `Importing... ${Math.round(props.progress)}%`
      }
      return 'Importing...'
    })
    
    // Format numbers with commas
    const formatNumber = (number) => {
      if (number === null || number === undefined) return '0'
      return number.toLocaleString()
    }
    
    return {
      circleSize,
      strokeWidth,
      radius,
      circumference,
      strokeDashoffset,
      compactStatusText,
      formatNumber
    }
  }
}
</script>

<style scoped>
.import-progress-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  border-radius: inherit;
  overflow: hidden;
}

.overlay-background {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(2px);
  border-radius: inherit;
}

.progress-content {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-md);
  z-index: 2;
  text-align: center;
}

/* Circular Progress */
.progress-circle {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.progress-ring {
  transform: rotate(-90deg);
}

.progress-ring-background {
  stroke: rgba(255, 255, 255, 0.2);
}

.progress-ring-progress {
  stroke: var(--color-brand);
  stroke-linecap: round;
  transition: stroke-dashoffset 0.3s ease;
}

.progress-text {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: var(--text-primary);
}

.percentage {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}

.progress-circle.small .percentage {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
}

/* Status Messages */
.status-message {
  color: var(--text-primary);
  max-width: 300px;
}

.status-text {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  margin-bottom: var(--spacing-xs);
  line-height: 1.4;
}

.status-details {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.filename {
  font-family: monospace;
  font-size: var(--font-size-xs);
  opacity: 0.8;
}

/* Compact Mode */
.import-progress-overlay.compact {
  flex-direction: row;
  justify-content: flex-start;
  padding: var(--spacing-sm);
}

.import-progress-overlay.compact .progress-content {
  flex-direction: row;
  gap: var(--spacing-sm);
  align-items: center;
}

.compact-status {
  color: var(--text-primary);
}

.compact-text {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  white-space: nowrap;
}

/* Pulsing Animation */
.pulse-indicator {
  position: absolute;
  top: 0;
  left: 0;
  width: 80px;
  height: 80px;
  border: 2px solid var(--color-brand);
  border-radius: 50%;
  opacity: 0.4;
  animation: pulse 2s infinite;
  pointer-events: none;
}

.pulse-indicator.small {
  width: 32px;
  height: 32px;
  border-width: 1px;
}

@keyframes pulse {
  0% {
    transform: scale(1);
    opacity: 0.4;
  }
  50% {
    transform: scale(1.2);
    opacity: 0.1;
  }
  100% {
    transform: scale(1);
    opacity: 0.4;
  }
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .status-message {
    max-width: 250px;
  }
  
  .status-text {
    font-size: var(--font-size-sm);
  }
  
  .status-details {
    font-size: var(--font-size-xs);
  }
}

/* Hover effects */
.import-progress-overlay:hover .overlay-background {
  background: rgba(0, 0, 0, 0.8);
}

.import-progress-overlay:hover .pulse-indicator {
  animation-duration: 1.5s;
}

/* Accessibility */
@media (prefers-reduced-motion: reduce) {
  .pulse-indicator {
    animation: none;
  }
  
  .progress-ring-progress {
    transition: none;
  }
}

/* High contrast mode */
@media (prefers-contrast: high) {
  .overlay-background {
    background: rgba(0, 0, 0, 0.9);
  }
  
  .progress-ring-background {
    stroke: rgba(255, 255, 255, 0.5);
  }
}
</style>
