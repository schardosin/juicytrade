package streaming

import (
	"context"
	"fmt"
	"testing"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers/base"
)

// MockProvider for testing
type MockProvider struct {
	base.BaseProviderImpl
	connected         bool
	subscribedSymbols map[string]bool
}

func (m *MockProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	fmt.Println("MockProvider: ConnectStreaming called")
	m.connected = true
	return true, nil
}

func (m *MockProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	fmt.Println("MockProvider: DisconnectStreaming called")
	m.connected = false
	return true, nil
}

func (m *MockProvider) IsStreamingConnected() bool {
	return m.connected
}

func (m *MockProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	for _, s := range symbols {
		m.subscribedSymbols[s] = true
	}
	return true, nil
}

func (m *MockProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	for _, s := range symbols {
		delete(m.subscribedSymbols, s)
	}
	return true, nil
}

func (m *MockProvider) GetSubscribedSymbols() map[string]bool {
	return m.subscribedSymbols
}

func (m *MockProvider) SetStreamingCache(cache base.StreamingCache) {}

// Stubs for other interface methods
func (m *MockProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	return nil, nil
}
func (m *MockProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	return nil, nil
}
func (m *MockProvider) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	return nil, nil
}
func (m *MockProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	return nil, nil
}
func (m *MockProvider) GetNextMarketDate(ctx context.Context) (string, error) { return "", nil }
func (m *MockProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	return nil, nil
}
func (m *MockProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockProvider) GetPositions(ctx context.Context) ([]*models.Position, error) { return nil, nil }
func (m *MockProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	return nil, nil
}
func (m *MockProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	return nil, nil
}
func (m *MockProvider) GetAccount(ctx context.Context) (*models.Account, error) { return nil, nil }
func (m *MockProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, nil
}
func (m *MockProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, nil
}
func (m *MockProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (m *MockProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	return true, nil
}
func (m *MockProvider) IsMarketOpen(ctx context.Context) (bool, error) { return true, nil }
func (m *MockProvider) GetLatestOptionQuotes(ctx context.Context, symbols []string) (map[string]map[string]interface{}, error) {
	return nil, nil
}
func (m *MockProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	return nil, nil
}

func TestStreamingHealthManager_Staleness(t *testing.T) {
	hm := NewStreamingHealthManager()
	// Set a short timeout for testing
	hm.dataTimeout = 100 * time.Millisecond

	provider := &MockProvider{subscribedSymbols: make(map[string]bool)}
	hm.RegisterProvider("mock", provider)

	connID := "mock_conn"
	hm.RegisterConnection(connID, "mock", "quote")
	hm.UpdateConnectionState(connID, StateConnected)

	// Record data
	hm.RecordDataReceived(connID)

	// Check immediately - should not be stale
	hm.mutex.RLock()
	metrics := hm.connections[connID]
	hm.mutex.RUnlock()

	if metrics.IsStale(hm.dataTimeout.Seconds()) {
		t.Errorf("Connection should not be stale immediately after data")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Check again - should be stale
	if !metrics.IsStale(hm.dataTimeout.Seconds()) {
		t.Errorf("Connection should be stale after timeout")
	}
}

func TestStreamingHealthManager_Recovery(t *testing.T) {
	hm := NewStreamingHealthManager()
	hm.reconnectDelay = 10 * time.Millisecond     // Fast reconnect for test
	hm.checkInterval = 10 * time.Millisecond      // Fast check for test
	hm.pingInterval = 10 * time.Millisecond       // Fast check for test
	hm.cleanupDelay = 10 * time.Millisecond       // Reduced cleanup delay
	hm.stabilizationDelay = 10 * time.Millisecond // Reduced stabilization delay
	hm.maxReconnectDelay = 50 * time.Millisecond  // Cap max delay

	provider := &MockProvider{subscribedSymbols: make(map[string]bool)}
	provider.connected = true
	hm.RegisterProvider("mock", provider)

	connID := "mock_conn"
	hm.RegisterConnection(connID, "mock", "quote")
	hm.UpdateConnectionState(connID, StateConnected)

	// Simulate failure
	hm.RecordError(connID, "connection lost")
	hm.UpdateConnectionState(connID, StateFailed)

	// Trigger recovery with forceImmediate=true for faster recovery
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hm.TriggerRecovery(ctx, connID, true)

	// Wait for recovery to complete - need longer wait due to async recovery
	// With forceImmediate=true, first attempt has 0 delay, but still needs
	// time for cleanup, connect, stabilization, and subscription restore
	time.Sleep(200 * time.Millisecond)

	hm.mutex.RLock()
	metrics := hm.connections[connID]
	hm.mutex.RUnlock()

	if metrics.ReconnectionCount == 0 {
		t.Errorf("Expected reconnection count > 0, got %d", metrics.ReconnectionCount)
	}

	if metrics.State != StateConnected {
		t.Errorf("Expected state Connected after recovery, got %s", metrics.State)
	}
}
