/**
 * Smart Strategy Data Store
 * 
 * Extends the Smart Trade Data System with strategy-specific data management.
 * Follows the same architectural patterns: WebSocket, Periodic, TTL Cache, Fresh strategies.
 */

import { ref, computed, reactive } from 'vue'
import axios from 'axios'

// Get API base URL (same pattern as api.js)
const getApiBaseUrl = () => {
  const envApiBaseUrl = import.meta.env.JUICYTRADE_API_BASE_URL;
  if (envApiBaseUrl && envApiBaseUrl.trim() !== '') {
    return envApiBaseUrl;
  }
  return "/api";
};

const API_BASE_URL = getApiBaseUrl();

class SmartStrategyDataStore {
  constructor() {
    // Strategy-specific data strategies following existing patterns
    this.strategies = {
      // User strategies with periodic refresh (30s)
      userStrategies: ref([]),
      lastUserStrategiesUpdate: ref(null),
      
      // Real-time strategy status (WebSocket)
      strategyStatuses: reactive(new Map()),
      
      // Execution statistics (Real-time)
      executionStats: ref({
        total_strategies: 0,
        running_strategies: 0,
        paused_strategies: 0,
        total_pnl: 0,
        total_trades: 0,
        uptime_seconds: 0
      }),
      
      // Backtest results cache (TTL - 5min)
      backtestCache: reactive(new Map()),
      backtestCacheTimestamps: reactive(new Map())
    }

    // Component registration system (same as existing)
    this.strategyUsageCount = new Map()
    this.componentRegistrations = new Map()

    // Loading and error states (centralized)
    this.loading = reactive(new Set())
    this.errors = reactive(new Map())

    // Periodic update intervals
    this.userStrategiesInterval = null
    this.executionStatsInterval = null

    // WebSocket integration
    this.webSocketCallbacks = new Map()

    // Initialize periodic updates
    this.initializePeriodicUpdates()
    this.initializeWebSocketIntegration()
  }

  // ============================================================================
  // Strategy Management (Periodic Updates - 30s)
  // ============================================================================

  getMyStrategies() {
    // Trigger initial load if needed
    if (!this.strategies.userStrategies.value?.length && !this.loading.has('strategies')) {
      this.loadUserStrategies()
    }

    return computed(() => this.strategies.userStrategies.value || [])
  }

  async loadUserStrategies() {
    try {
      this.loading.add('strategies')
      this.errors.delete('strategies')

      const response = await axios.get(`${API_BASE_URL}/api/strategies/my`)
      
      if (response.data) {
        this.strategies.userStrategies.value = response.data
        this.strategies.lastUserStrategiesUpdate.value = Date.now()
      }
    } catch (error) {
      console.error('Failed to load user strategies:', error)
      this.errors.set('strategies', error.message)
    } finally {
      this.loading.delete('strategies')
    }
  }

  // ============================================================================
  // Real-time Strategy Status (WebSocket)
  // ============================================================================

  getStrategyStatus(strategyId) {
    // Initialize status if not exists
    if (!this.strategies.strategyStatuses.has(strategyId)) {
      this.strategies.strategyStatuses.set(strategyId, ref(null))
      // Load initial status
      this.loadStrategyStatus(strategyId)
    }

    return computed(() => this.strategies.strategyStatuses.get(strategyId)?.value)
  }

  async loadStrategyStatus(strategyId) {
    try {
      this.loading.add(`strategy_status_${strategyId}`)
      
      const response = await axios.get(`${API_BASE_URL}/api/strategies/${strategyId}/status`)
      
      if (response.data) {
        const statusRef = this.strategies.strategyStatuses.get(strategyId)
        if (statusRef) {
          statusRef.value = response.data
        }
      }
    } catch (error) {
      console.error(`Failed to load strategy status for ${strategyId}:`, error)
      this.errors.set(`strategy_status_${strategyId}`, error.message)
    } finally {
      this.loading.delete(`strategy_status_${strategyId}`)
    }
  }

  // ============================================================================
  // Execution Statistics (Real-time)
  // ============================================================================

  getExecutionStats() {
    return computed(() => this.strategies.executionStats.value)
  }

  async loadExecutionStats() {
    try {
      const response = await axios.get(`${API_BASE_URL}/api/strategies/stats`)
      
      if (response.data) {
        this.strategies.executionStats.value = response.data
      }
    } catch (error) {
      console.error('Failed to load execution stats:', error)
    }
  }

  // ============================================================================
  // Strategy Actions (Always Fresh)
  // ============================================================================

