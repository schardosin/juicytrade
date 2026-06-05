package indicators

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
	"trade-backend-go/internal/providers/base"
)

// --- TestIsTransientError ---

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp 10.0.0.1:443: connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("read tcp 10.0.0.1:443: connection reset by peer"),
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "failed to connect",
			err:      errors.New("failed to connect to DXLink WebSocket: dial tcp ..."),
			expected: true,
		},
		{
			name:     "unexpected EOF",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "broken pipe (not transient)",
			err:      errors.New("broken pipe"),
			expected: false,
		},
		{
			name:     "VIX close price not available (not transient)",
			err:      errors.New("VIX close price not available in historical data"),
			expected: false,
		},
		{
			name:     "no VIX historical data (not transient)",
			err:      errors.New("no VIX historical data available"),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTransientError(tc.err)
			if got != tc.expected {
				t.Errorf("isTransientError(%v) = %v, want %v", tc.err, got, tc.expected)
			}
		})
	}
}

// --- TestFormatPassFail ---

func TestFormatPassFail(t *testing.T) {
	s := &Service{}
	if got := s.formatPassFail(true); got != "(PASS)" {
		t.Errorf("formatPassFail(true) = %q, want %q", got, "(PASS)")
	}
	if got := s.formatPassFail(false); got != "(FAIL)" {
		t.Errorf("formatPassFail(false) = %q, want %q", got, "(FAIL)")
	}
}

// --- Mock provider for retry/fallback tests ---

// mockProvider implements base.Provider for testing. Only GetHistoricalBars is
// functional; all other methods return nil/zero values.
type mockProvider struct {
	base.Provider // Embed to satisfy interface; unused methods will panic if called.

	// getHistoricalBarsFn is called by GetHistoricalBars. Tests set this to control behavior.
	getHistoricalBarsFn func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error)

	calls int // Track number of calls to GetHistoricalBars
	mu    sync.Mutex
}

func (m *mockProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	m.mu.Lock()
	m.calls++
	callNum := m.calls
	m.mu.Unlock()
	_ = callNum
	if m.getHistoricalBarsFn != nil {
		return m.getHistoricalBarsFn(ctx, symbol, timeframe, startDate, endDate, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockProvider) LookupSymbols(ctx context.Context, query string) ([]*models.SymbolSearchResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) PlaceOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) PlaceMultiLegOrder(ctx context.Context, orderData map[string]interface{}) (*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *mockProvider) CancelOrder(ctx context.Context, orderID string) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockProvider) StartAccountStream(ctx context.Context) error {
	return errors.New("not implemented")
}

func (m *mockProvider) StopAccountStream() {}

func (m *mockProvider) SetOrderEventCallback(callback func(*models.OrderEvent)) {}

func (m *mockProvider) IsAccountStreamConnected() bool { return false }

func (m *mockProvider) GetSubscribedSymbols() map[string]bool { return nil }

func (m *mockProvider) IsStreamingConnected() bool { return false }

func (m *mockProvider) Ping(ctx context.Context) error { return nil }

func (m *mockProvider) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockProvider) GetName() string { return "mock" }

func (m *mockProvider) SetStreamingQueue(queue chan *models.MarketData) {}

func (m *mockProvider) SetStreamingCache(cache base.StreamingCache) {}

// getCalls returns the number of calls made to GetHistoricalBars.
func (m *mockProvider) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// newTestService creates a Service with a mock provider for testing.
func newTestService(mock *mockProvider) *Service {
	pm := providers.NewTestProviderManager("mock_test", mock)
	return &Service{
		providerManager: pm,
		cache:           make(map[string]*cachedResult),
		quoteCache:      make(map[string]*cachedQuote),
		quoteFetchLock:  make(map[string]*sync.Mutex),
	}
}

// --- TestGetVIXValueWithRetry ---

