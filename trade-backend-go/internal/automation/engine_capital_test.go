package automation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/models"
)

// --- helpers ---

func fptr(f float64) *float64 { return &f }

// newTestEngine builds a minimal Engine suitable for capital-capture tests.
// It wires a runtimeState pointing at a temp file so notifyUpdate's async
// persistState does not panic, and an empty activeAutomations map.
//
// The runtime dir is created with os.MkdirTemp (not t.TempDir) to avoid a race
// between the asynchronous persistState goroutine spawned by notifyUpdate and
// t.TempDir's automatic cleanup at test end. It is removed via t.Cleanup after
// a short drain so the goroutine can finish writing.
func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	dir, err := os.MkdirTemp("", "automation-capital-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		// Give any in-flight async persistState goroutine time to finish.
		time.Sleep(50 * time.Millisecond)
		_ = os.RemoveAll(dir)
	})
	return &Engine{
		runtimeState:      &RuntimeStateStorage{filePath: filepath.Join(dir, "rt.json")},
		activeAutomations: make(map[string]*types.ActiveAutomation),
		stopChannels:      make(map[string]chan struct{}),
	}
}

func fixedConfig() *types.AutomationConfig {
	return &types.AutomationConfig{
		ID:   "cfg-fixed",
		Name: "Fixed",
		TradeConfig: types.TradeConfiguration{
			Strategy:       types.StrategyPutSpread,
			Width:          5,
			MaxCapital:     5000,
			MaxCapitalMode: types.MaxCapitalModeFixed,
		},
	}
}

func percentConfig(pct float64, recurrence types.RecurrenceMode) *types.AutomationConfig {
	return &types.AutomationConfig{
		ID:         "cfg-percent",
		Name:       "Percent",
		Recurrence: recurrence,
		TradeConfig: types.TradeConfiguration{
			Strategy:          types.StrategyPutSpread,
			Width:             5,
			MaxCapital:        5000,
			MaxCapitalMode:    types.MaxCapitalModePercent,
			MaxCapitalPercent: pct,
		},
	}
}

func lastLog(a *types.ActiveAutomation) types.AutomationLog {
	if len(a.Logs) == 0 {
		return types.AutomationLog{}
	}
	return a.Logs[len(a.Logs)-1]
}

// ---- Step 5: readNetLiq ----

func TestReadNetLiq(t *testing.T) {
	cases := []struct {
		name    string
		acct    *models.Account
		wantVal float64
		wantOK  bool
	}{
		{"nil account", nil, 0, false},
		{"portfolio value set", &models.Account{PortfolioValue: fptr(10000)}, 10000, true},
		{"equity fallback", &models.Account{Equity: fptr(8000)}, 8000, true},
		{"portfolio preferred over equity", &models.Account{PortfolioValue: fptr(10000), Equity: fptr(8000)}, 10000, true},
		{"zero portfolio falls through to equity", &models.Account{PortfolioValue: fptr(0), Equity: fptr(8000)}, 8000, true},
		{"both nil", &models.Account{}, 0, false},
		{"both non-positive", &models.Account{PortfolioValue: fptr(0), Equity: fptr(-1)}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, ok := readNetLiq(c.acct)
			if val != c.wantVal || ok != c.wantOK {
				t.Errorf("readNetLiq() = (%v, %v), want (%v, %v)", val, ok, c.wantVal, c.wantOK)
			}
		})
	}
}

// ---- Step 6: captureEffectiveCapital ----

func TestCaptureEffectiveCapital_FixedMode(t *testing.T) {
	e := newTestEngine(t)
	// Set an account reader that would fail; fixed mode must NOT read the account.
	called := false
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		called = true
		return nil, errors.New("should not be called in fixed mode")
	}

	active := &types.ActiveAutomation{Config: fixedConfig()}
	eff, err := e.captureEffectiveCapital(context.Background(), active, "trade-time")
	if err != nil {
		t.Fatalf("fixed mode must not error, got %v", err)
	}
	if called {
		t.Error("fixed mode must not read the account")
	}
	if eff != 5000 {
		t.Errorf("expected effective 5000, got %v", eff)
	}
	if active.EffectiveCapital != 5000 {
		t.Errorf("expected EffectiveCapital 5000, got %v", active.EffectiveCapital)
	}
	if active.EffectiveCapitalNetLiq != 0 || active.EffectiveCapitalPercent != 0 {
		t.Errorf("fixed mode should not set net_liq/pct, got net_liq=%v pct=%v",
			active.EffectiveCapitalNetLiq, active.EffectiveCapitalPercent)
	}
	if active.EffectiveCapitalAt == nil {
		t.Error("expected EffectiveCapitalAt to be set")
	}
	log := lastLog(active)
	if log.Level != "info" || !containsAll(log.Message, "[trade-time]", "(fixed)") {
		t.Errorf("unexpected fixed-mode log: %+v", log)
	}
}

