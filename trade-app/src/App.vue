<template>
  <div id="app">
    <router-view />
    <!-- Global Notification Container -->
    <NotificationContainer />
  </div>
</template>

<script>
import NotificationContainer from "./components/notifications/NotificationContainer.vue";

export default {
  name: "App",
  components: {
    NotificationContainer,
  },
};
</script>

<style>
#app {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen,
    Ubuntu, Cantarell, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  height: 100vh;
  overflow: hidden;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  padding: 0;
  background-color: #141519;
  color: #ffffff;
}

html,
body {
  height: 100%;
  overflow: hidden;
}
</style>
