<template>
  <div class="action-decision-inspector">
    <div v-if="!decisionTimeline || decisionTimeline.length === 0" class="no-data">
      <p>No decision data available for this backtest run.</p>
    </div>
    <div v-else>
      <div class="timeline-controls">
        <input type="text" v-model="filterText" placeholder="Filter by action or rule..." />
        <Button @click="exportDecisions" label="Export" icon="pi pi-download" class="p-button-sm" />
      </div>
      <div class="timeline-summary">
        <span class="summary-item">Total Decisions: {{ filteredTimeline.length }}</span>
        <span class="summary-item">Successful: {{ successCount }}</span>
        <span class="summary-item">Failed: {{ failCount }}</span>
      </div>
      <div class="timeline-container">
        <div v-for="(decision, index) in filteredTimeline" :key="index" class="timeline-item" :class="{ success: decision.result, failed: !decision.result }">
          <div class="timeline-header" @click="toggleDecision(index)">
            <span class="timestamp">{{ formatTimestamp(decision.timestamp) }}</span>
            <span class="action-name">{{ decision.action_name }}</span>
            <span class="rule-name">{{ decision.rule_description }}</span>
            <span class="result-badge">{{ decision.result ? 'SUCCESS' : 'FAIL' }}</span>
          </div>
          <div v-if="expandedDecision === index" class="timeline-details">
            <div v-if="decision.context_snapshot" class="context-snapshot">
              <h4>Context Snapshot</h4>
              <pre>{{ formatDetails(decision.context_snapshot) }}</pre>
            </div>
            <div v-if="decision.reason" class="reason">
              <h4>Reason</h4>
              <p>{{ decision.reason }}</p>
            </div>
            <div v-if="decision.steps" class="decision-chain">
              <h4>Decision Chain</h4>
              <ul>
                <li v-for="(step, stepIndex) in decision.steps" :key="stepIndex" :class="{ success: step.result, failed: !step.result }">
                  <div class="step-header">
                    <span class="step-name">{{ step.rule_name }}</span>
                    <span class="step-result">{{ step.result ? '✓' : '✗' }}</span>
                  </div>
                  <div class="step-details">
                    <pre>{{ formatDetails(step.context_snapshot) }}</pre>
                    <div v-if="step.rule_name === 'select_call_options' && step.context_snapshot.selected_legs">
                      <h5>Selected Legs:</h5>
                      <ul>
                        <li v-for="(leg, legIndex) in step.context_snapshot.selected_legs" :key="legIndex">
                          {{ leg.symbol }} - Delta: {{ leg.delta }}, Price: {{ leg.price }}, DTE: {{ leg.days_to_expiration }}
                        </li>
                      </ul>
                    </div>
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed } from 'vue'
import Button from 'primevue/button';

