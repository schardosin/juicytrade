package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/utils"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// GAP G-4 (test-plan.md): HTTP handler validation returns 400.
//
// These tests exercise the real gin route wiring for CreateConfig / UpdateConfig
// and assert that an invalid Max Capital configuration is rejected with HTTP 400
// and success:false in the response body.
//
// Handler construction strategy (differs by route because of when the engine is used):
//
//   - CreateConfig runs validateTradeCapital (automation.go:143) BEFORE it ever
//     touches h.engine (GetStorage().Create()). So the POST 400 tests use a
//     zero-value &AutomationHandler{} and never dereference the engine.
//
//   - UpdateConfig calls h.engine.IsRunning(id) FIRST (automation.go:171), which
//     dereferences the engine, so a nil engine would panic. The PUT 400 tests
//     therefore use a real handler (NewAutomationHandler(nil)). This is still
//     side-effect-free on the working tree: the invalid config returns 400 at
//     validateTradeCapital (automation.go:190) BEFORE GetStorage().Update() is
//     reached, so no automations.json is written. Engine construction only
//     MkdirAll's the config dir (NewStorage / NewRuntimeStateStorage); the test
//     removes that dir in t.Cleanup if it created it, leaving `git status` clean.
//
// The accept (HTTP 200) path is intentionally NOT asserted at the HTTP layer:
// the success path calls h.engine.GetStorage().Create(), which WOULD persist an
// automations.json via utils.GlobalPathManager and dirty the working tree. The
// accept case is already covered — at the same validation seam — by the unit
// tests in automation_capital_test.go (TestValidateTradeCapital: "percent 60 ok",
// "fixed 5000 ok", boundary cases), so no working-tree pollution is required.
// ---------------------------------------------------------------------------

// setupCapitalRouter wires CreateConfig on a gin test router backed by a
// zero-value handler (engine untouched on the CreateConfig 400 path).
func setupCapitalRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AutomationHandler{} // engine not dereferenced on the CreateConfig reject path
	r.POST("/automation/configs", h.CreateConfig)
	return r
}

// setupCapitalRouterWithEngine wires UpdateConfig on a gin test router backed by
// a real handler so h.engine.IsRunning does not panic. It is side-effect-free on
// the 400 path (validation rejects before any storage write). Any config dir the
// engine's MkdirAll creates is removed via t.Cleanup so the working tree stays
// clean. If the engine cannot be constructed (e.g. read-only env), the test is
// skipped rather than failing.
func setupCapitalRouterWithEngine(t *testing.T) *gin.Engine {
	t.Helper()

	// Record whether the config dir already exists so we only clean up what we create.
	configDir := utils.GlobalPathManager.ConfigDir()
	_, dirExistedErr := os.Stat(configDir)
	dirPreexisted := dirExistedErr == nil

	h, err := NewAutomationHandler(nil)
	if err != nil {
		t.Skipf("cannot construct AutomationHandler in this environment: %v", err)
	}

	t.Cleanup(func() {
		// Only the config directory may have been created (MkdirAll) during
		// engine construction; the 400 path writes no files. Remove it iff we
		// created it and it is still empty, to avoid clobbering real data.
		if dirPreexisted {
			return
		}
		if entries, readErr := os.ReadDir(configDir); readErr == nil && len(entries) == 0 {
			_ = os.Remove(configDir)
		}
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/automation/configs/:id", h.UpdateConfig)
	return r
}

// baseConfig returns a minimally-valid AutomationConfig (Name/Symbol/Strategy set)
// so the request passes the earlier required-field / strategy-default guards and
// reaches validateTradeCapital with the caller-supplied TradeConfig intact.
func baseConfig(tc types.TradeConfiguration) types.AutomationConfig {
	return types.AutomationConfig{
		Name:        "HTTP Capital Test",
		Symbol:      "SPX",
		TradeConfig: tc,
	}
}

// postConfig marshals cfg and performs POST /automation/configs.
func postConfig(t *testing.T, r *gin.Engine, cfg types.AutomationConfig) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/automation/configs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// putConfig marshals cfg and performs PUT /automation/configs/{id}.
func putConfig(t *testing.T, r *gin.Engine, id string, cfg types.AutomationConfig) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/automation/configs/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// assertRejected400 asserts an HTTP 400 with a JSON body carrying success:false.
func assertRejected400(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v (body: %s)", err, w.Body.String())
	}
	success, ok := body["success"].(bool)
	if !ok {
		t.Fatalf("success: missing or wrong type in body %v", body)
	}
	if success {
		t.Errorf("success: got true, want false (body: %s)", w.Body.String())
	}
}

// ---- CreateConfig (POST) 400 rejection cases ----

func TestCreateConfig_PercentZeroRejected(t *testing.T) {
	r := setupCapitalRouter()
	cfg := baseConfig(types.TradeConfiguration{
		Strategy:          types.StrategyPutSpread,
		Width:             20,
		MaxCapitalMode:    types.MaxCapitalModePercent,
		MaxCapitalPercent: 0,
	})
	assertRejected400(t, postConfig(t, r, cfg))
}

func TestCreateConfig_PercentAboveMaxRejected(t *testing.T) {
	r := setupCapitalRouter()
	cfg := baseConfig(types.TradeConfiguration{
		Strategy:          types.StrategyPutSpread,
		Width:             20,
		MaxCapitalMode:    types.MaxCapitalModePercent,
		MaxCapitalPercent: 150,
	})
	assertRejected400(t, postConfig(t, r, cfg))
}

func TestCreateConfig_FixedBelowMinRejected(t *testing.T) {
	r := setupCapitalRouter()
	cfg := baseConfig(types.TradeConfiguration{
		Strategy:       types.StrategyPutSpread,
		Width:          20,
		MaxCapitalMode: types.MaxCapitalModeFixed,
		MaxCapital:     50,
	})
	assertRejected400(t, postConfig(t, r, cfg))
}

// ---- UpdateConfig (PUT) 400 rejection cases ----

func TestUpdateConfig_PercentZeroRejected(t *testing.T) {
	r := setupCapitalRouterWithEngine(t)
	cfg := baseConfig(types.TradeConfiguration{
		Strategy:          types.StrategyPutSpread,
		Width:             20,
		MaxCapitalMode:    types.MaxCapitalModePercent,
		MaxCapitalPercent: 0,
	})
	assertRejected400(t, putConfig(t, r, "cfg-1", cfg))
}

func TestUpdateConfig_PercentAboveMaxRejected(t *testing.T) {
	r := setupCapitalRouterWithEngine(t)
	cfg := baseConfig(types.TradeConfiguration{
		Strategy:          types.StrategyPutSpread,
		Width:             20,
		MaxCapitalMode:    types.MaxCapitalModePercent,
		MaxCapitalPercent: 150,
	})
	assertRejected400(t, putConfig(t, r, "cfg-1", cfg))
}

func TestUpdateConfig_FixedBelowMinRejected(t *testing.T) {
	r := setupCapitalRouterWithEngine(t)
	cfg := baseConfig(types.TradeConfiguration{
		Strategy:       types.StrategyPutSpread,
		Width:          20,
		MaxCapitalMode: types.MaxCapitalModeFixed,
		MaxCapital:     50,
	})
	assertRejected400(t, putConfig(t, r, "cfg-1", cfg))
}
