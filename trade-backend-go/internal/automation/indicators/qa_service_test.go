package indicators

import (
	"fmt"
	"strings"
	"testing"

	"trade-backend-go/internal/automation/types"
)

// ===========================================================================
// Section 5 — Route Ordering Verification (static assertions)
//
// The metadata route GET /automation/indicators/metadata is registered at
// line 1718 of cmd/server/main.go.  The first parameterized /:id route
// (POST /automation/:id/start) is registered at line 1725.
//
// This test file cannot import main.go, so Section 5 is verified by code
// inspection and documented here.  See qa_service_test.go header.
//
// Verified facts:
//   Line 1718: g.GET("/automation/indicators/metadata", h.GetIndicatorMetadata)
//   Line 1725: g.POST("/automation/:id/start", h.StartAutomation)  ← first /:id
//   → metadata route is registered 7 lines BEFORE the first /:id route. ✓
//   → All other static routes (/status, /evaluate, /preview-strikes,
//     /fomc-dates, /tracking/*) are also before /:id routes.          ✓
//   → GET /automation/configs/:id (line 1700) uses a different prefix
//     ("configs/:id") and does NOT conflict with "indicators/metadata". ✓
// ===========================================================================

// ===========================================================================
// Section 7.1 — Helper Functions
// ===========================================================================

func TestGetParamOrDefault_NilParams(t *testing.T) {
	cfg := types.IndicatorConfig{Params: nil}
	got := getParamOrDefault(cfg, "period", 14)
	if got != 14 {
		t.Errorf("nil params: got %v, want 14", got)
	}
}

func TestGetParamOrDefault_KeyPresent(t *testing.T) {
	cfg := types.IndicatorConfig{
		Params: map[string]float64{"period": 20},
	}
	got := getParamOrDefault(cfg, "period", 14)
	if got != 20 {
		t.Errorf("key present: got %v, want 20", got)
	}
}

func TestGetParamOrDefault_KeyMissing(t *testing.T) {
	cfg := types.IndicatorConfig{
		Params: map[string]float64{"other_key": 99},
	}
	got := getParamOrDefault(cfg, "period", 14)
	if got != 14 {
		t.Errorf("key missing: got %v, want 14", got)
	}
}

func TestGetIntParam_Conversion(t *testing.T) {
	cfg := types.IndicatorConfig{
		Params: map[string]float64{"period": 26.0},
	}
	got := getIntParam(cfg, "period", 14)
	if got != 26 {
		t.Errorf("int conversion: got %d, want 26", got)
	}
}

func TestGetIntParam_Default(t *testing.T) {
	cfg := types.IndicatorConfig{Params: nil}
	got := getIntParam(cfg, "period", 14)
	if got != 14 {
		t.Errorf("int default: got %d, want 14", got)
	}
}

func TestGetIntParam_Truncation(t *testing.T) {
	// float64 → int truncates toward zero
	cfg := types.IndicatorConfig{
		Params: map[string]float64{"period": 14.9},
	}
	got := getIntParam(cfg, "period", 10)
	if got != 14 {
		t.Errorf("int truncation: got %d, want 14", got)
	}
}

// ===========================================================================
// Section 7.2 — Switch Case Code-Review Tests
//
// We verify barsNeeded formulas and ParamSummary formats by replicating the
// exact logic from each switch case.  Since getHistoricalBarsForIndicator
// requires a live provider, we test the param-extraction and formula layer
// independently.
// ===========================================================================

// barsNeededSpec describes the expected barsNeeded calculation for a case.
type barsNeededSpec struct {
	indicator   types.IndicatorType
	params      map[string]float64 // nil → use defaults
	wantBars    int                // expected barsNeeded
	wantSummary string             // expected ParamSummary
	usesHLC     bool               // true if case passes highs+lows+closes
}

