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
          @focus="showDropdown = true"
          @blur="handleBlur"
          @keydown="handleKeydown"
          @keyup.enter="performSearch"
        />

        <!-- Search Results Dropdown -->
        <div
          v-if="
            showDropdown &&
            (searchResults.length > 0 || isLoading || searchQuery.length > 0)
          "
          class="search-dropdown"
        >
          <!-- Loading State -->
          <div v-if="isLoading" class="search-loading">
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
          <span class="balance-value"
            >${{ netLiquidation.toLocaleString() }}</span
          >
        </div>
        <div class="buying-power">
          <span class="power-label">BP</span>
          <span class="power-value">${{ buyingPower.toLocaleString() }}</span>
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
  </div>
</template>

<script>
import { ref, computed, onMounted } from "vue";
import webSocketClient from "../services/webSocketClient";

export default {
  name: "TopBar",
  setup() {
    // Reactive data
    const searchInput = ref(null);
    const searchQuery = ref("");
    const searchResults = ref([]);
    const showDropdown = ref(false);
    const isLoading = ref(false);
    const highlightedIndex = ref(-1);
    const activeLink = ref("Trading");
    const netLiquidation = ref(125000);
    const buyingPower = ref(250000);
    const isConnected = ref(false);
    const userMenuRef = ref();

    let searchTimeout = null;

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

    // Methods
    const setActiveLink = (link) => {
      activeLink.value = link;
      // Here you would typically handle routing
      console.log(`Navigating to ${link}`);
    };

    const performSearch = async () => {
      if (searchQuery.value.trim()) {
        console.log(`Searching for: ${searchQuery.value}`);
        try {
          // Import the API service
          const { api } = await import("../services/api.js");
          const results = await api.lookupSymbols(searchQuery.value);

          if (results && results.length > 0) {
            // Take the first result and emit a symbol selection event
            const selectedSymbol = results[0];
            console.log("Selected symbol from search:", selectedSymbol);

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
      // Here you would open a settings dialog or navigate to settings
    };

    const checkConnectionStatus = () => {
      // Check WebSocket connection status
      const status = webSocketClient.getConnectionStatus();
      isConnected.value = status.isConnected;
    };

    // Handle input with debouncing
    const handleInput = () => {
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }

      if (searchQuery.value.length === 0) {
        searchResults.value = [];
        isLoading.value = false;
        return;
      }

      if (searchQuery.value.length < 1) {
        return;
      }

      isLoading.value = true;
      highlightedIndex.value = -1;

      searchTimeout = setTimeout(async () => {
        try {
          const { api } = await import("../services/api.js");
          const results = await api.lookupSymbols(searchQuery.value);
          searchResults.value = results || [];
        } catch (error) {
          console.error("Error searching symbols:", error);
          searchResults.value = [];
        } finally {
          isLoading.value = false;
        }
      }, 300); // 300ms debounce
    };

    // Handle blur with delay to allow click events
    const handleBlur = () => {
      setTimeout(() => {
        showDropdown.value = false;
        highlightedIndex.value = -1;
      }, 150);
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

    // Select a symbol
    const selectSymbol = (symbol) => {
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

    // Lifecycle hooks
    onMounted(() => {
      checkConnectionStatus();

      // Set up periodic connection status checks
      setInterval(checkConnectionStatus, 5000);
    });

    return {
      // Reactive data
      searchInput,
      searchQuery,
      searchResults,
      showDropdown,
      isLoading,
      highlightedIndex,
      activeLink,
      netLiquidation,
      buyingPower,
      userMenuRef,

      // Static data
      navLinks,
      userMenuItems,

      // Computed
      connectionStatus,
      connectionStatusClass,

      // Methods
      setActiveLink,
      performSearch,
      toggleUserMenu,
      openSettings,
      handleInput,
      handleBlur,
      handleKeydown,
      selectSymbol,
      getTypeClass,
      getTypeLabel,
    };
  },
};
</script>

<style scoped>
.top-bar {
  display: flex;
  align-items: center;
  height: 60px;
  background-color: #1a1a1a;
  border-bottom: 1px solid #333333;
  padding: 0 24px;
  gap: 24px;
}

.logo-section {
  display: flex;
  align-items: center;
  min-width: 120px;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #ff6b35;
  letter-spacing: -0.5px;
}

.nav-links {
  display: flex;
  gap: 8px;
}

.nav-link {
  padding: 8px 16px;
  background: none;
  border: none;
  color: #cccccc;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.nav-link:hover {
  background-color: #333333;
  color: #ffffff;
}

.nav-link.active {
  background-color: #444444;
  color: #ffffff;
}

.search-section {
  flex: 1;
  max-width: 400px;
  margin: 0 24px;
}

.search-container {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 12px;
  color: #888888;
  font-size: 14px;
  z-index: 1;
}

.search-input {
  width: 100%;
  padding: 8px 12px 8px 36px;
  background-color: #333333;
  border: 1px solid #444444;
  border-radius: 6px;
  color: #ffffff;
  font-size: 14px;
}

.search-input::placeholder {
  color: #888888;
}

.search-input:focus {
  outline: none;
  border-color: #007bff;
  background-color: #2a2a2a;
}

.search-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: #1a1a1a;
  border: 1px solid #333;
  border-top: none;
  border-radius: 0 0 4px 4px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.search-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  color: #666;
  font-size: 14px;
}

.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid #333;
  border-top: 2px solid #0066cc;
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
  padding: 8px 12px;
  cursor: pointer;
  transition: background-color 0.2s;
  border-bottom: 1px solid #2a2a2a;
}

.search-result-item:hover,
.search-result-item.highlighted {
  background: #2a2a2a;
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
  gap: 8px;
  margin-bottom: 2px;
}

.symbol-text {
  font-weight: 600;
  color: #fff;
  font-size: 14px;
}

.symbol-type {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.type-stock {
  background: #0066cc;
  color: #fff;
}

.type-etf {
  background: #00aa44;
  color: #fff;
}

.type-index {
  background: #ff6600;
  color: #fff;
}

.type-option {
  background: #9966cc;
  color: #fff;
}

.type-default {
  background: #666;
  color: #fff;
}

.symbol-description {
  color: #999;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.symbol-exchange {
  color: #666;
  font-size: 11px;
  font-weight: 500;
  margin-left: 12px;
}

.no-results {
  padding: 16px 12px;
  text-align: center;
}

.no-results-text {
  color: #999;
  font-size: 14px;
  margin-bottom: 4px;
}

.no-results-suggestion {
  color: #666;
  font-size: 12px;
}

.right-section {
  display: flex;
  align-items: center;
  gap: 16px;
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
  font-size: 12px;
}

.balance-label,
.power-label {
  color: #888888;
  font-weight: 500;
}

.balance-value,
.power-value {
  color: #ffffff;
  font-weight: 600;
}

.status-section {
  display: flex;
  align-items: center;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.connection-status.connected .status-dot {
  background-color: #00c851;
}

.connection-status.disconnected .status-dot {
  background-color: #ff4444;
}

.connection-status.connected .status-text {
  color: #00c851;
}

.connection-status.disconnected .status-text {
  color: #ff4444;
}

.user-menu,
.settings {
  display: flex;
  align-items: center;
}

.user-button,
.settings-button {
  color: #cccccc !important;
  width: 36px !important;
  height: 36px !important;
}

.user-button:hover,
.settings-button:hover {
  background-color: #333333 !important;
  color: #ffffff !important;
}

/* Dark theme overrides for PrimeVue components */
:deep(.p-inputtext) {
  background-color: #333333;
  border: 1px solid #444444;
  color: #ffffff;
}

:deep(.p-inputtext:focus) {
  border-color: #007bff;
  background-color: #2a2a2a;
  box-shadow: 0 0 0 0.2rem rgba(0, 123, 255, 0.25);
}

:deep(.p-menu) {
  background-color: #333333;
  border: 1px solid #444444;
}

:deep(.p-menu .p-menuitem-link) {
  color: #ffffff;
}

:deep(.p-menu .p-menuitem-link:hover) {
  background-color: #444444;
}

:deep(.p-menu .p-menuitem-separator) {
  border-top: 1px solid #444444;
}

:deep(.p-button.p-button-text) {
  color: #cccccc;
}

:deep(.p-button.p-button-text:hover) {
  background-color: #333333;
  color: #ffffff;
}
</style>
