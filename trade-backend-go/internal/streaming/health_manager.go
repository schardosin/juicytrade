package streaming

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"trade-backend-go/internal/providers/base"
)

// ConnectionState represents the state of a streaming connection
type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateDegraded     ConnectionState = "degraded"
	StateFailed       ConnectionState = "failed"
	StateRecovering   ConnectionState = "recovering"
)

// ConnectionMetrics tracks metrics for a single connection
type ConnectionMetrics struct {
	ProviderName      string          `json:"provider_name"`
	ConnectionType    string          `json:"connection_type"`
	State             ConnectionState `json:"state"`
	ConnectedAt       *time.Time      `json:"connected_at"`
	LastDataTime      *time.Time      `json:"last_data_time"`
	LastPingTime      *time.Time      `json:"last_ping_time"`
	MessageCount      int64           `json:"message_count"`
	ErrorCount        int64           `json:"error_count"`
	ReconnectionCount int64           `json:"reconnection_count"`
	LastError         string          `json:"last_error"`
	SubscribedSymbols map[string]bool `json:"subscribed_symbols"`
	mutex             sync.RWMutex
}

// NewConnectionMetrics creates new connection metrics
func NewConnectionMetrics(providerName, connectionType string) *ConnectionMetrics {
	return &ConnectionMetrics{
		ProviderName:      providerName,
		ConnectionType:    connectionType,
		State:             StateDisconnected,
		SubscribedSymbols: make(map[string]bool),
	}
}

// UptimeSeconds returns connection uptime in seconds
func (cm *ConnectionMetrics) UptimeSeconds() float64 {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	if cm.ConnectedAt != nil {
		return time.Since(*cm.ConnectedAt).Seconds()
	}
	return 0
}

// TimeSinceLastData returns time since last data in seconds
func (cm *ConnectionMetrics) TimeSinceLastData() float64 {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	if cm.LastDataTime != nil {
		return time.Since(*cm.LastDataTime).Seconds()
	}
	return 999999.0 // Large number instead of infinity
}

// IsStale checks if connection is stale (no data for too long)
func (cm *ConnectionMetrics) IsStale(timeout float64) bool {
	return cm.TimeSinceLastData() > timeout
}

// UpdateState safely updates the connection state
func (cm *ConnectionMetrics) UpdateState(state ConnectionState) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	oldState := cm.State
	cm.State = state
	
	if state == StateConnected {
		now := time.Now()
		cm.ConnectedAt = &now
	}
	
	if oldState != state {
		slog.Info(fmt.Sprintf("🔄 Connection %s: %s → %s", cm.ProviderName, oldState, state))
	}
}

// RecordDataReceived records that data was received
func (cm *ConnectionMetrics) RecordDataReceived() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	now := time.Now()
	cm.LastDataTime = &now
	cm.MessageCount++
}

// RecordError records an error
func (cm *ConnectionMetrics) RecordError(err string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	cm.ErrorCount++
	cm.LastError = err
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	failureThreshold int
	recoveryTimeout  time.Duration
	failureCount     int
	lastFailureTime  time.Time
	state            string // "closed", "open", "half_open"
	mutex            sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		state:            "closed",
	}
}

// CanExecute checks if operation can be executed
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	switch cb.state {
	case "closed":
		return true
	case "open":
		if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
			cb.state = "half_open"
			return true
		}
		return false
	case "half_open":
		return true
	default:
		return false
	}
}

// RecordSuccess records successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failureCount = 0
	cb.state = "closed"
}

// RecordFailure records failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	
	if cb.failureCount >= cb.failureThreshold {
		cb.state = "open"
		slog.Warn(fmt.Sprintf("🔴 Circuit breaker opened after %d failures", cb.failureCount))
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if cb.state == "open" {
		slog.Info("🔄 Circuit breaker manually reset to closed state")
	}
	cb.failureCount = 0
	cb.state = "closed"
}


