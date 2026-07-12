package types

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestGetEffectiveIndicatorGroups_HasGroups(t *testing.T) {
	config := &AutomationConfig{
		IndicatorGroups: []IndicatorGroup{
			{
				ID:   "grp_1",
				Name: "Low Vol",
				Indicators: []IndicatorConfig{
					{ID: "ind_1", Type: IndicatorVIX, Enabled: true, Operator: OperatorLessThan, Threshold: 20},
				},
			},
			{
				ID:   "grp_2",
				Name: "High Vol",
				Indicators: []IndicatorConfig{
					{ID: "ind_2", Type: IndicatorVIX, Enabled: true, Operator: OperatorGreaterThan, Threshold: 30},
				},
			},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].ID != "grp_1" {
		t.Errorf("expected first group ID 'grp_1', got %q", groups[0].ID)
	}
	if groups[1].Name != "High Vol" {
		t.Errorf("expected second group name 'High Vol', got %q", groups[1].Name)
	}
	if len(groups[0].Indicators) != 1 || len(groups[1].Indicators) != 1 {
		t.Errorf("expected 1 indicator per group, got %d and %d", len(groups[0].Indicators), len(groups[1].Indicators))
	}
}

func TestGetEffectiveIndicatorGroups_LegacyOnly(t *testing.T) {
	config := &AutomationConfig{
		Indicators: []IndicatorConfig{
			{ID: "ind_1", Type: IndicatorVIX, Enabled: true, Operator: OperatorLessThan, Threshold: 20},
			{ID: "ind_2", Type: IndicatorGap, Enabled: true, Operator: OperatorLessThan, Threshold: 1.0},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group for legacy config, got %d", len(groups))
	}
	if groups[0].ID != "default" {
		t.Errorf("expected group ID 'default', got %q", groups[0].ID)
	}
	if groups[0].Name != "Default" {
		t.Errorf("expected group name 'Default', got %q", groups[0].Name)
	}
	if len(groups[0].Indicators) != 2 {
		t.Errorf("expected 2 indicators in default group, got %d", len(groups[0].Indicators))
	}
}

func TestGetEffectiveIndicatorGroups_BothEmpty(t *testing.T) {
	config := &AutomationConfig{}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups when both empty, got %d", len(groups))
	}
}

func TestGetEffectiveIndicatorGroups_GroupsTakePriority(t *testing.T) {
	config := &AutomationConfig{
		Indicators: []IndicatorConfig{
			{ID: "legacy_ind", Type: IndicatorVIX, Enabled: true},
		},
		IndicatorGroups: []IndicatorGroup{
			{
				ID:   "grp_1",
				Name: "Primary",
				Indicators: []IndicatorConfig{
					{ID: "new_ind", Type: IndicatorRSI, Enabled: true},
				},
			},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (from IndicatorGroups), got %d", len(groups))
	}
	if groups[0].ID != "grp_1" {
		t.Errorf("expected group from IndicatorGroups to win, got ID %q", groups[0].ID)
	}
	if groups[0].Indicators[0].ID != "new_ind" {
		t.Errorf("expected indicator from IndicatorGroups, got ID %q", groups[0].Indicators[0].ID)
	}
}

func TestGenerateGroupID_Format(t *testing.T) {
	id := GenerateGroupID()
	if !strings.HasPrefix(id, "grp_") {
		t.Errorf("expected group ID to start with 'grp_', got %q", id)
	}
	// Format: grp_{unixnano}_{4chars} — should have at least "grp_" + digits + "_" + 4 chars
	parts := strings.SplitN(id, "_", 3)
	if len(parts) != 3 {
		t.Errorf("expected 3 parts separated by '_', got %d from %q", len(parts), id)
	}
	if parts[0] != "grp" {
		t.Errorf("expected first part 'grp', got %q", parts[0])
	}
	// The last part should be 4+ chars (the random suffix may contain underscores from unixnano)
	// Just verify total length is reasonable: "grp_" (4) + timestamp (~19) + "_" (1) + random (4) = ~28
	if len(id) < 10 {
		t.Errorf("expected group ID length >= 10, got %d for %q", len(id), id)
	}

	// Verify uniqueness (generate a second one)
	id2 := GenerateGroupID()
	if id == id2 {
		t.Errorf("expected unique IDs, but got same ID twice: %q", id)
	}
}

// ---- Step 1: MaxCapitalMode + config fields + constructor default ----

func TestNewTradeConfiguration_DefaultsToFixedMode(t *testing.T) {
	tc := NewTradeConfiguration()
	if tc.MaxCapitalMode != MaxCapitalModeFixed {
		t.Errorf("expected MaxCapitalMode %q, got %q", MaxCapitalModeFixed, tc.MaxCapitalMode)
	}
}

func TestTradeConfiguration_LegacyJSONUnmarshalsToEmptyMode(t *testing.T) {
	// A legacy config carrying only max_capital (no mode/percent fields).
	raw := `{"strategy":"put_spread","width":20,"target_delta":0.05,"max_capital":5000,"order_type":"limit"}`
	var tc TradeConfiguration
	if err := json.Unmarshal([]byte(raw), &tc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if tc.MaxCapitalMode != "" {
		t.Errorf("expected empty MaxCapitalMode for legacy config, got %q", tc.MaxCapitalMode)
	}
	if tc.MaxCapitalPercent != 0 {
		t.Errorf("expected MaxCapitalPercent 0 for legacy config, got %v", tc.MaxCapitalPercent)
	}
	if tc.MaxCapital != 5000 {
		t.Errorf("expected MaxCapital 5000, got %v", tc.MaxCapital)
	}
}

func TestTradeConfiguration_PercentJSONRoundTrip(t *testing.T) {
	tc := TradeConfiguration{
		Strategy:          StrategyPutSpread,
		Width:             20,
		MaxCapital:        5000,
		MaxCapitalMode:    MaxCapitalModePercent,
		MaxCapitalPercent: 60,
		OrderType:         "limit",
	}
	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"max_capital_mode":"percent"`) {
		t.Errorf("expected max_capital_mode in JSON, got %s", s)
	}
	if !strings.Contains(s, `"max_capital_percent":60`) {
		t.Errorf("expected max_capital_percent in JSON, got %s", s)
	}

	var back TradeConfiguration
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if back.MaxCapitalMode != MaxCapitalModePercent || back.MaxCapitalPercent != 60 {
		t.Errorf("round-trip mismatch: mode=%q percent=%v", back.MaxCapitalMode, back.MaxCapitalPercent)
	}
}

func TestTradeConfiguration_OmitemptyOmitsZeroValues(t *testing.T) {
	// Fixed config with zero percent and empty mode should not serialize the new fields.
	tc := TradeConfiguration{
		Strategy:   StrategyPutSpread,
		MaxCapital: 5000,
		OrderType:  "limit",
	}
	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "max_capital_mode") {
		t.Errorf("expected max_capital_mode to be omitted, got %s", s)
	}
	if strings.Contains(s, "max_capital_percent") {
		t.Errorf("expected max_capital_percent to be omitted, got %s", s)
	}
}

// ---- Step 2: ResolveEffectiveCapital + sentinel errors ----

func TestResolveEffectiveCapital_FixedMode(t *testing.T) {
	cases := []struct {
		name     string
		netLiq   float64
		netLiqOK bool
	}{
		{"ok true", 12345, true},
		{"ok false", 0, false},
		{"zero netliq ok", 0, true},
		{"negative netliq", -100, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tc := TradeConfiguration{MaxCapital: 5000, MaxCapitalMode: MaxCapitalModeFixed}
			eff, err := tc.ResolveEffectiveCapital(c.netLiq, c.netLiqOK)
			if err != nil {
				t.Fatalf("fixed mode must never error, got %v", err)
			}
			if eff != 5000 {
				t.Errorf("expected 5000, got %v", eff)
			}
		})
	}
}

func TestResolveEffectiveCapital_EmptyModeBehavesAsFixed(t *testing.T) {
	tc := TradeConfiguration{MaxCapital: 4200} // MaxCapitalMode == ""
	eff, err := tc.ResolveEffectiveCapital(0, false)
	if err != nil {
		t.Fatalf("empty mode must not error, got %v", err)
	}
	if eff != 4200 {
		t.Errorf("expected 4200, got %v", eff)
	}
}

func TestResolveEffectiveCapital_PercentAC3(t *testing.T) {
	// AC-3: $10,000 * 60% = $6,000
	tc := TradeConfiguration{MaxCapitalMode: MaxCapitalModePercent, MaxCapitalPercent: 60}
	eff, err := tc.ResolveEffectiveCapital(10000, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eff != 6000 {
		t.Errorf("expected 6000, got %v", eff)
	}
}

func TestResolveEffectiveCapital_Rounding(t *testing.T) {
	cases := []struct {
		name   string
		netLiq float64
		pct    float64
		want   float64
	}{
		// 10001 * 33.33% = 3333.3333 -> round -> 3333 (rounds down)
		{"round down", 10001, 33.33, 3333},
		// 9000 * 50% = 4500.0 exact
		{"exact", 9000, 50, 4500},
		// 1 * 50% = 0.5 -> math.Round rounds half away from zero -> 1
		{"half", 1, 50, 1},
		// 3 * 50% = 1.5 -> round -> 2 (rounds up)
		{"round up", 3, 50, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tc := TradeConfiguration{MaxCapitalMode: MaxCapitalModePercent, MaxCapitalPercent: c.pct}
			eff, err := tc.ResolveEffectiveCapital(c.netLiq, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if eff != c.want {
				t.Errorf("netLiq=%v pct=%v: expected %v, got %v", c.netLiq, c.pct, c.want, eff)
			}
		})
	}
}

func TestResolveEffectiveCapital_InvalidPercent(t *testing.T) {
	for _, pct := range []float64{0, 100.1, -5} {
		tc := TradeConfiguration{MaxCapitalMode: MaxCapitalModePercent, MaxCapitalPercent: pct}
		_, err := tc.ResolveEffectiveCapital(10000, true)
		if !errors.Is(err, ErrInvalidCapitalPercent) {
			t.Errorf("pct=%v: expected ErrInvalidCapitalPercent, got %v", pct, err)
		}
	}
}

func TestResolveEffectiveCapital_NetLiqUnavailable(t *testing.T) {
	// Valid pct but netLiq unusable.
	tc := TradeConfiguration{MaxCapitalMode: MaxCapitalModePercent, MaxCapitalPercent: 60}

	if _, err := tc.ResolveEffectiveCapital(10000, false); !errors.Is(err, ErrNetLiqUnavailable) {
		t.Errorf("netLiqOK=false: expected ErrNetLiqUnavailable, got %v", err)
	}
	if _, err := tc.ResolveEffectiveCapital(0, true); !errors.Is(err, ErrNetLiqUnavailable) {
		t.Errorf("netLiq=0: expected ErrNetLiqUnavailable, got %v", err)
	}
	if _, err := tc.ResolveEffectiveCapital(-100, true); !errors.Is(err, ErrNetLiqUnavailable) {
		t.Errorf("netLiq<0: expected ErrNetLiqUnavailable, got %v", err)
	}
}

func TestResolveEffectiveCapital_InvalidPercentTakesPrecedenceOverNetLiq(t *testing.T) {
	// When pct is invalid AND netLiq unusable, the permanent error should win.
	tc := TradeConfiguration{MaxCapitalMode: MaxCapitalModePercent, MaxCapitalPercent: 0}
	_, err := tc.ResolveEffectiveCapital(0, false)
	if !errors.Is(err, ErrInvalidCapitalPercent) {
		t.Errorf("expected ErrInvalidCapitalPercent to take precedence, got %v", err)
	}
}

// ---- Step 3: CalculateUnits takes effective capital as a parameter ----

func TestCalculateUnits_BasicFloor(t *testing.T) {
	cases := []struct {
		name             string
		width            int
		effectiveCapital float64
		want             int
	}{
		// 6000 / (5*100) = 12
		{"exact multiple", 5, 6000, 12},
		// 5999 / 500 = 11.998 -> floor 11
		{"floors truncated", 5, 5999, 11},
		// less than one unit -> 0
		{"below one unit", 20, 1000, 0},
		// zero capital -> 0
		{"zero capital", 20, 0, 0},
		// zero width -> 0 (guard)
		{"zero width", 0, 10000, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tc := TradeConfiguration{Strategy: StrategyPutSpread, Width: c.width}
			if got := tc.CalculateUnits(c.effectiveCapital); got != c.want {
				t.Errorf("width=%d capital=%v: expected %d units, got %d", c.width, c.effectiveCapital, c.want, got)
			}
		})
	}
}

func TestCalculateUnits_FixedModeRegressionParity(t *testing.T) {
	// AC-4: calling CalculateUnits with MaxCapital must reproduce the pre-refactor
	// behavior (which used tc.MaxCapital internally). We assert the value equals the
	// hand-computed floor(MaxCapital / (width*100)).
	cases := []struct {
		maxCapital float64
		width      int
		want       int
	}{
		{5000, 20, 2},  // 5000 / 2000 = 2.5 -> 2
		{6000, 5, 12},  // 6000 / 500 = 12
		{10000, 50, 2}, // 10000 / 5000 = 2
		{100, 20, 0},   // 100 / 2000 -> 0
	}
	for _, c := range cases {
		tc := TradeConfiguration{Strategy: StrategyPutSpread, Width: c.width, MaxCapital: c.maxCapital}
		// Fixed-mode-equivalent call as used by the temporary engine call sites.
		got := tc.CalculateUnits(tc.MaxCapital)
		if got != c.want {
			t.Errorf("maxCapital=%v width=%d: expected %d, got %d", c.maxCapital, c.width, c.want, got)
		}
	}
}

func TestCalculateUnits_IronCondorUsesWiderSide(t *testing.T) {
	// Iron Condor sizing uses max(putWidth, callWidth); the plain Width field is ignored.
	tc := TradeConfiguration{
		Strategy:       StrategyIronCondor,
		Width:          10, // should be ignored for iron condor
		PutSideConfig:  &IronCondorSideConfig{Width: 50},
		CallSideConfig: &IronCondorSideConfig{Width: 20},
	}
	// wider side = 50 -> maxRiskPerUnit = 5000. 6000 / 5000 = 1.2 -> 1 unit.
	if got := tc.CalculateUnits(6000); got != 1 {
		t.Errorf("expected 1 unit using wider side (50), got %d", got)
	}
	// Swap so call side is wider; result must be identical (max selection is symmetric).
	tc.PutSideConfig.Width = 20
	tc.CallSideConfig.Width = 50
	if got := tc.CalculateUnits(6000); got != 1 {
		t.Errorf("expected 1 unit using wider side (call=50), got %d", got)
	}
}
