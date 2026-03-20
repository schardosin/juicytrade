package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// 2.1 GetEffectiveIndicatorGroups — Edge Cases
// =============================================================================

// 2.1.1: Empty slice IndicatorGroups vs populated Indicators — should fallback.
// The distinction between nil vs empty slice matters for JSON deserialization.
func TestQA_GetEffectiveIndicatorGroups_EmptyGroupsNonEmptyIndicators(t *testing.T) {
	config := &AutomationConfig{
		IndicatorGroups: []IndicatorGroup{}, // empty slice, NOT nil
		Indicators: []IndicatorConfig{
			{ID: "ind_1", Type: IndicatorVIX, Enabled: true, Operator: OperatorLessThan, Threshold: 20},
			{ID: "ind_2", Type: IndicatorGap, Enabled: true, Operator: OperatorLessThan, Threshold: 1.5},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()

	// Empty slice has len 0, so fallback to legacy Indicators
	if len(groups) != 1 {
		t.Fatalf("expected 1 fallback group, got %d", len(groups))
	}
	if groups[0].ID != "default" {
		t.Errorf("expected fallback group ID 'default', got %q", groups[0].ID)
	}
	if groups[0].Name != "Default" {
		t.Errorf("expected fallback group name 'Default', got %q", groups[0].Name)
	}
	if len(groups[0].Indicators) != 2 {
		t.Errorf("expected 2 indicators in fallback group, got %d", len(groups[0].Indicators))
	}
	if groups[0].Indicators[0].ID != "ind_1" {
		t.Errorf("expected first indicator ID 'ind_1', got %q", groups[0].Indicators[0].ID)
	}
	if groups[0].Indicators[1].ID != "ind_2" {
		t.Errorf("expected second indicator ID 'ind_2', got %q", groups[0].Indicators[1].ID)
	}
}

// 2.1.2: Both fields are nil (not initialized). Returns empty slice without panic.
func TestQA_GetEffectiveIndicatorGroups_NilGroupsNilIndicators(t *testing.T) {
	config := &AutomationConfig{
		// Both Indicators and IndicatorGroups are nil (zero value)
	}

	groups := config.GetEffectiveIndicatorGroups()

	if groups == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

// 2.1.3: IndicatorGroups has one group with zero indicators. Returned as-is.
// (Vacuously true behavior is the evaluator's concern, not the helper's.)
func TestQA_GetEffectiveIndicatorGroups_GroupsWithEmptyIndicators(t *testing.T) {
	config := &AutomationConfig{
		IndicatorGroups: []IndicatorGroup{
			{ID: "grp_empty", Name: "Empty Group", Indicators: []IndicatorConfig{}},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ID != "grp_empty" {
		t.Errorf("expected group ID 'grp_empty', got %q", groups[0].ID)
	}
	if groups[0].Name != "Empty Group" {
		t.Errorf("expected group name 'Empty Group', got %q", groups[0].Name)
	}
	if len(groups[0].Indicators) != 0 {
		t.Errorf("expected 0 indicators in group, got %d", len(groups[0].Indicators))
	}
}

// 2.1.4: IndicatorGroups has 3 groups. Verify the returned slice preserves insertion order.
func TestQA_GetEffectiveIndicatorGroups_MultipleGroupsPreserveOrder(t *testing.T) {
	config := &AutomationConfig{
		IndicatorGroups: []IndicatorGroup{
			{ID: "grp_alpha", Name: "Alpha", Indicators: []IndicatorConfig{{ID: "ind_a", Type: IndicatorRSI}}},
			{ID: "grp_beta", Name: "Beta", Indicators: []IndicatorConfig{{ID: "ind_b", Type: IndicatorMACD}}},
			{ID: "grp_gamma", Name: "Gamma", Indicators: []IndicatorConfig{{ID: "ind_c", Type: IndicatorATR}}},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	expectedIDs := []string{"grp_alpha", "grp_beta", "grp_gamma"}
	expectedNames := []string{"Alpha", "Beta", "Gamma"}
	for i := 0; i < 3; i++ {
		if groups[i].ID != expectedIDs[i] {
			t.Errorf("group[%d]: expected ID %q, got %q", i, expectedIDs[i], groups[i].ID)
		}
		if groups[i].Name != expectedNames[i] {
			t.Errorf("group[%d]: expected Name %q, got %q", i, expectedNames[i], groups[i].Name)
		}
	}
}

// 2.1.5: When falling back to legacy, verify the synthetic group has exactly
// ID: "default" and Name: "Default". The frontend depends on these values.
func TestQA_GetEffectiveIndicatorGroups_LegacyFallbackIDAndName(t *testing.T) {
	config := &AutomationConfig{
		Indicators: []IndicatorConfig{
			{ID: "ind_legacy", Type: IndicatorVIX, Enabled: true, Operator: OperatorLessThan, Threshold: 25},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()

	if len(groups) != 1 {
		t.Fatalf("expected 1 fallback group, got %d", len(groups))
	}
	if groups[0].ID != "default" {
		t.Errorf("expected synthetic group ID to be exactly 'default', got %q", groups[0].ID)
	}
	if groups[0].Name != "Default" {
		t.Errorf("expected synthetic group Name to be exactly 'Default', got %q", groups[0].Name)
	}
}

// =============================================================================
// 2.2 JSON Serialization Round-Trip
// =============================================================================

// 2.2.1: AutomationConfig with 2 indicator groups, marshal/unmarshal, verify all fields survive.
func TestQA_AutomationConfig_JSONRoundTrip_WithGroups(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for JSON precision
	original := &AutomationConfig{
		ID:          "cfg_test",
		Name:        "Test Config",
		Description: "A test automation config",
		Symbol:      "SPX",
		Indicators:  []IndicatorConfig{}, // empty legacy
		IndicatorGroups: []IndicatorGroup{
			{
				ID:   "grp_1",
				Name: "Low Vol",
				Indicators: []IndicatorConfig{
					{
						ID:        "ind_vix",
						Type:      IndicatorVIX,
						Enabled:   true,
						Operator:  OperatorLessThan,
						Threshold: 20,
					},
					{
						ID:        "ind_rsi",
						Type:      IndicatorRSI,
						Enabled:   true,
						Operator:  OperatorGreaterThan,
						Threshold: 30,
						Symbol:    "SPY",
						Params:    map[string]float64{"period": 21},
					},
				},
			},
			{
				ID:   "grp_2",
				Name: "High Vol",
				Indicators: []IndicatorConfig{
					{
						ID:        "ind_vix2",
						Type:      IndicatorVIX,
						Enabled:   true,
						Operator:  OperatorGreaterThan,
						Threshold: 30,
					},
					{
						ID:        "ind_atr",
						Type:      IndicatorATR,
						Enabled:   false,
						Operator:  OperatorLessThan,
						Threshold: 5.0,
						Symbol:    "QQQ",
						Params:    map[string]float64{"period": 14},
					},
				},
			},
		},
		EntryTime:     "12:25",
		EntryTimezone: "America/New_York",
		Enabled:       true,
		Recurrence:    RecurrenceDaily,
		Created:       now,
		Updated:       now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored AutomationConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Top-level fields
	if restored.ID != original.ID {
		t.Errorf("ID: expected %q, got %q", original.ID, restored.ID)
	}
	if restored.Name != original.Name {
		t.Errorf("Name: expected %q, got %q", original.Name, restored.Name)
	}
	if restored.Symbol != original.Symbol {
		t.Errorf("Symbol: expected %q, got %q", original.Symbol, restored.Symbol)
	}
	if restored.Recurrence != original.Recurrence {
		t.Errorf("Recurrence: expected %q, got %q", original.Recurrence, restored.Recurrence)
	}

	// Groups
	if len(restored.IndicatorGroups) != 2 {
		t.Fatalf("expected 2 indicator groups, got %d", len(restored.IndicatorGroups))
	}

	// Group 1
	g1 := restored.IndicatorGroups[0]
	if g1.ID != "grp_1" {
		t.Errorf("group 1 ID: expected 'grp_1', got %q", g1.ID)
	}
	if g1.Name != "Low Vol" {
		t.Errorf("group 1 Name: expected 'Low Vol', got %q", g1.Name)
	}
	if len(g1.Indicators) != 2 {
		t.Fatalf("group 1: expected 2 indicators, got %d", len(g1.Indicators))
	}
	if g1.Indicators[0].ID != "ind_vix" || g1.Indicators[0].Type != IndicatorVIX {
		t.Errorf("group 1 indicator 0: expected ind_vix/vix, got %q/%q", g1.Indicators[0].ID, g1.Indicators[0].Type)
	}
	if g1.Indicators[1].Params["period"] != 21 {
		t.Errorf("group 1 indicator 1 params[period]: expected 21, got %v", g1.Indicators[1].Params["period"])
	}
	if g1.Indicators[1].Symbol != "SPY" {
		t.Errorf("group 1 indicator 1 symbol: expected 'SPY', got %q", g1.Indicators[1].Symbol)
	}

	// Group 2
	g2 := restored.IndicatorGroups[1]
	if g2.ID != "grp_2" {
		t.Errorf("group 2 ID: expected 'grp_2', got %q", g2.ID)
	}
	if g2.Indicators[1].Enabled != false {
		t.Errorf("group 2 indicator 1 Enabled: expected false, got true")
	}
	if g2.Indicators[1].Operator != OperatorLessThan {
		t.Errorf("group 2 indicator 1 Operator: expected 'lt', got %q", g2.Indicators[1].Operator)
	}
	if g2.Indicators[1].Threshold != 5.0 {
		t.Errorf("group 2 indicator 1 Threshold: expected 5.0, got %v", g2.Indicators[1].Threshold)
	}
}

// 2.2.2: Config with empty IndicatorGroups — test omitempty behavior.
func TestQA_AutomationConfig_JSONRoundTrip_EmptyGroups(t *testing.T) {
	config := &AutomationConfig{
		ID:              "cfg_empty",
		Name:            "Empty Groups",
		Symbol:          "SPX",
		Indicators:      []IndicatorConfig{},
		IndicatorGroups: []IndicatorGroup{},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// IndicatorGroups has `omitempty` tag — empty slice should be omitted from JSON
	jsonStr := string(data)
	if strings.Contains(jsonStr, "indicator_groups") {
		t.Errorf("expected 'indicator_groups' to be omitted for empty slice (omitempty), but found in JSON: %s", jsonStr)
	}

	// Now test round-trip with an explicit "indicator_groups": [] in JSON
	jsonWithEmptyGroups := `{"id":"cfg_empty","name":"Empty Groups","symbol":"SPX","indicators":[],"indicator_groups":[],"entry_time":"","entry_timezone":"","enabled":false,"recurrence":"","trade_config":{},"created":"0001-01-01T00:00:00Z","updated":"0001-01-01T00:00:00Z"}`

	var restored AutomationConfig
	if err := json.Unmarshal([]byte(jsonWithEmptyGroups), &restored); err != nil {
		t.Fatalf("unmarshal with explicit empty indicator_groups failed: %v", err)
	}

	// When JSON has "indicator_groups": [], Go unmarshals to an empty (non-nil) slice
	if restored.IndicatorGroups == nil {
		t.Error("expected IndicatorGroups to be non-nil empty slice after unmarshaling 'indicator_groups': [], got nil")
	}
	if len(restored.IndicatorGroups) != 0 {
		t.Errorf("expected 0 indicator groups, got %d", len(restored.IndicatorGroups))
	}
}

// 2.2.3: Legacy v1.0 JSON (has indicators, no indicator_groups key).
func TestQA_AutomationConfig_JSONRoundTrip_LegacyFormat(t *testing.T) {
	legacyJSON := `{
		"id": "cfg_legacy",
		"name": "Legacy Config",
		"symbol": "NDX",
		"indicators": [
			{"id": "ind_1", "type": "vix", "enabled": true, "operator": "lt", "threshold": 20},
			{"id": "ind_2", "type": "gap", "enabled": true, "operator": "lt", "threshold": 1.0, "symbol": "SPY"}
		],
		"entry_time": "12:25",
		"entry_timezone": "America/New_York",
		"enabled": true,
		"recurrence": "once",
		"trade_config": {},
		"created": "2025-01-01T00:00:00Z",
		"updated": "2025-01-01T00:00:00Z"
	}`

	var config AutomationConfig
	if err := json.Unmarshal([]byte(legacyJSON), &config); err != nil {
		t.Fatalf("unmarshal legacy JSON failed: %v", err)
	}

	// IndicatorGroups should be nil (key not present in JSON)
	if config.IndicatorGroups != nil {
		t.Errorf("expected IndicatorGroups to be nil for legacy JSON, got %v", config.IndicatorGroups)
	}

	// Indicators should be populated
	if len(config.Indicators) != 2 {
		t.Fatalf("expected 2 legacy indicators, got %d", len(config.Indicators))
	}

	// GetEffectiveIndicatorGroups should wrap into a Default group
	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 fallback group, got %d", len(groups))
	}
	if groups[0].ID != "default" {
		t.Errorf("expected fallback group ID 'default', got %q", groups[0].ID)
	}
	if groups[0].Name != "Default" {
		t.Errorf("expected fallback group name 'Default', got %q", groups[0].Name)
	}
	if len(groups[0].Indicators) != 2 {
		t.Errorf("expected 2 indicators in fallback group, got %d", len(groups[0].Indicators))
	}
	if groups[0].Indicators[1].Symbol != "SPY" {
		t.Errorf("expected second indicator symbol 'SPY', got %q", groups[0].Indicators[1].Symbol)
	}
}

// 2.2.4: GroupResult with nested IndicatorResult entries including Stale, Error, LastGoodValue.
func TestQA_GroupResult_JSONRoundTrip(t *testing.T) {
	lastGood := 18.5
	ts := time.Now().Truncate(time.Second)

	original := GroupResult{
		GroupID:   "grp_test",
		GroupName: "Test Group",
		Pass:      false,
		IndicatorResults: []IndicatorResult{
			{
				Type:      IndicatorVIX,
				Value:     22.5,
				Threshold: 20,
				Operator:  OperatorLessThan,
				Pass:      false,
				Enabled:   true,
				Stale:     false,
				Timestamp: ts,
				Details:   "Current VIX value",
			},
			{
				Type:          IndicatorRSI,
				Symbol:        "SPY",
				Value:         0,
				LastGoodValue: &lastGood,
				Stale:         true,
				Threshold:     30,
				Operator:      OperatorGreaterThan,
				Pass:          false,
				Enabled:       true,
				Timestamp:     ts,
				Error:         "data fetch timeout",
				ParamSummary:  "14",
			},
			{
				Type:      IndicatorATR,
				Symbol:    "QQQ",
				Value:     3.2,
				Threshold: 5.0,
				Operator:  OperatorLessThan,
				Pass:      true,
				Enabled:   true,
				Stale:     false,
				Timestamp: ts,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal GroupResult failed: %v", err)
	}

	var restored GroupResult
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal GroupResult failed: %v", err)
	}

	if restored.GroupID != "grp_test" {
		t.Errorf("GroupID: expected 'grp_test', got %q", restored.GroupID)
	}
	if restored.GroupName != "Test Group" {
		t.Errorf("GroupName: expected 'Test Group', got %q", restored.GroupName)
	}
	if restored.Pass != false {
		t.Error("Pass: expected false, got true")
	}
	if len(restored.IndicatorResults) != 3 {
		t.Fatalf("expected 3 indicator results, got %d", len(restored.IndicatorResults))
	}

	// Check stale indicator with LastGoodValue and Error
	staleResult := restored.IndicatorResults[1]
	if staleResult.Stale != true {
		t.Error("stale indicator: expected Stale=true, got false")
	}
	if staleResult.Error != "data fetch timeout" {
		t.Errorf("stale indicator Error: expected 'data fetch timeout', got %q", staleResult.Error)
	}
	if staleResult.LastGoodValue == nil {
		t.Fatal("stale indicator LastGoodValue: expected non-nil pointer")
	}
	if *staleResult.LastGoodValue != 18.5 {
		t.Errorf("stale indicator LastGoodValue: expected 18.5, got %v", *staleResult.LastGoodValue)
	}
	if staleResult.ParamSummary != "14" {
		t.Errorf("stale indicator ParamSummary: expected '14', got %q", staleResult.ParamSummary)
	}

	// Check non-stale indicator without optional fields
	freshResult := restored.IndicatorResults[2]
	if freshResult.LastGoodValue != nil {
		t.Errorf("fresh indicator: expected nil LastGoodValue, got %v", *freshResult.LastGoodValue)
	}
	if freshResult.Pass != true {
		t.Error("fresh indicator: expected Pass=true, got false")
	}
}

// 2.2.5: ActiveAutomation with both IndicatorResults and GroupResults populated.
func TestQA_ActiveAutomation_JSONRoundTrip_WithGroupResults(t *testing.T) {
	ts := time.Now().Truncate(time.Second)

	original := ActiveAutomation{
		Config: &AutomationConfig{
			ID:   "cfg_1",
			Name: "Test Automation",
		},
		Status: StatusEvaluating,
		IndicatorResults: []IndicatorResult{
			{Type: IndicatorVIX, Value: 18, Threshold: 20, Operator: OperatorLessThan, Pass: true, Enabled: true, Timestamp: ts},
			{Type: IndicatorRSI, Value: 45, Threshold: 30, Operator: OperatorGreaterThan, Pass: true, Enabled: true, Timestamp: ts, Symbol: "SPY"},
		},
		GroupResults: []GroupResult{
			{
				GroupID:   "grp_1",
				GroupName: "Low Vol",
				Pass:      true,
				IndicatorResults: []IndicatorResult{
					{Type: IndicatorVIX, Value: 18, Threshold: 20, Operator: OperatorLessThan, Pass: true, Enabled: true, Timestamp: ts},
				},
			},
			{
				GroupID:   "grp_2",
				GroupName: "Momentum",
				Pass:      false,
				IndicatorResults: []IndicatorResult{
					{Type: IndicatorRSI, Value: 45, Threshold: 30, Operator: OperatorGreaterThan, Pass: true, Enabled: true, Timestamp: ts, Symbol: "SPY"},
				},
			},
		},
		AllIndicatorsPass: true,
		PlacedOrders:      []PlacedOrder{},
		StartedAt:         ts,
		LastEvaluation:    ts,
		Logs:              []AutomationLog{},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal ActiveAutomation failed: %v", err)
	}

	var restored ActiveAutomation
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal ActiveAutomation failed: %v", err)
	}

	// Verify both flat and grouped results are present
	if len(restored.IndicatorResults) != 2 {
		t.Errorf("expected 2 flat IndicatorResults, got %d", len(restored.IndicatorResults))
	}
	if len(restored.GroupResults) != 2 {
		t.Errorf("expected 2 GroupResults, got %d", len(restored.GroupResults))
	}
	if restored.AllIndicatorsPass != true {
		t.Error("expected AllIndicatorsPass=true, got false")
	}

	// Verify group result fields
	if restored.GroupResults[0].GroupID != "grp_1" {
		t.Errorf("group result 0 GroupID: expected 'grp_1', got %q", restored.GroupResults[0].GroupID)
	}
	if restored.GroupResults[0].Pass != true {
		t.Error("group result 0: expected Pass=true, got false")
	}
	if restored.GroupResults[1].GroupID != "grp_2" {
		t.Errorf("group result 1 GroupID: expected 'grp_2', got %q", restored.GroupResults[1].GroupID)
	}
	if restored.GroupResults[1].Pass != false {
		t.Error("group result 1: expected Pass=false, got true")
	}

	// Verify Status survives
	if restored.Status != StatusEvaluating {
		t.Errorf("Status: expected %q, got %q", StatusEvaluating, restored.Status)
	}
}

// =============================================================================
// 2.3 GroupResult Struct Validation
// =============================================================================

// 2.3.1: Pass field is a data container — doesn't self-validate.
// Construct GroupResult with Pass: true but indicators containing a failing indicator.
// The struct doesn't enforce consistency; that's the evaluator's job.
func TestQA_GroupResult_PassFieldReflectsIndicators(t *testing.T) {
	gr := GroupResult{
		GroupID:   "grp_inconsistent",
		GroupName: "Inconsistent",
		Pass:      true, // Set to true even though an indicator fails
		IndicatorResults: []IndicatorResult{
			{Type: IndicatorVIX, Value: 25, Threshold: 20, Operator: OperatorLessThan, Pass: false, Enabled: true},
			{Type: IndicatorRSI, Value: 50, Threshold: 30, Operator: OperatorGreaterThan, Pass: true, Enabled: true},
		},
	}

	// The struct is just a data container — Pass is whatever the evaluator set it to.
	// It does NOT self-correct based on IndicatorResults.
	if gr.Pass != true {
		t.Error("expected Pass to remain true (struct is a data container, not self-validating)")
	}

	// Verify the failing indicator is still marked as failing
	if gr.IndicatorResults[0].Pass != false {
		t.Error("expected first indicator Pass=false, got true")
	}

	// This documents the contract: GroupResult.Pass is set by the evaluator,
	// not derived from the indicator results within the struct.
}

// 2.3.2: Empty IndicatorResults serializes to "indicator_results": [] not null.
// The frontend depends on iterating this array.
func TestQA_GroupResult_EmptyIndicatorResults(t *testing.T) {
	gr := GroupResult{
		GroupID:          "grp_empty_results",
		GroupName:        "Empty Results",
		Pass:             true,
		IndicatorResults: []IndicatorResult{},
	}

	data, err := json.Marshal(gr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"indicator_results":[]`) {
		t.Errorf("expected JSON to contain '\"indicator_results\":[]', got: %s", jsonStr)
	}

	// Verify it doesn't serialize as null
	if strings.Contains(jsonStr, `"indicator_results":null`) {
		t.Errorf("expected indicator_results to be [] not null, got: %s", jsonStr)
	}

	// Round-trip: verify it deserializes back to empty slice
	var restored GroupResult
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if restored.IndicatorResults == nil {
		t.Error("expected IndicatorResults to be non-nil empty slice after round-trip, got nil")
	}
	if len(restored.IndicatorResults) != 0 {
		t.Errorf("expected 0 indicator results after round-trip, got %d", len(restored.IndicatorResults))
	}
}

// =============================================================================
// 2.4 ID Generation
// =============================================================================

// 2.4.1: Generate 100 group IDs — all unique and start with grp_.
func TestQA_GenerateGroupID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 100)

	for i := 0; i < 100; i++ {
		id := GenerateGroupID()

		if !strings.HasPrefix(id, "grp_") {
			t.Errorf("iteration %d: expected group ID to start with 'grp_', got %q", i, id)
		}
		if seen[id] {
			t.Errorf("iteration %d: duplicate group ID generated: %q", i, id)
		}
		seen[id] = true
	}

	if len(seen) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(seen))
	}
}

// 2.4.2: Generate 100 indicator IDs — all unique and start with ind_.
func TestQA_GenerateIndicatorID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 100)

	for i := 0; i < 100; i++ {
		id := GenerateIndicatorID()

		if !strings.HasPrefix(id, "ind_") {
			t.Errorf("iteration %d: expected indicator ID to start with 'ind_', got %q", i, id)
		}
		if seen[id] {
			t.Errorf("iteration %d: duplicate indicator ID generated: %q", i, id)
		}
		seen[id] = true
	}

	if len(seen) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(seen))
	}
}
