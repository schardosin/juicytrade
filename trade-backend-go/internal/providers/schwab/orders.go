package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// =============================================================================
// Order Placement Types
// =============================================================================

// schwabOrderRequest represents the JSON body for placing an order via the Schwab API.
type schwabOrderRequest struct {
	Session                  string           `json:"session"`
	Duration                 string           `json:"duration"`
	OrderType                string           `json:"orderType"`
	ComplexOrderStrategyType string           `json:"complexOrderStrategyType,omitempty"`
	Price                    string           `json:"price,omitempty"`
	StopPrice                string           `json:"stopPrice,omitempty"`
	OrderStrategyType        string           `json:"orderStrategyType"`
	OrderLegCollection       []schwabOrderLeg `json:"orderLegCollection"`
}

// schwabOrderLeg represents a single leg in a Schwab order.
type schwabOrderLeg struct {
	Instruction string           `json:"instruction"`
	Quantity    int              `json:"quantity"`
	Instrument  schwabInstrument `json:"instrument"`
}

// schwabInstrument represents an instrument in a Schwab order leg.
type schwabInstrument struct {
	Symbol    string `json:"symbol"`
	AssetType string `json:"assetType"` // "EQUITY" or "OPTION"
}

// =============================================================================
// Order Placement
// =============================================================================

// PlaceOrder places a single-leg trading order.
// Uses POST /trader/v1/accounts/{accountHash}/orders
func (s *SchwabProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	req, err := buildSchwabOrderRequest(orderData)
	if err != nil {
		return nil, fmt.Errorf("schwab: failed to build order request: %w", err)
	}

	return s.submitOrder(ctx, req)
}

// PlaceMultiLegOrder places a multi-leg trading order.
// Uses the same endpoint as PlaceOrder with multiple legs.
func (s *SchwabProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	req, err := buildSchwabMultiLegOrderRequest(orderData)
	if err != nil {
		return nil, fmt.Errorf("schwab: failed to build multi-leg order request: %w", err)
	}

	return s.submitOrder(ctx, req)
}

// submitOrder submits an order request to the Schwab API and parses the response.
func (s *SchwabProvider) submitOrder(ctx context.Context, req *schwabOrderRequest) (*models.Order, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("schwab: failed to marshal order: %w", err)
	}

	reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "/orders")

	body, statusCode, err := s.doAuthenticatedRequest(ctx, http.MethodPost, reqURL, jsonBody)
	if err != nil {
		return nil, fmt.Errorf("schwab: PlaceOrder failed: %w", err)
	}

	// Schwab returns 201 Created on success with Location header containing order ID
	if statusCode != http.StatusCreated && statusCode != http.StatusOK {
		return nil, fmt.Errorf("schwab: unexpected status %d placing order: %s", statusCode, string(body))
	}

	// Extract order ID from response body or construct from request
	orderID := "pending"
	// Try parsing response body for order ID
	if len(body) > 0 {
		var respData map[string]interface{}
		if err := json.Unmarshal(body, &respData); err == nil {
			if id := extractOrderID(respData); id != "" {
				orderID = id
			}
		}
	}

	// Build primary symbol from first leg
	primarySymbol := ""
	primarySide := "buy"
	primaryAssetClass := "us_equity"
	if len(req.OrderLegCollection) > 0 {
		leg := req.OrderLegCollection[0]
		primarySymbol = leg.Instrument.Symbol
		primarySide = mapSchwabInstruction(leg.Instruction)
		if leg.Instrument.AssetType == "OPTION" {
			primaryAssetClass = "us_option"
			primarySymbol = convertSchwabOptionToOCC(primarySymbol)
		}
	}

	order := &models.Order{
		ID:          orderID,
		Symbol:      primarySymbol,
		AssetClass:  primaryAssetClass,
		Side:        primarySide,
		OrderType:   strings.ToLower(req.OrderType),
		Qty:         float64(req.OrderLegCollection[0].Quantity),
		Status:      "pending",
		TimeInForce: mapSchwabDuration(req.Duration),
		SubmittedAt: time.Now().Format(time.RFC3339),
	}

	if req.Price != "" {
		if p, err := strconv.ParseFloat(req.Price, 64); err == nil {
			order.LimitPrice = &p
		}
	}
	if req.StopPrice != "" {
		if p, err := strconv.ParseFloat(req.StopPrice, 64); err == nil {
			order.StopPrice = &p
		}
	}

	s.logger.Info("order placed", "orderID", orderID, "symbol", primarySymbol, "side", primarySide)
	return order, nil
}

