<template>
  <div class="data-point-timeline-navigator">
    <div class="navigator-header">
      <h3>Decision Timeline Navigator</h3>
      <div class="timeline-stats">
        <span class="stat">{{ totalDataPoints }} data points</span>
        <span class="stat" v-if="selectedDataPoint">
          Point {{ currentIndex + 1 }} of {{ totalDataPoints }}
        </span>
      </div>
    </div>

    <div class="timeline-controls" v-if="decisionTimeline.length > 0">
      <div class="navigation-buttons">
        <button 
          @click="goToFirst" 
          :disabled="currentIndex === 0"
          class="nav-btn"
          title="First data point"
        >
          ⏮
        </button>
        <button 
          @click="goToPrevious" 
          :disabled="currentIndex === 0"
          class="nav-btn"
          title="Previous data point"
        >
          ⏪
        </button>
        <button 
          @click="goToNext" 
          :disabled="currentIndex >= decisionTimeline.length - 1"
          class="nav-btn"
          title="Next data point"
        >
          ⏩
        </button>
        <button 
          @click="goToLast" 
          :disabled="currentIndex >= decisionTimeline.length - 1"
          class="nav-btn"
          title="Last data point"
        >
          ⏭
        </button>
      </div>

      <div class="timeline-slider">
        <input
          type="range"
          :min="0"
          :max="Math.max(0, decisionTimeline.length - 1)"
          v-model.number="currentIndex"
          @input="onSliderChange"
          class="timeline-range"
        />
      </div>

      <div class="jump-controls">
        <label>Jump to:</label>
        <input
          type="number"
          :min="1"
          :max="decisionTimeline.length"
          :value="currentIndex + 1"
          @change="jumpToDataPoint"
          class="jump-input"
        />
        <button @click="jumpToSignals" class="jump-btn">Signals Only</button>
        <button @click="jumpToTrades" class="jump-btn">Trades Only</button>
      </div>
    </div>

    <div class="current-datapoint-info" v-if="selectedDataPoint">
      <div class="datapoint-header">
        <div class="timestamp">
          <strong>{{ formatTimestamp(selectedDataPoint.timestamp) }}</strong>
        </div>
        <div class="decision-type" :class="getDecisionTypeClass(selectedDataPoint)">
          {{ selectedDataPoint.rule_description }}
        </div>
        <div class="result-badge" :class="getResultBadgeClass(selectedDataPoint)">
          {{ getResultLabel(selectedDataPoint) }}
        </div>
        <div v-if="selectedDataPoint.context_values?.trade_executed" class="trade-indicator">
          🚀 TRADE EXECUTED
        </div>
      </div>

      <div class="quick-context">
        <div class="context-item" v-if="selectedDataPoint.context_values?.current_price">
          <span class="label">Price:</span>
          <span class="value">${{ selectedDataPoint.context_values.current_price.toFixed(2) }}</span>
        </div>
        <div class="context-item" v-if="selectedDataPoint.context_values?.fast_ma">
          <span class="label">Fast MA:</span>
          <span class="value">{{ selectedDataPoint.context_values.fast_ma.toFixed(3) }}</span>
        </div>
        <div class="context-item" v-if="selectedDataPoint.context_values?.slow_ma">
          <span class="label">Slow MA:</span>
          <span class="value">{{ selectedDataPoint.context_values.slow_ma.toFixed(3) }}</span>
        </div>
        <div class="context-item" v-if="selectedDataPoint.context_values?.current_position !== undefined">
          <span class="label">Position:</span>
          <span class="value">{{ selectedDataPoint.context_values.current_position }}</span>
        </div>
      </div>
    </div>

    <div class="no-data" v-else-if="decisionTimeline.length === 0">
      <p>No decision timeline data available. Run a backtest to see detailed decision analysis.</p>
    </div>

    <div class="timeline-filters" v-if="decisionTimeline.length > 0">
      <h4>Show Only</h4>
      <div class="filter-options">
        <label class="filter-checkbox">
          <input type="checkbox" v-model="filters.showEntrySignals" @change="applyFilters">
          Entry Signals ({{ entrySignalCount }})
        </label>
        <label class="filter-checkbox">
          <input type="checkbox" v-model="filters.showExitSignals" @change="applyFilters">
          Exit Signals ({{ exitSignalCount }})
        </label>
        <label class="filter-checkbox">
          <input type="checkbox" v-model="filters.showEvaluations" @change="applyFilters">
          Decision Evaluations ({{ evaluationCount }})
        </label>
      </div>
      <div class="filter-note">
        <small>💡 Tip: Uncheck all to see everything, or select specific types to focus on</small>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'DataPointTimelineNavigator',
  props: {
    decisionTimeline: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      currentIndex: 0,
      filteredTimeline: [],
      filters: {
        showEntrySignals: false,
        showExitSignals: false,
        showEvaluations: false
      }
    }
  },
  computed: {
    selectedDataPoint() {
      return this.filteredTimeline[this.currentIndex] || null
    },
    totalDataPoints() {
      return this.filteredTimeline.length
    },
    entrySignalCount() {
      return this.decisionTimeline.filter(point => 
        point.signal_type === 'entry_signal'
      ).length
    },
    exitSignalCount() {
      return this.decisionTimeline.filter(point => 
        point.signal_type === 'exit_signal'
      ).length
    },
    evaluationCount() {
      return this.decisionTimeline.filter(point => 
        point.signal_type === 'evaluation' || !point.signal_type
      ).length
    }
  },
  watch: {
    decisionTimeline: {
      handler() {
        this.applyFilters()
        this.currentIndex = 0
        // Auto-select first data point after filters are applied
        this.$nextTick(() => {
          if (this.filteredTimeline.length > 0) {
            this.$emit('datapoint-selected', this.filteredTimeline[0])
          }
        })
      },
      immediate: true
    },
    selectedDataPoint(newDataPoint) {
      // Emit the selected data point to parent component
      this.$emit('datapoint-selected', newDataPoint)
    }
  },
  methods: {
    goToFirst() {
      this.currentIndex = 0
    },
    goToPrevious() {
      if (this.currentIndex > 0) {
        this.currentIndex--
      }
    },
    goToNext() {
      if (this.currentIndex < this.filteredTimeline.length - 1) {
        this.currentIndex++
      }
    },
    goToLast() {
      this.currentIndex = Math.max(0, this.filteredTimeline.length - 1)
    },
    onSliderChange(event) {
      this.currentIndex = parseInt(event.target.value)
    },
    jumpToDataPoint(event) {
      const pointNumber = parseInt(event.target.value)
      if (pointNumber >= 1 && pointNumber <= this.filteredTimeline.length) {
        this.currentIndex = pointNumber - 1
      }
    },
    jumpToSignals() {
      // Find next data point with a true result
      const nextSignalIndex = this.filteredTimeline.findIndex((point, index) => 
        index > this.currentIndex && point.result === true
      )
      if (nextSignalIndex !== -1) {
        this.currentIndex = nextSignalIndex
      }
    },
    jumpToTrades() {
      // Find next data point that resulted in a trade (true entry or exit signal)
      const nextTradeIndex = this.filteredTimeline.findIndex((point, index) => 
        index > this.currentIndex && 
        point.result === true && 
        (point.rule_description?.includes('Entry') || point.rule_description?.includes('Exit'))
      )
      if (nextTradeIndex !== -1) {
        this.currentIndex = nextTradeIndex
      }
    },
    applyFilters() {
      // If no filters are selected, show everything
      const hasAnyFilter = this.filters.showEntrySignals || 
                          this.filters.showExitSignals || 
                          this.filters.showEvaluations

      if (!hasAnyFilter) {
        this.filteredTimeline = [...this.decisionTimeline]
      } else {
        // Show only the selected types
        this.filteredTimeline = this.decisionTimeline.filter(point => {
          // Entry signals
          if (this.filters.showEntrySignals && point.signal_type === 'entry_signal') {
            return true
          }
          
          // Exit signals
          if (this.filters.showExitSignals && point.signal_type === 'exit_signal') {
            return true
          }
          
          // Decision evaluations (regular evaluations that don't result in trades)
          if (this.filters.showEvaluations && (point.signal_type === 'evaluation' || !point.signal_type)) {
            return true
          }
          
          return false
        })
      }

      // Adjust current index if it's out of bounds
      if (this.currentIndex >= this.filteredTimeline.length) {
        this.currentIndex = Math.max(0, this.filteredTimeline.length - 1)
      }
    },
    formatTimestamp(timestamp) {
      if (!timestamp) return 'Unknown'
      
      try {
        // Handle different timestamp formats
        let date
        if (typeof timestamp === 'string') {
          // If it's an ISO string, parse it directly
          if (timestamp.includes('T') || timestamp.includes('-')) {
            date = new Date(timestamp)
          } else {
            // If it's just a date string, try parsing
            date = new Date(timestamp)
          }
        } else if (typeof timestamp === 'number') {
          // If it's a Unix timestamp (seconds), convert to milliseconds
          if (timestamp < 10000000000) {
            date = new Date(timestamp * 1000)
          } else {
            // Already in milliseconds
            date = new Date(timestamp)
          }
        } else {
          date = new Date(timestamp)
        }
        
        // Check if date is valid
        if (isNaN(date.getTime())) {
          console.warn('Invalid timestamp after parsing:', timestamp, 'resulted in:', date)
          return String(timestamp)
        }
        
        // Intelligent formatting based on time granularity
        const isIntradayData = this.detectIntradayData(date)
        
        if (isIntradayData) {
          // For intraday data (minute/hour bars), show date + time
          return date.toLocaleString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            weekday: 'short',
            hour: '2-digit',
            minute: '2-digit',
            hour12: true
          })
        } else {
          // For daily data, show just the date
          return date.toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            weekday: 'short'
          })
        }
      } catch (e) {
        console.warn('Error formatting timestamp:', timestamp, e)
        return String(timestamp)
      }
    },
    
    detectIntradayData(currentDate) {
      // Check if we have multiple data points on the same day
      // This indicates intraday (minute/hour) data rather than daily data
      if (this.decisionTimeline.length < 2) {
        // Not enough data to determine, default to showing time
        return true
      }
      
      // Sample a few timestamps to detect the pattern
      const sampleSize = Math.min(10, this.decisionTimeline.length)
      const dates = []
      
      for (let i = 0; i < sampleSize; i++) {
        const timestamp = this.decisionTimeline[i].timestamp
        if (timestamp) {
          try {
            let date
            if (typeof timestamp === 'string') {
              date = new Date(timestamp)
            } else if (typeof timestamp === 'number') {
              date = new Date(timestamp < 10000000000 ? timestamp * 1000 : timestamp)
            } else {
              date = new Date(timestamp)
            }
            
            if (!isNaN(date.getTime())) {
              dates.push(date.toDateString()) // Just the date part
            }
          } catch (e) {
            // Skip invalid timestamps
          }
        }
      }
      
      // If we have multiple timestamps on the same day, it's intraday data
      const uniqueDates = new Set(dates)
      const hasDuplicateDates = uniqueDates.size < dates.length
      
      // Also check if any timestamp has non-zero hours/minutes
      const hasTimeComponent = this.decisionTimeline.slice(0, sampleSize).some(point => {
        if (!point.timestamp) return false
        try {
          let date
          if (typeof point.timestamp === 'string') {
            date = new Date(point.timestamp)
          } else if (typeof point.timestamp === 'number') {
            date = new Date(point.timestamp < 10000000000 ? point.timestamp * 1000 : point.timestamp)
          } else {
            date = new Date(point.timestamp)
          }
          
          if (!isNaN(date.getTime())) {
            // Check if time is not midnight (00:00:00)
            return date.getHours() !== 0 || date.getMinutes() !== 0 || date.getSeconds() !== 0
          }
        } catch (e) {
          // Skip invalid timestamps
        }
        return false
      })
      
      return hasDuplicateDates || hasTimeComponent
    },
    getDecisionTypeClass(dataPoint) {
      if (dataPoint.rule_description?.includes('Entry')) {
        return 'entry-decision'
      } else if (dataPoint.rule_description?.includes('Exit')) {
        return 'exit-decision'
      }
      return 'other-decision'
    },
    getResultLabel(dataPoint) {
      if (!dataPoint) return 'Unknown'
      
      // Check if this is an insufficient data case
      if (dataPoint.rule_description?.includes('Insufficient Data')) {
        return '⚪ INSUFFICIENT DATA'
      }
      
      // Check for entry signals
      if (dataPoint.rule_description?.includes('Entry')) {
        return dataPoint.result ? '🚀 ENTRY SIGNAL' : '❌ NO ENTRY SIGNAL'
      }
      
      // Check for exit signals
      if (dataPoint.rule_description?.includes('Exit')) {
        return dataPoint.result ? '🛑 EXIT SIGNAL' : '❌ NO EXIT SIGNAL'
      }
      
      // Default fallback
      return dataPoint.result ? '✓ TRUE' : '✗ FALSE'
    },
    getResultBadgeClass(dataPoint) {
      if (!dataPoint) return 'neutral'
      
      // Insufficient data case
      if (dataPoint.rule_description?.includes('Insufficient Data')) {
        return 'insufficient'
      }
      
      // Entry signals
      if (dataPoint.rule_description?.includes('Entry')) {
        return dataPoint.result ? 'entry-signal' : 'no-signal'
      }
      
      // Exit signals
      if (dataPoint.rule_description?.includes('Exit')) {
        return dataPoint.result ? 'exit-signal' : 'no-signal'
      }
      
      // Default
      return dataPoint.result ? 'success' : 'neutral'
    }
  }
}
</script>

