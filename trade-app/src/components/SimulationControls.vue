<template>
  <div class="simulation-controls">
    <!-- Row 1: Main controls — Date + IV + Reset -->
    <div class="controls-main-row">
      <div class="control-zone date-zone">
        <span class="zone-label">Date</span>
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
      <div class="control-zone iv-zone">
        <span class="zone-label">IV</span>
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
      <Button
        icon="pi pi-refresh"
        class="p-button-sm p-button-text p-button-rounded reset-btn"
        @click="handleReset"
        :disabled="isDefaultState || !hasPositions"
        v-tooltip="'Reset to Defaults'"
      />
    </div>

    <!-- Row 2: Hint/info text -->
    <div v-if="hasPositions && (maxDateValue || ivAvailable)" class="controls-info-row">
      <div class="date-hint">
        <span v-if="maxDateValue">Exp: {{ formatDate(maxDateValue) }}</span>
      </div>
      <div class="iv-info">
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
      <div></div>
    </div>

    <!-- Row 3: Line toggles -->
    <div class="controls-toggles-row">
      <div class="toggle-item">
        <Checkbox
          v-model="showExpirationLineValue"
          inputId="exp-line"
          :binary="true"
          :disabled="!hasPositions"
        />
        <label for="exp-line" class="toggle-label">P/L at Expiration</label>
      </div>
      <div class="toggle-item">
        <Checkbox
          v-model="showTheoreticalLineValue"
          inputId="theo-line"
          :binary="true"
          :disabled="!hasPositions || !ivAvailable"
        />
        <label for="theo-line" class="toggle-label">P/L Theoretical</label>
      </div>
    </div>
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
/* Container */
.simulation-controls {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  background-color: var(--bg-primary, #0b0d10);
  border-radius: var(--radius-md, 6px);
}

/* Row 1: Main controls — 3-column grid */
.controls-main-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}

/* Each control zone: inline flex row */
.control-zone {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

/* Inline zone labels */
.zone-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-secondary, #cccccc);
  white-space: nowrap;
  line-height: 28px;
  align-self: center;
}

/* Calendar and IV input sizing */
.date-calendar {
  flex: 1;
  min-width: 110px;
}

.iv-input {
  flex: 0 0 auto;
  width: 60px;
  max-width: 60px;
}

/* Reset button — icon only, compact */
.reset-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  flex-shrink: 0;
}

/* Row 2: Info/hint text — matches grid alignment */
.controls-info-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
}

.date-hint {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  padding-left: 32px;
}

.iv-info {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  padding-left: 20px;
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

/* Row 3: Toggle checkboxes — horizontal */
.controls-toggles-row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding-top: 2px;
}

.toggle-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.toggle-label {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  cursor: pointer;
  white-space: nowrap;
}

/* Responsive: narrow panels */
@media (max-width: 900px) {
  .controls-main-row {
    grid-template-columns: 1fr auto;
  }
  .iv-zone {
    grid-column: 1;
  }
  .reset-btn {
    grid-row: 1;
    grid-column: 2;
  }
  .controls-info-row {
    grid-template-columns: 1fr 1fr;
  }
}

/* PrimeVue deep overrides */
:deep(.p-calendar-input) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
  color: var(--text-primary, #ffffff) !important;
  height: 28px;
  padding: 4px 8px;
  font-size: 11px;
}

:deep(.p-calendar-input:focus) {
  border-color: var(--color-brand, #ff6b35) !important;
}

:deep(.p-inputnumber-input) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
  color: var(--text-primary, #ffffff) !important;
  text-align: center;
  height: 28px;
  padding: 4px 6px;
  font-size: 11px;
}

:deep(.p-inputnumber-input:focus) {
  border-color: var(--color-brand, #ff6b35) !important;
}

:deep(.p-datepicker-trigger) {
  height: 28px !important;
  width: 28px !important;
}

.control-zone :deep(.p-button.p-button-sm) {
  height: 28px;
  width: 28px;
  min-width: 28px;
  padding: 0;
}

:deep(.p-checkbox-box) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--text-tertiary, #555555) !important;
}

:deep(.p-checkbox-box.p-highlight) {
  background-color: #007bff !important;
  border-color: #007bff !important;
}

:deep(.p-button.p-button-text) {
  color: var(--text-secondary, #cccccc);
}

:deep(.p-button.p-button-text:hover) {
  background-color: var(--bg-tertiary, #1a1d23);
  color: var(--text-primary, #ffffff);
}
</style>