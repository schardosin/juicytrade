<template>
  <div class="simulation-controls">
    <div class="control-group date-selector">
      <label class="control-label">Evaluate at Date</label>
      <div class="control-row">
        <Button
          icon="pi pi-chevron-left"
          class="p-button-sm p-button-text p-button-plain"
          @click="handleDecrementDate"
          :disabled="isMinDate || !hasPositions"
          v-tooltip="'Previous day'"
        />
        <Calendar
          v-model="selectedDateValue"
          :minDate="minDateValue"
          :maxDate="maxDateValue"
          dateFormat="M d, yy"
          :showIcon="true"
          class="date-calendar"
          :disabled="!hasPositions"
          :manualInput="false"
        />
        <Button
          icon="pi pi-chevron-right"
          class="p-button-sm p-button-text p-button-plain"
          @click="handleIncrementDate"
          :disabled="isMaxDate || !hasPositions"
          v-tooltip="'Next day'"
        />
      </div>
      <div v-if="hasPositions && maxDateValue" class="date-hint">
        Expires: {{ formatDate(maxDateValue) }}
      </div>
    </div>

    <div class="control-group iv-selector">
      <label class="control-label">Implied Volatility</label>
      <div class="control-row">
        <Button
          label="-"
          class="p-button-sm p-button-text p-button-plain"
          @click="handleDecrementIV"
          :disabled="isMinIV || !ivAvailable || !hasPositions"
          v-tooltip="ivTooltip"
        />
        <InputNumber
          v-model="ivPercentValue"
          :min="5"
          :max="200"
          :step="1"
          suffix="%"
          :disabled="!ivAvailable || !hasPositions"
          class="iv-input"
          v-tooltip="ivTooltip"
        />
        <Button
          label="+"
          class="p-button-sm p-button-text p-button-plain"
          @click="handleIncrementIV"
          :disabled="isMaxIV || !ivAvailable || !hasPositions"
          v-tooltip="ivTooltip"
        />
      </div>
      <div v-if="hasPositions" class="iv-info">
        <span v-if="ivAvailable && defaultIVValue !== null">
          Current: {{ formatPercent(defaultIVValue) }}
          <span
            v-if="ivDeltaValue !== 0"
            :class="ivDeltaValue > 0 ? 'iv-delta-positive' : 'iv-delta-negative'"
          >
            ({{ ivDeltaValue > 0 ? '+' : '' }}{{ ivDeltaValue.toFixed(1) }}%)
          </span>
        </span>
        <span v-else class="iv-unavailable">
          IV data unavailable
        </span>
      </div>
    </div>

    <div class="control-group line-toggles">
      <label class="control-label">Lines</label>
      <div class="toggle-row">
        <Checkbox
          v-model="showExpirationLineValue"
          inputId="exp-line"
          :binary="true"
          :disabled="!hasPositions"
        />
        <label for="exp-line" class="toggle-label">P/L at Expiration</label>
      </div>
      <div class="toggle-row">
        <Checkbox
          v-model="showTheoreticalLineValue"
          inputId="theo-line"
          :binary="true"
          :disabled="!hasPositions || !ivAvailable"
        />
        <label for="theo-line" class="toggle-label">P/L Theoretical</label>
      </div>
    </div>

    <Button
      label="Reset to Defaults"
      icon="pi pi-refresh"
      class="p-button-sm p-button-outlined reset-btn"
      @click="handleReset"
      :disabled="isDefaultState || !hasPositions"
    />
  </div>
</template>

<script>
import { computed, ref, watch } from 'vue';
import Button from 'primevue/button';
import Calendar from 'primevue/calendar';
import InputNumber from 'primevue/inputnumber';
import Checkbox from 'primevue/checkbox';
import { useSimulationState } from '../composables/useSimulationState.js';