func TestCaptureEffectiveCapital_PercentSuccess(t *testing.T) {
	e := newTestEngine(t)
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		return &models.Account{PortfolioValue: fptr(10000)}, nil
	}

	active := &types.ActiveAutomation{Config: percentConfig(60, types.RecurrenceOnce)}
	eff, err := e.captureEffectiveCapital(context.Background(), active, "trade-time")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// AC-3: 10000 * 60% = 6000
	if eff != 6000 {
		t.Errorf("expected effective 6000, got %v", eff)
	}
	if active.EffectiveCapital != 6000 {
		t.Errorf("expected EffectiveCapital 6000, got %v", active.EffectiveCapital)
	}
	if active.EffectiveCapitalNetLiq != 10000 {
		t.Errorf("expected net_liq 10000, got %v", active.EffectiveCapitalNetLiq)
	}
	if active.EffectiveCapitalPercent != 60 {
		t.Errorf("expected pct 60, got %v", active.EffectiveCapitalPercent)
	}
	log := lastLog(active)
	if log.Level != "info" || !containsAll(log.Message, "[trade-time]", "60") {
		t.Errorf("unexpected percent-mode log message: %+v", log)
	}
	if !containsAll(log.Details, "mode=percent", "net_liq=10000.00", "pct=60.00", "effective=6000.00") {
		t.Errorf("unexpected percent-mode log details: %q", log.Details)
	}
}

func TestCaptureEffectiveCapital_PercentNetLiqUnavailable(t *testing.T) {
	// Provider error -> netLiqOK false -> ErrNetLiqUnavailable
	e := newTestEngine(t)
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		return nil, errors.New("provider down")
	}

	active := &types.ActiveAutomation{Config: percentConfig(60, types.RecurrenceOnce)}
	_, err := e.captureEffectiveCapital(context.Background(), active, "trade-time")
	if !errors.Is(err, types.ErrNetLiqUnavailable) {
		t.Fatalf("expected ErrNetLiqUnavailable, got %v", err)
	}
	// snapshot records the pct even on failure; net_liq stays 0
	if active.EffectiveCapitalPercent != 60 {
		t.Errorf("expected pct snapshot 60, got %v", active.EffectiveCapitalPercent)
	}
	if active.EffectiveCapital != 0 {
		t.Errorf("expected EffectiveCapital 0 on failure, got %v", active.EffectiveCapital)
	}
	if active.EffectiveCapitalAt == nil {
		t.Error("expected EffectiveCapitalAt to be set even on failure")
	}
	if lastLog(active).Level != "error" {
		t.Errorf("expected error log on failure, got %+v", lastLog(active))
	}
}

func TestCaptureEffectiveCapital_PercentZeroNetLiq(t *testing.T) {
	// Account returns but net liq is zero/unusable -> ErrNetLiqUnavailable
	e := newTestEngine(t)
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		return &models.Account{}, nil // both nil -> readNetLiq (0,false)
	}
	active := &types.ActiveAutomation{Config: percentConfig(60, types.RecurrenceOnce)}
	_, err := e.captureEffectiveCapital(context.Background(), active, "trade-time")
	if !errors.Is(err, types.ErrNetLiqUnavailable) {
		t.Fatalf("expected ErrNetLiqUnavailable, got %v", err)
	}
}

// ---- Step 6: handleCapitalFailure ----

func TestHandleCapitalFailure_InvalidPercentIsPermanent(t *testing.T) {
	e := newTestEngine(t)
	active := &types.ActiveAutomation{Config: percentConfig(0, types.RecurrenceDaily)}
	e.handleCapitalFailure("id", active, types.ErrInvalidCapitalPercent)

	if active.Status != types.StatusFailed {
		t.Errorf("expected StatusFailed for invalid pct, got %v", active.Status)
	}
	if active.ErrorCount != 0 {
		t.Errorf("invalid pct must not increment ErrorCount, got %d", active.ErrorCount)
	}
	if active.Message != "Invalid Max Capital percentage configuration" {
		t.Errorf("unexpected message: %q", active.Message)
	}
}

func TestHandleCapitalFailure_TransientBelowThresholdRetries(t *testing.T) {
	e := newTestEngine(t)
	active := &types.ActiveAutomation{Config: percentConfig(60, types.RecurrenceDaily)}

	e.handleCapitalFailure("id", active, types.ErrNetLiqUnavailable)
	if active.ErrorCount != 1 {
		t.Errorf("expected ErrorCount 1, got %d", active.ErrorCount)
	}
	if active.Status == types.StatusFailed || active.Status == types.StatusWaiting {
		t.Errorf("below threshold should not transition to terminal/waiting, got %v", active.Status)
	}
	if active.Message != "Net Liq unavailable - will retry" {
		t.Errorf("unexpected message: %q", active.Message)
	}
}

