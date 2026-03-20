<template>
  <div class="automation-dashboard">
    <!-- Header Section -->
    <div class="dashboard-header">
      <div class="header-content">
        <h1 class="dashboard-title">
          <i class="pi pi-bolt"></i>
          Automation Dashboard
        </h1>
        <p class="dashboard-subtitle">
          Configure and monitor automated credit spread trading
        </p>
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-refresh"
          class="p-button-text"
          @click="refreshData"
          :loading="isRefreshing"
          title="Refresh"
        />
        <Button
          icon="pi pi-plus"
          label="New Config"
          class="p-button-primary"
          @click="$router.push('/automation/create')"
        />
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading-state">
      <div class="loading-spinner"></div>
      <p>Loading automation configs...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <i class="pi pi-exclamation-triangle"></i>
      <p>{{ error }}</p>
      <Button label="Retry" class="p-button-sm" @click="refreshData" />
    </div>

    <!-- Empty State -->
    <div v-else-if="!configs.length" class="empty-state">
      <div class="empty-icon">
        <i class="pi pi-inbox"></i>
      </div>
      <h3>No Automation Configs</h3>
      <p>Create your first automation config to start automated trading</p>
      <Button
        icon="pi pi-plus"
        label="Create Config"
        class="p-button-primary"
        @click="$router.push('/automation/create')"
      />
    </div>

    <!-- Configs Grid -->
    <div v-else class="configs-grid">
      <div
        v-for="config in configs"
        :key="config.id"
        class="config-card"
        :class="[getStatusClass(config), { 'mobile-collapsed': isMobile && !isCardExpanded(config.id) }]"
      >
        <div class="config-header" @click="isMobile ? toggleCardExpanded(config.id) : null">
          <div class="config-info">
            <h3 class="config-name">{{ config.name }}</h3>
            <div class="config-meta">
              <span class="config-symbol">{{ config.symbol }}</span>
              <span class="config-strategy">{{ formatStrategy(config.trade_config?.strategy) }}</span>
              <span class="config-recurrence" :class="'recurrence-' + (config.recurrence || 'once')">
                <i :class="config.recurrence === 'daily' ? 'pi pi-sync' : 'pi pi-circle'"></i>
                {{ config.recurrence === 'daily' ? 'Daily' : 'Once' }}
              </span>
            </div>
          </div>
          <div class="config-header-right">
            <span class="status-badge" :class="getRunningStatusClass(config)">
              <i :class="getStatusIcon(config)"></i>
              <span v-if="!isMobile">{{ getStatusText(config) }}</span>
            </span>
            <button 
              v-if="isMobile" 
              class="expand-toggle"
              @click.stop="toggleCardExpanded(config.id)"
            >
              <i :class="isCardExpanded(config.id) ? 'pi pi-chevron-up' : 'pi pi-chevron-down'"></i>
            </button>
          </div>
        </div>

        <!-- Card Details - Collapsible on mobile -->
        <div v-show="!isMobile || isCardExpanded(config.id)" class="card-details">
        <!-- Indicators Summary -->
          <div class="indicators-section">
            <div class="section-label">Entry Criteria</div>
            <!-- Multiple groups: grouped display -->
            <template v-if="config.indicator_groups?.length > 1">
              <div
                v-for="(group, gIdx) in config.indicator_groups"
                :key="group.id || gIdx"
              >
                <div v-if="gIdx > 0" class="or-divider-compact">
                  <span class="or-divider-compact-label">OR</span>
                </div>
                <div class="indicator-group-dashboard">
                  <div class="group-label">{{ group.name || 'Group ' + (gIdx + 1) }}</div>
                  <div class="indicators-grid">
                    <div
                      v-for="indicator in (group.indicators || []).filter(ind => ind.enabled)"
                      :key="indicator.id || indicator.type"
                      class="indicator-chip"
                      :class="getIndicatorStatusClass(config, indicator)"
                    >
                      <span class="indicator-name">{{ formatIndicatorType(indicator.type) }}</span>
                      <span class="indicator-value">{{ formatIndicatorCondition(indicator) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </template>
            <!-- Single group or flat: flat display (identical to current) -->
            <template v-else>
              <div class="indicators-grid">
                <div
                  v-for="indicator in getEnabledIndicators(config)"
                  :key="indicator.id || indicator.type"
                  class="indicator-chip"
                  :class="getIndicatorStatusClass(config, indicator)"
                >
                  <span class="indicator-name">{{ formatIndicatorType(indicator.type) }}</span>
                  <span class="indicator-value">{{ formatIndicatorCondition(indicator) }}</span>
                </div>
              </div>
            </template>
          </div>

          <!-- Trade Config Summary -->
          <div class="trade-summary">
            <div class="summary-item">
              <span class="summary-label">Entry Time</span>
              <span class="summary-value">{{ config.entry_time }} {{ config.entry_timezone }}</span>
            </div>
            <template v-if="config.trade_config?.strategy === 'iron_condor'">
              <div class="summary-item">
                <span class="summary-label">Put Delta/Width</span>
                <span class="summary-value">{{ config.trade_config?.put_side_config?.target_delta || 'N/A' }} / {{ config.trade_config?.put_side_config?.width || 'N/A' }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Call Delta/Width</span>
                <span class="summary-value">{{ config.trade_config?.call_side_config?.target_delta || 'N/A' }} / {{ config.trade_config?.call_side_config?.width || 'N/A' }}</span>
              </div>
            </template>
            <template v-else>
              <div class="summary-item">
                <span class="summary-label">Delta</span>
                <span class="summary-value">{{ config.trade_config?.target_delta || 'N/A' }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Width</span>
                <span class="summary-value">{{ config.trade_config?.width || 'N/A' }}</span>
              </div>
            </template>
            <div class="summary-item">
              <span class="summary-label">Max Capital</span>
              <span class="summary-value">${{ formatNumber(config.trade_config?.max_capital) }}</span>
            </div>
          </div>

          <!-- Status Details (when running) -->
          <div v-if="getAutomationStatus(config.id)" class="status-details">
            <div class="status-row">
              <span class="status-label">State:</span>
              <span class="status-value">{{ getAutomationStatus(config.id)?.status || getAutomationStatus(config.id)?.state }}</span>
            </div>
            <!-- TradedToday indicator for daily automations -->
            <div v-if="config.recurrence === 'daily'" class="status-row">
              <span class="status-label">Today's Trade:</span>
              <span class="status-value traded-today-status" :class="{ 'traded': getAutomationStatus(config.id)?.traded_today }">
                <i :class="getAutomationStatus(config.id)?.traded_today ? 'pi pi-check-circle' : 'pi pi-clock'"></i>
                {{ getAutomationStatus(config.id)?.traded_today ? 'Completed' : 'Pending' }}
              </span>
            </div>
            <div v-if="getAutomationStatus(config.id)?.message" class="status-row">
              <span class="status-label">Message:</span>
              <span class="status-value">{{ getAutomationStatus(config.id)?.message }}</span>
            </div>
            <!-- Group-aware indicator results -->
            <template v-if="getAutomationStatus(config.id)?.group_results?.length > 1">
              <div class="indicator-results">
                <div class="results-label">Last Evaluation:</div>
                <div
                  v-for="(gr, grIdx) in getAutomationStatus(config.id).group_results"
                  :key="gr.group_id || grIdx"
                >
                  <div v-if="grIdx > 0" class="or-divider-compact">
                    <span class="or-divider-compact-label">OR</span>
                  </div>
                  <div class="indicator-group-dashboard">
                    <div class="group-header-dashboard">
                      <span class="group-label">{{ gr.group_name || 'Group ' + (grIdx + 1) }}</span>
                      <span class="group-result-badge" :class="gr.pass ? 'passed' : 'failed'">
                        {{ gr.pass ? '&#10003; Pass' : '&#10007; Fail' }}
                      </span>
                    </div>
                    <div class="results-grid">
                      <div
                        v-for="(result, idx) in gr.indicator_results"
                        :key="idx"
                        class="result-chip"
                        :class="{ 
                          'passed': result.pass && !result.stale, 
                          'failed': !result.pass && !result.stale,
                          'stale': result.stale 
                        }"
                        :title="result.stale ? `Stale data: ${result.error || 'Fetch failed'}` : result.details"
                      >
                        <span>{{ formatIndicatorType(result.type) }}</span>
                        <span v-if="result.stale" class="stale-icon">&#9888;</span>
                        <span>{{ formatIndicatorValue(result) }}</span>
                      </div>
                    </div>
                  </div>
                </div>
                <div class="overall-status" :class="getAutomationStatus(config.id)?.all_indicators_pass ? 'passing' : 'failing'">
                  <template v-if="getAutomationStatus(config.id)?.all_indicators_pass">
                    &#10003; Passing ({{ getAutomationStatus(config.id).group_results.find(g => g.pass)?.group_name || 'Group' }})
                  </template>
                  <template v-else>
                    &#10007; Not passing
                  </template>
                </div>
              </div>
            </template>
            <!-- Flat indicator results (single group or backward compat) -->
            <div v-else-if="getAutomationStatus(config.id)?.indicator_results" class="indicator-results">
              <div class="results-label">Last Evaluation:</div>
              <div class="results-grid">
                <div
                  v-for="(result, idx) in getAutomationStatus(config.id)?.indicator_results"
                  :key="idx"
                  class="result-chip"
                  :class="{ 
                    'passed': result.pass && !result.stale, 
                    'failed': !result.pass && !result.stale,
                    'stale': result.stale 
                  }"
                  :title="result.stale ? `Stale data: ${result.error || 'Fetch failed'}` : result.details"
                >
                  <span>{{ formatIndicatorType(result.type) }}</span>
                  <span v-if="result.stale" class="stale-icon">&#9888;</span>
                  <span>{{ formatIndicatorValue(result) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="config-actions">
          <Button
            v-if="!isConfigRunning(config.id)"
            icon="pi pi-play"
            class="p-button-text p-button-success p-button-sm"
            title="Start Automation"
            @click="startAutomation(config)"
            :loading="actionLoading === `start_${config.id}`"
          />
          <Button
            v-else
            icon="pi pi-stop"
            class="p-button-text p-button-danger p-button-sm"
            title="Stop Automation"
            @click="stopAutomation(config)"
            :loading="actionLoading === `stop_${config.id}`"
          />
          <Button
            icon="pi pi-list"
            class="p-button-text p-button-sm"
            :class="{ 'p-button-warning': isConfigRunning(config.id) }"
            title="View Logs"
            @click="showLogs(config)"
          />
          <Button
            icon="pi pi-sync"
            class="p-button-text p-button-sm"
            title="Test Indicators"
            @click="evaluateIndicators(config)"
            :loading="actionLoading === `eval_${config.id}`"
          />
          <Button
            icon="pi pi-copy"
            class="p-button-text p-button-sm"
            title="Duplicate Config"
            @click="duplicateConfig(config)"
            :loading="actionLoading === `duplicate_${config.id}`"
          />
          <Button
            v-if="config.recurrence === 'daily' && getAutomationStatus(config.id)?.traded_today"
            icon="pi pi-replay"
            class="p-button-text p-button-warning p-button-sm"
            title="Reset Today's Trade (Allow trading again today)"
            @click="resetTradedToday(config)"
            :loading="actionLoading === `reset_${config.id}`"
          />
          <Button
            icon="pi pi-pencil"
            class="p-button-text p-button-sm"
            title="Edit Config"
            @click="editConfig(config)"
          />
          <Button
            :icon="config.enabled ? 'pi pi-eye' : 'pi pi-eye-slash'"
            class="p-button-text p-button-sm"
            :class="config.enabled ? '' : 'p-button-secondary'"
            :title="config.enabled ? 'Disable Config' : 'Enable Config'"
            @click="toggleConfig(config)"
            :loading="actionLoading === `toggle_${config.id}`"
          />
          <Button
            icon="pi pi-trash"
            class="p-button-text p-button-danger p-button-sm"
            title="Delete Config"
            @click="confirmDelete(config)"
            :loading="actionLoading === `delete_${config.id}`"
          />
        </div>
      </div>
    </div>

    <!-- Logs Dialog -->
    <Dialog
      v-model:visible="showLogsDialog"
      :header="'Automation Logs - ' + (selectedConfigForLogs?.name || '')"
      :modal="true"
      :style="{ width: '700px', maxHeight: '80vh' }"
    >
      <div class="logs-container">
        <div v-if="logsLoading" class="logs-loading">
          <div class="loading-spinner"></div>
          <p>Loading logs...</p>
        </div>
        <div v-else-if="!automationLogs.length" class="logs-empty">
          <i class="pi pi-info-circle"></i>
          <p>No logs available. Start the automation to see activity.</p>
        </div>
        <div v-else class="logs-list">
          <div
            v-for="(log, idx) in automationLogs"
            :key="idx"
            class="log-entry"
            :class="'log-' + log.level"
          >
            <span class="log-time">{{ formatLogTime(log.timestamp) }}</span>
            <span class="log-level" :class="'level-' + log.level">{{ log.level.toUpperCase() }}</span>
            <span class="log-message">{{ log.message }}</span>
            <span v-if="log.details" class="log-details">{{ log.details }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <Button
          label="Refresh"
          icon="pi pi-refresh"
          class="p-button-text"
          @click="refreshLogs"
          :loading="logsLoading"
        />
        <Button
          label="Close"
          class="p-button-secondary"
          @click="showLogsDialog = false"
        />
      </template>
    </Dialog>

    <!-- Evaluation Result Dialog -->
    <Dialog
      v-model:visible="showEvalDialog"
      header="Indicator Evaluation Results"
      :modal="true"
      :style="{ width: '500px' }"
    >
      <div v-if="evalResult" class="eval-results">
        <div class="eval-summary" :class="{ 'all-passed': evalResult.all_passed }">
          <i :class="evalResult.all_passed ? 'pi pi-check-circle' : 'pi pi-times-circle'"></i>
          <span v-if="evalResult.group_results?.length > 1">
            {{ evalResult.all_passed
              ? `${evalResult.group_results.filter(g => g.pass).length} of ${evalResult.group_results.length} groups passing`
              : 'No groups passing' }}
          </span>
          <span v-else>{{ evalResult.all_passed ? 'All indicators passed' : 'Some indicators failed' }}</span>
        </div>

        <!-- Multi-group eval results -->
        <template v-if="evalResult.group_results?.length > 1">
          <div
            v-for="(gr, grIdx) in evalResult.group_results"
            :key="gr.group_id || grIdx"
          >
            <div v-if="grIdx > 0" class="or-divider-compact">
              <span class="or-divider-compact-label">OR</span>
            </div>
            <div class="eval-group-section">
              <div class="eval-group-header">
                <span class="eval-group-name">{{ gr.group_name || 'Group ' + (grIdx + 1) }}</span>
                <span class="group-result-badge" :class="gr.pass ? 'passed' : 'failed'">
                  {{ gr.pass ? '&#10003; Pass' : '&#10007; Fail' }}
                </span>
              </div>
              <div class="eval-details">
                <div
                  v-for="(result, idx) in gr.indicator_results"
                  :key="idx"
                  class="eval-item"
                  :class="{ 'passed': result.pass, 'failed': !result.pass }"
                >
                  <div class="eval-header">
                    <span class="eval-type">{{ formatIndicatorType(result.type) }}</span>
                    <span class="eval-status">
                      <i :class="result.pass ? 'pi pi-check' : 'pi pi-times'"></i>
                    </span>
                  </div>
                  <div class="eval-body">
                    <span class="eval-actual">Current: {{ formatIndicatorValue(result) }}</span>
                    <span class="eval-condition">{{ result.operator }} {{ result.threshold }}</span>
                  </div>
                  <div v-if="result.symbol" class="eval-symbol">
                    Symbol: {{ result.symbol }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- Flat eval results (single group or legacy) -->
        <template v-else>
          <div class="eval-details">
            <div
              v-for="(result, idx) in evalResult.results"
              :key="idx"
              class="eval-item"
              :class="{ 'passed': result.pass, 'failed': !result.pass }"
            >
              <div class="eval-header">
                <span class="eval-type">{{ formatIndicatorType(result.type) }}</span>
                <span class="eval-status">
                  <i :class="result.pass ? 'pi pi-check' : 'pi pi-times'"></i>
                </span>
              </div>
              <div class="eval-body">
                <span class="eval-actual">Current: {{ formatIndicatorValue(result) }}</span>
                <span class="eval-condition">{{ result.operator }} {{ result.threshold }}</span>
              </div>
              <div v-if="result.symbol" class="eval-symbol">
                Symbol: {{ result.symbol }}
              </div>
            </div>
          </div>
        </template>
      </div>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <Dialog
      v-model:visible="showDeleteDialog"
      header="Confirm Delete"
      :modal="true"
      :style="{ width: '400px' }"
    >
      <p>Are you sure you want to delete "{{ configToDelete?.name }}"?</p>
      <p class="text-muted">This action cannot be undone.</p>
      <template #footer>
        <Button
          label="Cancel"
          class="p-button-text"
          @click="showDeleteDialog = false"
        />
        <Button
          label="Delete"
          class="p-button-danger"
          @click="deleteConfig"
          :loading="actionLoading === `delete_${configToDelete?.id}`"
        />
      </template>
    </Dialog>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../services/api.js'
import webSocketClient from '../../services/webSocketClient.js'
import { useMobileDetection } from '../../composables/useMobileDetection.js'

export default {
  name: 'AutomationDashboard',
  setup() {
    const router = useRouter()
    const { isMobile } = useMobileDetection()

    // State
    const configs = ref([])
    const statuses = ref({})
    const isLoading = ref(true)
    const isRefreshing = ref(false)
    const error = ref(null)
    const actionLoading = ref(null)
    const expandedCards = ref(new Set()) // Track expanded cards on mobile
    
    // Dialogs
    const showEvalDialog = ref(false)
    const evalResult = ref(null)
    const showDeleteDialog = ref(false)
    const configToDelete = ref(null)
    
    // Logs state
    const showLogsDialog = ref(false)
    const selectedConfigForLogs = ref(null)
    const automationLogs = ref([])
    const logsLoading = ref(false)

    // Polling interval
    let statusInterval = null
    
    // WebSocket handler for automation updates
    const handleAutomationUpdate = (message) => {
      if (message.automation_id && message.data) {
        console.log('Received automation update:', message)
        // Update the status for this automation
        // If we're receiving updates, the automation is running
        const isActiveState = ['waiting', 'evaluating', 'trading', 'monitoring'].includes(message.data.status)
        statuses.value = {
          ...statuses.value,
          [message.automation_id]: {
            ...statuses.value[message.automation_id],
            ...message.data,
            state: message.data.status,
            is_running: isActiveState,
          }
        }
        
        // If we're viewing logs for this automation, update them
        if (selectedConfigForLogs.value?.id === message.automation_id && message.data.logs) {
          automationLogs.value = message.data.logs
        }
      }
    }

    // Methods
    const loadConfigs = async () => {
      try {
        error.value = null
        const response = await api.getAutomationConfigs()
        // Handle both response.data.configs (from API) and response.configs formats
        const configsData = response.data?.configs || response.configs || []
        // Extract config object from each item if nested and sort by name for consistent ordering
        const extractedConfigs = configsData.map(item => item.config || item)
        extractedConfigs.sort((a, b) => (a.name || '').localeCompare(b.name || ''))
        configs.value = extractedConfigs
      } catch (err) {
        console.error('Failed to load configs:', err)
        error.value = 'Failed to load automation configs'
      }
    }

    const loadStatuses = async () => {
      try {
        const response = await api.getAllAutomationStatuses()
        // Backend returns { data: { automations: [...] } }
        // Convert to a map keyed by config ID for easy lookup
        const automations = response.data?.automations || response.automations || []
        const statusMap = {}
        for (const item of automations) {
          if (item.config?.id) {
            statusMap[item.config.id] = {
              state: item.status || 'idle',
              is_running: item.is_running || false,
              ...item.details // Spread the full details (includes message, indicator_results, logs, etc.)
            }
          }
        }
        statuses.value = statusMap
      } catch (err) {
        console.error('Failed to load statuses:', err)
      }
    }

    const refreshData = async () => {
      isRefreshing.value = true
      try {
        await Promise.all([loadConfigs(), loadStatuses()])
      } finally {
        isRefreshing.value = false
      }
    }

    const getAutomationStatus = (configId) => {
      return statuses.value[configId]
    }

    const isConfigRunning = (configId) => {
      const status = getAutomationStatus(configId)
      if (!status) return false
      // Check both is_running flag and state
      if (status.is_running) return true
      // Also check state for active states
      const activeStates = ['waiting', 'evaluating', 'trading', 'monitoring']
      return activeStates.includes(status.state)
    }

    const getStatusClass = (config) => {
      if (!config.enabled) return 'disabled'
      if (isConfigRunning(config.id)) {
        const status = getAutomationStatus(config.id)
        return status?.all_indicators_pass === true ? 'running-ready' : 'running-not-ready'
      }
      return 'idle'
    }

    const getRunningStatusClass = (config) => {
      if (!config.enabled) return 'status-disabled'
      const status = getAutomationStatus(config.id)
      if (status) {
        switch (status.state) {
          case 'waiting':
          case 'evaluating':
            return status.all_indicators_pass === true ? 'status-running' : 'status-waiting'
          case 'trading':
          case 'monitoring':
            return 'status-running'
          case 'completed':
            return 'status-completed'
          case 'failed':
            return 'status-failed'
        }
      }
      return 'status-idle'
    }

    const getStatusIcon = (config) => {
      if (!config.enabled) return 'pi pi-ban'
      const status = getAutomationStatus(config.id)
      if (status) {
        switch (status.state) {
          case 'waiting':
            return 'pi pi-clock'
          case 'evaluating':
            return 'pi pi-search'
          case 'trading':
          case 'monitoring':
            return 'pi pi-spin pi-spinner'
          case 'completed':
            return 'pi pi-check-circle'
          case 'failed':
            return 'pi pi-times-circle'
        }
      }
      return 'pi pi-circle'
    }

    const getStatusText = (config) => {
      if (!config.enabled) return 'Disabled'
      const status = getAutomationStatus(config.id)
      if (status && status.state) {
        return status.state.charAt(0).toUpperCase() + status.state.slice(1)
      }
      return 'Idle'
    }

    const getEnabledIndicators = (config) => {
      if (config.indicator_groups?.length > 0) {
        return config.indicator_groups.flatMap(g => (g.indicators || []).filter(ind => ind.enabled))
      }
      return (config.indicators || []).filter(ind => ind.enabled)
    }

    const getIndicatorStatusClass = (config, indicator) => {
      const status = getAutomationStatus(config.id)
      if (!status || !status.indicator_results) return ''
      const result = status.indicator_results[indicator.type]
      if (result) {
        return result.passed ? 'indicator-passed' : 'indicator-failed'
      }
      return ''
    }

    const formatIndicatorType = (type) => {
      const types = {
        vix: 'VIX', gap: 'Gap', range: 'Range', trend: 'Trend', calendar: 'FOMC',
        rsi: 'RSI', macd: 'MACD', momentum: 'Momentum', cmo: 'CMO',
        stoch: 'Stoch', stoch_rsi: 'StochRSI', adx: 'ADX', cci: 'CCI',
        sma: 'SMA', ema: 'EMA', atr: 'ATR', bb_percent: 'BB%B'
      }
      return types[type] || type
    }

    const formatIndicatorCondition = (indicator) => {
      const operators = { gt: '>', lt: '<', eq: '=', ne: '!=' }
      return `${operators[indicator.operator] || indicator.operator} ${indicator.threshold}`
    }

    const formatIndicatorValue = (result) => {
      if (result.value === null || result.value === undefined) return 'N/A'
      if (typeof result.value === 'number') {
        // For stale results, show the cached value (if we have one)
        // The stale icon is already shown separately
        return result.value.toFixed(2)
      }
      return String(result.value)
    }

    const formatStrategy = (strategy) => {
      const strategies = {
        put_spread: 'Put Spread',
        call_spread: 'Call Spread',
        iron_condor: 'Iron Condor'
      }
      return strategies[strategy] || strategy || 'N/A'
    }

    const formatNumber = (num) => {
      if (num === null || num === undefined) return 'N/A'
      return num.toLocaleString()
    }

    const startAutomation = async (config) => {
      actionLoading.value = `start_${config.id}`
      try {
        await api.startAutomation(config.id)
        await loadStatuses()
      } catch (err) {
        console.error('Failed to start automation:', err)
        alert('Failed to start automation: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }

    const stopAutomation = async (config) => {
      actionLoading.value = `stop_${config.id}`
      try {
        await api.stopAutomation(config.id)
        await loadStatuses()
      } catch (err) {
        console.error('Failed to stop automation:', err)
        alert('Failed to stop automation: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }

    const evaluateIndicators = async (config) => {
      actionLoading.value = `eval_${config.id}`
      try {
        const response = await api.evaluateAutomationConfig(config.id)
        evalResult.value = {
          all_passed: response.data?.all_pass ?? false,
          results: response.data?.indicators || [],
          group_results: response.data?.group_results || [],
        }
        showEvalDialog.value = true
      } catch (err) {
        console.error('Failed to evaluate indicators:', err)
        alert('Failed to evaluate indicators: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }

    const toggleConfig = async (config) => {
      actionLoading.value = `toggle_${config.id}`
      try {
        await api.toggleAutomationConfig(config.id)
        await loadConfigs()
      } catch (err) {
        console.error('Failed to toggle config:', err)
        alert('Failed to toggle config: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }

    const editConfig = (config) => {
      router.push(`/automation/edit/${config.id}`)
    }

    const duplicateConfig = async (config) => {
      actionLoading.value = `duplicate_${config.id}`
      try {
        // Create a copy of the config without the id
        const newConfig = {
          name: `${config.name} (Copy)`,
          description: config.description,
          symbol: config.symbol,
          indicators: config.indicators || [],
          indicator_groups: config.indicator_groups || [],
          entry_time: config.entry_time,
          entry_timezone: config.entry_timezone,
          recurrence: config.recurrence,
          enabled: false, // Start disabled so user can review
          trade_config: config.trade_config
        }
        
        const response = await api.createAutomationConfig(newConfig)
        
        // Refresh the list to show the new config
        await loadConfigs()
        
        // Navigate to edit the new config so user can customize it
        if (response.data?.id) {
          router.push(`/automation/edit/${response.data.id}`)
        }
      } catch (err) {
        console.error('Failed to duplicate config:', err)
        alert('Failed to duplicate config: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }

    const resetTradedToday = async (config) => {
      actionLoading.value = `reset_${config.id}`
      try {
        await api.resetAutomationTradedToday(config.id)
        await loadStatuses()
      } catch (err) {
        console.error('Failed to reset traded today:', err)
        alert('Failed to reset: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }

    const confirmDelete = (config) => {
      configToDelete.value = config
      showDeleteDialog.value = true
    }

    const deleteConfig = async () => {
      if (!configToDelete.value) return
      actionLoading.value = `delete_${configToDelete.value.id}`
      try {
        await api.deleteAutomationConfig(configToDelete.value.id)
        showDeleteDialog.value = false
        configToDelete.value = null
        await loadConfigs()
      } catch (err) {
        console.error('Failed to delete config:', err)
        alert('Failed to delete config: ' + (err.response?.data?.message || err.message))
      } finally {
        actionLoading.value = null
      }
    }
    
    // Logs methods
    const showLogs = async (config) => {
      selectedConfigForLogs.value = config
      showLogsDialog.value = true
      await refreshLogs()
    }
    
    const refreshLogs = async () => {
      if (!selectedConfigForLogs.value) return
      logsLoading.value = true
      try {
        const response = await api.getAutomationLogs(selectedConfigForLogs.value.id)
        automationLogs.value = response.data?.logs || []
      } catch (err) {
        console.error('Failed to load logs:', err)
        automationLogs.value = []
      } finally {
        logsLoading.value = false
      }
    }
    
    const formatLogTime = (timestamp) => {
      if (!timestamp) return ''
      const date = new Date(timestamp)
      return date.toLocaleTimeString('en-US', { 
        hour: '2-digit', 
        minute: '2-digit', 
        second: '2-digit',
        hour12: false
      })
    }

    // Mobile card expand/collapse functions
    const toggleCardExpanded = (configId) => {
      if (expandedCards.value.has(configId)) {
        expandedCards.value.delete(configId)
      } else {
        expandedCards.value.add(configId)
      }
      // Force reactivity
      expandedCards.value = new Set(expandedCards.value)
    }

    const isCardExpanded = (configId) => {
      return expandedCards.value.has(configId)
    }

    // Lifecycle
    onMounted(async () => {
      isLoading.value = true
      try {
        await Promise.all([loadConfigs(), loadStatuses()])
      } finally {
        isLoading.value = false
      }

      // Poll for status updates every 5 seconds
      statusInterval = setInterval(loadStatuses, 5000)
      
      // Register WebSocket callback for automation updates
      webSocketClient.addCallback('automation_update', handleAutomationUpdate)
    })

    onUnmounted(() => {
      if (statusInterval) {
        clearInterval(statusInterval)
      }
      // Clean up WebSocket callback to prevent memory leaks
      webSocketClient.removeCallback('automation_update', handleAutomationUpdate)
    })

    return {
      // State
      configs,
      statuses,
      isLoading,
      isRefreshing,
      error,
      actionLoading,
      
      // Dialogs
      showEvalDialog,
      evalResult,
      showDeleteDialog,
      configToDelete,
      
      // Logs
      showLogsDialog,
      selectedConfigForLogs,
      automationLogs,
      logsLoading,

      // Methods
      refreshData,
      getAutomationStatus,
      isConfigRunning,
      getStatusClass,
      getRunningStatusClass,
      getStatusIcon,
      getStatusText,
      getEnabledIndicators,
      getIndicatorStatusClass,
      formatIndicatorType,
      formatIndicatorCondition,
      formatIndicatorValue,
      formatStrategy,
      formatNumber,
      startAutomation,
      stopAutomation,
      evaluateIndicators,
      toggleConfig,
      editConfig,
      duplicateConfig,
      resetTradedToday,
      confirmDelete,
      deleteConfig,
      showLogs,
      refreshLogs,
      formatLogTime,
      
      // Mobile
      isMobile,
      expandedCards,
      toggleCardExpanded,
      isCardExpanded,
    }
  }
}
</script>

<style scoped>
.automation-dashboard {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header Section */
.dashboard-header {
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

.dashboard-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.dashboard-title i {
  color: var(--color-brand);
}

.dashboard-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
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

/* Configs Grid */
.configs-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: var(--spacing-lg);
}

.config-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  transition: var(--transition-normal);
}

.config-card:hover {
  border-color: var(--border-secondary);
  box-shadow: var(--shadow-md);
}

.config-card.running-ready {
  border-left: 4px solid var(--color-success);
}

.config-card.running-not-ready {
  border-left: 4px solid var(--color-warning);
}

.config-card.idle {
  border-left: 4px solid var(--color-info);
}

.config-card.disabled {
  border-left: 4px solid var(--text-tertiary);
  opacity: 0.7;
}

/* Config Header */
.config-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-md);
}

.config-info {
  flex: 1;
  min-width: 0;
}

.config-name {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.config-meta {
  display: flex;
  gap: var(--spacing-sm);
}

.config-symbol,
.config-strategy {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.config-recurrence {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: var(--font-size-xs);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-weight: var(--font-weight-medium);
}

.config-recurrence.recurrence-once {
  background: var(--bg-tertiary);
  color: var(--text-tertiary);
}

.config-recurrence.recurrence-daily {
  background: rgba(59, 130, 246, 0.15);
  color: var(--color-info);
}

.config-recurrence i {
  font-size: var(--font-size-xs);
}

.status-badge {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
}

.status-running {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.status-waiting {
  background: rgba(251, 191, 36, 0.1);
  color: var(--color-warning);
}

.status-completed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.status-failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.status-idle {
  background: rgba(59, 130, 246, 0.1);
  color: var(--color-info);
}

.status-disabled {
  background: rgba(107, 114, 128, 0.1);
  color: var(--text-tertiary);
}

/* Indicators Section */
.indicators-section {
  margin-bottom: var(--spacing-md);
}

.section-label {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: var(--spacing-xs);
}

.indicators-grid {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
}

.indicator-chip {
  display: flex;
  gap: var(--spacing-xs);
  padding: 4px 8px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.indicator-chip.indicator-passed {
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid var(--color-success);
}

.indicator-chip.indicator-failed {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--color-danger);
}

.indicator-name {
  color: var(--text-secondary);
}

.indicator-value {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

/* Trade Summary */
.trade-summary {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-md);
}

.summary-item {
  display: flex;
  flex-direction: column;
}

.summary-label {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.summary-value {
  font-size: var(--font-size-sm);
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

/* Status Details */
.status-details {
  padding: var(--spacing-sm);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-md);
  font-size: var(--font-size-sm);
}

.status-row {
  display: flex;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-xs);
}

.status-label {
  color: var(--text-tertiary);
}

.status-value {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
}

.traded-today-status {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.traded-today-status.traded {
  color: var(--color-success);
}

.traded-today-status:not(.traded) {
  color: var(--color-warning);
}

.results-label {
  color: var(--text-tertiary);
  font-size: var(--font-size-xs);
  margin-bottom: var(--spacing-xs);
}

.results-grid {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
}

.result-chip {
  display: flex;
  gap: var(--spacing-xs);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
}

.result-chip.passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.result-chip.failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.result-chip.stale {
  background: rgba(251, 191, 36, 0.15);
  color: var(--color-warning);
  border: 1px dashed var(--color-warning);
}

.result-chip .stale-icon {
  font-size: 10px;
}

/* Config Actions */
.config-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-xs);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

/* Evaluation Dialog */
.eval-results {
  padding: var(--spacing-md);
}

.eval-summary {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  background: rgba(239, 68, 68, 0.1);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-md);
  color: var(--color-danger);
}

.eval-summary.all-passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.eval-summary i {
  font-size: var(--font-size-xl);
}

.eval-details {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.eval-item {
  padding: var(--spacing-sm);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border-left: 3px solid var(--color-danger);
}

.eval-item.passed {
  border-left-color: var(--color-success);
}

.eval-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.eval-type {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.eval-status i {
  font-size: var(--font-size-lg);
}

.eval-item.passed .eval-status i {
  color: var(--color-success);
}

.eval-item.failed .eval-status i {
  color: var(--color-danger);
}

.eval-body {
  display: flex;
  justify-content: space-between;
  font-size: var(--font-size-sm);
}

.eval-actual {
  color: var(--text-primary);
}

.eval-condition {
  color: var(--text-secondary);
}

.eval-symbol {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  margin-top: var(--spacing-xs);
}

.text-muted {
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
}

/* Logs Dialog */
.logs-container {
  max-height: 400px;
  overflow-y: auto;
}

.logs-loading,
.logs-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--text-secondary);
}

.logs-empty i {
  font-size: var(--font-size-2xl);
  color: var(--text-tertiary);
  margin-bottom: var(--spacing-md);
}

.logs-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.log-entry {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm);
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-family: monospace;
  border-left: 3px solid var(--border-primary);
}

.log-entry.log-info {
  border-left-color: var(--color-info);
}

.log-entry.log-warn {
  border-left-color: var(--color-warning);
  background: rgba(251, 191, 36, 0.05);
}

.log-entry.log-error {
  border-left-color: var(--color-danger);
  background: rgba(239, 68, 68, 0.05);
}

.log-time {
  color: var(--text-tertiary);
  min-width: 70px;
}

.log-level {
  font-weight: var(--font-weight-semibold);
  min-width: 45px;
  text-transform: uppercase;
  font-size: var(--font-size-xs);
}

.level-info {
  color: var(--color-info);
}

.level-warn {
  color: var(--color-warning);
}

.level-error {
  color: var(--color-danger);
}

.log-message {
  flex: 1;
  color: var(--text-primary);
}

.log-details {
  width: 100%;
  padding-left: calc(70px + 45px + var(--spacing-sm) * 2);
  color: var(--text-secondary);
  font-size: var(--font-size-xs);
}

/* Config Header Right (status + expand button) */
.config-header-right {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.expand-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  cursor: pointer;
  transition: var(--transition-fast);
}

.expand-toggle:hover {
  background: var(--bg-quaternary, var(--border-primary));
  color: var(--text-primary);
}

/* Indicator Group Dashboard Container */
.indicator-group-dashboard {
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  padding: var(--spacing-sm);
}

.group-label {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
  margin-bottom: var(--spacing-xs);
}

.group-header-dashboard {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.group-result-badge {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  white-space: nowrap;
}

.group-result-badge.passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.group-result-badge.failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

/* Compact OR Divider for Dashboard */
.or-divider-compact {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin: var(--spacing-sm) 0;
}

.or-divider-compact::before,
.or-divider-compact::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border-primary);
}

.or-divider-compact-label {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  color: var(--color-brand);
  text-transform: uppercase;
  letter-spacing: 1px;
}

/* Overall Status Line */
.overall-status {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  margin-top: var(--spacing-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
}

.overall-status.passing {
  color: var(--color-success);
  background: rgba(34, 197, 94, 0.1);
}

.overall-status.failing {
  color: var(--color-danger);
  background: rgba(239, 68, 68, 0.1);
}

/* Eval Dialog Group Sections */
.eval-group-section {
  margin-bottom: var(--spacing-sm);
}

.eval-group-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-sm);
  padding-bottom: var(--spacing-xs);
  border-bottom: 1px solid var(--border-primary);
}

.eval-group-name {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

/* Responsive */
@media (max-width: 768px) {
  .automation-dashboard {
    padding: var(--spacing-md);
  }

  .dashboard-header {
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .configs-grid {
    grid-template-columns: 1fr;
  }

  .trade-summary {
    grid-template-columns: 1fr;
  }

  /* Mobile Card Styles */
  .config-card {
    padding: var(--spacing-md);
  }

  .config-card.mobile-collapsed {
    padding-bottom: var(--spacing-sm);
  }

  .config-header {
    cursor: pointer;
  }

  .config-card.mobile-collapsed .config-header {
    margin-bottom: var(--spacing-xs);
  }

  .config-name {
    font-size: var(--font-size-md);
  }

  .config-meta {
    flex-wrap: wrap;
    gap: var(--spacing-xs);
  }

  .config-symbol,
  .config-strategy,
  .config-recurrence {
    font-size: var(--font-size-xs);
    padding: 2px 6px;
  }

  /* Status badge - icon only on mobile */
  .status-badge {
    padding: var(--spacing-xs);
  }

  /* Mobile collapsed card - compact actions */
  .config-card.mobile-collapsed .config-actions {
    padding-top: var(--spacing-sm);
    border-top: none;
    justify-content: flex-start;
    flex-wrap: wrap;
    gap: 4px;
  }

  /* Make action buttons smaller on mobile collapsed view */
  .config-card.mobile-collapsed .config-actions .p-button {
    padding: 6px;
    min-width: unset;
  }

  /* Hide less important actions on mobile collapsed view */
  .config-card.mobile-collapsed .config-actions .p-button[title="Duplicate Config"],
  .config-card.mobile-collapsed .config-actions .p-button[title="Test Indicators"] {
    display: none;
  }

  /* When expanded, show all actions */
  .config-card:not(.mobile-collapsed) .config-actions {
    flex-wrap: wrap;
    justify-content: flex-start;
  }
}
</style>
