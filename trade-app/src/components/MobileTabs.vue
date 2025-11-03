<template>
  <div class="mobile-tabs">
    <!-- Tab Navigation -->
    <div class="tab-nav">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        :class="['tab-item', { active: activeTab === tab.id }]"
        @click="selectTab(tab.id)"
      >
        <div class="tab-icon">
          <i :class="tab.icon"></i>
        </div>
        <span class="tab-label">{{ tab.label }}</span>
      </div>
    </div>

    <!-- Tab Content -->
    <div class="tab-content">
      <div
        v-for="tab in tabs"
        :key="`content-${tab.id}`"
        v-show="activeTab === tab.id"
        class="tab-panel"
        :class="{ active: activeTab === tab.id }"
      >
        <slot :name="tab.id" :tab="tab">
          <div class="default-content">
            <h3>{{ tab.label }}</h3>
            <p>Content for {{ tab.label }} tab</p>
          </div>
        </slot>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, watch } from 'vue';

export default {
  name: 'MobileTabs',
  props: {
    tabs: {
      type: Array,
      required: true,
      validator: (tabs) => {
        return tabs.every(tab => 
          tab.id && tab.label && tab.icon
        );
      }
    },
    defaultTab: {
      type: String,
      default: null
    },
    modelValue: {
      type: String,
      default: null
    }
  },
  emits: ['update:modelValue', 'tab-change'],
  setup(props, { emit }) {
    const activeTab = ref(
      props.modelValue || 
      props.defaultTab || 
      (props.tabs.length > 0 ? props.tabs[0].id : null)
    );

    const selectTab = (tabId) => {
      if (activeTab.value !== tabId) {
        activeTab.value = tabId;
        emit('update:modelValue', tabId);
        emit('tab-change', tabId);
      }
    };

    // Watch for external changes to modelValue
    watch(
      () => props.modelValue,
      (newValue) => {
        if (newValue && newValue !== activeTab.value) {
          activeTab.value = newValue;
        }
      }
    );

    return {
      activeTab,
      selectTab
    };
  }
};
</script>

<style scoped>
.mobile-tabs {
  display: flex;
  flex-direction: column;
  height: 100%;
  background-color: var(--bg-primary, #0b0d10);
}

.tab-nav {
  display: flex;
  background-color: var(--bg-secondary, #141519);
  border-bottom: 1px solid var(--border-primary, #1a1d23);
  overflow-x: auto;
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE/Edge */
}

.tab-nav::-webkit-scrollbar {
  display: none; /* Chrome/Safari */
}

.tab-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-width: 80px;
  padding: 12px 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  border-bottom: 2px solid transparent;
  flex-shrink: 0;
}

.tab-item:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.tab-item.active {
  background-color: var(--bg-tertiary, #1a1d23);
  border-bottom-color: var(--color-brand, #007bff);
}

.tab-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-bottom: 4px;
}

.tab-icon i {
  font-size: 16px;
  color: var(--text-secondary, #cccccc);
  transition: color 0.2s ease;
}

.tab-item:hover .tab-icon i,
.tab-item.active .tab-icon i {
  color: var(--text-primary, #ffffff);
}

.tab-item.active .tab-icon i {
  color: var(--color-brand, #007bff);
}

.tab-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-tertiary, #888888);
  transition: color 0.2s ease;
  text-align: center;
  line-height: 1.2;
  white-space: nowrap;
}

.tab-item:hover .tab-label,
.tab-item.active .tab-label {
  color: var(--text-secondary, #cccccc);
}

.tab-item.active .tab-label {
  color: var(--color-brand, #007bff);
  font-weight: 600;
}

.tab-content {
  flex: 1;
  overflow: hidden;
  position: relative;
}

.tab-panel {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  overflow-y: auto;
  opacity: 0;
  transform: translateX(10px);
  transition: all 0.2s ease;
  pointer-events: none;
}

.tab-panel.active {
  opacity: 1;
  transform: translateX(0);
  pointer-events: auto;
}

.default-content {
  padding: 20px;
  text-align: center;
  color: var(--text-secondary, #cccccc);
}

.default-content h3 {
  margin: 0 0 12px 0;
  color: var(--text-primary, #ffffff);
}

.default-content p {
  margin: 0;
  color: var(--text-tertiary, #888888);
}

/* Scrollbar styling for tab panels */
.tab-panel::-webkit-scrollbar {
  width: 6px;
}

.tab-panel::-webkit-scrollbar-track {
  background: var(--bg-secondary, #141519);
}

.tab-panel::-webkit-scrollbar-thumb {
  background: var(--border-secondary, #2a2d33);
  border-radius: 3px;
}

.tab-panel::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary, #3a3d43);
}

/* Mobile-specific adjustments */
@media (max-width: 480px) {
  .tab-item {
    min-width: 70px;
    padding: 10px 6px;
  }
  
  .tab-icon {
    width: 20px;
    height: 20px;
  }
  
  .tab-icon i {
    font-size: 14px;
  }
  
  .tab-label {
    font-size: 10px;
  }
}

/* Touch-friendly tap targets */
@media (hover: none) and (pointer: coarse) {
  .tab-item {
    min-height: 48px; /* Minimum touch target size */
  }
}

/* Smooth scrolling for tab navigation */
.tab-nav {
  scroll-behavior: smooth;
}

/* Active tab indicator animation */
.tab-item::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 50%;
  width: 0;
  height: 2px;
  background-color: var(--color-brand, #007bff);
  transition: all 0.3s ease;
  transform: translateX(-50%);
}

.tab-item.active::after {
  width: 80%;
}
</style>