// StreamingHealthManager manages streaming connection health
type StreamingHealthManager struct {
	connections     map[string]*ConnectionMetrics
	circuitBreakers map[string]*CircuitBreaker
	providers       map[string]base.Provider
	recoveryTasks   map[string]context.CancelFunc
	recovering      map[string]bool // Track which connections are currently recovering
	recoveryCond    map[string]*sync.Cond // Condition variables for blocking on recovery completion
	isMonitoring    bool
	shutdownChan    chan struct{}
	mutex           sync.RWMutex
	
	// Configuration
	dataTimeout       time.Duration
	checkInterval     time.Duration
	pingInterval      time.Duration
	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration
	cleanupDelay      time.Duration
	stabilizationDelay time.Duration
	
	// Metrics
	startTime           time.Time
	totalReconnections  int64
	totalFailures       int64
}

// NewStreamingHealthManager creates a new health manager
func NewStreamingHealthManager() *StreamingHealthManager {
	return &StreamingHealthManager{
		connections:       make(map[string]*ConnectionMetrics),
		circuitBreakers:   make(map[string]*CircuitBreaker),
		providers:         make(map[string]base.Provider),
		recoveryTasks:     make(map[string]context.CancelFunc),
		recovering:        make(map[string]bool),
		recoveryCond:      make(map[string]*sync.Cond),
		shutdownChan:      make(chan struct{}),
		dataTimeout:       60 * time.Second,
		checkInterval:     10 * time.Second,
		pingInterval:      30 * time.Second,
		reconnectDelay:    500 * time.Millisecond,
		maxReconnectDelay: 5 * time.Minute,
		cleanupDelay:      1 * time.Second,
		stabilizationDelay: 2 * time.Second,
		startTime:         time.Now(),
	}
}

// IsRecovering checks if a connection is currently in recovery
func (hm *StreamingHealthManager) IsRecovering(connectionID string) bool {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	return hm.recovering[connectionID]
}

// WaitForRecovery blocks until recovery completes or times out
func (hm *StreamingHealthManager) WaitForRecovery(ctx context.Context, connectionID string, timeout time.Duration) error {
	hm.mutex.Lock()
	
	// If not recovering, return immediately
	if !hm.recovering[connectionID] {
		hm.mutex.Unlock()
		return nil
	}
	
	// Get or create condition variable for this connection
	cond := hm.recoveryCond[connectionID]
	if cond == nil {
		cond = sync.NewCond(&hm.mutex)
		hm.recoveryCond[connectionID] = cond
	}
	
	// Create channel to signal completion
	done := make(chan struct{})
	
	// Wait in goroutine
	go func() {
		cond.L.Lock()
		for hm.recovering[connectionID] {
			cond.Wait()
		}
		cond.L.Unlock()
		close(done)
	}()
	
	hm.mutex.Unlock()
	
	// Wait for either completion or timeout
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for recovery to complete")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RegisterProvider registers a provider for health monitoring
func (hm *StreamingHealthManager) RegisterProvider(name string, provider base.Provider) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	hm.providers[name] = provider
	slog.Info(fmt.Sprintf("📋 Registered provider %s for health monitoring", name))
}

// RegisterConnection registers a new connection for monitoring
func (hm *StreamingHealthManager) RegisterConnection(connectionID, providerName, connectionType string) *ConnectionMetrics {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	metrics := NewConnectionMetrics(providerName, connectionType)
	hm.connections[connectionID] = metrics
	hm.circuitBreakers[connectionID] = NewCircuitBreaker(5, time.Minute)
	
	slog.Info(fmt.Sprintf("📋 Registered connection %s (%s)", connectionID, providerName))
	return metrics
}

// UpdateConnectionState updates connection state with proper logging
func (hm *StreamingHealthManager) UpdateConnectionState(connectionID string, state ConnectionState) {
	hm.mutex.RLock()
	metrics, exists := hm.connections[connectionID]
	hm.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	metrics.UpdateState(state)
	
	// Update circuit breaker
	hm.mutex.RLock()
	cb, exists := hm.circuitBreakers[connectionID]
	hm.mutex.RUnlock()
	
	if exists {
		switch state {
		case StateConnected:
			cb.RecordSuccess()
		case StateFailed:
			cb.RecordFailure()
			hm.mutex.Lock()
			hm.totalFailures++
			hm.mutex.Unlock()
		}
	}
}