// =============================================================================
// Order Request Builders
// =============================================================================

// buildSchwabOrderRequest constructs a single-leg Schwab order request from generic order data.
func buildSchwabOrderRequest(orderData map[string]interface{}) (*schwabOrderRequest, error) {
	symbol, _ := orderData["symbol"].(string)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in order data")
	}

	side, _ := orderData["side"].(string)
	orderType, _ := orderData["type"].(string)
	if orderType == "" {
		orderType, _ = orderData["order_type"].(string)
	}
	duration, _ := orderData["time_in_force"].(string)
	if duration == "" {
		duration, _ = orderData["duration"].(string)
	}

	qty := extractOrderQty(orderData)
	if qty <= 0 {
		return nil, fmt.Errorf("missing or invalid quantity in order data")
	}

	// Determine asset type and instruction
	assetClass, _ := orderData["asset_class"].(string)
	isOption := strings.Contains(strings.ToLower(assetClass), "option") || isOptionSymbol(symbol)

	assetType := "EQUITY"
	schwabSymbol := symbol
	if isOption {
		assetType = "OPTION"
		schwabSymbol = convertOCCToSchwab(symbol)
	}

	instruction := mapSideToInstruction(side, isOption)

	req := &schwabOrderRequest{
		Session:           "NORMAL",
		Duration:          mapDurationToSchwab(duration),
		OrderType:         mapOrderTypeToSchwab(orderType),
		OrderStrategyType: "SINGLE",
		OrderLegCollection: []schwabOrderLeg{
			{
				Instruction: instruction,
				Quantity:    qty,
				Instrument: schwabInstrument{
					Symbol:    schwabSymbol,
					AssetType: assetType,
				},
			},
		},
	}

	// Schwab requires complexOrderStrategyType for option orders
	if isOption {
		req.ComplexOrderStrategyType = "NONE"
	}

	// Set price for limit orders
	if price, ok := orderData["price"].(float64); ok && price > 0 {
		req.Price = fmt.Sprintf("%.2f", price)
	} else if price, ok := orderData["limit_price"].(float64); ok && price > 0 {
		req.Price = fmt.Sprintf("%.2f", price)
	}

	// Set stop price for stop orders
	if stopPrice, ok := orderData["stop_price"].(float64); ok && stopPrice > 0 {
		req.StopPrice = fmt.Sprintf("%.2f", stopPrice)
	}

	return req, nil
}

