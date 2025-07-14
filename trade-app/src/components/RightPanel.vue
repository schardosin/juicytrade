<template>
  <div class="right-panel" :class="{ expanded: isExpanded }">
    <!-- Left Icon Menu (Always Visible) -->
    <div class="left-icon-menu">
      <div
        class="menu-icon"
        :class="{ active: activeSection === 'overview' }"
        @click="toggleSection('overview')"
        title="Overview"
      >
        <i class="pi pi-th-large"></i>
        <span class="icon-label">OVW</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'positions' }"
        @click="toggleSection('positions')"
        title="Positions"
      >
        <i class="pi pi-briefcase"></i>
        <span class="icon-label">POS</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'activity' }"
        @click="toggleSection('activity')"
        title="Activity"
      >
        <i class="pi pi-chart-line"></i>
        <span class="icon-label">ACT</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'watchlist' }"
        @click="toggleSection('watchlist')"
        title="Watchlist"
      >
        <i class="pi pi-star"></i>
        <span class="icon-label">WLT</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'alerts' }"
        @click="toggleSection('alerts')"
        title="Alerts"
      >
        <i class="pi pi-clock"></i>
        <span class="icon-label">ALR</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'options' }"
        @click="toggleSection('options')"
        title="Options Chain Helper"
      >
        <i class="pi pi-calendar"></i>
        <span class="icon-label">TTL</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'options-helper' }"
        @click="toggleSection('options-helper')"
        title="Options Helper"
      >
        <i class="pi pi-link"></i>
        <span class="icon-label">OCH</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'news' }"
        @click="toggleSection('news')"
        title="News"
      >
        <i class="pi pi-file"></i>
        <span class="icon-label">NWS</span>
      </div>

      <div
        class="menu-icon"
        :class="{ active: activeSection === 'analysis' }"
        @click="toggleSection('analysis')"
        title="Analysis"
      >
        <i class="pi pi-chart-bar"></i>
        <span class="icon-label">ANYS</span>
      </div>
    </div>

    <!-- Expandable Content Area -->
    <div class="content-area" :class="{ visible: isExpanded }">
      <div class="content-header">
        <h3 class="content-title">{{ getSectionTitle(activeSection) }}</h3>
        <button
          class="collapse-btn"
          @click="collapsePanel"
          title="Collapse Panel"
        >
          <i class="pi pi-chevron-right"></i>
        </button>
      </div>

      <div class="content-body">
        <!-- Overview Section -->
        <div v-if="activeSection === 'overview'" class="section-content">
          <RightPanelSection
            title="SPY Chart"
            icon="pi pi-chart-line"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <div class="chart-wrapper">
              <PayoffChart
                v-if="chartData"
                :chartData="chartData"
                :underlyingPrice="currentPrice"
                title="Position P&L"
                :showInfo="false"
                height="350px"
                :symbol="currentSymbol"
                :isLivePrice="isLivePrice"
              />
              <div v-else class="no-chart-message">
                <p>Select options to view payoff chart</p>
              </div>
            </div>
          </RightPanelSection>

          <RightPanelSection
            title="SPY Quote Details"
            icon="pi pi-list"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <QuoteDetailsSection
              :symbol="currentSymbol"
              :currentPrice="currentPrice"
              :priceChange="priceChange"
              :additionalQuoteData="additionalQuoteData"
            />
          </RightPanelSection>

          <RightPanelSection
            title="SPY Position Detail"
            icon="pi pi-briefcase"
            :defaultExpanded="true"
            @toggle="onSectionToggle"
          >
            <div class="position-details">
              <div class="position-summary">
                <div class="summary-row">
                  <span class="label">Qty</span>
                  <span class="label">Description</span>
                  <span class="label">Trade Price</span>
                  <span class="label">Mark</span>
                </div>
                <div v-if="selectedOptions.length === 0" class="no-positions">
                  No items to show
                </div>
                <div v-else>
                  <div
                    v-for="option in selectedOptions"
                    :key="option.symbol"
                    class="position-row"
                  >
                    <span class="qty">{{ option.quantity }}</span>
                    <span class="description">{{
                      formatOptionDescription(option)
                    }}</span>
                    <span class="trade-price"
                      >${{ formatPrice(option.price) }}</span
                    >
                    <span class="mark"
                      >${{ formatPrice(option.currentPrice) }}</span
                    >
                  </div>
                </div>
              </div>
            </div>
          </RightPanelSection>
        </div>

        <!-- Other Sections (Placeholder) -->
        <div v-else class="section-content">
          <div class="placeholder-content">
            <h4>{{ getSectionTitle(activeSection) }}</h4>
            <p>This section is coming soon...</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed } from "vue";
import RightPanelSection from "./RightPanelSection.vue";
import QuoteDetailsSection from "./QuoteDetailsSection.vue";
import PayoffChart from "./PayoffChart.vue";