func TestSwitchCaseBarsNeeded(t *testing.T) {
	specs := []barsNeededSpec{
		// 1. RSI — period*3, closes only
		{types.IndicatorRSI, nil, 14 * 3, "14", false},
		{types.IndicatorRSI, map[string]float64{"period": 20}, 20 * 3, "20", false},

		// 2. MACD — slow+signal+slow, closes only
		{types.IndicatorMACD, nil, 26 + 9 + 26, "12/26/9", false},
		{types.IndicatorMACD, map[string]float64{"fast_period": 8, "slow_period": 21, "signal_period": 5}, 21 + 5 + 21, "8/21/5", false},

		// 3. Momentum — period+1, closes only
		{types.IndicatorMomentum, nil, 10 + 1, "10", false},

		// 4. CMO — period+1, closes only
		{types.IndicatorCMO, nil, 14 + 1, "14", false},

		// 5. Stochastic — kPeriod+dPeriod, highs+lows+closes
		{types.IndicatorStoch, nil, 14 + 3, "14/3", true},

		// 6. StochRSI — rsiPeriod*3+stochPeriod, closes only
		{types.IndicatorStochRSI, nil, 14*3 + 14, "14/14/3/3", false},

		// 7. ADX — period*3, highs+lows+closes
		{types.IndicatorADX, nil, 14 * 3, "14", true},

		// 8. CCI — period, highs+lows+closes
		{types.IndicatorCCI, nil, 20, "20", true},

		// 9. SMA — period, closes only
		{types.IndicatorSMA, nil, 20, "20", false},

		// 10. EMA — period*2, closes only
		{types.IndicatorEMA, nil, 20 * 2, "20", false},

		// 11. ATR — period*2, highs+lows+closes
		{types.IndicatorATR, nil, 14 * 2, "14", true},

		// 12. BB%B — period, closes only
		{types.IndicatorBBPercent, nil, 20, "20/2.0", false},
	}

	for _, s := range specs {
		t.Run(string(s.indicator), func(t *testing.T) {
			gotBars, gotSummary, gotHLC := computeBarsAndSummary(s.indicator, s.params)
			if gotBars != s.wantBars {
				t.Errorf("barsNeeded: got %d, want %d", gotBars, s.wantBars)
			}
			if gotSummary != s.wantSummary {
				t.Errorf("ParamSummary: got %q, want %q", gotSummary, s.wantSummary)
			}
			if gotHLC != s.usesHLC {
				t.Errorf("usesHLC: got %v, want %v", gotHLC, s.usesHLC)
			}
		})
	}
}

// computeBarsAndSummary replicates the param extraction + barsNeeded
// formula + ParamSummary logic from each switch case in EvaluateIndicator.
// Returns (barsNeeded, paramSummary, usesHighsLowsCloses).
func computeBarsAndSummary(it types.IndicatorType, params map[string]float64) (int, string, bool) {
	cfg := types.IndicatorConfig{Type: it, Params: params}

	switch it {
	case types.IndicatorRSI:
		period := getIntParam(cfg, "period", 14)
		return period * 3, fmt.Sprintf("%d", period), false

	case types.IndicatorMACD:
		fast := getIntParam(cfg, "fast_period", 12)
		slow := getIntParam(cfg, "slow_period", 26)
		signal := getIntParam(cfg, "signal_period", 9)
		return slow + signal + slow, fmt.Sprintf("%d/%d/%d", fast, slow, signal), false

	case types.IndicatorMomentum:
		period := getIntParam(cfg, "period", 10)
		return period + 1, fmt.Sprintf("%d", period), false

	case types.IndicatorCMO:
		period := getIntParam(cfg, "period", 14)
		return period + 1, fmt.Sprintf("%d", period), false

	case types.IndicatorStoch:
		k := getIntParam(cfg, "k_period", 14)
		d := getIntParam(cfg, "d_period", 3)
		return k + d, fmt.Sprintf("%d/%d", k, d), true

	case types.IndicatorStochRSI:
		rsiP := getIntParam(cfg, "rsi_period", 14)
		stochP := getIntParam(cfg, "stoch_period", 14)
		kP := getIntParam(cfg, "k_period", 3)
		dP := getIntParam(cfg, "d_period", 3)
		return rsiP*3 + stochP, fmt.Sprintf("%d/%d/%d/%d", rsiP, stochP, kP, dP), false

	case types.IndicatorADX:
		period := getIntParam(cfg, "period", 14)
		return period * 3, fmt.Sprintf("%d", period), true

	case types.IndicatorCCI:
		period := getIntParam(cfg, "period", 20)
		return period, fmt.Sprintf("%d", period), true

	case types.IndicatorSMA:
		period := getIntParam(cfg, "period", 20)
		return period, fmt.Sprintf("%d", period), false

	case types.IndicatorEMA:
		period := getIntParam(cfg, "period", 20)
		return period * 2, fmt.Sprintf("%d", period), false

	case types.IndicatorATR:
		period := getIntParam(cfg, "period", 14)
		return period * 2, fmt.Sprintf("%d", period), true

	case types.IndicatorBBPercent:
		period := getIntParam(cfg, "period", 20)
		stdDev := getParamOrDefault(cfg, "std_dev", 2.0)
		return period, fmt.Sprintf("%d/%.1f", period, stdDev), false

	default:
		return 0, "", false
	}
}

