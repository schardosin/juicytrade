package models

import (
	"fmt"
	"sync"
)

// Normalized event types
const (
	NormalizedEventSubmitted       = "order_submitted"
	NormalizedEventFilled          = "order_filled"
	NormalizedEventPartiallyFilled = "order_partially_filled"
	NormalizedEventCancelled       = "order_cancelled"
)

// OrderState tracks the state of each order to detect transitions
type OrderState struct {
	PreviousStatus string
	HasNotified    bool // Whether we've already sent a normalized event
}

// OrderEventNormalizer normalizes order events based on status transitions
type OrderEventNormalizer struct {
	mu          sync.RWMutex
	orderStates map[string]*OrderState
}

// NewOrderEventNormalizer creates a new normalizer
func NewOrderEventNormalizer() *OrderEventNormalizer {
	return &OrderEventNormalizer{
		orderStates: make(map[string]*OrderState),
	}
}

// NormalizeEvent determines the normalized event type for an order event
// Returns the normalized event type and whether an event should be emitted
func (n *OrderEventNormalizer) NormalizeEvent(event *OrderEvent) (string, bool) {
	if event == nil {
		return "", false
	}

	orderID := event.GetIDAsString()
	if orderID == "" {
		return "", false
	}

	// Skip child orders (multi-leg orders)
	if event.ParentID != nil {
		return "", false
	}

	currentStatus := event.Status

	n.mu.Lock()
	defer n.mu.Unlock()

	// Get or create state for this order
	state, exists := n.orderStates[orderID]
	if !exists {
		state = &OrderState{}
		n.orderStates[orderID] = state
	}

	// Determine normalized event based on status transition
	var normalizedEvent string
	shouldNotify := false

	switch {
	// New order - first status we see
	case state.PreviousStatus == "":
		switch currentStatus {
		case "pending", "new", "received":
			normalizedEvent = NormalizedEventSubmitted
			shouldNotify = true
		case "filled":
			normalizedEvent = NormalizedEventFilled
			shouldNotify = true
		case "canceled", "cancelled":
			normalizedEvent = NormalizedEventCancelled
			shouldNotify = true
		case "rejected":
			normalizedEvent = NormalizedEventCancelled
			shouldNotify = true
		}

	// Transition from pending/open to terminal states
	case state.PreviousStatus == "pending" || state.PreviousStatus == "open":
		switch currentStatus {
		case "filled":
			// Check if partially or fully filled
			if event.RemainingQuantity > 0 {
				normalizedEvent = NormalizedEventPartiallyFilled
			} else {
				normalizedEvent = NormalizedEventFilled
			}
			shouldNotify = true
		case "canceled", "cancelled":
			normalizedEvent = NormalizedEventCancelled
			shouldNotify = true
		case "rejected":
			normalizedEvent = NormalizedEventCancelled
			shouldNotify = true
		}

	// Transition from partially_filled
	case state.PreviousStatus == "partially_filled":
		switch currentStatus {
		case "filled":
			normalizedEvent = NormalizedEventFilled
			shouldNotify = true
		case "canceled", "cancelled":
			// Don't notify - already notified as partially filled
			shouldNotify = false
		}

	// Already in terminal state - no more notifications
	default:
		shouldNotify = false
	}

	// If we've already notified AND this is not a new transition, don't notify again
	if state.HasNotified && !shouldNotify {
		state.PreviousStatus = currentStatus
		return "", false
	}

	// Update state
	if shouldNotify {
		state.HasNotified = true
	}
	state.PreviousStatus = currentStatus

	return normalizedEvent, shouldNotify
}

// ClearOrder removes an order from tracking (useful for testing or cleanup)
func (n *OrderEventNormalizer) ClearOrder(orderID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.orderStates, orderID)
}

// ClearAll removes all tracked orders
func (n *OrderEventNormalizer) ClearAll() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.orderStates = make(map[string]*OrderState)
}

// GetState returns the current state of an order (for debugging)
func (n *OrderEventNormalizer) GetState(orderID string) (string, bool, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	state, exists := n.orderStates[orderID]
	if !exists {
		return "", false, false
	}
	return state.PreviousStatus, state.HasNotified, true
}

// Global normalizer instance
var globalNormalizer *OrderEventNormalizer
var normalizerOnce sync.Once

// GetGlobalNormalizer returns the singleton normalizer instance
func GetGlobalNormalizer() *OrderEventNormalizer {
	normalizerOnce.Do(func() {
		globalNormalizer = NewOrderEventNormalizer()
	})
	return globalNormalizer
}

// String returns a string representation of the OrderEvent
func (e *OrderEvent) String() string {
	return fmt.Sprintf("OrderEvent{ID:%s Status:%s Normalized:%s Symbol:%s}",
		e.GetIDAsString(), e.Status, e.NormalizedEvent, e.Symbol)
}