export default {
  name: 'ActionDecisionInspector',
  components: {
    Button
  },
  props: {
    decisionTimeline: {
      type: Array,
      default: () => []
    }
  },
  setup(props) {
    const filterText = ref('')
    const expandedDecision = ref(null)

    const filteredTimeline = computed(() => {
      if (!filterText.value) {
        return props.decisionTimeline
      }
      return props.decisionTimeline.filter(decision => {
        const searchText = filterText.value.toLowerCase()
        return (
          decision.action_name.toLowerCase().includes(searchText) ||
          decision.rule_description.toLowerCase().includes(searchText)
        )
      })
    })

    const successCount = computed(() => {
      return filteredTimeline.value.filter(d => d.result).length
    })

    const failCount = computed(() => {
      return filteredTimeline.value.filter(d => !d.result).length
    })

    const toggleDecision = (index) => {
      expandedDecision.value = expandedDecision.value === index ? null : index
    }

    const formatTimestamp = (timestamp) => {
      return new Date(timestamp).toLocaleString()
    }

    const formatDetails = (details) => {
      return JSON.stringify(details, null, 2)
    }

    const exportDecisions = () => {
      const data = JSON.stringify(props.decisionTimeline, null, 2)
      const blob = new Blob([data], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'decision_timeline.json'
      a.click()
      URL.revokeObjectURL(url)
    }

    return {
      filterText,
      expandedDecision,
      filteredTimeline,
      successCount,
      failCount,
      toggleDecision,
      formatTimestamp,
      formatDetails,
      exportDecisions
    }
  }
}
</script>

<style scoped>
.action-decision-inspector {
  font-family: sans-serif;
  background: transparent;
  color: var(--text-color);
}

.no-data {
  padding: 2rem;
  text-align: center;
  color: var(--text-color-secondary);
  background: var(--surface-card);
  border-radius: 6px;
  border: 1px solid var(--surface-border);
}

.timeline-controls {
  display: flex;
  gap: 1rem;
  margin-bottom: 1rem;
  align-items: center;
}

.timeline-controls input {
  flex: 1;
  padding: 0.5rem;
  border: 1px solid var(--surface-border);
  border-radius: 4px;
  background: var(--surface-ground);
  color: var(--text-color);
}

.timeline-summary {
  display: flex;
  gap: 2rem;
  margin-bottom: 1rem;
  padding: 1rem;
  background: var(--surface-card);
  border-radius: 6px;
  border: 1px solid var(--surface-border);
}

.summary-item {
  font-weight: 500;
  color: var(--text-color);
}

.timeline-container {
  border: 1px solid var(--surface-border);
  border-radius: 6px;
  background: var(--surface-card);
  overflow: hidden;
}

.timeline-item {
  border-bottom: 1px solid var(--surface-border);
}

.timeline-item:last-child {
  border-bottom: none;
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  cursor: pointer;
  background: var(--surface-ground);
  transition: background-color 0.2s;
}

.timeline-header:hover {
  background: var(--surface-hover);
}

.timeline-details {
  padding: 1rem;
  background: var(--surface-card);
  border-top: 1px solid var(--surface-border);
}

.success .timeline-header {
  border-left: 4px solid var(--green-500);
}

.failed .timeline-header {
  border-left: 4px solid var(--red-500);
}

.timestamp {
  font-size: 0.875rem;
  color: var(--text-color-secondary);
  font-family: monospace;
}

.action-name {
  font-weight: 500;
  color: var(--text-color);
}

.rule-name {
  color: var(--text-color-secondary);
  font-style: italic;
}

.result-badge {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}

.success .result-badge {
  background: var(--green-100);
  color: var(--green-800);
}

.failed .result-badge {
  background: var(--red-100);
  color: var(--red-800);
}

.context-snapshot, .reason, .decision-chain {
  margin-top: 1rem;
}

.context-snapshot h4, .reason h4, .decision-chain h4 {
  margin: 0 0 0.5rem 0;
  color: var(--text-color);
  font-size: 1rem;
}

.context-snapshot pre {
  background: var(--surface-ground);
  padding: 1rem;
  border-radius: 4px;
  border: 1px solid var(--surface-border);
  overflow-x: auto;
  font-size: 0.875rem;
  color: var(--text-color);
}

.decision-chain ul {
  list-style-type: none;
  padding: 0;
  margin: 0;
}

.decision-chain li {
  margin-bottom: 1rem;
  padding: 1rem;
  background: var(--surface-ground);
  border-radius: 4px;
  border: 1px solid var(--surface-border);
}

.step-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.step-name {
  font-weight: 500;
  color: var(--text-color);
}

.step-result {
  font-weight: bold;
  font-size: 1.2rem;
}

.success .step-result {
  color: var(--green-500);
}

.failed .step-result {
  color: var(--red-500);
}

.step-details {
  margin-top: 0.5rem;
}

.step-details pre {
  background: var(--surface-card);
  padding: 0.5rem;
  border-radius: 4px;
  border: 1px solid var(--surface-border);
  font-size: 0.75rem;
  max-height: 200px;
  overflow-y: auto;
}

.step-details h5 {
  margin: 1rem 0 0.5rem 0;
  color: var(--text-color);
  font-size: 0.875rem;
}

.step-details ul {
  margin: 0;
  padding-left: 1rem;
}

.step-details li {
  margin: 0.25rem 0;
  padding: 0;
  background: none;
  border: none;
  font-size: 0.875rem;
  color: var(--text-color-secondary);
}
</style>
