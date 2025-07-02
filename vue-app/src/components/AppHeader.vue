<template>
  <div class="app-header">
    <Menubar :model="menuItems" class="custom-menubar">
      <template #start>
        <div class="app-logo">
          <i
            class="pi pi-chart-line"
            style="font-size: 1.5rem; margin-right: 0.5rem; color: #007bff"
          ></i>
          <span class="app-title">Smart Iron Butterfly</span>
        </div>
      </template>
    </Menubar>
  </div>
</template>

<script>
import { ref, computed } from "vue";
import { useRouter, useRoute } from "vue-router";

export default {
  name: "AppHeader",
  setup() {
    const router = useRouter();
    const route = useRoute();

    const menuItems = computed(() => [
      {
        label: "Trade Setup",
        icon: "pi pi-cog",
        command: () => {
          router.push("/");
        },
        class: route.name === "TradeSetup" ? "active-menu-item" : "",
      },
      {
        label: "Trade Management",
        icon: "pi pi-chart-bar",
        command: () => {
          router.push("/trade-management");
        },
        class: route.name === "TradeManagement" ? "active-menu-item" : "",
      },
    ]);

    return {
      menuItems,
    };
  },
};
</script>

<style scoped>
.app-header {
  margin-bottom: 2rem;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.custom-menubar {
  border-radius: 0;
  border: none;
  border-bottom: 1px solid #e9ecef;
}

.app-logo {
  display: flex;
  align-items: center;
  font-weight: 600;
  font-size: 1.2rem;
  color: #2c3e50;
}

.app-title {
  background: linear-gradient(45deg, #007bff, #6610f2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-weight: 700;
}

:deep(.p-menubar-root-list) {
  margin-left: auto;
}

:deep(.p-menuitem-link) {
  padding: 1rem 1.5rem;
  border-radius: 6px;
  margin: 0 0.25rem;
  transition: all 0.2s ease;
}

:deep(.p-menuitem-link:hover) {
  background-color: #f8f9fa;
  color: #007bff;
}

:deep(.active-menu-item .p-menuitem-link) {
  background-color: transparent;
  color: #007bff;
  border-bottom: 2px solid #007bff;
  font-weight: 600;
}

:deep(.active-menu-item .p-menuitem-link:hover) {
  background-color: #f8f9fa;
  color: #007bff;
  border-bottom: 2px solid #007bff;
}

:deep(.p-menuitem-icon) {
  margin-right: 0.5rem;
}
</style>