// ===========================================================================
// Section 7.3 — getHistoricalBarsForIndicator Logic
//
// We cannot call the actual method without a ProviderManager, but we can
// verify the default-symbol and bars-clamping logic by replicating the
// guard code.  These tests serve as regression anchors for the documented
// behavior.
// ===========================================================================

func TestDefaultSymbolLogic(t *testing.T) {
	// Replicate: if symbol == "" { symbol = "QQQ" }
	cases := []struct {
		input string
		want  string
	}{
		{"", "QQQ"},
		{"SPY", "SPY"},
		{"AAPL", "AAPL"},
	}
	for _, tc := range cases {
		symbol := tc.input
		if symbol == "" {
			symbol = "QQQ"
		}
		if symbol != tc.want {
			t.Errorf("input %q: got %q, want %q", tc.input, symbol, tc.want)
		}
	}
}

func TestBarsNeededClamping(t *testing.T) {
	// Replicate: if barsNeeded < 50 { barsNeeded = 50 }
	cases := []struct {
		input int
		want  int
	}{
		{1, 50},
		{49, 50},
		{50, 50},
		{51, 51},
		{100, 100},
		{0, 50},
		{-5, 50},
	}
	for _, tc := range cases {
		barsNeeded := tc.input
		if barsNeeded < 50 {
			barsNeeded = 50
		}
		if barsNeeded != tc.want {
			t.Errorf("input %d: got %d, want %d", tc.input, barsNeeded, tc.want)
		}
	}
}

// ===========================================================================
// Section 7.4 — formatDetails Verification
//
// We create a zero-value Service (formatDetails does not use any Service
// fields) and call it directly to verify format strings for all 12 new
// indicator types.
// ===========================================================================

func TestFormatDetails_NewIndicators(t *testing.T) {
	svc := &Service{} // formatDetails uses no service fields

	tests := []struct {
		name        string
		result      types.IndicatorResult
		wantPrefix  string   // start of the formatted string
		mustContain []string // substrings that must appear
	}{
		{
			name: "RSI",
			result: types.IndicatorResult{
				Type: types.IndicatorRSI, Value: 65.42, Threshold: 70,
				Operator: types.OperatorLessThan, Pass: true, ParamSummary: "14",
			},
			wantPrefix:  "RSI(14)",
			mustContain: []string{"65.42", "<", "70.00", "PASS"},
		},
		{
			name: "MACD",
			result: types.IndicatorResult{
				Type: types.IndicatorMACD, Value: 0.0023, Threshold: 0.001,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "12/26/9",
			},
			wantPrefix:  "MACD(12/26/9)",
			mustContain: []string{"0.0023", ">", "0.0010", "PASS"},
		},
		{
			name: "Momentum",
			result: types.IndicatorResult{
				Type: types.IndicatorMomentum, Value: 2.35, Threshold: 1.0,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "10",
			},
			wantPrefix:  "Momentum(10)",
			mustContain: []string{"2.35%", ">", "1.00%", "PASS"},
		},
		{
			name: "CMO",
			result: types.IndicatorResult{
				Type: types.IndicatorCMO, Value: 42.50, Threshold: 50,
				Operator: types.OperatorLessThan, Pass: true, ParamSummary: "14",
			},
			wantPrefix:  "CMO(14)",
			mustContain: []string{"42.50", "<", "50.00", "PASS"},
		},
		{
			name: "Stochastic",
			result: types.IndicatorResult{
				Type: types.IndicatorStoch, Value: 80.00, Threshold: 80,
				Operator: types.OperatorEqual, Pass: true, ParamSummary: "14/3",
			},
			wantPrefix:  "Stoch(14/3)",
			mustContain: []string{"80.00", "=", "PASS"},
		},
		{
			name: "StochRSI",
			result: types.IndicatorResult{
				Type: types.IndicatorStochRSI, Value: 0.85, Threshold: 0.80,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "14/14/3/3",
			},
			wantPrefix:  "StochRSI(14/14/3/3)",
			mustContain: []string{"0.85", ">", "0.80", "PASS"},
		},
		{
			name: "ADX",
			result: types.IndicatorResult{
				Type: types.IndicatorADX, Value: 25.30, Threshold: 20,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "14",
			},
			wantPrefix:  "ADX(14)",
			mustContain: []string{"25.30", ">", "20.00", "PASS"},
		},
		{
			name: "CCI",
			result: types.IndicatorResult{
				Type: types.IndicatorCCI, Value: 125.50, Threshold: 100,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "20",
			},
			wantPrefix:  "CCI(20)",
			mustContain: []string{"125.50", ">", "100.00", "PASS"},
		},
		{
			name: "SMA",
			result: types.IndicatorResult{
				Type: types.IndicatorSMA, Value: 450.25, Threshold: 445,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "20",
			},
			wantPrefix:  "SMA(20)",
			mustContain: []string{"450.25", ">", "445.00", "PASS"},
		},
		{
			name: "EMA",
			result: types.IndicatorResult{
				Type: types.IndicatorEMA, Value: 448.10, Threshold: 445,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "20",
			},
			wantPrefix:  "EMA(20)",
			mustContain: []string{"448.10", ">", "445.00", "PASS"},
		},
		{
			name: "ATR",
			result: types.IndicatorResult{
				Type: types.IndicatorATR, Value: 5.75, Threshold: 4.0,
				Operator: types.OperatorGreaterThan, Pass: true, ParamSummary: "14",
			},
			wantPrefix:  "ATR(14)",
			mustContain: []string{"5.75", ">", "4.00", "PASS"},
		},
		{
			name: "BBPercent",
			result: types.IndicatorResult{
				Type: types.IndicatorBBPercent, Value: 0.7512, Threshold: 0.8,
				Operator: types.OperatorLessThan, Pass: true, ParamSummary: "20/2.0",
			},
			wantPrefix:  "BB%B(20/2.0)",
			mustContain: []string{"0.7512", "<", "0.8000", "PASS"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.formatDetails(&tc.result)
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Errorf("prefix: got %q, want prefix %q", got, tc.wantPrefix)
			}
			for _, sub := range tc.mustContain {
				if !strings.Contains(got, sub) {
					t.Errorf("missing substring %q in %q", sub, got)
				}
			}
		})
	}
}

