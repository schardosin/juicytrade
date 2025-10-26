<template>
  <div class="granular-decision-breakdown-panel">
    <div class="panel-header">
      <h3>Decision Breakdown</h3>
      <div class="decision-summary" v-if="selectedDataPoint">
        <span class="decision-result" :class="selectedDataPoint.result ? 'positive' : 'negative'">
          {{ selectedDataPoint.result ? 'PASSED' : 'FAILED' }}
        </span>
        <span class="decision-check">Check #{{ selectedDataPoint.check_number || 'N/A' }}</span>
      </div>
    </div>

    <div v-if="!selectedDataPoint" class="no-selection">
      <p>Select a data point from the timeline navigator to see detailed decision breakdown.</p>
    </div>

    <div v-else class="decision-content">
      <!-- Rule Description Section -->
      <div class="section rule-section">
        <h4>Rule Evaluation</h4>
        <div class="rule-info">
          <div class="rule-name">
            <strong>{{ selectedDataPoint.rule_description || 'Unknown Rule' }}</strong>
          </div>
          <div class="rule-result" :class="selectedDataPoint.result ? 'success' : 'failure'">
            <span class="result-icon">{{ selectedDataPoint.result ? '✓' : '✗' }}</span>
            <span class="result-text">{{ selectedDataPoint.result ? 'CONDITION MET' : 'CONDITION NOT MET' }}</span>
          </div>
        </div>
      </div>

      <!-- Market Context Section -->
      <div class="section context-section" v-if="selectedDataPoint.context_values">
        <h4>Market Context</h4>
        <div class="context-grid">
          <div class="context-card" v-for="(value, key) in selectedDataPoint.context_values" :key="key">
            <div class="context-label">{{ formatContextLabel(key) }}</div>
            <div class="context-value" :class="getContextValueClass(key, value)">
              {{ formatContextValue(key, value) }}
            </div>
          </div>
        </div>
      </div>

      <!-- Strategy Parameters Section -->
      <div class="section parameters-section" v-if="selectedDataPoint.parameters">
        <h4>Strategy Parameters</h4>
        <div class="parameters-grid">
          <div class="parameter-item" v-for="(value, key) in selectedDataPoint.parameters" :key="key">
            <span class="parameter-label">{{ formatParameterLabel(key) }}:</span>
            <span class="parameter-value">{{ formatParameterValue(key, value) }}</span>
          </div>
        </div>
      </div>

      <!-- Evaluation Details Section -->
      <div class="section evaluation-section" v-if="selectedDataPoint.evaluation_details">
        <h4>Evaluation Details</h4>
        <div class="evaluation-info">
          <div class="detail-item" v-if="selectedDataPoint.evaluation_details.chain_type">
            <span class="detail-label">Chain Type:</span>
            <span class="detail-value chain-type" :class="selectedDataPoint.evaluation_details.chain_type">
              {{ selectedDataPoint.evaluation_details.chain_type.toUpperCase() }}
            </span>
          </div>
          <div class="detail-item" v-if="selectedDataPoint.evaluation_details.rules_evaluated">
            <span class="detail-label">Rules Evaluated:</span>
            <span class="detail-value">{{ selectedDataPoint.evaluation_details.rules_evaluated }}</span>
          </div>
          <div class="detail-item" v-if="selectedDataPoint.evaluation_details.risk_management_triggered !== undefined">
            <span class="detail-label">Risk Management:</span>
            <span class="detail-value" :class="selectedDataPoint.evaluation_details.risk_management_triggered ? 'triggered' : 'normal'">
              {{ selectedDataPoint.evaluation_details.risk_management_triggered ? 'TRIGGERED' : 'Normal' }}
            </span>
          </div>
        </div>
      </div>

      <!-- Decision State Section (if available) -->
      <div class="section decision-state-section" v-if="selectedDataPoint.evaluation_details?.decision_state">
        <h4>Decision Chain State</h4>
        <div class="decision-state">
          <pre class="state-json">{{ formatDecisionState(selectedDataPoint.evaluation_details.decision_state) }}</pre>
        </div>
      </div>

      <!-- Timestamp and Metadata -->
      <div class="section metadata-section">
        <h4>Metadata</h4>
        <div class="metadata-grid">
          <div class="metadata-item">
            <span class="metadata-label">Timestamp:</span>
            <span class="metadata-value">{{ formatTimestamp(selectedDataPoint.timestamp) }}</span>
          </div>
          <div class="metadata-item" v-if="selectedDataPoint.strategy_id">
            <span class="metadata-label">Strategy ID:</span>
            <span class="metadata-value">{{ selectedDataPoint.strategy_id }}</span>
          </div>
          <div class="metadata-item" v-if="selectedDataPoint.strategy_name">
            <span class="metadata-label">Strategy Name:</span>
            <span class="metadata-value">{{ selectedDataPoint.strategy_name }}</span>
          </div>
          <div class="metadata-item" v-if="selectedDataPoint.error">
            <span class="metadata-label">Error:</span>
            <span class="metadata-value error">{{ selectedDataPoint.error }}</span>
          </div>
        </div>
      </div>

      <!-- Analysis Insights -->
      <div class="section insights-section">
        <h4>Analysis Insights</h4>
        <div class="insights">
          <div class="insight-item" v-for="insight in generateInsights(selectedDataPoint)" :key="insight.type">
            <div class="insight-icon" :class="insight.type">{{ insight.icon }}</div>
            <div class="insight-content">
              <div class="insight-title">{{ insight.title }}</div>
              <div class="insight-description">{{ insight.description }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'GranularDecisionBreakdownPanel',
  props: {
    selectedDataPoint: {
      type: Object,
      default: null
    }
  },
  methods: {
    formatContextLabel(key) {
      const labelMap = {
        'current_price': 'Current Price',
        'fast_ma': 'Fast Moving Average',
        'slow_ma': 'Slow Moving Average',
        'ma_difference': 'MA Difference',
        'current_position': 'Current Position',
        'entry_price': 'Entry Price',
        'unrealized_pnl': 'Unrealized P&L',
        'position_size_calculation': 'Position Size'
      }
      return labelMap[key] || key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())
    },

    formatContextValue(key, value) {
      if (value === null || value === undefined) return 'N/A'
      
      if (key.includes('price') || key.includes('pnl')) {
        return `$${Number(value).toFixed(2)}`
      }
      if (key.includes('ma') || key.includes('difference')) {
        return Number(value).toFixed(3)
      }
      if (key === 'current_position' || key === 'position_size_calculation') {
        return Number(value).toLocaleString()
      }
      
      return String(value)
    },

    getContextValueClass(key, value) {
      if (key === 'ma_difference') {
        return value > 0 ? 'positive' : value < 0 ? 'negative' : 'neutral'
      }
      if (key === 'unrealized_pnl') {
        return value > 0 ? 'profit' : value < 0 ? 'loss' : 'neutral'
      }
      if (key === 'current_position') {
        return value > 0 ? 'long' : value < 0 ? 'short' : 'flat'
      }
      return ''
    },

    formatParameterLabel(key) {
      const labelMap = {
        'fast_period': 'Fast Period',
        'slow_period': 'Slow Period',
        'stop_loss_pct': 'Stop Loss %',
        'take_profit_pct': 'Take Profit %',
        'symbol': 'Symbol'
      }
      return labelMap[key] || key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())
    },

    formatParameterValue(key, value) {
      if (key.includes('pct')) {
        return `${value}%`
      }
      return String(value)
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
      // Use the same logic as the timeline navigator
      // Check if we have access to the parent's decision timeline data
      if (this.$parent && this.$parent.decisionTimeline && this.$parent.decisionTimeline.length >= 2) {
        const timeline = this.$parent.decisionTimeline
        const sampleSize = Math.min(10, timeline.length)
        const dates = []
        
        for (let i = 0; i < sampleSize; i++) {
          const timestamp = timeline[i].timestamp
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
        const hasTimeComponent = timeline.slice(0, sampleSize).some(point => {
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
      }
      
      // Fallback: if we can't access parent data, check the current timestamp
      if (currentDate) {
        return currentDate.getHours() !== 0 || currentDate.getMinutes() !== 0 || currentDate.getSeconds() !== 0
      }
      
      // Default to showing time if we can't determine
      return true
    },

    formatDecisionState(state) {
      if (!state) return 'No state data available'
      try {
        return JSON.stringify(state, null, 2)
      } catch (e) {
        return String(state)
      }
    },

    generateInsights(dataPoint) {
      const insights = []

      if (!dataPoint) return insights

      // Moving Average Analysis
      if (dataPoint.context_values?.fast_ma && dataPoint.context_values?.slow_ma) {
        const fastMa = dataPoint.context_values.fast_ma
        const slowMa = dataPoint.context_values.slow_ma
        const difference = fastMa - slowMa
        const percentDiff = Math.abs(difference / slowMa * 100)

        if (Math.abs(difference) < 0.1) {
          insights.push({
            type: 'warning',
            icon: '⚠️',
            title: 'Near Crossover',
            description: `Moving averages are very close (${difference.toFixed(3)}). Potential signal imminent.`
          })
        } else if (percentDiff > 2) {
          insights.push({
            type: 'info',
            icon: 'ℹ️',
            title: 'Strong Trend',
            description: `Large MA separation (${percentDiff.toFixed(1)}%) indicates strong ${difference > 0 ? 'bullish' : 'bearish'} trend.`
          })
        }
      }

      // Position Analysis
      if (dataPoint.context_values?.current_position !== undefined) {
        const position = dataPoint.context_values.current_position
        if (position === 0) {
          insights.push({
            type: 'neutral',
            icon: '⚪',
            title: 'No Position',
            description: 'Currently flat - ready for new entry signals.'
          })
        } else {
          const pnl = dataPoint.context_values?.unrealized_pnl || 0
          const pnlStatus = pnl > 0 ? 'profitable' : pnl < 0 ? 'losing' : 'breakeven'
          insights.push({
            type: position > 0 ? 'success' : 'info',
            icon: position > 0 ? '📈' : '📉',
            title: `${position > 0 ? 'Long' : 'Short'} Position`,
            description: `Holding ${Math.abs(position)} shares, currently ${pnlStatus} (${pnl >= 0 ? '+' : ''}$${pnl.toFixed(2)}).`
          })
        }
      }

      // Decision Result Analysis
      if (dataPoint.rule_description?.includes('Entry') && dataPoint.result) {
        insights.push({
          type: 'success',
          icon: '🚀',
          title: 'Entry Signal Triggered',
          description: 'All entry conditions met - position should be opened.'
        })
      } else if (dataPoint.rule_description?.includes('Exit') && dataPoint.result) {
        insights.push({
          type: 'warning',
          icon: '🛑',
          title: 'Exit Signal Triggered',
          description: 'Exit conditions met - position should be closed.'
        })
      }

      // Risk Management Analysis
      if (dataPoint.evaluation_details?.risk_management_triggered) {
        insights.push({
          type: 'error',
          icon: '🚨',
          title: 'Risk Management Alert',
          description: 'Risk management rules triggered - immediate action required.'
        })
      }

      return insights
    }
  }
}
</script>

<style scoped>
.granular-decision-breakdown-panel {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.panel-header {
  background: var(--bg-tertiary);
  padding: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-header h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}

.decision-summary {
  display: flex;
  gap: var(--spacing-md);
  align-items: center;
}

.decision-result {
  padding: var(--spacing-xs) var(--spacing-md);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-sm);
}

.decision-result.positive {
  background: rgba(var(--color-success-rgb), 0.1);
  color: var(--color-success);
  border: 1px solid rgba(var(--color-success-rgb), 0.2);
}

.decision-result.negative {
  background: rgba(var(--color-danger-rgb), 0.1);
  color: var(--color-danger);
  border: 1px solid rgba(var(--color-danger-rgb), 0.2);
}

.decision-check {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
}

.no-selection {
  padding: var(--spacing-2xl);
  text-align: center;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.decision-content {
  padding: var(--spacing-lg);
}

.section {
  margin-bottom: var(--spacing-xl);
}

.section h4 {
  margin: 0 0 var(--spacing-md) 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  border-bottom: 2px solid var(--border-primary);
  padding-bottom: var(--spacing-xs);
}

/* Rule Section */
.rule-info {
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  padding: var(--spacing-lg);
}

.rule-name {
  font-size: var(--font-size-lg);
  margin-bottom: var(--spacing-sm);
  color: var(--text-primary);
}

.rule-result {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.result-icon {
  font-size: var(--font-size-xl);
}

.rule-result.success {
  color: var(--color-success);
}

.rule-result.failure {
  color: var(--color-danger);
}

/* Context Section */
.context-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--spacing-md);
}

.context-card {
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  padding: var(--spacing-md);
  transition: var(--transition-normal);
}

.context-card:hover {
  border-color: var(--border-secondary);
  box-shadow: var(--shadow-sm);
}

.context-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-bottom: var(--spacing-xs);
  font-weight: var(--font-weight-medium);
}

