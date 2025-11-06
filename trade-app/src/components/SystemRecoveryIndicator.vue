<template>
  <!-- Main recovery indicator (shown after grace period) -->
  <div v-if="showIndicator" class="recovery-indicator" :class="indicatorClass">
    <div class="recovery-content">
      <i class="recovery-icon" :class="iconClass"></i>
      <span class="recovery-message">{{ message }}</span>
      <button 
        v-if="canRetry" 
        @click="retryConnection" 
        class="retry-button"
        :disabled="isRetrying"
      >
        {{ isRetrying ? 'Retrying...' : 'Retry' }}
      </button>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import smartMarketDataStore from '../services/smartMarketDataStore.js';
import webSocketClient from '../services/webSocketClient.js';
import authService from '../services/authService.js';

export default {
  name: 'SystemRecoveryIndicator',
  setup() {
    const isRetrying = ref(false);
    const lastRecoveryEvent = ref(null);
    const showTemporarySuccess = ref(false);
    const lastRetryTime = ref(0);
    const retryCooldown = 10000; // 10 second cooldown between manual retries
    
    // Silent recovery configuration
    const silentRecoveryGracePeriod = 15000; // 15 seconds grace period
    const silentRecoveryStartTime = ref(null);
    const inSilentRecovery = ref(false);
    const recoveryConfidence = ref(0.8); // Default confidence score
    const recoveryAttempts = ref(0);
    const maxSilentAttempts = 3;

    // Get system health from the smart data store
    const systemHealth = smartMarketDataStore.getSystemHealth();

    // Enhanced logic for when to show indicator
    const showIndicator = computed(() => {
      // Don't show any indicators if user is not authenticated
      if (!authService.isAuthenticated()) return false;
      
      // Always show success messages
      if (showTemporarySuccess.value) return true;
      
      // Don't show anything if system is healthy
      if (systemHealth.value.isHealthy) return false;
      
      // If we're in silent recovery period, don't show error to user
      if (inSilentRecovery.value) {
        const timeInSilentRecovery = Date.now() - silentRecoveryStartTime.value;
        if (timeInSilentRecovery < silentRecoveryGracePeriod) {
          // Still within grace period - show subtle loading instead of error
          return false; // We'll show a subtle indicator instead
        }
      }
      
      // Show indicator if recovery is in progress (after grace period)
      if (systemHealth.value.recoveryInProgress) return true;
      
      // Show indicator if system is unhealthy and grace period has expired
      return !systemHealth.value.isHealthy;
    });

    // Separate computed for showing subtle recovery indicator
    const showSubtleRecovery = computed(() => {
      return false; // Completely disabled
    });

    const indicatorClass = computed(() => {
      if (showTemporarySuccess.value) return 'success';
      if (systemHealth.value.recoveryInProgress) return 'recovering';
      if (!systemHealth.value.isHealthy) return 'error';
      return 'healthy';
    });

    const subtleIndicatorClass = computed(() => {
      return ''; // Not used anymore
    });

    const iconClass = computed(() => {
      if (showTemporarySuccess.value) return 'fas fa-check-circle';
      if (systemHealth.value.recoveryInProgress) return 'fas fa-sync-alt fa-spin';
      if (!systemHealth.value.isHealthy) return 'fas fa-exclamation-triangle';
      return 'fas fa-wifi';
    });

    const subtleIconClass = computed(() => {
      return ''; // Not used anymore
    });

    const message = computed(() => {
      if (showTemporarySuccess.value) return 'Connection restored';
      if (systemHealth.value.recoveryInProgress) return 'Recovering connection...';
      if (!systemHealth.value.isHealthy) {
        const failedCount = systemHealth.value.failedComponents.length;
        if (failedCount > 0) {
          return `Connection issues detected (${failedCount} components)`;
        }
        return 'Connection issues detected';
      }
      return 'Connected';
    });

    const subtleMessage = computed(() => {
      return ''; // Not used anymore
    });

    const canRetry = computed(() => {
      const now = Date.now();
      const cooldownRemaining = retryCooldown - (now - lastRetryTime.value);
      
      return !systemHealth.value.isHealthy && 
             !systemHealth.value.recoveryInProgress && 
             !isRetrying.value &&
             !inSilentRecovery.value &&
             cooldownRemaining <= 0;
    });

    // Calculate recovery confidence based on historical success and current conditions
    const calculateRecoveryConfidence = () => {
      let confidence = 0.8; // Base confidence
      
      // Reduce confidence based on number of recent attempts
      if (recoveryAttempts.value > 0) {
        confidence -= (recoveryAttempts.value * 0.1);
      }
      
      // Increase confidence for WebSocket-only issues (easier to recover)
      const failedComponents = systemHealth.value.failedComponents;
      if (failedComponents.length === 1 && failedComponents.includes('websocket')) {
        confidence += 0.1;
      }
      
      // Decrease confidence for multiple component failures
      if (failedComponents.length > 2) {
        confidence -= 0.2;
      }
      
      return Math.max(0.1, Math.min(1.0, confidence));
    };

    // Start silent recovery when system becomes unhealthy
    const startSilentRecovery = () => {
      // Don't start recovery if user is not authenticated
      if (!authService.isAuthenticated()) {
        console.log('🔒 Skipping silent recovery - user not authenticated');
        return;
      }
      
      if (inSilentRecovery.value) return; // Already in silent recovery
      
      console.log('🤫 Starting silent recovery period');
      inSilentRecovery.value = true;
      silentRecoveryStartTime.value = Date.now();
      recoveryConfidence.value = calculateRecoveryConfidence();
      recoveryAttempts.value = 0;
      
      // Start aggressive recovery attempts during silent period
      performSilentRecoveryAttempts();
    };

    // Perform recovery attempts during silent period
    const performSilentRecoveryAttempts = async () => {
      while (inSilentRecovery.value && 
             recoveryAttempts.value < maxSilentAttempts && 
             !systemHealth.value.isHealthy) {
        
        recoveryAttempts.value++;
        console.log(`🤫 Silent recovery attempt ${recoveryAttempts.value}/${maxSilentAttempts}`);
        
        try {
          // Try different recovery strategies based on failed components
          const failedComponents = systemHealth.value.failedComponents;
          
          if (failedComponents.includes('websocket')) {
            await smartMarketDataStore.triggerRecovery('websocket_disconnected');
          }
          
          if (failedComponents.includes('greeks')) {
            await smartMarketDataStore.triggerRecovery('greeks_stale');
          }
          
          if (failedComponents.includes('api')) {
            await smartMarketDataStore.triggerRecovery('api_failure');
          }
          
          if (failedComponents.includes('dataFreshness')) {
            await smartMarketDataStore.triggerRecovery('data_stale');
          }
          
          // If no specific recovery worked, try a comprehensive recovery
          if (recoveryAttempts.value === maxSilentAttempts && !systemHealth.value.isHealthy) {
            console.log('🤫 Final attempt: trying comprehensive recovery');
            await smartMarketDataStore.triggerRecovery('websocket_disconnected');
            await new Promise(resolve => setTimeout(resolve, 1000));
            await smartMarketDataStore.triggerRecovery('api_failure');
          }
          
          // Wait between attempts
          await new Promise(resolve => setTimeout(resolve, 2000));
          
        } catch (error) {
          console.warn(`⚠️ Silent recovery attempt ${recoveryAttempts.value} failed:`, error);
        }
      }
      
      // Check if we should exit silent recovery
      setTimeout(() => {
        checkSilentRecoveryExit();
      }, 1000);
    };

    // Check if we should exit silent recovery period
    const checkSilentRecoveryExit = () => {
      if (!inSilentRecovery.value) return;
      
      const timeInSilentRecovery = Date.now() - silentRecoveryStartTime.value;
      
      if (systemHealth.value.isHealthy) {
        // Recovery successful!
        console.log('✅ Silent recovery successful');
        inSilentRecovery.value = false;
        // Don't show success message for silent recovery - it should be invisible to user
      } else if (timeInSilentRecovery >= silentRecoveryGracePeriod || 
                 recoveryAttempts.value >= maxSilentAttempts) {
        // Grace period expired or max attempts reached - now show error to user
        console.log('⏰ Silent recovery period ended, showing error to user');
        inSilentRecovery.value = false;
        
        // Now the showIndicator computed will show the error since we're out of silent recovery
        // and system is still unhealthy
      }
    };

    // Watch for system health changes to trigger silent recovery
    const unwatchHealth = smartMarketDataStore.getSystemHealth();
    let previousHealthy = unwatchHealth.value.isHealthy;

    // Recovery event handlers
    const handleWebSocketRecovery = (event) => {
      console.log('🎉 WebSocket recovery event received:', event.detail);
      lastRecoveryEvent.value = event.detail;
      
      // Only show success message if explicitly requested or recovery was visible to user
      if (event.detail.showUI || !inSilentRecovery.value) {
        showSuccessMessage();
      } else {
        // Silent recovery succeeded - just update internal state
        console.log('✅ Silent recovery completed successfully');
        inSilentRecovery.value = false;
      }
    };

    const handleInternalRecovery = (event) => {
      const detail = event.detail;
      lastRecoveryEvent.value = detail;
      
      if (detail.showUI) {
        // This is a recovery that should be shown to the user
        if (systemHealth.value.isHealthy) {
          showSuccessMessage();
        }
      } else if (systemHealth.value.isHealthy && inSilentRecovery.value) {
        // Silent recovery succeeded
        console.log('✅ Silent recovery completed successfully');
        inSilentRecovery.value = false;
      }
    };

    const showSuccessMessage = () => {
      showTemporarySuccess.value = true;
      setTimeout(() => {
        showTemporarySuccess.value = false;
      }, 3000); // Show success for 3 seconds
    };

    // Manual retry function
    const retryConnection = async () => {
      if (isRetrying.value) return;
      
      isRetrying.value = true;
      lastRetryTime.value = Date.now();
      
      try {
        console.log('🔄 Manual retry initiated');
        
        // Try multiple recovery strategies with proper sequencing
        await smartMarketDataStore.triggerRecovery('websocket_disconnected');
        await new Promise(resolve => setTimeout(resolve, 2000)); // Wait 2s for WebSocket to stabilize
        
        await smartMarketDataStore.triggerRecovery('api_failure');
        await new Promise(resolve => setTimeout(resolve, 1000)); // Wait 1s for API to stabilize
        
        // Final health check after stabilization
        await smartMarketDataStore.performHealthCheck();
        
        console.log('✅ Manual retry completed');
        
        // Always show success message for manual retry since user initiated it
        if (systemHealth.value.isHealthy) {
          showSuccessMessage();
        }
        
      } catch (error) {
        console.error('❌ Manual retry failed:', error);
      } finally {
        isRetrying.value = false;
      }
    };

    // Lifecycle hooks
    onMounted(() => {
      // Listen for WebSocket recovery events (shown to user)
      window.addEventListener('websocket-recovered', handleWebSocketRecovery);
      
      // Listen for internal recovery events (may or may not be shown to user)
      window.addEventListener('system-recovery-internal', handleInternalRecovery);
      
      // Watch for health changes to trigger silent recovery
      const healthWatcher = setInterval(() => {
        const currentHealthy = systemHealth.value.isHealthy;
        
        // If system just became unhealthy, start silent recovery (only if authenticated)
        if (previousHealthy && !currentHealthy && !inSilentRecovery.value && authService.isAuthenticated()) {
          startSilentRecovery();
        }
        
        // If we're in silent recovery, check if we should exit
        if (inSilentRecovery.value) {
          checkSilentRecoveryExit();
        }
        
        previousHealthy = currentHealthy;
      }, 1000); // Check every second
      
      // Store watcher reference for cleanup
      window.healthWatcher = healthWatcher;
    });

    onUnmounted(() => {
      window.removeEventListener('websocket-recovered', handleWebSocketRecovery);
      window.removeEventListener('system-recovery-internal', handleInternalRecovery);
      if (window.healthWatcher) {
        clearInterval(window.healthWatcher);
        delete window.healthWatcher;
      }
    });

    return {
      showIndicator,
      indicatorClass,
      iconClass,
      message,
      canRetry,
      isRetrying,
      retryConnection,
      systemHealth,
      inSilentRecovery,
      recoveryConfidence
    };
  }
};
</script>

