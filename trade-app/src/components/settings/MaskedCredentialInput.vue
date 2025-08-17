<template>
  <div class="masked-credential-input">
    <div class="input-wrapper">
      <InputText
        :value="displayValue"
        @input="handleInput"
        :type="inputType"
        :placeholder="placeholder"
        :class="{ 'has-existing-value': hasExistingValue && !isChanged }"
        :autocomplete="inputType === 'password' ? 'current-password' : 'off'"
        class="credential-input"
      />
      
      <div class="input-indicators">
        <!-- Existing value indicator -->
        <div 
          v-if="hasExistingValue && !isChanged && isSensitive" 
          class="existing-value-indicator"
          title="Value is set in backend"
        >
          <i class="pi pi-check-circle"></i>
          <span>Set</span>
        </div>
        
        <!-- Changed indicator -->
        <div 
          v-if="isChanged" 
          class="changed-indicator"
          title="Value has been modified"
        >
          <i class="pi pi-pencil"></i>
          <span>Modified</span>
        </div>
      </div>
    </div>
    
    <!-- Field hint -->
    <small v-if="fieldHint" class="field-hint">
      {{ fieldHint }}
    </small>
  </div>
</template>

<script>
import { ref, computed, watch } from 'vue';
import InputText from 'primevue/inputtext';

export default {
  name: 'MaskedCredentialInput',
  components: {
    InputText
  },
  props: {
    modelValue: {
      type: String,
      default: ''
    },
    placeholder: {
      type: String,
      default: ''
    },
    inputType: {
      type: String,
      default: 'text'
    },
    isSensitive: {
      type: Boolean,
      default: false
    },
    hasExistingValue: {
      type: Boolean,
      default: false
    },
    originalValue: {
      type: String,
      default: ''
    },
    defaultValue: {
      type: String,
      default: ''
    }
  },
  emits: ['update:modelValue', 'change'],
  setup(props, { emit }) {
    const showValue = ref(false);
    
    // Compute if the field has been changed
    const isChanged = computed(() => {
      if (props.isSensitive && props.hasExistingValue) {
        // For sensitive fields with existing values, check if user entered something new
        return props.modelValue && props.modelValue !== '••••••••' && props.modelValue !== props.originalValue;
      }
      return props.modelValue !== props.originalValue;
    });
    
    // Compute the display value
    const displayValue = computed(() => {
      if (props.isSensitive && props.hasExistingValue && !props.modelValue) {
        return '••••••••';
      }
      return props.modelValue;
    });
    
    // Compute field hint
    const fieldHint = computed(() => {
      if (props.defaultValue && !props.hasExistingValue) {
        return `Default: ${props.defaultValue}`;
      }
      if (props.hasExistingValue && props.isSensitive && !isChanged.value) {
        return 'Using existing value from backend';
      }
      return '';
    });
    
    const toggleShowValue = () => {
      showValue.value = !showValue.value;
    };
    
    const handleInput = (event) => {
      const value = event.target.value;
      
      // Don't emit masked placeholder values
      if (value === '••••••••') {
        return;
      }
      
      emit('update:modelValue', value);
      emit('change', {
        field: props.field,
        value: value,
        isChanged: value !== props.originalValue
      });
    };
    
    // Watch for changes to emit change events
    watch(() => props.modelValue, (newValue) => {
      emit('change', {
        value: newValue,
        isChanged: newValue !== props.originalValue
      });
    });
    
    return {
      showValue,
      isChanged,
      displayValue,
      fieldHint,
      toggleShowValue,
      handleInput
    };
  }
};
</script>

<style scoped>
.masked-credential-input {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.credential-input {
  flex: 1;
  padding-right: 80px; /* Make room for indicators */
}

.credential-input.has-existing-value {
  background-color: rgba(0, 200, 81, 0.05);
  border-color: var(--color-success);
}

.input-indicators {
  position: absolute;
  right: var(--spacing-sm);
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  pointer-events: none;
}


.existing-value-indicator,
.changed-indicator {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.existing-value-indicator {
  background-color: rgba(0, 200, 81, 0.1);
  color: var(--color-success);
  border: 1px solid var(--color-success);
}

.changed-indicator {
  background-color: rgba(255, 107, 53, 0.1);
  color: var(--color-brand);
  border: 1px solid var(--color-brand);
}

.existing-value-indicator i,
.changed-indicator i {
  font-size: var(--font-size-xs);
}

.field-hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-style: italic;
  margin-top: var(--spacing-xs);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .credential-input {
    padding-right: 60px;
  }
  
  .existing-value-indicator span,
  .changed-indicator span {
    display: none;
  }
}
</style>