export default {
  name: "RightPanel",
  components: {
    RightPanelSection,
    QuoteDetailsSection,
    PayoffChart,
  },
  props: {
    currentSymbol: {
      type: String,
      required: true,
    },
    currentPrice: {
      type: Number,
      default: 0,
    },
    priceChange: {
      type: Number,
      default: 0,
    },
    isLivePrice: {
      type: Boolean,
      default: false,
    },
    selectedOptions: {
      type: Array,
      default: () => [],
    },
    chartData: {
      type: Object,
      default: null,
    },
    additionalQuoteData: {
      type: Object,
      default: () => ({}),
    },
  },
  emits: ["panel-collapsed"],
  setup(props, { emit }) {
    const activeSection = ref("overview");
    const isExpanded = ref(false);

    const toggleSection = (section) => {
      if (activeSection.value === section && isExpanded.value) {
        // If clicking the same active section, collapse
        isExpanded.value = false;
      } else {
        // Switch to new section and expand
        activeSection.value = section;
        isExpanded.value = true;
      }
    };

    const collapsePanel = () => {
      isExpanded.value = false;
      emit("panel-collapsed");
    };

    const getSectionTitle = (section) => {
      const titles = {
        overview: "Overview",
        positions: "Positions",
        activity: "Activity",
        watchlist: "Watchlist",
        alerts: "Alerts",
        options: "Options",
        "options-helper": "Options Chain Helper",
        news: "News",
        analysis: "Analysis",
      };
      return titles[section] || "Section";
    };

    const onSectionToggle = (data) => {
      console.log("Section toggled:", data);
    };

    const formatOptionDescription = (option) => {
      const type = option.type?.toUpperCase() || "CALL";
      const strike = option.strike_price || option.strike || 0;
      const expiry = option.expiry || "";
      return `${type} ${strike} ${expiry}`;
    };

    const formatPrice = (price) => {
      if (price === null || price === undefined) return "0.00";
      return Number(price).toFixed(2);
    };

    return {
      activeSection,
      isExpanded,
      toggleSection,
      collapsePanel,
      getSectionTitle,
      onSectionToggle,
      formatOptionDescription,
      formatPrice,
    };
  },
};
</script>

<style scoped>
.right-panel {
  display: flex;
  background-color: var(--bg-primary, #0b0d10);
  border-left: 1px solid var(--border-primary, #1a1d23);
  height: 100%;
  position: relative;
  z-index: 10;
}

.right-panel.expanded {
  width: 700px;
}

.left-icon-menu {
  width: 60px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-primary, #0b0d10);
  border-right: 1px solid var(--border-secondary, #2a2d33);
  padding-top: 8px;
  height: 100%;
}

.menu-icon {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 60px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  border-radius: 4px;
  margin: 2px 4px;
  position: relative;
}

.menu-icon:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.menu-icon.active {
  background-color: var(--bg-quaternary, #2a2d33);
}

.menu-icon i {
  font-size: 16px;
  color: var(--text-secondary, #cccccc);
  margin-bottom: 2px;
}

.menu-icon.active i {
  color: var(--text-primary, #ffffff);
}

.icon-label {
  font-size: 9px;
  color: var(--text-tertiary, #888888);
  font-weight: 500;
  text-align: center;
  line-height: 1;
}

.menu-icon.active .icon-label {
  color: var(--text-secondary, #cccccc);
}

.content-area {
  display: none; /* Hidden by default */
  flex-direction: column;
  width: 640px; /* Fixed width */
  flex-shrink: 0;
  background-color: #141519;
  border-left: 1px solid #2a2d33;
}

.content-area.visible {
  display: flex; /* Shown when visible */
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: var(--bg-secondary, #141519);
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
  flex-shrink: 0;
}

.content-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
}

.collapse-btn {
  background: var(--bg-tertiary, #1a1d23);
  border: 1px solid var(--border-secondary, #2a2d33);
  color: var(--text-secondary, #cccccc);
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
}

.collapse-btn:hover {
  background-color: var(--bg-quaternary, #2a2d33);
  color: var(--text-primary, #ffffff);
}

.content-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

.content-body::-webkit-scrollbar {
  width: 6px;
}

.content-body::-webkit-scrollbar-track {
  background: var(--bg-secondary, #141519);
}

.content-body::-webkit-scrollbar-thumb {
  background: var(--border-secondary, #2a2d33);
  border-radius: 3px;
}

.content-body::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary, #3a3d43);
}

.section-content {
  height: 100%;
}

.chart-wrapper {
  padding: 0;
}

.no-chart-message {
  padding: 40px 16px;
  text-align: center;
  color: var(--text-tertiary, #888888);
}

.position-details {
  padding: 16px;
}

.position-summary {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.summary-row {
  display: grid;
  grid-template-columns: 60px 1fr 80px 80px;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-secondary, #2a2d33);
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #cccccc);
}

.position-row {
  display: grid;
  grid-template-columns: 60px 1fr 80px 80px;
  gap: 12px;
  padding: 8px 0;
  font-size: 12px;
  color: var(--text-primary, #ffffff);
}

.no-positions {
  text-align: center;
  padding: 40px 16px;
  color: var(--text-tertiary, #888888);
  font-size: 14px;
}

.placeholder-content {
  padding: 40px 16px;
  text-align: center;
}

.placeholder-content h4 {
  margin: 0 0 16px 0;
  color: var(--text-primary, #ffffff);
}

.placeholder-content p {
  margin: 0;
  color: var(--text-tertiary, #888888);
}

.qty {
  text-align: center;
}

.description {
  font-size: 11px;
}

.trade-price,
.mark {
  text-align: right;
  font-family: monospace;
}
</style>