.context-value {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  font-family: 'Courier New', monospace;
  color: var(--text-primary);
}

.context-value.positive {
  color: var(--color-success);
}

.context-value.negative {
  color: var(--color-danger);
}

.context-value.profit {
  color: var(--color-success);
}

.context-value.loss {
  color: var(--color-danger);
}

.context-value.long {
  color: var(--color-info);
}

.context-value.short {
  color: var(--color-warning);
}

.context-value.flat {
  color: var(--text-tertiary);
}

/* Parameters Section */
.parameters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 8px;
}

.parameter-item {
  display: flex;
  justify-content: space-between;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
}

.parameter-label {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.parameter-value {
  font-family: 'Courier New', monospace;
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

/* Evaluation Section */
.evaluation-info {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.detail-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-label {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.detail-value {
  font-family: 'Courier New', monospace;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  color: var(--text-primary);
}

.chain-type.entry {
  background: rgba(var(--color-success-rgb), 0.1);
  color: var(--color-success);
  border-color: rgba(var(--color-success-rgb), 0.2);
}

.chain-type.exit {
  background: rgba(var(--color-danger-rgb), 0.1);
  color: var(--color-danger);
  border-color: rgba(var(--color-danger-rgb), 0.2);
}

.detail-value.triggered {
  background: rgba(var(--color-danger-rgb), 0.1);
  color: var(--color-danger);
  border-color: rgba(var(--color-danger-rgb), 0.2);
  font-weight: var(--font-weight-semibold);
}

/* Decision State Section */
.decision-state {
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  padding: var(--spacing-md);
}

.state-json {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

/* Metadata Section */
.metadata-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-sm);
}

.metadata-item {
  display: flex;
  justify-content: space-between;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
}

.metadata-label {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.metadata-value {
  font-family: 'Courier New', monospace;
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.metadata-value.error {
  color: var(--color-danger);
  font-weight: var(--font-weight-semibold);
}

/* Insights Section */
.insights {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.insight-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border-radius: 6px;
  border-left: 4px solid #dee2e6;
}

.insight-item.success {
  background: #d4edda;
  border-left-color: #28a745;
}

.insight-item.warning {
  background: #fff3cd;
  border-left-color: #ffc107;
}

.insight-item.error {
  background: #f8d7da;
  border-left-color: #dc3545;
}

.insight-item.info {
  background: #d1ecf1;
  border-left-color: #17a2b8;
}

.insight-item.neutral {
  background: #f8f9fa;
  border-left-color: #6c757d;
}

.insight-icon {
  font-size: 1.2em;
  margin-top: 2px;
}

.insight-content {
  flex: 1;
}

.insight-title {
  font-weight: 600;
  color: #495057;
  margin-bottom: 4px;
}

.insight-description {
  color: #6c757d;
  font-size: 0.9em;
  line-height: 1.4;
}

/* Responsive design */
@media (max-width: 768px) {
  .context-grid,
  .parameters-grid,
  .metadata-grid {
    grid-template-columns: 1fr;
  }
  
  .evaluation-info {
    flex-direction: column;
    gap: 8px;
  }
  
  .decision-summary {
    flex-direction: column;
    gap: 8px;
    align-items: flex-end;
  }
}
</style>
