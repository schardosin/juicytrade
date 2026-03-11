package indicators

import (
	"encoding/json"
	"testing"

	"trade-backend-go/internal/automation/types"
)

// ===========================================================================
// Section 3 — Backward Compatibility of Original 5 Indicators
// ===========================================================================

// TestOriginalIndicatorTypeConstants verifies the 5 original IndicatorType
// constants have not been renamed or changed.
func TestOriginalIndicatorTypeConstants(t *testing.T) {
	expected := map[types.IndicatorType]string{
		types.IndicatorVIX:      "vix",
		types.IndicatorGap:      "gap",
		types.IndicatorRange:    "range",
		types.IndicatorTrend:    "trend",
		types.IndicatorCalendar: "calendar",
	}
	for constant, want := range expected {
		if string(constant) != want {
			t.Errorf("IndicatorType constant %q: got %q, want %q", want, string(constant), want)
		}
	}
}

// TestIndicatorConfigParamsJSONTag verifies the Params field on
// IndicatorConfig carries the json:"params,omitempty" tag so that
// existing configs without params deserialize with Params == nil.
func TestIndicatorConfigParamsJSONTag(t *testing.T) {
	// Marshal a config without Params; the key "params" should be absent.
	cfg := types.IndicatorConfig{
		Type:      types.IndicatorVIX,
		Enabled:   true,
		Operator:  "lt",
		Threshold: 30,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// The omitempty tag means the "params" key should not appear.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal to map failed: %v", err)
	}
	if _, found := raw["params"]; found {
		t.Errorf("expected \"params\" key to be omitted when Params is nil, but it was present in JSON: %s", string(data))
	}
}

// TestNewIndicatorConfigOriginalTypeParamsNil verifies that
// NewIndicatorConfig for an original indicator type (e.g. VIX) returns a
// config whose Params field is nil, since VIX has no params in metadata.
func TestNewIndicatorConfigOriginalTypeParamsNil(t *testing.T) {
	originals := []types.IndicatorType{
		types.IndicatorVIX,
		types.IndicatorGap,
		types.IndicatorRange,
		types.IndicatorTrend,
		types.IndicatorCalendar,
	}
	for _, it := range originals {
		cfg := types.NewIndicatorConfig(it)
		if cfg.Type != it {
			t.Errorf("NewIndicatorConfig(%q): Type = %q, want %q", it, cfg.Type, it)
		}
		if cfg.Params != nil {
			t.Errorf("NewIndicatorConfig(%q): Params = %v, want nil", it, cfg.Params)
		}
		if !cfg.Enabled {
			t.Errorf("NewIndicatorConfig(%q): Enabled should be true", it)
		}
	}
}

