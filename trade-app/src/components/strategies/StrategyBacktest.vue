<template>
  <div class="strategy-backtest">
    <!-- Header Section -->
    <div class="backtest-header">
      <div class="header-content">
        <h1 class="backtest-title">
          <i class="pi pi-chart-bar"></i>
          Strategy Backtest
        </h1>
        <p class="backtest-subtitle">
          Historical performance analysis for strategy: <strong>{{ strategyName }}</strong>
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
          <i class="pi pi-cog"></i>
        </div>
        <h2>Strategy Backtesting Coming Soon</h2>
        <p>
          Comprehensive historical backtesting with performance analytics, 
          risk metrics, and optimization tools is currently under development.
        </p>
        <div class="planned-features">
          <h3>Planned Features:</h3>
          <ul>
            <li>Historical performance simulation</li>
            <li>Risk-adjusted return metrics (Sharpe, Sortino, etc.)</li>
            <li>Drawdown analysis and risk assessment</li>
            <li>Strategy parameter optimization</li>
            <li>Benchmark comparison and analysis</li>
            <li>Monte Carlo simulation and stress testing</li>
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
  name: 'StrategyBacktest',
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
.strategy-backtest {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header Section */
.backtest-header {
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

.backtest-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.backtest-title i {
  color: var(--color-brand);
}

.backtest-subtitle {
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
  .strategy-backtest {
    padding: var(--spacing-lg);
  }

  .backtest-header {
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
