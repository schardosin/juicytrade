package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"trade-backend-go/internal/models"
)

// =============================================================================
// Order Retrieval
// =============================================================================

// GetOrders gets orders with optional status filter.
// Uses GET /trader/v1/accounts/{accountHash}/orders with date range and status filter.
func (s *SchwabProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	params := url.Values{}

	// Map JuicyTrade status filter to Schwab status
	schwabStatus := mapOrderStatusFilter(status)
	if schwabStatus != "" {
		params.Set("status", schwabStatus)
	}

	// Default time range: 30 days back to now
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	params.Set("fromEnteredTime", from.Format("2006-01-02T15:04:05.000Z"))
	params.Set("toEnteredTime", now.Format("2006-01-02T15:04:05.000Z"))

	reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "/orders?" + params.Encode())

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetOrders failed: %w", err)
	}

	// Schwab returns an array of orders
	var ordersRaw []interface{}
	if err := json.Unmarshal(body, &ordersRaw); err != nil {
		// Try as single object wrapping an array
		var wrapper map[string]interface{}
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil, fmt.Errorf("schwab: failed to parse orders response: %w", err)
		}
		// Empty response or non-array
		return []*models.Order{}, nil
	}

	var orders []*models.Order
	for _, raw := range ordersRaw {
		data, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		order := transformSchwabOrder(data)
		if order != nil {
			orders = append(orders, order)
		}
	}

	s.logger.Debug("GetOrders completed", "status", status, "count", len(orders))
	return orders, nil
}

// mapOrderStatusFilter maps a JuicyTrade status filter to a Schwab status parameter.
func mapOrderStatusFilter(status string) string {
	switch strings.ToLower(status) {
	case "open", "working":
		return "WORKING"
	case "filled":
		return "FILLED"
	case "canceled", "cancelled":
		return "CANCELED"
	case "rejected":
		return "REJECTED"
	case "expired":
		return "EXPIRED"
	case "pending":
		return "QUEUED"
	case "all", "":
		return "" // No filter — get all
	default:
		return strings.ToUpper(status)
	}
}

// =============================================================================
// Order Cancellation
// =============================================================================

// CancelOrder cancels an existing order.
// Uses DELETE /trader/v1/accounts/{accountHash}/orders/{orderID}
func (s *SchwabProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "/orders/" + orderID)

	_, statusCode, err := s.doAuthenticatedRequest(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("schwab: CancelOrder failed for order %s: %w", orderID, err)
	}

	if statusCode == http.StatusOK || statusCode == http.StatusNoContent {
		s.logger.Info("order cancelled", "orderID", orderID)
		return true, nil
	}

	return false, fmt.Errorf("schwab: unexpected status %d when cancelling order %s", statusCode, orderID)
}

// =============================================================================
// Order Transformation
// =============================================================================

// transformSchwabOrder transforms a Schwab order response object into an Order model.
//
// Schwab order structure:
//
//	{
//	  "orderId": 123456789,
//	  "session": "NORMAL",
//	  "duration": "DAY",
//	  "orderType": "LIMIT",
//	  "price": 150.0,
//	  "status": "WORKING",
//	  "enteredTime": "2024-05-16T10:30:00+0000",
//	  "closeTime": "2024-05-16T16:00:00+0000",
//	  "quantity": 100,
//	  "filledQuantity": 0,
//	  "orderLegCollection": [{
//	    "instruction": "BUY",
//	    "quantity": 100,
//	    "instrument": {"symbol": "AAPL", "assetType": "EQUITY"}
//	  }]
//	}
func transformSchwabOrder(data map[string]interface{}) *models.Order {
	// Extract order ID — may be float64 or string from JSON
	orderID := extractOrderID(data)
	if orderID == "" {
		return nil
	}

	// Map status
	rawStatus, _ := extractString(data, "status")
	status := mapSchwabOrderStatus(rawStatus)

	// Map order type
	rawOrderType, _ := extractString(data, "orderType")
	orderType := strings.ToLower(rawOrderType)

	// Map time in force (duration)
	rawDuration, _ := extractString(data, "duration")
	timeInForce := mapSchwabDuration(rawDuration)

	// Extract price fields
	limitPrice := extractFloat64Ptr(data, "price")
	stopPrice := extractFloat64Ptr(data, "stopPrice")

	// Extract quantities
	qty, _ := extractFloat64(data, "quantity")
	filledQty, _ := extractFloat64(data, "filledQuantity")

	// Extract timestamps
	enteredTime, _ := extractString(data, "enteredTime")
	closeTime, _ := extractString(data, "closeTime")

	// Extract legs from orderLegCollection
	legs, primarySymbol, primaryAssetClass, primarySide := extractOrderLegs(data)

	order := &models.Order{
		ID:          orderID,
		Symbol:      primarySymbol,
		AssetClass:  primaryAssetClass,
		Side:        primarySide,
		OrderType:   orderType,
		Qty:         qty,
		FilledQty:   filledQty,
		LimitPrice:  limitPrice,
		StopPrice:   stopPrice,
		Status:      status,
		TimeInForce: timeInForce,
		SubmittedAt: enteredTime,
		Legs:        legs,
	}

	if closeTime != "" {
		order.FilledAt = &closeTime
	}

	return order
}