// RecordDataReceived records that data was received on a connection
func (hm *StreamingHealthManager) RecordDataReceived(connectionID string) {
	hm.mutex.RLock()
	metrics, exists := hm.connections[connectionID]
	hm.mutex.RUnlock()
	
	if exists {
		metrics.RecordDataReceived()
	}
}

// RecordError records an error for a connection
func (hm *StreamingHealthManager) RecordError(connectionID string, err string) {
	hm.mutex.RLock()
	metrics, exists := hm.connections[connectionID]
	hm.mutex.RUnlock()
	
	if exists {
		metrics.RecordError(err)
		slog.Error(fmt.Sprintf("❌ Connection %s error: %s", connectionID, err))
	}
}

// StartMonitoring starts the health monitoring system
func (hm *StreamingHealthManager) StartMonitoring(ctx context.Context) {
	hm.mutex.Lock()
	if hm.isMonitoring {
		hm.mutex.Unlock()
		slog.Warn("⚠️ Health monitoring already running")
		return
	}
	hm.isMonitoring = true
	hm.mutex.Unlock()
	
	slog.Info("🏥 Streaming health monitoring started")
	
	go hm.healthMonitorLoop(ctx)
}

// StopMonitoring stops the health monitoring system
func (hm *StreamingHealthManager) StopMonitoring() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	if !hm.isMonitoring {
		return
	}
	
	slog.Info("🛑 Stopping streaming health monitoring...")
	
	close(hm.shutdownChan)
	hm.isMonitoring = false
	
	// Cancel all recovery tasks
	for _, cancel := range hm.recoveryTasks {
		cancel()
	}
	hm.recoveryTasks = make(map[string]context.CancelFunc)
	
	slog.Info("✅ Streaming health monitoring stopped")
}

// healthMonitorLoop is the main health monitoring loop
func (hm *StreamingHealthManager) healthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-hm.shutdownChan:
			return
		case <-ticker.C:
			hm.checkAllConnections(ctx)
		}
	}
}

// checkAllConnections checks health of all registered connections
func (hm *StreamingHealthManager) checkAllConnections(ctx context.Context) {
	hm.mutex.RLock()
	connections := make(map[string]*ConnectionMetrics)
	for id, metrics := range hm.connections {
		connections[id] = metrics
	}
	hm.mutex.RUnlock()
	
	currentTime := time.Now()
	
	for connectionID, metrics := range connections {
		hm.checkConnectionHealth(ctx, connectionID, metrics, currentTime)
	}
}

// checkConnectionHealth checks health of a single connection
func (hm *StreamingHealthManager) checkConnectionHealth(ctx context.Context, connectionID string, metrics *ConnectionMetrics, currentTime time.Time) {
	// Skip if already recovering
	hm.mutex.RLock()
	_, isRecovering := hm.recoveryTasks[connectionID]
	hm.mutex.RUnlock()
	
	if isRecovering {
		return
	}
	
	// Add startup grace period - but only if connection has been established
	startupGracePeriod := 60 * time.Second
	var connectionAge time.Duration
	
	// FIX: Check if ConnectedAt is nil before dereferencing
	if metrics.ConnectedAt != nil {
		connectionAge = currentTime.Sub(*metrics.ConnectedAt)
	} else {
		connectionAge = 0
	}
	
	// Check for stale connections (but only after grace period and if connected)
	if metrics.State == StateConnected &&
		metrics.ConnectedAt != nil &&
		connectionAge > startupGracePeriod &&
		metrics.IsStale(hm.dataTimeout.Seconds()) {
		
		staleDuration := metrics.TimeSinceLastData()
		slog.Warn(fmt.Sprintf("⚠️ Connection %s is stale (no data for %.1fs)", connectionID, staleDuration))
		hm.UpdateConnectionState(connectionID, StateDegraded)
		
		// CHANGED: Trigger recovery immediately when degraded, don't wait 5 minutes!
		// A connection that's been stale for 60+ seconds is already unhealthy.
		slog.Error(fmt.Sprintf("❌ Connection %s is stale, triggering immediate recovery", connectionID))
		hm.TriggerRecovery(ctx, connectionID, false)
	} else if metrics.State == StateFailed {
		hm.TriggerRecovery(ctx, connectionID, false)
	}
	
	// Perform periodic ping for healthy connections (after grace period)
	if metrics.State == StateConnected &&
		metrics.ConnectedAt != nil &&
		connectionAge > startupGracePeriod {
		hm.performPeriodicPing(ctx, connectionID, metrics, currentTime)
	}
}
// performPeriodicPing performs periodic ping checks on healthy connections
func (hm *StreamingHealthManager) performPeriodicPing(ctx context.Context, connectionID string, metrics *ConnectionMetrics, currentTime time.Time) {
	// Check if it's time for a ping
	if metrics.LastPingTime == nil || currentTime.Sub(*metrics.LastPingTime) > hm.pingInterval {
		metrics.mutex.Lock()
		now := currentTime
		metrics.LastPingTime = &now
		metrics.mutex.Unlock()
		
		// Get provider
		hm.mutex.RLock()
		provider, exists := hm.providers[metrics.ProviderName]
		hm.mutex.RUnlock()
		
		if !exists {
			return
		}
		
		// Check if provider is still connected
		if !provider.IsStreamingConnected() {
			slog.Warn(fmt.Sprintf("⚠️ Ping check: %s reports not connected", connectionID))
			hm.UpdateConnectionState(connectionID, StateDegraded)
			hm.TriggerRecovery(ctx, connectionID, false)
			return
		}

		// Perform active ping
		if err := provider.Ping(ctx); err != nil {
			slog.Warn(fmt.Sprintf("⚠️ Ping check failed for %s: %v", connectionID, err))
			hm.UpdateConnectionState(connectionID, StateDegraded)
			hm.TriggerRecovery(ctx, connectionID, false)
		} else {
			slog.Debug(fmt.Sprintf("✅ Ping successful for %s", connectionID))
		}
	}
}

