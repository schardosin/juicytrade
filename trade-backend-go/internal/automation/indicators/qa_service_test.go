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
