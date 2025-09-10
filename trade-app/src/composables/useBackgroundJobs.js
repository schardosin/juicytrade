/**
 * Background Job Tracking Composable
 * 
 * Tracks active import jobs and matches them to data cards by symbol + asset_type.
 * Provides real-time progress updates for visual overlays.
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { api } from '../services/api.js'

const activeJobs = ref({})
const isPolling = ref(false)
const lastError = ref(null)
let pollInterval = null

export function useBackgroundJobs() {
  
  /**
   * Extract symbol and asset type from import job for matching with data cards
   */
  const extractJobIdentifier = (job) => {
    try {
      // Try to get symbol from job metadata or filename
      let symbol = null
      let assetType = null
      
      // For CSV jobs, symbol is directly available
      if (job.csv_symbol) {
        symbol = job.csv_symbol.toUpperCase()
        assetType = 'EQUITIES' // CSV files are typically equities
      } else {
        // For DBN jobs, try to extract from filename or progress info
        const filename = job.filename || ''
        
        // Check if it's an options file (OPRA/CBBO)
        if (filename.toLowerCase().includes('opra') || filename.toLowerCase().includes('cbbo')) {
          assetType = 'OPTIONS'
          
          // For CBBO files, they are multi-symbol options files
          // Use MULTI identifier for global progress display
          symbol = 'MULTI'
          
          // Try to extract specific symbol from current processing info if available
          if (job.progress?.current_symbol) {
            const currentSymbol = job.progress.current_symbol
            // Extract symbol from status messages like "Processing August 15, 2025" or symbol names
            const symbolMatch = currentSymbol.match(/\b([A-Z]{1,5})\b/)
            if (symbolMatch && symbolMatch[1] !== 'AUGUST' && symbolMatch[1] !== 'PROCESSING') {
              // Only use if it's actually a symbol, not a month name
              symbol = symbolMatch[1]
            }
          }
          
          // Fallback: try to extract from filename patterns for single-symbol files
          if (symbol === 'MULTI') {
            // Look for common patterns in OPRA filenames
            const patterns = [
              /([A-Z]{1,5})-\d{8}/,  // SYMBOL-YYYYMMDD
              /([A-Z]{1,5})_\d{8}/,  // SYMBOL_YYYYMMDD
              /([A-Z]{1,5})\d{8}/    // SYMBOLYYYYMMDD
            ]
            
            for (const pattern of patterns) {
              const match = filename.match(pattern)
              if (match) {
                symbol = match[1]
                break
              }
            }
          }
        } else if (filename.toLowerCase().includes('bbo') || filename.toLowerCase().includes('ohlcv')) {
          // Handle BBO and OHLCV files (typically equities)
          assetType = 'EQUITIES'
          
          // Try to extract symbol from current processing info first
          if (job.progress?.current_symbol) {
            const currentSymbol = job.progress.current_symbol
            // Extract symbol from status messages like "Processing August 15, 2025" or symbol names
            const symbolMatch = currentSymbol.match(/\b([A-Z]{1,5})\b/)
            if (symbolMatch) {
              symbol = symbolMatch[1]
            }
          }
          
          // For multi-symbol files like "xnas-itch-20180501-20250907.bbo-1m.dbn"
          // Try to extract the currently processing symbol from progress info
          if (!symbol && job.progress?.current_symbol) {
            // Look for date patterns and try to extract symbol from context
            const currentInfo = job.progress.current_symbol
            
            // If it's just a date like "Processing September 20, 2022", use MULTI
            if (currentInfo.includes('Processing') && /\d{4}/.test(currentInfo)) {
              symbol = 'MULTI'
            } else {
              // Try to extract symbol from other patterns
              const symbolMatch = currentInfo.match(/\b([A-Z]{2,5})\b/)
              if (symbolMatch) {
                symbol = symbolMatch[1]
              } else {
                symbol = 'MULTI'
              }
            }
          }
          
          // Final fallback for multi-symbol files
          if (!symbol) {
            symbol = 'MULTI'
          }
        } else {
          // Assume equities for other files
          assetType = 'EQUITIES'
          
          // Try to extract symbol from filename
          const symbolMatch = filename.match(/([A-Z]{1,5})/)
          if (symbolMatch) {
            symbol = symbolMatch[1]
          }
        }
      }
      
      if (symbol && assetType) {
        return `${symbol}_${assetType}`
      }
      
      return null
    } catch (error) {
      console.warn('Error extracting job identifier:', error)
      return null
    }
  }

  /**
   * Fetch active import jobs and update state
   */
  const fetchActiveJobs = async () => {
    try {
      const response = await api.get('/api/data-import/jobs?status=running&limit=50')
      const jobs = response.data?.data?.jobs || []
      
      console.log('🔍 Background Jobs Debug:', JSON.stringify({
        totalJobs: jobs.length,
        jobs: jobs.map(job => ({
          job_id: job.job_id,
          filename: job.filename,
          status: job.status,
          csv_symbol: job.csv_symbol,
          progress: job.progress
        }))
      }, null, 2))
      
      const newActiveJobs = {}
      
      for (const job of jobs) {
        const identifier = extractJobIdentifier(job)
        console.log('🎯 Job Identifier Extraction:', {
          job_id: job.job_id,
          filename: job.filename,
          csv_symbol: job.csv_symbol,
          extracted_identifier: identifier
        })
        
        if (identifier) {
          // Get progress percentage using our day-based progress
          let progressPercentage = 0
          if (job.progress?.progress_percentage !== undefined) {
            progressPercentage = job.progress.progress_percentage
          } else if (job.progress?.processed_records && job.progress?.total_records) {
            progressPercentage = Math.round((job.progress.processed_records / job.progress.total_records) * 100)
          }
          
          // Format status message
          let statusMessage = 'Importing...'
          if (job.progress?.current_symbol) {
            statusMessage = job.progress.current_symbol
          } else if (job.progress?.processed_records) {
            statusMessage = `Processing ${job.progress.processed_records.toLocaleString()} records`
          }
          
          const jobInfo = {
            jobId: job.job_id,
            progress: Math.min(100, Math.max(0, progressPercentage)),
            statusMessage,
            filename: job.filename,
            startedAt: job.started_at,
            processedRecords: job.progress?.processed_records || 0,
            totalRecords: job.progress?.total_records || 0
          }
          
          newActiveJobs[identifier] = jobInfo
          
          // For multi-symbol jobs, also create specific symbol entries using backend data
          if (identifier === 'MULTI_OPTIONS' || identifier === 'MULTI_EQUITIES') {
            const primarySymbols = job.progress?.primary_symbols || []
            const assetType = identifier === 'MULTI_OPTIONS' ? 'OPTIONS' : 'EQUITIES'
            
            // Create job entries for each primary symbol the backend is processing
            for (const symbol of primarySymbols) {
              if (symbol && symbol !== 'MULTI') {
                const specificIdentifier = `${symbol}_${assetType}`
                newActiveJobs[specificIdentifier] = {
                  ...jobInfo,
                  statusMessage: `${statusMessage} (${symbol} ${assetType.toLowerCase()})`
                }
                console.log(`🎯 Created specific symbol entry from backend data: ${specificIdentifier}`)
              }
            }
          }
        }
      }
      
      activeJobs.value = newActiveJobs
      lastError.value = null
      
    } catch (error) {
      console.error('Error fetching active jobs:', error)
      lastError.value = error.message
      
      // On error, don't clear existing jobs immediately - they might still be valid
      // Only clear after multiple consecutive errors
    }
  }

  /**
   * Start polling for active jobs with adaptive frequency
   */
  const startPolling = () => {
    if (isPolling.value) return
    
    isPolling.value = true
    
    // Initial fetch
    fetchActiveJobs()
    
    // Adaptive polling: slower during heavy imports to reduce resource contention
    const getPollingInterval = () => {
      const jobCount = Object.keys(activeJobs.value).length
      if (jobCount === 0) {
        return 5000 // 5 seconds when no jobs
      } else if (jobCount >= 2) {
        return 8000 // 8 seconds when multiple jobs (heavy load)
      } else {
        return 4000 // 4 seconds for single job
      }
    }
    
    // Dynamic polling with adaptive intervals
    const scheduleNextPoll = () => {
      if (isPolling.value) {
        const interval = getPollingInterval()
        pollInterval = setTimeout(() => {
          fetchActiveJobs().finally(() => {
            scheduleNextPoll()
          })
        }, interval)
      }
    }
    
    scheduleNextPoll()
  }

  /**
   * Stop polling for active jobs
   */
  const stopPolling = () => {
    if (!isPolling.value) return
    
    isPolling.value = false
    
    if (pollInterval) {
      clearTimeout(pollInterval)
      pollInterval = null
    }
  }

  /**
   * Get job info for a specific symbol and asset type
   */
  const getJobForSymbol = (symbol, assetType) => {
    const identifier = `${symbol.toUpperCase()}_${assetType.toUpperCase()}`
    return activeJobs.value[identifier] || null
  }

  /**
   * Check if a symbol has an active import job
   */
  const hasActiveJob = (symbol, assetType) => {
    return getJobForSymbol(symbol, assetType) !== null
  }

  /**
   * Get all active job identifiers
   */
  const activeJobIdentifiers = computed(() => {
    return Object.keys(activeJobs.value)
  })

  /**
   * Get total number of active jobs
   */
  const activeJobCount = computed(() => {
    return Object.keys(activeJobs.value).length
  })

  /**
   * Manual refresh of active jobs
   */
  const refresh = async () => {
    await fetchActiveJobs()
  }

  // Auto-start polling when composable is used
  onMounted(() => {
    startPolling()
  })

  // Clean up polling when component unmounts
  onUnmounted(() => {
    stopPolling()
  })

  return {
    // State
    activeJobs: computed(() => activeJobs.value),
    isPolling: computed(() => isPolling.value),
    lastError: computed(() => lastError.value),
    activeJobCount,
    activeJobIdentifiers,
    
    // Methods
    startPolling,
    stopPolling,
    getJobForSymbol,
    hasActiveJob,
    refresh
  }
}

// Global instance for sharing across components
let globalBackgroundJobs = null

export function useGlobalBackgroundJobs() {
  if (!globalBackgroundJobs) {
    globalBackgroundJobs = useBackgroundJobs()
  }
  return globalBackgroundJobs
}
