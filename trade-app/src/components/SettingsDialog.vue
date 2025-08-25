<template>
  <Dialog
    v-model:visible="isVisible"
    modal
    :closable="true"
    :draggable="false"
    class="settings-dialog"
    header="Settings"
    :style="{ width: '1100px', height: '800px' }"
    @hide="onClose"
  >
    <div class="settings-container">
      <!-- Left Sidebar -->
      <div class="settings-sidebar">
        <nav class="sidebar-nav">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            :class="['nav-item', { active: activeTab === tab.key }]"
            @click="setActiveTab(tab.key)"
          >
            <i :class="tab.icon"></i>
            <span>{{ tab.label }}</span>
          </button>
        </nav>
      </div>

      <!-- Main Content -->
      <div class="settings-content">
        <!-- Providers Tab -->
        <div v-if="activeTab === 'providers'" class="tab-content">
          <ProvidersTab />
        </div>

        <!-- General Tab (Placeholder) -->
        <div v-else-if="activeTab === 'general'" class="tab-content">
          <div class="placeholder-content">
            <h3>General Settings</h3>
            <p>General settings will be implemented here.</p>
          </div>
        </div>

        <!-- Trading Tab (Placeholder) -->
        <div v-else-if="activeTab === 'trading'" class="tab-content">
          <div class="placeholder-content">
            <h3>Trading Settings</h3>
            <p>Trading preferences will be implemented here.</p>
          </div>
        </div>

        <!-- Notifications Tab (Placeholder) -->
        <div v-else-if="activeTab === 'notifications'" class="tab-content">
          <div class="placeholder-content">
            <h3>Notification Settings</h3>
            <p>Notification preferences will be implemented here.</p>
          </div>
        </div>

        <!-- About Tab (Placeholder) -->
        <div v-else-if="activeTab === 'about'" class="tab-content">
          <div class="placeholder-content">
            <h3>About</h3>
            <p>Application information and version details.</p>
          </div>
        </div>
      </div>
    </div>
  </Dialog>
</template>

<script>
import { ref, computed } from "vue";
import Dialog from "primevue/dialog";
import ProvidersTab from "./settings/ProvidersTab.vue";

export default {
  name: "SettingsDialog",
  components: {
    Dialog,
    ProvidersTab,
  },
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:visible"],
  setup(props, { emit }) {
    const activeTab = ref("providers");

    // Available tabs
    const tabs = [
      {
        key: "providers",
        label: "Providers",
        icon: "pi pi-server",
      },
      {
        key: "general",
        label: "General",
        icon: "pi pi-cog",
      },
      {
        key: "trading",
        label: "Trading",
        icon: "pi pi-chart-line",
      },
      {
        key: "notifications",
        label: "Notifications",
        icon: "pi pi-bell",
      },
      {
        key: "about",
        label: "About",
        icon: "pi pi-info-circle",
      },
    ];

    // Computed visibility
    const isVisible = computed({
      get: () => props.visible,
      set: (value) => emit("update:visible", value),
    });

    // Methods
    const setActiveTab = (tabKey) => {
      activeTab.value = tabKey;
    };

    const onClose = () => {
      emit("update:visible", false);
    };

    return {
      activeTab,
      tabs,
      isVisible,
      setActiveTab,
      onClose,
    };
  },
};
</script>

<style>
.p-dialog .p-dialog-content {
    background-color: var(--bg-secondary) !important;
    color: var(--text-secondary) !important;
    padding: 0;
}
</style>

<style scoped>
.settings-dialog {
  --dialog-bg: var(--bg-secondary);
  --dialog-border: var(--border-primary);
}

:deep(.p-dialog) {
  background-color: var(--bg-primary) !important;
  border: 1px solid var(--border-primary) !important;
  border-radius: var(--radius-lg) !important;
  box-shadow: var(--shadow-lg) !important;
  padding: 0 !important;
  margin: 0 !important;
}

:deep(.p-dialog-header) {
  background-color: var(--bg-primary) !important;
  border-bottom: 1px solid var(--border-primary) !important;
  color: var(--text-primary) !important;
  padding: var(--spacing-lg) var(--spacing-xl) !important;
  margin: 0 !important;
}

:deep(.p-dialog-title) {
  font-size: var(--font-size-lg) !important;
  font-weight: var(--font-weight-semibold) !important;
  color: var(--text-primary) !important;
}

:deep(.p-dialog-header-icon) {
  color: var(--text-secondary) !important;
}

:deep(.p-dialog-header-icon:hover) {
  background-color: var(--bg-tertiary) !important;
  color: var(--text-primary) !important;
}

.settings-container {
  display: flex;
  height: calc(800px - 80px);
  min-height: calc(800px - 80px);
  width: 100%;
  background-color: var(--bg-secondary);
  margin: 0;
  padding: 0;
}

.settings-sidebar {
  width: 250px;
  min-width: 250px;
  max-width: 250px;
  background-color: var(--bg-primary);
  border-right: 1px solid var(--border-primary);
  display: flex;
  flex-direction: column;
  height: calc(800px - 80px);
  min-height: calc(800px - 80px);
  flex-shrink: 0;
  flex-grow: 0;
}

.sidebar-nav {
  flex: 1;
  padding: var(--spacing-md) 0;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  width: 100%;
  padding: var(--spacing-md) var(--spacing-lg);
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-normal);
  text-align: left;
  position: relative;
}

.nav-item:hover {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.nav-item.active {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
  border-right: 3px solid var(--color-info);
}

.nav-item i {
  font-size: var(--font-size-md);
  width: 16px;
  text-align: center;
}

.nav-item span {
  font-weight: var(--font-weight-medium);
}

.nav-item.active span {
  font-weight: var(--font-weight-semibold);
}

.settings-content {
  flex: 1;
  background-color: var(--bg-secondary);
  overflow-y: auto;
  min-height: 100%;
}

.tab-content {
  min-height: 100%;
  padding: var(--spacing-xl);
}

.placeholder-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  text-align: center;
  color: var(--text-secondary);
}

.placeholder-content h3 {
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.placeholder-content p {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-tertiary);
  max-width: 400px;
  line-height: 1.5;
}

/* Custom scrollbar for content area */
.settings-content::-webkit-scrollbar {
  width: 6px;
}

.settings-content::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

.settings-content::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.settings-content::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}
</style>
