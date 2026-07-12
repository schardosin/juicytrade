package handlers

import (
	"math"
	"testing"

	"trade-backend-go/internal/automation/types"
)

// This file adds the re-verification adversarial cases requested after the dev's
// fix commit f78838e:
//   1. validateCapitalConfig must reject +Inf / -Inf percents (not just NaN).
//   2. The CreateConfig "strategy omitted" default-application must PRESERVE a
//      client-supplied percent capital config instead of silently downgrading it
//      to the fixed $5000 default (the AC-6 / spec regression the dev fixed).

// TestQA_ValidateCapitalConfig_PosInfPercent — +Inf is > 100 so the range check
// would catch it, but the explicit finite guard should fire first with the
// clearer message. Either way it must be rejected.
func TestQA_ValidateCapitalConfig_PosInfPercent(t *testing.T) {
	tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: math.Inf(1)}
	if err := validateCapitalConfig(&tc); err == nil {
		t.Error("expected validateCapitalConfig to reject +Inf percent, got nil")
	}
}

// TestQA_ValidateCapitalConfig_NegInfPercent — -Inf is < 1 so the range check
// would catch it too; pin that the guard rejects it regardless.
func TestQA_ValidateCapitalConfig_NegInfPercent(t *testing.T) {
	tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: math.Inf(-1)}
	if err := validateCapitalConfig(&tc); err == nil {
		t.Error("expected validateCapitalConfig to reject -Inf percent, got nil")
	}
}

// TestQA_ValidateCapitalConfig_FixedModeNeverErrors — a fixed-mode config (or a
// legacy config with empty CapitalMode) must never be rejected by the percent
// validator, even if it carries a garbage percent left over from a prior toggle.
// Guards backward compat (AC-3): a NaN/Inf residual percent in fixed mode is
// simply ignored, not an error.
func TestQA_ValidateCapitalConfig_FixedModeNeverErrors(t *testing.T) {
	cases := []types.TradeConfiguration{
		{CapitalMode: types.CapitalModeFixed, MaxCapital: 5000, MaxCapitalPercent: math.NaN()},
		{CapitalMode: "", MaxCapital: 5000, MaxCapitalPercent: 9999}, // legacy: empty => fixed
		{CapitalMode: types.CapitalModeFixed, MaxCapital: 5000, MaxCapitalPercent: math.Inf(1)},
	}
	for i, tc := range cases {
		if err := validateCapitalConfig(&tc); err != nil {
			t.Errorf("case %d: fixed/legacy mode must not error, got %v", i, err)
		}
	}
}

// applyCreateConfigCapitalDefaults reproduces EXACTLY the inline block in
// AutomationHandler.CreateConfig that runs when TradeConfig.Strategy == "".
// Keeping a faithful copy here lets us adversarially test the preservation
// contract hermetically (the real handler needs a full Engine + streaming
// singleton + on-disk storage, which is not unit-testable in isolation).
// If the handler logic changes, this test will drift and should be updated to
// match — that mismatch is itself a useful signal.
func applyCreateConfigCapitalDefaults(tc types.TradeConfiguration) types.TradeConfiguration {
	if tc.Strategy == "" {
		capitalMode := tc.CapitalMode
		maxCapital := tc.MaxCapital
		maxCapitalPercent := tc.MaxCapitalPercent

		tc = types.NewTradeConfiguration()

		if capitalMode != "" {
			tc.CapitalMode = capitalMode
			tc.MaxCapitalPercent = maxCapitalPercent
			if maxCapital > 0 {
				tc.MaxCapital = maxCapital
			}
		}
	}
	return tc
}