<style scoped>
.recovery-indicator {
  position: fixed;
  top: 70px; /* Below the top bar */
  right: 20px;
  z-index: 1000;
  padding: 12px 16px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  min-width: 280px;
  animation: slideIn 0.3s ease-out;
}

.recovery-indicator.healthy {
  background: rgba(34, 197, 94, 0.9);
  color: white;
}

.recovery-indicator.success {
  background: rgba(34, 197, 94, 0.9);
  color: white;
}

.recovery-indicator.recovering {
  background: rgba(251, 191, 36, 0.9);
  color: white;
}

.recovery-indicator.error {
  background: rgba(239, 68, 68, 0.9);
  color: white;
}

.recovery-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.recovery-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.recovery-message {
  flex: 1;
  font-size: 14px;
  font-weight: 500;
}

.retry-button {
  background: rgba(255, 255, 255, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.3);
  color: white;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.retry-button:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.3);
  border-color: rgba(255, 255, 255, 0.5);
}

.retry-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

/* Dark theme adjustments */
@media (prefers-color-scheme: dark) {
  .recovery-indicator {
    border-color: rgba(255, 255, 255, 0.2);
  }
}

/* Mobile responsiveness */
@media (max-width: 768px) {
  .recovery-indicator {
    right: 10px;
    left: 10px;
    min-width: auto;
    top: 60px;
  }
  
  .recovery-content {
    gap: 8px;
  }
  
  .recovery-message {
    font-size: 13px;
  }
  
  .retry-button {
    padding: 4px 8px;
    font-size: 11px;
  }
}
</style>