// buildSchwabMultiLegOrderRequest constructs a multi-leg Schwab order request.
func buildSchwabMultiLegOrderRequest(orderData map[string]interface{}) (*schwabOrderRequest, error) {
	orderType, _ := orderData["type"].(string)
	if orderType == "" {
		orderType, _ = orderData["order_type"].(string)
	}
	duration, _ := orderData["time_in_force"].(string)
	if duration == "" {
		duration, _ = orderData["duration"].(string)
	}

	req := &schwabOrderRequest{
		Session:                  "NORMAL",
		Duration:                 mapDurationToSchwab(duration),
		OrderType:                mapOrderTypeToSchwab(orderType),
		ComplexOrderStrategyType: "CUSTOM",
		OrderStrategyType:        "SINGLE",
	}

	// Set price
	if price, ok := orderData["price"].(float64); ok && price > 0 {
		req.Price = fmt.Sprintf("%.2f", price)
	}

	// Build legs from orderData["legs"]
	legsRaw, ok := orderData["legs"]
	if !ok {
		return nil, fmt.Errorf("missing legs in multi-leg order data")
	}
	legsArr, ok := legsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid legs format in multi-leg order data")
	}

	for _, legRaw := range legsArr {
		legData, ok := legRaw.(map[string]interface{})
		if !ok {
			continue
		}

		symbol, _ := legData["symbol"].(string)
		side, _ := legData["side"].(string)
		legQty := extractOrderQty(legData)

		assetClass, _ := legData["asset_class"].(string)
		isOption := strings.Contains(strings.ToLower(assetClass), "option") || isOptionSymbol(symbol)

		assetType := "EQUITY"
		schwabSymbol := symbol
		if isOption {
			assetType = "OPTION"
			schwabSymbol = convertOCCToSchwab(symbol)
		}

		action, _ := legData["action"].(string)
		instruction := mapActionToInstruction(action, side, isOption)

		req.OrderLegCollection = append(req.OrderLegCollection, schwabOrderLeg{
			Instruction: instruction,
			Quantity:    legQty,
			Instrument: schwabInstrument{
				Symbol:    schwabSymbol,
				AssetType: assetType,
			},
		})
	}

	if len(req.OrderLegCollection) == 0 {
		return nil, fmt.Errorf("no valid legs in multi-leg order data")
	}

	return req, nil
}

// =============================================================================
// Order Mapping Helpers
// =============================================================================

// mapSideToInstruction maps a JuicyTrade side to a Schwab instruction.
func mapSideToInstruction(side string, isOption bool) string {
	switch strings.ToLower(side) {
	case "buy":
		if isOption {
			return "BUY_TO_OPEN"
		}
		return "BUY"
	case "sell":
		if isOption {
			return "SELL_TO_CLOSE"
		}
		return "SELL"
	default:
		return strings.ToUpper(side)
	}
}

// mapActionToInstruction maps an action string to a Schwab instruction,
// with optional side fallback.
func mapActionToInstruction(action, side string, isOption bool) string {
	switch strings.ToUpper(action) {
	case "BUY_TO_OPEN":
		return "BUY_TO_OPEN"
	case "BUY_TO_CLOSE":
		return "BUY_TO_CLOSE"
	case "SELL_TO_OPEN":
		return "SELL_TO_OPEN"
	case "SELL_TO_CLOSE":
		return "SELL_TO_CLOSE"
	default:
		return mapSideToInstruction(side, isOption)
	}
}

// mapDurationToSchwab maps a JuicyTrade time-in-force to a Schwab duration.
func mapDurationToSchwab(duration string) string {
	switch strings.ToLower(duration) {
	case "day", "":
		return "DAY"
	case "gtc":
		return "GOOD_TILL_CANCEL"
	case "fok":
		return "FILL_OR_KILL"
	default:
		return "DAY"
	}
}

// mapOrderTypeToSchwab maps a JuicyTrade order type to a Schwab order type.
func mapOrderTypeToSchwab(orderType string) string {
	switch strings.ToLower(orderType) {
	case "market", "":
		return "MARKET"
	case "limit":
		return "LIMIT"
	case "stop":
		return "STOP"
	case "stop_limit":
		return "STOP_LIMIT"
	case "net_debit":
		return "NET_DEBIT"
	case "net_credit":
		return "NET_CREDIT"
	default:
		return strings.ToUpper(orderType)
	}
}

// extractOrderQty extracts an integer quantity from order data.
func extractOrderQty(data map[string]interface{}) int {
	if v, ok := data["qty"].(float64); ok {
		return int(v)
	}
	if v, ok := data["quantity"].(float64); ok {
		return int(v)
	}
	if v, ok := data["qty"].(int); ok {
		return v
	}
	if v, ok := data["quantity"].(int); ok {
		return v
	}
	return 0
}
