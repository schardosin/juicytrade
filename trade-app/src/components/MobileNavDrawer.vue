<template>
  <!-- Backdrop -->
  <div
    v-if="visible"
    class="nav-backdrop"
    @click="$emit('close')"
  ></div>

  <!-- Navigation Drawer -->
  <div
    class="mobile-nav-drawer"
    :class="{ 'drawer-open': visible }"
  >
    <!-- Header -->
    <div class="drawer-header">
      <div class="app-logo">
        <img src="/logos/juicytrade-logo.svg" alt="juicytrade" class="logo-svg" />
      </div>
      <button
        class="close-btn"
        @click="$emit('close')"
        aria-label="Close navigation"
      >
        <i class="pi pi-times"></i>
      </button>
    </div>

    <!-- Navigation Items -->
    <div class="nav-items">
      <div
        v-for="item in navItems"
        :key="item.id"
        :class="['nav-item', { active: activeItem === item.id }]"
        @click="handleNavClick(item)"
      >
        <div class="nav-icon">
          <i :class="item.icon"></i>
        </div>
        <div class="nav-label">{{ item.label }}</div>
        <div v-if="activeItem === item.id" class="active-indicator"></div>
      </div>
    </div>

    <!-- Bottom Section -->
    <div class="nav-bottom">
      <div class="nav-item" @click="openHelp">
        <div class="nav-icon">
          <i class="pi pi-question-circle"></i>
        </div>
        <div class="nav-label">Help</div>
      </div>
      
      <!-- User Info Section -->
      <div class="user-section">
        <div class="user-info">
          <div class="user-avatar">
            <i class="pi pi-user"></i>
          </div>
          <div class="user-details">
            <div class="user-name">Trader</div>
            <div class="user-status">Connected</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, watch } from "vue";
import { useRouter, useRoute } from "vue-router";

export default {
  name: "MobileNavDrawer",
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["close", "navigate"],
  setup(props, { emit }) {
    const router = useRouter();
    const route = useRoute();
    const activeItem = ref("trade");

    // Navigation items matching the desktop SideNav
    const navItems = [
      {
        id: "positions",
        label: "Positions",
        icon: "pi pi-chart-line",
        route: "/positions",
      },
      {
        id: "trade",
        label: "Trade",
        icon: "pi pi-shopping-cart",
        route: "/trade",
      },
      {
        id: "chart",
        label: "Chart",
        icon: "pi pi-chart-bar",
        route: "/chart",
      },
      {
        id: "automation",
        label: "Auto",
        icon: "pi pi-bolt",
        route: "/automation",
      },
      {
        id: "activity",
        label: "Activity",
        icon: "pi pi-clock",
        route: "/activity",
      },
      {
        id: "history",
        label: "History",
        icon: "pi pi-history",
        route: "/history",
      },
    ];

    const handleNavClick = (item) => {
      activeItem.value = item.id;
      emit("navigate", item);
      
      // Navigate to route
      if (item.route) {
        router.push(item.route);
      }
      
      // Close drawer after navigation
      emit("close");
    };

    const updateActiveItemFromRoute = () => {
      const currentPath = route.path;
      const currentItem = navItems.find((item) => item.route === currentPath);
      if (currentItem) {
        activeItem.value = currentItem.id;
      } else if (currentPath === "/" || currentPath === "/trade") {
        activeItem.value = "trade";
      }
    };

    const openHelp = () => {
      console.log("Opening help");
      emit("close");
    };

    // Watch for route changes
    watch(route, updateActiveItemFromRoute, { immediate: true });

    // Prevent body scroll when drawer is open
    watch(
      () => props.visible,
      (isVisible) => {
        if (isVisible) {
          document.body.style.overflow = "hidden";
        } else {
          document.body.style.overflow = "";
        }
      }
    );

    onMounted(() => {
      updateActiveItemFromRoute();
    });

    return {
      activeItem,
      navItems,
      handleNavClick,
      openHelp,
    };
  },
};
</script>

<style scoped>
.nav-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  z-index: 998;
  backdrop-filter: blur(2px);
}

.mobile-nav-drawer {
  position: fixed;
  top: 0;
  left: 0;
  height: 100vh;
  width: 280px;
  background-color: var(--bg-primary, #0b0d10);
  border-right: 1px solid var(--border-primary, #1a1d23);
  z-index: 999;
  transform: translateX(-100%);
  transition: transform 0.3s ease-in-out;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.mobile-nav-drawer.drawer-open {
  transform: translateX(0);
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-primary, #1a1d23);
  background-color: var(--bg-secondary, #141519);
}

.app-logo {
  display: flex;
  align-items: center;
}

.logo-svg {
  height: 32px;
  width: auto;
  display: block;
  object-fit: contain;
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-secondary, #cccccc);
  font-size: 18px;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
}

.close-btn:hover {
  background-color: var(--bg-tertiary, #1a1d23);
  color: var(--text-primary, #ffffff);
}

.nav-items {
  flex: 1;
  padding: 16px 0;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 12px 20px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  margin: 2px 12px;
  border-radius: 8px;
}

.nav-item:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.nav-item.active {
  background-color: var(--bg-quaternary, #2a2d33);
  color: var(--text-primary, #ffffff);
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-right: 16px;
}

.nav-icon i {
  font-size: 18px;
  color: var(--text-secondary, #cccccc);
  transition: color 0.2s ease;
}

.nav-item:hover .nav-icon i,
.nav-item.active .nav-icon i {
  color: var(--text-primary, #ffffff);
}

.nav-label {
  font-size: 16px;
  font-weight: 500;
  color: var(--text-secondary, #cccccc);
  transition: color 0.2s ease;
  flex: 1;
}

.nav-item:hover .nav-label,
.nav-item.active .nav-label {
  color: var(--text-primary, #ffffff);
}

.active-indicator {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 24px;
  background-color: var(--color-brand, #007bff);
  border-radius: 2px 0 0 2px;
}

.nav-bottom {
  border-top: 1px solid var(--border-primary, #1a1d23);
  padding: 16px 0;
}

.user-section {
  padding: 16px 20px;
  border-top: 1px solid var(--border-primary, #1a1d23);
}

.user-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background-color: var(--bg-tertiary, #1a1d23);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary, #cccccc);
}

.user-avatar i {
  font-size: 18px;
}

.user-details {
  flex: 1;
}

.user-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
  margin-bottom: 2px;
}

.user-status {
  font-size: 12px;
  color: var(--color-success, #00c851);
  display: flex;
  align-items: center;
}

.user-status::before {
  content: "";
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: var(--color-success, #00c851);
  margin-right: 6px;
}

/* Scrollbar styling */
.mobile-nav-drawer::-webkit-scrollbar {
  width: 4px;
}

.mobile-nav-drawer::-webkit-scrollbar-track {
  background: transparent;
}

.mobile-nav-drawer::-webkit-scrollbar-thumb {
  background: var(--border-secondary, #2a2d33);
  border-radius: 2px;
}

.mobile-nav-drawer::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary, #3a3d43);
}

/* Animation for smooth opening */
@media (prefers-reduced-motion: reduce) {
  .mobile-nav-drawer {
    transition: none;
  }
}
</style>