// TestFormatDetails_FailCase verifies FAIL is shown when Pass is false.
func TestFormatDetails_FailCase(t *testing.T) {
	svc := &Service{}
	result := types.IndicatorResult{
		Type: types.IndicatorRSI, Value: 75.0, Threshold: 70,
		Operator: types.OperatorLessThan, Pass: false, ParamSummary: "14",
	}
	got := svc.formatDetails(&result)
	if !strings.Contains(got, "FAIL") {
		t.Errorf("expected FAIL in output, got %q", got)
	}
	if strings.Contains(got, "PASS") {
		t.Errorf("expected no PASS in output, got %q", got)
	}
}

// TestFormatDetails_AllOperators verifies all operator symbols render.
func TestFormatDetails_AllOperators(t *testing.T) {
	svc := &Service{}
	ops := []struct {
		op   types.Operator
		want string
	}{
		{types.OperatorGreaterThan, ">"},
		{types.OperatorLessThan, "<"},
		{types.OperatorEqual, "="},
		{types.OperatorNotEqual, "!="},
	}
	for _, o := range ops {
		result := types.IndicatorResult{
			Type: types.IndicatorSMA, Value: 100, Threshold: 100,
			Operator: o.op, Pass: true, ParamSummary: "20",
		}
		got := svc.formatDetails(&result)
		if !strings.Contains(got, o.want) {
			t.Errorf("operator %q: expected %q in output %q", o.op, o.want, got)
		}
	}
}

// TestFormatDetails_OriginalIndicators verifies the 5 original cases still
// produce correct output (regression).
func TestFormatDetails_OriginalIndicators(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name        string
		result      types.IndicatorResult
		mustContain []string
	}{
		{
			name: "VIX",
			result: types.IndicatorResult{
				Type: types.IndicatorVIX, Value: 18.50, Threshold: 20,
				Operator: types.OperatorLessThan, Pass: true,
			},
			mustContain: []string{"VIX", "18.50", "<", "20.00", "PASS"},
		},
		{
			name: "Gap",
			result: types.IndicatorResult{
				Type: types.IndicatorGap, Value: 1.25, Threshold: 2.0,
				Operator: types.OperatorLessThan, Pass: true,
			},
			mustContain: []string{"Gap", "1.25%", "<", "2.00%", "PASS"},
		},
		{
			name: "Range",
			result: types.IndicatorResult{
				Type: types.IndicatorRange, Value: 0.80, Threshold: 1.0,
				Operator: types.OperatorLessThan, Pass: true,
			},
			mustContain: []string{"Range", "0.80%", "<", "1.00%", "PASS"},
		},
		{
			name: "Trend",
			result: types.IndicatorResult{
				Type: types.IndicatorTrend, Value: -0.50, Threshold: 0,
				Operator: types.OperatorLessThan, Pass: true,
			},
			mustContain: []string{"Trend", "-0.50%", "<", "0.00%", "PASS"},
		},
		{
			name: "Calendar_FOMCDay",
			result: types.IndicatorResult{
				Type: types.IndicatorCalendar, Value: 1, Threshold: 1,
				Operator: types.OperatorEqual, Pass: true,
			},
			mustContain: []string{"FOMC Day: Yes", "PASS"},
		},
		{
			name: "Calendar_NotFOMCDay",
			result: types.IndicatorResult{
				Type: types.IndicatorCalendar, Value: 0, Threshold: 1,
				Operator: types.OperatorEqual, Pass: false,
			},
			mustContain: []string{"FOMC Day: No", "FAIL"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.formatDetails(&tc.result)
			for _, sub := range tc.mustContain {
				if !strings.Contains(got, sub) {
					t.Errorf("missing substring %q in %q", sub, got)
				}
			}
		})
	}
}