// UpdateSubscriptions updates subscribed symbols for a connection
func (hm *StreamingHealthManager) UpdateSubscriptions(connectionID string, symbols []string) {
	hm.mutex.RLock()
	metrics, exists := hm.connections[connectionID]
	hm.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()
	
	// Update subscribed symbols
	metrics.SubscribedSymbols = make(map[string]bool)
	for _, symbol := range symbols {
		metrics.SubscribedSymbols[symbol] = true
	}
	
	slog.Debug(fmt.Sprintf("📋 Updated subscriptions for %s: %d symbols", connectionID, len(symbols)))
}

// TriggerRecovery triggers recovery for a failed connection
func (hm *StreamingHealthManager) TriggerRecovery(ctx context.Context, connectionID string, forceImmediate bool) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	// Check if already recovering
	if _, exists := hm.recoveryTasks[connectionID]; exists {
		return
	}
	
	// If forceImmediate (user-triggered), reset circuit breaker to allow recovery
	cb, exists := hm.circuitBreakers[connectionID]
	if forceImmediate && exists {
		cb.Reset()
	}
	
	// Check circuit breaker
	if exists && !cb.CanExecute() {
		slog.Warn(fmt.Sprintf("🔴 Circuit breaker open for %s, skipping recovery", connectionID))
		return
	}
	
	slog.Info(fmt.Sprintf("🔄 Triggering recovery for %s (forceImmediate: %v)", connectionID, forceImmediate))
	
	// Update state directly to avoid deadlock (UpdateConnectionState acquires RLock)
	if metrics, exists := hm.connections[connectionID]; exists {
		metrics.UpdateState(StateRecovering)
	}
	
	// Mark as recovering and create condition variable if needed
	hm.recovering[connectionID] = true
	if hm.recoveryCond[connectionID] == nil {
		hm.recoveryCond[connectionID] = sync.NewCond(&hm.mutex)
	}
	
	// Start recovery task
	// CRITICAL: Use context.Background() (or a detached context) so that the recovery
	// task survives the cancellation of the triggering request (ctx).
	recoveryCtx, cancel := context.WithCancel(context.Background())
	hm.recoveryTasks[connectionID] = cancel
	
	go hm.recoverConnection(recoveryCtx, connectionID, forceImmediate)
}

