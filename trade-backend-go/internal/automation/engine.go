package automation

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"trade-backend-go/internal/automation/indicators"
	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
)

// Engine manages automation instances
type Engine struct {
	providerManager   *providers.ProviderManager
	indicatorService  *indicators.Service
	storage           *Storage
	trackingStore     *TrackingStore
	runtimeState      *RuntimeStateStorage
	activeAutomations map[string]*types.ActiveAutomation
	mu                sync.RWMutex
	stopChannels      map[string]chan struct{}
	updateCallbacks   []func(string, *types.ActiveAutomation) // Callbacks for status updates
	callbackMu        sync.RWMutex
}

// NewEngine creates a new automation engine
func NewEngine(pm *providers.ProviderManager) (*Engine, error) {
	storage, err := NewStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	engine := &Engine{
		providerManager:   pm,
		indicatorService:  indicators.NewService(pm),
		storage:           storage,
		trackingStore:     NewTrackingStore(),
		runtimeState:      NewRuntimeStateStorage(),
		activeAutomations: make(map[string]*types.ActiveAutomation),
		stopChannels:      make(map[string]chan struct{}),
		updateCallbacks:   make([]func(string, *types.ActiveAutomation), 0),
	}

	// Restore any automations that were running before server restart
	if err := engine.restoreFromPersistentState(); err != nil {
		slog.Error("Failed to restore automation state", "error", err)
		// Don't fail startup, just log the error
	}

	return engine, nil
}

// RegisterUpdateCallback registers a callback for automation status updates
func (e *Engine) RegisterUpdateCallback(callback func(string, *types.ActiveAutomation)) {
	e.callbackMu.Lock()
	defer e.callbackMu.Unlock()
	e.updateCallbacks = append(e.updateCallbacks, callback)
}

// notifyUpdate notifies all registered callbacks of an automation update
// and persists the current state to disk
func (e *Engine) notifyUpdate(id string, automation *types.ActiveAutomation) {
	// Persist state to disk for crash recovery
	go e.persistState()

	e.callbackMu.RLock()
	defer e.callbackMu.RUnlock()
	for _, callback := range e.updateCallbacks {
		go callback(id, automation)
	}
}

// GetStorage returns the storage instance
func (e *Engine) GetStorage() *Storage {
	return e.storage
}

// GetTrackingStore returns the tracking store instance
func (e *Engine) GetTrackingStore() *TrackingStore {
	return e.trackingStore
}

// GetIndicatorService returns the indicator service
func (e *Engine) GetIndicatorService() *indicators.Service {
	return e.indicatorService
}

// restoreFromPersistentState restores automations that were running before server restart
func (e *Engine) restoreFromPersistentState() error {
	state, err := e.runtimeState.Load()
	if err != nil {
		return fmt.Errorf("failed to load runtime state: %w", err)
	}

	if len(state.Automations) == 0 {
		slog.Info("No automations to restore")
		return nil
	}

	slog.Info("Restoring automations from persistent state", "count", len(state.Automations))

	restored := 0
	for id, persisted := range state.Automations {
		// Load the config
		config, err := e.storage.Get(persisted.ConfigID)
		if err != nil {
			slog.Warn("Cannot restore automation - config not found",
				"id", id,
				"configId", persisted.ConfigID,
				"error", err,
			)
			continue
		}

		// Check if config is still enabled
		if !config.Enabled {
			slog.Info("Skipping restore - automation is disabled",
				"id", id,
				"name", config.Name,
			)
			continue
		}

		// Restore the automation
		active := RestoreAutomation(persisted, config)
		e.activeAutomations[id] = active
		e.stopChannels[id] = make(chan struct{})

		// Start the automation loop
		go e.runAutomation(id, e.stopChannels[id])

		slog.Info("Automation restored successfully",
			"id", id,
			"name", config.Name,
			"status", active.Status,
			"tradedToday", active.TradedToday,
		)
		restored++
	}

	slog.Info("Automation restoration complete",
		"restored", restored,
		"total", len(state.Automations),
	)

	return nil
}

// persistState saves the current state of all active automations
func (e *Engine) persistState() {
	e.mu.RLock()
	automations := make(map[string]*types.ActiveAutomation)
	for id, active := range e.activeAutomations {
		automations[id] = active
	}
	e.mu.RUnlock()

	if err := e.runtimeState.Save(automations); err != nil {
		slog.Error("Failed to persist automation state", "error", err)
	}
}

// Start starts an automation by ID
func (e *Engine) Start(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if already running
	if _, exists := e.activeAutomations[id]; exists {
		return fmt.Errorf("automation %s is already running", id)
	}

	// Load config
	config, err := e.storage.Get(id)
	if err != nil {
		return err
	}

	if !config.Enabled {
		return fmt.Errorf("automation %s is disabled", id)
	}

	// Create active automation
	active := &types.ActiveAutomation{
		Config:    config,
		Status:    types.StatusWaiting,
		StartedAt: time.Now(),
		Logs:      make([]types.AutomationLog, 0),
	}
	active.AddLog("info", "Automation started")

	e.activeAutomations[id] = active
	e.stopChannels[id] = make(chan struct{})

	// Start the automation loop
	go e.runAutomation(id, e.stopChannels[id])

	slog.Info("Automation started", "id", id, "name", config.Name)
	e.notifyUpdate(id, active)

	return nil
}

// Stop stops an automation by ID
func (e *Engine) Stop(id string) error {
	e.mu.Lock()

	active, exists := e.activeAutomations[id]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("automation %s is not running", id)
	}

	// Signal stop
	if stopChan, ok := e.stopChannels[id]; ok {
		close(stopChan)
		delete(e.stopChannels, id)
	}

	active.Status = types.StatusCancelled
	active.AddLog("info", "Automation stopped by user")

	// Remove from active automations before notifying
	// This ensures persistState won't include the stopped automation
	delete(e.activeAutomations, id)
	e.mu.Unlock()

	// Persist state to remove this automation from the saved state
	e.persistState()

	slog.Info("Automation stopped", "id", id)
	return nil
}

// GetStatus returns the status of an automation
func (e *Engine) GetStatus(id string) (*types.ActiveAutomation, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	active, exists := e.activeAutomations[id]
	if !exists {
		return nil, fmt.Errorf("automation %s is not running", id)
	}

	return active, nil
}

// GetAllStatus returns status of all running automations
func (e *Engine) GetAllStatus() map[string]*types.ActiveAutomation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*types.ActiveAutomation)
	for id, active := range e.activeAutomations {
		result[id] = active
	}
	return result
}

// IsRunning checks if an automation is running
func (e *Engine) IsRunning(id string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.activeAutomations[id]
	return exists
}

// ResetTradedToday resets the TradedToday flag for a daily automation
// This allows the automation to trade again on the same day
func (e *Engine) ResetTradedToday(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	active, exists := e.activeAutomations[id]
	if !exists {
		return fmt.Errorf("automation %s is not running", id)
	}

	// Only allow reset for daily automations
	if active.Config.Recurrence != types.RecurrenceDaily {
		return fmt.Errorf("reset is only available for daily automations")
	}

	if !active.TradedToday {
		return fmt.Errorf("automation has not traded today, nothing to reset")
	}

	active.TradedToday = false
	active.ErrorCount = 0
	active.Status = types.StatusWaiting
	active.Message = "Manual reset - ready to trade again"
	active.AddLog("info", "TradedToday manually reset by user - automation can trade again today")

	slog.Info("🔄 TradedToday manually reset", "id", id)
	e.notifyUpdate(id, active)

	return nil
}

// EvaluateIndicators evaluates indicators for a config without starting
// Uses the config ID for caching results
func (e *Engine) EvaluateIndicators(ctx context.Context, config *types.AutomationConfig) []types.IndicatorResult {
	return e.indicatorService.EvaluateAllIndicators(ctx, config.ID, config.Indicators)
}

// PreviewStrikes finds strikes for a given configuration without placing an order
// This allows users to preview what strikes would be selected for their config
func (e *Engine) PreviewStrikes(ctx context.Context, config *types.AutomationConfig) (*types.StrikeSelection, error) {
	return e.findStrikesForDelta(ctx, config)
}

// runAutomation is the main automation loop
func (e *Engine) runAutomation(id string, stopChan chan struct{}) {
	ticker := time.NewTicker(30 * time.Second) // Evaluate every 30 seconds
	defer ticker.Stop()

	slog.Info("🤖 Automation loop started", "id", id)

	// Run first evaluation immediately (don't wait 30 seconds)
	e.runAutomationTick(id, stopChan)

	for {
		select {
		case <-stopChan:
			slog.Info("🛑 Automation loop stopped via channel", "id", id)
			return
		case <-ticker.C:
			e.runAutomationTick(id, stopChan)
		}
	}
}

