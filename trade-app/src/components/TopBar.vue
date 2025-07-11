<template>
  <div class="top-bar">
    <!-- Logo Section -->
    <div class="logo-section">
      <div class="logo">
        <span class="logo-text">tastytrade</span>
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
          v-model="searchQuery"
          placeholder="Find a symbol or company"
          class="search-input"
          @keyup.enter="performSearch"
        />
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
    const searchQuery = ref("");
    const activeLink = ref("Trading");
    const netLiquidation = ref(125000);
    const buyingPower = ref(250000);
    const isConnected = ref(false);
    const userMenuRef = ref();

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

    const performSearch = () => {
      if (searchQuery.value.trim()) {
        console.log(`Searching for: ${searchQuery.value}`);
        // Here you would implement the search functionality
        // For now, we'll just emit an event or call a method
        searchQuery.value = "";
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

    // Lifecycle hooks
    onMounted(() => {
      checkConnectionStatus();

      // Set up periodic connection status checks
      setInterval(checkConnectionStatus, 5000);
    });

    return {
      // Reactive data
      searchQuery,
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
