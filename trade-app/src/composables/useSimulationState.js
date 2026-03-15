import { ref, computed, watch } from 'vue';
import { calculateWeightedAverageIV } from '../utils/theoreticalPayoff.js';

const IV_MIN = 0.05;
const IV_MAX = 2.0;
const IV_STEP = 0.01;

export function useSimulationState(positions, marketData) {
  const now = new Date();
  now.setHours(0, 0, 0, 0);
  const selectedDate = ref(now);
  const selectedIVOverride = ref(null);
  const showExpirationLine = ref(true);
  const showTheoreticalLine = ref(true);

  const defaultIV = computed(() => {
    const positionsValue = typeof positions === 'function' ? positions() : positions?.value ?? positions;
    
    if (!positionsValue || positionsValue.length === 0) {
      return null;
    }

    const getIV = (symbol) => {
      const greeksData = marketData.getOptionGreeks(symbol);
      const greeks = greeksData?.value;
      
      if (!greeks || !greeks.implied_volatility) {
        return null;
      }
      
      const iv = parseFloat(greeks.implied_volatility);
      return Number.isFinite(iv) && iv > 0 ? iv : null;
    };

    return calculateWeightedAverageIV(positionsValue, getIV);
  });

  const effectiveIV = computed(() => {
    const override = selectedIVOverride.value;
    if (override !== null && override !== undefined && Number.isFinite(override)) {
      return override;
    }
    return defaultIV.value;
  });

  const minDate = computed(() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    return today;
  });

  const maxDate = computed(() => {
    const positionsValue = typeof positions === 'function' ? positions() : positions?.value ?? positions;
    
    if (!positionsValue || positionsValue.length === 0) {
      return null;
    }

    let latestExpiry = null;
    for (const pos of positionsValue) {
      if (pos.expiry_date) {
        const expiryDate = new Date(pos.expiry_date + 'T16:00:00');
        if (!latestExpiry || expiryDate > latestExpiry) {
          latestExpiry = expiryDate;
        }
      }
    }

    return latestExpiry;
  });

  const ivAvailable = computed(() => {
    const positionsValue = typeof positions === 'function' ? positions() : positions?.value ?? positions;
    
    if (!positionsValue || positionsValue.length === 0) {
      return false;
    }

    for (const pos of positionsValue) {
      if (!pos.symbol) continue;
      
      const greeksData = marketData.getOptionGreeks(pos.symbol);
      const greeks = greeksData?.value;
      
      if (!greeks || !greeks.implied_volatility) {
        return false;
      }
      
      const iv = parseFloat(greeks.implied_volatility);
      if (!Number.isFinite(iv) || iv <= 0) {
        return false;
      }
    }

    return true;
  });

  const reset = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    selectedDate.value = today;
    selectedIVOverride.value = null;
    showExpirationLine.value = true;
    showTheoreticalLine.value = true;
  };

  const incrementDate = () => {
    const current = selectedDate.value;
    const next = new Date(current);
    next.setDate(next.getDate() + 1);
    next.setHours(0, 0, 0, 0);
    
    const max = maxDate.value;
    if (max) {
      const maxNormalized = new Date(max);
      maxNormalized.setHours(0, 0, 0, 0);
      if (next > maxNormalized) {
        return;
      }
    }
    
    selectedDate.value = next;
  };

  const decrementDate = () => {
    const current = selectedDate.value;
    const prev = new Date(current);
    prev.setDate(prev.getDate() - 1);
    prev.setHours(0, 0, 0, 0);
    
    const min = minDate.value;
    if (prev < min) {
      return;
    }
    
    selectedDate.value = prev;
  };

  const incrementIV = () => {
    const current = selectedIVOverride.value ?? defaultIV.value ?? IV_MIN;
    const newIV = Math.min(current + IV_STEP, IV_MAX);
    selectedIVOverride.value = newIV;
  };

  const decrementIV = () => {
    const current = selectedIVOverride.value ?? defaultIV.value ?? IV_MIN;
    const newIV = Math.max(current - IV_STEP, IV_MIN);
    selectedIVOverride.value = newIV;
  };

  const positionsRef = typeof positions === 'function' 
    ? { value: computed(positions) } 
    : positions;

  if (positionsRef && typeof positionsRef === 'object' && 'value' in positionsRef) {
    watch(
      () => positionsRef.value,
      (newPositions, oldPositions) => {
        const wasEmpty = !oldPositions || oldPositions.length === 0;
        const isNowEmpty = !newPositions || newPositions.length === 0;
        
        if (!wasEmpty && isNowEmpty) {
          return;
        }
        
        if (!wasEmpty && !isNowEmpty) {
          const oldSymbols = new Set((oldPositions || []).map(p => p.symbol));
          const newSymbols = new Set((newPositions || []).map(p => p.symbol));
          
          let hasChange = false;
          if (oldSymbols.size !== newSymbols.size) {
            hasChange = true;
          } else {
            for (const sym of newSymbols) {
              if (!oldSymbols.has(sym)) {
                hasChange = true;
                break;
              }
            }
          }
          
          if (hasChange) {
            reset();
          }
        }
      },
      { deep: false }
    );
  } else {
    watch(
      () => {
        const val = typeof positions === 'function' ? positions() : positions;
        return val?.length ?? 0;
      },
      () => {
        reset();
      }
    );
  }

  return {
    selectedDate,
    selectedIVOverride,
    showExpirationLine,
    showTheoreticalLine,
    
    defaultIV,
    effectiveIV,
    minDate,
    maxDate,
    ivAvailable,
    
    reset,
    incrementDate,
    decrementDate,
    incrementIV,
    decrementIV,
  };
}

export default useSimulationState;