// recoverConnection recovers a failed connection with retry logic and subscription restoration
func (hm *StreamingHealthManager) recoverConnection(ctx context.Context, connectionID string, forceImmediate bool) {
	// Add timeout to entire recovery process
	recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 30*time.Second)
	defer recoveryCancel()
	
	defer func() {
		hm.mutex.Lock()
		delete(hm.recoveryTasks, connectionID)
		
		// Clear recovering flag and broadcast to waiters
		hm.recovering[connectionID] = false
		if cond := hm.recoveryCond[connectionID]; cond != nil {
			cond.Broadcast()
		}
		hm.mutex.Unlock()
		
		slog.Info(fmt.Sprintf("🏁 Recovery process ended for %s", connectionID))
	}()
	
	slog.Info(fmt.Sprintf("🔄 Starting recovery for %s (forceImmediate: %v)", connectionID, forceImmediate))
	
	hm.mutex.RLock()
	metrics, exists := hm.connections[connectionID]
	if !exists {
		hm.mutex.RUnlock()
		slog.Error(fmt.Sprintf("❌ No metrics found for %s", connectionID))
		return
	}
	
	provider, providerExists := hm.providers[metrics.ProviderName]
	hm.mutex.RUnlock()
	
	if !providerExists {
		slog.Error(fmt.Sprintf("❌ No provider found for %s", connectionID))
		return
	}
	
	// Store subscriptions for restoration
	metrics.mutex.RLock()
	subscribedSymbols := make([]string, 0, len(metrics.SubscribedSymbols))
	for symbol := range metrics.SubscribedSymbols {
		subscribedSymbols = append(subscribedSymbols, symbol)
	}
	metrics.mutex.RUnlock()
	
	if len(subscribedSymbols) > 0 {
		slog.Info(fmt.Sprintf("🔄 Stored %d subscriptions for recovery of %s", len(subscribedSymbols), connectionID))
	}
	
	// Retry recovery up to 3 times (reduced from 5 for faster failure)
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-recoveryCtx.Done():
			slog.Error(fmt.Sprintf("❌ Recovery timeout for %s after attempt %d", connectionID, attempt))
			hm.UpdateConnectionState(connectionID, StateFailed)
			return
		default:
		}
		
		slog.Info(fmt.Sprintf("🔄 Recovery attempt #%d/%d for %s", attempt, maxRetries, connectionID))
		
		// Calculate backoff delay
		var delay time.Duration
		if forceImmediate && attempt == 1 {
			delay = 0
			slog.Info(fmt.Sprintf("⚡ Immediate recovery for %s", connectionID))
		} else {
			backoffMultiplier := int64(1 << uint(attempt-1))
			delay = time.Duration(int64(hm.reconnectDelay) * backoffMultiplier)
			if delay > hm.maxReconnectDelay {
				delay = hm.maxReconnectDelay
			}
		}
		
		if delay > 0 {
			slog.Info(fmt.Sprintf("⏳ Waiting %.1fs before reconnection", delay.Seconds()))
			select {
			case <-recoveryCtx.Done():
				slog.Error(fmt.Sprintf("❌ Recovery cancelled during delay for %s", connectionID))
				return
			case <-time.After(delay):
			}
		}
		
		// Step 1: Disconnect (cleanup)
		slog.Info(fmt.Sprintf("🔄 [%d/%d] Disconnecting %s", attempt, maxRetries, connectionID))
		if _, err := provider.DisconnectStreaming(recoveryCtx); err != nil {
			slog.Warn(fmt.Sprintf("⚠️  Disconnect error: %v", err))
		}
		
		// Brief cleanup delay
		time.Sleep(500 * time.Millisecond)
		
		// Step 2: Reconnect
		// CRITICAL: Use context.Background() for the connection, not recoveryCtx!
		// The connection needs to survive beyond the recovery process.
		// recoveryCtx is only for timing out the recovery attempt, not the connection itself.
		slog.Info(fmt.Sprintf("🔄 [%d/%d] Connecting %s", attempt, maxRetries, connectionID))
		success, err := provider.ConnectStreaming(context.Background())
		if err != nil || !success {
			slog.Error(fmt.Sprintf("❌ [%d/%d] Connect failed for %s: %v", attempt, maxRetries, connectionID, err))
			if attempt == maxRetries {
				hm.UpdateConnectionState(connectionID, StateFailed)
				return
			}
			continue
		}
		
		// Step 3: Quick verification
		slog.Info(fmt.Sprintf("🔄 [%d/%d] Verifying %s", attempt, maxRetries, connectionID))
		time.Sleep(1 * time.Second) // Reduced stabilization delay
		
		if !provider.IsStreamingConnected() {
			slog.Error(fmt.Sprintf("❌ [%d/%d] Verification failed for %s", attempt, maxRetries, connectionID))
			if attempt == maxRetries {
				hm.UpdateConnectionState(connectionID, StateFailed)
				return
			}
			continue
		}
		
		// Step 4: Restore subscriptions
		if len(subscribedSymbols) > 0 {
			slog.Info(fmt.Sprintf("🔄 [%d/%d] Restoring %d subscriptions for %s", attempt, maxRetries, len(subscribedSymbols), connectionID))
			// Use Background context so subscriptions survive recovery completion
			success, err := provider.SubscribeToSymbols(context.Background(), subscribedSymbols, []string{"Quote"})
			if err != nil || !success {
				slog.Error(fmt.Sprintf("❌ Subscription restore failed: %v", err))
			} else {
				slog.Info(fmt.Sprintf("✅ Restored %d subscriptions", len(subscribedSymbols)))
			}
		}
		
		// Success!
		slog.Info(fmt.Sprintf("✅ Recovery SUCCESS for %s (attempt %d/%d)", connectionID, attempt, maxRetries))
		hm.UpdateConnectionState(connectionID, StateConnected)
		
		hm.mutex.Lock()
		metrics.ReconnectionCount++
		hm.totalReconnections++
		hm.mutex.Unlock()
		
		return
	}
	
	// If we get here, all retries failed
	slog.Error(fmt.Sprintf("❌ Failed to recover %s after %d attempts", connectionID, maxRetries))
	hm.UpdateConnectionState(connectionID, StateFailed)
}

