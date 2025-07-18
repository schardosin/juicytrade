<template>
  <teleport to="body">
    <div
      v-if="notifications.length > 0"
      class="notification-container"
      :class="{ 'has-notifications': notifications.length > 0 }"
    >
      <transition-group
        name="notification"
        tag="div"
        class="notification-list"
        @enter="onEnter"
        @leave="onLeave"
      >
        <NotificationItem
          v-for="notification in notifications"
          :key="notification.id"
          :notification="notification"
          :show-progress-bar="showProgressBar"
          @dismiss="handleDismiss"
          @pause="handlePause"
          @resume="handleResume"
        />
      </transition-group>
    </div>
  </teleport>
</template>

<script>
import { computed } from "vue";
import NotificationItem from "./NotificationItem.vue";
import notificationService from "../../services/notificationService.js";

export default {
  name: "NotificationContainer",
  components: {
    NotificationItem,
  },
  props: {
    position: {
      type: String,
      default: "top-right",
      validator: (value) =>
        [
          "top-left",
          "top-right",
          "top-center",
          "bottom-left",
          "bottom-right",
          "bottom-center",
        ].includes(value),
    },
    showProgressBar: {
      type: Boolean,
      default: true,
    },
    maxNotifications: {
      type: Number,
      default: 5,
    },
  },
  setup(props) {
    // Get notifications from service
    const notifications = computed(() => {
      const allNotifications = notificationService.getAll();
      // Limit the number of visible notifications
      return allNotifications.slice(0, props.maxNotifications);
    });

    // Handle notification dismiss
    const handleDismiss = (id) => {
      notificationService.dismiss(id);
    };

    // Handle notification pause
    const handlePause = (id) => {
      notificationService.pause(id);
    };

    // Handle notification resume
    const handleResume = (id) => {
      notificationService.resume(id);
    };

    // Animation hooks
    const onEnter = (el) => {
      // Force reflow to ensure animation works
      el.offsetHeight;
    };

    const onLeave = (el) => {
      // Clean up any remaining timers or references
      el.style.transition = "all 0.3s ease";
      el.style.transform = "translateX(100%)";
      el.style.opacity = "0";
    };

    return {
      notifications,
      handleDismiss,
      handlePause,
      handleResume,
      onEnter,
      onLeave,
    };
  },
};
</script>

<style scoped>
.notification-container {
  position: fixed;
  z-index: 10000;
  pointer-events: none;
  max-height: 100vh;
  overflow: hidden;
}

/* Position variants */
.notification-container {
  top: var(--spacing-xl);
  right: var(--spacing-xl);
}

.notification-container.position-top-left {
  top: var(--spacing-xl);
  left: var(--spacing-xl);
  right: auto;
}

.notification-container.position-top-center {
  top: var(--spacing-xl);
  left: 50%;
  right: auto;
  transform: translateX(-50%);
}

.notification-container.position-bottom-left {
  bottom: var(--spacing-xl);
  left: var(--spacing-xl);
  top: auto;
  right: auto;
}

.notification-container.position-bottom-right {
  bottom: var(--spacing-xl);
  right: var(--spacing-xl);
  top: auto;
}

.notification-container.position-bottom-center {
  bottom: var(--spacing-xl);
  left: 50%;
  right: auto;
  top: auto;
  transform: translateX(-50%);
}

.notification-list {
  display: flex;
  flex-direction: column;
  gap: 0;
  pointer-events: auto;
}

/* Transition animations */
.notification-enter-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.notification-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.notification-enter-from {
  transform: translateX(100%);
  opacity: 0;
}

.notification-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

.notification-move {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

/* For bottom positions, reverse the animation direction */
.notification-container.position-bottom-left .notification-enter-from,
.notification-container.position-bottom-right .notification-enter-from,
.notification-container.position-bottom-center .notification-enter-from {
  transform: translateY(100%);
}

.notification-container.position-bottom-left .notification-leave-to,
.notification-container.position-bottom-right .notification-leave-to,
.notification-container.position-bottom-center .notification-leave-to {
  transform: translateY(100%);
}

/* For left positions, reverse the horizontal animation */
.notification-container.position-top-left .notification-enter-from,
.notification-container.position-bottom-left .notification-enter-from {
  transform: translateX(-100%);
}

.notification-container.position-top-left .notification-leave-to,
.notification-container.position-bottom-left .notification-leave-to {
  transform: translateX(-100%);
}

/* For center positions, use scale animation */
.notification-container.position-top-center .notification-enter-from,
.notification-container.position-bottom-center .notification-enter-from {
  transform: translateX(-50%) scale(0.8);
}

.notification-container.position-top-center .notification-leave-to,
.notification-container.position-bottom-center .notification-leave-to {
  transform: translateX(-50%) scale(0.8);
}

/* Responsive design */
@media (max-width: 768px) {
  .notification-container {
    top: var(--spacing-md);
    right: var(--spacing-md);
    left: var(--spacing-md);
    max-width: none;
  }

  .notification-container.position-top-left,
  .notification-container.position-top-center,
  .notification-container.position-bottom-left,
  .notification-container.position-bottom-right,
  .notification-container.position-bottom-center {
    left: var(--spacing-md);
    right: var(--spacing-md);
    transform: none;
  }

  .notification-container.position-bottom-left,
  .notification-container.position-bottom-right,
  .notification-container.position-bottom-center {
    bottom: var(--spacing-md);
    top: auto;
  }

  /* Mobile animations - slide from top */
  .notification-enter-from {
    transform: translateY(-100%);
    opacity: 0;
  }

  .notification-leave-to {
    transform: translateY(-100%);
    opacity: 0;
  }
}

/* Accessibility - respect reduced motion preferences */
@media (prefers-reduced-motion: reduce) {
  .notification-enter-active,
  .notification-leave-active,
  .notification-move {
    transition: opacity 0.2s ease;
  }

  .notification-enter-from,
  .notification-leave-to {
    transform: none;
    opacity: 0;
  }
}

/* High contrast mode */
@media (prefers-contrast: high) {
  .notification-container {
    filter: contrast(1.2);
  }
}

/* Print styles - hide notifications when printing */
@media print {
  .notification-container {
    display: none !important;
  }
}
</style>
