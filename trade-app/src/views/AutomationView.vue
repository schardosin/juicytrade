<template>
  <div class="automation-view-container">
    <!-- Top Bar -->
    <TopBar @toggle-mobile-nav="showMobileNav = true" />

    <!-- Mobile Navigation Drawer -->
    <MobileNavDrawer
      v-if="isMobile"
      :visible="showMobileNav"
      @close="showMobileNav = false"
      @navigate="onMobileNavigation"
    />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Main Left Navigation (Desktop Only) -->
      <SideNav v-if="!isMobile" />

      <!-- Content Area -->
      <div class="content-area" :class="{ 'mobile-layout': isMobile }">
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

    <!-- Mobile Bottom Button Bar (only on mobile) -->
    <MobileBottomButtonBar
      v-if="isMobile"
      :activeSection="showMobileOverlay ? activeMobileSection : null"
      @section-selected="onMobileSectionSelected"
    />

    <!-- Mobile Full Screen Overlay (only on mobile) -->
    <MobileFullScreenOverlay
      v-if="isMobile"
      :visible="showMobileOverlay"
      :activeSection="activeMobileSection"
      :currentSymbol="currentSymbol"
      :currentPrice="currentPrice"
      :priceChange="priceChange"
      :isLivePrice="isLivePrice"
      :chartData="chartData"
      :additionalQuoteData="additionalQuoteData"
      :allPositions="allMobilePositions"
      :checkedPositions="mobileCheckedPositions"
      :isAllSelected="isMobileAllSelected"
      :isIndeterminate="isMobileIndeterminate"
      @close="closeMobileOverlay"
      @toggle-position-check="onMobileTogglePositionCheck"
      @toggle-select-all="onMobileToggleSelectAll"
    />
  </div>
</template>

<script>
import { ref, computed } from 'vue'
import TopBar from '../components/TopBar.vue'
import SideNav from '../components/SideNav.vue'
import RightPanel from '../components/RightPanel.vue'
import MobileNavDrawer from '../components/MobileNavDrawer.vue'
import MobileBottomButtonBar from '../components/MobileBottomButtonBar.vue'
import MobileFullScreenOverlay from '../components/MobileFullScreenOverlay.vue'
import { useGlobalSymbol } from '../composables/useGlobalSymbol'
import { useMobileDetection } from '../composables/useMobileDetection'
import { useMarketData } from '../composables/useMarketData.js'
import { generateMultiLegPayoff } from '../utils/chartUtils'