// GetHealthStatus returns comprehensive health status
func (hm *StreamingHealthManager) GetHealthStatus() map[string]interface{} {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	totalConnections := len(hm.connections)
	healthyConnections := 0
	
	connectionsStatus := make(map[string]interface{})
	for connID, metrics := range hm.connections {
		if metrics.State == StateConnected && !metrics.IsStale(hm.dataTimeout.Seconds()) {
			healthyConnections++
		}
		
		connectionsStatus[connID] = map[string]interface{}{
			"provider_name":        metrics.ProviderName,
			"state":                string(metrics.State),
			"uptime_seconds":       metrics.UptimeSeconds(),
			"time_since_last_data": metrics.TimeSinceLastData(),
			"is_stale":             metrics.IsStale(hm.dataTimeout.Seconds()),
			"message_count":        metrics.MessageCount,
			"error_count":          metrics.ErrorCount,
			"reconnection_count":   metrics.ReconnectionCount,
			"last_error":           metrics.LastError,
		}
	}
	
	var overallStatus string
	if totalConnections == 0 {
		overallStatus = "healthy"
	} else {
		healthRatio := float64(healthyConnections) / float64(totalConnections)
		switch {
		case healthRatio >= 0.8:
			overallStatus = "healthy"
		case healthRatio >= 0.5:
			overallStatus = "warning"
		case healthRatio > 0:
			overallStatus = "critical"
		default:
			overallStatus = "failed"
		}
	}
	
	return map[string]interface{}{
		"overall_status":       overallStatus,
		"monitoring_active":    hm.isMonitoring,
		"uptime_seconds":       time.Since(hm.startTime).Seconds(),
		"total_connections":    totalConnections,
		"healthy_connections":  healthyConnections,
		"total_reconnections":  hm.totalReconnections,
		"total_failures":       hm.totalFailures,
		"active_recoveries":    len(hm.recoveryTasks),
		"connections":          connectionsStatus,
		"timestamp":            time.Now().Unix(),
	}
}

// Global instance
var globalHealthManager *StreamingHealthManager
var healthManagerOnce sync.Once

// GetHealthManager returns the singleton health manager instance
func GetHealthManager() *StreamingHealthManager {
	healthManagerOnce.Do(func() {
		globalHealthManager = NewStreamingHealthManager()
	})
	return globalHealthManager
}
