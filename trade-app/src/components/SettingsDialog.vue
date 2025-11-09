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
        <div v-if="activeTab === 'providers'" class="tab-content providers-tab-content">
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

        <!-- About Tab -->
        <div v-else-if="activeTab === 'about'" class="tab-content about-content">
          <div class="about-header">
            <img src="/logos/juicytrade-logo.svg" alt="Juicy Trade Logo" class="about-logo" />
            <p class="about-subtitle">Sophisticated Free Options Trading Platform</p>
          </div>

          <div class="about-description">
            <p>
              A sophisticated options trading application with a modular, multi-provider architecture, 
              featuring a FastAPI backend and a Vue.js frontend. The application is designed to support 
              multiple brokerage providers, with a clear separation between live and paper trading environments.
            </p>
          </div>

          <div class="about-features">
            <h3>Key Features</h3>
            <div class="features-grid">
              <div class="feature-item">
                <i class="pi pi-server"></i>
                <div>
                  <h4>Multi-Provider Support</h4>
                  <p>Alpaca, Tradier, TastyTrade, and Public.com integration</p>
                </div>
              </div>
              <div class="feature-item">
                <i class="pi pi-shield"></i>
                <div>
                  <h4>Comprehensive Authentication</h4>
                  <p>OAuth, simple auth, token-based, and enterprise SSO</p>
                </div>
              </div>
              <div class="feature-item">
                <i class="pi pi-chart-line"></i>
                <div>
                  <h4>Professional Trading Interface</h4>
                  <p>Real-time streaming, options chains, and payoff charts</p>
                </div>
              </div>
              <div class="feature-item">
                <i class="pi pi-cog"></i>
                <div>
                  <h4>Advanced Options Calculator</h4>
                  <p>Greeks, P&L analysis, and risk management</p>
                </div>
              </div>
              <div class="feature-item">
                <i class="pi pi-wifi"></i>
                <div>
                  <h4>Sleep-Resistant Streaming</h4>
                  <p>Web Worker-based architecture with automatic recovery</p>
                </div>
              </div>
              <div class="feature-item">
                <i class="pi pi-eye"></i>
                <div>
                  <h4>Live & Paper Trading</h4>
                  <p>Separate configurations for testing and live trading</p>
                </div>
              </div>
            </div>
          </div>

          <div class="about-providers">
            <h3>Supported Providers</h3>
            <div class="providers-grid">
              <div class="provider-item">
                <img src="/logos/alpaca.svg" alt="Alpaca" class="provider-logo" />
                <div>
                  <h4>Alpaca Markets</h4>
                  <p>Commission-free stock and options trading</p>
                </div>
              </div>
              <div class="provider-item">
                <img src="/logos/tradier.svg" alt="Tradier" class="provider-logo" />
                <div>
                  <h4>Tradier</h4>
                  <p>Professional trading platform with advanced options</p>
                </div>
              </div>
              <div class="provider-item">
                <img src="/logos/tastytrade.svg" alt="TastyTrade" class="provider-logo" />
                <div>
                  <h4>TastyTrade</h4>
                  <p>Options-focused platform with DXLink streaming</p>
                </div>
              </div>
              <div class="provider-item">
                <img src="/logos/public.svg" alt="Public.com" class="provider-logo" />
                <div>
                  <h4>Public.com</h4>
                  <p>Market data provider for quotes and historical data</p>
                </div>
              </div>
            </div>
          </div>

          <div class="about-links">
            <h3>Resources</h3>
            <div class="links-grid">
              <a 
                href="https://github.com/schardosin/juicytrade" 
                target="_blank" 
                rel="noopener noreferrer"
                class="link-item"
              >
                <i class="pi pi-github"></i>
                <div>
                  <h4>GitHub Repository</h4>
                  <p>View source code, report issues, and contribute</p>
                </div>
                <i class="pi pi-external-link"></i>
              </a>
            </div>
          </div>

          <div class="about-architecture">
            <h3>Architecture</h3>
            <div class="architecture-grid">
              <div class="arch-item">
                <i class="pi pi-database"></i>
                <div>
                  <h4>FastAPI Backend</h4>
                  <p>Python-based API with WebSocket streaming and multi-provider support</p>
                </div>
              </div>
              <div class="arch-item">
                <i class="pi pi-desktop"></i>
                <div>
                  <h4>Vue.js Frontend</h4>
                  <p>Modern SPA with real-time updates and professional trading interface</p>
                </div>
              </div>
            </div>
          </div>

          <div class="about-footer">
            <p class="version-info">
              Built with modern web technologies for professional options trading
            </p>
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

.providers-tab-content {
  padding-bottom: 0;
  padding-top: 12px;
  position: relative;
  height: 100%;
  display: flex;
  flex-direction: column;
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

/* About Tab Styles */
.about-content {
  padding: var(--spacing-lg) var(--spacing-xl);
  max-width: 800px;
  margin: 0 auto;
}

.about-header {
  text-align: center;
  margin-bottom: var(--spacing-xl);
  padding-bottom: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.about-logo {
  width: 200px;
  margin-bottom: var(--spacing-xs);
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.1));
}

.about-title {
  font-size: 2.5rem;
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-sm) 0;
  background: linear-gradient(135deg, var(--color-info), var(--color-success));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.about-subtitle {
  font-size: var(--font-size-lg);
  color: var(--text-secondary);
  margin: 0;
  font-weight: var(--font-weight-medium);
}

.about-description {
  margin-bottom: var(--spacing-xl);
}

.about-description p {
  font-size: var(--font-size-md);
  line-height: 1.6;
  color: var(--text-secondary);
  text-align: center;
  max-width: 700px;
  margin: 0 auto;
}

.about-features,
.about-providers,
.about-links,
.about-architecture {
  margin-bottom: var(--spacing-xl);
}

.about-features h3,
.about-providers h3,
.about-links h3,
.about-architecture h3 {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-lg) 0;
  text-align: center;
}

.features-grid,
.providers-grid,
.links-grid,
.architecture-grid {
  display: grid;
  gap: var(--spacing-lg);
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
}

.feature-item,
.provider-item,
.arch-item {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background-color: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.feature-item:hover,
.provider-item:hover,
.arch-item:hover {
  border-color: var(--color-info);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.feature-item i,
.arch-item i {
  font-size: 1.5rem;
  color: var(--color-info);
  margin-top: 2px;
  flex-shrink: 0;
}

.provider-logo {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  margin-top: 2px;
}

.feature-item h4,
.provider-item h4,
.arch-item h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.feature-item p,
.provider-item p,
.arch-item p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.4;
}

.link-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  background-color: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  text-decoration: none;
  color: inherit;
  transition: var(--transition-normal);
  position: relative;
}

.link-item:hover {
  border-color: var(--color-info);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  text-decoration: none;
  color: inherit;
}

.link-item i.pi-github {
  font-size: 1.5rem;
  color: var(--color-info);
  flex-shrink: 0;
}

.link-item i.pi-external-link {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  margin-left: auto;
  flex-shrink: 0;
}

.link-item h4 {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.link-item p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.4;
}

.about-footer {
  text-align: center;
  padding-top: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
  margin-top: var(--spacing-xl);
}

.version-info {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  margin: 0;
  font-style: italic;
}

/* Responsive adjustments for About section */
@media (max-width: 768px) {
  .about-content {
    padding: var(--spacing-md);
  }
  
  .about-logo {
    width: 120px;
    height: 120px;
  }
  
  .about-title {
    font-size: 2rem;
  }
  
  .features-grid,
  .providers-grid,
  .architecture-grid {
    grid-template-columns: 1fr;
  }
}
</style>
