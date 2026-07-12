package types

import (
	"math"
	"testing"
)

// qaFloatPtr is a local helper (the dev's floatPtr lives in capital_test.go in
// the same package, but we keep an independent one to stay self-contained).
func qaFloatPtr(v float64) *float64 { return &v }

// TestQA_ResolveMaxCapital_InclusiveBoundaries verifies the 1 and 100 endpoints
// are ACCEPTED (the dev only tested that 0.5 and 100.1 are rejected and that 60
// is accepted — the exact inclusive boundaries were never asserted).
func TestQA_ResolveMaxCapital_InclusiveBoundaries(t *testing.T) {
	cases := []struct {
		pct    float64
		netLiq float64
		want   float64
	}{
		{1, 10000, 100},     // lower boundary
		{100, 10000, 10000}, // upper boundary (100% of Net Liq)
	}
	for _, c := range cases {
		tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: c.pct}
		got, err := tc.ResolveMaxCapital(qaFloatPtr(c.netLiq))
		if err != nil {
			t.Errorf("pct=%v netLiq=%v: unexpected error %v", c.pct, c.netLiq, err)
			continue
		}
		if got != c.want {
			t.Errorf("pct=%v netLiq=%v: got %v, want %v", c.pct, c.netLiq, got, c.want)
		}
	}
}

// TestQA_ResolveMaxCapital_NaNPercent is an adversarial test: a corrupted or
// maliciously-crafted persisted config could carry a NaN percent. The range
// check `pct < 1 || pct > 100` is FALSE for NaN (all comparisons with NaN are
// false), so NaN slips through validation and yields a NaN dollar cap. A NaN cap
// then flows into CalculateUnitsWithCapital -> int(NaN/x) which is implementation
// -defined and yields 0, silently blocking the trade rather than failing loudly.
// EXPECTATION: percent mode should reject non-finite percentages.
func TestQA_ResolveMaxCapital_NaNPercent(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: math.NaN()}
	got, err := tc.ResolveMaxCapital(qaFloatPtr(10000))
	if err == nil {
		t.Errorf("expected error for NaN percent, got nil (resolved=%v). NaN bypasses the 1..100 range check.", got)
	}
}

// TestQA_ResolveMaxCapital_InfPercent — +Inf similarly bypasses `pct > 100`?
// Actually +Inf > 100 is true, so it should be rejected. This test pins that
// behavior so a future refactor can't regress it.
func TestQA_ResolveMaxCapital_InfPercent(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: math.Inf(1)}
	if _, err := tc.ResolveMaxCapital(qaFloatPtr(10000)); err == nil {
		t.Error("expected error for +Inf percent, got nil")
	}
}

// TestQA_ResolveMaxCapital_NaNNetLiq is an adversarial test on the OTHER input:
// a NaN Net Liq. The check `*netLiq <= 0` is FALSE for NaN, so a NaN Net Liq
// passes the guard and produces a NaN cap — violating AC-6's "non-positive or
// unavailable Net Liq must fail" spirit (NaN is neither positive nor usable).
func TestQA_ResolveMaxCapital_NaNNetLiq(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 60}
	got, err := tc.ResolveMaxCapital(qaFloatPtr(math.NaN()))
	if err == nil {
		t.Errorf("expected error for NaN Net Liq, got nil (resolved=%v). NaN bypasses the <=0 guard.", got)
	}
}

// TestQA_CalculateUnitsWithCapital_TruncationBoundary pins the exact int-
// truncation behavior at unit boundaries. $4000 / (20*100) = 2.0 -> 2 units,
// but $3999.99 -> 1.9999 -> 1 unit. Percent mode can easily land on fractional
// dollars (e.g. 39.9999% of Net Liq), so this is a real edge.
func TestQA_CalculateUnitsWithCapital_TruncationBoundary(t *testing.T) {
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 20}
	if got := tc.CalculateUnitsWithCapital(4000); got != 2 {
		t.Errorf("4000 -> %d units, want 2", got)
	}
	if got := tc.CalculateUnitsWithCapital(3999.99); got != 1 {
		t.Errorf("3999.99 -> %d units, want 1 (int truncation)", got)
	}
	// Just below one unit -> 0 (trade should be blocked).
	if got := tc.CalculateUnitsWithCapital(1999.99); got != 0 {
		t.Errorf("1999.99 -> %d units, want 0", got)
	}
}

// TestQA_CalculateUnitsWithCapital_NegativeCapital — defensive: a negative
// resolved cap (should never happen, but no fallback means we want deterministic
// behavior) must yield 0 units, never a negative or panic.
func TestQA_CalculateUnitsWithCapital_NegativeCapital(t *testing.T) {
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 20}
	if got := tc.CalculateUnitsWithCapital(-6000); got != 0 {
		t.Errorf("negative capital -> %d units, want 0", got)
	}
}

// TestQA_CalculateUnitsWithCapital_IronCondorEqualWidths verifies the wider-side
// selection when both sides are equal (boundary of the > comparison uses the
// call side via the else branch). Dev tested 50 vs 30; this pins the tie case.
func TestQA_CalculateUnitsWithCapital_IronCondorEqualWidths(t *testing.T) {
	tc := &TradeConfiguration{
		Strategy:       StrategyIronCondor,
		Width:          10,
		PutSideConfig:  &IronCondorSideConfig{Width: 40},
		CallSideConfig: &IronCondorSideConfig{Width: 40},
	}
	// 40 wide -> $8000 / (40*100) = 2.0 -> 2 units.
	if got := tc.CalculateUnitsWithCapital(8000); got != 2 {
		t.Errorf("equal-width IC -> %d units, want 2", got)
	}
}

// TestQA_CalculateUnitsWithCapital_IronCondorOnlyPutSide is an adversarial test
// on a malformed IC config: PutSideConfig set but CallSideConfig nil. The
// wider-side branch requires BOTH non-nil, so it silently falls back to the
// top-level Width (10). This pins that behavior so it's a conscious contract,
// not an accident. Width 10 -> $6000/(10*100)=6 units.
func TestQA_CalculateUnitsWithCapital_IronCondorOnlyPutSide(t *testing.T) {
	tc := &TradeConfiguration{
		Strategy:      StrategyIronCondor,
		Width:         10,
		PutSideConfig: &IronCondorSideConfig{Width: 50},
		// CallSideConfig intentionally nil
	}
	got := tc.CalculateUnitsWithCapital(6000)
	if got != 6 {
		t.Errorf("IC with nil call side falls back to top-level width 10 -> %d units, want 6 "+
			"(if this changed, the wider-side guard was altered)", got)
	}
}

// TestQA_ResolveMaxCapital_FixedIgnoresBogusPercent confirms that in fixed mode a
// stale/garbage MaxCapitalPercent (e.g. left over from a prior percent config) is
// completely ignored and never causes an error — backward-compat safety (AC-3).
func TestQA_ResolveMaxCapital_FixedIgnoresBogusPercent(t *testing.T) {
	tc := &TradeConfiguration{
		CapitalMode:       CapitalModeFixed,
		MaxCapital:        7500,
		MaxCapitalPercent: 9999, // garbage, must be ignored
	}
	got, err := tc.ResolveMaxCapital(nil)
	if err != nil {
		t.Fatalf("fixed mode with garbage percent should not error: %v", err)
	}
	if got != 7500 {
		t.Errorf("fixed mode resolved %v, want 7500", got)
	}
}
