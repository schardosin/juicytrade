package handlers

import (
	"testing"

	"trade-backend-go/internal/automation/types"
)

// TestValidateCapitalConfig_FixedMode verifies fixed/legacy configs are never
// rejected by the percent-range check (AC-3).
func TestValidateCapitalConfig_FixedMode(t *testing.T) {
	cases := []types.TradeConfiguration{
		{CapitalMode: types.CapitalModeFixed, MaxCapital: 5000},
		{MaxCapital: 5000}, // legacy: empty mode => fixed
		// Even a bogus percent value is ignored when mode is fixed.
		{CapitalMode: types.CapitalModeFixed, MaxCapitalPercent: 999},
	}
	for i, tc := range cases {
		if err := validateCapitalConfig(&tc); err != nil {
			t.Errorf("case %d: expected no error for fixed mode, got %v", i, err)
		}
	}
}

// TestValidateCapitalConfig_PercentValid accepts in-range percentages (AC-2).
func TestValidateCapitalConfig_PercentValid(t *testing.T) {
	for _, pct := range []float64{1, 2.5, 50, 60, 100} {
		tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: pct}
		if err := validateCapitalConfig(&tc); err != nil {
			t.Errorf("expected no error for percent %v, got %v", pct, err)
		}
	}
}

// TestValidateCapitalConfig_PercentOutOfRange rejects out-of-range percentages
// with a clear message (AC-2).
func TestValidateCapitalConfig_PercentOutOfRange(t *testing.T) {
	for _, pct := range []float64{0, 0.5, 100.1, 150, -5} {
		tc := types.TradeConfiguration{CapitalMode: types.CapitalModePercent, MaxCapitalPercent: pct}
		err := validateCapitalConfig(&tc)
		if err == nil {
			t.Errorf("expected error for out-of-range percent %v, got nil", pct)
			continue
		}
		if err.Error() != "max_capital_percent must be between 1 and 100" {
			t.Errorf("percent %v: unexpected message %q", pct, err.Error())
		}
	}
}
