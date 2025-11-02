import { createRouter, createWebHistory } from "vue-router";
import OptionsTrading from "../views/OptionsTrading.vue";
import ChartView from "../views/ChartView.vue";
import PositionsView from "../views/PositionsView.vue";
import SmartMarketDataTest from "../components/SmartMarketDataTest.vue";
import SetupView from "../views/SetupView.vue";
import StrategiesView from "../views/StrategiesView.vue";
import LoginPage from "../components/auth/LoginPage.vue";
import { api } from "../services/api.js";
import authService from "../services/authService.js";

const routes = [
  {
    path: "/login",
    name: "Login",
    component: LoginPage,
    meta: {
      title: "Login - JuicyTrade",
      requiresAuth: false,
      requiresSetup: false,
    },
  },
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
  {
    path: "/strategies",
    name: "Strategies",
    component: StrategiesView,
    meta: {
      title: "Strategies - Trading Platform",
    },
    children: [
      {
        path: "",
        name: "StrategyDashboard",
        component: () => import("../components/strategies/StrategyDashboard.vue"),
        meta: {
          title: "Strategy Dashboard - Trading Platform",
        },
      },
      {
        path: "library",
        name: "StrategyLibrary",
        component: () => import("../components/strategies/StrategyLibrary.vue"),
        meta: {
          title: "Strategy Library - Trading Platform",
        },
      },
      {
        path: "monitor/:id",
        name: "StrategyMonitor",
        component: () => import("../components/strategies/StrategyMonitor.vue"),
        meta: {
          title: "Strategy Monitor - Trading Platform",
        },
      },
      {
        path: "backtest/:id",
        name: "StrategyBacktest",
        component: () => import("../components/strategies/StrategyBacktest.vue"),
        meta: {
          title: "Strategy Backtest - Trading Platform",
        },
      },
      {
        path: "backtest",
        name: "StrategyBacktesting",
        component: () => import("../components/strategies/StrategyBacktesting.vue"),
        meta: {
          title: "Strategy Backtesting - Trading Platform",
        },
      },
      {
        path: "live",
        name: "StrategyLiveTrading",
        component: () => import("../components/strategies/StrategyLiveTrading.vue"),
        meta: {
          title: "Live Trading - Trading Platform",
        },
      },
      {
        path: "data",
        name: "StrategyData",
        component: () => import("../components/strategies/StrategyData.vue"),
        meta: {
          title: "Data Management - Trading Platform",
        },
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Navigation guard to check authentication and setup status
router.beforeEach(async (to, from, next) => {
  // Skip checks for routes that don't require them
  if (to.meta.requiresAuth === false || to.meta.requiresSetup === false || 
      to.name === 'Login' || to.name === 'Setup') {
    next();
    return;
  }

  try {
    // Initialize auth service if not already done
    if (!authService.isAuthenticated() && authService.getAuthMethod() === 'disabled') {
      await authService.init();
    }

    // Check authentication first
    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.log('Authentication required, redirecting to login');
      next({ 
        name: 'Login', 
        query: { next: to.fullPath } 
      });
      return;
    }

    // Only check setup status if user is authenticated (or auth is disabled)
    // This prevents API calls before authentication is complete
    if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
      // Check if mandatory routes are configured
      const setupStatus = await api.checkSetupStatus();
      
      if (!setupStatus.is_setup_complete) {
        // Redirect to setup if mandatory routes are not configured
        console.log('Setup incomplete, redirecting to setup wizard. Missing services:', setupStatus.missing_mandatory_services);
        next({ name: 'Setup' });
        return;
      }
    }
    
    // Both auth and setup are complete, proceeding to the requested route
    next();
  } catch (error) {
    console.error('Error in navigation guard:', error);
    // If we can't check status, allow navigation but log the error
    // This prevents the app from being completely unusable if the API is down
    next();
  }
});

export default router;
