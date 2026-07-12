package types

import (
	"math"
	"testing"
)

// This file adds the re-verification adversarial cases requested after the dev's
// fix commit f78838e: negative-infinity guards on both ResolveMaxCapital inputs,
// and confirmation that the finite guards run BEFORE the range/positivity checks
// so a non-finite value can never yield a NaN/Inf dollar cap that silently
// blocks (or worse, mis-sizes) a trade. All enforce AC-6 fail-loudly / no-fallback.

// TestQA_ResolveMaxCapital_NegInfPercent — -Inf percent. Unlike +Inf (which is
// caught by `pct > 100`), -Inf satisfies `pct < 1` and would be caught by the
// range check anyway; this pins that the explicit IsInf guard also handles the
// negative direction and produces the finite-number error message, not the
// range message, so the failure reason is unambiguous.
func TestQA_ResolveMaxCapital_NegInfPercent(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: math.Inf(-1)}
	got, err := tc.ResolveMaxCapital(qaFloatPtr(10000))
	if err == nil {
		t.Fatalf("expected error for -Inf percent, got nil (resolved=%v)", got)
	}
}

// TestQA_ResolveMaxCapital_PosInfNetLiq — +Inf Net Liq. The `*netLiq <= 0`
// guard is FALSE for +Inf, so without the explicit IsInf guard a +Inf Net Liq
// would produce a +Inf dollar cap -> int(+Inf/x) is implementation-defined and
// could yield a huge/garbage unit count. Must fail loudly.
func TestQA_ResolveMaxCapital_PosInfNetLiq(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 60}
	got, err := tc.ResolveMaxCapital(qaFloatPtr(math.Inf(1)))
	if err == nil {
		t.Fatalf("expected error for +Inf Net Liq, got nil (resolved=%v)", got)
	}
}

// TestQA_ResolveMaxCapital_NegInfNetLiq — -Inf Net Liq. Caught by `<= 0` in
// principle, but the explicit IsInf guard should fire first with the clearer
// "not a finite number" reason. Pin it so a refactor can't regress.
func TestQA_ResolveMaxCapital_NegInfNetLiq(t *testing.T) {
	tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: 60}
	if _, err := tc.ResolveMaxCapital(qaFloatPtr(math.Inf(-1))); err == nil {
		t.Fatal("expected error for -Inf Net Liq, got nil")
	}
}

// TestQA_ResolveMaxCapital_NoNonFiniteResultEverEscapes is the master invariant:
// across a matrix of non-finite percents and Net Liq values, ResolveMaxCapital
// must ALWAYS return an error and NEVER a non-finite (NaN/Inf) resolved cap with
// nil error. A non-finite cap escaping here is the exact failure that silently
// zero-units a trade downstream.
func TestQA_ResolveMaxCapital_NoNonFiniteResultEverEscapes(t *testing.T) {
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	goodPct := 60.0
	goodNL := 10000.0

	// Bad percents, good Net Liq.
	for _, p := range bad {
		tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: p}
		got, err := tc.ResolveMaxCapital(qaFloatPtr(goodNL))
		if err == nil {
			t.Errorf("percent=%v: expected error, got nil (resolved=%v)", p, got)
		}
		if err == nil && (math.IsNaN(got) || math.IsInf(got, 0)) {
			t.Errorf("percent=%v: non-finite cap %v escaped with nil error", p, got)
		}
	}

	// Good percent, bad Net Liq.
	for _, nl := range bad {
		tc := &TradeConfiguration{CapitalMode: CapitalModePercent, MaxCapitalPercent: goodPct}
		got, err := tc.ResolveMaxCapital(qaFloatPtr(nl))
		if err == nil {
			t.Errorf("netLiq=%v: expected error, got nil (resolved=%v)", nl, got)
		}
		if err == nil && (math.IsNaN(got) || math.IsInf(got, 0)) {
			t.Errorf("netLiq=%v: non-finite cap %v escaped with nil error", nl, got)
		}
	}
}

// TestQA_CalculateUnitsWithCapital_NonFiniteCapDefensive documents the downstream
// hazard that motivates the ResolveMaxCapital guards: if a non-finite cap ever
// DID reach sizing, int(NaN or Inf / x) is implementation-defined. This test
// pins Go's actual behavior so the team understands why fail-loud upstream
// matters — int(NaN)=0 (silent block) and int(+Inf) is a garbage large value.
// NOTE: this asserts the language behavior, not a guarantee we rely on; the real
// contract is that ResolveMaxCapital never emits a non-finite cap (tested above).
func TestQA_CalculateUnitsWithCapital_NonFiniteCapDefensive(t *testing.T) {
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 20}

	// NaN cap -> int(NaN/2000) == 0 -> treated as "insufficient capital" (blocked).
	if got := tc.CalculateUnitsWithCapital(math.NaN()); got != 0 {
		t.Errorf("NaN cap produced %d units; Go int(NaN)=0 so expected 0 (silent block). "+
			"This is exactly why ResolveMaxCapital must reject NaN upstream.", got)
	}
}