// runAutomationTick runs a single automation evaluation tick
func (e *Engine) runAutomationTick(id string, stopChan chan struct{}) {
	e.mu.RLock()
	active, exists := e.activeAutomations[id]
	e.mu.RUnlock()

	if !exists {
		slog.Warn("🤖 Automation not found in tick", "id", id)
		return
	}

	slog.Info("🤖 Automation tick",
		"id", id,
		"status", active.Status,
		"message", active.Message)

	// Check market hours
	if !e.isMarketHours() {
		if active.Status != types.StatusWaiting {
			e.mu.Lock()
			active.Status = types.StatusWaiting
			active.Message = "Waiting for market hours"
			active.AddLog("info", "Market closed - waiting for market hours")
			e.mu.Unlock()
			e.notifyUpdate(id, active)
		}
		slog.Info("🤖 Market closed, waiting", "id", id)
		return
	}

	// Process based on current status
	switch active.Status {
	case types.StatusWaiting:
		e.handleWaitingState(id, active, stopChan)
	case types.StatusEvaluating:
		e.handleEvaluatingState(id, active, stopChan)
	case types.StatusTrading:
		e.handleTradingState(id, active, stopChan)
	case types.StatusMonitoring:
		e.handleMonitoringState(id, active, stopChan)
	case types.StatusCompleted, types.StatusFailed, types.StatusCancelled:
		// Terminal states - will exit on next select
		slog.Info("🤖 Automation in terminal state", "id", id, "status", active.Status)
		return
	}
}

// handleWaitingState handles the waiting state - waiting for entry time
func (e *Engine) handleWaitingState(id string, active *types.ActiveAutomation, stopChan chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check for new trading day and reset TradedToday if needed
	e.checkAndResetForNewDay(active)

	// For daily recurrence: skip if already traded today
	if active.TradedToday {
		recurrence := active.Config.Recurrence
		if recurrence == "" {
			recurrence = types.RecurrenceOnce
		}
		if recurrence == types.RecurrenceDaily {
			e.mu.Lock()
			active.Message = fmt.Sprintf("Already traded today (%s). Waiting for next trading day.", active.LastTradeDate)
			e.mu.Unlock()
			slog.Info("🤖 Daily automation already traded today, waiting for next day",
				"id", id,
				"lastTradeDate", active.LastTradeDate)
			e.notifyUpdate(id, active)
			return
		}
	}

	// Log entry time check
	isEntryTime := e.isEntryTime(active.Config)
	slog.Info("🤖 Checking entry conditions",
		"id", id,
		"entryTime", active.Config.EntryTime,
		"isEntryTime", isEntryTime)

	// Evaluate indicators (use config ID for caching)
	results := e.indicatorService.EvaluateAllIndicators(ctx, id, active.Config.Indicators)

	e.mu.Lock()
	active.IndicatorResults = results
	active.AllIndicatorsPass = e.indicatorService.AllIndicatorsPass(results)
	active.LastEvaluation = time.Now()
	e.mu.Unlock()

	// Check if any indicators are stale
	hasStaleIndicators := false
	for _, result := range results {
		if result.Enabled && result.Stale {
			hasStaleIndicators = true
			break
		}
	}

	if hasStaleIndicators {
		slog.Warn("🤖 Indicators evaluated with stale data",
			"id", id,
			"allPass", active.AllIndicatorsPass,
			"results", results)
	} else {
		slog.Info("🤖 Indicators evaluated",
			"id", id,
			"allPass", active.AllIndicatorsPass,
			"results", results)
	}

	// Check if it's time to trade
	if isEntryTime && active.AllIndicatorsPass {
		e.mu.Lock()
		active.Status = types.StatusTrading
		active.AddLog("info", "Entry conditions met, starting trade execution")
		active.Message = "Entry conditions met - executing trade"
		e.mu.Unlock()
		slog.Info("🚀 Automation entering trading state", "id", id)

		// Immediately execute trading (don't wait for next tick)
		e.handleTradingState(id, active, stopChan)
		return
	} else if isEntryTime && !active.AllIndicatorsPass {
		e.mu.Lock()
		if hasStaleIndicators {
			active.AddLog("warn", "Entry time reached but some indicators have stale data - trade blocked")
			active.Message = "Entry time reached but indicators have stale data"
		} else {
			active.AddLog("warn", "Entry time reached but indicators not passing")
			active.Message = "Entry time reached but indicators not passing"
		}
		e.mu.Unlock()
		slog.Warn("🤖 Entry time but indicators failing",
			"id", id,
			"hasStaleIndicators", hasStaleIndicators,
			"results", results)
	} else if !isEntryTime {
		e.mu.Lock()
		active.Message = fmt.Sprintf("Waiting for entry time: %s", active.Config.EntryTime)
		e.mu.Unlock()
		slog.Info("🤖 Waiting for entry time",
			"id", id,
			"configuredTime", active.Config.EntryTime)
	}

	e.notifyUpdate(id, active)
}

// checkAndResetForNewDay checks if it's a new trading day and resets TradedToday
func (e *Engine) checkAndResetForNewDay(active *types.ActiveAutomation) {
	if !active.TradedToday || active.LastTradeDate == "" {
		return // Nothing to reset
	}

	ny, _ := time.LoadLocation("America/New_York")
	todayStr := time.Now().In(ny).Format("2006-01-02")

	// If today is different from last trade date, reset TradedToday
	if todayStr != active.LastTradeDate {
		e.mu.Lock()
		active.TradedToday = false
		active.AddLog("info", fmt.Sprintf("New trading day detected (%s). Resetting for new trades.", todayStr))
		slog.Info("🌅 New trading day - resetting TradedToday",
			"lastTradeDate", active.LastTradeDate,
			"today", todayStr)
		e.mu.Unlock()
	}
}

// handleEvaluatingState handles the evaluating state
func (e *Engine) handleEvaluatingState(id string, active *types.ActiveAutomation, stopChan chan struct{}) {
	// Similar to waiting but with more frequent checks
	e.handleWaitingState(id, active, stopChan)
}

// handleTradingState handles the trading state - placing orders
func (e *Engine) handleTradingState(id string, active *types.ActiveAutomation, stopChan chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	e.mu.Lock()
	active.AddLog("info", "Finding strikes for target delta")
	e.mu.Unlock()

	// Find strikes for target delta
	strikes, err := e.findStrikesForDelta(ctx, active.Config)
	if err != nil {
		e.mu.Lock()
		active.ErrorCount++
		active.AddLog("error", fmt.Sprintf("Failed to find strikes: %v", err))
		if active.ErrorCount >= 3 {
			// For daily automations, mark as traded today and wait for next day
			if active.Config.Recurrence == types.RecurrenceDaily {
				active.TradedToday = true
				active.Status = types.StatusWaiting
				active.Message = "Failed to find strikes - waiting for next trading day"
				active.AddLog("warn", "Max strike-finding attempts reached - will retry next trading day")
				slog.Info("🔄 Strike finding failed, waiting for next trading day",
					"id", id,
					"errorCount", active.ErrorCount,
				)
			} else {
				active.Status = types.StatusFailed
				active.Message = "Failed to find strikes after multiple attempts"
			}
		}
		e.mu.Unlock()
		e.notifyUpdate(id, active)
		return
	}

	// Calculate position size
	units := active.Config.TradeConfig.CalculateUnits()
	if units == 0 {
		e.mu.Lock()
		active.Status = types.StatusFailed
		active.Message = "Insufficient capital for minimum position size"
		active.AddLog("error", "Insufficient capital")
		e.mu.Unlock()
		e.notifyUpdate(id, active)
		return
	}

	// Place the order
	order, err := e.placeSpreadOrder(ctx, id, active.Config, strikes, units)
	if err != nil {
		e.mu.Lock()
		active.ErrorCount++
		active.AddLog("error", fmt.Sprintf("Failed to place order: %v", err))
		if active.ErrorCount >= 3 {
			// For daily automations, mark as traded today and wait for next day
			if active.Config.Recurrence == types.RecurrenceDaily {
				active.TradedToday = true
				active.Status = types.StatusWaiting
				active.Message = "Failed to place order - waiting for next trading day"
				active.AddLog("warn", "Max order placement attempts reached - will retry next trading day")
				slog.Info("🔄 Order placement failed, waiting for next trading day",
					"id", id,
					"errorCount", active.ErrorCount,
				)
			} else {
				active.Status = types.StatusFailed
				active.Message = "Failed to place order after multiple attempts"
			}
		}
		e.mu.Unlock()
		e.notifyUpdate(id, active)
		return
	}

	// Order placed successfully
	e.mu.Lock()
	active.CurrentOrder = order
	active.PlacedOrders = append(active.PlacedOrders, *order)
	active.Status = types.StatusMonitoring
	active.AddLog("info", fmt.Sprintf("Order placed: %s at %.2f", order.OrderID, order.LimitPrice))
	e.mu.Unlock()

	slog.Info("Order placed", "id", id, "orderID", order.OrderID, "price", order.LimitPrice)
	e.notifyUpdate(id, active)
}

