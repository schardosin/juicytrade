<template>
  <div v-if="visible" class="dialog-overlay" @click="closeDialog">
    <div class="dialog-content" @click.stop>
      <div class="dialog-header">
        <h2>Create Backtest</h2>
        <button class="btn-close" @click="closeDialog">×</button>
      </div>
      
      <div class="dialog-body">
        <!-- Strategy Selection (when no strategy is pre-selected and none selected yet) -->
        <div v-if="!strategy && !selectedStrategy" class="form-section">
          <h3>Select Strategy</h3>
          <div class="strategy-selector">
            <div 
              v-for="availableStrategy in availableStrategies" 
              :key="availableStrategy.strategy_id"
              class="strategy-option"
              @click="selectStrategy(availableStrategy)"
            >
              <div class="strategy-info">
                <h4>{{ availableStrategy.name }}</h4>
                <p>{{ availableStrategy.description || 'No description' }}</p>
              </div>
              <div class="strategy-meta">
                <span class="meta-badge">{{ availableStrategy.risk_level || 'Medium' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Strategy Info (when strategy is pre-selected or selected) -->
        <div v-if="currentStrategy" class="form-section">
          <h3>Strategy: {{ currentStrategy.name }}</h3>
          <p class="strategy-description">{{ currentStrategy.description || 'No description provided' }}</p>
          
          <!-- Add button to change strategy selection when in selection mode -->
          <div v-if="!strategy && selectedStrategy" class="strategy-change">
            <button 
              type="button" 
              class="btn btn-outline btn-sm" 
              @click="clearSelection"
            >
              Change Strategy
            </button>
          </div>
        </div>

        <!-- Parameters Configuration -->
        <div v-if="currentStrategy && strategyParameters" class="form-section">
          <!-- Framework Parameters -->
          <div class="parameter-group">
            <h4>Framework Settings</h4>
            <div class="form-row">
              <div class="form-group">
                <label>Start Date</label>
                <input 
                  type="date" 
                  v-model="localBacktestConfig.start_date"
                  class="form-input"
                />
              </div>
              <div class="form-group">
                <label>End Date</label>
                <input 
                  type="date" 
                  v-model="localBacktestConfig.end_date"
                  class="form-input"
                />
              </div>
            </div>
            <div class="form-row">
              <div class="form-group">
                <label>Data Timeframe</label>
                <select v-model="localBacktestConfig.timeframe" class="form-input">
                  <option value="1m">1 Minute</option>
                  <option value="5m">5 Minutes</option>
                  <option value="15m">15 Minutes</option>
                  <option value="30m">30 Minutes</option>
                  <option value="1h">1 Hour</option>
                  <option value="4h">4 Hours</option>
                  <option value="D">Daily</option>
                </select>
              </div>
              <div class="form-group">
                <label>Initial Capital ($)</label>
                <input 
                  type="number" 
                  v-model="localBacktestConfig.initial_capital"
                  min="1000"
                  step="1000"
                  class="form-input"
                />
              </div>
            </div>
            <div class="form-row">
              <div class="form-group">
                <label>Speed Multiplier</label>
                <select v-model="localBacktestConfig.speed_multiplier" class="form-input">
                  <option value="1">1x (Real-time)</option>
                  <option value="10">10x (Fast)</option>
                  <option value="100">100x (Very Fast)</option>
                  <option value="1000">1000x (Maximum)</option>
                </select>
              </div>
              <div class="form-group">
                <!-- Empty space for layout balance -->
              </div>
            </div>
          </div>

          <!-- Strategy Parameters -->
          <div class="parameter-group">
            <h4>Strategy Parameters</h4>
            <div class="parameters-grid">
              <div
                v-for="(param, paramName) in strategyParameters.parameters"
                :key="paramName"
                class="form-group"
              >
                <label>
                  {{ formatParameterName(paramName) }}
                  <span v-if="param.description" class="param-description">
                    {{ param.description }}
                  </span>
                </label>

                <!-- String Input -->
                <input
                  v-if="param.type === 'string'"
                  type="text"
                  v-model="localStrategyConfig[paramName]"
                  :placeholder="param.default || 'Enter value'"
                  class="form-input"
                />

                <!-- Integer Input -->
                <input
                  v-else-if="param.type === 'integer'"
                  type="number"
                  v-model="localStrategyConfig[paramName]"
                  :min="param.min"
                  :max="param.max"
                  step="1"
                  :placeholder="param.default?.toString() || '0'"
                  class="form-input"
                />

                <!-- Float Input -->
                <input
                  v-else-if="param.type === 'float'"
                  type="number"
                  v-model="localStrategyConfig[paramName]"
                  :min="param.min"
                  :max="param.max"
                  step="0.1"
                  :placeholder="param.default?.toString() || '0.0'"
                  class="form-input"
                />

                <!-- Boolean Input -->
                <div v-else-if="param.type === 'boolean'" class="checkbox-group">
                  <input
                    type="checkbox"
                    :id="paramName"
                    v-model="localStrategyConfig[paramName]"
                  />
                  <label :for="paramName">{{ param.default ? 'Enabled' : 'Disabled' }}</label>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="dialog-footer">
        <button class="btn btn-outline" @click="closeDialog">Cancel</button>
        <button 
          class="btn btn-primary" 
          @click="runBacktest"
          :disabled="!currentStrategy || isRunning"
          :class="{ loading: isRunning }"
        >
          <span v-if="isRunning">Running...</span>
          <span v-else>Run Backtest</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, watch, computed } from 'vue'
import { api } from '../../services/api.js'

export default {
  name: 'BacktestConfigDialog',
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    strategy: {
      type: Object,
      default: null
    }
  },
  emits: ['close', 'backtest-started'],
  setup(props, { emit }) {
    // Local state
    const strategyParameters = ref(null)
    const isRunning = ref(false)
    const availableStrategies = ref([])
    const selectedStrategy = ref(null)
    
    // Default configuration
    const defaultBacktestConfig = {
      start_date: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0], // 30 days ago
      end_date: new Date().toISOString().split('T')[0], // today
      initial_capital: 10000,
      speed_multiplier: 1000,
      timeframe: '1m'
    }
    
    const localBacktestConfig = ref({ ...defaultBacktestConfig })
    const localStrategyConfig = ref({})

    // Computed property to determine current strategy (prop or selected)
    const currentStrategy = computed(() => props.strategy || selectedStrategy.value)

    // Watch for dialog visibility to load strategies when needed
    watch(() => props.visible, async (isVisible) => {
      if (isVisible && !props.strategy) {
        // Only load strategies if no strategy is pre-selected
        await loadAvailableStrategies()
      }
    })

    // Watch for strategy changes to load parameters
    watch(() => props.strategy, async (newStrategy) => {
      if (newStrategy) {
        await loadStrategyParameters(newStrategy)
      } else {
        strategyParameters.value = null
        localStrategyConfig.value = {}
      }
    }, { immediate: true })

    // Watch for selected strategy changes
    watch(selectedStrategy, async (newStrategy) => {
      if (newStrategy) {
        await loadStrategyParameters(newStrategy)
      }
    })

    // Load strategy parameters
    const loadStrategyParameters = async (strategy) => {
      try {
        const response = await api.get(`/api/strategies/${strategy.strategy_id}/parameters`)
        if (response.data.success) {
          strategyParameters.value = response.data
          
          // Initialize strategy config with default values
          const config = {}
          Object.entries(response.data.parameters).forEach(([paramName, param]) => {
            config[paramName] = param.default
          })
          localStrategyConfig.value = config
        }
      } catch (err) {
        console.error('Error loading strategy parameters:', err)
      }
    }

    // Parameter formatting utility
    const formatParameterName = (paramName) => {
      return paramName
        .replace(/_/g, ' ')
        .replace(/\b\w/g, l => l.toUpperCase())
    }

    // Dialog functions
    const closeDialog = () => {
      emit('close')
      // Reset state
      strategyParameters.value = null
      localStrategyConfig.value = {}
      localBacktestConfig.value = { ...defaultBacktestConfig }
      selectedStrategy.value = null // Reset selected strategy to prevent duplication
    }

    // Load available strategies
    const loadAvailableStrategies = async () => {
      try {
        const strategies = await api.getStrategies()
        availableStrategies.value = strategies || []
      } catch (err) {
        console.error('Error loading strategies:', err)
      }
    }

    // Strategy selection
    const selectStrategy = (strategy) => {
      selectedStrategy.value = strategy
    }

    // Clear strategy selection to go back to selection interface
    const clearSelection = () => {
      selectedStrategy.value = null
      strategyParameters.value = null
      localStrategyConfig.value = {}
    }

    const runBacktest = async () => {
      const targetStrategy = currentStrategy.value
      if (!targetStrategy) return
      
      try {
        isRunning.value = true
        
        // Prepare backtest request
        const backtestRequest = {
          parameters: localStrategyConfig.value,
          start_date: localBacktestConfig.value.start_date,
          end_date: localBacktestConfig.value.end_date,
          initial_capital: localBacktestConfig.value.initial_capital,
          speed_multiplier: localBacktestConfig.value.speed_multiplier,
          timeframe: localBacktestConfig.value.timeframe
        }
        
        const response = await axios.post(
          `/api/strategies/${targetStrategy.strategy_id}/backtest`, 
          backtestRequest,
          { timeout: 600000 } // 10 minute timeout for long-running backtests
        )
        
        if (response.data.success) {
          console.log('Backtest started successfully:', response.data)
          emit('backtest-started', {
            success: true,
            message: 'Backtest started successfully!',
            data: response.data
          })
          closeDialog()
        } else {
          console.error('Backtest failed:', response.data.error)
          emit('backtest-started', {
            success: false,
            message: `Backtest failed: ${response.data.error}`,
            error: response.data.error
          })
        }
      } catch (err) {
        console.error('Error running backtest:', err)
        emit('backtest-started', {
          success: false,
          message: 'Failed to start backtest. Please try again.',
          error: err.message
        })
      } finally {
        isRunning.value = false
      }
    }

    return {
      // State
      strategyParameters,
      isRunning,
      availableStrategies,
      selectedStrategy,
      currentStrategy,
      localBacktestConfig,
      localStrategyConfig,
      
      // Functions
      formatParameterName,
      closeDialog,
      runBacktest,
      selectStrategy,
      clearSelection
    }
  }
}
</script>