func TestGetVIXValueWithRetry(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				return []map[string]interface{}{{"close": 18.5}}, nil
			},
		}
		s := newTestService(mock)

		value, err := s.getVIXValueWithRetry(context.Background(), "VIX")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != 18.5 {
			t.Errorf("expected value 18.5, got %f", value)
		}
		if mock.getCalls() != 1 {
			t.Errorf("expected 1 call to GetHistoricalBars, got %d", mock.getCalls())
		}
	})

	t.Run("fails then succeeds on retry", func(t *testing.T) {
		callCount := 0
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				callCount++
				if callCount == 1 {
					return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
				}
				return []map[string]interface{}{{"close": 22.3}}, nil
			},
		}
		s := newTestService(mock)

		value, err := s.getVIXValueWithRetry(context.Background(), "VIX")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != 22.3 {
			t.Errorf("expected value 22.3, got %f", value)
		}
		if mock.getCalls() != 2 {
			t.Errorf("expected 2 calls to GetHistoricalBars, got %d", mock.getCalls())
		}
	})

	t.Run("non-transient error does not retry", func(t *testing.T) {
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				return nil, errors.New("no VIX historical data available")
			},
		}
		s := newTestService(mock)

		_, err := s.getVIXValueWithRetry(context.Background(), "VIX")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if mock.getCalls() != 1 {
			t.Errorf("expected 1 call (no retry for non-transient), got %d", mock.getCalls())
		}
	})

	t.Run("all attempts fail with transient error", func(t *testing.T) {
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
			},
		}
		s := newTestService(mock)

		_, err := s.getVIXValueWithRetry(context.Background(), "VIX")
		if err == nil {
			t.Fatal("expected error after all retries, got nil")
		}
		expectedMsg := fmt.Sprintf("VIX evaluation failed after %d attempts", vixMaxRetries+1)
		if got := err.Error(); !contains(got, expectedMsg) {
			t.Errorf("error message %q does not contain %q", got, expectedMsg)
		}
		// Should have attempted 3 times (1 initial + 2 retries)
		if mock.getCalls() != vixMaxRetries+1 {
			t.Errorf("expected %d calls, got %d", vixMaxRetries+1, mock.getCalls())
		}
	})
}

// --- TestVIXCacheFallback ---

func TestVIXCacheFallback(t *testing.T) {
	t.Run("cache within TTL used as fallback", func(t *testing.T) {
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
			},
		}
		s := newTestService(mock)

		// Seed cache with a value from 2 minutes ago (within 5-min TTL)
		configID := "auto_123"
		indicatorID := "ind_vix_1"
		s.cache[cacheKey(configID, indicatorID)] = &cachedResult{
			Value:     19.5,
			Timestamp: time.Now().Add(-2 * time.Minute),
		}

		config := types.IndicatorConfig{
			ID:        indicatorID,
			Type:      types.IndicatorVIX,
			Symbol:    "VIX",
			Enabled:   true,
			Threshold: 30.0,
			Operator:  types.OperatorLessThan,
		}

		result := s.EvaluateIndicator(context.Background(), configID, config)

		if result.Stale {
			t.Error("expected Stale=false (cache fallback should not be stale)")
		}
		if result.Value != 19.5 {
			t.Errorf("expected Value=19.5 from cache, got %f", result.Value)
		}
		if result.Error != "" {
			t.Errorf("expected Error to be empty, got %q", result.Error)
		}
		if !result.Pass {
			t.Error("expected Pass=true (19.5 < 30.0)")
		}
	})

	t.Run("cache older than TTL falls through to stale", func(t *testing.T) {
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
			},
		}
		s := newTestService(mock)

		// Seed cache with a value from 10 minutes ago (beyond 5-min TTL)
		configID := "auto_123"
		indicatorID := "ind_vix_1"
		s.cache[cacheKey(configID, indicatorID)] = &cachedResult{
			Value:     19.5,
			Timestamp: time.Now().Add(-10 * time.Minute),
		}

		config := types.IndicatorConfig{
			ID:        indicatorID,
			Type:      types.IndicatorVIX,
			Symbol:    "VIX",
			Enabled:   true,
			Threshold: 30.0,
			Operator:  types.OperatorLessThan,
		}

		result := s.EvaluateIndicator(context.Background(), configID, config)

		if !result.Stale {
			t.Error("expected Stale=true (cache too old, not within fallback TTL)")
		}
	})

	t.Run("no cache exists falls through to stale", func(t *testing.T) {
		mock := &mockProvider{
			getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
				return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
			},
		}
		s := newTestService(mock)

		configID := "auto_123"
		indicatorID := "ind_vix_1"
		// No cache seeded

		config := types.IndicatorConfig{
			ID:        indicatorID,
			Type:      types.IndicatorVIX,
			Symbol:    "VIX",
			Enabled:   true,
			Threshold: 30.0,
			Operator:  types.OperatorLessThan,
		}

		result := s.EvaluateIndicator(context.Background(), configID, config)

		if !result.Stale {
			t.Error("expected Stale=true (no cache available)")
		}
		if result.Error == "" {
			t.Error("expected Error to be set when no cache available")
		}
	})
}

// contains is a simple helper for substring check in tests.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
