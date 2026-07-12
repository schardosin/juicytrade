package handlers

import (
	"math"
	"testing"

	"trade-backend-go/internal/automation/types"
)

// TestQA_ValidateCapitalConfig_NaNPercent is an adversarial test: validateCapitalConfig
// uses `tc.MaxCapitalPercent < 1 || tc.MaxCapitalPercent > 100`. For NaN both
// comparisons are FALSE, so a NaN percent PASSES validation on create/update and
// is persisted. At activation/execution ResolveMaxCapital then produces a NaN
// dollar cap. EXPECTATION: the backend should reject a non-finite percent at the
// API boundary (it never trusts the client).
func TestQA_ValidateCapitalConfig_NaNPercent(t *testing.T) {
	tc := types.TradeConfiguration{
		CapitalMode:       types.CapitalModePercent,
		MaxCapitalPercent: math.NaN(),
	}
	if err := validateCapitalConfig(&tc); err == nil {
		t.Error("expected validateCapitalConfig to reject NaN percent, got nil — NaN bypasses the 1..100 range check")
	}
}

// TestQA_ValidateCapitalConfig_ExactBoundaries pins that the inclusive endpoints
// 1 and 100 are accepted (the dev's list included 1 and 100 but this makes the
// boundary contract explicit and independent).
func TestQA_ValidateCapitalConfig_ExactBoundaries(t *testing.T) {
	for _, pct := range []float64{1, 100} {
		tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: pct}
		if err := validateCapitalConfig(&tc); err != nil {
			t.Errorf("boundary percent %v should be accepted, got %v", pct, err)
		}
	}
}

// TestQA_ValidateCapitalConfig_JustOutsideBoundaries — the smallest representable
// steps outside the range must be rejected. This guards against a future change
// to `<=`/`>=` that would wrongly reject 1 or 100, or `<`/`>` that would wrongly
// accept 0.9999999 / 100.0000001.
func TestQA_ValidateCapitalConfig_JustOutsideBoundaries(t *testing.T) {
	for _, pct := range []float64{0.9999999, 100.0000001} {
		tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: pct}
		if err := validateCapitalConfig(&tc); err == nil {
			t.Errorf("percent %v is out of range and must be rejected", pct)
		}
	}
}

// TestQA_ValidateCapitalConfig_PercentModeZeroValue documents that an explicit
// percent mode with a zero (or unset) percent — e.g. a client that toggled to
// percent but never entered a value — is correctly rejected rather than silently
// resolving to $0 capital.
func TestQA_ValidateCapitalConfig_PercentModeZeroValue(t *testing.T) {
	tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: 0}
	if err := validateCapitalConfig(&tc); err == nil {
		t.Error("percent mode with 0 percent must be rejected (would resolve to $0 cap)")
	}
}