// handleMonitoringState handles the monitoring state - watching for fills
func (e *Engine) handleMonitoringState(id string, active *types.ActiveAutomation, stopChan chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if active.CurrentOrder == nil {
		e.mu.Lock()
		active.Status = types.StatusFailed
		active.Message = "No order to monitor"
		e.mu.Unlock()
		e.notifyUpdate(id, active)
		return
	}

	// Check order status
	orderStatus, err := e.checkOrderStatus(ctx, active.CurrentOrder.OrderID)
	if err != nil {
		e.mu.Lock()
		active.AddLog("warn", fmt.Sprintf("Failed to check order status: %v", err))
		e.mu.Unlock()
		return
	}

	switch orderStatus {
	case "filled":
		now := time.Now()
		ny, _ := time.LoadLocation("America/New_York")
		todayStr := now.In(ny).Format("2006-01-02")

		e.mu.Lock()
		active.CurrentOrder.Status = "filled"
		active.CurrentOrder.FilledAt = &now

		// Update order status in tracking store
		e.trackingStore.UpdateOrderStatus(active.CurrentOrder.OrderID, "filled", &now)

		// Create a position record for the filled order
		position := &types.AutomationPosition{
			AutomationID: id,
			ConfigName:   active.Config.Name,
			OrderID:      active.CurrentOrder.OrderID,
			Symbol:       active.Config.Symbol,
			Strategy:     active.Config.TradeConfig.Strategy,
			Legs:         active.CurrentOrder.Legs,
			OpenedAt:     now,
			EntryCredit:  active.CurrentOrder.LimitPrice,
			Status:       "open",
		}
		e.trackingStore.TrackPosition(position)

		// Check recurrence mode to determine next state
		recurrence := active.Config.Recurrence
		if recurrence == "" {
			recurrence = types.RecurrenceOnce // Default to "once"
		}

		if recurrence == types.RecurrenceDaily {
			// Daily recurrence: reset to waiting state for next trading day
			active.Status = types.StatusWaiting
			active.TradedToday = true
			active.LastTradeDate = todayStr
			active.CurrentOrder = nil // Clear current order for next day
			active.ErrorCount = 0     // Reset error count
			active.Message = fmt.Sprintf("Order filled! Waiting for next trading day (traded today: %s)", todayStr)
			active.AddLog("info", fmt.Sprintf("Order filled! Daily recurrence - will reset for next trading day. Trade date: %s", todayStr))
			slog.Info("Automation order filled - daily recurrence, waiting for next day",
				"id", id,
				"orderID", active.CurrentOrder,
				"lastTradeDate", todayStr)
		} else {
			// Once mode: mark as completed (terminal state)
			active.Status = types.StatusCompleted
			active.Message = "Order filled successfully"
			active.AddLog("info", "Order filled!")
			slog.Info("Automation completed - order filled", "id", id)
		}
		e.mu.Unlock()

		e.notifyUpdate(id, active)
		return

	case "cancelled", "rejected":
		now := time.Now()
		ny, _ := time.LoadLocation("America/New_York")
		todayStr := now.In(ny).Format("2006-01-02")

		e.mu.Lock()
		active.CurrentOrder.Status = orderStatus
		active.CurrentOrder.CancelledAt = &now

		// Update order status in tracking store
		e.trackingStore.UpdateOrderStatus(active.CurrentOrder.OrderID, orderStatus, nil)

		// Check recurrence mode to determine behavior after failure
		recurrence := active.Config.Recurrence
		if recurrence == "" {
			recurrence = types.RecurrenceOnce
		}

		if recurrence == types.RecurrenceDaily {
			// Daily recurrence: reset to waiting state to retry
			// This allows the automation to try again on the next evaluation tick
			active.Status = types.StatusWaiting
			active.CurrentOrder = nil
			active.ErrorCount++
			active.Message = fmt.Sprintf("Order %s by broker - will retry (attempt %d)", orderStatus, active.ErrorCount)
			active.AddLog("warn", fmt.Sprintf("Order %s by broker - resetting to waiting state for retry", orderStatus))
			slog.Warn("Daily automation order failed - will retry",
				"id", id,
				"orderStatus", orderStatus,
				"errorCount", active.ErrorCount,
				"date", todayStr)

			// If too many failures today, mark as failed for today but allow retry next day
			if active.ErrorCount >= 5 {
				active.TradedToday = true // Prevent more attempts today
				active.LastTradeDate = todayStr
				active.Message = fmt.Sprintf("Too many failures today (%d) - waiting for next trading day", active.ErrorCount)
				active.AddLog("error", fmt.Sprintf("Max daily failures reached (%d) - will retry next trading day", active.ErrorCount))
				slog.Error("Daily automation max failures reached for today",
					"id", id,
					"errorCount", active.ErrorCount,
					"date", todayStr)
			}
		} else {
			// Once mode: mark as failed (terminal state)
			active.Status = types.StatusFailed
			active.Message = fmt.Sprintf("Order %s", orderStatus)
			active.AddLog("error", fmt.Sprintf("Order %s", orderStatus))
		}
		e.mu.Unlock()

		e.notifyUpdate(id, active)
		return

	case "open", "pending":
		// Check if we should adjust
		e.handleOrderAdjustment(ctx, id, active)
	}
}

// handleOrderAdjustment handles price reduction or delta adjustment
func (e *Engine) handleOrderAdjustment(ctx context.Context, id string, active *types.ActiveAutomation) {
	if active.CurrentOrder == nil {
		return
	}

	config := active.Config.TradeConfig

	// Check if enough time has passed for adjustment
	timeSinceOrder := time.Since(active.CurrentOrder.PlacedAt)
	attemptInterval := time.Duration(config.AttemptInterval) * time.Second

	if timeSinceOrder >= attemptInterval {
		// Check if we've exceeded max attempts
		if active.CurrentOrder.AttemptNumber >= config.MaxAttempts {
			ny, _ := time.LoadLocation("America/New_York")
			todayStr := time.Now().In(ny).Format("2006-01-02")

			// Check recurrence mode
			recurrence := active.Config.Recurrence
			if recurrence == "" {
				recurrence = types.RecurrenceOnce
			}

			e.mu.Lock()
			if recurrence == types.RecurrenceDaily {
				// Daily recurrence: mark as done for today, will retry next day
				active.Status = types.StatusWaiting
				active.TradedToday = true // Prevent more attempts today
				active.LastTradeDate = todayStr
				active.CurrentOrder = nil
				active.Message = fmt.Sprintf("Max attempts reached for today - waiting for next trading day")
				active.AddLog("warn", fmt.Sprintf("Max price ladder attempts (%d) reached - will retry next trading day", config.MaxAttempts))
				slog.Warn("Daily automation max attempts reached - will retry next day",
					"id", id,
					"maxAttempts", config.MaxAttempts,
					"date", todayStr)
			} else {
				// Once mode: terminal failure
				active.Status = types.StatusFailed
				active.Message = "Max order attempts reached"
				active.AddLog("error", "Max attempts reached, giving up")
			}
			e.mu.Unlock()
			e.notifyUpdate(id, active)
			return
		}

		// Check for delta drift if DeltaDriftLimit is configured
		if config.DeltaDriftLimit > 0 {
			driftDetected, newStrikes := e.checkDeltaDrift(ctx, id, active)
			if driftDetected && newStrikes != nil {
				// Delta has drifted - cancel current order and place new one with updated strikes
				if err := e.cancelOrder(ctx, active.CurrentOrder.OrderID); err != nil {
					e.mu.Lock()
					active.AddLog("warn", fmt.Sprintf("Failed to cancel order for delta drift: %v", err))
					e.mu.Unlock()
					return
				}

				// Calculate units (same as original)
				units := config.CalculateUnits()
				if units == 0 {
					e.mu.Lock()
					active.Status = types.StatusFailed
					active.Message = "Insufficient capital for position size"
					active.AddLog("error", "Insufficient capital after delta drift")
					e.mu.Unlock()
					e.notifyUpdate(id, active)
					return
				}

				// Place new order with updated strikes
				newOrder, err := e.placeSpreadOrder(ctx, id, active.Config, newStrikes, units)
				if err != nil {
					e.mu.Lock()
					active.AddLog("error", fmt.Sprintf("Failed to place order after delta drift: %v", err))
					e.mu.Unlock()
					return
				}

				e.mu.Lock()
				active.CurrentOrder = newOrder
				active.PlacedOrders = append(active.PlacedOrders, *newOrder)
				active.AddLog("info", fmt.Sprintf("Order replaced due to delta drift: new delta %.4f at strike %.0f, price %.2f",
					newStrikes.ShortDelta, newStrikes.ShortStrike, newOrder.LimitPrice))
				e.mu.Unlock()

				slog.Info("Order replaced due to delta drift",
					"id", id,
					"oldDelta", active.CurrentOrder.ActualDelta,
					"newDelta", newStrikes.ShortDelta,
					"newStrike", newStrikes.ShortStrike)
				e.notifyUpdate(id, active)
				return
			}
		}

		// No delta drift (or not configured) - proceed with price reduction
		newPrice := active.CurrentOrder.LimitPrice - config.PriceLadderStep
		if newPrice <= 0 {
			e.mu.Lock()
			active.Status = types.StatusFailed
			active.Message = "Price reduced to zero"
			active.AddLog("error", "Price ladder exhausted")
			e.mu.Unlock()
			e.notifyUpdate(id, active)
			return
		}

		// Check minimum credit threshold
		if config.MinCredit > 0 && newPrice < config.MinCredit {
			e.mu.Lock()
			active.Status = types.StatusFailed
			active.Message = fmt.Sprintf("Price %.2f below minimum credit %.2f", newPrice, config.MinCredit)
			active.AddLog("error", fmt.Sprintf("Price ladder reached minimum credit threshold: %.2f < %.2f", newPrice, config.MinCredit))
			e.mu.Unlock()
			e.notifyUpdate(id, active)
			return
		}

		// Cancel current order
		if err := e.cancelOrder(ctx, active.CurrentOrder.OrderID); err != nil {
			e.mu.Lock()
			active.AddLog("warn", fmt.Sprintf("Failed to cancel order: %v", err))
			e.mu.Unlock()
			return
		}

		// Place new order with reduced price
		newOrder, err := e.replaceOrderWithNewPrice(ctx, id, active, newPrice)
		if err != nil {
			e.mu.Lock()
			active.AddLog("error", fmt.Sprintf("Failed to place replacement order: %v", err))
			e.mu.Unlock()
			return
		}

		e.mu.Lock()
		active.CurrentOrder = newOrder
		active.PlacedOrders = append(active.PlacedOrders, *newOrder)
		active.AddLog("info", fmt.Sprintf("Order replaced at new price: %.2f (attempt %d)", newPrice, newOrder.AttemptNumber))
		e.mu.Unlock()

		slog.Info("Order replaced with lower price", "id", id, "newPrice", newPrice, "attempt", newOrder.AttemptNumber)
		e.notifyUpdate(id, active)
	}
}

