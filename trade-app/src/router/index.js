import { createRouter, createWebHistory } from "vue-router";
import OptionsTrading from "../views/OptionsTrading.vue";

const routes = [
  {
    path: "/",
    name: "OptionsTrading",
    component: OptionsTrading,
    meta: {
      title: "Options Trading Platform",
    },
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
