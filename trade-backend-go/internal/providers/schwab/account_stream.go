package schwab

import (
	"context"
	"fmt"
	"strings"
	"time"

	"trade-backend-go/internal/models"
)

// =============================================================================
// Account Event Streaming
//
// Schwab uses the same WebSocket connection for market data and account
// activity. Account events are received by subscribing to the ACCT_ACTIVITY
// service on the already-established streaming connection.
// =============================================================================

// StartAccountStream starts the account events stream for real-time order
// updates. Schwab shares the market data WebSocket for account activity,
// so this method ensures the WebSocket is connected, then subscribes to
// the ACCT_ACTIVITY service.
func (s *SchwabProvider) StartAccountStream(ctx context.Context) error {
	// Ensure the WebSocket connection is established
	if !s.IsConnected {
		if _, err := s.ConnectStreaming(ctx); err != nil {
			return fmt.Errorf("schwab: failed to connect streaming for account events: %w", err)
		}
	}

	// Subscribe to ACCT_ACTIVITY service
	req := schwabStreamRequest{
		Requests: []schwabStreamRequestItem{
			{
				RequestID:              s.nextRequestID(),
				Service:                "ACCT_ACTIVITY",
				Command:                "SUBS",
				SchwabClientCustomerID: s.streamCustomerID,
				SchwabClientCorrelID:   s.streamCorrelID,
				Parameters:             map[string]interface{}{},
			},
		},
	}

	if err := s.sendStreamMessage(req); err != nil {
		return fmt.Errorf("schwab: failed to subscribe to ACCT_ACTIVITY: %w", err)
	}

	s.acctStreamMu.Lock()
	s.acctStreamActive = true
	s.acctStreamMu.Unlock()

	s.logger.Info("account event streaming started")
	return nil
}

// StopAccountStream stops the account events stream by unsubscribing from
// the ACCT_ACTIVITY service. The underlying WebSocket connection remains
// open for market data streaming.
func (s *SchwabProvider) StopAccountStream() {
	s.acctStreamMu.Lock()
	wasActive := s.acctStreamActive
	s.acctStreamActive = false
	s.acctStreamMu.Unlock()

	if !wasActive {
		return
	}

	// Send UNSUBS for ACCT_ACTIVITY (best-effort)
	if s.IsConnected {
		req := schwabStreamRequest{
			Requests: []schwabStreamRequestItem{
				{
					RequestID:              s.nextRequestID(),
					Service:                "ACCT_ACTIVITY",
					Command:                "UNSUBS",
					SchwabClientCustomerID: s.streamCustomerID,
					SchwabClientCorrelID:   s.streamCorrelID,
				},
			},
		}
		_ = s.sendStreamMessage(req) // ignore error — best effort
	}

	s.logger.Info("account event streaming stopped")
}

// SetOrderEventCallback sets the callback function that will be invoked
// when account order events are received from the stream.
func (s *SchwabProvider) SetOrderEventCallback(callback func(*models.OrderEvent)) {
	s.acctStreamMu.Lock()
	defer s.acctStreamMu.Unlock()
	s.orderEventCallback = callback
}

// IsAccountStreamConnected returns true if both the WebSocket streaming
// connection is active and the ACCT_ACTIVITY subscription is active.
func (s *SchwabProvider) IsAccountStreamConnected() bool {
	s.acctStreamMu.RLock()
	defer s.acctStreamMu.RUnlock()
	return s.acctStreamActive && s.IsConnected
}

// =============================================================================
// Account Activity Processing
// =============================================================================

// processAccountActivity processes an ACCT_ACTIVITY streaming data message.
// It parses each content item into a models.OrderEvent, normalizes it via the
// global normalizer, and invokes the stored callback if appropriate.
func (s *SchwabProvider) processAccountActivity(data schwabStreamDataItem) {
	s.acctStreamMu.RLock()
	callback := s.orderEventCallback
	s.acctStreamMu.RUnlock()

	if callback == nil {
		s.logger.Debug("account activity received but no callback set",
			"items", len(data.Content))
		return
	}

	for _, item := range data.Content {
		orderEvent := s.parseAccountActivityItem(item)
		if orderEvent == nil {
			continue
		}

		// Normalize the event to detect meaningful state transitions
		normalizedEvent, shouldEmit := models.GetGlobalNormalizer().NormalizeEvent(orderEvent)
		if !shouldEmit {
			s.logger.Debug("account activity skipped (no state transition)",
				"order_id", orderEvent.GetIDAsString(), "status", orderEvent.Status)
			continue
		}

		orderEvent.NormalizedEvent = normalizedEvent

		s.logger.Info("account event dispatched",
			"order_id", orderEvent.GetIDAsString(),
			"status", orderEvent.Status,
			"normalized", normalizedEvent,
			"symbol", orderEvent.Symbol)

		callback(orderEvent)
	}
}