// TestJSONRoundTripNoParams marshals an IndicatorConfig without Params,
// unmarshals it back, and verifies Params remains nil.
func TestJSONRoundTripNoParams(t *testing.T) {
	original := types.IndicatorConfig{
		Type:      types.IndicatorVIX,
		Enabled:   true,
		Operator:  "lt",
		Threshold: 30,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded types.IndicatorConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Enabled != original.Enabled {
		t.Errorf("Enabled: got %v, want %v", decoded.Enabled, original.Enabled)
	}
	if decoded.Operator != original.Operator {
		t.Errorf("Operator: got %q, want %q", decoded.Operator, original.Operator)
	}
	if decoded.Threshold != original.Threshold {
		t.Errorf("Threshold: got %v, want %v", decoded.Threshold, original.Threshold)
	}
	if decoded.Params != nil {
		t.Errorf("Params: got %v, want nil", decoded.Params)
	}
}

// TestCreateConfigDefaultIndicators verifies that the default indicator
// list (as documented in handlers/automation.go CreateConfig) contains
// exactly the 5 original indicator types, in order.
func TestCreateConfigDefaultIndicators(t *testing.T) {
	// Reproduce the exact logic from CreateConfig: when no indicators
	// are provided, it creates these 5 defaults via NewIndicatorConfig.
	defaults := []types.IndicatorConfig{
		types.NewIndicatorConfig(types.IndicatorVIX),
		types.NewIndicatorConfig(types.IndicatorGap),
		types.NewIndicatorConfig(types.IndicatorRange),
		types.NewIndicatorConfig(types.IndicatorTrend),
		types.NewIndicatorConfig(types.IndicatorCalendar),
	}

	if len(defaults) != 5 {
		t.Fatalf("expected 5 default indicators, got %d", len(defaults))
	}

	expectedTypes := []types.IndicatorType{
		types.IndicatorVIX,
		types.IndicatorGap,
		types.IndicatorRange,
		types.IndicatorTrend,
		types.IndicatorCalendar,
	}

	for i, want := range expectedTypes {
		got := defaults[i]
		if got.Type != want {
			t.Errorf("default[%d]: Type = %q, want %q", i, got.Type, want)
		}
		if !got.Enabled {
			t.Errorf("default[%d] (%q): Enabled should be true", i, want)
		}
		if got.Params != nil {
			t.Errorf("default[%d] (%q): Params should be nil, got %v", i, want, got.Params)
		}
		if got.ID == "" {
			t.Errorf("default[%d] (%q): ID should be non-empty", i, want)
		}
	}

	// Verify all IDs are unique.
	seen := make(map[string]bool, len(defaults))
	for i, d := range defaults {
		if seen[d.ID] {
			t.Errorf("default[%d] (%q): duplicate ID %q", i, d.Type, d.ID)
		}
		seen[d.ID] = true
	}
}

// ===========================================================================
// Section 4 — Metadata Registry Completeness and Correctness
// ===========================================================================

// TestMetadataRegistryCount verifies GetIndicatorMetadata returns exactly
// 17 entries.
func TestMetadataRegistryCount(t *testing.T) {
	meta := types.GetIndicatorMetadata()
	if len(meta) != 17 {
		t.Fatalf("GetIndicatorMetadata() returned %d entries, want 17", len(meta))
	}
}

// TestMetadataRequiredFields verifies every entry has all required fields
// populated: Type, Label, Description, Category, Params (non-nil slice),
// ValueRange, and NeedsSymbol (checked indirectly via the bool).
func TestMetadataRequiredFields(t *testing.T) {
	for _, m := range types.GetIndicatorMetadata() {
		if m.Type == "" {
			t.Errorf("entry with empty Type found")
		}
		if m.Label == "" {
			t.Errorf("%q: Label is empty", m.Type)
		}
		if m.Description == "" {
			t.Errorf("%q: Description is empty", m.Type)
		}
		if m.Category == "" {
			t.Errorf("%q: Category is empty", m.Type)
		}
		if m.Params == nil {
			t.Errorf("%q: Params is nil (should be non-nil, possibly empty slice)", m.Type)
		}
		if m.ValueRange == "" {
			t.Errorf("%q: ValueRange is empty", m.Type)
		}
		// NeedsSymbol is a bool; always has a value, tested separately.
	}
}

// TestMetadataCategories verifies each indicator type maps to its correct
// category.
func TestMetadataCategories(t *testing.T) {
	expected := map[types.IndicatorType]string{
		types.IndicatorVIX:      "market",
		types.IndicatorGap:      "market",
		types.IndicatorRange:    "market",
		types.IndicatorTrend:    "market",
		types.IndicatorCalendar: "calendar",

		types.IndicatorRSI:      "momentum",
		types.IndicatorMACD:     "momentum",
		types.IndicatorMomentum: "momentum",
		types.IndicatorCMO:      "momentum",
		types.IndicatorStoch:    "momentum",
		types.IndicatorStochRSI: "momentum",

		types.IndicatorADX: "trend",
		types.IndicatorCCI: "trend",
		types.IndicatorSMA: "trend",
		types.IndicatorEMA: "trend",

		types.IndicatorATR:       "volatility",
		types.IndicatorBBPercent: "volatility",
	}

	metaMap := metadataByType(t)

	for it, wantCat := range expected {
		m, ok := metaMap[it]
		if !ok {
			t.Errorf("metadata missing for type %q", it)
			continue
		}
		if m.Category != wantCat {
			t.Errorf("%q: Category = %q, want %q", it, m.Category, wantCat)
		}
	}
}

// TestMetadataNeedsSymbol verifies NeedsSymbol is false for vix and
// calendar and true for all other 15 indicators.
func TestMetadataNeedsSymbol(t *testing.T) {
	noSymbol := map[types.IndicatorType]bool{
		types.IndicatorVIX:      true,
		types.IndicatorCalendar: true,
	}

	for _, m := range types.GetIndicatorMetadata() {
		if noSymbol[m.Type] {
			if m.NeedsSymbol {
				t.Errorf("%q: NeedsSymbol = true, want false", m.Type)
			}
		} else {
			if !m.NeedsSymbol {
				t.Errorf("%q: NeedsSymbol = false, want true", m.Type)
			}
		}
	}
}

// TestOriginalIndicatorsEmptyParams verifies the 5 original indicators
// have len(Params) == 0.
func TestOriginalIndicatorsEmptyParams(t *testing.T) {
	originals := map[types.IndicatorType]bool{
		types.IndicatorVIX:      true,
		types.IndicatorGap:      true,
		types.IndicatorRange:    true,
		types.IndicatorTrend:    true,
		types.IndicatorCalendar: true,
	}

	metaMap := metadataByType(t)

	for it := range originals {
		m, ok := metaMap[it]
		if !ok {
			t.Errorf("metadata missing for type %q", it)
			continue
		}
		if len(m.Params) != 0 {
			t.Errorf("%q: expected 0 params, got %d", it, len(m.Params))
		}
	}
}

// TestParamDefaults verifies the default values for every parameterized
// indicator match the design specification.
func TestParamDefaults(t *testing.T) {
	metaMap := metadataByType(t)

	tests := []struct {
		indicator types.IndicatorType
		defaults  map[string]float64
	}{
		{types.IndicatorRSI, map[string]float64{"period": 14}},
		{types.IndicatorMACD, map[string]float64{"fast_period": 12, "slow_period": 26, "signal_period": 9}},
		{types.IndicatorMomentum, map[string]float64{"period": 10}},
		{types.IndicatorCMO, map[string]float64{"period": 14}},
		{types.IndicatorStoch, map[string]float64{"k_period": 14, "d_period": 3}},
		{types.IndicatorStochRSI, map[string]float64{"rsi_period": 14, "stoch_period": 14, "k_period": 3, "d_period": 3}},
		{types.IndicatorADX, map[string]float64{"period": 14}},
		{types.IndicatorCCI, map[string]float64{"period": 20}},
		{types.IndicatorSMA, map[string]float64{"period": 20}},
		{types.IndicatorEMA, map[string]float64{"period": 20}},
		{types.IndicatorATR, map[string]float64{"period": 14}},
		{types.IndicatorBBPercent, map[string]float64{"period": 20, "std_dev": 2.0}},
	}

	for _, tc := range tests {
		m, ok := metaMap[tc.indicator]
		if !ok {
			t.Errorf("metadata missing for %q", tc.indicator)
			continue
		}
		paramMap := paramsByKey(m.Params)
		for key, wantDefault := range tc.defaults {
			p, ok := paramMap[key]
			if !ok {
				t.Errorf("%q: missing param %q", tc.indicator, key)
				continue
			}
			if p.DefaultValue != wantDefault {
				t.Errorf("%q param %q: DefaultValue = %v, want %v", tc.indicator, key, p.DefaultValue, wantDefault)
			}
		}
		// Also verify param count matches expected.
		if len(m.Params) != len(tc.defaults) {
			t.Errorf("%q: param count = %d, want %d", tc.indicator, len(m.Params), len(tc.defaults))
		}
	}
}

// TestBollingerStdDevParam verifies the std_dev param for Bollinger %B
// has Type "float", Step 0.1, Min 0.5, Max 5.0.
func TestBollingerStdDevParam(t *testing.T) {
	metaMap := metadataByType(t)
	m, ok := metaMap[types.IndicatorBBPercent]
	if !ok {
		t.Fatal("metadata missing for bb_percent")
	}

	paramMap := paramsByKey(m.Params)
	stdDev, ok := paramMap["std_dev"]
	if !ok {
		t.Fatal("bb_percent: missing std_dev param")
	}

	if stdDev.Type != "float" {
		t.Errorf("std_dev Type = %q, want \"float\"", stdDev.Type)
	}
	if stdDev.Step != 0.1 {
		t.Errorf("std_dev Step = %v, want 0.1", stdDev.Step)
	}
	if stdDev.Min != 0.5 {
		t.Errorf("std_dev Min = %v, want 0.5", stdDev.Min)
	}
	if stdDev.Max != 5.0 {
		t.Errorf("std_dev Max = %v, want 5.0", stdDev.Max)
	}
}

// TestAllIntegerParamsTypeAndStep verifies every param with Type "int"
// has Step == 1, and conversely that only "int" and "float" types exist.
func TestAllIntegerParamsTypeAndStep(t *testing.T) {
	for _, m := range types.GetIndicatorMetadata() {
		for _, p := range m.Params {
			switch p.Type {
			case "int":
				if p.Step != 1 {
					t.Errorf("%q param %q: int param has Step = %v, want 1", m.Type, p.Key, p.Step)
				}
			case "float":
				// float params are valid (e.g. std_dev)
			default:
				t.Errorf("%q param %q: unexpected Type %q (want \"int\" or \"float\")", m.Type, p.Key, p.Type)
			}
		}
	}
}

// TestAllPeriodParamsMinMax verifies all period-related params (keys
// containing "period") have Min >= 1 and Max == 500.
func TestAllPeriodParamsMinMax(t *testing.T) {
	for _, m := range types.GetIndicatorMetadata() {
		for _, p := range m.Params {
			if !isPeriodParam(p.Key) {
				continue
			}
			if p.Min < 1 {
				t.Errorf("%q param %q: Min = %v, want >= 1", m.Type, p.Key, p.Min)
			}
			if p.Max != 500 {
				t.Errorf("%q param %q: Max = %v, want 500", m.Type, p.Key, p.Max)
			}
		}
	}
}

// TestMetadataOrder verifies entries appear in the canonical order:
// existing first, then momentum, trend, volatility.
func TestMetadataOrder(t *testing.T) {
	meta := types.GetIndicatorMetadata()
	expectedOrder := []types.IndicatorType{
		"vix", "gap", "range", "trend", "calendar",
		"rsi", "macd", "momentum", "cmo", "stoch", "stoch_rsi",
		"adx", "cci", "sma", "ema",
		"atr", "bb_percent",
	}
	if len(meta) != len(expectedOrder) {
		t.Fatalf("metadata length = %d, expected order length = %d", len(meta), len(expectedOrder))
	}
	for i, want := range expectedOrder {
		if meta[i].Type != want {
			t.Errorf("metadata[%d]: Type = %q, want %q", i, meta[i].Type, want)
		}
	}
}

// ===========================================================================
// Helpers
// ===========================================================================

// metadataByType returns GetIndicatorMetadata indexed by IndicatorType.
func metadataByType(t *testing.T) map[types.IndicatorType]types.IndicatorMeta {
	t.Helper()
	m := make(map[types.IndicatorType]types.IndicatorMeta)
	for _, entry := range types.GetIndicatorMetadata() {
		if _, dup := m[entry.Type]; dup {
			t.Fatalf("duplicate metadata entry for type %q", entry.Type)
		}
		m[entry.Type] = entry
	}
	return m
}

// paramsByKey returns a param slice indexed by Key.
func paramsByKey(params []types.IndicatorParamDef) map[string]types.IndicatorParamDef {
	m := make(map[string]types.IndicatorParamDef, len(params))
	for _, p := range params {
		m[p.Key] = p
	}
	return m
}

// isPeriodParam returns true if the param key represents a period value.
func isPeriodParam(key string) bool {
	switch key {
	case "period", "fast_period", "slow_period", "signal_period",
		"k_period", "d_period", "rsi_period", "stoch_period":
		return true
	}
	return false
}