<style scoped>
.data-point-timeline-navigator {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
}

.navigator-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
  padding-bottom: var(--spacing-sm);
  border-bottom: 1px solid var(--border-primary);
}

.navigator-header h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}

.timeline-stats {
  display: flex;
  gap: var(--spacing-lg);
}

.stat {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
}

.timeline-controls {
  margin-bottom: var(--spacing-lg);
}

.navigation-buttons {
  display: flex;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-md);
}

.nav-btn {
  background: var(--color-brand);
  color: white;
  border: none;
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-lg);
  transition: var(--transition-normal);
  font-weight: var(--font-weight-medium);
}

.nav-btn:hover:not(:disabled) {
  background: var(--color-brand-dark);
  transform: translateY(-1px);
}

.nav-btn:disabled {
  background: var(--text-tertiary);
  cursor: not-allowed;
  opacity: 0.5;
}

.timeline-slider {
  margin-bottom: var(--spacing-md);
}

.timeline-range {
  width: 100%;
  height: 6px;
  border-radius: var(--radius-sm);
  background: var(--bg-tertiary);
  outline: none;
  -webkit-appearance: none;
}

.timeline-range::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--color-brand);
  cursor: pointer;
  transition: var(--transition-normal);
}

.timeline-range::-webkit-slider-thumb:hover {
  background: var(--color-brand-dark);
  transform: scale(1.1);
}

