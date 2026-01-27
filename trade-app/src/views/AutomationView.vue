<template>
  <div class="automation-view-container">
    <!-- Top Bar -->
    <TopBar />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Main Left Navigation (same as other views) -->
      <SideNav />

      <!-- Content Area -->
      <div class="content-area">
        <router-view />
      </div>

      <!-- Right Panel (Desktop Only) -->
      <RightPanel
        v-if="!isMobile"
        :currentSymbol="currentSymbol"
        :currentPrice="currentPrice"
        :priceChange="priceChange"
        :isLivePrice="isLivePrice"
        :chartData="chartData"
        :additionalQuoteData="additionalQuoteData"
        :forceExpanded="isRightPanelExpanded"
        :forceSection="rightPanelSection"
        @panel-collapsed="onRightPanelCollapsed"
        @positions-changed="onPositionsChanged"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed } from 'vue'
import TopBar from '../components/TopBar.vue'
import SideNav from '../components/SideNav.vue'
import RightPanel from '../components/RightPanel.vue'
import { useGlobalSymbol } from '../composables/useGlobalSymbol'
import { useMobileDetection } from '../composables/useMobileDetection'

export default {
  name: 'AutomationView',
  components: {
    TopBar,
    SideNav,
    RightPanel
  },
  setup() {
    // Mobile detection
    const { isMobile } = useMobileDetection()

    // Use global symbol state
    const { globalSymbolState } = useGlobalSymbol()

    // Computed properties for global symbol state
    const currentSymbol = computed(() => globalSymbolState.currentSymbol)
    const currentPrice = computed(() => globalSymbolState.currentPrice)
    const priceChange = computed(() => globalSymbolState.priceChange)
    const isLivePrice = computed(() => globalSymbolState.isLivePrice)

    // Right panel state
    const chartData = ref(null)
    const additionalQuoteData = ref({})
    const isRightPanelExpanded = ref(false)
    const rightPanelSection = ref(null)

    // Right panel event handlers
    const onRightPanelCollapsed = () => {
      isRightPanelExpanded.value = false
    }

    const onPositionsChanged = (positions) => {
      // Handle positions changes if needed
      // For automation view, we mainly want to show orders/positions
      // Chart generation can be added later if needed
    }

    return {
      // Mobile
      isMobile,
      
      // Symbol data
      currentSymbol,
      currentPrice,
      priceChange,
      isLivePrice,
      
      // Right panel
      chartData,
      additionalQuoteData,
      isRightPanelExpanded,
      rightPanelSection,
      onRightPanelCollapsed,
      onPositionsChanged
    }
  }
}
</script>

<style scoped>
.automation-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: #141519;
  color: #ffffff;
}

.main-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
  position: relative;
}

.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: #141519;
}
</style>
