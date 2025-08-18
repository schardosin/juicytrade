import { createRouter, createWebHistory } from "vue-router";
import OptionsTrading from "../views/OptionsTrading.vue";
import ChartView from "../views/ChartView.vue";
import PositionsView from "../views/PositionsView.vue";
import SmartMarketDataTest from "../components/SmartMarketDataTest.vue";
import SetupView from "../views/SetupView.vue";
import { api } from "../services/api.js";

const routes = [
  {
    path: "/setup",
    name: "Setup",
    component: SetupView,
    meta: {
      title: "Setup - Trading Platform",
      requiresSetup: false, // This route doesn't require setup to be complete
    },
  },
  {
    path: "/",
    name: "Trade",
    component: OptionsTrading,
    meta: {
      title: "Trading Platform",
    },
  },
  {
    path: "/trade",
    name: "TradeAlias",
    component: OptionsTrading,
    meta: {
      title: "Trading Platform",
    },
  },
  {
    path: "/chart",
    name: "ChartView",
    component: ChartView,
    meta: {
      title: "Chart View - Trading Platform",
    },
  },
  {
    path: "/positions",
    name: "Positions",
    component: PositionsView,
    meta: {
      title: "Positions - Trading Platform",
    },
  },
  {
    path: "/test",
    name: "SmartMarketDataTest",
    component: SmartMarketDataTest,
    meta: {
      title: "Smart Market Data Test - Trading Platform",
    },
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Navigation guard to check setup status
router.beforeEach(async (to, from, next) => {
  // Skip setup check for the setup route itself
  if (to.meta.requiresSetup === false || to.name === 'Setup') {
    next();
    return;
  }

  try {
    // Check if mandatory routes are configured
    const setupStatus = await api.checkSetupStatus();
    
    console.log('Setup status check:', setupStatus);
    
    if (!setupStatus.is_setup_complete) {
      // Redirect to setup if mandatory routes are not configured
      console.log('Setup incomplete, redirecting to setup wizard. Missing services:', setupStatus.missing_mandatory_services);
      next({ name: 'Setup' });
      return;
    }
    
    // Setup is complete, proceeding to the requested route
    console.log('Setup is complete, proceeding to route:', to.name);
    next();
  } catch (error) {
    console.error('Error checking setup status:', error);
    // If we can't check setup status, allow navigation but log the error
    // This prevents the app from being completely unusable if the API is down
    next();
  }
});

export default router;
