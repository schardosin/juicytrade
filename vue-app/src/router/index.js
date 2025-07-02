import { createRouter, createWebHistory } from "vue-router";
import TradeSetup from "../views/TradeSetup.vue";
import TradeManagement from "../views/TradeManagement.vue";

const routes = [
  {
    path: "/",
    name: "TradeSetup",
    component: TradeSetup,
    meta: {
      title: "Trade Setup",
    },
  },
  {
    path: "/trade-management",
    name: "TradeManagement",
    component: TradeManagement,
    meta: {
      title: "Trade Management",
    },
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