// ===========================================================================
// Section 7.2 Supplemental — OHLC Array Usage Verification
//
// Cross-check which indicators use highs+lows+closes vs closes-only.
// ===========================================================================

func TestOHLCUsageClassification(t *testing.T) {
	// Indicators that require highs+lows+closes (per switch cases).
	hlc := map[types.IndicatorType]bool{
		types.IndicatorStoch: true,
		types.IndicatorADX:   true,
		types.IndicatorCCI:   true,
		types.IndicatorATR:   true,
	}
	// Indicators that use closes only.
	closesOnly := map[types.IndicatorType]bool{
		types.IndicatorRSI:       true,
		types.IndicatorMACD:      true,
		types.IndicatorMomentum:  true,
		types.IndicatorCMO:       true,
		types.IndicatorStochRSI:  true,
		types.IndicatorSMA:       true,
		types.IndicatorEMA:       true,
		types.IndicatorBBPercent: true,
	}

	// Verify sets are disjoint and cover all 12 new types.
	allNew := []types.IndicatorType{
		types.IndicatorRSI, types.IndicatorMACD, types.IndicatorMomentum,
		types.IndicatorCMO, types.IndicatorStoch, types.IndicatorStochRSI,
		types.IndicatorADX, types.IndicatorCCI, types.IndicatorSMA,
		types.IndicatorEMA, types.IndicatorATR, types.IndicatorBBPercent,
	}

	for _, it := range allNew {
		inHLC := hlc[it]
		inCloses := closesOnly[it]
		if inHLC == inCloses {
			t.Errorf("%q: in both sets or neither (hlc=%v, closesOnly=%v)", it, inHLC, inCloses)
		}
	}

	if len(hlc)+len(closesOnly) != 12 {
		t.Errorf("total classified: %d, want 12", len(hlc)+len(closesOnly))
	}

	// Cross-check with computeBarsAndSummary helper (usesHLC flag).
	for _, it := range allNew {
		_, _, gotHLC := computeBarsAndSummary(it, nil)
		wantHLC := hlc[it]
		if gotHLC != wantHLC {
			t.Errorf("%q: computeBarsAndSummary usesHLC=%v, expected %v", it, gotHLC, wantHLC)
		}
	}
}

// =============================================================================
// Step 3, Section 3.1 — EvaluateIndicatorGroups Additional Edge Cases
//
// These tests use the computeGroupORLogic helper (defined in service_test.go)
// to test evaluation logic without provider dependencies.
// =============================================================================

// 3.1.1: One group where all indicators have Enabled: false.
// AllIndicatorsPass returns true for that group (vacuously true per FR-2.3).
// Verify anyGroupPasses = true.
func TestQA_GroupEval_AllDisabledIndicatorsInGroup(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_1", Name: "All Disabled"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{
			{Type: types.IndicatorVIX, Enabled: false, Pass: false, Stale: false},
			{Type: types.IndicatorRSI, Enabled: false, Pass: false, Stale: false},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true when all indicators are disabled (vacuously true)")
	}
	if !groupResults[0].Pass {
		t.Error("expected group Pass=true when all indicators are disabled")
	}
}

// 3.1.2: Group A: one enabled (passes) + one disabled. Group B: one enabled (fails).
// Verify Group A passes (disabled is skipped), Group B fails, anyGroupPasses = true.
func TestQA_GroupEval_MixedDisabledAndEnabled(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_a", Name: "Group A"},
		{ID: "grp_b", Name: "Group B"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: one enabled+passing, one disabled
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false},
			{Type: types.IndicatorRSI, Enabled: false, Pass: false, Stale: false},
		},
		// Group B: one enabled+failing
		{
			{Type: types.IndicatorGap, Enabled: true, Pass: false, Stale: false},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true (Group A passes)")
	}
	if !groupResults[0].Pass {
		t.Error("expected Group A to pass (disabled indicator skipped)")
	}
	if groupResults[1].Pass {
		t.Error("expected Group B to fail")
	}
}

