import { createRouter, createWebHistory } from "vue-router";
import OptionsTrading from "../views/OptionsTrading.vue";
import ChartView from "../views/ChartView.vue";
import PositionsView from "../views/PositionsView.vue";

const routes = [
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
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