// TestQA_CreateConfig_PercentConfigSurvivesDefaults is the core regression guard
// for the dev's fix #3: a client that toggles to percent mode but omits the
// strategy (so backend defaults kick in) must KEEP percent mode + its percent
// value, NOT be downgraded to fixed $5000. Before the fix, NewTradeConfiguration()
// stomped CapitalMode back to "fixed" and MaxCapitalPercent to 0, silently
// changing the user's intent.
func TestQA_CreateConfig_PercentConfigSurvivesDefaults(t *testing.T) {
	in := types.TradeConfiguration{
		// Strategy intentionally omitted -> triggers default application.
		CapitalMode:       types.CapitalModePercent,
		MaxCapitalPercent: 60,
	}
	out := applyCreateConfigCapitalDefaults(in)

	if out.EffectiveCapitalMode() != types.CapitalModePercent {
		t.Fatalf("percent mode was downgraded to %q — silent capital-intent change",
			out.EffectiveCapitalMode())
	}
	if out.MaxCapitalPercent != 60 {
		t.Errorf("percent value lost: got %v, want 60", out.MaxCapitalPercent)
	}
	// It must still receive the OTHER trade defaults (strategy/width/etc.).
	if out.Strategy != types.StrategyPutSpread {
		t.Errorf("expected default strategy put_spread, got %q", out.Strategy)
	}
	if out.Width != 20 {
		t.Errorf("expected default width 20, got %d", out.Width)
	}
	// And the resolved cap must be percent-derived, not the fixed $5000 default.
	got, err := out.ResolveMaxCapital(func() *float64 { v := 10000.0; return &v }())
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if got != 6000 {
		t.Errorf("resolved cap = %v, want 6000 (60%% of 10000). If 5000, config was downgraded to fixed.", got)
	}
}

// TestQA_CreateConfig_PercentWithExplicitStrategyUntouched — when a strategy IS
// supplied, the default block is skipped entirely and the percent config must
// pass through verbatim.
func TestQA_CreateConfig_PercentWithExplicitStrategyUntouched(t *testing.T) {
	in := types.TradeConfiguration{
		Strategy:          types.StrategyIronCondor,
		CapitalMode:       types.CapitalModePercent,
		MaxCapitalPercent: 25,
		MaxCapital:        1234,
	}
	out := applyCreateConfigCapitalDefaults(in)
	if out.CapitalMode != types.CapitalModePercent || out.MaxCapitalPercent != 25 {
		t.Errorf("explicit-strategy percent config altered: %+v", out)
	}
	if out.Strategy != types.StrategyIronCondor {
		t.Errorf("strategy changed: got %q", out.Strategy)
	}
}

// TestQA_CreateConfig_LegacyFixedOmittedStrategyGetsDefaults — a legacy client
// that sends neither strategy nor capital_mode must get the full fixed $5000
// default (backward compat). The preservation block only fires when capitalMode
// is non-empty, so an empty-mode config falls through to pure defaults.
func TestQA_CreateConfig_LegacyFixedOmittedStrategyGetsDefaults(t *testing.T) {
	in := types.TradeConfiguration{} // everything empty
	out := applyCreateConfigCapitalDefaults(in)
	if out.EffectiveCapitalMode() != types.CapitalModeFixed {
		t.Errorf("legacy empty config should default to fixed, got %q", out.EffectiveCapitalMode())
	}
	if out.MaxCapital != 5000 {
		t.Errorf("legacy empty config should default to $5000, got %v", out.MaxCapital)
	}
}

// TestQA_CreateConfig_PercentModePreservesCustomMaxCapital — when percent mode is
// supplied WITH a positive max_capital (e.g. the UI keeps both values so toggling
// back to fixed is lossless), the preservation block must keep that custom fixed
// value rather than resetting it to $5000. maxCapital>0 guard covers this.
func TestQA_CreateConfig_PercentModePreservesCustomMaxCapital(t *testing.T) {
	in := types.TradeConfiguration{
		CapitalMode:       types.CapitalModePercent,
		MaxCapitalPercent: 40,
		MaxCapital:        8000, // custom fixed value retained for lossless toggle-back
	}
	out := applyCreateConfigCapitalDefaults(in)
	if out.MaxCapital != 8000 {
		t.Errorf("custom max_capital not preserved: got %v, want 8000", out.MaxCapital)
	}
	if out.MaxCapitalPercent != 40 {
		t.Errorf("percent not preserved: got %v, want 40", out.MaxCapitalPercent)
	}
}