export default {
  name: 'AutomationView',
  components: {
    TopBar,
    SideNav,
    RightPanel,
    MobileNavDrawer,
    MobileBottomButtonBar,
    MobileFullScreenOverlay
  },
  setup() {
    // Mobile detection
    const { isMobile, isTablet, isDesktop } = useMobileDetection()

    // Mobile navigation state
    const showMobileNav = ref(false)

    // Parse option symbol to extract strike, type, expiry
    const parseOptionSymbol = (optionSymbol) => {
      if (!optionSymbol) return null
      try {
        // Format 1: SPY250714C00624000 (SYMBOL + YYMMDD + C/P + STRIKE with 8 digits)
        let match = optionSymbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/)
        if (match) {
          const [, underlying, dateStr, type, strikeStr] = match
          const year = 2000 + parseInt(dateStr.substring(0, 2))
          const month = dateStr.substring(2, 4)
          const day = dateStr.substring(4, 6)
          const expiry = `${year}-${month}-${day}`
          const strike = parseInt(strikeStr) / 1000
          return {
            underlying_symbol: underlying,
            option_type: type === "C" ? "call" : "put",
            strike_price: strike,
            expiry_date: expiry
          }
        }

        // Format 2: SPY_250714_C_624 (SYMBOL_YYMMDD_C/P_STRIKE)
        match = optionSymbol.match(/^([A-Z]+)_(\d{6})_([CP])_(\d+(?:\.\d+)?)$/)
        if (match) {
          const [, underlying, dateStr, type, strikeStr] = match
          const year = 2000 + parseInt(dateStr.substring(0, 2))
          const month = dateStr.substring(2, 4)
          const day = dateStr.substring(4, 6)
          const expiry = `${year}-${month}-${day}`
          const strike = parseFloat(strikeStr)
          return {
            underlying_symbol: underlying,
            option_type: type === "C" ? "call" : "put",
            strike_price: strike,
            expiry_date: expiry
          }
        }

        return null
      } catch (e) {
        console.error("Error parsing option symbol:", e)
        return null
      }
    }

    // Mobile overlay state
    const showMobileOverlay = ref(false)
    const activeMobileSection = ref('overview')
    const mobileCheckedPositions = ref(new Set())

    // Use global symbol state
    const { globalSymbolState } = useGlobalSymbol()

    // Use market data for positions
    const { getPositionsForSymbol } = useMarketData()

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

    // Chart data update function
    const updateChartData = (positions = null) => {
      if (!positions || positions.length === 0) {
        chartData.value = null
        return
      }

      try {
        const payoffData = generateMultiLegPayoff(
          positions,
          currentPrice.value,
          0 // No adjusted net credit in automation view
        )

        // Force reactivity by creating a new object reference
        chartData.value = {
          ...payoffData,
          timestamp: Date.now()
        }
      } catch (error) {
        console.error("Error generating payoff chart:", error)
        chartData.value = null
      }
    }

    // Right panel event handlers
    const onRightPanelCollapsed = () => {
      isRightPanelExpanded.value = false
    }

    const onPositionsChanged = (positions) => {
      // Update chart when positions change from RightPanel
      updateChartData(positions)
    }

    // Mobile navigation handler
    const onMobileNavigation = (navItem) => {
      console.log("Mobile navigation:", navItem)
    }

    // Mobile positions for overlay - enriched with parsed option data
    const allMobilePositions = computed(() => {
      const positionsComputed = getPositionsForSymbol(currentSymbol.value)
      const positionsData = positionsComputed.value
      
      if (positionsData?.positions && Array.isArray(positionsData.positions)) {
        return positionsData.positions.map((pos, index) => {
          // Parse option symbol to get missing details
          const parsedOption = parseOptionSymbol(pos.symbol)
          
          // Calculate current price from entry price if not available
          const currentPrice = pos.current_price || pos.avg_entry_price || 
            (pos.cost_basis ? Math.abs(pos.cost_basis / (pos.qty * 100)) : 0)
          
          return {
            id: pos.id || `existing:${pos.symbol || index}`,
            symbol: pos.symbol,
            asset_class: pos.asset_class || "us_option",
            underlying_symbol: pos.underlying_symbol || parsedOption?.underlying_symbol,
            qty: pos.qty,
            // Use parsed option data to fill missing fields
            strike_price: parsedOption?.strike_price || pos.strike_price || 0,
            option_type: parsedOption?.option_type || pos.option_type || "call",
            expiry_date: parsedOption?.expiry_date || pos.expiry_date || "",
            current_price: currentPrice,
            avg_entry_price: pos.avg_entry_price || currentPrice,
            unrealized_pl: pos.unrealized_pl || 0,
            cost_basis: pos.cost_basis,
            isExisting: true,
            isSelected: false
          }
        })
      }
      return []
    })

    // Mobile selection states
    const isMobileAllSelected = computed(() => {
      if (allMobilePositions.value.length === 0) return false
      return allMobilePositions.value.every((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      )
    })

    const isMobileIndeterminate = computed(() => {
      if (allMobilePositions.value.length === 0) return false
      const checkedCount = allMobilePositions.value.filter((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      ).length
      return checkedCount > 0 && checkedCount < allMobilePositions.value.length
    })

    // Mobile overlay methods
    const onMobileSectionSelected = (sectionKey) => {
      activeMobileSection.value = sectionKey
      showMobileOverlay.value = true
    }

    const closeMobileOverlay = () => {
      showMobileOverlay.value = false
    }

    const onMobileTogglePositionCheck = (positionId) => {
      if (mobileCheckedPositions.value.has(positionId)) {
        mobileCheckedPositions.value.delete(positionId)
      } else {
        mobileCheckedPositions.value.add(positionId)
      }
      // Update chart with checked positions
      const checkedPositionsList = allMobilePositions.value.filter((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      )
      updateChartData(checkedPositionsList)
    }

    const onMobileToggleSelectAll = () => {
      if (isMobileAllSelected.value) {
        mobileCheckedPositions.value.clear()
      } else {
        allMobilePositions.value.forEach((pos) => {
          mobileCheckedPositions.value.add(pos.id)
        })
      }
      // Update chart with checked positions
      const checkedPositionsList = allMobilePositions.value.filter((pos) =>
        mobileCheckedPositions.value.has(pos.id)
      )
      updateChartData(checkedPositionsList)
    }

    return {
      // Mobile
      isMobile,
      isTablet,
      isDesktop,
      showMobileNav,
      onMobileNavigation,
      
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
      onPositionsChanged,

      // Mobile overlay
      showMobileOverlay,
      activeMobileSection,
      mobileCheckedPositions,
      allMobilePositions,
      isMobileAllSelected,
      isMobileIndeterminate,
      onMobileSectionSelected,
      closeMobileOverlay,
      onMobileTogglePositionCheck,
      onMobileToggleSelectAll
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

/* Mobile layout adjustments */
.content-area.mobile-layout {
  padding-left: 0;
  margin-left: 0;
}

/* Mobile: Adjust content area height for bottom bar */
@media (max-width: 768px) {
  .main-layout {
    flex-direction: column;
  }
  
  .content-area {
    margin-right: 0 !important;
    /* Reserve space for mobile bottom button bar (80px) */
    padding-bottom: 80px;
  }
}
</style>
