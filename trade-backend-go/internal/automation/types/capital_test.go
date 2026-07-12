package types

import (
	"strings"
	"testing"
)

func floatPtr(v float64) *float64 { return &v }

func TestEffectiveCapitalMode(t *testing.T) {
	tests := []struct {
		name string
		mode CapitalMode
		want CapitalMode
	}{
		{"empty defaults to fixed", "", CapitalModeFixed},
		{"explicit fixed", CapitalModeFixed, CapitalModeFixed},
		{"explicit percent", CapitalModePercent, CapitalModePercent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TradeConfiguration{CapitalMode: tt.mode}
			if got := tc.EffectiveCapitalMode(); got != tt.want {
				t.Errorf("EffectiveCapitalMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveMaxCapital_Fixed(t *testing.T) {
	// Fixed mode returns MaxCapital and ignores netLiq (even if nil).
	tc := &TradeConfiguration{MaxCapital: 5000, CapitalMode: CapitalModeFixed}
	got, err := tc.ResolveMaxCapital(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 5000 {
		t.Errorf("ResolveMaxCapital() = %v, want 5000", got)
	}

	// Empty mode (legacy) also treated as fixed.
	legacy := &TradeConfiguration{MaxCapital: 1234}
	got, err = legacy.ResolveMaxCapital(floatPtr(99999))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1234 {
		t.Errorf("legacy ResolveMaxCapital() = %v, want 1234", got)
	}
}

func TestResolveMaxCapital_Percent(t *testing.T) {
	// 60% of $10,000 = $6,000 (AC-4 / AC-7 worked example).
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 60}
	got, err := tc.ResolveMaxCapital(floatPtr(10000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 6000 {
		t.Errorf("ResolveMaxCapital(10000 @ 60%%) = %v, want 6000", got)
	}

	// Fractional percent.
	tc2 := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 2.5}
	got2, err := tc2.ResolveMaxCapital(floatPtr(20000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got2 != 500 {
		t.Errorf("ResolveMaxCapital(20000 @ 2.5%%) = %v, want 500", got2)
	}
}

func TestResolveMaxCapital_PercentOutOfRange(t *testing.T) {
	for _, pct := range []float64{0, 0.5, 100.1, 150, -10} {
		tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: pct}
		_, err := tc.ResolveMaxCapital(floatPtr(10000))
		if err == nil {
			t.Errorf("expected error for out-of-range percent %v, got nil", pct)
		}
	}
}

func TestResolveMaxCapital_NetLiqUnavailable_NoFallback(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 60, MaxCapital: 5000}

	// nil Net Liq -> error (no fallback to MaxCapital).
	if _, err := tc.ResolveMaxCapital(nil); err == nil {
		t.Error("expected error for nil netLiq, got nil")
	}

	// zero Net Liq -> error.
	if _, err := tc.ResolveMaxCapital(floatPtr(0)); err == nil {
		t.Error("expected error for zero netLiq, got nil")
	}

	// negative Net Liq -> error.
	if _, err := tc.ResolveMaxCapital(floatPtr(-100)); err == nil {
		t.Error("expected error for negative netLiq, got nil")
	}
}

func TestResolveMaxCapital_NoFallbackMessage(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 60}
	_, err := tc.ResolveMaxCapital(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "net liq") {
		t.Errorf("expected error to mention net liq, got %q", err.Error())
	}
}

func TestCalculateUnitsWithCapital_Spread(t *testing.T) {
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 20}

	// $6,000 / (20 * 100) = 3.0 -> 3 units.
	if got := tc.CalculateUnitsWithCapital(6000); got != 3 {
		t.Errorf("CalculateUnitsWithCapital(6000) = %d, want 3", got)
	}

	// $5,000 / (20 * 100) = 2.5 -> 2 units (int truncation).
	if got := tc.CalculateUnitsWithCapital(5000); got != 2 {
		t.Errorf("CalculateUnitsWithCapital(5000) = %d, want 2", got)
	}

	// Too small -> 0 units.
	if got := tc.CalculateUnitsWithCapital(1000); got != 0 {
		t.Errorf("CalculateUnitsWithCapital(1000) = %d, want 0", got)
	}
}

func TestCalculateUnitsWithCapital_IronCondorWiderSide(t *testing.T) {
	tc := &TradeConfiguration{
		Strategy:       StrategyIronCondor,
		Width:          10, // should be ignored in favor of wider side
		PutSideConfig:  &IronCondorSideConfig{Width: 50},
		CallSideConfig: &IronCondorSideConfig{Width: 30},
	}

	// Wider side is 50 -> $12,000 / (50 * 100) = 2.4 -> 2 units.
	if got := tc.CalculateUnitsWithCapital(12000); got != 2 {
		t.Errorf("IC CalculateUnitsWithCapital(12000) = %d, want 2", got)
	}
}

func TestCalculateUnitsWithCapital_ZeroWidth(t *testing.T) {
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 0}
	if got := tc.CalculateUnitsWithCapital(6000); got != 0 {
		t.Errorf("CalculateUnitsWithCapital with zero width = %d, want 0", got)
	}
}

func TestCalculateUnits_BackwardCompatible(t *testing.T) {
	// CalculateUnits must delegate to CalculateUnitsWithCapital(MaxCapital),
	// preserving legacy fixed-mode sizing exactly.
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 20, MaxCapital: 6000}
	if got := tc.CalculateUnits(); got != 3 {
		t.Errorf("CalculateUnits() = %d, want 3", got)
	}
	if tc.CalculateUnits() != tc.CalculateUnitsWithCapital(tc.MaxCapital) {
		t.Error("CalculateUnits() must equal CalculateUnitsWithCapital(MaxCapital)")
	}
}

func TestNewTradeConfiguration_DefaultsFixed(t *testing.T) {
	tc := NewTradeConfiguration()
	if tc.EffectiveCapitalMode() != CapitalModeFixed {
		t.Errorf("new trade config should default to fixed mode, got %q", tc.EffectiveCapitalMode())
	}
	if tc.MaxCapital != 5000 {
		t.Errorf("new trade config MaxCapital = %v, want 5000", tc.MaxCapital)
	}
}
