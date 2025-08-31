/**
 * Strategy Data Composable - Smart Data System Integration
 * 
 * This composable follows the same patterns as useMarketData() and integrates
 * seamlessly with the existing Smart Trade Data System architecture.
 */

import { computed } from 'vue'
import { smartStrategyDataStore } from '../services/smartStrategyDataStore.js'

export function useStrategyData() {
  // Strategy Management (Periodic Updates - 30s)
  const getMyStrategies = () => {
    return smartStrategyDataStore.getMyStrategies()
  }

  // Real-time Strategy Status (WebSocket)
  const getStrategyStatus = (strategyId) => {
    if (!strategyId) return computed(() => null)
    return smartStrategyDataStore.getStrategyStatus(strategyId)
  }

  // Global Execution Statistics (Real-time)
  const getExecutionStats = () => {
    return smartStrategyDataStore.getExecutionStats()
  }

  // Strategy Actions (Always Fresh)
  const uploadStrategy = async (file, config) => {
    return await smartStrategyDataStore.uploadStrategy(file, config)
  }

  const startStrategy = async (strategyId, config) => {
    return await smartStrategyDataStore.startStrategy(strategyId, config)
  }

  const stopStrategy = async (strategyId) => {
    return await smartStrategyDataStore.stopStrategy(strategyId)
  }

  const pauseStrategy = async (strategyId) => {
    return await smartStrategyDataStore.pauseStrategy(strategyId)
  }

  const resumeStrategy = async (strategyId) => {
    return await smartStrategyDataStore.resumeStrategy(strategyId)
  }

  const deleteStrategy = async (strategyId) => {
    return await smartStrategyDataStore.deleteStrategy(strategyId)
  }

  // Historical Data (TTL Cache - 5min)
  const getStrategyBacktest = async (strategyId, config) => {
    return await smartStrategyDataStore.getStrategyBacktest(strategyId, config)
  }

  const validateStrategy = async (strategyId) => {
    return await smartStrategyDataStore.validateStrategy(strategyId)
  }

  // Loading and error states (centralized)
  const isLoading = (operation) => {
    return computed(() => smartStrategyDataStore.loading.has(operation))
  }

  const getError = (operation) => {
    return computed(() => smartStrategyDataStore.errors.get(operation))
  }

  // Component registration helpers (same pattern as existing system)
  const registerStrategyUsage = (strategyId, componentId) => {
    return smartStrategyDataStore.registerStrategyUsage(strategyId, componentId)
  }

  const unregisterStrategyUsage = (strategyId, componentId) => {
    return smartStrategyDataStore.unregisterStrategyUsage(strategyId, componentId)
  }

  return {
    // Strategy Management
    getMyStrategies,
    getStrategyStatus,
    getExecutionStats,

    // Strategy Actions
    uploadStrategy,
    startStrategy,
    stopStrategy,
    pauseStrategy,
    resumeStrategy,
    deleteStrategy,

    // Historical & Analysis
    getStrategyBacktest,
    validateStrategy,

    // State Management
    isLoading,
    getError,

    // Component Registration
    registerStrategyUsage,
    unregisterStrategyUsage
  }
}
