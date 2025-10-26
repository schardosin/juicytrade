<template>
  <div class="strategy-monitor">
    <!-- Header Section -->
    <div class="monitor-header">
      <div class="header-content">
        <h1 class="monitor-title">
          <i class="pi pi-eye"></i>
          Strategy Monitor
        </h1>
        <p class="monitor-subtitle">
          Real-time monitoring for strategy: <strong>{{ strategyName }}</strong>
        </p>
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-arrow-left"
          label="Back to Library"
          class="p-button-outlined"
          @click="$router.push('/strategies/library')"
        />
        <Button
          icon="pi pi-chart-line"
          label="Dashboard"
          class="p-button-outlined"
          @click="$router.push('/strategies')"
        />
      </div>
    </div>

    <!-- Coming Soon Section -->
    <div class="coming-soon-section">
      <div class="coming-soon-content">
        <div class="coming-soon-icon">
          <i class="pi pi-wrench"></i>
        </div>
        <h2>Strategy Monitor Coming Soon</h2>
        <p>
          Real-time strategy monitoring with rule visualization, performance metrics, 
          and live trade tracking is currently under development.
        </p>
        <div class="planned-features">
          <h3>Planned Features:</h3>
          <ul>
            <li>Real-time rule execution visualization</li>
            <li>Live P&L tracking and performance charts</li>
            <li>Trade history and execution details</li>
            <li>Strategy health monitoring and alerts</li>
            <li>Risk metrics and position tracking</li>
          </ul>
        </div>
        <div class="back-actions">
          <Button
            icon="pi pi-book"
            label="Strategy Library"
            class="p-button-primary"
            @click="$router.push('/strategies/library')"
          />
          <Button
            icon="pi pi-chart-line"
            label="Dashboard"
            class="p-button-outlined"
            @click="$router.push('/strategies')"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useStrategyData } from '../../composables/useStrategyData.js'

export default {
  name: 'StrategyMonitor',
  setup() {
    const route = useRoute()
    const router = useRouter()
    const { getMyStrategies } = useStrategyData()

    const strategies = getMyStrategies()
    const strategyId = computed(() => route.params.id)

    const strategyName = computed(() => {
      const strategy = strategies.value.find(s => s.strategy_id === strategyId.value)
      return strategy?.name || 'Unknown Strategy'
    })

    return {
      strategyId,
      strategyName
    }
  }
}
</script>

<style scoped>
.strategy-monitor {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header Section */
.monitor-header {
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

.monitor-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.monitor-title i {
  color: var(--color-brand);
}

.monitor-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Coming Soon Section */
.coming-soon-section {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}

.coming-soon-content {
  text-align: center;
  max-width: 600px;
  padding: var(--spacing-2xl);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
}

.coming-soon-icon {
  font-size: var(--font-size-4xl);
  color: var(--color-brand);
  margin-bottom: var(--spacing-lg);
}

.coming-soon-content h2 {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.coming-soon-content > p {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  line-height: 1.6;
  margin: 0 0 var(--spacing-xl) 0;
}

.planned-features {
  text-align: left;
  margin-bottom: var(--spacing-xl);
}

.planned-features h3 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.planned-features ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.planned-features li {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) 0;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.planned-features li::before {
  content: '✓';
  color: var(--color-success);
  font-weight: var(--font-weight-bold);
  font-size: var(--font-size-lg);
}

.back-actions {
  display: flex;
  gap: var(--spacing-md);
  justify-content: center;
}

/* Responsive Design */
@media (max-width: 768px) {
  .strategy-monitor {
    padding: var(--spacing-lg);
  }

  .monitor-header {
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .header-actions {
    width: 100%;
    justify-content: stretch;
  }

  .header-actions .p-button {
    flex: 1;
  }

  .back-actions {
    flex-direction: column;
  }

  .back-actions .p-button {
    width: 100%;
  }
}
</style>