  async uploadStrategy(file, config) {
    try {
      this.loading.add('upload_strategy')
      this.errors.delete('upload_strategy')

      const formData = new FormData()
      formData.append('file', file)
      formData.append('name', config.name)
      formData.append('description', config.description || '')

      const response = await axios.post(`${API_BASE_URL}/api/strategies/upload`, formData, {
        headers: {
          'Content-Type': 'multipart/form-data'
        }
      })

      if (response.data.success) {
        // Refresh user strategies list
        await this.loadUserStrategies()
        return response.data
      } else {
        // Return validation failure details instead of throwing error
        console.log('Strategy validation failed:', response.data)
        return {
          success: false,
          error: response.data.error || 'Upload failed',
          message: response.data.message,
          validation_details: response.data.validation_details
        }
      }
    } catch (error) {
      console.error('Strategy upload failed:', error)
      this.errors.set('upload_strategy', error.message)
      
      // Check if this is an axios error with response data
      if (error.response && error.response.data) {
        return {
          success: false,
          error: error.response.data.error || 'Upload failed',
          message: error.response.data.message,
          validation_details: error.response.data.validation_details
        }
      }
      
      throw error
    } finally {
      this.loading.delete('upload_strategy')
    }
  }

  async startStrategy(strategyId, config) {
    try {
      this.loading.add(`start_strategy_${strategyId}`)
      
      const response = await axios.post(`${API_BASE_URL}/api/strategies/${strategyId}/start`, config)
      
      if (response.data.success) {
        // Refresh strategy status
        await this.loadStrategyStatus(strategyId)
        // Refresh user strategies list
        await this.loadUserStrategies()
        return response.data
      } else {
        throw new Error(response.data.error || 'Failed to start strategy')
      }
    } catch (error) {
      console.error(`Failed to start strategy ${strategyId}:`, error)
      this.errors.set(`start_strategy_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`start_strategy_${strategyId}`)
    }
  }

  async stopStrategy(strategyId) {
    try {
      this.loading.add(`stop_strategy_${strategyId}`)
      
      const response = await axios.post(`${API_BASE_URL}/api/strategies/${strategyId}/stop`)
      
      if (response.data.success) {
        // Refresh strategy status
        await this.loadStrategyStatus(strategyId)
        return response.data
      } else {
        throw new Error(response.data.error || 'Failed to stop strategy')
      }
    } catch (error) {
      console.error(`Failed to stop strategy ${strategyId}:`, error)
      this.errors.set(`stop_strategy_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`stop_strategy_${strategyId}`)
    }
  }

  async pauseStrategy(strategyId) {
    try {
      this.loading.add(`pause_strategy_${strategyId}`)
      
      const response = await axios.post(`${API_BASE_URL}/api/strategies/${strategyId}/pause`)
      
      if (response.data.success) {
        await this.loadStrategyStatus(strategyId)
        return response.data
      } else {
        throw new Error(response.data.error || 'Failed to pause strategy')
      }
    } catch (error) {
      console.error(`Failed to pause strategy ${strategyId}:`, error)
      this.errors.set(`pause_strategy_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`pause_strategy_${strategyId}`)
    }
  }

  async resumeStrategy(strategyId) {
    try {
      this.loading.add(`resume_strategy_${strategyId}`)
      
      const response = await axios.post(`${API_BASE_URL}/api/strategies/${strategyId}/resume`)
      
      if (response.data.success) {
        await this.loadStrategyStatus(strategyId)
        return response.data
      } else {
        throw new Error(response.data.error || 'Failed to resume strategy')
      }
    } catch (error) {
      console.error(`Failed to resume strategy ${strategyId}:`, error)
      this.errors.set(`resume_strategy_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`resume_strategy_${strategyId}`)
    }
  }

