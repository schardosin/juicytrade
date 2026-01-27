package automation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/utils"
)

// RuntimeState represents the persisted state of all active automations
type RuntimeState struct {
	Version     string                          `json:"version"`
	UpdatedAt   time.Time                       `json:"updated_at"`
	Automations map[string]*PersistedAutomation `json:"automations"`
}

// PersistedAutomation represents the state of a single automation that needs to survive restarts
type PersistedAutomation struct {
	ConfigID      string                 `json:"config_id"`
	Status        types.AutomationStatus `json:"status"`
	StartedAt     time.Time              `json:"started_at"`
	TradedToday   bool                   `json:"traded_today"`
	LastTradeDate string                 `json:"last_trade_date,omitempty"`
	ErrorCount    int                    `json:"error_count"`
	Message       string                 `json:"message,omitempty"`
	CurrentOrder  *types.PlacedOrder     `json:"current_order,omitempty"`
	PlacedOrders  []types.PlacedOrder    `json:"placed_orders,omitempty"`
	// We don't persist logs - they can get large and aren't critical for recovery
}

// RuntimeStateStorage handles persistence of running automation state
type RuntimeStateStorage struct {
	filePath string
	mu       sync.RWMutex
}

// NewRuntimeStateStorage creates a new runtime state storage using GlobalPathManager
func NewRuntimeStateStorage() *RuntimeStateStorage {
	filePath := utils.GlobalPathManager.GetConfigFilePath("automation_runtime_state.json")

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	os.MkdirAll(dir, 0755)

	slog.Info("Automation runtime state storage initialized", "path", filePath)

	return &RuntimeStateStorage{
		filePath: filePath,
	}
}

// Save persists the current state of all active automations
func (r *RuntimeStateStorage) Save(automations map[string]*types.ActiveAutomation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := RuntimeState{
		Version:     "1.0",
		UpdatedAt:   time.Now(),
		Automations: make(map[string]*PersistedAutomation),
	}

	for id, active := range automations {
		// Only persist automations that are in a recoverable state
		// Don't persist completed/failed/cancelled - they're terminal
		if isRecoverableStatus(active.Status) {
			state.Automations[id] = &PersistedAutomation{
				ConfigID:      active.Config.ID,
				Status:        active.Status,
				StartedAt:     active.StartedAt,
				TradedToday:   active.TradedToday,
				LastTradeDate: active.LastTradeDate,
				ErrorCount:    active.ErrorCount,
				Message:       active.Message,
				CurrentOrder:  active.CurrentOrder,
				PlacedOrders:  active.PlacedOrders,
			}
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal runtime state: %w", err)
	}

	// Atomic write: write to temp file first, then rename
	tempFile := r.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write runtime state: %w", err)
	}

	if err := os.Rename(tempFile, r.filePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename runtime state file: %w", err)
	}

	return nil
}

// Load reads the persisted runtime state
func (r *RuntimeStateStorage) Load() (*RuntimeState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file - return empty state
			return &RuntimeState{
				Version:     "1.0",
				Automations: make(map[string]*PersistedAutomation),
			}, nil
		}
		return nil, fmt.Errorf("failed to read runtime state: %w", err)
	}

	var state RuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse runtime state: %w", err)
	}

	if state.Automations == nil {
		state.Automations = make(map[string]*PersistedAutomation)
	}

	return &state, nil
}

// Clear removes the persisted state file (used after successful restoration)
func (r *RuntimeStateStorage) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	err := os.Remove(r.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear runtime state: %w", err)
	}
	return nil
}

// Remove removes a single automation from persisted state
func (r *RuntimeStateStorage) Remove(id string) error {
	state, err := r.Load()
	if err != nil {
		return err
	}

	delete(state.Automations, id)

	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal runtime state: %w", err)
	}

	return os.WriteFile(r.filePath, data, 0644)
}

// isRecoverableStatus returns true if the automation status can be recovered after restart
func isRecoverableStatus(status types.AutomationStatus) bool {
	switch status {
	case types.StatusWaiting, types.StatusEvaluating, types.StatusTrading, types.StatusMonitoring:
		return true
	default:
		// StatusIdle, StatusCompleted, StatusFailed, StatusCancelled are not recoverable
		return false
	}
}

// RestoreAutomation creates an ActiveAutomation from persisted state
func RestoreAutomation(persisted *PersistedAutomation, config *types.AutomationConfig) *types.ActiveAutomation {
	active := &types.ActiveAutomation{
		Config:        config,
		Status:        persisted.Status,
		StartedAt:     persisted.StartedAt,
		TradedToday:   persisted.TradedToday,
		LastTradeDate: persisted.LastTradeDate,
		ErrorCount:    persisted.ErrorCount,
		Message:       persisted.Message,
		CurrentOrder:  persisted.CurrentOrder,
		PlacedOrders:  persisted.PlacedOrders,
		Logs:          make([]types.AutomationLog, 0),
	}

	// Add a log entry about the restoration
	active.AddLog("info", fmt.Sprintf("Automation restored after server restart (was in %s state)", persisted.Status))

	// If we were monitoring an order, we need to be careful
	if persisted.Status == types.StatusMonitoring && persisted.CurrentOrder != nil {
		active.AddLog("warn", fmt.Sprintf("Resuming order monitoring for order %s", persisted.CurrentOrder.OrderID))
	}

	// If we crashed while trading, reset to evaluating to re-evaluate conditions
	if persisted.Status == types.StatusTrading {
		active.Status = types.StatusEvaluating
		active.AddLog("warn", "Was in trading state during restart - reverting to evaluating")
		slog.Warn("Automation was in trading state during restart",
			"id", config.ID,
			"newStatus", types.StatusEvaluating,
		)
	}

	return active
}
