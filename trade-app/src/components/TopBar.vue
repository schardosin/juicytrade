<template>
  <div class="top-bar">
    <!-- Logo Section -->
    <div class="logo-section">
      <div class="logo">
        <span class="logo-text">juicytrade</span>
      </div>
    </div>

    <!-- Navigation Links -->
    <div class="nav-links">
      <button
        v-for="link in navLinks"
        :key="link.value"
        :class="['nav-link', { active: activeLink === link.value }]"
        @click="setActiveLink(link.value)"
      >
        {{ link.label }}
      </button>
    </div>

    <!-- Search Section -->
    <div class="search-section">
      <div class="search-container">
        <i class="pi pi-search search-icon"></i>
        <InputText
          ref="searchInput"
          v-model="searchQuery"
          placeholder="Find a symbol or company"
          class="search-input"
          @input="handleInput"
          @focus="handleFocus"
          @blur="handleBlur"
          @keydown="handleKeydown"
          @keyup.enter="performSearch"
        />

        <!-- Search Results Dropdown -->
        <div
          v-if="
            showDropdown &&
            (searchResults.length > 0 || searchLoading || searchQuery.length > 0)
          "
          class="search-dropdown"
        >
          <!-- Loading State -->
          <div v-if="searchLoading" class="search-loading">
            <div class="loading-spinner"></div>
            <span>Searching...</span>
          </div>

          <!-- Search Results -->
          <div v-else-if="searchResults.length > 0" class="results-section">
            <div
              v-for="(result, index) in searchResults"
              :key="result.symbol"
              class="search-result-item"
              :class="{ highlighted: index === highlightedIndex }"
              @mousedown="selectSymbol(result)"
              @mouseenter="highlightedIndex = index"
            >
              <div class="symbol-info">
                <div class="symbol-main">
                  <span class="symbol-text">{{ result.symbol }}</span>
                  <span class="symbol-type" :class="getTypeClass(result.type)">
                    {{ getTypeLabel(result.type) }}
                  </span>
                </div>
                <div class="symbol-description">{{ result.description }}</div>
              </div>
              <div class="symbol-exchange">{{ result.exchange }}</div>
            </div>
          </div>

          <!-- No Results -->
          <div v-else-if="searchQuery.length > 0" class="no-results">
            <div class="no-results-text">
              No symbols found for "{{ searchQuery }}"
            </div>
            <div class="no-results-suggestion">Try a different search term</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Right Section -->
    <div class="right-section">
      <!-- Account Info -->
      <div class="account-info">
        <div class="account-balance">
          <span class="balance-label">Net Liq</span>
          <span
            class="balance-value"
            :class="{ loading: accountLoading, error: accountError }"
          >
            <span v-if="accountLoading" class="loading-dots">...</span>
            <span v-else-if="accountError" class="error-text">--</span>
            <span v-else>${{ netLiquidation.toLocaleString() }}</span>
          </span>
        </div>
        <div class="buying-power">
          <span class="power-label">BP</span>
          <span
            class="power-value"
            :class="{ loading: accountLoading, error: accountError }"
          >
            <span v-if="accountLoading" class="loading-dots">...</span>
            <span v-else-if="accountError" class="error-text">--</span>
            <span v-else>${{ buyingPower.toLocaleString() }}</span>
          </span>
        </div>
      </div>

      <!-- Trade Account Indicator -->
      <div class="trade-account-section">
        <div 
          class="trade-account-indicator"
          :class="{ loading: providersLoading, error: providersError }"
          @mouseenter="showProviderTooltip = true"
          @mouseleave="showProviderTooltip = false"
        >
          <i class="pi pi-building account-icon"></i>
          <span class="account-name">
            <span v-if="providersLoading" class="loading-dots">...</span>
            <span v-else-if="providersError" class="error-text">--</span>
            <span v-else>{{ tradeAccountName }}</span>
          </span>
          <span class="account-type" :class="tradeAccountTypeClass">
            {{ tradeAccountType }}
          </span>
        </div>

        <!-- Provider Configuration Tooltip -->
        <div 
          v-if="showProviderTooltip && !providersLoading && !providersError"
          class="provider-tooltip"
        >
          <div class="tooltip-header">
            <h4>Provider Configuration</h4>
          </div>
          <div class="tooltip-content">
            <div class="provider-category">
              <h5>Trading Services</h5>
              <div class="provider-item">
                <span class="service-name">Trade Account</span>
                <span class="provider-name">{{ formatProviderName('trade_account') }}</span>
              </div>
            </div>
            <div class="provider-category">
              <h5>Market Data</h5>
              <div v-for="service in marketDataServices" :key="service.key" class="provider-item">
                <span class="service-name">{{ service.label }}</span>
                <span class="provider-name">{{ formatProviderName(service.key) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Status Indicator -->
      <div class="status-section">
        <div class="connection-status" :class="connectionStatusClass">
          <span class="status-dot"></span>
          <span class="status-text">{{ connectionStatus }}</span>
        </div>
      </div>

      <!-- User Menu -->
      <div class="user-menu">
        <Button
          icon="pi pi-user"
          class="user-button"
          text
          rounded
          @click="toggleUserMenu"
        />
        <Menu ref="userMenuRef" :model="userMenuItems" :popup="true" />
      </div>

      <!-- Settings -->
      <div class="settings">
        <Button
          icon="pi pi-cog"
          class="settings-button"
          text
          rounded
          @click="openSettings"
        />
      </div>
    </div>

    <!-- Settings Dialog -->
    <SettingsDialog v-model:visible="showSettingsDialog" />
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import webSocketClient from "../services/webSocketClient";
import { useMarketData } from "../composables/useMarketData.js";
import SettingsDialog from "./SettingsDialog.vue";

export default {
  name: "TopBar",
  components: {
    SettingsDialog,
  },
  setup() {
    // Use unified market data composable
    const { 
      lookupSymbols, 
      getBalance, 
      getAccountInfo, 
      getAvailableProviders, 
      getProviderConfig,
      isLoading,
      getError
    } = useMarketData();

    // Reactive data
    const searchInput = ref(null);
    const searchQuery = ref("");
    const searchResults = ref([]);
    const showDropdown = ref(false);
    const searchLoading = ref(false);
    const highlightedIndex = ref(-1);
    const activeLink = ref("Trading");
    const netLiquidation = ref(0);
    const buyingPower = ref(0);
    const isConnected = ref(false);
    const userMenuRef = ref();
    const accountLoading = ref(true);
    const accountError = ref(null);
    const showSettingsDialog = ref(false);
    
    // Provider configuration data - now using smart data system
    const showProviderTooltip = ref(false);

    // Get reactive data from smart data system
    const reactiveBalance = getBalance();
    const reactiveAccountInfo = getAccountInfo();
    const reactiveAvailableProviders = getAvailableProviders();
    const reactiveProviderConfig = getProviderConfig();

    // Loading and error states from smart data system
    const providersLoading = computed(() => 
      isLoading("providers.available").value || isLoading("providers.config").value
    );
    const providersError = computed(() => 
      getError("providers.available").value || getError("providers.config").value
    );

    let searchTimeout = null;
    let connectionStatusInterval = null;

    // Navigation links
    const navLinks = [
      { label: "Dashboard", value: "Dashboard" },
      { label: "Trading", value: "Trading" },
      { label: "Manage", value: "Manage" },
    ];

    // User menu items
    const userMenuItems = [
      {
        label: "Account Settings",
        icon: "pi pi-user",
        command: () => {
          console.log("Account Settings clicked");
        },
      },
      {
        label: "Preferences",
        icon: "pi pi-cog",
        command: () => {
          console.log("Preferences clicked");
        },
      },
      {
        separator: true,
      },
      {
        label: "Sign Out",
        icon: "pi pi-sign-out",
        command: () => {
          console.log("Sign Out clicked");
        },
      },
    ];

    // Computed properties
    const connectionStatus = computed(() => {
      return isConnected.value ? "Connected" : "Disconnected";
    });

    const connectionStatusClass = computed(() => ({
      connected: isConnected.value,
      disconnected: !isConnected.value,
    }));

    // Trade account computed properties using smart data system
    const tradeAccountName = computed(() => {
      const config = reactiveProviderConfig.value;
      const providers = reactiveAvailableProviders.value;
      
      if (!config || !providers) {
        return "Unknown";
      }
      
      const tradeProvider = config.trade_account;
      
      if (!tradeProvider || !providers[tradeProvider]) {
        return "Unknown";
      }
      
      const providerData = providers[tradeProvider];
      return providerData.display_name || tradeProvider;
    });

    const tradeAccountType = computed(() => {
      const config = reactiveProviderConfig.value;
      const providers = reactiveAvailableProviders.value;
      
      if (!config || !providers) return "";
      
      const tradeProvider = config.trade_account;
      if (!tradeProvider || !providers[tradeProvider]) {
        return "";
      }
      
      const providerData = providers[tradeProvider];
      return providerData.paper ? "Paper" : "Live";
    });

    const tradeAccountTypeClass = computed(() => {
      const config = reactiveProviderConfig.value;
      const providers = reactiveAvailableProviders.value;
      
      if (!config || !providers) return "type-unknown";
      
      const tradeProvider = config.trade_account;
      if (!tradeProvider || !providers[tradeProvider]) {
        return "type-unknown";
      }
      
      const providerData = providers[tradeProvider];
      return providerData.paper ? "type-paper" : "type-live";
    });

    // Market data services for tooltip
    const marketDataServices = [
      { key: "stock_quotes", label: "Stock Quotes" },
      { key: "options_chain", label: "Options Chain" },
      { key: "expiration_dates", label: "Expiration Dates" },
      { key: "historical_data", label: "Historical Data" },
      { key: "symbol_lookup", label: "Symbol Lookup" },
      { key: "next_market_date", label: "Next Market Date" },
      { key: "market_calendar", label: "Market Calendar" },
      { key: "streaming_quotes", label: "Streaming Quotes" },
    ];

    // Methods
    const setActiveLink = (link) => {
      activeLink.value = link;
      // Here you would typically handle routing
    };

    const performSearch = async () => {
      if (searchQuery.value.trim()) {
        try {
          // Use unified data access - cached for 10 minutes
          const results = await lookupSymbols(searchQuery.value);

          if (results && results.length > 0) {
            // Take the first result and emit a symbol selection event
            const selectedSymbol = results[0];

            // Emit an event that the parent can listen to
            // For now, we'll use a global event bus or direct method call
            window.dispatchEvent(
              new CustomEvent("symbol-selected", {
                detail: selectedSymbol,
              })
            );

            searchQuery.value = selectedSymbol.symbol;
          } else {
            console.log("No symbols found");
            // Could show a toast notification here
          }
        } catch (error) {
          console.error("Error searching for symbol:", error);
        }
      }
    };

    const toggleUserMenu = (event) => {
      userMenuRef.value.toggle(event);
    };

    const openSettings = () => {
      console.log("Settings clicked");
      showSettingsDialog.value = true;
    };


    // Handle focus - improved state management
    const handleFocus = () => {
      showDropdown.value = true;
      // If we have a query and no results, trigger search
      if (searchQuery.value.length > 0 && searchResults.value.length === 0 && !searchLoading.value) {
        handleInput();
      }
    };

    // Handle input with improved debouncing and state management
    const handleInput = () => {
      // Clear any existing timeout
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }

      // Reset state
      highlightedIndex.value = -1;

      if (searchQuery.value.length === 0) {
        searchResults.value = [];
        searchLoading.value = false;
        showDropdown.value = false;
        return;
      }

      if (searchQuery.value.length < 1) {
        return;
      }

      // Show dropdown and loading state
      showDropdown.value = true;
      searchLoading.value = true;

      searchTimeout = setTimeout(async () => {
        try {
          // Use unified data access - cached for 10 minutes
          const results = await lookupSymbols(searchQuery.value);
          searchResults.value = results || [];
        } catch (error) {
          console.error("Error searching symbols:", error);
          searchResults.value = [];
        } finally {
          searchLoading.value = false;
        }
      }, 300); // 300ms debounce
    };

    // Handle blur with improved race condition prevention
    const handleBlur = (event) => {
      // Use a longer timeout to prevent race conditions with mousedown events
      setTimeout(() => {
        showDropdown.value = false;
        highlightedIndex.value = -1;
      }, 200);
    };

    // Handle keyboard navigation
    const handleKeydown = (event) => {
      if (!showDropdown.value) return;

      const results = searchResults.value;

      switch (event.key) {
        case "ArrowDown":
          event.preventDefault();
          highlightedIndex.value = Math.min(
            highlightedIndex.value + 1,
            results.length - 1
          );
          break;
        case "ArrowUp":
          event.preventDefault();
          highlightedIndex.value = Math.max(highlightedIndex.value - 1, -1);
          break;
        case "Enter":
          event.preventDefault();
          if (highlightedIndex.value >= 0 && results[highlightedIndex.value]) {
            selectSymbol(results[highlightedIndex.value]);
          } else {
            performSearch();
          }
          break;
        case "Escape":
          showDropdown.value = false;
          searchInput.value?.blur();
          break;
      }
    };

    // Select a symbol with improved state cleanup
    const selectSymbol = (symbol) => {
      // Clear search state first to prevent race conditions
      searchResults.value = [];
      searchLoading.value = false;
      highlightedIndex.value = -1;
      
      // Clear any pending search timeout
      if (searchTimeout) {
        clearTimeout(searchTimeout);
        searchTimeout = null;
      }
      
      // Update query and hide dropdown
      searchQuery.value = symbol.symbol;
      showDropdown.value = false;

      // Emit symbol selection event
      window.dispatchEvent(
        new CustomEvent("symbol-selected", {
          detail: symbol,
        })
      );
    };

    // Get type class for styling
    const getTypeClass = (type) => {
      switch (type?.toLowerCase()) {
        case "stock":
          return "type-stock";
        case "etf":
          return "type-etf";
        case "index":
          return "type-index";
        case "option":
          return "type-option";
        default:
          return "type-default";
      }
    };

    // Get type label
    const getTypeLabel = (type) => {
      switch (type?.toLowerCase()) {
        case "stock":
          return "Stock";
        case "etf":
          return "ETF";
        case "index":
          return "Index";
        case "option":
          return "Option";
        default:
          return type || "Security";
      }
    };

    // Format provider name for display using smart data system
    const formatProviderName = (serviceKey) => {
      const config = reactiveProviderConfig.value;
      const providers = reactiveAvailableProviders.value;
      
      if (!config || !providers) return "Unknown";
      
      const providerName = config[serviceKey];
      if (!providerName || typeof providerName !== 'string' || !providers[providerName]) {
        return "Unknown";
      }
      
      const providerData = providers[providerName];
      const displayName = providerData.display_name || providerName;
      const accountType = providerData.paper ? "(Paper)" : "(Live)";
      return `${displayName} ${accountType}`;
    };

    // Watch reactive account data and update local state
    const updateAccountDisplay = () => {
      const balanceData = reactiveBalance.value;
      const accountData = reactiveAccountInfo.value;

      if (balanceData || accountData) {
        accountLoading.value = false;
        accountError.value = null;

        // Use balance data first, fallback to account data
        const data = balanceData || accountData;

        // Update Net Liq (Portfolio Value/Equity)
        netLiquidation.value =
          data.portfolio_value || data.equity || data.net_liquidation || 0;

        // Update Buying Power
        buyingPower.value =
          data.buying_power ||
          data.options_buying_power ||
          data.day_trading_buying_power ||
          0;
      } else {
        // Still loading or error state
        accountLoading.value = true;
        accountError.value = null;
      }
    };

    // Lifecycle hooks
    onMounted(() => {
      // Initial account display update
      updateAccountDisplay();
    });

    // Watch for reactive data changes and update display
    // This replaces the old periodic API calls
    watch([reactiveBalance, reactiveAccountInfo], updateAccountDisplay, {
      immediate: true,
      deep: true,
    });

    watch(
      () => webSocketClient.isConnected.value,
      (newStatus) => {
        isConnected.value = newStatus;
      },
      { immediate: true }
    );

    // Clean up intervals when component is unmounted
    onUnmounted(() => {
      if (searchTimeout) {
        clearTimeout(searchTimeout);
        searchTimeout = null;
      }

    });

    return {
      // Reactive data
      searchInput,
      searchQuery,
      searchResults,
      showDropdown,
      searchLoading,
      highlightedIndex,
      activeLink,
      netLiquidation,
      buyingPower,
      userMenuRef,
      accountLoading,
      accountError,
      showSettingsDialog,
      providersLoading,
      providersError,
      showProviderTooltip,

      // Static data
      navLinks,
      userMenuItems,
      marketDataServices,

      // Computed
      connectionStatus,
      connectionStatusClass,
      tradeAccountName,
      tradeAccountType,
      tradeAccountTypeClass,

      // Methods
      setActiveLink,
      performSearch,
      toggleUserMenu,
      openSettings,
      handleFocus,
      handleInput,
      handleBlur,
      handleKeydown,
      selectSymbol,
      getTypeClass,
      getTypeLabel,
      formatProviderName,
    };
  },
};
</script>

<style scoped>
.top-bar {
  display: flex;
  align-items: center;
  height: 60px;
  background-color: var(--bg-primary);
  border-bottom: 1px solid var(--border-primary);
  padding: 0 var(--spacing-xl);
  gap: var(--spacing-xl);
}

.logo-section {
  display: flex;
  align-items: center;
  min-width: 120px;
}

.logo-text {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--color-brand);
  letter-spacing: -0.5px;
}

