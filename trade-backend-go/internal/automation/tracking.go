package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trade-backend-go/internal/automation/types"
)

// TrackingStore manages order-to-automation mappings and position tracking
type TrackingStore struct {
	mu                    sync.RWMutex
	orders                map[string]*types.PlacedOrder        // orderID -> PlacedOrder
	positions             map[string]*types.AutomationPosition // positionKey -> AutomationPosition
	ordersByAutomation    map[string][]string                  // automationID -> []orderIDs
	positionsByAutomation map[string][]string                  // automationID -> []positionKeys
	dataDir               string
}

// NewTrackingStore creates a new tracking store
func NewTrackingStore(dataDir string) *TrackingStore {
	store := &TrackingStore{
		orders:                make(map[string]*types.PlacedOrder),
		positions:             make(map[string]*types.AutomationPosition),
		ordersByAutomation:    make(map[string][]string),
		positionsByAutomation: make(map[string][]string),
		dataDir:               dataDir,
	}

	// Load persisted data
	store.load()

	return store
}

// TrackOrder records an order placed by an automation
func (s *TrackingStore) TrackOrder(order *types.PlacedOrder) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.orders[order.OrderID] = order

	// Add to automation's order list
	if order.AutomationID != "" {
		s.ordersByAutomation[order.AutomationID] = append(
			s.ordersByAutomation[order.AutomationID],
			order.OrderID,
		)
	}

	s.persist()
}

// UpdateOrderStatus updates the status of a tracked order
func (s *TrackingStore) UpdateOrderStatus(orderID, status string, filledAt *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order, ok := s.orders[orderID]; ok {
		order.Status = status
		if filledAt != nil {
			order.FilledAt = filledAt
		}
		s.persist()
	}
}

// GetOrderByID retrieves a tracked order by its ID
func (s *TrackingStore) GetOrderByID(orderID string) *types.PlacedOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.orders[orderID]
}

// GetOrdersByAutomation retrieves all orders for an automation
func (s *TrackingStore) GetOrdersByAutomation(automationID string) []*types.PlacedOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.PlacedOrder
	if orderIDs, ok := s.ordersByAutomation[automationID]; ok {
		for _, orderID := range orderIDs {
			if order, ok := s.orders[orderID]; ok {
				result = append(result, order)
			}
		}
	}
	return result
}

// TrackPosition records a position created from a filled order
func (s *TrackingStore) TrackPosition(position *types.AutomationPosition) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use orderID as the position key for simplicity
	positionKey := position.OrderID
	s.positions[positionKey] = position

	// Add to automation's position list
	if position.AutomationID != "" {
		s.positionsByAutomation[position.AutomationID] = append(
			s.positionsByAutomation[position.AutomationID],
			positionKey,
		)
	}

	s.persist()
}

// GetPositionsByAutomation retrieves all positions for an automation
func (s *TrackingStore) GetPositionsByAutomation(automationID string) []*types.AutomationPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.AutomationPosition
	if positionKeys, ok := s.positionsByAutomation[automationID]; ok {
		for _, key := range positionKeys {
			if position, ok := s.positions[key]; ok {
				result = append(result, position)
			}
		}
	}
	return result
}

// GetAutomationIDForOrder returns the automation ID that placed a given order
func (s *TrackingStore) GetAutomationIDForOrder(orderID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if order, ok := s.orders[orderID]; ok {
		return order.AutomationID
	}
	return ""
}

// GetAllTrackedOrders returns all tracked orders
func (s *TrackingStore) GetAllTrackedOrders() []*types.PlacedOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.PlacedOrder, 0, len(s.orders))
	for _, order := range s.orders {
		result = append(result, order)
	}
	return result
}

// GetAllTrackedPositions returns all tracked positions
func (s *TrackingStore) GetAllTrackedPositions() []*types.AutomationPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.AutomationPosition, 0, len(s.positions))
	for _, position := range s.positions {
		result = append(result, position)
	}
	return result
}

// ClosePosition marks a position as closed
func (s *TrackingStore) ClosePosition(orderID string, closedAt time.Time, exitDebit, pnl float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if position, ok := s.positions[orderID]; ok {
		position.ClosedAt = &closedAt
		position.ExitDebit = &exitDebit
		position.PnL = &pnl
		position.Status = "closed"
		s.persist()
	}
}

// Persistence methods

type trackingData struct {
	Orders                map[string]*types.PlacedOrder        `json:"orders"`
	Positions             map[string]*types.AutomationPosition `json:"positions"`
	OrdersByAutomation    map[string][]string                  `json:"orders_by_automation"`
	PositionsByAutomation map[string][]string                  `json:"positions_by_automation"`
}

func (s *TrackingStore) getFilePath() string {
	return filepath.Join(s.dataDir, "automation_tracking.json")
}

func (s *TrackingStore) persist() {
	if s.dataDir == "" {
		return
	}

	data := trackingData{
		Orders:                s.orders,
		Positions:             s.positions,
		OrdersByAutomation:    s.ordersByAutomation,
		PositionsByAutomation: s.positionsByAutomation,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	// Ensure directory exists
	os.MkdirAll(s.dataDir, 0755)

	// Write to temp file first, then rename (atomic)
	tempPath := s.getFilePath() + ".tmp"
	if err := os.WriteFile(tempPath, bytes, 0644); err != nil {
		return
	}
	os.Rename(tempPath, s.getFilePath())
}

func (s *TrackingStore) load() {
	if s.dataDir == "" {
		return
	}

	bytes, err := os.ReadFile(s.getFilePath())
	if err != nil {
		return // File doesn't exist yet, that's OK
	}

	var data trackingData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return
	}

	if data.Orders != nil {
		s.orders = data.Orders
	}
	if data.Positions != nil {
		s.positions = data.Positions
	}
	if data.OrdersByAutomation != nil {
		s.ordersByAutomation = data.OrdersByAutomation
	}
	if data.PositionsByAutomation != nil {
		s.positionsByAutomation = data.PositionsByAutomation
	}
}

// CleanupOldData removes orders/positions older than the specified duration
func (s *TrackingStore) CleanupOldData(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	// Clean old orders (keep filled ones that became positions)
	for orderID, order := range s.orders {
		if order.PlacedAt.Before(cutoff) && order.Status != "filled" {
			delete(s.orders, orderID)
		}
	}

	// Clean closed positions older than cutoff
	for key, position := range s.positions {
		if position.Status == "closed" && position.ClosedAt != nil && position.ClosedAt.Before(cutoff) {
			delete(s.positions, key)
		}
	}

	s.persist()
}
