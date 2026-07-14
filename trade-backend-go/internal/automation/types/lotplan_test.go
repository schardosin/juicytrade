package types

import "testing"

func multiLotAutomation(plan []int, lotIndex int, legsDrift bool) *ActiveAutomation {
	return &ActiveAutomation{
		Config: &AutomationConfig{
			TradeConfig: TradeConfiguration{LegsDrift: legsDrift},
		},
		OrderPlan:       plan,
		CurrentLotIndex: lotIndex,
	}
}

// TestIsMultiLot verifies single-order/legacy runs never engage multi-order paths.
func TestIsMultiLot(t *testing.T) {
	cases := []struct {
		name string
		plan []int
		want bool
	}{
		{"nil_plan_legacy", nil, false},
		{"empty_plan", []int{}, false},
		{"single_lot", []int{6}, false},
		{"two_lots", []int{2, 2}, true},
		{"four_lots", []int{2, 2, 2, 1}, true},
	}
	for _, tc := range cases {
		a := multiLotAutomation(tc.plan, 0, false)
		if got := a.IsMultiLot(); got != tc.want {
			t.Errorf("IsMultiLot(%v) = %v, want %v", tc.plan, got, tc.want)
		}
	}
}

// TestHasMorePendingLots pins the sequential advance decision (AC-5).
func TestHasMorePendingLots(t *testing.T) {
	cases := []struct {
		name     string
		plan     []int
		lotIndex int
		want     bool
	}{
		{"single_lot_done", []int{6}, 0, false},
		{"first_of_three", []int{2, 2, 2}, 0, true},
		{"middle_of_three", []int{2, 2, 2}, 1, true},
		{"last_of_three", []int{2, 2, 2}, 2, false},
		{"last_of_four_remainder", []int{2, 2, 2, 1}, 3, false},
		{"third_of_four_remainder", []int{2, 2, 2, 1}, 2, true},
	}
	for _, tc := range cases {
		a := multiLotAutomation(tc.plan, tc.lotIndex, false)
		if got := a.HasMorePendingLots(); got != tc.want {
			t.Errorf("%s: HasMorePendingLots(plan=%v idx=%d) = %v, want %v", tc.name, tc.plan, tc.lotIndex, got, tc.want)
		}
	}
}

// TestDriftAllowed_SingleOrder verifies OD-1: single-order runs preserve today's
// delta-drift behavior EXACTLY, regardless of the legs_drift value.
func TestDriftAllowed_SingleOrder(t *testing.T) {
	for _, legsDrift := range []bool{false, true} {
		a := multiLotAutomation([]int{6}, 0, legsDrift)
		if !a.DriftAllowed(0.01) {
			t.Errorf("single-order run (legsDrift=%v) must allow drift when limit>0 (OD-1)", legsDrift)
		}
		// A drift limit of 0 disables drift as it does today.
		if a.DriftAllowed(0) {
			t.Errorf("single-order run (legsDrift=%v) must NOT allow drift when limit==0", legsDrift)
		}
	}
}

// TestDriftAllowed_MultiLot verifies AC-7 / AC-8 drift gating for multi-lot runs.
func TestDriftAllowed_MultiLot(t *testing.T) {
	// legs_drift == false: drift disabled for the whole run (AC-7).
	frozen := multiLotAutomation([]int{2, 2, 2}, 1, false)
	if frozen.DriftAllowed(0.01) {
		t.Error("multi-lot run with legs_drift=false must NOT allow mid-order drift (AC-7)")
	}

	// legs_drift == true: existing mid-order drift still applies (AC-8).
	drifting := multiLotAutomation([]int{2, 2, 2}, 1, true)
	if !drifting.DriftAllowed(0.01) {
		t.Error("multi-lot run with legs_drift=true must allow mid-order drift (AC-8)")
	}
	// Even with legs_drift=true, a zero limit disables drift.
	if drifting.DriftAllowed(0) {
		t.Error("drift must be disabled when limit==0 even with legs_drift=true")
	}
}

// TestShouldReuseLockedStrikes verifies strike-selection per lot (AC-7 / AC-8).
func TestShouldReuseLockedStrikes(t *testing.T) {
	// legs_drift == false: lot 0 finds fresh, lots >=1 reuse locked strikes.
	frozen := multiLotAutomation([]int{2, 2, 2}, 0, false)
	if frozen.ShouldReuseLockedStrikes(0) {
		t.Error("lot 0 must always find fresh strikes, never reuse")
	}
	if !frozen.ShouldReuseLockedStrikes(1) {
		t.Error("legs_drift=false: lot 1 must reuse locked strikes (AC-7)")
	}
	if !frozen.ShouldReuseLockedStrikes(2) {
		t.Error("legs_drift=false: lot 2 must reuse locked strikes (AC-7)")
	}

	// legs_drift == true: every lot re-selects strikes.
	drifting := multiLotAutomation([]int{2, 2, 2}, 0, true)
	for i := 0; i < 3; i++ {
		if drifting.ShouldReuseLockedStrikes(i) {
			t.Errorf("legs_drift=true: lot %d must re-select strikes, not reuse (AC-8)", i)
		}
	}
}
