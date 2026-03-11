package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupMetadataRouter creates a Gin test router with the metadata endpoint.
// GetIndicatorMetadata does not use the engine field, so a zero-value
// handler is safe.
func setupMetadataRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AutomationHandler{} // engine not needed for metadata
	r.GET("/automation/indicators/metadata", h.GetIndicatorMetadata)
	return r
}

// TestGetIndicatorMetadata_Status200 sends a GET and verifies 200 OK.
func TestGetIndicatorMetadata_Status200(t *testing.T) {
	r := setupMetadataRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/automation/indicators/metadata", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

// TestGetIndicatorMetadata_ResponseShape verifies the full JSON response
// structure returned by the metadata endpoint.
func TestGetIndicatorMetadata_ResponseShape(t *testing.T) {
	r := setupMetadataRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/automation/indicators/metadata", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	// 1. success: true
	success, ok := body["success"].(bool)
	if !ok || !success {
		t.Errorf("success: got %v, want true", body["success"])
	}

	// 2. message present
	msg, ok := body["message"].(string)
	if !ok || msg == "" {
		t.Errorf("message: got %v, want non-empty string", body["message"])
	}

	// 3. data object
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data: expected object, got %T", body["data"])
	}

	// 4. data.total == 17
	total, ok := data["total"].(float64) // JSON numbers are float64
	if !ok || int(total) != 17 {
		t.Errorf("data.total: got %v, want 17", data["total"])
	}

	// 5. data.indicators is an array with 17 items
	indicators, ok := data["indicators"].([]interface{})
	if !ok {
		t.Fatalf("data.indicators: expected array, got %T", data["indicators"])
	}
	if len(indicators) != 17 {
		t.Fatalf("data.indicators length: got %d, want 17", len(indicators))
	}

	// 6. Verify total matches array length
	if int(total) != len(indicators) {
		t.Errorf("total (%d) != len(indicators) (%d)", int(total), len(indicators))
	}
}

// requiredIndicatorKeys are the JSON keys every indicator object must have.
var requiredIndicatorKeys = []string{
	"type", "label", "description", "category",
	"params", "value_range", "needs_symbol",
}

// TestGetIndicatorMetadata_RequiredKeys checks that every indicator object
// contains all required JSON keys.
func TestGetIndicatorMetadata_RequiredKeys(t *testing.T) {
	indicators := fetchIndicators(t)

	for i, raw := range indicators {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("indicators[%d]: expected object, got %T", i, raw)
		}
		for _, key := range requiredIndicatorKeys {
			if _, exists := obj[key]; !exists {
				t.Errorf("indicators[%d] (type=%v): missing key %q", i, obj["type"], key)
			}
		}
	}
}

// TestGetIndicatorMetadata_VIXParamsEmptyArray verifies the params field
// for VIX is an empty JSON array [] and not null.
func TestGetIndicatorMetadata_VIXParamsEmptyArray(t *testing.T) {
	indicators := fetchIndicators(t)

	// Find VIX entry.
	var vix map[string]interface{}
	for _, raw := range indicators {
		obj := raw.(map[string]interface{})
		if obj["type"] == "vix" {
			vix = obj
			break
		}
	}
	if vix == nil {
		t.Fatal("VIX indicator not found in response")
	}

	// params must be an array (not null).
	params, ok := vix["params"].([]interface{})
	if !ok {
		t.Fatalf("VIX params: expected array, got %T (value: %v)", vix["params"], vix["params"])
	}
	if len(params) != 0 {
		t.Errorf("VIX params: expected empty array, got %d items", len(params))
	}
}

// TestGetIndicatorMetadata_RSIParams verifies the RSI indicator has exactly
// 1 param with key "period" and default_value 14.
func TestGetIndicatorMetadata_RSIParams(t *testing.T) {
	indicators := fetchIndicators(t)

	// Find RSI entry.
	var rsi map[string]interface{}
	for _, raw := range indicators {
		obj := raw.(map[string]interface{})
		if obj["type"] == "rsi" {
			rsi = obj
			break
		}
	}
	if rsi == nil {
		t.Fatal("RSI indicator not found in response")
	}

	params, ok := rsi["params"].([]interface{})
	if !ok {
		t.Fatalf("RSI params: expected array, got %T", rsi["params"])
	}
	if len(params) != 1 {
		t.Fatalf("RSI params: expected 1 item, got %d", len(params))
	}

	param := params[0].(map[string]interface{})
	if param["key"] != "period" {
		t.Errorf("RSI param key: got %v, want \"period\"", param["key"])
	}
	// JSON numbers are float64.
	if defaultVal, ok := param["default_value"].(float64); !ok || defaultVal != 14 {
		t.Errorf("RSI param default_value: got %v, want 14", param["default_value"])
	}

	// Verify the param itself has all required param keys.
	requiredParamKeys := []string{"key", "label", "default_value", "min", "max", "step", "type"}
	for _, key := range requiredParamKeys {
		if _, exists := param[key]; !exists {
			t.Errorf("RSI param: missing key %q", key)
		}
	}
}

// TestGetIndicatorMetadata_AllOriginalParamsEmpty verifies all 5 original
// indicators have empty params arrays (not null).
func TestGetIndicatorMetadata_AllOriginalParamsEmpty(t *testing.T) {
	indicators := fetchIndicators(t)

	originals := map[string]bool{
		"vix": true, "gap": true, "range": true,
		"trend": true, "calendar": true,
	}

	for _, raw := range indicators {
		obj := raw.(map[string]interface{})
		typeName, _ := obj["type"].(string)
		if !originals[typeName] {
			continue
		}
		params, ok := obj["params"].([]interface{})
		if !ok {
			t.Errorf("%s params: expected array, got %T (value: %v)", typeName, obj["params"], obj["params"])
			continue
		}
		if len(params) != 0 {
			t.Errorf("%s params: expected empty array, got %d items", typeName, len(params))
		}
	}
}

// TestGetIndicatorMetadata_ParamObjectKeys verifies every param object in
// every indicator has the required param-level keys.
func TestGetIndicatorMetadata_ParamObjectKeys(t *testing.T) {
	indicators := fetchIndicators(t)
	requiredParamKeys := []string{"key", "label", "default_value", "min", "max", "step", "type"}

	for _, raw := range indicators {
		obj := raw.(map[string]interface{})
		typeName, _ := obj["type"].(string)
		params, ok := obj["params"].([]interface{})
		if !ok {
			continue // covered by other tests
		}
		for j, pRaw := range params {
			p, ok := pRaw.(map[string]interface{})
			if !ok {
				t.Errorf("%s param[%d]: expected object, got %T", typeName, j, pRaw)
				continue
			}
			for _, key := range requiredParamKeys {
				if _, exists := p[key]; !exists {
					t.Errorf("%s param[%d]: missing key %q", typeName, j, key)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// fetchIndicators performs the GET request and returns the indicators array.
func fetchIndicators(t *testing.T) []interface{} {
	t.Helper()
	r := setupMetadataRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/automation/indicators/metadata", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data: expected object, got %T", body["data"])
	}

	indicators, ok := data["indicators"].([]interface{})
	if !ok {
		t.Fatalf("data.indicators: expected array, got %T", data["indicators"])
	}
	return indicators
}
