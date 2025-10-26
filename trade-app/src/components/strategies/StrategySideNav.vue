<template>
  <div class="side-nav">
    <!-- Navigation Items -->
    <div class="nav-items">
      <div
        v-for="item in navItems"
        :key="item.id"
        :class="['nav-item', { active: activeItem === item.id }]"
        @click="setActiveItem(item.id)"
      >
        <div class="nav-icon">
          <i :class="item.icon"></i>
        </div>
        <div class="nav-label">{{ item.label }}</div>
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
    </div>
  </div>
</template>

<script>
import { ref, onMounted, watch } from "vue";
import { useRouter, useRoute } from "vue-router";

export default {
  name: "StrategySideNav",
  setup() {
    const router = useRouter();
    const route = useRoute();
    // Reactive data
    const activeItem = ref("dashboard");

    // Strategy-specific navigation items
    const navItems = [
      {
        id: "dashboard",
        label: "Dashboard",
        icon: "pi pi-chart-line",
        route: "/strategies",
      },
      {
        id: "library",
        label: "Library",
        icon: "pi pi-folder",
        route: "/strategies/library",
      },
      {
        id: "live",
        label: "Live Trading",
        icon: "pi pi-bolt",
        route: "/strategies/live",
      },
      {
        id: "backtest",
        label: "Backtest",
        icon: "pi pi-chart-bar",
        route: "/strategies/backtest",
      },
      {
        id: "data",
        label: "Data",
        icon: "pi pi-database",
        route: "/strategies/data",
      },
    ];

    // Methods
    const setActiveItem = (itemId) => {
      activeItem.value = itemId;
      const item = navItems.find((nav) => nav.id === itemId);
      if (item) {
        router.push(item.route);
      }
    };

    const updateActiveItemFromRoute = () => {
      const currentPath = route.path;
      
      // Map routes to nav items
      if (currentPath === "/strategies" || currentPath === "/strategies/") {
        activeItem.value = "dashboard";
      } else if (currentPath.startsWith("/strategies/library")) {
        activeItem.value = "library";
      } else if (currentPath.startsWith("/strategies/live")) {
        activeItem.value = "live";
      } else if (currentPath.startsWith("/strategies/backtest")) {
        activeItem.value = "backtest";
      } else if (currentPath.startsWith("/strategies/data")) {
        activeItem.value = "data";
      } else {
        // Find exact match
        const currentItem = navItems.find((item) => item.route === currentPath);
        if (currentItem) {
          activeItem.value = currentItem.id;
        }
      }
    };

    const openHelp = () => {
      console.log("Opening strategy help");
      // Here you would open strategy-specific help documentation
    };

    // Watch for route changes
    watch(route, updateActiveItemFromRoute, { immediate: true });

    // Set initial active item on mount
    onMounted(() => {
      updateActiveItemFromRoute();
    });

    return {
      // Reactive data
      activeItem,

      // Static data
      navItems,

      // Methods
      setActiveItem,
      openHelp,
    };
  },
};
</script>

<style scoped>
.side-nav {
  width: 90px;
  background-color: var(--bg-primary);
  border-right: 1px solid var(--border-primary);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: var(--spacing-lg) 0;
  overflow-y: auto;
}

.nav-items {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-sm);
  cursor: pointer;
  border-radius: var(--radius-lg);
  margin: 0 var(--spacing-sm);
  transition: var(--transition-normal);
  position: relative;
}

.nav-item:hover {
  background-color: var(--bg-tertiary);
}

.nav-item.active {
  background-color: var(--bg-tertiary);
}

.nav-item.active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 24px;
  background-color: var(--color-brand);
  border-radius: 0 2px 2px 0;
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-bottom: var(--spacing-xs);
}

.nav-icon i {
  font-size: 18px;
  color: var(--text-secondary);
  transition: var(--transition-normal);
}

.nav-item:hover .nav-icon i,
.nav-item.active .nav-icon i {
  color: var(--text-primary);
}

.nav-label {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  color: var(--text-tertiary);
  text-align: center;
  line-height: 1.2;
  transition: var(--transition-normal);
  word-break: break-word;
  max-width: 60px;
}

.nav-item:hover .nav-label,
.nav-item.active .nav-label {
  color: var(--text-secondary);
}

.nav-bottom {
  margin-top: auto;
  padding-top: var(--spacing-lg);
  border-top: 1px solid var(--border-primary);
}

/* Scrollbar styling for the nav */
.side-nav::-webkit-scrollbar {
  width: 4px;
}

.side-nav::-webkit-scrollbar-track {
  background: transparent;
}

.side-nav::-webkit-scrollbar-thumb {
  background: var(--border-secondary);
  border-radius: 2px;
}

.side-nav::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary);
}

/* Responsive adjustments */
@media (max-height: 600px) {
  .nav-item {
    padding: var(--spacing-sm) 6px;
  }

  .nav-icon {
    width: 20px;
    height: 20px;
    margin-bottom: 2px;
  }

  .nav-icon i {
    font-size: 16px;
  }

  .nav-label {
    font-size: 9px;
  }
}
</style>