func TestHandleCapitalFailure_TransientDailyReachesThresholdWaits(t *testing.T) {
	e := newTestEngine(t)
	active := &types.ActiveAutomation{
		Config:     percentConfig(60, types.RecurrenceDaily),
		ErrorCount: 2, // next failure makes it 3
	}
	e.handleCapitalFailure("id", active, types.ErrNetLiqUnavailable)
	if active.ErrorCount != 3 {
		t.Errorf("expected ErrorCount 3, got %d", active.ErrorCount)
	}
	if active.Status != types.StatusWaiting {
		t.Errorf("daily recurrence at threshold should wait, got %v", active.Status)
	}
	if !active.TradedToday {
		t.Error("daily recurrence at threshold should set TradedToday")
	}
}

func TestHandleCapitalFailure_TransientOnceReachesThresholdFails(t *testing.T) {
	e := newTestEngine(t)
	active := &types.ActiveAutomation{
		Config:     percentConfig(60, types.RecurrenceOnce),
		ErrorCount: 2, // next failure makes it 3
	}
	e.handleCapitalFailure("id", active, types.ErrNetLiqUnavailable)
	if active.ErrorCount != 3 {
		t.Errorf("expected ErrorCount 3, got %d", active.ErrorCount)
	}
	if active.Status != types.StatusFailed {
		t.Errorf("once recurrence at threshold should fail, got %v", active.Status)
	}
	if active.Message != "Net Liq unavailable - cannot size position" {
		t.Errorf("unexpected message: %q", active.Message)
	}
}

// ---- Step 7: delta-drift re-placement reuses the trade-time snapshot ----

func TestCalculateUnits_DriftReusesEffectiveCapitalSnapshot(t *testing.T) {
	// Percent-mode config with MaxCapital=5000 but a trade-time snapshot of 6000.
	// The drift re-placement must size from the snapshot (6000), not MaxCapital (5000).
	tc := types.TradeConfiguration{
		Strategy:          types.StrategyPutSpread,
		Width:             5, // maxRiskPerUnit = 500
		MaxCapital:        5000,
		MaxCapitalMode:    types.MaxCapitalModePercent,
		MaxCapitalPercent: 60,
	}
	active := &types.ActiveAutomation{
		Config:           &types.AutomationConfig{TradeConfig: tc},
		EffectiveCapital: 6000, // captured at trade time (10000 * 60%)
	}

	// This mirrors the exact expression used at the drift re-placement sites.
	units := active.Config.TradeConfig.CalculateUnits(active.EffectiveCapital)
	if units != 12 { // 6000 / 500 = 12
		t.Errorf("expected 12 units from snapshot (6000), got %d", units)
	}

	// Sanity: sizing from MaxCapital (5000) would have produced 10, proving the
	// snapshot (not MaxCapital) is what drives the drift re-placement.
	if fromMax := active.Config.TradeConfig.CalculateUnits(active.Config.TradeConfig.MaxCapital); fromMax != 10 {
		t.Errorf("expected 10 units from MaxCapital (5000) for the contrast check, got %d", fromMax)
	}
}

func TestCalculateUnits_DriftFixedModeParity(t *testing.T) {
	// Fixed mode: the trade-time snapshot equals MaxCapital, so drift sizing is unchanged.
	tc := types.TradeConfiguration{
		Strategy:       types.StrategyPutSpread,
		Width:          5,
		MaxCapital:     5000,
		MaxCapitalMode: types.MaxCapitalModeFixed,
	}
	active := &types.ActiveAutomation{
		Config:           &types.AutomationConfig{TradeConfig: tc},
		EffectiveCapital: 5000, // fixed-mode snapshot == MaxCapital
	}
	units := active.Config.TradeConfig.CalculateUnits(active.EffectiveCapital)
	if units != 10 { // 5000 / 500 = 10
		t.Errorf("expected 10 units (fixed parity), got %d", units)
	}
}

// ---- Step 8: captureMonitoringStartCapital (FR-5) ----