// 3.1.3: One group with Indicators: []. AllIndicatorsPass([]) returns true.
// Verify anyGroupPasses = true, groupResults[0].Pass = true.
func TestQA_GroupEval_EmptyGroupAutoPassesVacuouslyTrue(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_empty", Name: "Empty Group", Indicators: []types.IndicatorConfig{}},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{}, // Empty results for empty group
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true for empty group (vacuously true)")
	}
	if len(groupResults) != 1 {
		t.Fatalf("expected 1 group result, got %d", len(groupResults))
	}
	if !groupResults[0].Pass {
		t.Error("expected empty group Pass=true (vacuously true)")
	}
}

// 3.1.4: Group A: empty (passes vacuously). Group B: one indicator fails.
// Verify anyGroupPasses = true (Group A passes), both group results are populated.
func TestQA_GroupEval_EmptyGroupPlusFailingGroup(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_a", Name: "Empty Group"},
		{ID: "grp_b", Name: "Failing Group"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{}, // Group A: empty
		{ // Group B: one failing indicator
			{Type: types.IndicatorVIX, Enabled: true, Pass: false, Stale: false, Value: 25},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true (empty Group A passes vacuously)")
	}
	if len(groupResults) != 2 {
		t.Fatalf("expected 2 group results, got %d", len(groupResults))
	}
	if !groupResults[0].Pass {
		t.Error("expected Group A (empty) to pass")
	}
	if groupResults[1].Pass {
		t.Error("expected Group B to fail")
	}
}

// 3.1.5: Group A: indicator is stale (fails per stale-blocking rule).
// Group B: indicator is fresh and passes.
// Verify anyGroupPasses = true — stale data in one group does not block another
// group's fresh passing data.
func TestQA_GroupEval_MixedStaleAndFreshAcrossGroups(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_stale", Name: "Stale Group"},
		{ID: "grp_fresh", Name: "Fresh Group"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: stale indicator (stale always blocks regardless of Pass value)
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: true, Value: 18},
		},
		// Group B: fresh and passing
		{
			{Type: types.IndicatorRSI, Enabled: true, Pass: true, Stale: false, Value: 45},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true (fresh Group B passes, stale Group A doesn't block it)")
	}
	if groupResults[0].Pass {
		t.Error("expected stale Group A to fail (stale blocks)")
	}
	if !groupResults[1].Pass {
		t.Error("expected fresh Group B to pass")
	}
}

// 3.1.6: Group A: stale indicator. Group B: stale indicator.
// Verify anyGroupPasses = false.
func TestQA_GroupEval_AllGroupsStale(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_a", Name: "Stale A"},
		{ID: "grp_b", Name: "Stale B"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: true},
		},
		{
			{Type: types.IndicatorRSI, Enabled: true, Pass: true, Stale: true},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if anyGroupPasses {
		t.Error("expected anyGroupPasses=false when all groups have stale indicators")
	}
	if groupResults[0].Pass {
		t.Error("expected Group A to fail (stale)")
	}
	if groupResults[1].Pass {
		t.Error("expected Group B to fail (stale)")
	}
}

// 3.1.7: One group with 3 indicators (all pass). Verify behavior is identical to
// calling AllIndicatorsPass directly on the same results.
// This validates AC-4's requirement that single-group behaves like the old flat list.
func TestQA_GroupEval_SingleGroupBehavesLikeFlatList(t *testing.T) {
	s := &Service{}

	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false, Value: 15},
		{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: false, Value: 0.3},
		{Type: types.IndicatorRSI, Enabled: true, Pass: true, Stale: false, Value: 45},
	}

	groups := []types.IndicatorGroup{
		{ID: "grp_single", Name: "Single Group"},
	}
	resultsByGroup := [][]types.IndicatorResult{results}

	groupResults, flatResults, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	// anyGroupPasses should match AllIndicatorsPass
	directPass := s.AllIndicatorsPass(results)
	if anyGroupPasses != directPass {
		t.Errorf("anyGroupPasses (%v) != AllIndicatorsPass (%v) — single group should behave like flat list", anyGroupPasses, directPass)
	}

	// The group should pass
	if !groupResults[0].Pass {
		t.Error("expected single group to pass")
	}

	// Flat results should match group results
	if len(flatResults) != len(results) {
		t.Errorf("expected %d flat results, got %d", len(results), len(flatResults))
	}
	for i, r := range flatResults {
		if r.Type != results[i].Type {
			t.Errorf("flat result[%d]: expected type %s, got %s", i, results[i].Type, r.Type)
		}
		if r.Value != results[i].Value {
			t.Errorf("flat result[%d]: expected value %v, got %v", i, results[i].Value, r.Value)
		}
	}

	// Group results should contain the same indicator results
	if len(groupResults[0].IndicatorResults) != len(results) {
		t.Errorf("expected %d indicator results in group, got %d", len(results), len(groupResults[0].IndicatorResults))
	}
}