// checkDeltaDrift checks if the current order's delta has drifted beyond the limit
// Returns (driftDetected, newStrikes) - newStrikes is nil if no drift or error getting new strikes
func (e *Engine) checkDeltaDrift(ctx context.Context, id string, active *types.ActiveAutomation) (bool, *types.StrikeSelection) {
	if active.CurrentOrder == nil || len(active.CurrentOrder.Legs) == 0 {
		return false, nil
	}

	config := active.Config.TradeConfig
	targetDelta := config.TargetDelta
	driftLimit := config.DeltaDriftLimit

	// Get the short leg symbol (first leg in a credit spread is the short)
	shortLegSymbol := ""
	for _, leg := range active.CurrentOrder.Legs {
		if leg.Side == "sell_to_open" {
			shortLegSymbol = leg.Symbol
			break
		}
	}

	if shortLegSymbol == "" {
		slog.Warn("Could not find short leg symbol for delta drift check", "id", id)
		return false, nil
	}

	// Get current Greeks for the short leg
	greeks, err := e.providerManager.GetOptionsGreeksBatch(ctx, []string{shortLegSymbol})
	if err != nil {
		slog.Warn("Failed to get Greeks for delta drift check", "id", id, "error", err)
		return false, nil
	}

	greeksData, ok := greeks[shortLegSymbol]
	if !ok || greeksData == nil {
		slog.Warn("No Greeks data returned for short leg", "id", id, "symbol", shortLegSymbol)
		return false, nil
	}

	// Extract current delta
	var currentDelta float64
	switch d := greeksData["delta"].(type) {
	case float64:
		currentDelta = d
	case *float64:
		if d != nil {
			currentDelta = *d
		}
	default:
		slog.Warn("Could not extract delta from Greeks", "id", id, "greeks", greeksData)
		return false, nil
	}

	// Make delta absolute for comparison
	if currentDelta < 0 {
		currentDelta = -currentDelta
	}

	// Check if delta has drifted beyond the limit
	deltaDrift := currentDelta - targetDelta
	if deltaDrift < 0 {
		deltaDrift = -deltaDrift
	}

	slog.Info("Delta drift check",
		"id", id,
		"targetDelta", targetDelta,
		"currentDelta", currentDelta,
		"drift", deltaDrift,
		"limit", driftLimit)

	if deltaDrift > driftLimit {
		slog.Info("Delta drift detected, finding new strikes",
			"id", id,
			"drift", deltaDrift,
			"limit", driftLimit)

		// Find new strikes with the target delta
		newStrikes, err := e.findStrikesForDelta(ctx, active.Config)
		if err != nil {
			slog.Warn("Failed to find new strikes after delta drift", "id", id, "error", err)
			e.mu.Lock()
			active.AddLog("warn", fmt.Sprintf("Delta drifted (%.4f) but failed to find new strikes: %v", deltaDrift, err))
			e.mu.Unlock()
			return true, nil
		}

		return true, newStrikes
	}

	return false, nil
}