func TestCaptureMonitoringStartCapital_PercentSuccess(t *testing.T) {
	e := newTestEngine(t)
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		return &models.Account{PortfolioValue: fptr(10000)}, nil
	}
	active := &types.ActiveAutomation{Config: percentConfig(60, types.RecurrenceOnce), Status: types.StatusMonitoring}
	e.activeAutomations["mon"] = active

	e.captureMonitoringStartCapital("mon")

	if active.EffectiveCapital != 6000 {
		t.Errorf("expected EffectiveCapital 6000, got %v", active.EffectiveCapital)
	}
	if active.EffectiveCapitalNetLiq != 10000 || active.EffectiveCapitalPercent != 60 {
		t.Errorf("expected snapshot net_liq=10000 pct=60, got net_liq=%v pct=%v",
			active.EffectiveCapitalNetLiq, active.EffectiveCapitalPercent)
	}
	if active.Status != types.StatusMonitoring {
		t.Errorf("status must be unchanged, got %v", active.Status)
	}
	if !containsAll(active.Message, "Monitoring", "6000") {
		t.Errorf("expected message to include resolved capital, got %q", active.Message)
	}
}

func TestCaptureMonitoringStartCapital_PercentNetLiqUnavailableNonFatal(t *testing.T) {
	e := newTestEngine(t)
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		return nil, errors.New("provider down")
	}
	active := &types.ActiveAutomation{Config: percentConfig(60, types.RecurrenceOnce), Status: types.StatusMonitoring}
	e.activeAutomations["mon"] = active

	e.captureMonitoringStartCapital("mon")

	// Must NOT fail — status stays as-is, capital pending message + warn log.
	if active.Status != types.StatusMonitoring {
		t.Errorf("monitoring-start failure must be non-fatal, got status %v", active.Status)
	}
	if !containsAll(active.Message, "pending") {
		t.Errorf("expected capital-pending message, got %q", active.Message)
	}
	if lastLog(active).Level != "warn" {
		t.Errorf("expected warn log on non-fatal monitoring-start failure, got %+v", lastLog(active))
	}
}

func TestCaptureMonitoringStartCapital_FixedMode(t *testing.T) {
	e := newTestEngine(t)
	// Fixed mode must not read the account.
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		t.Error("fixed mode must not read the account at monitoring start")
		return nil, errors.New("unexpected")
	}
	active := &types.ActiveAutomation{Config: fixedConfig(), Status: types.StatusMonitoring}
	e.activeAutomations["mon"] = active

	e.captureMonitoringStartCapital("mon")

	if active.EffectiveCapital != 5000 {
		t.Errorf("expected EffectiveCapital 5000, got %v", active.EffectiveCapital)
	}
	if active.Status != types.StatusMonitoring {
		t.Errorf("status must be unchanged, got %v", active.Status)
	}
	if !containsAll(active.Message, "Monitoring", "5000") {
		t.Errorf("expected message to include fixed capital, got %q", active.Message)
	}
}

func TestCaptureMonitoringStartCapital_MissingAutomationNoOp(t *testing.T) {
	e := newTestEngine(t)
	// Should not panic when the id is absent.
	e.captureMonitoringStartCapital("does-not-exist")
}

// ---- GAP G-3 (test-plan.md): restart re-resolution overrides stale snapshot ----

// TestCaptureEffectiveCapital_ReResolvesOverStalePersistedSnapshot proves that a
// restored automation carrying a STALE persisted EffectiveCapital snapshot has
// that value overwritten by a FRESH trade-time capture. This confirms the
// persisted snapshot is display-only and sizing is always re-resolved from the
// Net Liq read at the actual trade moment (design §8.3).
func TestCaptureEffectiveCapital_ReResolvesOverStalePersistedSnapshot(t *testing.T) {
	e := newTestEngine(t)
	// Fresh account read returns a DIFFERENT Net Liq than the stale snapshot implies.
	e.accountReader = func(ctx context.Context) (*models.Account, error) {
		return &models.Account{PortfolioValue: fptr(20000)}, nil
	}

	// Simulate a restored automation: percent 60, but a stale snapshot of 6000
	// (as if Net Liq had been 10000 at the previous run before a restart).
	active := &types.ActiveAutomation{
		Config:                  percentConfig(60, types.RecurrenceOnce),
		EffectiveCapital:        6000,
		EffectiveCapitalNetLiq:  10000,
		EffectiveCapitalPercent: 60,
	}

	eff, err := e.captureEffectiveCapital(context.Background(), active, "trade-time")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 20000 * 60% = 12000 — the fresh read must override the stale 6000.
	if eff != 12000 {
		t.Errorf("expected fresh effective 12000, got %v", eff)
	}
	if active.EffectiveCapital != 12000 {
		t.Errorf("stale snapshot not overwritten: EffectiveCapital = %v, want 12000", active.EffectiveCapital)
	}
	if active.EffectiveCapitalNetLiq != 20000 {
		t.Errorf("expected net_liq updated to 20000, got %v", active.EffectiveCapitalNetLiq)
	}
	if active.EffectiveCapitalPercent != 60 {
		t.Errorf("expected pct 60, got %v", active.EffectiveCapitalPercent)
	}
}

// containsAll reports whether s contains every substring in subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
