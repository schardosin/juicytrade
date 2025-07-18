<template>
  <div
    class="notification-item"
    :class="[
      `notification-${notification.type}`,
      { 'notification-visible': notification.visible },
    ]"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
    @click="handleClick"
  >
    <!-- Icon -->
    <div class="notification-icon">
      <i :class="getIconClass()"></i>
    </div>

    <!-- Content -->
    <div class="notification-content">
      <div v-if="notification.title" class="notification-title">
        {{ notification.title }}
      </div>
      <div class="notification-message">
        {{ notification.message }}
      </div>
    </div>

    <!-- Loading spinner for loading type -->
    <div v-if="notification.type === 'loading'" class="notification-spinner">
      <div class="spinner"></div>
    </div>

    <!-- Close button -->
    <button
      v-if="!notification.persistent || notification.type !== 'loading'"
      class="notification-close"
      @click.stop="handleClose"
      aria-label="Close notification"
    >
      <i class="pi pi-times"></i>
    </button>

    <!-- Progress bar for timed notifications -->
    <div
      v-if="!notification.persistent && showProgressBar"
      class="notification-progress"
    >
      <div
        class="notification-progress-bar"
        :style="{ animationDuration: `${notification.duration}ms` }"
      ></div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted } from "vue";

export default {
  name: "NotificationItem",
  props: {
    notification: {
      type: Object,
      required: true,
    },
    showProgressBar: {
      type: Boolean,
      default: true,
    },
  },
  emits: ["dismiss", "pause", "resume"],
  setup(props, { emit }) {
    const isHovered = ref(false);

    // Get appropriate icon for notification type
    const getIconClass = () => {
      const iconMap = {
        success: "pi pi-check-circle",
        error: "pi pi-times-circle",
        warning: "pi pi-exclamation-triangle",
        info: "pi pi-info-circle",
        loading: "pi pi-spin pi-spinner",
      };
      return iconMap[props.notification.type] || "pi pi-info-circle";
    };

    // Handle mouse enter (pause timer)
    const handleMouseEnter = () => {
      isHovered.value = true;
      emit("pause", props.notification.id);
    };

    // Handle mouse leave (resume timer)
    const handleMouseLeave = () => {
      isHovered.value = false;
      emit("resume", props.notification.id);
    };

    // Handle notification click
    const handleClick = () => {
      // Optional: Add click handler for notification actions
      // For now, just dismiss on click
      if (!props.notification.persistent) {
        emit("dismiss", props.notification.id);
      }
    };

    // Handle close button click
    const handleClose = () => {
      emit("dismiss", props.notification.id);
    };

    return {
      isHovered,
      getIconClass,
      handleMouseEnter,
      handleMouseLeave,
      handleClick,
      handleClose,
    };
  },
};
</script>

<style scoped>
.notification-item {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-md);
  padding: var(--spacing-lg);
  margin-bottom: var(--spacing-md);
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  cursor: pointer;
  position: relative;
  overflow: hidden;
  min-width: 320px;
  max-width: 400px;
  transform: translateX(100%);
  opacity: 0;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.notification-item.notification-visible {
  transform: translateX(0);
  opacity: 1;
}

.notification-item:hover {
  background-color: var(--bg-quaternary);
  border-color: var(--border-tertiary);
  box-shadow: var(--shadow-md);
}

/* Type-specific styling */
.notification-success {
  border-left: 4px solid var(--color-success);
}

.notification-error {
  border-left: 4px solid var(--color-danger);
}

.notification-warning {
  border-left: 4px solid var(--color-warning);
}

.notification-info {
  border-left: 4px solid var(--color-info);
}

.notification-loading {
  border-left: 4px solid var(--color-primary);
}

/* Icon styling */
.notification-icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 2px;
}

.notification-success .notification-icon {
  color: var(--color-success);
}

.notification-error .notification-icon {
  color: var(--color-danger);
}

.notification-warning .notification-icon {
  color: var(--color-warning);
}

.notification-info .notification-icon {
  color: var(--color-info);
}

.notification-loading .notification-icon {
  color: var(--color-primary);
}

.notification-icon i {
  font-size: 18px;
}

/* Content styling */
.notification-content {
  flex: 1;
  min-width: 0;
}

.notification-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: var(--spacing-xs);
  line-height: 1.4;
}

.notification-message {
  font-size: var(--font-size-base);
  color: var(--text-secondary);
  line-height: 1.5;
  word-wrap: break-word;
}

/* Spinner for loading notifications */
.notification-spinner {
  flex-shrink: 0;
  margin-left: var(--spacing-sm);
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Close button */
.notification-close {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  border: none;
  background: none;
  color: var(--text-tertiary);
  cursor: pointer;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: var(--transition-fast);
  margin-top: 2px;
}

.notification-close:hover {
  background-color: var(--bg-quaternary);
  color: var(--text-secondary);
}

.notification-close i {
  font-size: 12px;
}

/* Progress bar */
.notification-progress {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 3px;
  background-color: var(--border-secondary);
  overflow: hidden;
}

.notification-progress-bar {
  height: 100%;
  width: 100%;
  background-color: var(--color-primary);
  transform: translateX(-100%);
  animation: progress-bar linear forwards;
}

.notification-success .notification-progress-bar {
  background-color: var(--color-success);
}

.notification-error .notification-progress-bar {
  background-color: var(--color-danger);
}

.notification-warning .notification-progress-bar {
  background-color: var(--color-warning);
}

.notification-info .notification-progress-bar {
  background-color: var(--color-info);
}

@keyframes progress-bar {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(0);
  }
}

/* Responsive design */
@media (max-width: 480px) {
  .notification-item {
    min-width: 280px;
    max-width: 320px;
    padding: var(--spacing-md);
  }

  .notification-title {
    font-size: var(--font-size-base);
  }

  .notification-message {
    font-size: var(--font-size-sm);
  }
}

/* Accessibility */
.notification-item:focus {
  outline: 2px solid var(--color-info);
  outline-offset: 2px;
}

.notification-close:focus {
  outline: 2px solid var(--color-info);
  outline-offset: 1px;
}

/* High contrast mode support */
@media (prefers-contrast: high) {
  .notification-item {
    border-width: 2px;
  }

  .notification-success {
    border-left-width: 6px;
  }

  .notification-error {
    border-left-width: 6px;
  }

  .notification-warning {
    border-left-width: 6px;
  }

  .notification-info {
    border-left-width: 6px;
  }

  .notification-loading {
    border-left-width: 6px;
  }
}

/* Reduced motion support */
@media (prefers-reduced-motion: reduce) {
  .notification-item {
    transition: opacity 0.2s ease;
  }

  .notification-progress-bar {
    animation: none;
    transform: translateX(0);
  }

  .spinner {
    animation: none;
  }
}
</style>
