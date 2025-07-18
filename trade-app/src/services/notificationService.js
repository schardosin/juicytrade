import { ref, reactive } from "vue";

/**
 * Centralized Notification Service
 * Manages all application notifications with queue system and auto-dismiss
 */
class NotificationService {
  constructor() {
    this.notifications = ref([]);
    this.nextId = 1;
    this.defaultDurations = {
      success: 4000,
      error: 6000,
      warning: 5000,
      info: 4000,
      loading: 0, // No auto-dismiss for loading
    };
  }

  /**
   * Add a new notification
   * @param {Object} notification - Notification configuration
   * @returns {number} Notification ID
   */
  add(notification) {
    const id = this.nextId++;
    const duration =
      notification.duration !== undefined
        ? notification.duration
        : this.defaultDurations[notification.type] || 4000;

    const notificationItem = reactive({
      id,
      type: notification.type || "info",
      title: notification.title || "",
      message: notification.message || "",
      duration,
      persistent: duration === 0,
      timestamp: Date.now(),
      visible: false, // For animation
      timer: null,
      pausedTime: null,
      remainingTime: duration,
    });

    // Add to notifications array
    this.notifications.value.push(notificationItem);

    // Trigger entrance animation
    setTimeout(() => {
      notificationItem.visible = true;
    }, 10);

    // Set auto-dismiss timer if not persistent
    if (!notificationItem.persistent) {
      this.startTimer(notificationItem);
    }

    return id;
  }

  /**
   * Start auto-dismiss timer for notification
   * @param {Object} notification - Notification item
   */
  startTimer(notification) {
    if (notification.timer) {
      clearTimeout(notification.timer);
    }

    notification.timer = setTimeout(() => {
      this.dismiss(notification.id);
    }, notification.remainingTime);
  }

  /**
   * Pause auto-dismiss timer (on hover)
   * @param {number} id - Notification ID
   */
  pause(id) {
    const notification = this.notifications.value.find((n) => n.id === id);
    if (notification && notification.timer && !notification.persistent) {
      clearTimeout(notification.timer);
      notification.pausedTime = Date.now();
    }
  }

  /**
   * Resume auto-dismiss timer (on mouse leave)
   * @param {number} id - Notification ID
   */
  resume(id) {
    const notification = this.notifications.value.find((n) => n.id === id);
    if (notification && notification.pausedTime && !notification.persistent) {
      const elapsed = Date.now() - notification.pausedTime;
      notification.remainingTime = Math.max(
        0,
        notification.remainingTime - elapsed
      );
      notification.pausedTime = null;

      if (notification.remainingTime > 0) {
        this.startTimer(notification);
      } else {
        this.dismiss(notification.id);
      }
    }
  }

  /**
   * Dismiss a notification by ID
   * @param {number} id - Notification ID
   */
  dismiss(id) {
    const notification = this.notifications.value.find((n) => n.id === id);
    if (notification) {
      // Calculate how long the notification has been visible
      const timeVisible = Date.now() - notification.timestamp;
      const minimumVisibleTime = 1000; // 1 second minimum

      // Clear timer if exists
      if (notification.timer) {
        clearTimeout(notification.timer);
      }

      // If notification hasn't been visible for minimum time, delay dismissal
      if (timeVisible < minimumVisibleTime) {
        const remainingTime = minimumVisibleTime - timeVisible;
        setTimeout(() => {
          this.dismiss(id); // Recursive call after minimum time
        }, remainingTime);
        return;
      }

      // Trigger exit animation
      notification.visible = false;

      // Remove from array after animation
      setTimeout(() => {
        const index = this.notifications.value.findIndex((n) => n.id === id);
        if (index > -1) {
          this.notifications.value.splice(index, 1);
        }
      }, 300); // Match CSS animation duration
    }
  }

  /**
   * Dismiss all notifications
   */
  dismissAll() {
    this.notifications.value.forEach((notification) => {
      if (notification.timer) {
        clearTimeout(notification.timer);
      }
      notification.visible = false;
    });

    // Clear array after animations
    setTimeout(() => {
      this.notifications.value.splice(0);
    }, 300);
  }

  /**
   * Show success notification
   * @param {string} message - Notification message
   * @param {string} title - Optional title
   * @param {number} duration - Optional duration override
   * @returns {number} Notification ID
   */
  showSuccess(message, title = "", duration = undefined) {
    return this.add({
      type: "success",
      title,
      message,
      duration,
    });
  }

  /**
   * Show error notification
   * @param {string} message - Notification message
   * @param {string} title - Optional title
   * @param {number} duration - Optional duration override
   * @returns {number} Notification ID
   */
  showError(message, title = "", duration = undefined) {
    return this.add({
      type: "error",
      title,
      message,
      duration,
    });
  }

  /**
   * Show warning notification
   * @param {string} message - Notification message
   * @param {string} title - Optional title
   * @param {number} duration - Optional duration override
   * @returns {number} Notification ID
   */
  showWarning(message, title = "", duration = undefined) {
    return this.add({
      type: "warning",
      title,
      message,
      duration,
    });
  }

  /**
   * Show info notification
   * @param {string} message - Notification message
   * @param {string} title - Optional title
   * @param {number} duration - Optional duration override
   * @returns {number} Notification ID
   */
  showInfo(message, title = "", duration = undefined) {
    return this.add({
      type: "info",
      title,
      message,
      duration,
    });
  }

  /**
   * Show loading notification (persistent until manually dismissed)
   * @param {string} message - Notification message
   * @param {string} title - Optional title
   * @returns {number} Notification ID
   */
  showLoading(message, title = "") {
    return this.add({
      type: "loading",
      title,
      message,
      duration: 0, // Persistent
    });
  }

  /**
   * Update an existing notification
   * @param {number} id - Notification ID
   * @param {Object} updates - Properties to update
   */
  update(id, updates) {
    const notification = this.notifications.value.find((n) => n.id === id);
    if (notification) {
      Object.assign(notification, updates);

      // If changing to non-persistent, start timer
      if (
        updates.duration !== undefined &&
        updates.duration > 0 &&
        notification.persistent
      ) {
        notification.persistent = false;
        notification.remainingTime = updates.duration;
        this.startTimer(notification);
      }
    }
  }

  /**
   * Get all active notifications
   * @returns {Array} Array of notification objects
   */
  getAll() {
    return this.notifications.value;
  }

  /**
   * Get notification count
   * @returns {number} Number of active notifications
   */
  getCount() {
    return this.notifications.value.length;
  }
}

// Create singleton instance
const notificationService = new NotificationService();

export default notificationService;
