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
import { ref } from "vue";

export default {
  name: "SideNav",
  setup() {
    // Reactive data
    const activeItem = ref("trade");

    // Navigation items matching the JuicyTrade layout
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
        route: "/options-trading",
      },
      {
        id: "activity",
        label: "Activity",
        icon: "pi pi-clock",
        route: "/activity",
      },
      {
        id: "watchlist",
        label: "Watchlist",
        icon: "pi pi-eye",
        route: "/watchlist",
      },
      {
        id: "history",
        label: "History",
        icon: "pi pi-history",
        route: "/history",
      },
      {
        id: "fixed-income",
        label: "Fixed Income",
        icon: "pi pi-money-bill",
        route: "/fixed-income",
      },
      {
        id: "backtest",
        label: "Backtest",
        icon: "pi pi-chart-bar",
        route: "/backtest",
      },
      {
        id: "lastview",
        label: "tastylive",
        icon: "pi pi-video",
        route: "/tastylive",
      },
    ];

    // Methods
    const setActiveItem = (itemId) => {
      activeItem.value = itemId;
      const item = navItems.find((nav) => nav.id === itemId);
      if (item) {
        console.log(`Navigating to ${item.route}`);
        // Here you would typically handle routing
        // this.$router.push(item.route);
      }
    };

    const openHelp = () => {
      console.log("Opening help");
      // Here you would open help documentation or support
    };

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
  width: 80px;
  background-color: #1a1a1a;
  border-right: 1px solid #333333;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 16px 0;
  overflow-y: auto;
}

.nav-items {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  cursor: pointer;
  border-radius: 8px;
  margin: 0 8px;
  transition: all 0.2s ease;
  position: relative;
}

.nav-item:hover {
  background-color: #333333;
}

.nav-item.active {
  background-color: #444444;
}

.nav-item.active::before {
  content: "";
  position: absolute;
  left: -8px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 24px;
  background-color: #ff6b35;
  border-radius: 0 2px 2px 0;
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-bottom: 4px;
}

.nav-icon i {
  font-size: 18px;
  color: #cccccc;
  transition: color 0.2s ease;
}

.nav-item:hover .nav-icon i,
.nav-item.active .nav-icon i {
  color: #ffffff;
}

.nav-label {
  font-size: 10px;
  font-weight: 500;
  color: #888888;
  text-align: center;
  line-height: 1.2;
  transition: color 0.2s ease;
  word-break: break-word;
  max-width: 60px;
}

.nav-item:hover .nav-label,
.nav-item.active .nav-label {
  color: #cccccc;
}

.nav-bottom {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid #333333;
}

/* Scrollbar styling for the nav */
.side-nav::-webkit-scrollbar {
  width: 4px;
}

.side-nav::-webkit-scrollbar-track {
  background: transparent;
}

.side-nav::-webkit-scrollbar-thumb {
  background: #444444;
  border-radius: 2px;
}

.side-nav::-webkit-scrollbar-thumb:hover {
  background: #555555;
}

/* Responsive adjustments */
@media (max-height: 600px) {
  .nav-item {
    padding: 8px 6px;
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
