import { ref, onMounted, onUnmounted } from 'vue';
import webSocketClient from '../services/webSocketClient.js';
import notificationService from '../services/notificationService.js';
import { playOrderNotificationSound } from '../utils/notificationSound.js';

// Singleton state - shared across all useOrderEvents instances
let globalUnsubscribe = null;
let globalListenerActive = false;
const consumers = new Set();
let toastsEnabled = false; // Only one consumer should enable toasts

function getStatusNotificationType(status) {
    const statusLower = status?.toLowerCase() || '';

    if (statusLower === 'filled') return 'success';
    if (statusLower === 'partially_filled') return 'info';
    if (statusLower === 'canceled' || statusLower === 'cancelled' || statusLower === 'expired') return 'warning';
    if (statusLower === 'rejected' || statusLower === 'error') return 'error';
    return 'info';
}

function globalHandleOrderEvent(message) {
    if (!message || !message.data) {
        return;
    }

    const eventData = message.data;

    // Skip heartbeat events
    if (eventData.event === 'heartbeat') return;

    const orderId = eventData.id || eventData.ID;
    const normalizedEvent = eventData.normalized_event || eventData.normalizedEvent || '';

    // Skip child orders (Tradier sends separate events for each leg with parent_id)
    const parentId = eventData.parent_id || eventData.parentId;
    if (parentId) {
        // Still notify all consumers for data refresh
        consumers.forEach(consumer => {
            if (consumer.onOrderUpdate) {
                try {
                    consumer.onOrderUpdate(eventData);
                } catch (e) {
                    console.error('[ORDER-EVENT] Error in consumer callback:', e);
                }
            }
        });
        return;
    }

    // Notify all consumers immediately for data refresh
    consumers.forEach(consumer => {
        if (consumer.onOrderUpdate) {
            try {
                consumer.onOrderUpdate(eventData);
            } catch (e) {
                console.error('[ORDER-EVENT] Error in consumer callback:', e);
            }
        }
    });

    // Show toast only for normalized events (backend handles state tracking)
    if (!normalizedEvent || !toastsEnabled) {
        return;
    }

    // The normalized event determines what to show
    let messageText = '';
    let title = '';
    const symbol = eventData.symbol || eventData.Symbol || 'N/A';
    const price = eventData.price || eventData.Price || 0;
    const orderType = eventData.type || eventData.order_type || '';
    const orderInfo = orderId ? ` #${orderId}` : '';

    switch (normalizedEvent) {
        case 'order_submitted':
            title = `Order Submitted`;
            const priceInfo = price > 0 ? ` @ $${price.toFixed(2)}` : '';
            messageText = symbol !== 'N/A' 
                ? `Order ${symbol} submitted${priceInfo} (${orderType})`
                : `Order${orderInfo} submitted${priceInfo} (${orderType})`;
            break;
            
        case 'order_filled':
            title = symbol !== 'N/A' ? `Order Filled: ${symbol}` : `Order Filled`;
            const filledQty = eventData.exec_quantity || eventData.execQuantity || eventData.executed_quantity || 0;
            const avgFillPrice = eventData.avg_fill_price || eventData.avgFillPrice || 0;
            if (filledQty > 0 && avgFillPrice > 0) {
                messageText = `Filled ${filledQty} @ $${avgFillPrice.toFixed(2)}`;
            } else if (price > 0) {
                messageText = `Filled @ $${price.toFixed(2)}`;
            } else {
                messageText = `Order filled`;
            }
            break;
            
        case 'order_partially_filled':
            title = `Order Partially Filled`;
            const partialFilledQty = eventData.exec_quantity || eventData.execQuantity || eventData.executed_quantity || 0;
            const partialAvgFillPrice = eventData.avg_fill_price || eventData.avgFillPrice || 0;
            const remainingQty = eventData.remaining_quantity || eventData.remainingQuantity || 0;
            if (partialFilledQty > 0 && partialAvgFillPrice > 0) {
                messageText = `Filled ${partialFilledQty} @ $${partialAvgFillPrice.toFixed(2)} (${remainingQty} remaining)`;
            } else {
                messageText = symbol !== 'N/A' 
                    ? `Order ${symbol} partially filled` 
                    : `Order${orderInfo} partially filled`;
            }
            break;
            
        case 'order_cancelled':
            title = `Order Canceled`;
            const remaining = eventData.remaining_quantity || eventData.remainingQuantity || 0;
            if (remaining > 0) {
                messageText = `Order${orderInfo} canceled (${remaining} remaining)`;
            } else {
                messageText = symbol !== 'N/A' 
                    ? `Order ${symbol} was canceled` 
                    : `Order${orderInfo} was canceled`;
            }
            break;
            
        default:
            return; // Unknown normalized event, don't show toast
    }

    // Show the toast
    const notificationType = getStatusNotificationType(eventData.status);
    const notificationMethods = {
        'success': notificationService.showSuccess.bind(notificationService),
        'warning': notificationService.showWarning.bind(notificationService),
        'error': notificationService.showError.bind(notificationService),
        'info': notificationService.showInfo.bind(notificationService),
    };

    const notify = notificationMethods[notificationType] || notificationService.showInfo.bind(notificationService);
    notify(messageText, title, 5000);
    
    // Play notification sound
    playOrderNotificationSound();
}

