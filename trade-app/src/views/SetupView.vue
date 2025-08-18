<template>
  <div class="setup-view">
    <SetupWizard />
  </div>
</template>

<script>
import SetupWizard from '../components/setup/SetupWizard.vue';

export default {
  name: 'SetupView',
  components: {
    SetupWizard
  }
};
</script>

<style scoped>
.setup-view {
  height: 100vh;
  width: 100vw;
  background: linear-gradient(135deg, #141519 0%, #1a1d23 100%);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
</style>