export default {
  name: 'SimulationControls',
  components: {
    Button,
    Calendar,
    InputNumber,
    Checkbox,
  },
  props: {
    positions: {
      type: Array,
      default: () => [],
    },
    marketData: {
      type: Object,
      required: true,
    },
    modelValue: {
      type: Object,
      default: null,
    },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const {
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
    } = useSimulationState(computed(() => props.positions), props.marketData);

    const selectedDateValue = computed({
      get: () => selectedDate.value,
      set: (val) => {
        if (val) {
          const normalized = new Date(val);
          normalized.setHours(0, 0, 0, 0);
          selectedDate.value = normalized;
        }
      },
    });

    const ivPercentValue = computed({
      get: () => {
        const iv = effectiveIV.value;
        return iv !== null ? Math.round(iv * 100) : null;
      },
      set: (val) => {
        if (val !== null && val !== undefined) {
          selectedIVOverride.value = val / 100;
        }
      },
    });

    const showExpirationLineValue = computed({
      get: () => showExpirationLine.value,
      set: (val) => {
        showExpirationLine.value = val;
      },
    });

    const showTheoreticalLineValue = computed({
      get: () => showTheoreticalLine.value,
      set: (val) => {
        showTheoreticalLine.value = val;
      },
    });

    const hasPositions = computed(() => {
      return props.positions && props.positions.length > 0;
    });

    const minDateValue = computed(() => minDate.value);
    const maxDateValue = computed(() => maxDate.value);
    const defaultIVValue = computed(() => defaultIV.value);

    const isMinDate = computed(() => {
      if (!minDate.value) return true;
      const current = new Date(selectedDate.value);
      current.setHours(0, 0, 0, 0);
      const min = new Date(minDate.value);
      min.setHours(0, 0, 0, 0);
      return current.getTime() <= min.getTime();
    });

    const isMaxDate = computed(() => {
      if (!maxDate.value) return true;
      const current = new Date(selectedDate.value);
      current.setHours(0, 0, 0, 0);
      const max = new Date(maxDate.value);
      max.setHours(0, 0, 0, 0);
      return current.getTime() >= max.getTime();
    });

    const isMinIV = computed(() => {
      const currentIV = effectiveIV.value ?? 0.05;
      return currentIV <= 0.05;
    });

    const isMaxIV = computed(() => {
      const currentIV = effectiveIV.value ?? 2.0;
      return currentIV >= 2.0;
    });

    const ivDeltaValue = computed(() => {
      if (defaultIV.value === null || selectedIVOverride.value === null) {
        return 0;
      }
      return (selectedIVOverride.value - defaultIV.value) * 100;
    });

    const ivTooltip = computed(() => {
      if (!hasPositions.value) {
        return 'No positions selected';
      }
      if (!ivAvailable.value) {
        return 'IV data not available for all legs';
      }
      return '';
    });

    const isDefaultState = computed(() => {
      const today = new Date();
      today.setHours(0, 0, 0, 0);
      const selectedNormalized = new Date(selectedDate.value);
      selectedNormalized.setHours(0, 0, 0, 0);
      const dateIsDefault = selectedNormalized.getTime() === today.getTime();
      const ivIsDefault = selectedIVOverride.value === null;
      const linesAreDefault = showExpirationLine.value === true && showTheoreticalLine.value === true;
      return dateIsDefault && ivIsDefault && linesAreDefault;
    });

    const formatDate = (date) => {
      if (!date) return '';
      const d = new Date(date);
      return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    };

    const formatPercent = (decimal) => {
      if (decimal === null || decimal === undefined) return '';
      return (decimal * 100).toFixed(1) + '%';
    };

    const handleDecrementDate = () => decrementDate();
    const handleIncrementDate = () => incrementDate();
    const handleDecrementIV = () => decrementIV();
    const handleIncrementIV = () => incrementIV();
    const handleReset = () => reset();

    const emitState = () => {
      emit('update:modelValue', {
        selectedDate: selectedDate.value,
        selectedIVOverride: selectedIVOverride.value,
        showExpirationLine: showExpirationLine.value,
        showTheoreticalLine: showTheoreticalLine.value,
        effectiveIV: effectiveIV.value,
        defaultIV: defaultIV.value,
        ivAvailable: ivAvailable.value,
        minDate: minDate.value,
        maxDate: maxDate.value,
      });
    };

    watch(
      [selectedDate, selectedIVOverride, showExpirationLine, showTheoreticalLine],
      () => {
        emitState();
      },
      { deep: true }
    );

    emitState();

    return {
      selectedDateValue,
      ivPercentValue,
      showExpirationLineValue,
      showTheoreticalLineValue,
      hasPositions,
      minDateValue,
      maxDateValue,
      defaultIVValue,
      isMinDate,
      isMaxDate,
      isMinIV,
      isMaxIV,
      ivAvailable,
      ivDeltaValue,
      ivTooltip,
      isDefaultState,
      formatDate,
      formatPercent,
      handleDecrementDate,
      handleIncrementDate,
      handleDecrementIV,
      handleIncrementIV,
      handleReset,
    };
  },
};
</script>

<style scoped>
.simulation-controls {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 12px;
  background-color: var(--bg-primary, #0b0d10);
  border-radius: var(--radius-md, 6px);
}

.control-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.control-label {
  font-size: var(--font-size-sm, 11px);
  font-weight: var(--font-weight-medium, 500);
  color: var(--text-secondary, #cccccc);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.control-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.date-calendar {
  flex: 1;
}

.date-hint {
  font-size: var(--font-size-xs, 10px);
  color: var(--text-tertiary, #888888);
}

.iv-input {
  flex: 1;
  min-width: 80px;
}

.iv-info {
  font-size: var(--font-size-xs, 10px);
  color: var(--text-tertiary, #888888);
}

.iv-unavailable {
  color: var(--color-warning, #ffbb33);
}

.iv-delta-positive {
  color: var(--color-success, #00c851);
}

.iv-delta-negative {
  color: var(--color-danger, #ff4444);
}

.line-toggles {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.toggle-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toggle-label {
  font-size: var(--font-size-base, 12px);
  color: var(--text-primary, #ffffff);
  cursor: pointer;
}

.reset-btn {
  width: 100%;
  margin-top: 8px;
}

:deep(.p-calendar-input) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
  color: var(--text-primary, #ffffff) !important;
}

:deep(.p-calendar-input:focus) {
  border-color: var(--color-brand, #ff6b35) !important;
}

:deep(.p-inputnumber-input) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
  color: var(--text-primary, #ffffff) !important;
  text-align: center;
}

:deep(.p-inputnumber-input:focus) {
  border-color: var(--color-brand, #ff6b35) !important;
}

:deep(.p-checkbox-box) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
}

:deep(.p-checkbox-box.p-highlight) {
  background-color: var(--color-brand, #ff6b35) !important;
  border-color: var(--color-brand, #ff6b35) !important;
}

:deep(.p-button.p-button-text) {
  color: var(--text-secondary, #cccccc);
}

:deep(.p-button.p-button-text:hover) {
  background-color: var(--bg-tertiary, #1a1d23);
  color: var(--text-primary, #ffffff);
}

:deep(.p-button.p-button-outlined) {
  border-color: var(--border-secondary, #2a2d33);
  color: var(--text-secondary, #cccccc);
}

:deep(.p-button.p-button-outlined:hover) {
  background-color: var(--bg-tertiary, #1a1d23);
  border-color: var(--color-brand, #ff6b35);
  color: var(--color-brand, #ff6b35);
}
</style>