  async deleteStrategy(strategyId) {
    try {
      this.loading.add(`delete_strategy_${strategyId}`)
      
      const response = await axios.delete(`${API_BASE_URL}/api/strategies/${strategyId}`)
      
      if (response.data.success) {
        // Remove from local state
        this.strategies.strategyStatuses.delete(strategyId)
        // Refresh user strategies list
        await this.loadUserStrategies()
        return response.data
      } else {
        throw new Error(response.data.error || 'Failed to delete strategy')
      }
    } catch (error) {
      console.error(`Failed to delete strategy ${strategyId}:`, error)
      this.errors.set(`delete_strategy_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`delete_strategy_${strategyId}`)
    }
  }

  // ============================================================================
  // Historical Data (TTL Cache - 5min)
  // ============================================================================

  async getStrategyBacktest(strategyId, config) {
    const cacheKey = `${strategyId}_${JSON.stringify(config)}`
    const now = Date.now()
    const cacheTimeout = 5 * 60 * 1000 // 5 minutes

    // Check cache
    if (this.strategies.backtestCache.has(cacheKey)) {
      const timestamp = this.strategies.backtestCacheTimestamps.get(cacheKey)
      if (timestamp && (now - timestamp) < cacheTimeout) {
        return this.strategies.backtestCache.get(cacheKey)
      }
    }

    try {
      this.loading.add(`backtest_${strategyId}`)
      
      // NEW TEMPLATE-BASED APPROACH: Include parameters directly in the backtest request
      const backtestRequest = {
        parameters: config.parameters || {}, // Strategy parameters
        start_date: config.start_date,
        end_date: config.end_date,
        initial_capital: config.initial_capital,
        speed_multiplier: config.speed_multiplier
      }
      
      const response = await axios.post(`${API_BASE_URL}/api/strategies/${strategyId}/backtest`, backtestRequest)
      
      if (response.data.success) {
        // Cache the result
        this.strategies.backtestCache.set(cacheKey, response.data)
        this.strategies.backtestCacheTimestamps.set(cacheKey, now)
        return response.data
      } else {
        throw new Error(response.data.error || 'Backtest failed')
      }
    } catch (error) {
      console.error(`Backtest failed for strategy ${strategyId}:`, error)
      this.errors.set(`backtest_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`backtest_${strategyId}`)
    }
  }

  async validateStrategy(strategyId) {
    try {
      this.loading.add(`validate_${strategyId}`)
      
      const response = await axios.post(`${API_BASE_URL}/api/strategies/${strategyId}/validate`)
      
      return response.data
    } catch (error) {
      console.error(`Validation failed for strategy ${strategyId}:`, error)
      this.errors.set(`validate_${strategyId}`, error.message)
      throw error
    } finally {
      this.loading.delete(`validate_${strategyId}`)
    }
  }

  // ============================================================================
  // Component Registration System (Same as existing)
  // ============================================================================

  registerStrategyUsage(strategyId, componentId) {
    const currentCount = this.strategyUsageCount.get(strategyId) || 0
    this.strategyUsageCount.set(strategyId, currentCount + 1)
    
    if (!this.componentRegistrations.has(componentId)) {
      this.componentRegistrations.set(componentId, new Set())
    }
    this.componentRegistrations.get(componentId).add(strategyId)
    
    if (currentCount === 0) {
      this.subscribeToStrategy(strategyId)
    }
  }

  unregisterStrategyUsage(strategyId, componentId) {
    const currentCount = this.strategyUsageCount.get(strategyId) || 0
    if (currentCount > 0) {
      this.strategyUsageCount.set(strategyId, currentCount - 1)
      
      if (this.componentRegistrations.has(componentId)) {
        this.componentRegistrations.get(componentId).delete(strategyId)
      }
      
      if (currentCount - 1 === 0) {
        this.unsubscribeFromStrategy(strategyId)
      }
    }
  }

  subscribeToStrategy(strategyId) {
    // This would integrate with your existing WebSocket system
    console.log(`Subscribing to strategy updates for ${strategyId}`)
    // TODO: Integrate with existing WebSocket worker
  }

  unsubscribeFromStrategy(strategyId) {
    // This would integrate with your existing WebSocket system
    console.log(`Unsubscribing from strategy updates for ${strategyId}`)
    // TODO: Integrate with existing WebSocket worker
  }

  // ============================================================================
  // Periodic Updates (Same pattern as existing)
  // ============================================================================

  initializePeriodicUpdates() {
    // User strategies refresh every 30 seconds
    this.userStrategiesInterval = setInterval(() => {
      if (this.strategies.userStrategies.value.length > 0) {
        this.loadUserStrategies()
      }
    }, 30000)

    // Execution stats refresh every 10 seconds
    this.executionStatsInterval = setInterval(() => {
      this.loadExecutionStats()
    }, 10000)
  }

  initializeWebSocketIntegration() {
    // TODO: Integrate with existing WebSocket worker
    // This would extend your existing streaming-worker.js with strategy message types
    console.log('Strategy WebSocket integration initialized')
  }

  // ============================================================================
  // Cleanup
  // ============================================================================

  destroy() {
    if (this.userStrategiesInterval) {
      clearInterval(this.userStrategiesInterval)
    }
    if (this.executionStatsInterval) {
      clearInterval(this.executionStatsInterval)
    }
  }
}

// Create singleton instance
export const smartStrategyDataStore = new SmartStrategyDataStore()