// parseAccountActivityItem parses a single ACCT_ACTIVITY content item from
// Schwab's streaming data format into a models.OrderEvent.
//
// Schwab ACCT_ACTIVITY content items contain fields like:
//   - "1": account number
//   - "2": message type (e.g., "OrderEntryRequest", "OrderFill", "UROUT")
//   - "3": message data (JSON string with order details)
//
// The exact format depends on the Schwab streaming API version. This parser
// handles both the structured object format and the indexed field format.
func (s *SchwabProvider) parseAccountActivityItem(item map[string]interface{}) *models.OrderEvent {
	event := &models.OrderEvent{
		Event:   "order_update",
		Account: s.accountHash,
	}

	// Schwab sends account activity in varying formats. Try to extract
	// order information from all known field locations.

	// Try structured format with named fields
	if orderID, ok := extractStringField(item, "orderId", "orderID", "OrderId", "ORDER_ID"); ok {
		event.ID = orderID
	}

	if status, ok := extractStringField(item, "status", "Status", "ORDER_STATUS"); ok {
		event.Status = mapSchwabOrderStatus(status)
	}

	if symbol, ok := extractStringField(item, "symbol", "Symbol", "SYMBOL"); ok {
		event.Symbol = symbol
	}

	if side, ok := extractStringField(item, "side", "Side", "instruction", "Instruction"); ok {
		event.Side = mapSchwabActivitySide(side)
	}

	if orderType, ok := extractStringField(item, "orderType", "OrderType", "ORDER_TYPE"); ok {
		event.Type = strings.ToLower(orderType)
	}

	// Extract numeric fields
	if qty, ok := extractFloatField(item, "quantity", "Quantity", "orderQty", "filledQuantity"); ok {
		event.Quantity = qty
	}

	if price, ok := extractFloatField(item, "price", "Price", "limitPrice"); ok {
		event.Price = price
	}

	if avgFill, ok := extractFloatField(item, "avgFillPrice", "averageFillPrice", "executionPrice"); ok {
		event.AvgFillPrice = avgFill
	}

	if execQty, ok := extractFloatField(item, "executedQuantity", "filledQuantity", "executionQuantity"); ok {
		event.ExecutedQuantity = execQty
	}

	// Try indexed field format (Schwab streaming uses numerical keys for some services)
	if event.ID == nil {
		if v, ok := item["key"]; ok {
			event.ID = v
		}
	}

	// Try to extract from the "1", "2", "3" indexed format
	if event.ID == nil {
		if msgData, ok := item["3"].(string); ok && msgData != "" {
			// Field "3" may contain order details as a message string
			s.logger.Debug("account activity raw message", "data", msgData)
		}
	}

	// Set timestamps
	now := time.Now().Format(time.RFC3339)
	if event.TransactionDate == "" {
		event.TransactionDate = now
	}
	if event.CreateDate == "" {
		event.CreateDate = now
	}

	// Skip events with no usable order information
	if event.ID == nil && event.Status == "" && event.Symbol == "" {
		s.logger.Debug("account activity item has no parseable order data")
		return nil
	}

	return event
}

// =============================================================================
// Account Activity Helpers
// =============================================================================

// extractStringField tries to find a string value in a map using multiple
// possible key names. Returns the value and whether it was found.
func extractStringField(item map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		if v, ok := item[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

// extractFloatField tries to find a float64 value in a map using multiple
// possible key names. Handles float64, int, and string-encoded numbers.
func extractFloatField(item map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if v, ok := item[key]; ok {
			switch val := v.(type) {
			case float64:
				return val, true
			case int:
				return float64(val), true
			case int64:
				return float64(val), true
			}
		}
	}
	return 0, false
}

// mapSchwabActivitySide maps Schwab order instruction strings from streaming
// activity data to a normalized buy/sell side.
func mapSchwabActivitySide(instruction string) string {
	upper := strings.ToUpper(instruction)
	switch {
	case strings.Contains(upper, "BUY"):
		return "buy"
	case strings.Contains(upper, "SELL"):
		return "sell"
	default:
		return strings.ToLower(instruction)
	}
}
