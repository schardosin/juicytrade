import notificationService from "../services/notificationService";

/**
 * Composable for using the notification system
 * Provides a convenient interface to the notification service
 */
export function useNotifications() {
  return {
    // Direct access to the service
    service: notificationService,

    // Convenience methods
    showSuccess: (message, title, duration) =>
      notificationService.showSuccess(message, title, duration),

    showError: (message, title, duration) =>
      notificationService.showError(message, title, duration),

    showWarning: (message, title, duration) =>
      notificationService.showWarning(message, title, duration),

    showInfo: (message, title, duration) =>
      notificationService.showInfo(message, title, duration),

    showLoading: (message, title) =>
      notificationService.showLoading(message, title),

    dismiss: (id) => notificationService.dismiss(id),

    dismissAll: () => notificationService.dismissAll(),

    update: (id, updates) => notificationService.update(id, updates),

    // Get reactive notifications array
    notifications: notificationService.notifications,

    // Get notification count
    getCount: () => notificationService.getCount(),

    // Utility methods for common use cases
    showOrderSuccess: (orderId, action = "processed") => {
      return notificationService.showSuccess(
        `Order #${orderId} has been ${action} successfully`,
        "Order Success"
      );
    },

    showOrderError: (orderId, error, action = "processing") => {
      return notificationService.showError(
        error || `Failed to ${action} order`,
        `Order Error #${orderId}`
      );
    },

    showOrderLoading: (orderId, action = "processing") => {
      return notificationService.showLoading(
        `${
          action.charAt(0).toUpperCase() + action.slice(1)
        } order #${orderId}...`,
        "Order Processing"
      );
    },

    showApiError: (error, context = "API") => {
      let message = "An unexpected error occurred";

      if (typeof error === "string") {
        message = error;
      } else if (error?.response?.data?.message) {
        message = error.response.data.message;
      } else if (error?.message) {
        message = error.message;
      }

      return notificationService.showError(message, `${context} Error`);
    },

    showNetworkError: () => {
      return notificationService.showError(
        "Unable to connect to the server. Please check your internet connection.",
        "Network Error"
      );
    },

    showValidationError: (errors) => {
      const message = Array.isArray(errors) ? errors.join(", ") : errors;
      return notificationService.showWarning(message, "Validation Error");
    },
  };
}
