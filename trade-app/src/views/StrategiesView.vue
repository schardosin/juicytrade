<template>
  <div class="strategies-view-container">
    <!-- Top Bar -->
    <TopBar />

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Left Navigation -->
      <SideNav />

      <!-- Content Area -->
      <div class="content-area">
        <router-view />
      </div>
    </div>
  </div>
</template>

<script>
import TopBar from '../components/TopBar.vue'
import SideNav from '../components/SideNav.vue'

export default {
  name: 'StrategiesView',
  components: {
    TopBar,
    SideNav
  }
}
</script>

<style scoped>
.strategies-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: #141519;
  color: #ffffff;
}

.main-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
  position: relative;
}

.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: #141519;
}
</style>
