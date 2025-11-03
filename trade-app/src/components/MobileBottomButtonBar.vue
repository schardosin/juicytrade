<template>
  <div class="mobile-bottom-button-bar">
    <button
      v-for="section in sections"
      :key="section.key"
      class="bottom-button"
      :class="{ active: activeSection === section.key }"
      @click="openSection(section.key)"
    >
      <i :class="section.icon"></i>
      <span class="button-label">{{ section.label }}</span>
    </button>
  </div>
</template>

<script>
export default {
  name: 'MobileBottomButtonBar',
  props: {
    activeSection: {
      type: String,
      default: null
    }
  },
  emits: ['section-selected'],
  setup(props, { emit }) {

    const sections = [
      {
        key: 'overview',
        label: 'Overview',
        icon: 'pi pi-th-large'
      },
      {
        key: 'analysis',
        label: 'Analysis',
        icon: 'pi pi-chart-bar'
      },
      {
        key: 'activity',
        label: 'Activity',
        icon: 'pi pi-chart-line'
      },
      {
        key: 'watchlist',
        label: 'Watchlist',
        icon: 'pi pi-star'
      },
      {
        key: 'alerts',
        label: 'Alerts',
        icon: 'pi pi-clock'
      },
      {
        key: 'news',
        label: 'News',
        icon: 'pi pi-file'
      }
    ];

    const openSection = (sectionKey) => {
      emit('section-selected', sectionKey);
    };

    return {
      sections,
      openSection
    };
  }
};
</script>

<style scoped>
.mobile-bottom-button-bar {
  display: flex;
  background-color: #141519;
  border-top: 1px solid #2a2d33;
  padding: 8px 4px;
  gap: 4px;
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.3);
}

.bottom-button {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 8px 4px;
  background: none;
  border: none;
  color: #cccccc;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.2s ease;
  min-height: 56px;
  position: relative;
}

.bottom-button:hover {
  background-color: #1a1d23;
  color: #ffffff;
}

.bottom-button.active {
  background-color: #007bff;
  color: #ffffff;
}

.bottom-button i {
  font-size: 18px;
  margin-bottom: 2px;
}

.button-label {
  font-size: 10px;
  font-weight: 500;
  text-align: center;
  line-height: 1;
}

/* Touch-friendly adjustments */
@media (hover: none) and (pointer: coarse) {
  .bottom-button {
    min-height: 60px;
    padding: 10px 4px;
  }
  
  .bottom-button i {
    font-size: 20px;
  }
  
  .button-label {
    font-size: 11px;
  }
}
</style>