// findStrikesForDelta finds appropriate strikes for the target delta
func (e *Engine) findStrikesForDelta(ctx context.Context, config *types.AutomationConfig) (*types.StrikeSelection, error) {
	// Get expiration dates
	expirations, err := e.providerManager.GetExpirationDates(ctx, config.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration dates: %w", err)
	}

	if len(expirations) == 0 {
		return nil, fmt.Errorf("no expiration dates available for %s", config.Symbol)
	}

	// Determine target expiration based on ExpirationMode
	var expiry string
	expirationMode := config.TradeConfig.ExpirationMode
	if expirationMode == "" {
		expirationMode = "0dte" // Default to 0DTE
	}

	switch expirationMode {
	case "custom":
		// Use custom expiration date if provided
		if config.TradeConfig.CustomExpiration != "" {
			expiry = config.TradeConfig.CustomExpiration
		} else {
			return nil, fmt.Errorf("custom expiration mode selected but no date provided")
		}
	case "0dte", "1dte", "2dte":
		// Calculate target date based on DTE
		dteOffset := 0
		switch expirationMode {
		case "1dte":
			dteOffset = 1
		case "2dte":
			dteOffset = 2
		}

		targetDate := time.Now().AddDate(0, 0, dteOffset).Format("2006-01-02")

		// Find the expiration matching or closest to target date
		found := false
		for _, exp := range expirations {
			expDate, ok := exp["date"].(string)
			if !ok {
				if expDate, ok = exp["expiration_date"].(string); !ok {
					if expDate, ok = exp["expirationDate"].(string); !ok {
						continue
					}
				}
			}
			if expDate >= targetDate {
				expiry = expDate
				found = true
				break
			}
		}
		if !found && len(expirations) > 0 {
			// Fall back to first available expiration
			if exp, ok := expirations[0]["date"].(string); ok {
				expiry = exp
			} else if exp, ok := expirations[0]["expiration_date"].(string); ok {
				expiry = exp
			} else if exp, ok := expirations[0]["expirationDate"].(string); ok {
				expiry = exp
			}
		}
	default:
		// Default to first expiration (0DTE)
		if exp, ok := expirations[0]["date"].(string); ok {
			expiry = exp
		} else if exp, ok := expirations[0]["expiration_date"].(string); ok {
			expiry = exp
		} else if exp, ok := expirations[0]["expirationDate"].(string); ok {
			expiry = exp
		} else {
			return nil, fmt.Errorf("could not extract expiration date from response")
		}
	}

	if expiry == "" {
		return nil, fmt.Errorf("could not determine expiration date for mode: %s", expirationMode)
	}

	// Get current underlying price for ATM reference
	quotes, err := e.providerManager.GetStockQuotes(ctx, []string{config.Symbol})
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying price: %w", err)
	}

	var underlyingPrice *float64
	if quote, ok := quotes[config.Symbol]; ok {
		// Try Last first, then fall back to mid of Bid/Ask
		if quote.Last != nil && *quote.Last > 0 {
			underlyingPrice = quote.Last
			slog.Info("Got underlying price from Last", "symbol", config.Symbol, "price", *underlyingPrice)
		} else if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
			mid := (*quote.Bid + *quote.Ask) / 2
			underlyingPrice = &mid
			slog.Info("Got underlying price from Bid/Ask mid", "symbol", config.Symbol, "bid", *quote.Bid, "ask", *quote.Ask, "mid", mid)
		} else if quote.Bid != nil && *quote.Bid > 0 {
			underlyingPrice = quote.Bid
			slog.Info("Got underlying price from Bid only", "symbol", config.Symbol, "price", *underlyingPrice)
		} else if quote.Ask != nil && *quote.Ask > 0 {
			underlyingPrice = quote.Ask
			slog.Info("Got underlying price from Ask only", "symbol", config.Symbol, "price", *underlyingPrice)
		} else {
			slog.Warn("Quote received but no valid price data",
				"symbol", config.Symbol,
				"hasLast", quote.Last != nil,
				"lastValue", quote.Last,
				"hasBid", quote.Bid != nil,
				"bidValue", quote.Bid,
				"hasAsk", quote.Ask != nil,
				"askValue", quote.Ask)
		}
	} else {
		slog.Warn("No quote returned for symbol", "symbol", config.Symbol, "quotesReturned", len(quotes))
	}

	// FALLBACK: For indices (NDX, SPX, VIX, RUT), streaming quotes often don't work
	// Use historical bars API as a fallback (same approach as GetVIXValue)
	if underlyingPrice == nil {
		indexSymbols := map[string]bool{"NDX": true, "SPX": true, "VIX": true, "RUT": true}
		if indexSymbols[config.Symbol] {
			slog.Info("Streaming quote failed for index, trying historical bars fallback", "symbol", config.Symbol)
			bars, err := e.providerManager.GetHistoricalBars(ctx, config.Symbol, "D", nil, nil, 1)
			if err == nil && len(bars) > 0 {
				// Try to get close price from the most recent bar
				if closeVal, ok := bars[0]["close"]; ok {
					if closeFloat, ok := closeVal.(float64); ok && closeFloat > 0 {
						underlyingPrice = &closeFloat
						slog.Info("Got underlying price from historical bars", "symbol", config.Symbol, "price", closeFloat)
					}
				}
			} else {
				slog.Warn("Historical bars fallback also failed", "symbol", config.Symbol, "error", err)
			}
		}
	}

	// CRITICAL: If we couldn't get the underlying price, we CANNOT proceed
	// as we'll select completely wrong strikes
	if underlyingPrice == nil {
		return nil, fmt.Errorf("failed to get underlying price for %s - cannot determine ATM strike. Check if symbol is correct and market is open", config.Symbol)
	}

	// Get options chain (basic structure first)
	// Parameters: ctx, symbol, expiry, underlyingPrice, atmRange, includeGreeks, strikesOnly
	// Adjust atmRange based on target delta - lower delta targets need more OTM strikes
	atmRange := 200
	if config.TradeConfig.TargetDelta <= 0.10 {
		atmRange = 300 // Need many more strikes for low delta targets
	}
	chain, err := e.providerManager.GetOptionsChainSmart(ctx, config.Symbol, expiry, underlyingPrice, atmRange, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get options chain: %w", err)
	}

	if len(chain) == 0 {
		return nil, fmt.Errorf("no options available for %s expiry %s", config.Symbol, expiry)
	}

	// Determine option type based on strategy
	optionType := "put"
	if config.TradeConfig.Strategy == types.StrategyCallSpread {
		optionType = "call"
	}

	// Track execution time for performance monitoring
	selectionStartTime := time.Now()

	// Filter to matching option type and sort by strike price
	type optionInfo struct {
		symbol string
		strike float64
		index  int
	}
	var allOptions []*optionInfo

	for i, opt := range chain {
		if opt.Type != optionType {
			continue
		}
		allOptions = append(allOptions, &optionInfo{
			symbol: opt.Symbol,
			strike: opt.StrikePrice,
			index:  i,
		})
	}

	if len(allOptions) == 0 {
		return nil, fmt.Errorf("no %s options found for %s expiry %s (total options: %d)", optionType, config.Symbol, expiry, len(chain))
	}

	// Sort by strike price (ascending)
	sort.Slice(allOptions, func(i, j int) bool {
		return allOptions[i].strike < allOptions[j].strike
	})

	// Find ATM index (closest to underlying price)
	atmIndex := 0
	if underlyingPrice != nil {
		minDist := 999999.0
		for i, opt := range allOptions {
			dist := opt.strike - *underlyingPrice
			if dist < 0 {
				dist = -dist
			}
			if dist < minDist {
				minDist = dist
				atmIndex = i
			}
		}
	} else {
		// Default to middle if no underlying price
		atmIndex = len(allOptions) / 2
	}

	slog.Info("Starting windowed strike selection",
		"totalOptions", len(allOptions),
		"atmIndex", atmIndex,
		"atmStrike", allOptions[atmIndex].strike,
		"underlyingPrice", underlyingPrice,
		"targetDelta", config.TradeConfig.TargetDelta)

	// Debug: Log all available strikes for puts/calls near ATM
	if len(allOptions) > 0 {
		startLog := atmIndex - 50
		if startLog < 0 {
			startLog = 0
		}
		endLog := atmIndex + 10
		if endLog > len(allOptions) {
			endLog = len(allOptions)
		}
		for i := startLog; i < endLog; i++ {
			slog.Debug("AVAILABLE STRIKE",
				"index", i,
				"strike", allOptions[i].strike,
				"symbol", allOptions[i].symbol,
				"distFromATM", i-atmIndex)
		}
	}

	// Windowed approach: start with initial window, expand as needed
	// Window size and expansion parameters
	const initialWindowSize = 40 // Initial window around ATM
	const windowExpansion = 20   // How many to load when expanding
	const maxIterations = 5      // Maximum number of expansion iterations

	targetDelta := config.TradeConfig.TargetDelta
	width := config.TradeConfig.Width

	// Track loaded Greeks and quotes
	greeksMap := make(map[string]map[string]interface{})
	quotesMap := make(map[string]*models.StockQuote)

	// Track window bounds (indices into allOptions)
	windowStart := atmIndex - initialWindowSize/2
	windowEnd := atmIndex + initialWindowSize/2
	if windowStart < 0 {
		windowStart = 0
	}
	if windowEnd > len(allOptions) {
		windowEnd = len(allOptions)
	}

	// Best short strike found so far
	var bestShort *struct {
		symbol string
		strike float64
		delta  float64
		bid    float64
		ask    float64
	}

	// Helper function to extract delta from greeks
	extractDelta := func(greeks map[string]interface{}) (float64, bool) {
		if greeks == nil {
			return 0, false
		}
		switch d := greeks["delta"].(type) {
		case float64:
			return d, true
		case *float64:
			if d != nil {
				return *d, true
			}
		}
		return 0, false
	}

	// Helper function to get bid/ask from quote
	getBidAsk := func(symbol string) (float64, float64) {
		bid, ask := 0.0, 0.0
		if quote, ok := quotesMap[symbol]; ok && quote != nil {
			if quote.Bid != nil {
				bid = *quote.Bid
			}
			if quote.Ask != nil {
				ask = *quote.Ask
			}
		}
		return bid, ask
	}

	// Search for target delta with windowed loading
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Collect symbols in current window that haven't been loaded yet
		var symbolsToLoad []string
		for i := windowStart; i < windowEnd && i < len(allOptions); i++ {
			opt := allOptions[i]
			if _, loaded := greeksMap[opt.symbol]; !loaded {
				symbolsToLoad = append(symbolsToLoad, opt.symbol)
			}
		}

		if len(symbolsToLoad) > 0 {
			slog.Info("Loading Greeks for window",
				"iteration", iteration+1,
				"windowStart", windowStart,
				"windowEnd", windowEnd,
				"symbolsToLoad", len(symbolsToLoad))

			// Fetch Greeks for this window with retry for missing symbols
			var validGreeksCount int
			maxRetries := 2
			for retry := 0; retry <= maxRetries; retry++ {
				var symbolsForThisAttempt []string
				if retry == 0 {
					symbolsForThisAttempt = symbolsToLoad
				} else {
					// On retry, only load symbols that are still missing Greeks
					for _, sym := range symbolsToLoad {
						if greeksMap[sym] == nil {
							symbolsForThisAttempt = append(symbolsForThisAttempt, sym)
						}
					}
					if len(symbolsForThisAttempt) == 0 {
						break // All symbols have Greeks now
					}
					slog.Info("Retrying Greeks fetch for missing symbols",
						"retry", retry,
						"missingCount", len(symbolsForThisAttempt))
				}

				newGreeks, err := e.providerManager.GetOptionsGreeksBatch(ctx, symbolsForThisAttempt)
				if err != nil {
					slog.Warn("Failed to get Greeks for window", "error", err, "iteration", iteration, "retry", retry)
					continue
				}

				// Store received Greeks
				for symbol, greeks := range newGreeks {
					if greeks != nil {
						greeksMap[symbol] = greeks
					}
				}

				// Count total valid Greeks after this attempt
				validGreeksCount = 0
				for _, sym := range symbolsToLoad {
					if g := greeksMap[sym]; g != nil {
						if _, ok := g["delta"]; ok {
							validGreeksCount++
						}
					}
				}

				slog.Info("Greeks received",
					"requested", len(symbolsForThisAttempt),
					"received", len(newGreeks),
					"totalValidWithDelta", validGreeksCount,
					"totalRequested", len(symbolsToLoad),
					"retry", retry)

				// If we got at least 90% of Greeks, stop retrying
				if float64(validGreeksCount) >= float64(len(symbolsToLoad))*0.9 {
					break
				}
			}

			// SAFEGUARD: If we got very few Greeks, something is wrong
			minRequiredRatio := 0.5 // At least 50% should have data
			if float64(validGreeksCount) < float64(len(symbolsToLoad))*minRequiredRatio {
				slog.Warn("Low Greeks coverage - data quality issue",
					"requested", len(symbolsToLoad),
					"validWithDelta", validGreeksCount,
					"minRequired", int(float64(len(symbolsToLoad))*minRequiredRatio))
			}

			// Fetch quotes for this window
			newQuotes, err := e.providerManager.GetStockQuotes(ctx, symbolsToLoad)
			if err != nil {
				slog.Warn("Failed to get quotes for window", "error", err, "iteration", iteration)
			} else {
				for symbol, quote := range newQuotes {
					quotesMap[symbol] = quote
				}
			}
		}

		// Search from ATM outward in the OTM direction
		// For puts: search from atmIndex toward lower indices (lower strikes = more OTM)
		// For calls: search from atmIndex toward higher indices (higher strikes = more OTM)
		// Stop at the FIRST strike where delta <= target (closest to target from OTM side)

		withDeltaCount := 0
		roundedTarget := float64(int(targetDelta*100+0.5)) / 100

		// Determine search bounds within current window
		searchStart := atmIndex
		if searchStart < windowStart {
			searchStart = windowStart
		}
		if searchStart >= windowEnd {
			searchStart = windowEnd - 1
		}

		// Track if we've gone past target delta (found OTM strikes with delta < target)
		foundBelowTarget := false
		lastAboveTarget := (*struct {
			symbol string
			strike float64
			delta  float64
			bid    float64
			ask    float64
		})(nil)

		if optionType == "put" {
			// For puts: search downward from ATM (lower strikes = more OTM = lower delta)
			for i := searchStart; i >= windowStart; i-- {
				opt := allOptions[i]
				delta, ok := extractDelta(greeksMap[opt.symbol])
				if !ok {
					continue
				}

				withDeltaCount++
				absDelta := delta
				if absDelta < 0 {
					absDelta = -absDelta
				}

				roundedDelta := float64(int(absDelta*100+0.5)) / 100

				if roundedDelta <= roundedTarget {
					// Found a strike at or below target - this is our best candidate
					foundBelowTarget = true
					bid, ask := getBidAsk(opt.symbol)

					// If we already have a candidate, keep the one with higher delta (closer to target)
					if bestShort == nil || absDelta > bestShort.delta {
						bestShort = &struct {
							symbol string
							strike float64
							delta  float64
							bid    float64
							ask    float64
						}{
							symbol: opt.symbol,
							strike: opt.strike,
							delta:  absDelta,
							bid:    bid,
							ask:    ask,
						}
					}
				} else {
					// Delta still above target - track this as potential fallback
					bid, ask := getBidAsk(opt.symbol)
					lastAboveTarget = &struct {
						symbol string
						strike float64
						delta  float64
						bid    float64
						ask    float64
					}{
						symbol: opt.symbol,
						strike: opt.strike,
						delta:  absDelta,
						bid:    bid,
						ask:    ask,
					}

					// If we already found something below target and now we're above again,
					// we've found the best (we're moving ITM now)
					if foundBelowTarget {
						break
					}
				}
			}
		} else {
			// For calls: search upward from ATM (higher strikes = more OTM = lower delta)
			for i := searchStart; i < windowEnd && i < len(allOptions); i++ {
				opt := allOptions[i]
				delta, ok := extractDelta(greeksMap[opt.symbol])
				if !ok {
					continue
				}

				withDeltaCount++
				absDelta := delta
				if absDelta < 0 {
					absDelta = -absDelta
				}

				roundedDelta := float64(int(absDelta*100+0.5)) / 100

				if roundedDelta <= roundedTarget {
					// Found a strike at or below target - this is our best candidate
					foundBelowTarget = true
					bid, ask := getBidAsk(opt.symbol)

					// If we already have a candidate, keep the one with higher delta (closer to target)
					if bestShort == nil || absDelta > bestShort.delta {
						bestShort = &struct {
							symbol string
							strike float64
							delta  float64
							bid    float64
							ask    float64
						}{
							symbol: opt.symbol,
							strike: opt.strike,
							delta:  absDelta,
							bid:    bid,
							ask:    ask,
						}
					}
				} else {
					// Delta still above target - track this as potential fallback
					bid, ask := getBidAsk(opt.symbol)
					lastAboveTarget = &struct {
						symbol string
						strike float64
						delta  float64
						bid    float64
						ask    float64
					}{
						symbol: opt.symbol,
						strike: opt.strike,
						delta:  absDelta,
						bid:    bid,
						ask:    ask,
					}

					// If we already found something below target and now we're above again,
					// we've found the best (we're moving ITM now)
					if foundBelowTarget {
						break
					}
				}
			}
		}

		slog.Info("Window search results",
			"iteration", iteration+1,
			"windowStart", windowStart,
			"windowEnd", windowEnd,
			"atmIndex", atmIndex,
			"withDeltaCount", withDeltaCount,
			"targetDelta", targetDelta,
			"foundBelowTarget", foundBelowTarget,
			"foundMatch", bestShort != nil,
			"bestStrike", func() float64 {
				if bestShort != nil {
					return bestShort.strike
				}
				return 0
			}(),
			"bestDelta", func() float64 {
				if bestShort != nil {
					return bestShort.delta
				}
				return 0
			}())

		// Check if we found a good match
		if bestShort != nil {
			deltaFromTarget := targetDelta - bestShort.delta

			// If we found something very close to target (within 0.01), stop
			if deltaFromTarget <= 0.01 {
				slog.Info("Found target delta within threshold",
					"strike", bestShort.strike,
					"delta", bestShort.delta,
					"deltaFromTarget", deltaFromTarget,
					"iteration", iteration+1)
				break
			}

			// If we found strikes both above and below target in this window,
			// we have the best possible - the highest delta that's <= target
			if foundBelowTarget && lastAboveTarget != nil {
				slog.Info("Found best possible delta (have strikes above and below target)",
					"strike", bestShort.strike,
					"delta", bestShort.delta,
					"lastAboveTargetDelta", lastAboveTarget.delta,
					"iteration", iteration+1)
				break
			}
		}

		// If no delta data at all in searched range, log and continue trying to expand
		if withDeltaCount == 0 {
			slog.Warn("No delta data found in search range",
				"iteration", iteration+1,
				"searchStart", searchStart,
				"windowStart", windowStart,
				"windowEnd", windowEnd,
				"greeksMapSize", len(greeksMap))
			// Don't break - try to expand window and load more Greeks
		}

		// Need to expand window to find more OTM strikes (lower delta)
		if optionType == "put" {
			// Put OTM = lower strikes = expand windowStart down
			if windowStart > 0 {
				newStart := windowStart - windowExpansion
				if newStart < 0 {
					newStart = 0
				}
				slog.Info("Expanding window down (more OTM puts)",
					"oldStart", windowStart,
					"newStart", newStart)
				windowStart = newStart
			} else {
				slog.Info("Cannot expand further (at start of options), using best available")
				break
			}
		} else {
			// Call OTM = higher strikes = expand windowEnd up
			if windowEnd < len(allOptions) {
				newEnd := windowEnd + windowExpansion
				if newEnd > len(allOptions) {
					newEnd = len(allOptions)
				}
				slog.Info("Expanding window up (more OTM calls)",
					"oldEnd", windowEnd,
					"newEnd", newEnd)
				windowEnd = newEnd
			} else {
				slog.Info("Cannot expand further (at end of options), using best available")
				break
			}
		}
	}

	// Fallback: if no exact match found, search all loaded Greeks
	if bestShort == nil {
		slog.Info("No match found in window, searching all loaded Greeks",
			"totalOptions", len(allOptions),
			"optionsWithGreeks", len(greeksMap),
			"optionType", optionType,
			"windowEnd", windowEnd,
			"windowStart", windowStart,
			"atmIndex", atmIndex)

		withDeltaCount := 0
		roundedTarget := float64(int(targetDelta*100+0.5)) / 100
		fallbackBatchSize := 20

		// Search all options that have Greeks loaded, from most OTM to ATM
		// This handles the case where the window covered all options
		var searchStart, searchEnd int
		var searchDirection int

		if optionType == "call" {
			// Calls: search from highest strike (most OTM) towards ATM
			searchStart = len(allOptions) - 1
			searchEnd = -1
			searchDirection = -1
		} else {
			// Puts: search from lowest strike (most OTM) towards ATM
			searchStart = 0
			searchEnd = len(allOptions)
			searchDirection = 1
		}

		slog.Info("Fallback search range",
			"searchStart", searchStart,
			"searchEnd", searchEnd,
			"searchDirection", searchDirection)

		// Search incrementally, loading Greeks as needed
		idx := searchStart
		batchCount := 0
		for (searchDirection == 1 && idx < searchEnd) || (searchDirection == -1 && idx > searchEnd) {
			// Determine batch range
			var batchIndices []int
			for i := 0; i < fallbackBatchSize && ((searchDirection == 1 && idx < searchEnd) || (searchDirection == -1 && idx > searchEnd)); i++ {
				batchIndices = append(batchIndices, idx)
				idx += searchDirection
			}

			if len(batchIndices) == 0 {
				break
			}

			batchCount++

			// Collect symbols that need Greeks in this batch
			var symbolsToLoad []string
			for _, i := range batchIndices {
				opt := allOptions[i]
				if _, loaded := greeksMap[opt.symbol]; !loaded {
					symbolsToLoad = append(symbolsToLoad, opt.symbol)
				}
			}

			// Load Greeks for this batch if needed
			if len(symbolsToLoad) > 0 {
				newGreeks, err := e.providerManager.GetOptionsGreeksBatch(ctx, symbolsToLoad)
				if err != nil {
					slog.Warn("Failed to load Greeks batch in fallback", "error", err, "batch", batchCount)
				} else {
					for symbol, greeks := range newGreeks {
						greeksMap[symbol] = greeks
					}
				}

				// Also load quotes
				newQuotes, err := e.providerManager.GetStockQuotes(ctx, symbolsToLoad)
				if err == nil {
					for symbol, quote := range newQuotes {
						quotesMap[symbol] = quote
					}
				}
			}

			// Search this batch for a match
			for _, i := range batchIndices {
				opt := allOptions[i]
				delta, ok := extractDelta(greeksMap[opt.symbol])
				if !ok {
					continue
				}

				withDeltaCount++
				absDelta := delta
				if absDelta < 0 {
					absDelta = -absDelta
				}

				// Round to 2 decimals for comparison
				roundedDelta := float64(int(absDelta*100+0.5)) / 100

				// Only consider strikes with delta <= target (rounded)
				if roundedDelta <= roundedTarget {
					// Among qualifying strikes, pick the one with highest delta (closest to target)
					if bestShort == nil || absDelta > bestShort.delta {
						bid, ask := getBidAsk(opt.symbol)
						bestShort = &struct {
							symbol string
							strike float64
							delta  float64
							bid    float64
							ask    float64
						}{
							symbol: opt.symbol,
							strike: opt.strike,
							delta:  absDelta,
							bid:    bid,
							ask:    ask,
						}
					}
				}
			}

			// If we found a match in this batch, stop searching
			// As we go further OTM, deltas decrease, so once we find a match,
			// continuing won't find a better one (higher delta <= target)
			if bestShort != nil {
				slog.Info("Found fallback match, stopping search",
					"strike", bestShort.strike,
					"delta", bestShort.delta,
					"batchesSearched", batchCount)
				break
			}
		}

		if bestShort == nil {
			return nil, fmt.Errorf("no options with delta <= %.4f for %s expiry %s (checked %d options with greeks out of %d total)", targetDelta, config.Symbol, expiry, withDeltaCount, len(allOptions))
		}

		slog.Info("Using fallback strike",
			"strike", bestShort.strike,
			"delta", bestShort.delta,
			"roundedDelta", float64(int(bestShort.delta*100+0.5))/100,
			"targetDelta", targetDelta,
			"roundedTarget", roundedTarget)
	}

	// CRITICAL VALIDATION: Ensure the selected strike makes sense
	// This prevents returning wrong strikes due to partial data failures

	// 1. Delta sanity check - verify roundedDelta <= roundedTarget
	// Selection logic already guarantees this, but double-check as safety measure
	roundedSelectedDelta := float64(int(bestShort.delta*100+0.5)) / 100
	roundedTargetDelta := float64(int(targetDelta*100+0.5)) / 100

	if roundedSelectedDelta > roundedTargetDelta {
		slog.Error("VALIDATION FAILED: Selected strike delta exceeds target",
			"selectedStrike", bestShort.strike,
			"selectedDelta", bestShort.delta,
			"roundedSelectedDelta", roundedSelectedDelta,
			"targetDelta", targetDelta,
			"roundedTargetDelta", roundedTargetDelta)
		return nil, fmt.Errorf("strike validation failed: selected delta %.2f exceeds target %.2f",
			roundedSelectedDelta, roundedTargetDelta)
	}

	// 2. Strike vs underlying price sanity check (if we have underlying price)
	if underlyingPrice != nil {
		strikeDistance := bestShort.strike - *underlyingPrice
		if strikeDistance < 0 {
			strikeDistance = -strikeDistance
		}
		maxDistancePercent := 0.15 // 15% max distance from underlying
		maxDistance := *underlyingPrice * maxDistancePercent

		if strikeDistance > maxDistance {
			slog.Error("VALIDATION FAILED: Selected strike is too far from underlying price",
				"selectedStrike", bestShort.strike,
				"underlyingPrice", *underlyingPrice,
				"distance", strikeDistance,
				"maxDistance", maxDistance)
			return nil, fmt.Errorf("strike validation failed: strike %.0f is %.1f%% away from underlying %.0f (max: %.0f%%)",
				bestShort.strike, (strikeDistance / *underlyingPrice * 100), *underlyingPrice, maxDistancePercent*100)
		}
	}

	// 3. Log successful validation
	slog.Info("Strike validation passed",
		"strike", bestShort.strike,
		"delta", bestShort.delta,
		"targetDelta", targetDelta,
		"underlyingPrice", underlyingPrice)

	// Find long strike (for put spread: lower strike, for call spread: higher strike)
	longStrike := bestShort.strike - float64(width)
	if optionType == "call" {
		longStrike = bestShort.strike + float64(width)
	}

	// Find the long option - may need to load Greeks/quotes if not in window
	// NOTE: We find the NEAREST available strike since indices like NDX have $25 or $50 intervals
	// For puts: long strike should be <= target (lower/more OTM)
	// For calls: long strike should be >= target (higher/more OTM)

	slog.Info("Searching for long strike", "targetLongStrike", longStrike, "optionType", optionType, "width", width)

	// Step 1: Find the best candidate strike WITHOUT loading Greeks
	var bestLongCandidate *struct {
		symbol string
		strike float64
	}
	var bestLongDistance float64 = 999999

	for _, opt := range allOptions {
		var distance float64

		if optionType == "put" {
			// For puts, long strike should be <= calculated target (lower strike = more OTM)
			if opt.strike > longStrike {
				continue
			}
			distance = longStrike - opt.strike
		} else {
			// For calls, long strike should be >= calculated target (higher strike = more OTM)
			if opt.strike < longStrike {
				continue
			}
			distance = opt.strike - longStrike
		}

		// Pick the strike closest to our target
		if distance < bestLongDistance {
			bestLongDistance = distance
			bestLongCandidate = &struct {
				symbol string
				strike float64
			}{
				symbol: opt.symbol,
				strike: opt.strike,
			}

			// If exact match, stop searching
			if distance == 0 {
				break
			}
		}
	}

	if bestLongCandidate == nil {
		return nil, fmt.Errorf("could not find long strike near %.2f (searched %d options)", longStrike, len(allOptions))
	}

	// Log if we had to use a different strike than calculated
	if bestLongCandidate.strike != longStrike {
		slog.Info("Using nearest available long strike",
			"targetLongStrike", longStrike,
			"actualLongStrike", bestLongCandidate.strike,
			"difference", bestLongDistance,
			"optionType", optionType)
	}

	// Step 2: Load Greeks/quotes for the selected candidate ONLY
	if _, loaded := greeksMap[bestLongCandidate.symbol]; !loaded {
		slog.Info("Loading Greeks for long leg", "symbol", bestLongCandidate.symbol, "strike", bestLongCandidate.strike)
		newGreeks, err := e.providerManager.GetOptionsGreeksBatch(ctx, []string{bestLongCandidate.symbol})
		if err == nil {
			for symbol, greeks := range newGreeks {
				greeksMap[symbol] = greeks
			}
		}
		newQuotes, err := e.providerManager.GetStockQuotes(ctx, []string{bestLongCandidate.symbol})
		if err == nil {
			for symbol, quote := range newQuotes {
				quotesMap[symbol] = quote
			}
		}
	}

	// Step 3: Build the longOption struct with Greeks and quotes
	delta, _ := extractDelta(greeksMap[bestLongCandidate.symbol])
	if delta < 0 {
		delta = -delta
	}
	bid, ask := getBidAsk(bestLongCandidate.symbol)

	longOption := &struct {
		symbol string
		strike float64
		delta  float64
		bid    float64
		ask    float64
	}{
		symbol: bestLongCandidate.symbol,
		strike: bestLongCandidate.strike,
		delta:  delta,
		bid:    bid,
		ask:    ask,
	}

	// Calculate credit
	naturalCredit := bestShort.bid - longOption.ask
	midCredit := ((bestShort.bid + bestShort.ask) / 2) - ((longOption.bid + longOption.ask) / 2)

	// Log successful strike selection with timing
	selectionDuration := time.Since(selectionStartTime)
	slog.Info("Strike selection completed",
		"shortStrike", bestShort.strike,
		"shortDelta", bestShort.delta,
		"longStrike", longOption.strike,
		"longDelta", longOption.delta,
		"targetDelta", targetDelta,
		"midCredit", midCredit,
		"duration", selectionDuration.String())

	// SAFEGUARD: Warn if selection took too long (might indicate issues)
	if selectionDuration > 10*time.Second {
		slog.Warn("Strike selection took unusually long - verify results carefully",
			"duration", selectionDuration.String(),
			"shortStrike", bestShort.strike,
			"shortDelta", bestShort.delta)
	}

	// Create leg details for preview
	shortLeg := &types.LegDetail{
		Symbol: bestShort.symbol,
		Strike: bestShort.strike,
		Delta:  bestShort.delta,
		Bid:    bestShort.bid,
		Ask:    bestShort.ask,
		Mid:    (bestShort.bid + bestShort.ask) / 2,
	}
	longLegDetail := &types.LegDetail{
		Symbol: longOption.symbol,
		Strike: longOption.strike,
		Delta:  longOption.delta,
		Bid:    longOption.bid,
		Ask:    longOption.ask,
		Mid:    (longOption.bid + longOption.ask) / 2,
	}

	return &types.StrikeSelection{
		ShortStrike:   bestShort.strike,
		LongStrike:    longOption.strike,
		ShortSymbol:   bestShort.symbol,
		LongSymbol:    longOption.symbol,
		ShortDelta:    bestShort.delta,
		LongDelta:     longOption.delta,
		OptionType:    optionType,
		Expiry:        expiry,
		NaturalCredit: naturalCredit,
		MidCredit:     midCredit,
		ShortLeg:      shortLeg,
		LongLeg:       longLegDetail,
	}, nil
}

