<template>
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

export default {
  name: 'SystemRecoveryIndicator',
  setup() {
    const isRetrying = ref(false);
    const lastRecoveryEvent = ref(null);
    const showTemporarySuccess = ref(false);
    const lastRetryTime = ref(0);
    const retryCooldown = 10000; // 10 second cooldown between manual retries

    // Get system health from the smart data store
    const systemHealth = smartMarketDataStore.getSystemHealth();

    // Computed properties for UI state
    const showIndicator = computed(() => {
      return !systemHealth.value.isHealthy || 
             systemHealth.value.recoveryInProgress || 
             showTemporarySuccess.value;
    });

    const indicatorClass = computed(() => {
      if (showTemporarySuccess.value) return 'success';
      if (systemHealth.value.recoveryInProgress) return 'recovering';
      if (!systemHealth.value.isHealthy) return 'error';
      return 'healthy';
    });

    const iconClass = computed(() => {
      if (showTemporarySuccess.value) return 'fas fa-check-circle';
      if (systemHealth.value.recoveryInProgress) return 'fas fa-sync-alt fa-spin';
      if (!systemHealth.value.isHealthy) return 'fas fa-exclamation-triangle';
      return 'fas fa-wifi';
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

    const canRetry = computed(() => {
      const now = Date.now();
      const cooldownRemaining = retryCooldown - (now - lastRetryTime.value);
      
      return !systemHealth.value.isHealthy && 
             !systemHealth.value.recoveryInProgress && 
             !isRetrying.value &&
             cooldownRemaining <= 0;
    });

    // Recovery event handlers
    const handleWebSocketRecovery = (event) => {
      console.log('🎉 WebSocket recovery event received:', event.detail);
      lastRecoveryEvent.value = event.detail;
      showSuccessMessage();
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
        showSuccessMessage();
        
      } catch (error) {
        console.error('❌ Manual retry failed:', error);
      } finally {
        isRetrying.value = false;
      }
    };

    // Lifecycle hooks
    onMounted(() => {
      // Listen for WebSocket recovery events
      window.addEventListener('websocket-recovered', handleWebSocketRecovery);
      
      // Perform initial health check
      setTimeout(() => {
        smartMarketDataStore.performHealthCheck();
      }, 1000);
    });

    onUnmounted(() => {
      window.removeEventListener('websocket-recovered', handleWebSocketRecovery);
    });

    return {
      showIndicator,
      indicatorClass,
      iconClass,
      message,
      canRetry,
      isRetrying,
      retryConnection,
      systemHealth
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