.nav-links {
  display: flex;
  gap: var(--spacing-sm);
}

.nav-link {
  padding: var(--spacing-sm) var(--spacing-lg);
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

.nav-link:hover {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.nav-link.active {
  background-color: var(--bg-quaternary);
  color: var(--text-primary);
}

.search-section {
  flex: 1;
  max-width: 400px;
  margin: 0 var(--spacing-xl);
}

.search-container {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: var(--spacing-md);
  color: var(--text-tertiary);
  font-size: var(--font-size-md);
  z-index: 1;
}

.search-input {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md) var(--spacing-sm) 36px !important;
  background-color: var(--bg-secondary) !important;
  border: 1px solid var(--border-secondary) !important;
  border-radius: var(--radius-md) !important;
  color: var(--text-primary) !important;
  font-size: var(--font-size-md) !important;
}

.search-input::placeholder {
  color: var(--text-tertiary);
}

.search-input:focus {
  outline: none;
  border-color: var(--color-info);
  background-color: var(--bg-tertiary);
}

.search-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-top: none;
  border-radius: 0 0 var(--radius-sm) var(--radius-sm);
  max-height: 300px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: var(--shadow-md);
}

.search-loading {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  color: var(--text-tertiary);
  font-size: var(--font-size-md);
}

.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-info);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