.timeline-range::-moz-range-thumb {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--color-brand);
  cursor: pointer;
  border: none;
  transition: var(--transition-normal);
}

.jump-controls {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.jump-controls label {
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.jump-input {
  width: 80px;
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
}

.jump-input:focus {
  outline: none;
  border-color: var(--color-brand);
  box-shadow: 0 0 0 2px rgba(var(--color-brand-rgb), 0.2);
}

.jump-btn {
  background: var(--color-success);
  color: white;
  border: none;
  padding: var(--spacing-xs) var(--spacing-md);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  transition: var(--transition-normal);
}

.jump-btn:hover {
  background: var(--color-success-dark);
  transform: translateY(-1px);
}

.current-datapoint-info {
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  padding: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
}

.datapoint-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.timestamp {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.decision-type {
  font-weight: var(--font-weight-medium);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.entry-decision {
  background: rgba(var(--color-success-rgb), 0.1);
  color: var(--color-success);
  border: 1px solid rgba(var(--color-success-rgb), 0.2);
}

.exit-decision {
  background: rgba(var(--color-danger-rgb), 0.1);
  color: var(--color-danger);
  border: 1px solid rgba(var(--color-danger-rgb), 0.2);
}

.other-decision {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  border: 1px solid var(--border-primary);
}

.result-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-sm);
}

.result-badge.success {
  background: rgba(var(--color-success-rgb), 0.1);
  color: var(--color-success);
  border: 1px solid rgba(var(--color-success-rgb), 0.2);
}

.result-badge.neutral {
  background: var(--bg-tertiary);
  color: var(--text-tertiary);
  border: 1px solid var(--border-primary);
}

.result-badge.entry-signal {
  background: rgba(34, 197, 94, 0.1);
  color: #16a34a;
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.result-badge.exit-signal {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.result-badge.no-signal {
  background: rgba(156, 163, 175, 0.1);
  color: #6b7280;
  border: 1px solid rgba(156, 163, 175, 0.3);
}

.result-badge.insufficient {
  background: rgba(245, 158, 11, 0.1);
  color: #d97706;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.trade-indicator {
  background: linear-gradient(135deg, #10b981, #059669);
  color: white;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-bold);
  font-size: var(--font-size-xs);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  box-shadow: 0 2px 4px rgba(16, 185, 129, 0.3);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.8;
  }
}

.quick-context {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-lg);
}

.context-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.context-item .label {
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.context-item .value {
  color: var(--text-primary);
  font-family: 'Courier New', monospace;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}

.no-data {
  text-align: center;
  padding: var(--spacing-2xl);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.timeline-filters {
  border-top: 1px solid var(--border-primary);
  padding-top: var(--spacing-lg);
}

.timeline-filters h4 {
  margin: 0 0 var(--spacing-md) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.filter-options {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-lg);
}

.filter-checkbox {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  cursor: pointer;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  transition: var(--transition-normal);
}

.filter-checkbox:hover {
  color: var(--text-primary);
}

.filter-checkbox input[type="checkbox"] {
  margin: 0;
  accent-color: var(--color-brand);
}

.filter-note {
  margin-top: var(--spacing-md);
  padding: var(--spacing-sm);
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  border-left: 3px solid var(--color-brand);
}

.filter-note small {
  color: var(--text-secondary);
  font-style: italic;
}

/* Responsive design */
@media (max-width: 768px) {
  .datapoint-header {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .quick-context {
    flex-direction: column;
    gap: 8px;
  }
  
  .jump-controls {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .filter-options {
    flex-direction: column;
    gap: 8px;
  }
}
</style>