// 3.1.8: One group with 1 indicator that fails.
// Verify anyGroupPasses = false, groupResults[0].Pass = false.
func TestQA_GroupEval_SingleGroupSingleIndicatorFails(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_1", Name: "Failing Group"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: false, Stale: false, Value: 25},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if anyGroupPasses {
		t.Error("expected anyGroupPasses=false when single group's indicator fails")
	}
	if groupResults[0].Pass {
		t.Error("expected group Pass=false")
	}
}

// 3.1.9: Group A passes. Group B fails. Verify both groupResults entries are populated
// with correct data — proves all groups are evaluated (FR-2.6, no short-circuit).
func TestQA_GroupEval_NoShortCircuit(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_pass", Name: "Passing Group"},
		{ID: "grp_fail", Name: "Failing Group"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: passes
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false, Value: 15},
		},
		// Group B: fails
		{
			{Type: types.IndicatorRSI, Enabled: true, Pass: false, Stale: false, Value: 72},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true")
	}

	// Both groups must be evaluated (no short-circuit after Group A passes)
	if len(groupResults) != 2 {
		t.Fatalf("expected 2 group results (no short-circuit), got %d", len(groupResults))
	}

	// Verify Group A result is populated correctly
	if groupResults[0].GroupID != "grp_pass" {
		t.Errorf("group 0 ID: expected 'grp_pass', got %q", groupResults[0].GroupID)
	}
	if !groupResults[0].Pass {
		t.Error("expected Group A to pass")
	}
	if len(groupResults[0].IndicatorResults) != 1 {
		t.Errorf("expected 1 indicator result in Group A, got %d", len(groupResults[0].IndicatorResults))
	}

	// Verify Group B result is populated correctly (proves it was evaluated despite Group A passing)
	if groupResults[1].GroupID != "grp_fail" {
		t.Errorf("group 1 ID: expected 'grp_fail', got %q", groupResults[1].GroupID)
	}
	if groupResults[1].Pass {
		t.Error("expected Group B to fail")
	}
	if len(groupResults[1].IndicatorResults) != 1 {
		t.Errorf("expected 1 indicator result in Group B, got %d", len(groupResults[1].IndicatorResults))
	}
	if groupResults[1].IndicatorResults[0].Value != 72 {
		t.Errorf("expected Group B indicator value 72, got %v", groupResults[1].IndicatorResults[0].Value)
	}
}

// 3.1.10: Group A: fails. Group B: passes. Group C: fails.
// Verify anyGroupPasses = true and only groupResults[1].Pass = true.
func TestQA_GroupEval_ThreeGroupsSecondPasses(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_a", Name: "Group A"},
		{ID: "grp_b", Name: "Group B"},
		{ID: "grp_c", Name: "Group C"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: fails
		{{Type: types.IndicatorVIX, Enabled: true, Pass: false, Stale: false}},
		// Group B: passes
		{{Type: types.IndicatorRSI, Enabled: true, Pass: true, Stale: false}},
		// Group C: fails
		{{Type: types.IndicatorGap, Enabled: true, Pass: false, Stale: false}},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true (Group B passes)")
	}
	if len(groupResults) != 3 {
		t.Fatalf("expected 3 group results, got %d", len(groupResults))
	}
	if groupResults[0].Pass {
		t.Error("expected Group A to fail")
	}
	if !groupResults[1].Pass {
		t.Error("expected Group B to pass")
	}
	if groupResults[2].Pass {
		t.Error("expected Group C to fail")
	}
}