.search-result-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-sm) var(--spacing-md);
  cursor: pointer;
  transition: var(--transition-normal);
  border-bottom: 1px solid var(--border-secondary);
}

.search-result-item:hover,
.search-result-item.highlighted {
  background: var(--bg-quaternary);
}

.search-result-item:last-child {
  border-bottom: none;
}

.symbol-info {
  flex: 1;
  min-width: 0;
}

.symbol-main {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: 2px;
}

.symbol-text {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.symbol-type {
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.type-stock {
  background: var(--color-info);
  color: var(--text-primary);
}

.type-etf {
  background: var(--color-success);
  color: var(--text-primary);
}

.type-index {
  background: var(--color-brand);
  color: var(--text-primary);
}

.type-option {
  background: var(--color-primary);
  color: var(--text-primary);
}

.type-default {
  background: var(--text-tertiary);
  color: var(--text-primary);
}

.symbol-description {
  color: var(--text-secondary);
  font-size: var(--font-size-base);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.symbol-exchange {
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  margin-left: var(--spacing-md);
}

.no-results {
  padding: var(--spacing-lg) var(--spacing-md);
  text-align: center;
}

.no-results-text {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin-bottom: var(--spacing-xs);
}

.no-results-suggestion {
  color: var(--text-tertiary);
  font-size: var(--font-size-base);
}

.right-section {
  display: flex;
  align-items: center;
  gap: var(--spacing-lg);
}

.account-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  text-align: right;
}

.account-balance,
.buying-power {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--font-size-base);
}

.balance-label,
.power-label {
  color: var(--text-tertiary);
  font-weight: var(--font-weight-medium);
}

.balance-value,
.power-value {
  color: var(--text-primary);
  font-weight: var(--font-weight-semibold);
}

.balance-value.loading,
.power-value.loading {
  color: var(--text-tertiary);
}

.balance-value.error,
.power-value.error {
  color: var(--color-danger);
}

.loading-dots {
  animation: pulse 1.5s ease-in-out infinite;
}

.error-text {
  font-style: italic;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.status-section {
  display: flex;
  align-items: center;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-base);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.connection-status.connected .status-dot {
  background-color: var(--color-connected);
}

.connection-status.disconnected .status-dot {
  background-color: var(--color-disconnected);
}

.connection-status.connected .status-text {
  color: var(--color-connected);
}

.connection-status.disconnected .status-text {
  color: var(--color-disconnected);
}

.user-menu,
.settings {
  display: flex;
  align-items: center;
}

.user-button,
.settings-button {
  color: var(--text-secondary) !important;
  width: 36px !important;
  height: 36px !important;
}

.user-button:hover,
.settings-button:hover {
  background-color: var(--bg-tertiary) !important;
  color: var(--text-primary) !important;
}

/* Trade Account Indicator Styles */
.trade-account-section {
  position: relative;
  display: flex;
  align-items: center;
}

.trade-account-indicator {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition-normal);
  font-size: var(--font-size-base);
}

.trade-account-indicator:hover {
  background-color: var(--bg-tertiary);
  border-color: var(--border-tertiary);
}

.trade-account-indicator.loading {
  opacity: 0.7;
}

.trade-account-indicator.error {
  border-color: var(--color-danger);
  background-color: rgba(239, 68, 68, 0.1);
}

.account-icon {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
}

.account-name {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
  white-space: nowrap;
}

.account-type {
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.account-type.type-paper {
  background-color: var(--color-info);
  color: white;
}

.account-type.type-live {
  background-color: var(--color-warning);
  color: white;
}

.account-type.type-unknown {
  background-color: var(--text-tertiary);
  color: white;
}

/* Provider Tooltip Styles */
.provider-tooltip {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  min-width: 320px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  z-index: 1000;
  overflow: hidden;
}

.tooltip-header {
  padding: var(--spacing-md) var(--spacing-lg);
  background: var(--bg-quaternary);
  border-bottom: 1px solid var(--border-secondary);
}

.tooltip-header h4 {
  margin: 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.tooltip-content {
  padding: var(--spacing-md);
  max-height: 400px;
  overflow-y: auto;
}

.provider-category {
  margin-bottom: var(--spacing-lg);
}

.provider-category:last-child {
  margin-bottom: 0;
}

.provider-category h5 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  padding-bottom: var(--spacing-xs);
  border-bottom: 1px solid var(--border-primary);
}

.provider-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-xs) 0;
  font-size: var(--font-size-base);
}

.service-name {
  color: var(--text-secondary);
  font-weight: var(--font-weight-medium);
}

.provider-name {
  color: var(--text-primary);
  font-weight: var(--font-weight-medium);
  text-align: right;
  max-width: 150px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Custom scrollbar for tooltip */
.tooltip-content::-webkit-scrollbar {
  width: 4px;
}

.tooltip-content::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

.tooltip-content::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: var(--radius-sm);
}

.tooltip-content::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}
</style>