// placeSpreadOrder places a credit spread order
func (e *Engine) placeSpreadOrder(ctx context.Context, automationID string, config *types.AutomationConfig, strikes *types.StrikeSelection, units int) (*types.PlacedOrder, error) {
	// Build order legs - must use []interface{} for proper type assertion in provider
	legs := []interface{}{
		map[string]interface{}{
			"symbol": strikes.ShortSymbol,
			"side":   "sell_to_open",
			"qty":    float64(units),
		},
		map[string]interface{}{
			"symbol": strikes.LongSymbol,
			"side":   "buy_to_open",
			"qty":    float64(units),
		},
	}

	slog.Info("Placing spread order",
		"shortSymbol", strikes.ShortSymbol,
		"longSymbol", strikes.LongSymbol,
		"units", units,
		"midCredit", strikes.MidCredit)

	// Calculate starting price: mid credit minus starting offset
	startingOffset := config.TradeConfig.StartingOffset
	if startingOffset == 0 {
		startingOffset = 0.0 // Default to no offset (start at mid)
	}

	startingCredit := strikes.MidCredit - startingOffset

	// Check minimum credit threshold
	if config.TradeConfig.MinCredit > 0 && startingCredit < config.TradeConfig.MinCredit {
		return nil, fmt.Errorf("starting credit %.2f is below minimum credit threshold %.2f", startingCredit, config.TradeConfig.MinCredit)
	}

	// Ensure credit is positive
	if startingCredit <= 0 {
		return nil, fmt.Errorf("starting credit %.2f is not positive (mid: %.2f, offset: %.2f)", startingCredit, strikes.MidCredit, startingOffset)
	}

	limitPrice := -startingCredit // Negative for credit

	orderData := map[string]interface{}{
		"legs":          legs,
		"limit_price":   limitPrice,
		"order_type":    config.TradeConfig.OrderType,
		"time_in_force": config.TradeConfig.TimeInForce,
	}

	order, err := e.providerManager.PlaceMultiLegOrder(ctx, orderData)
	if err != nil {
		return nil, err
	}

	placedOrder := &types.PlacedOrder{
		OrderID:       order.ID,
		AutomationID:  automationID,
		ConfigName:    config.Name,
		Legs:          order.Legs,
		LimitPrice:    startingCredit, // Store as positive credit
		TargetDelta:   config.TradeConfig.TargetDelta,
		ActualDelta:   strikes.ShortDelta,
		AttemptNumber: 1,
		Status:        "open",
		PlacedAt:      time.Now(),
	}

	// Track the order in the tracking store
	e.trackingStore.TrackOrder(placedOrder)

	return placedOrder, nil
}