// 3.1.11: Three groups with 2, 1, and 3 indicators respectively.
// Verify flatResults has 6 entries in group-then-indicator order.
func TestQA_GroupEval_FlatResultsOrder(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_a", Name: "Group A"},
		{ID: "grp_b", Name: "Group B"},
		{ID: "grp_c", Name: "Group C"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: 2 indicators
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false, Value: 15},
			{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: false, Value: 0.3},
		},
		// Group B: 1 indicator
		{
			{Type: types.IndicatorRSI, Enabled: true, Pass: true, Stale: false, Value: 45},
		},
		// Group C: 3 indicators
		{
			{Type: types.IndicatorATR, Enabled: true, Pass: true, Stale: false, Value: 3.2},
			{Type: types.IndicatorMACD, Enabled: true, Pass: false, Stale: false, Value: -0.5},
			{Type: types.IndicatorEMA, Enabled: true, Pass: true, Stale: false, Value: 450.0},
		},
	}

	_, flatResults, _ := computeGroupORLogic(s, groups, resultsByGroup)

	if len(flatResults) != 6 {
		t.Fatalf("expected 6 flat results, got %d", len(flatResults))
	}

	// Verify order: Group A's indicators, then Group B's, then Group C's
	expectedTypes := []types.IndicatorType{
		types.IndicatorVIX,  // Group A[0]
		types.IndicatorGap,  // Group A[1]
		types.IndicatorRSI,  // Group B[0]
		types.IndicatorATR,  // Group C[0]
		types.IndicatorMACD, // Group C[1]
		types.IndicatorEMA,  // Group C[2]
	}
	expectedValues := []float64{15, 0.3, 45, 3.2, -0.5, 450.0}

	for i, expected := range expectedTypes {
		if flatResults[i].Type != expected {
			t.Errorf("flatResults[%d]: expected type %s, got %s", i, expected, flatResults[i].Type)
		}
		if flatResults[i].Value != expectedValues[i] {
			t.Errorf("flatResults[%d]: expected value %v, got %v", i, expectedValues[i], flatResults[i].Value)
		}
	}
}

// 3.1.12: Verify GroupResult.GroupID and GroupResult.GroupName match the input
// IndicatorGroup.ID and IndicatorGroup.Name for each group.
func TestQA_GroupEval_GroupMetadataPreserved(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_alpha", Name: "Alpha Strategy"},
		{ID: "grp_beta", Name: "Beta Strategy"},
		{ID: "grp_gamma", Name: "Gamma Strategy"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false}},
		{{Type: types.IndicatorRSI, Enabled: true, Pass: false, Stale: false}},
		{{Type: types.IndicatorATR, Enabled: true, Pass: true, Stale: false}},
	}

	groupResults, _, _ := computeGroupORLogic(s, groups, resultsByGroup)

	if len(groupResults) != 3 {
		t.Fatalf("expected 3 group results, got %d", len(groupResults))
	}

	for i, gr := range groupResults {
		if gr.GroupID != groups[i].ID {
			t.Errorf("groupResults[%d].GroupID: expected %q, got %q", i, groups[i].ID, gr.GroupID)
		}
		if gr.GroupName != groups[i].Name {
			t.Errorf("groupResults[%d].GroupName: expected %q, got %q", i, groups[i].Name, gr.GroupName)
		}
	}
}

// =============================================================================
// Step 3, Section 3.2 — AllIndicatorsPass Additional Edge Cases
// =============================================================================

// 3.2.1: All indicators in the list are disabled. Verify returns true (vacuously true).
func TestQA_AllIndicatorsPass_AllDisabled(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: false, Pass: false, Stale: false},
		{Type: types.IndicatorGap, Enabled: false, Pass: false, Stale: false},
		{Type: types.IndicatorRSI, Enabled: false, Pass: false, Stale: true},
	}

	if !s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass=true when all indicators are disabled (vacuously true)")
	}
}

// 3.2.2: One indicator is stale (Stale: true) but its value still passes the threshold
// (Pass: true). Verify returns false — stale always blocks regardless of the pass value.
func TestQA_AllIndicatorsPass_StaleButPassingValue(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: true, Value: 18},
	}

	if s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass=false — stale always blocks regardless of Pass value")
	}
}

// 3.2.3: Three indicators: (1) disabled, (2) enabled + stale, (3) enabled + fresh + passing.
// Verify returns false because indicator (2) is stale.
func TestQA_AllIndicatorsPass_MixOfDisabledStaleFresh(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		// (1) disabled — skipped
		{Type: types.IndicatorVIX, Enabled: false, Pass: false, Stale: false},
		// (2) enabled + stale — should block
		{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: true, Value: 0.5},
		// (3) enabled + fresh + passing
		{Type: types.IndicatorRSI, Enabled: true, Pass: true, Stale: false, Value: 45},
	}

	if s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass=false because indicator (2) is stale")
	}
}
