import { ref, onMounted, onUnmounted } from 'vue';
import webSocketClient from '../services/webSocketClient.js';
import notificationService from '../services/notificationService.js';
import { playOrderNotificationSound } from '../utils/notificationSound.js';

// Singleton state - shared across all useOrderEvents instances
let globalUnsubscribe = null;
let globalListenerActive = false;
const consumers = new Set();
let toastsEnabled = false; // Only one consumer should enable toasts

// Debounce state - wait for final status before showing toast
const pendingEvents = new Map(); // orderId -> { event, timeoutId }
const DEBOUNCE_MS = 150; // Wait 150ms for final status

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
    const status = eventData.status || eventData.Status;

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

    // Terminal statuses that should immediately override pending states
    const terminalStatuses = ['filled', 'canceled', 'cancelled', 'rejected', 'expired'];
    const isTerminalStatus = terminalStatuses.includes(status?.toLowerCase());
    
    const orderKey = String(orderId);
    
    if (isTerminalStatus) {
        // Terminal status: cancel any pending debounce and process immediately
        if (pendingEvents.has(orderKey)) {
            const pending = pendingEvents.get(orderKey);
            clearTimeout(pending.timeoutId);
            pendingEvents.delete(orderKey);
        }
        // Process terminal status immediately
        processOrderEvent(eventData);
    } else {
        // Non-terminal status (pending, open, new): debounce to allow terminal status to override
        // But keep the FIRST non-terminal event (pending) for the toast, not later ones (open)
        if (pendingEvents.has(orderKey)) {
            // Already have a pending event, just reset the timer but keep original event
            const pending = pendingEvents.get(orderKey);
            clearTimeout(pending.timeoutId);
            
            const timeoutId = setTimeout(() => {
                const stored = pendingEvents.get(orderKey);
                pendingEvents.delete(orderKey);
                if (stored) {
                    processOrderEvent(stored.event);
                }
            }, DEBOUNCE_MS);
            
            pending.timeoutId = timeoutId;
        } else {
            // First non-terminal event, store it
            const timeoutId = setTimeout(() => {
                const stored = pendingEvents.get(orderKey);
                pendingEvents.delete(orderKey);
                if (stored) {
                    processOrderEvent(stored.event);
                }
            }, DEBOUNCE_MS);
            
            pendingEvents.set(orderKey, { event: eventData, timeoutId });
        }
    }

    // Always notify consumers immediately for data refresh (but toast is debounced)
    consumers.forEach(consumer => {
        if (consumer.onOrderUpdate) {
            try {
                consumer.onOrderUpdate(eventData);
            } catch (e) {
                console.error('[ORDER-EVENT] Error in consumer callback:', e);
            }
        }
    });
}

function processOrderEvent(eventData) {
    const orderId = eventData.id || eventData.ID;
    const symbol = eventData.symbol || eventData.Symbol || 'N/A';
    const status = eventData.status || eventData.Status;
    const remainingQty = eventData.remaining_quantity || eventData.remainingQuantity || 0;
    const filledQty = eventData.exec_quantity || eventData.execQuantity || eventData.executed_quantity || 0;
    const avgFillPrice = eventData.avg_fill_price || eventData.avgFillPrice || 0;
    const orderType = eventData.type || eventData.order_type || '';

    // Build notification message based on status
    let messageText = '';
    let title = '';
    const price = eventData.price || eventData.Price || 0;
    let shouldShowToast = true;

    if (status?.toLowerCase() === 'filled') {
        title = symbol !== 'N/A' ? `Order Filled: ${symbol}` : `Order Filled`;
        if (filledQty > 0 && avgFillPrice > 0) {
            messageText = `Filled ${filledQty} @ $${avgFillPrice.toFixed(2)}`;
        } else if (price > 0) {
            messageText = `Filled @ $${price.toFixed(2)}`;
        } else {
            messageText = `Order filled`;
        }
    } else if (status?.toLowerCase() === 'canceled' || status?.toLowerCase() === 'cancelled') {
        title = `Order Canceled`;
        const orderInfo = orderId ? ` #${orderId}` : '';
        if (remainingQty > 0) {
            messageText = `Order${orderInfo} canceled (${remainingQty} remaining)`;
        } else {
            messageText = symbol !== 'N/A' 
                ? `Order ${symbol} was canceled` 
                : `Order${orderInfo} was canceled`;
        }
    } else if (status?.toLowerCase() === 'rejected') {
        title = `Order Rejected`;
        const orderInfo = orderId ? ` #${orderId}` : '';
        messageText = symbol !== 'N/A' 
            ? `Order ${symbol} was rejected` 
            : `Order${orderInfo} was rejected`;
    } else if (status?.toLowerCase() === 'pending') {
        title = `Order Submitted`;
        const orderInfo = orderId ? ` #${orderId}` : '';
        const priceInfo = price > 0 ? ` @ $${price.toFixed(2)}` : '';
        messageText = symbol !== 'N/A' 
            ? `Order ${symbol} submitted${priceInfo} (${orderType})`
            : `Order${orderInfo} submitted${priceInfo} (${orderType})`;
    } else if (status?.toLowerCase() === 'open' || status?.toLowerCase() === 'new') {
        // Order is now live in the market - skip notification since we already showed "submitted"
        shouldShowToast = false;
    } else if (status?.toLowerCase() === 'partially_filled') {
        title = `Order Partially Filled`;
        const orderInfo = orderId ? ` #${orderId}` : '';
        if (filledQty > 0 && avgFillPrice > 0) {
            messageText = `Filled ${filledQty} @ $${avgFillPrice.toFixed(2)} (${remainingQty} remaining)`;
        } else {
            messageText = symbol !== 'N/A' 
                ? `Order ${symbol} partially filled` 
                : `Order${orderInfo} partially filled`;
        }
    } else {
        shouldShowToast = false;
    }

    // Show toast only once (via global flag)
    if (shouldShowToast && toastsEnabled && messageText) {
        const notificationType = getStatusNotificationType(status);
        const notificationMethods = {
            'success': notificationService.showSuccess.bind(notificationService),
            'warning': notificationService.showWarning.bind(notificationService),
            'error': notificationService.showError.bind(notificationService),
            'info': notificationService.showInfo.bind(notificationService),
        };

        const notify = notificationMethods[notificationType] || notificationService.showInfo.bind(notificationService);
        notify(messageText, title, 5000);
        
        // Play notification sound for order events
        playOrderNotificationSound();
    }
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
