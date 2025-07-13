import { createRouter, createWebHistory } from "vue-router";
import OptionsTrading from "../views/OptionsTrading.vue";
import ChartView from "../views/ChartView.vue";

const routes = [
  {
    path: "/",
    name: "OptionsTrading",
    component: OptionsTrading,
    meta: {
      title: "Options Trading Platform",
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
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