// extractOrderID extracts the order ID from a Schwab order, handling both
// numeric (float64) and string representations.
func extractOrderID(data map[string]interface{}) string {
	if v, ok := data["orderId"]; ok {
		switch id := v.(type) {
		case float64:
			return fmt.Sprintf("%.0f", id)
		case string:
			return id
		case json.Number:
			return id.String()
		}
	}
	return ""
}

// mapSchwabOrderStatus maps a Schwab order status to the normalized JuicyTrade status.
func mapSchwabOrderStatus(status string) string {
	switch strings.ToUpper(status) {
	case "WORKING", "AWAITING_PARENT_ORDER", "AWAITING_CONDITION", "AWAITING_MANUAL_REVIEW":
		return "open"
	case "FILLED":
		return "filled"
	case "CANCELED":
		return "canceled"
	case "REJECTED":
		return "rejected"
	case "EXPIRED":
		return "expired"
	case "PENDING_ACTIVATION", "QUEUED", "ACCEPTED":
		return "pending"
	case "REPLACED":
		return "replaced"
	default:
		if status == "" {
			return "unknown"
		}
		return strings.ToLower(status)
	}
}

// mapSchwabDuration maps a Schwab duration to the JuicyTrade time in force.
func mapSchwabDuration(duration string) string {
	switch strings.ToUpper(duration) {
	case "DAY":
		return "day"
	case "GOOD_TILL_CANCEL":
		return "gtc"
	case "FILL_OR_KILL":
		return "fok"
	default:
		if duration == "" {
			return "day"
		}
		return strings.ToLower(duration)
	}
}

// extractOrderLegs extracts legs from the orderLegCollection and returns
// the legs, primary symbol, asset class, and side (from the first leg).
func extractOrderLegs(data map[string]interface{}) ([]models.OrderLeg, string, string, string) {
	var legs []models.OrderLeg
	primarySymbol := ""
	primaryAssetClass := "us_equity"
	primarySide := "buy"

	legsRaw, ok := data["orderLegCollection"]
	if !ok {
		return legs, primarySymbol, primaryAssetClass, primarySide
	}
	legsArr, ok := legsRaw.([]interface{})
	if !ok {
		return legs, primarySymbol, primaryAssetClass, primarySide
	}

	for i, legRaw := range legsArr {
		legData, ok := legRaw.(map[string]interface{})
		if !ok {
			continue
		}

		instrument := extractMap(legData, "instrument")
		symbol := ""
		assetType := "EQUITY"
		if instrument != nil {
			symbol, _ = extractString(instrument, "symbol")
			assetType, _ = extractString(instrument, "assetType")
		}

		// Convert option symbols to OCC format
		if strings.ToUpper(assetType) == "OPTION" {
			symbol = convertSchwabOptionToOCC(symbol)
		}

		instruction, _ := extractString(legData, "instruction")
		legQty, _ := extractFloat64(legData, "quantity")
		side := mapSchwabInstruction(instruction)

		leg := models.OrderLeg{
			Symbol: symbol,
			Side:   side,
			Qty:    legQty,
		}
		legs = append(legs, leg)

		// Use first leg as primary
		if i == 0 {
			primarySymbol = symbol
			primarySide = side
			if strings.ToUpper(assetType) == "OPTION" {
				primaryAssetClass = "us_option"
			} else {
				primaryAssetClass = "us_equity"
			}
		}
	}

	return legs, primarySymbol, primaryAssetClass, primarySide
}

// mapSchwabInstruction maps a Schwab instruction to a JuicyTrade side.
func mapSchwabInstruction(instruction string) string {
	switch strings.ToUpper(instruction) {
	case "BUY", "BUY_TO_OPEN", "BUY_TO_COVER":
		return "buy"
	case "SELL", "SELL_TO_CLOSE", "SELL_SHORT", "SELL_TO_OPEN":
		return "sell"
	default:
		return strings.ToLower(instruction)
	}
}