// checkOrderStatus checks the status of an order
func (e *Engine) checkOrderStatus(ctx context.Context, orderID string) (string, error) {
	orders, err := e.providerManager.GetOrders(ctx, "all")
	if err != nil {
		return "", err
	}

	for _, order := range orders {
		if order.ID == orderID {
			return order.Status, nil
		}
	}

	return "", fmt.Errorf("order %s not found", orderID)
}

// cancelOrder cancels an order
func (e *Engine) cancelOrder(ctx context.Context, orderID string) error {
	_, err := e.providerManager.CancelOrder(ctx, orderID)
	return err
}

// replaceOrderWithNewPrice places a new order with a reduced price
func (e *Engine) replaceOrderWithNewPrice(ctx context.Context, automationID string, active *types.ActiveAutomation, newPrice float64) (*types.PlacedOrder, error) {
	if active.CurrentOrder == nil {
		return nil, fmt.Errorf("no current order to replace")
	}

	// Build legs from current order - must use []interface{} for proper type assertion in provider
	legs := make([]interface{}, len(active.CurrentOrder.Legs))
	for i, leg := range active.CurrentOrder.Legs {
		legs[i] = map[string]interface{}{
			"symbol": leg.Symbol,
			"side":   leg.Side,
			"qty":    leg.Qty,
		}
	}

	orderData := map[string]interface{}{
		"legs":          legs,
		"limit_price":   -newPrice, // Negative for credit
		"order_type":    active.Config.TradeConfig.OrderType,
		"time_in_force": active.Config.TradeConfig.TimeInForce,
	}

	order, err := e.providerManager.PlaceMultiLegOrder(ctx, orderData)
	if err != nil {
		return nil, err
	}

	placedOrder := &types.PlacedOrder{
		OrderID:       order.ID,
		AutomationID:  automationID,
		ConfigName:    active.Config.Name,
		Legs:          order.Legs,
		LimitPrice:    newPrice,
		TargetDelta:   active.CurrentOrder.TargetDelta,
		ActualDelta:   active.CurrentOrder.ActualDelta,
		AttemptNumber: active.CurrentOrder.AttemptNumber + 1,
		Status:        "open",
		PlacedAt:      time.Now(),
	}

	// Track the replacement order
	e.trackingStore.TrackOrder(placedOrder)

	return placedOrder, nil
}

// isMarketHours checks if we're within market hours
func (e *Engine) isMarketHours() bool {
	ny, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(ny)

	// Check weekday
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}

	// Market hours: 9:30 AM - 4:00 PM ET
	marketOpen := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, ny)
	marketClose := time.Date(now.Year(), now.Month(), now.Day(), 16, 0, 0, 0, ny)

	return now.After(marketOpen) && now.Before(marketClose)
}

// isEntryTime checks if it's time to enter a trade
func (e *Engine) isEntryTime(config *types.AutomationConfig) bool {
	loc, err := time.LoadLocation(config.EntryTimezone)
	if err != nil {
		loc, _ = time.LoadLocation("America/New_York")
	}

	now := time.Now().In(loc)
	entryTime, err := time.Parse("15:04", config.EntryTime)
	if err != nil {
		return false
	}

	// Create entry time for today
	entryToday := time.Date(now.Year(), now.Month(), now.Day(),
		entryTime.Hour(), entryTime.Minute(), 0, 0, loc)

	// Allow 5-minute window starting exactly at entry time
	windowStart := entryToday
	windowEnd := entryToday.Add(5 * time.Minute)

	return !now.Before(windowStart) && now.Before(windowEnd)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
