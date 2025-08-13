<template>
  <div id="app">
    <router-view />
    <!-- Global Notification Container -->
    <NotificationContainer />
    <!-- System Recovery Indicator -->
    <SystemRecoveryIndicator />
  </div>
</template>

<script>
import { watch, onMounted, onUnmounted } from "vue";
import { useRoute } from "vue-router";
import { useSelectedLegs } from "./composables/useSelectedLegs.js";
import NotificationContainer from "./components/notifications/NotificationContainer.vue";
import SystemRecoveryIndicator from "./components/SystemRecoveryIndicator.vue";
import webSocketClient from "./services/webSocketClient.js";

export default {
  name: "App",
  components: {
    NotificationContainer,
    SystemRecoveryIndicator,
  },
  setup() {
    const route = useRoute();
    const { clearAll } = useSelectedLegs();

    // Clear selected legs when navigating between views
    watch(
      () => route.name,
      (newRouteName, oldRouteName) => {
        // Only clear if we're actually changing routes (not initial load)
        if (oldRouteName && newRouteName !== oldRouteName) {
          clearAll();
        }
      }
    );

    // Critical: Add page unload handlers to prevent zombie workers
    const handlePageUnload = (event) => {
      console.log("🚨 Page unloading - performing immediate cleanup");
      
      // Immediately disconnect WebSocket client and terminate worker
      try {
        webSocketClient.disconnect();
        console.log("✅ WebSocket client disconnected on page unload");
      } catch (error) {
        console.error("❌ Error during page unload cleanup:", error);
      }
      
      // Don't prevent page unload, just cleanup
      return undefined;
    };

    const handleBeforeUnload = (event) => {
      console.log("⚠️ Page about to unload - starting cleanup");
      handlePageUnload(event);
      // Don't show confirmation dialog
      return undefined;
    };

    const handleVisibilityChange = () => {
      if (document.hidden) {
        console.log("👁️ Page became hidden - potential cleanup needed");
        // Don't disconnect immediately on hidden, but prepare for cleanup
        // This handles cases where the page is hidden but not closed
      }
    };

    onMounted(() => {
      // Add multiple cleanup handlers for different scenarios
      window.addEventListener('beforeunload', handleBeforeUnload);
      window.addEventListener('unload', handlePageUnload);
      window.addEventListener('pagehide', handlePageUnload);
      document.addEventListener('visibilitychange', handleVisibilityChange);      
    });

    onUnmounted(() => {
      // Clean up event listeners
      window.removeEventListener('beforeunload', handleBeforeUnload);
      window.removeEventListener('unload', handlePageUnload);
      window.removeEventListener('pagehide', handlePageUnload);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      
      // Final cleanup
      handlePageUnload();
      
      console.log("🧹 App unmounted - cleanup completed");
    });

    return {};
  },
};
</script>

<style>
#app {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen,
    Ubuntu, Cantarell, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  height: 100vh;
  overflow: hidden;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  padding: 0;
  background-color: #141519;
  color: #ffffff;
}

html,
body {
  height: 100%;
  overflow: hidden;
}
</style>