<style scoped>
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

.dialog-body {
  padding: var(--spacing-lg);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
}

.form-section {
  margin-bottom: var(--spacing-xl);
}

.form-section h3 {
  margin: 0 0 var(--spacing-lg) 0;
  color: var(--text-primary);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}

.strategy-description {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-md);
  line-height: 1.4;
}

.parameter-group {
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-lg);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
}

.parameter-group h4 {
  margin: 0 0 var(--spacing-lg) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
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

.param-description {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: var(--font-weight-normal);
  font-style: italic;
  margin-top: 2px;
}

.form-input {
  padding: var(--spacing-sm);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.form-input:focus {
  outline: none;
  border-color: var(--color-brand);
  box-shadow: 0 0 0 2px rgba(255, 107, 53, 0.1);
}

.parameters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: var(--spacing-lg);
}

.checkbox-group {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.checkbox-group input[type="checkbox"] {
  width: 16px;
  height: 16px;
}

.checkbox-group label {
  cursor: pointer;
  user-select: none;
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

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Strategy Selector Styles */
.strategy-selector {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.strategy-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition-normal);
}

.strategy-option:hover {
  border-color: var(--color-brand);
  background: var(--bg-tertiary);
}

.strategy-option.active {
  border-color: var(--color-brand);
  background: rgba(255, 107, 53, 0.1);
}

.strategy-info h4 {
  margin: 0 0 var(--spacing-xs) 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.strategy-info p {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.strategy-meta {
  display: flex;
  align-items: center;
}

.meta-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  background: var(--bg-quaternary);
  color: var(--text-secondary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
}

@media (max-width: 768px) {
  .form-row {
    grid-template-columns: 1fr;
  }
  
  .parameters-grid {
    grid-template-columns: 1fr;
  }
  
  .dialog-content {
    width: 95vw;
    margin: var(--spacing-md);
  }
}
</style>
