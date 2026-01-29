<template>
  <div class="automation-config-form">
    <!-- Header -->
    <div class="form-header">
      <div class="header-content">
        <h1 class="form-title">
          <i class="pi pi-cog"></i>
          {{ isEditMode ? 'Edit' : 'Create' }} Automation Config
        </h1>
        <p class="form-subtitle">
          Configure automated credit spread trading criteria
        </p>
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-times"
          label="Cancel"
          class="p-button-text"
          @click="cancel"
        />
        <Button
          icon="pi pi-check"
          label="Save Config"
          class="p-button-primary"
          @click="saveConfig"
          :loading="isSaving"
        />
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading-state">
      <div class="loading-spinner"></div>
      <p>Loading config...</p>
    </div>

    <!-- Form Content -->
    <div v-else class="form-content">
      <!-- Basic Info Section -->
      <div class="form-section">
        <h2 class="section-title">Basic Information</h2>
        <div class="form-grid">
          <div class="form-field">
            <label for="name">Config Name</label>
            <InputText
              id="name"
              v-model="config.name"
              placeholder="e.g., NDX Morning Put Spread"
              :class="{ 'p-invalid': errors.name }"
            />
            <small v-if="errors.name" class="p-error">{{ errors.name }}</small>
          </div>
          <div class="form-field">
            <label for="symbol">Symbol</label>
            <Dropdown
              id="symbol"
              v-model="config.symbol"
              :options="symbols"
              placeholder="Select Symbol"
              :class="{ 'p-invalid': errors.symbol }"
            />
            <small v-if="errors.symbol" class="p-error">{{ errors.symbol }}</small>
          </div>
        </div>
      </div>

      <!-- Entry Time Section -->
      <div class="form-section">
        <h2 class="section-title">Entry Time & Schedule</h2>
        <div class="form-grid">
          <div class="form-field">
            <label for="entryTime">Entry Time</label>
            <InputText
              id="entryTime"
              v-model="config.entry_time"
              placeholder="HH:MM (e.g., 12:25)"
              :class="{ 'p-invalid': errors.entry_time }"
            />
            <small class="field-hint">24-hour format (e.g., 12:25 for 12:25 PM)</small>
            <small v-if="errors.entry_time" class="p-error">{{ errors.entry_time }}</small>
          </div>
          <div class="form-field">
            <label for="timezone">Timezone</label>
            <Dropdown
              id="timezone"
              v-model="config.entry_timezone"
              :options="timezones"
              placeholder="Select Timezone"
            />
          </div>
          <div class="form-field">
            <label for="recurrence">Recurrence</label>
            <Dropdown
              id="recurrence"
              v-model="config.recurrence"
              :options="recurrenceOptions"
              optionLabel="label"
              optionValue="value"
              placeholder="Select Recurrence"
            />
            <small class="field-hint">{{ getRecurrenceHint() }}</small>
          </div>
        </div>
      </div>

      <!-- Indicators Section -->
      <div class="form-section">
        <h2 class="section-title">Entry Indicators</h2>
        <p class="section-description">
          All enabled indicators must pass for a trade to be executed
        </p>
        
        <div class="indicators-list" :class="{ 'mobile-indicators': isMobile }">
          <div
            v-for="(indicator, index) in config.indicators"
            :key="indicator.type"
            class="indicator-row"
            :class="{ 'disabled': !indicator.enabled }"
          >
            <div class="indicator-toggle">
              <InputSwitch v-model="indicator.enabled" />
            </div>
            <div class="indicator-type">
              <span class="type-label">{{ formatIndicatorType(indicator.type) }}</span>
              <span v-if="!isMobile" class="type-description">{{ getIndicatorDescription(indicator.type) }}</span>
            </div>
            <div class="indicator-config" :class="{ 'dimmed': !indicator.enabled }">
              <Dropdown
                v-model="indicator.operator"
                :options="displayOperators"
                optionLabel="label"
                optionValue="value"
                class="operator-dropdown"
                :class="{ 'mobile-operator': isMobile }"
                :disabled="!indicator.enabled"
              />
              <InputNumber
                v-model="indicator.threshold"
                :minFractionDigits="indicator.type === 'calendar' ? 0 : 2"
                :maxFractionDigits="indicator.type === 'calendar' ? 0 : 2"
                class="threshold-input"
                :class="{ 'mobile-threshold': isMobile }"
                :disabled="!indicator.enabled"
                :placeholder="indicator.type === 'calendar' ? '0 or 1' : 'Value'"
              />
              <InputText
                v-if="indicator.type !== 'calendar'"
                v-model="indicator.symbol"
                :placeholder="getIndicatorDefaultSymbol(indicator.type)"
                class="symbol-input"
                :class="{ 'mobile-symbol': isMobile }"
                :disabled="!indicator.enabled"
              />
              <!-- Mobile test result - shown inline after Test All -->
              <span 
                v-if="isMobile && indicatorResults[indicator.type]" 
                class="mobile-test-result" 
                :class="getIndicatorResultClass(indicatorResults[indicator.type])"
                :title="indicatorResults[indicator.type].stale ? `Stale: ${indicatorResults[indicator.type].error}` : ''"
              >
                <span v-if="indicatorResults[indicator.type].stale" class="stale-icon">⚠</span>
                {{ indicatorResults[indicator.type].value?.toFixed(2) || 'N/A' }}
              </span>
            </div>
            <div v-if="!isMobile" class="indicator-test">
              <Button
                icon="pi pi-sync"
                class="p-button-text p-button-sm"
                title="Test this indicator"
                @click="testIndicator(indicator)"
                :loading="testingIndicator === indicator.type"
                :disabled="!indicator.enabled"
              />
              <span 
                v-if="indicatorResults[indicator.type]" 
                class="test-result" 
                :class="getIndicatorResultClass(indicatorResults[indicator.type])"
                :title="indicatorResults[indicator.type].stale ? `Stale: ${indicatorResults[indicator.type].error}` : ''"
              >
                <span v-if="indicatorResults[indicator.type].stale" class="stale-icon">⚠</span>
                {{ indicatorResults[indicator.type].value?.toFixed(2) || 'N/A' }}
              </span>
            </div>
          </div>
        </div>

        <div class="test-all-section">
          <Button
            icon="pi pi-play"
            label="Test All Indicators"
            class="p-button-outlined"
            @click="testAllIndicators"
            :loading="testingAll"
          />
          <span v-if="allIndicatorsResult !== null" class="all-result" :class="allIndicatorsResult ? 'passed' : 'failed'">
            {{ allIndicatorsResult ? 'All Passed' : 'Some Failed' }}
          </span>
        </div>
      </div>

      <!-- Trade Configuration Section -->
      <div class="form-section">
        <h2 class="section-title">Trade Configuration</h2>
        
        <div class="form-grid">
          <div class="form-field">
            <label for="strategy">Strategy</label>
            <Dropdown
              id="strategy"
              v-model="config.trade_config.strategy"
              :options="strategies"
              optionLabel="label"
              optionValue="value"
              placeholder="Select Strategy"
            />
          </div>
          <div class="form-field">
            <label for="targetDelta">Target Delta</label>
            <InputNumber
              id="targetDelta"
              v-model="config.trade_config.target_delta"
              :minFractionDigits="2"
              :maxFractionDigits="3"
              :min="0.01"
              :max="0.50"
              placeholder="e.g., 0.05"
            />
            <small class="field-hint">Short strike delta (e.g., 0.05 = 5 delta)</small>
          </div>
          <div class="form-field">
            <label for="width">Spread Width</label>
            <InputNumber
              id="width"
              v-model="config.trade_config.width"
              :min="5"
              :max="100"
              placeholder="e.g., 20"
            />
            <small class="field-hint">Strike width in points</small>
          </div>
          <div class="form-field">
            <label for="maxCapital">Max Capital</label>
            <InputNumber
              id="maxCapital"
              v-model="config.trade_config.max_capital"
              mode="currency"
              currency="USD"
              :min="100"
              placeholder="e.g., 5000"
            />
            <small class="field-hint">Maximum capital to risk on this trade</small>
          </div>
        </div>

        <h3 class="subsection-title">Order Settings</h3>
        <div class="form-grid">
          <div class="form-field">
            <label for="orderType">Order Type</label>
            <Dropdown
              id="orderType"
              v-model="config.trade_config.order_type"
              :options="orderTypes"
              optionLabel="label"
              optionValue="value"
            />
          </div>
          <div class="form-field">
            <label for="timeInForce">Time in Force</label>
            <Dropdown
              id="timeInForce"
              v-model="config.trade_config.time_in_force"
              :options="timeInForceOptions"
              optionLabel="label"
              optionValue="value"
            />
          </div>
        </div>

        <h3 class="subsection-title">Price Ladder Settings</h3>
        <div class="form-grid">
          <div class="form-field">
            <label for="startingOffset">Starting Price Offset</label>
            <InputNumber
              id="startingOffset"
              v-model="config.trade_config.starting_offset"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              :min="0"
              :max="2.00"
              placeholder="e.g., 0.10"
            />
            <small class="field-hint">Amount below mid to start (e.g., 0.10 for more aggressive fill)</small>
          </div>
          <div class="form-field">
            <label for="minCredit">Minimum Credit</label>
            <InputNumber
              id="minCredit"
              v-model="config.trade_config.min_credit"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              :min="0"
              :max="10.00"
              placeholder="e.g., 0.30"
            />
            <small class="field-hint">Stop if credit falls below this threshold</small>
          </div>
          <div class="form-field">
            <label for="priceLadderStep">Price Ladder Step</label>
            <InputNumber
              id="priceLadderStep"
              v-model="config.trade_config.price_ladder_step"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              :min="0.01"
              :max="1.00"
              placeholder="e.g., 0.05"
            />
            <small class="field-hint">Increment for price improvement attempts</small>
          </div>
          <div class="form-field">
            <label for="maxAttempts">Max Attempts</label>
            <InputNumber
              id="maxAttempts"
              v-model="config.trade_config.max_attempts"
              :min="1"
              :max="50"
              placeholder="e.g., 10"
            />
            <small class="field-hint">Maximum price improvement attempts</small>
          </div>
          <div class="form-field">
            <label for="attemptInterval">Attempt Interval (sec)</label>
            <InputNumber
              id="attemptInterval"
              v-model="config.trade_config.attempt_interval"
              :min="5"
              :max="300"
              placeholder="e.g., 30"
            />
            <small class="field-hint">Seconds between attempts</small>
          </div>
          <div class="form-field">
            <label for="deltaDriftLimit">Delta Drift Limit</label>
            <InputNumber
              id="deltaDriftLimit"
              v-model="config.trade_config.delta_drift_limit"
              :minFractionDigits="2"
              :maxFractionDigits="3"
              :min="0.001"
              :max="0.10"
              placeholder="e.g., 0.01"
            />
            <small class="field-hint">Max delta change before re-selecting strikes</small>
          </div>
        </div>

        <h3 class="subsection-title">Expiration Settings</h3>
        <div class="form-grid">
          <div class="form-field">
            <label for="expirationMode">Expiration Mode</label>
            <Dropdown
              id="expirationMode"
              v-model="config.trade_config.expiration_mode"
              :options="expirationModes"
              optionLabel="label"
              optionValue="value"
              placeholder="Select Expiration"
            />
            <small class="field-hint">Select which expiration to trade</small>
          </div>
          <div class="form-field" v-if="config.trade_config.expiration_mode === 'custom'">
            <label for="customExpiration">Custom Expiration Date</label>
            <InputText
              id="customExpiration"
              v-model="config.trade_config.custom_expiration"
              placeholder="YYYY-MM-DD"
            />
            <small class="field-hint">Specific date in YYYY-MM-DD format</small>
          </div>
        </div>

        <!-- Strike Preview Section -->
        <h3 class="subsection-title">Strike Preview</h3>
        <div class="strike-preview-section">
          <Button
            icon="pi pi-search"
            label="Preview Strikes"
            class="p-button-outlined"
            @click="loadStrikePreview"
            :loading="loadingPreview"
            :disabled="!config.symbol || !config.trade_config.strategy"
          />
          
          <div v-if="strikePreview" class="strike-preview-content" :class="{ 'mobile-preview': isMobile }">
            <div class="preview-expiry">
              <strong>Exp:</strong> {{ strikePreview.spread?.expiry || 'N/A' }}
            </div>
            
            <div class="preview-legs">
              <div class="preview-leg short-leg">
                <h4>Short (Sell)</h4>
                <div class="leg-details">
                  <div class="leg-row">
                    <span class="leg-label">Strike:</span>
                    <span class="leg-value">{{ strikePreview.short_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                  </div>
                  <div class="leg-row">
                    <span class="leg-label">Delta:</span>
                    <span class="leg-value">{{ strikePreview.short_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                  </div>
                  <div class="leg-row">
                    <span class="leg-label">Bid/Ask:</span>
                    <span class="leg-value">{{ formatPrice(strikePreview.short_leg?.bid) }}/{{ formatPrice(strikePreview.short_leg?.ask) }}</span>
                  </div>
                  <div class="leg-row">
                    <span class="leg-label">Mid:</span>
                    <span class="leg-value highlight">{{ formatPrice(strikePreview.short_leg?.mid) }}</span>
                  </div>
                </div>
              </div>
              
              <div class="preview-leg long-leg">
                <h4>Long (Buy)</h4>
                <div class="leg-details">
                  <div class="leg-row">
                    <span class="leg-label">Strike:</span>
                    <span class="leg-value">{{ strikePreview.long_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                  </div>
                  <div class="leg-row">
                    <span class="leg-label">Delta:</span>
                    <span class="leg-value">{{ strikePreview.long_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                  </div>
                  <div class="leg-row">
                    <span class="leg-label">Bid/Ask:</span>
                    <span class="leg-value">{{ formatPrice(strikePreview.long_leg?.bid) }}/{{ formatPrice(strikePreview.long_leg?.ask) }}</span>
                  </div>
                  <div class="leg-row">
                    <span class="leg-label">Mid:</span>
                    <span class="leg-value highlight">{{ formatPrice(strikePreview.long_leg?.mid) }}</span>
                  </div>
                </div>
              </div>
            </div>
            
            <div class="preview-spread">
              <h4>Spread Summary</h4>
              <div class="spread-details">
                <div class="spread-row">
                  <span class="spread-label">Natural:</span>
                  <span class="spread-value">{{ formatPrice(strikePreview.spread?.natural_credit) }}</span>
                </div>
                <div class="spread-row">
                  <span class="spread-label">Mid Credit:</span>
                  <span class="spread-value highlight">{{ formatPrice(strikePreview.spread?.mid_credit) }}</span>
                </div>
                <div class="spread-row" v-if="config.trade_config.starting_offset > 0">
                  <span class="spread-label">Start Price:</span>
                  <span class="spread-value highlight-green">{{ formatPrice(calculateStartingPrice()) }}</span>
                </div>
              </div>
            </div>
          </div>
          
          <div v-if="previewError" class="preview-error">
            {{ previewError }}
          </div>
        </div>
      </div>

      <!-- Enable/Disable Section -->
      <div class="form-section">
        <div class="enable-toggle">
          <InputSwitch v-model="config.enabled" />
          <span class="enable-label">
            {{ config.enabled ? 'Config Enabled' : 'Config Disabled' }}
          </span>
          <span class="enable-hint">
            {{ config.enabled ? 'This config will be available for automation' : 'This config will not be used for automation' }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '../../services/api.js'
import { useMobileDetection } from '../../composables/useMobileDetection.js'

export default {
  name: 'AutomationConfigForm',
  setup() {
    const router = useRouter()
    const route = useRoute()
    const { isMobile } = useMobileDetection()

    // Check if we're in edit mode
    const isEditMode = computed(() => !!route.params.id)
    const configId = computed(() => route.params.id)

    // State
    const isLoading = ref(false)
    const isSaving = ref(false)
    const errors = ref({})
    
    // Indicator testing
    const testingIndicator = ref(null)
    const testingAll = ref(false)
    const indicatorResults = ref({})
    const allIndicatorsResult = ref(null)

    // Strike preview
    const loadingPreview = ref(false)
    const strikePreview = ref(null)
    const previewError = ref(null)

    // Config data
    const config = ref({
      name: '',
      symbol: 'NDX',
      entry_time: '12:25',
      entry_timezone: 'America/New_York',
      recurrence: 'once',
      enabled: true,
      indicators: [
        { type: 'vix', enabled: true, operator: 'gt', threshold: 13.5, symbol: 'VIX' },
        { type: 'gap', enabled: true, operator: 'lt', threshold: 1.0, symbol: 'QQQ' },
        { type: 'range', enabled: true, operator: 'lt', threshold: 2.5, symbol: 'QQQ' },
        { type: 'trend', enabled: true, operator: 'lt', threshold: 1.5, symbol: 'QQQ' },
        { type: 'calendar', enabled: true, operator: 'eq', threshold: 0, symbol: '' },
      ],
      trade_config: {
        strategy: 'put_spread',
        width: 20,
        target_delta: 0.05,
        max_capital: 5000,
        order_type: 'limit',
        time_in_force: 'day',
        price_ladder_step: 0.05,
        max_attempts: 10,
        attempt_interval: 30,
        delta_drift_limit: 0.01,
        starting_offset: 0.10,
        min_credit: 0.30,
        expiration_mode: '0dte',
        custom_expiration: '',
      }
    })

    // Options
    const symbols = ['NDX', 'SPX']
    const timezones = ['America/New_York', 'America/Chicago', 'America/Los_Angeles', 'UTC']
    const operators = [
      { label: '> (greater than)', value: 'gt' },
      { label: '< (less than)', value: 'lt' },
      { label: '= (equals)', value: 'eq' },
      { label: '!= (not equals)', value: 'ne' },
    ]
    const mobileOperators = [
      { label: '>', value: 'gt' },
      { label: '<', value: 'lt' },
      { label: '=', value: 'eq' },
      { label: '!=', value: 'ne' },
    ]
    // Use symbols-only operators on mobile
    const displayOperators = computed(() => isMobile.value ? mobileOperators : operators)
    const strategies = [
      { label: 'Put Credit Spread', value: 'put_spread' },
      { label: 'Call Credit Spread', value: 'call_spread' },
    ]
    const orderTypes = [
      { label: 'Limit', value: 'limit' },
      { label: 'Market', value: 'market' },
    ]
    const timeInForceOptions = [
      { label: 'Day', value: 'day' },
      { label: 'GTC (Good Till Cancel)', value: 'gtc' },
    ]
    const expirationModes = [
      { label: '0DTE (Same Day)', value: '0dte' },
      { label: '1DTE (Next Day)', value: '1dte' },
      { label: '2DTE (Two Days Out)', value: '2dte' },
      { label: 'Custom Date', value: 'custom' },
    ]
    const recurrenceOptions = [
      { label: 'Run Once', value: 'once' },
      { label: 'Daily (Repeat Each Day)', value: 'daily' },
    ]

    // Methods
    const formatIndicatorType = (type) => {
      const types = {
        vix: 'VIX Level',
        gap: 'Gap %',
        range: 'Range %',
        trend: 'Trend %',
        calendar: 'FOMC Calendar'
      }
      return types[type] || type
    }

    const getIndicatorDescription = (type) => {
      const descriptions = {
        vix: 'CBOE Volatility Index level',
        gap: '(Open - Prev Close) / Prev Close * 100',
        range: '(High - Low) / Open * 100',
        trend: '(Current - Open) / Open * 100',
        calendar: '0 = not FOMC day, 1 = FOMC day'
      }
      return descriptions[type] || ''
    }

    const getIndicatorDefaultSymbol = (type) => {
      const defaults = {
        vix: 'VIX',
        gap: 'QQQ',
        range: 'QQQ',
        trend: 'QQQ',
        calendar: ''
      }
      return defaults[type] || ''
    }

    const getRecurrenceHint = () => {
      if (config.value.recurrence === 'daily') {
        return 'Will automatically reset and trade again each trading day'
      }
      return 'Will stop after one successful trade'
    }

    const getIndicatorResultClass = (result) => {
      if (result.stale) return 'stale'
      return result.passed ? 'passed' : 'failed'
    }

    const loadConfig = async () => {
      if (!configId.value) return
      
      isLoading.value = true
      try {
        const response = await api.getAutomationConfig(configId.value)
        // Handle nested response format: response.data.config or response.config
        const configData = response.data?.config || response.config
        if (configData) {
          // Merge with defaults to ensure all fields exist
          config.value = {
            ...config.value,
            ...configData,
            trade_config: {
              ...config.value.trade_config,
              ...configData.trade_config
            }
          }
        }
      } catch (err) {
        console.error('Failed to load config:', err)
        alert('Failed to load config: ' + (err.response?.data?.message || err.message))
      } finally {
        isLoading.value = false
      }
    }

    const validateConfig = () => {
      errors.value = {}
      
      if (!config.value.name?.trim()) {
        errors.value.name = 'Config name is required'
      }
      
      if (!config.value.symbol) {
        errors.value.symbol = 'Symbol is required'
      }
      
      if (!config.value.entry_time?.match(/^\d{1,2}:\d{2}$/)) {
        errors.value.entry_time = 'Invalid time format (use HH:MM)'
      }
      
      return Object.keys(errors.value).length === 0
    }

    const saveConfig = async () => {
      if (!validateConfig()) return
      
      isSaving.value = true
      try {
        // Clean up indicator symbols (remove empty ones for calendar)
        const cleanedConfig = {
          ...config.value,
          indicators: config.value.indicators.map(ind => ({
            ...ind,
            symbol: ind.symbol?.trim() || undefined
          }))
        }
        
        if (isEditMode.value) {
          await api.updateAutomationConfig(configId.value, cleanedConfig)
        } else {
          await api.createAutomationConfig(cleanedConfig)
        }
        router.push('/automation')
      } catch (err) {
        console.error('Failed to save config:', err)
        alert('Failed to save config: ' + (err.response?.data?.message || err.message))
      } finally {
        isSaving.value = false
      }
    }

    const cancel = () => {
      router.push('/automation')
    }

    const testIndicator = async (indicator) => {
      testingIndicator.value = indicator.type
      try {
        const response = await api.previewAutomationIndicators({
          indicators: [indicator]
        })
        // Handle response format: data.indicators is an array
        const indicators = response.data?.indicators || response.indicators || []
        const result = indicators.find(ind => ind.type === indicator.type)
        if (result) {
          // Map API format to our expected format
          indicatorResults.value[indicator.type] = {
            value: result.value,
            passed: result.pass,
            operator: result.operator,
            threshold: result.threshold,
            symbol: result.symbol,
            details: result.details,
            stale: result.stale || false,
            error: result.error || ''
          }
        }
      } catch (err) {
        console.error('Failed to test indicator:', err)
      } finally {
        testingIndicator.value = null
      }
    }

    const testAllIndicators = async () => {
      testingAll.value = true
      allIndicatorsResult.value = null
      indicatorResults.value = {}
      
      try {
        const enabledIndicators = config.value.indicators.filter(ind => ind.enabled)
        const response = await api.previewAutomationIndicators({
          indicators: enabledIndicators
        })
        // Handle response format: data.indicators is an array, data.all_pass is boolean
        const indicators = response.data?.indicators || response.indicators || []
        const allPass = response.data?.all_pass ?? response.all_pass ?? false
        
        // Convert array to object keyed by type
        indicators.forEach(result => {
          indicatorResults.value[result.type] = {
            value: result.value,
            passed: result.pass,
            operator: result.operator,
            threshold: result.threshold,
            symbol: result.symbol,
            details: result.details,
            stale: result.stale || false,
            error: result.error || ''
          }
        })
        allIndicatorsResult.value = allPass
      } catch (err) {
        console.error('Failed to test indicators:', err)
        allIndicatorsResult.value = false
      } finally {
        testingAll.value = false
      }
    }

    const loadStrikePreview = async () => {
      loadingPreview.value = true
      strikePreview.value = null
      previewError.value = null
      
      try {
        const response = await api.previewStrikes({
          symbol: config.value.symbol,
          strategy: config.value.trade_config.strategy,
          target_delta: config.value.trade_config.target_delta,
          width: config.value.trade_config.width,
          expiration_mode: config.value.trade_config.expiration_mode || '0dte',
          custom_expiration: config.value.trade_config.custom_expiration || '',
        })
        
        if (response.success && response.data) {
          strikePreview.value = response.data
        } else {
          previewError.value = response.message || 'Failed to load strike preview'
        }
      } catch (err) {
        console.error('Failed to load strike preview:', err)
        previewError.value = err.response?.data?.message || err.message || 'Failed to load strike preview'
      } finally {
        loadingPreview.value = false
      }
    }

    const formatPrice = (price) => {
      if (price === null || price === undefined) return 'N/A'
      return '$' + price.toFixed(2)
    }

    const calculateStartingPrice = () => {
      if (!strikePreview.value?.spread?.mid_credit) return null
      const midCredit = strikePreview.value.spread.mid_credit
      const offset = config.value.trade_config.starting_offset || 0
      return midCredit - offset
    }

    // Lifecycle
    onMounted(() => {
      if (isEditMode.value) {
        loadConfig()
      }
    })

    return {
      // State
      config,
      isEditMode,
      isLoading,
      isSaving,
      errors,
      testingIndicator,
      testingAll,
      indicatorResults,
      allIndicatorsResult,
      loadingPreview,
      strikePreview,
      previewError,

      // Options
      symbols,
      timezones,
      operators,
      mobileOperators,
      displayOperators,
      strategies,
      orderTypes,
      timeInForceOptions,
      expirationModes,
      recurrenceOptions,
      
      // Mobile
      isMobile,

      // Methods
      formatIndicatorType,
      getIndicatorDescription,
      getIndicatorDefaultSymbol,
      getRecurrenceHint,
      getIndicatorResultClass,
      saveConfig,
      cancel,
      testIndicator,
      testAllIndicators,
      loadStrikePreview,
      formatPrice,
      calculateStartingPrice,
    }
  }
}
</script>

<style scoped>
.automation-config-form {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header */
.form-header {
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

.form-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.form-title i {
  color: var(--color-brand);
}

.form-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Loading State */
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-2xl);
  text-align: center;
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

/* Form Content */
.form-content {
  max-width: 900px;
}

.form-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
}

.section-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.section-description {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  margin: 0 0 var(--spacing-md) 0;
}

.subsection-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  margin: var(--spacing-lg) 0 var(--spacing-md) 0;
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-md);
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.form-field label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
}

.field-hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.p-error {
  color: var(--color-danger);
}

/* Indicators List */
.indicators-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.indicator-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  transition: opacity 0.2s;
}

.indicator-row.disabled {
  opacity: 0.5;
}

.indicator-toggle {
  flex-shrink: 0;
}

.indicator-type {
  flex: 0 0 200px;
  min-width: 150px;
  max-width: 200px;
}

.type-label {
  display: block;
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}

.type-description {
  display: block;
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.indicator-config {
  display: flex;
  gap: 12px;
  align-items: center;
}

.indicator-config.dimmed {
  opacity: 0.5;
}

.operator-dropdown {
  flex: 0 0 auto;
}

:deep(.operator-dropdown.p-dropdown) {
  width: 220px;
}

.threshold-input {
  flex: 0 0 auto;
}

:deep(.threshold-input.p-inputnumber) {
  width: 100px;
}

:deep(.threshold-input .p-inputnumber-input) {
  width: 100px;
  border-radius: var(--radius-sm);
}

.symbol-input {
  flex: 0 0 auto;
}

:deep(.symbol-input.p-inputtext) {
  width: 80px;
  border-radius: var(--radius-sm);
}

.indicator-test {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex: 0 0 auto;
}

.test-result {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.test-result.passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.test-result.failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.test-result.stale {
  background: rgba(251, 191, 36, 0.15);
  color: var(--color-warning);
  border: 1px dashed var(--color-warning);
}

.test-result .stale-icon,
.mobile-test-result .stale-icon {
  font-size: 10px;
  margin-right: 2px;
}

.test-all-section {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

.all-result {
  font-weight: var(--font-weight-semibold);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
}

.all-result.passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.all-result.failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

/* Strike Preview Section */
.strike-preview-section {
  margin-top: var(--spacing-md);
}

.strike-preview-content {
  margin-top: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-primary);
}

.preview-expiry {
  margin-bottom: var(--spacing-md);
  padding-bottom: var(--spacing-sm);
  border-bottom: 1px solid var(--border-primary);
  color: var(--text-secondary);
}

.preview-legs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-md);
}

.preview-leg {
  padding: var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--bg-secondary);
}

.preview-leg h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.preview-leg.short-leg h4 {
  color: var(--color-danger);
}

.preview-leg.long-leg h4 {
  color: var(--color-success);
}

.leg-details, .spread-details {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.leg-row, .spread-row {
  display: flex;
  justify-content: space-between;
  font-size: var(--font-size-sm);
}

.leg-label, .spread-label {
  color: var(--text-tertiary);
}

.leg-value, .spread-value {
  color: var(--text-primary);
  font-family: monospace;
}

.leg-value.highlight, .spread-value.highlight {
  font-weight: var(--font-weight-semibold);
  color: var(--color-brand);
}

.spread-value.highlight-green {
  font-weight: var(--font-weight-semibold);
  color: var(--color-success);
}

.preview-spread {
  padding: var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--bg-secondary);
  border: 1px solid var(--color-brand);
}

.preview-spread h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.preview-error {
  margin-top: var(--spacing-md);
  padding: var(--spacing-sm);
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-sm);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

/* Enable Toggle */
.enable-toggle {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.enable-label {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.enable-hint {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
}

/* Responsive */
@media (max-width: 768px) {
  .automation-config-form {
    padding: var(--spacing-md);
  }

  .form-header {
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .indicator-row {
    flex-wrap: wrap;
  }

  .indicator-config {
    width: 100%;
    flex-wrap: wrap;
  }
}

/* Mobile Indicators - Single Line Layout */
.mobile-indicators .indicator-row {
  flex-wrap: nowrap;
  padding: var(--spacing-sm);
  gap: var(--spacing-xs);
  align-items: center;
}

.mobile-indicators .indicator-toggle {
  flex: 0 0 auto;
}

.mobile-indicators .indicator-type {
  flex: 0 0 auto;
  min-width: auto;
  max-width: none;
}

.mobile-indicators .type-label {
  font-size: var(--font-size-sm);
  white-space: nowrap;
}

.mobile-indicators .indicator-config {
  flex: 1;
  display: flex;
  gap: var(--spacing-xs);
  flex-wrap: nowrap;
  justify-content: flex-end;
  min-width: 0;
}

.mobile-indicators .indicator-config.dimmed {
  opacity: 0.5;
}

/* Mobile operator dropdown - compact, no trigger arrow */
:deep(.mobile-operator.p-dropdown) {
  width: 44px !important;
  min-width: 44px !important;
}

:deep(.mobile-operator .p-dropdown-label) {
  padding: 6px 8px;
  font-size: var(--font-size-sm);
  text-align: center;
  text-overflow: clip;
  overflow: visible;
}

:deep(.mobile-operator .p-dropdown-trigger) {
  display: none;
}

/* Mobile threshold input - compact */
:deep(.mobile-threshold.p-inputnumber) {
  width: 60px !important;
}

:deep(.mobile-threshold .p-inputnumber-input) {
  width: 60px !important;
  padding: 6px 4px;
  font-size: var(--font-size-sm);
  text-align: center;
}

/* Mobile symbol input - compact */
:deep(.mobile-symbol.p-inputtext) {
  width: 50px !important;
  padding: 6px 4px;
  font-size: var(--font-size-sm);
  text-align: center;
}

/* Mobile test result - inline display */
.mobile-test-result {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  white-space: nowrap;
  min-width: 40px;
  text-align: center;
}

.mobile-test-result.passed {
  background: rgba(34, 197, 94, 0.15);
  color: var(--color-success);
}

.mobile-test-result.failed {
  background: rgba(239, 68, 68, 0.15);
  color: var(--color-danger);
}

.mobile-test-result.stale {
  background: rgba(251, 191, 36, 0.15);
  color: var(--color-warning);
  border: 1px dashed var(--color-warning);
}

/* Mobile Strike Preview */
.mobile-preview {
  padding: var(--spacing-sm);
}

.mobile-preview .preview-expiry {
  font-size: var(--font-size-sm);
  margin-bottom: var(--spacing-sm);
  padding-bottom: var(--spacing-xs);
}

.mobile-preview .preview-legs {
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-xs);
  margin-bottom: var(--spacing-sm);
}

.mobile-preview .preview-leg {
  padding: var(--spacing-xs);
}

.mobile-preview .preview-leg h4 {
  font-size: var(--font-size-xs);
  margin-bottom: var(--spacing-xs);
}

.mobile-preview .leg-details {
  gap: 2px;
}

.mobile-preview .leg-row {
  font-size: var(--font-size-xs);
}

.mobile-preview .leg-label {
  font-size: var(--font-size-xs);
}

.mobile-preview .leg-value {
  font-size: var(--font-size-xs);
}

.mobile-preview .preview-spread {
  padding: var(--spacing-xs);
}

.mobile-preview .preview-spread h4 {
  font-size: var(--font-size-xs);
  margin-bottom: var(--spacing-xs);
}

.mobile-preview .spread-details {
  gap: 2px;
}

.mobile-preview .spread-row {
  font-size: var(--font-size-xs);
}

.mobile-preview .spread-label {
  font-size: var(--font-size-xs);
}

.mobile-preview .spread-value {
  font-size: var(--font-size-xs);
}
</style>