function ensureGlobalListener() {
    if (!globalListenerActive) {
        globalUnsubscribe = webSocketClient.onOrderEvent(globalHandleOrderEvent);
        globalListenerActive = true;
    }
}

function cleanupGlobalListenerIfEmpty() {
    if (consumers.size === 0 && globalListenerActive && globalUnsubscribe) {
        globalUnsubscribe();
        globalUnsubscribe = null;
        globalListenerActive = false;
        toastsEnabled = false;
    }
}

export function useOrderEvents(options = {}) {
    const { onOrderUpdate, showToasts = true } = options;
    const isReady = ref(false);
    const isListenerActive = ref(false);
    
    // Create a consumer object for this instance
    const consumer = { onOrderUpdate };

    function setupOrderEventListener() {
        // Add this consumer to the set
        consumers.add(consumer);
        
        // Enable toasts if this consumer wants them (first one wins)
        if (showToasts && !toastsEnabled) {
            toastsEnabled = true;
        }
        
        // Ensure the global listener is active
        ensureGlobalListener();
        
        isListenerActive.value = true;
        isReady.value = true;
    }

    function cleanupOrderEventListener() {
        consumers.delete(consumer);
        isListenerActive.value = false;
        isReady.value = false;
        
        // If this was the toast-enabled consumer, check if any remaining consumer wants toasts
        // For simplicity, we'll disable toasts when the consumer count changes
        // The next consumer with showToasts=true will re-enable
        if (consumers.size === 0) {
            toastsEnabled = false;
        }
        
        cleanupGlobalListenerIfEmpty();
    }

    function ensureListenerActive() {
        if (!isListenerActive.value) {
            setupOrderEventListener();
        }
    }

    onMounted(() => {
        setupOrderEventListener();
    });

    onUnmounted(() => {
        cleanupOrderEventListener();
    });

    return {
        setupOrderEventListener,
        cleanupOrderEventListener,
        handleOrderEvent: globalHandleOrderEvent, // For testing
        isReady,
        isListenerActive,
        ensureListenerActive,
    };
}

// Test function - call window.testOrderEvents() from browser console
window.testOrderEvents = function() {
    const testEvent = {
        type: 'order_event',
        data: {
            id: 'TEST123',
            symbol: 'AAPL',
            status: 'filled',
            side: 'buy',
            quantity: 10,
            executed_quantity: 10,
            avg_fill_price: 150.00,
            account: 'TEST'
        },
        timestamp: Date.now()
    };
    webSocketClient.handleMessage(testEvent);
};
