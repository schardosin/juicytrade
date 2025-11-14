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
func (cm *ConnectionMetrics) IsStale() bool {
	return cm.TimeSinceLastData() > 120 // 2 minutes
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

// StreamingHealthManager manages streaming connection health
type StreamingHealthManager struct {
	connections     map[string]*ConnectionMetrics
	circuitBreakers map[string]*CircuitBreaker
	providers       map[string]base.Provider
	recoveryTasks   map[string]context.CancelFunc
	isMonitoring    bool
	shutdownChan    chan struct{}
	mutex           sync.RWMutex
	
	// Configuration
	dataTimeout       time.Duration
	pingInterval      time.Duration
	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration
	
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
		shutdownChan:      make(chan struct{}),
		dataTimeout:       2 * time.Minute,
		pingInterval:      30 * time.Second,
		reconnectDelay:    5 * time.Second,
		maxReconnectDelay: 5 * time.Minute,
		startTime:         time.Now(),
	}
}

// RegisterProvider registers a provider for health monitoring
func (hm *StreamingHealthManager) RegisterProvider(providerName string, provider base.Provider) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	hm.providers[providerName] = provider
	slog.Info(fmt.Sprintf("📋 Registered provider %s for health monitoring", providerName))
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
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
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
	
	// Add startup grace period
	startupGracePeriod := 60 * time.Second
	connectionAge := currentTime.Sub(*metrics.ConnectedAt)
	
	// Check for stale connections (but only after grace period)
	if metrics.State == StateConnected &&
		connectionAge > startupGracePeriod &&
		metrics.IsStale() {
		
		slog.Warn(fmt.Sprintf("⚠️ Connection %s is stale (no data for %.1fs)", connectionID, metrics.TimeSinceLastData()))
		hm.UpdateConnectionState(connectionID, StateDegraded)
		
		// Trigger recovery if very stale
		if metrics.TimeSinceLastData() > 300 { // 5 minutes
			slog.Error(fmt.Sprintf("❌ Connection %s is very stale, triggering recovery", connectionID))
			hm.triggerRecovery(ctx, connectionID)
		}
	} else if metrics.State == StateFailed {
		hm.triggerRecovery(ctx, connectionID)
	}
}

// triggerRecovery triggers recovery for a failed connection
func (hm *StreamingHealthManager) triggerRecovery(ctx context.Context, connectionID string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	// Check if already recovering
	if _, exists := hm.recoveryTasks[connectionID]; exists {
		return
	}
	
	// Check circuit breaker
	cb, exists := hm.circuitBreakers[connectionID]
	if exists && !cb.CanExecute() {
		slog.Warn(fmt.Sprintf("🔴 Circuit breaker open for %s, skipping recovery", connectionID))
		return
	}
	
	slog.Info(fmt.Sprintf("🔄 Triggering recovery for %s", connectionID))
	hm.UpdateConnectionState(connectionID, StateRecovering)
	
	// Start recovery task
	recoveryCtx, cancel := context.WithCancel(ctx)
	hm.recoveryTasks[connectionID] = cancel
	
	go hm.recoverConnection(recoveryCtx, connectionID)
}

// recoverConnection recovers a failed connection
func (hm *StreamingHealthManager) recoverConnection(ctx context.Context, connectionID string) {
	defer func() {
		hm.mutex.Lock()
		delete(hm.recoveryTasks, connectionID)
		hm.mutex.Unlock()
	}()
	
	hm.mutex.RLock()
	metrics, exists := hm.connections[connectionID]
	if !exists {
		hm.mutex.RUnlock()
		return
	}
	
	provider, providerExists := hm.providers[metrics.ProviderName]
	hm.mutex.RUnlock()
	
	if !providerExists {
		slog.Error(fmt.Sprintf("❌ No provider found for %s", connectionID))
		return
	}
	
	// Calculate backoff delay with exponential backoff
	backoffMultiplier := int64(1)
	if metrics.ReconnectionCount < 10 { // Prevent overflow
		backoffMultiplier = 1 << uint(metrics.ReconnectionCount)
	} else {
		backoffMultiplier = 1024 // Cap at 2^10
	}
	delay := time.Duration(int64(hm.reconnectDelay) * backoffMultiplier)
	if delay > hm.maxReconnectDelay {
		delay = hm.maxReconnectDelay
	}
	
	slog.Info(fmt.Sprintf("⏳ Waiting %.1fs before reconnection attempt for %s", delay.Seconds(), connectionID))
	
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}
	
	// Attempt disconnection first (cleanup)
	if _, err := provider.DisconnectStreaming(ctx); err != nil {
		slog.Warn(fmt.Sprintf("⚠️ Error during disconnect for %s: %v", connectionID, err))
	}
	
	// Wait for cleanup
	time.Sleep(time.Second)
	
	// Attempt reconnection
	success, err := provider.ConnectStreaming(ctx)
	if err != nil || !success {
		slog.Error(fmt.Sprintf("❌ Failed to recover %s: %v", connectionID, err))
		hm.UpdateConnectionState(connectionID, StateFailed)
		return
	}
	
	slog.Info(fmt.Sprintf("✅ Successfully recovered %s", connectionID))
	hm.UpdateConnectionState(connectionID, StateConnected)
	
	hm.mutex.Lock()
	metrics.ReconnectionCount++
	hm.totalReconnections++
	hm.mutex.Unlock()
}

// GetHealthStatus returns comprehensive health status
func (hm *StreamingHealthManager) GetHealthStatus() map[string]interface{} {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	totalConnections := len(hm.connections)
	healthyConnections := 0
	
	connectionsStatus := make(map[string]interface{})
	for connID, metrics := range hm.connections {
		if metrics.State == StateConnected && !metrics.IsStale() {
			healthyConnections++
		}
		
		connectionsStatus[connID] = map[string]interface{}{
			"provider_name":        metrics.ProviderName,
			"state":                string(metrics.State),
			"uptime_seconds":       metrics.UptimeSeconds(),
			"time_since_last_data": metrics.TimeSinceLastData(),
			"is_stale":             metrics.IsStale(),
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
